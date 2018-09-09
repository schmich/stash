package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/howeyc/gopass"
	cli "github.com/jawher/mow.cli"
	"github.com/mattn/go-isatty"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/schmich/stash/crypt"
	"github.com/schmich/stash/storage"
	log "github.com/sirupsen/logrus"
)

func pack(paths []string, writer io.Writer) error {
	archive := tar.NewWriter(writer)

	packFile := func(path string) error {
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		defer file.Close()

		if _, err = io.Copy(archive, file); err != nil {
			return err
		}

		return nil
	}

	packStdin := func() error {
		stdin, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return errors.Wrap(err, "create archive")
		}

		header := &tar.Header{Name: "$stdin", Size: int64(len(stdin))}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}

		if _, err := archive.Write(stdin); err != nil {
			return err
		}

		return nil
	}

	isTerminal := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
	if !isTerminal {
		log.Debug("Copy from stdin.")
		if err := packStdin(); err != nil {
			return err
		}
	} else if len(paths) == 0 {
		log.Info("Copy from stdin (^D when done).")
		if err := packStdin(); err != nil {
			return err
		}
	}

	for _, path := range paths {
		if path == "-" && isTerminal {
			log.Info("Copy from stdin (^D when done).")
			if err := packStdin(); err != nil {
				return err
			}
			continue
		}

		absname, err := filepath.Abs(path)
		if err != nil {
			return errors.Wrap(err, "create archive")
		}

		basename := filepath.Base(absname)

		err = filepath.Walk(path, func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// TODO: Convert Windows slahes to Unix slashes?
			// TODO: Store directories, too (e.g. empty dirs).
			if !info.Mode().IsRegular() {
				return nil
			}

			rel, err := filepath.Rel(path, file)
			if err != nil {
				return err
			}

			archivePath := filepath.Join(basename, rel)
			header, err := tar.FileInfoHeader(info, info.Name())
			header.Name = archivePath

			if err = archive.WriteHeader(header); err != nil {
				return err
			}

			log.Debugf("Copy %s.", archivePath)
			if err = packFile(file); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return errors.Wrap(err, "create archive")
		}
	}

	if err := archive.Close(); err != nil {
		return errors.Wrap(err, "create archive")
	}

	return nil
}

func unpack(reader io.Reader) error {
	archive := tar.NewReader(reader)

	unpackFile := func(header *tar.Header) error {
		file, err := os.OpenFile(header.Name, os.O_CREATE|os.O_TRUNC|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode))
		if err != nil {
			return err
		}

		defer file.Close()

		if _, err = io.Copy(file, archive); err != nil {
			return err
		}

		return nil
	}

	for {
		header, err := archive.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		} else if header == nil {
			continue
		}

		if header.Name == "$stdin" {
			io.Copy(os.Stdout, archive)
			continue
		}

		// TODO: Handle directories, links, devices, ...
		// TODO: Handle filename conflicts, handle relative paths, ...
		// TODO: Set access time, mod time, change time
		// TODO: Do not allow relative paths (resolve to something relative to pwd).

		directory := filepath.Dir(header.Name)
		if err = os.MkdirAll(directory, 0700); err != nil {
			return err
		}

		log.Debugf("Unpack %s.", header.Name)
		if err = unpackFile(header); err != nil {
			return err
		}
	}
}

func runCopy(client storage.Client, password []byte, paths []string) error {
	// files -> pack -> compress -> encrypt -> encode/upload

	// TODO: Limit upload size.
	log.Debug("Upload.")
	uploader := client.Upload()
	encrypter := crypt.NewEncrypter(uploader, password)
	compressor, err := gzip.NewWriterLevel(encrypter, gzip.BestCompression)
	if err != nil {
		return err
	}

	if err := pack(paths, compressor); err != nil {
		return err
	}

	if err := compressor.Close(); err != nil {
		return err
	}

	if err := encrypter.Close(); err != nil {
		return err
	}

	if err := uploader.Close(); err != nil {
		return err
	}

	log.Infof("Stash ID: %s", uploader.GetID())
	return nil
}

