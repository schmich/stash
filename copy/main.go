package main

import (
	"context"
	"encoding/base64"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/flowup/cloudfunc/api"
	"github.com/schmich/stash/identifier"
)

type CopyRequest struct {
	Payload string `json:"payload"`
}

type CopyResponse struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

func store(encodedPayload string) (string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", err
	}

	id, err := identifier.New()
	if err != nil {
		return "", err
	}

	// TODO: Ensure object with ID does not already exist.

	bucket := client.Bucket("stash-215008")
	obj := bucket.Object(id)

	writer := obj.NewWriter(ctx)
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encodedPayload))
	if _, err := io.Copy(writer, reader); err != nil {
		return "", err
	}

	if err = writer.Close(); err != nil {
		return "", err
	}

	return id, nil
}

func run(function *api.CloudFunc) (string, error) {
	req, err := function.GetRequest()
	if err != nil {
		return "", err
	}

	var input CopyRequest
	if err = req.BindBody(&input); err != nil {
		return "", err
	}

	return store(input.Payload)
}

func main() {
	function := api.NewCloudFunc()
	id, err := run(function)
	if err == nil {
		function.SendResponse(&CopyResponse{ID: id})
	} else {
		function.SendResponse(&CopyResponse{Error: err.Error()})
	}
}
