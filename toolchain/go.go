package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

type GoToolchain struct {
	version     string
	binLocation string
}

//

func newGo(version string) *GoToolchain {
	return &GoToolchain{
		version: version,
	}
}

func (g *GoToolchain) getDownloadUrl() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	archiveExt := "tar.gz"
	if os == "windows" {
		archiveExt = "zip"
	}

	return fmt.Sprintf("https://go.dev/dl/%s.%s-%s.%s", g.version, os, arch, archiveExt)
}

func (g *GoToolchain) download() {
	downloadUrl := g.getDownloadUrl()

	prefix := fmt.Sprintf(".yabs/go/%s", g.version)

	g.binLocation = filepath.Join(prefix, "go", "bin")

	if _, err := os.Stat(prefix); os.IsNotExist(err) {
		if err = os.MkdirAll(prefix, os.ModePerm); err != nil {
			log.Fatalf("downloading: %s", err)
		}
	} else {
		log.Printf("already have %s", g.version)
		return
	}

	log.Printf("downloading %q from %s", g.version, downloadUrl)

	resp, err := http.Get(downloadUrl)
	if err != nil {
		log.Fatalf("getting: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("bad status: %s", resp.Status)
	}

	if runtime.GOOS == "windows" {
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		size, err := io.Copy(writer, resp.Body)
		if err != nil {
			log.Fatalf("writing zip to buf: %s", err)
		}

		reader := bytes.NewReader(buf.Bytes())
		log.Printf("extracting zip")
		if err := g.extractZip(prefix, reader, size); err != nil {
			log.Fatal(err)
		}

	} else {
		log.Printf("extracting tar.gz")
		if err := g.extractTarGz(prefix, resp.Body); err != nil {
			log.Fatal(err)
		}
	}
}

func (g *GoToolchain) extractTarGz(prefix string, r io.Reader) error {
	tarStream, err := gzip.NewReader(r)
	if err != nil {
		log.Fatalf("tar gz: gzip: %s", err)
	}

	tarReader := tar.NewReader(tarStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		name := filepath.Join(prefix, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(name, os.ModePerm); err != nil {
				log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0777)
			if err != nil {
				log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
		default:
			log.Fatalf(
				"ExtractTarGz: uknown type: %d in %s",
				header.Typeflag,
				header.Name)
		}
	}

	return nil
}

func (g *GoToolchain) extractZip(prefix string, r io.ReaderAt, size int64) error {
	zp, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	for _, file := range zp.File {
		path := filepath.Join(prefix, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, os.ModePerm); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
				return err
			}

			fd, err := file.Open()
			if err != nil {
				return err
			}

			dst, err := os.Create(path)
			if err != nil {
				fd.Close()
				return err
			}

			if _, err = io.Copy(dst, fd); err != nil {
				return err
			}

			fd.Close()
			dst.Close()
		}
	}

	return nil
}
