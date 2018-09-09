package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"

	"cloud.google.com/go/storage"
	"github.com/flowup/cloudfunc/api"
)

type PasteRequest struct {
	ID string `json:"id"`
}

type PasteResponse struct {
	Payload string `json:"payload,omitempty"`
	Error   string `json:"error,omitempty"`
}

func retrieve(id string) (string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", err
	}

	bucket := client.Bucket("stash-215008")
	obj := bucket.Object(id)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return "", err
	}

	var encoded bytes.Buffer
	writer := base64.NewEncoder(base64.StdEncoding, &encoded)
	if _, err := io.Copy(writer, reader); err != nil {
		return "", err
	}

	if err = reader.Close(); err != nil {
		return "", err
	}

	if err = writer.Close(); err != nil {
		return "", err
	}

	return encoded.String(), nil
}

func run(function *api.CloudFunc) (string, error) {
	req, err := function.GetRequest()
	if err != nil {
		return "", err
	}

	var input PasteRequest
	if err = req.BindBody(&input); err != nil {
		return "", err
	}

	return retrieve(input.ID)
}

func main() {
	function := api.NewCloudFunc()
	payload, err := run(function)
	if err == nil {
		function.SendResponse(&PasteResponse{Payload: payload})
	} else {
		function.SendResponse(&PasteResponse{Error: err.Error()})
	}
}
