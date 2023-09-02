package internal

import (
	"io"
	"os"
)

func IsEmptyDir(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	return err == io.EOF
}
