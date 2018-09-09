package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

type encrypter struct {
	createStream func() (*cipher.StreamWriter, error)
	stream       *cipher.StreamWriter
}

type decrypter struct {
	createStream func() (*cipher.StreamReader, error)
	stream       *cipher.StreamReader
}

func NewEncrypter(writer io.Writer, password []byte) io.WriteCloser {
	encrypter := &encrypter{}

	encrypter.createStream = func() (*cipher.StreamWriter, error) {
		salt := make([]byte, 64)
		count, err := rand.Read(salt)
		if err != nil {
			return nil, err
		}

		if count != len(salt) {
			return nil, errors.New("failed to generate salt")
		}

		key := pbkdf2.Key(password, salt, 10000, aes.BlockSize, sha256.New)
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		iv := make([]byte, aes.BlockSize)
		count, err = rand.Read(iv)
		if err != nil {
			return nil, err
		}

		if count != len(iv) {
			return nil, errors.New("failed to generate IV")
		}

		if _, err := writer.Write(salt); err != nil {
			return nil, err
		}

		if _, err := writer.Write(iv); err != nil {
			return nil, err
		}

		return &cipher.StreamWriter{
			S: cipher.NewOFB(block, iv),
			W: &nopCloser{writer},
		}, nil
	}

	return encrypter
}

func (encrypter *encrypter) Write(buf []byte) (int, error) {
	if encrypter.stream == nil {
		var err error
		if encrypter.stream, err = encrypter.createStream(); err != nil {
			return 0, err
		}
	}
	return encrypter.stream.Write(buf)
}

func (encrypter *encrypter) Close() error {
	if encrypter.stream == nil {
		return nil
	}
	return encrypter.stream.Close()
}

func NewDecrypter(reader io.Reader, password []byte) io.Reader {
	decrypter := &decrypter{}

	decrypter.createStream = func() (*cipher.StreamReader, error) {
		salt := make([]byte, 64)
		if _, err := io.ReadFull(reader, salt); err != nil {
			return nil, err
		}

		key := pbkdf2.Key(password, salt, 10000, aes.BlockSize, sha256.New)
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}

		iv := make([]byte, aes.BlockSize)
		if _, err := io.ReadFull(reader, iv); err != nil {
			return nil, err
		}

		return &cipher.StreamReader{
			S: cipher.NewOFB(block, iv),
			R: reader,
		}, nil
	}

	return decrypter
}

func (decrypter *decrypter) Read(buf []byte) (int, error) {
	if decrypter.stream == nil {
		var err error
		if decrypter.stream, err = decrypter.createStream(); err != nil {
			return 0, err
		}
	}
	return decrypter.stream.Read(buf)
}