func runPaste(client storage.Client, password []byte, id string) error {
	// download/decode -> decrypt -> decompress -> unpack -> files

	log.Debug("Download.")
	downloader := client.Download(id)
	decrypter := crypt.NewDecrypter(downloader, password)
	decompressor, err := gzip.NewReader(decrypter)
	if err != nil {
		return err
	}

	if err := unpack(decompressor); err != nil {
		return err
	}

	if err := downloader.Close(); err != nil {
		return err
	}

	return nil
}

func getEnvPassword() []byte {
	value := os.Getenv("STASH_PASSWORD")
	if value != "" {
		return []byte(value)
	}

	return nil
}

func getConfigPassword() ([]byte, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return []byte{}, err
	}

	stashPath := filepath.Join(homeDir, ".stash")
	content, err := ioutil.ReadFile(stashPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return []byte{}, err
	}

	var config struct {
		Password string `json:"password"`
	}

	if err = json.Unmarshal(content, &config); err != nil {
		return []byte{}, err
	}

	if config.Password != "" {
		log.Debugf("Using password in %s.", stashPath)
		return []byte(config.Password), nil
	}

	return nil, nil
}

func getInteractivePassword() ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Password: ")
	return gopass.GetPasswd()
}

func getPassword(passwords ...string) ([]byte, error) {
	for _, password := range passwords {
		if password != "" {
			return []byte(password), nil
		}
	}

	password := getEnvPassword()
	if password != nil {
		return password, nil
	}

	password, err := getConfigPassword()
	if err != nil {
		return []byte{}, err
	} else if password != nil {
		return password, nil
	}

	return getInteractivePassword()
}

type plainFormatter struct {
}

func (f *plainFormatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(entry.Message + "\n"), nil
}

func main() {
	app := cli.App("stash", "Encrypted Internet clipboard")
	log.SetFormatter(&plainFormatter{})
	log.SetLevel(log.InfoLevel)
	log.SetOutput(os.Stderr)

	appVerbose := app.BoolOpt("v verbose", false, "Verbose output")
	appPassword := app.StringOpt("p password", "", "Password")

	client := storage.NewGCPClient("https://us-central1-stash-215008.cloudfunctions.net")
	// client := storage.NewFilesystemClient("/tmp/stash-storage")
	// client := storage.NewInMemoryClient();

	app.Command("copy c", "Copy data: files, directories, and/or stdin", func(cmd *cli.Cmd) {
		copyPassword := cmd.StringOpt("p password", "", "Password")
		copyVerbose := cmd.BoolOpt("v verbose", false, "Verbose output")
		paths := cmd.StringsArg("PATH", nil, "File or directory to copy")
		cmd.Spec = "[OPTIONS] [PATH...]"

		cmd.Action = func() {
			if *copyVerbose || *appVerbose {
				log.SetLevel(log.DebugLevel)
			}

			password, err := getPassword(*copyPassword, *appPassword)
			if err != nil {
				log.Fatalf("Error: %s", err)
			}

			err = runCopy(client, password, *paths)
			if err != nil {
				log.Fatalf("Error: %s", err)
			}
		}
	})

	app.Command("paste p", "Paste data", func(cmd *cli.Cmd) {
		pastePassword := cmd.StringOpt("p password", "", "Password")
		pasteVerbose := cmd.BoolOpt("v verbose", false, "Verbose output")
		parts := cmd.StringsArg("STASH_ID", nil, "Stash ID from copy")
		cmd.Spec = "[OPTIONS] STASH_ID..."

		cmd.Action = func() {
			if *pasteVerbose || *appVerbose {
				log.SetLevel(log.DebugLevel)
			}

			password, err := getPassword(*pastePassword, *appPassword)
			if err != nil {
				log.Fatalf("Error: %s", err)
			}

			id := strings.Join(*parts, " ")
			err = runPaste(client, password, id)
			if err != nil {
				log.Fatalf("Error: %s", err)
			}
		}
	})

	app.Run(os.Args)
}
