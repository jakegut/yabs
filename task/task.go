package task

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jakegut/yabs/internal"
)

type BuildCtx struct {
	// Run func(name string, args ...string) *RunConfig
	Out string
	Dep map[string]string
}

func NewBuildCtx(out string) BuildCtx {
	return BuildCtx{
		Out: out,
		Dep: map[string]string{},
	}
}

func (BuildCtx) Run(name string, args ...string) *RunConfig {
	return &RunConfig{
		Cmd: append([]string{name}, args...),
		env: map[string]string{},
		out: "",
	}
}

func (bc *BuildCtx) GetDep(name string) string {
	return bc.Dep[name]
}

type OutType int

const (
	None OutType = iota
	File
	Dir
)

type BuildCtxFunc func(BuildCtx)

type CacheLocFunc func(string) string

type Task struct {
	Name        string
	Fn          BuildCtxFunc
	Dep         []string
	Out         string
	Checksum    string
	Dirty       bool
	Time        int64
	GetCacheLoc CacheLocFunc
}

func (t *Task) cache() error {
	loc := t.GetCacheLoc(t.Checksum)
	if err := os.MkdirAll(filepath.Dir(loc), os.ModePerm); err != nil && !os.IsExist(err) {
		return fmt.Errorf("task cache: %s", err)
	}

	_, err := os.Lstat(loc)
	if err == nil {
		return nil
	}

	if err = os.Symlink(t.Out, loc); err != nil {
		return fmt.Errorf("task cache: %s", err)
	}
	return nil
}

func (t *Task) ChecksumEntries() error {
	outType := None
	var err error
	fd, err := os.Stat(t.Out)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat tmp out: %s", err)
	} else if fd != nil {
		if fd.IsDir() {
			if internal.IsEmptyDir(t.Out) {
				outType = None
			} else {
				outType = Dir
			}
		} else {
			if fd.Size() == 0 {
				outType = None
			} else {
				outType = File
			}
		}
	}

	checksum := ""

	switch outType {
	case File:
		checksum, err = checksumFile(t.Out)
		if err != nil {
			return err
		}
	case Dir:
		checksum, err = checksumDir(t.Out)
		if err != nil {
			return err
		}
	case None:
		t.Out = ""
		return nil
	}

	if checksum == t.Checksum {
		t.Dirty = false
		err := removeDir(t.Out)
		if err != nil {
			return err
		}
		out := t.GetCacheLoc(checksum)
		lk, _ := os.Readlink(out)
		t.Out = lk
		return nil
	} else {
		t.Checksum = checksum
	}

	return t.cache()
}

func removeDir(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove dir: %s", err)
	}
	return nil
}

func getFileChecksum(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("file checksum: %s", err)

	}
	if st.Mode()&fs.ModeSymlink != 0 {
		lk, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("file checksum: %s", err)
		}
		path = lk
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("file checksum: %s", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("file checksum: %s", err)
	}

	return h.Sum(nil), nil
}

func checksumFile(loc string) (string, error) {
	sum, err := getFileChecksum(loc)

	if err != nil {
		return "", fmt.Errorf("checksum file: %s", err)
	}

	return hex.EncodeToString(sum), nil
}

func checksumDir(loc string) (string, error) {

	hsh := sha256.New()

	err := filepath.Walk(loc, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		sum, err := getFileChecksum(path)
		if err != nil {
			return err
		}

		_, err = hsh.Write(sum)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("checksum dir: %s", err)
	}

	return hex.EncodeToString(hsh.Sum(nil)), nil
}
