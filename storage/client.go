package storage

import (
	"io"
)

type Client interface {
	Upload() Uploader
	Download(string) io.ReadCloser
}

type Uploader interface {
	io.WriteCloser
	GetID() string
}