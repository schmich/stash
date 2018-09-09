package storage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/ddliu/go-httpclient"
)

type CopyRequest struct {
	Payload string `json:"payload"`
}

type CopyResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type PasteRequest struct {
	ID string `json:"id"`
}

type PasteResponse struct {
	Payload string `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
}

type gcpClient struct {
	endpoint string
}

type gcpUploader struct {
	buffer   bytes.Buffer
	writer   io.WriteCloser
	endpoint string
	id       string
}

type gcpDownloader struct {
	reader   io.Reader
	endpoint string
	id       string
}

func NewGCPClient(endpoint string) Client {
	return &gcpClient{endpoint: endpoint}
}

func (client *gcpClient) Upload() Uploader {
	uploader := &gcpUploader{endpoint: client.endpoint}
	uploader.writer = base64.NewEncoder(base64.StdEncoding, &uploader.buffer)
	return uploader
}

func (uploader *gcpUploader) GetID() string {
	return uploader.id
}

func (uploader *gcpUploader) Write(buf []byte) (int, error) {
	return uploader.writer.Write(buf)
}

func (uploader *gcpUploader) Close() error {
	if err := uploader.writer.Close(); err != nil {
		return err
	}

	payload := uploader.buffer.String()
	request := CopyRequest{Payload: payload}
	res, err := httpclient.
		WithHeader("Content-Type", "application/json").
		PostJson(uploader.endpoint+"/copy", request)

	if err != nil {
		return err
	}

	body, err := res.ReadAll()
	if err != nil {
		return err
	}

	var response CopyResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return err
	}

	if response.Error != "" {
		return errors.New(response.Error)
	}

	uploader.id = response.ID
	return nil
}

func (client *gcpClient) Download(id string) io.ReadCloser {
	return &gcpDownloader{endpoint: client.endpoint, id: id}
}

func (downloader *gcpDownloader) Read(buf []byte) (int, error) {
	if downloader.reader == nil {
		request := PasteRequest{ID: downloader.id}
		res, err := httpclient.
			WithHeader("Content-Type", "application/json").
			PostJson(downloader.endpoint+"/paste", request)

		if err != nil {
			return 0, err
		}

		body, err := res.ReadAll()
		if err != nil {
			return 0, err
		}

		var response PasteResponse
		err = json.Unmarshal(body, &response)
		if err != nil {
			return 0, err
		}

		if response.Error != "" {
			return 0, errors.New(response.Error)
		}

		downloader.reader = base64.NewDecoder(base64.StdEncoding, strings.NewReader(response.Payload))
	}

	return downloader.reader.Read(buf)
}

func (downloader *gcpDownloader) Close() error {
	return nil
}
