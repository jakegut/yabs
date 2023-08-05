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
	version      string
	binLocation  string
	downloadPath string
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
		archiveExt = "zip"
	}

	return fmt.Sprintf("https://go.dev/dl/%s.%s-%s.%s", g.version, os, arch, archiveExt)
}

func (g *GoToolchain) download() {
	downloadUrl := g.getDownloadUrl()

	prefix := fmt.Sprintf(".yabs/go/%s", g.version)

	if _, err := os.Stat(prefix); os.IsNotExist(err) {
		os.MkdirAll(prefix, 0755)
	} else {
		log.Printf("already have %s", g.version)
		return
	}

	out, err := os.CreateTemp("", "yabs-go-toolchain-archive-")
	if err != nil {
		log.Fatalf("create temp: %s", err)
	}
	defer out.Close()

	log.Printf("downloading %q from %s to %s", g.version, downloadUrl, out.Name())

	resp, err := http.Get(downloadUrl)
	if err != nil {
		log.Fatalf("getting: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Fatalf("copying %s", err)
	}

	g.downloadPath = out.Name()
	g.extract(prefix)
}

func (g *GoToolchain) extract(prefix string) {
	r, err := os.Open(g.downloadPath)
	if err != nil {
		log.Fatalf("extract: %s", err)
	}

	if runtime.GOOS != "windows" {
		g.extractTarGz(prefix, r)
		r.Close()
		os.Remove(g.downloadPath)
		g.downloadPath = ""
	} else {
		r.Close()
		log.Fatalf("windows not supported")
	}

	g.binLocation = filepath.Join(prefix, "go/bin")
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
			if err := os.Mkdir(name, 0755); err != nil {
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
