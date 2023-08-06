package toolchain

import (
	"archive/tar"
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
		// archiveExt = "zip"
		// TODO: implement unzipping
		log.Fatalf("windows not supported")
	}

	return fmt.Sprintf("https://go.dev/dl/%s.%s-%s.%s", g.version, os, arch, archiveExt)
}

func (g *GoToolchain) download() {
	downloadUrl := g.getDownloadUrl()

	prefix := fmt.Sprintf(".yabs/go/%s", g.version)

	g.binLocation = filepath.Join(prefix, "go", "bin")

	if _, err := os.Stat(prefix); os.IsNotExist(err) {
		os.MkdirAll(prefix, 0770)
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

	g.extractTarGz(prefix, resp.Body)
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
			if err := os.Mkdir(name, 0770); err != nil {
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
