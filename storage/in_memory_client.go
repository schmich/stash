package storage

import (
	"bytes"
	"fmt"
	"io"

	"github.com/schmich/stash/identifier"
)

type inMemoryClient struct {
	storage map[string][]byte
}

func NewInMemoryClient() Client {
	return &inMemoryClient{storage: make(map[string][]byte)}
}

type inMemoryUploader struct {
	client *inMemoryClient
	buffer bytes.Buffer
	id     string
}

type inMemoryDownloader struct {
	buffer *bytes.Buffer
	err    error
}

func (client *inMemoryClient) Upload() Uploader {
	return &inMemoryUploader{client: client}
}

func (uploader *inMemoryUploader) Write(buf []byte) (int, error) {
	return uploader.buffer.Write(buf)
}

func (uploader *inMemoryUploader) Close() error {
	var err error
	uploader.id, err = identifier.New()
	if err != nil {
		return err
	}

	uploader.client.storage[uploader.id] = uploader.buffer.Bytes()
	return nil
}

func (uploader *inMemoryUploader) GetID() string {
	return uploader.id
}

func (client *inMemoryClient) Download(id string) io.ReadCloser {
	if payload, ok := client.storage[id]; ok {
		return &inMemoryDownloader{buffer: bytes.NewBuffer(payload)}
	}

	return &inMemoryDownloader{
		err: fmt.Errorf("payload not found for \"%s\"", id),
	}
}

func (downloader *inMemoryDownloader) Read(buf []byte) (int, error) {
	if downloader.err != nil {
		return 0, downloader.err
	}

	return downloader.buffer.Read(buf)
}

func (downloader *inMemoryDownloader) Close() error {
	return downloader.err
}
