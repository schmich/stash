package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/schmich/stash/identifier"
)

type filesystemClient struct {
	directory string
}

type filesystemUploader struct {
	writer io.WriteCloser
	err    error
	id     string
}

type filesystemDownloader struct {
	reader io.ReadCloser
	err    error
}

func NewFilesystemClient(directory string) Client {
	return &filesystemClient{directory: directory}
}

func (client *filesystemClient) ensureStorageExists() error {
	info, err := os.Stat(client.directory)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return os.MkdirAll(client.directory, 0700)
	}

	if !info.IsDir() {
		return fmt.Errorf("storage already exists but is not a directory: \"%s\"", client.directory)
	}

	return nil
}

func (client *filesystemClient) Upload() Uploader {
	err := client.ensureStorageExists()
	if err != nil {
		return &filesystemUploader{err: err}
	}

	id, err := identifier.New()
	if err != nil {
		return &filesystemUploader{err: err}
	}

	path := filepath.Join(client.directory, id)
	file, err := os.Create(path)
	if err != nil {
		return &filesystemUploader{err: err}
	}

	return &filesystemUploader{writer: file, id: id}
}

func (client *filesystemClient) Download(id string) io.ReadCloser {
	path := filepath.Join(client.directory, id)
	file, err := os.Open(path)
	if err != nil {
		return &filesystemDownloader{err: err}
	}

	return &filesystemDownloader{reader: file}
}

func (uploader *filesystemUploader) Write(buf []byte) (int, error) {
	if uploader.err != nil {
		return 0, uploader.err
	}

	return uploader.writer.Write(buf)
}

func (uploader *filesystemUploader) Close() error {
	if uploader.err != nil {
		return uploader.err
	}

	return uploader.writer.Close()
}

func (uploader *filesystemUploader) GetID() string {
	return uploader.id
}

func (downloader *filesystemDownloader) Read(buf []byte) (int, error) {
	if downloader.err != nil {
		return 0, downloader.err
	}

	return downloader.reader.Read(buf)
}

func (downloader *filesystemDownloader) Close() error {
	if downloader.err != nil {
		return downloader.err
	}

	return downloader.reader.Close()
}
