package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jakegut/yabs"
)

type ToolchainProvider struct {
	// Type of Toolchain, e.g. `go`, `node`
	Type string
	// Version of toolchain to download
	Version string
	// Location of directory where bin lives, relative to archive
	BinLoc []string
	// Function to get the download URL of the toolchain, based on the provider
	DownloadURL DownloadURLFunc
}

type DownloadURLFunc func(ToolchainProvider) string

func (tp ToolchainProvider) getPrefix() string {
	return fmt.Sprintf(".yabs/%s/%s", tp.Type, tp.Version)
}

func (tp ToolchainProvider) GetTargetName() string {
	return fmt.Sprintf("%s@%s", tp.Type, tp.Version)
}

func (tp ToolchainProvider) Register(y *yabs.Yabs) {
	name := tp.GetTargetName()
	y.Register(name, []string{}, func(bc yabs.BuildCtx) {
		tp.Download()

		if err := os.Mkdir(bc.Out, os.ModePerm); err != nil {
			log.Fatal(err)
		}

		binLoc := filepath.Join(append([]string{tp.getPrefix()}, tp.BinLoc...)...)

		if err := filepath.WalkDir(binLoc, func(path string, d fs.DirEntry, err error) error {
			rel, err := filepath.Rel(binLoc, path)
			if err != nil {
				return fmt.Errorf("rel: %s", err)
			}
			loc := filepath.Join(bc.Out, rel)
			// make the resulting link in the out folder link to the original link
			if d.Type()&fs.ModeSymlink != 0 {
				lk, err := filepath.EvalSymlinks(path)
				if err != nil {
					return err
				}

				absLk, err := filepath.Abs(lk)
				if err != nil {
					log.Fatal(err)
				}
				relLk, err := filepath.Rel(filepath.Dir(loc), absLk)
				if err != nil {
					log.Fatal(err)
				}

				if err = os.Symlink(relLk, loc); err != nil {
					return err
				}
			} else if !d.IsDir() {
				if err = os.Link(path, loc); err != nil {
					return err
				}
			} else {
				os.MkdirAll(loc, os.ModePerm)
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
	})
}

func (tp ToolchainProvider) Download() error {
	downloadUrl := tp.DownloadURL(tp)

	prefix := tp.getPrefix()

	if _, err := os.Stat(prefix); os.IsNotExist(err) {
		if err = os.MkdirAll(prefix, os.ModePerm); err != nil {
			log.Fatalf("downloading: %s", err)
		}
	} else {
		log.Printf("already have %s@%s", tp.Type, tp.Version)
		return nil
	}

	log.Printf("downloading %s@%s from %s", tp.Type, tp.Version, downloadUrl)

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
		if err := tp.extractZip(prefix, reader, size); err != nil {
			log.Fatal(err)
		}

	} else {
		log.Printf("extracting tar.gz")
		if err := tp.extractTarGz(prefix, resp.Body); err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func (tp ToolchainProvider) extractTarGz(prefix string, r io.Reader) error {
	tarStream, err := gzip.NewReader(r)
	if err != nil {
		log.Fatalf("tar gz: gzip: %s", err)
	}

	tarReader := tar.NewReader(tarStream)

	symlinks := map[string]string{}

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
		case tar.TypeSymlink:
			// fullLink := filepath.Join(prefix, filepath.Dir(header.Name), header.Linkname)
			// symlinks[fullLink] = name
			if err := os.Symlink(header.Linkname, name); err != nil {
				log.Fatalf("extracttargz: %s", err)
			}
		default:
			log.Fatalf(
				"ExtractTarGz: unknown type: %c in %s",
				header.Typeflag,
				header.Name)
		}
	}

	for fullLink, name := range symlinks {
		if err := os.Symlink(fullLink, name); err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func (tp ToolchainProvider) extractZip(prefix string, r io.ReaderAt, size int64) error {
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
