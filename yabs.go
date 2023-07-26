package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type RunConfig struct {
	cmd []string
	env map[string]string
	out string
}

func NewRunConfig(name string, args ...string) *RunConfig {
	return &RunConfig{
		cmd: append([]string{name}, args...),
		env: map[string]string{},
		out: "",
	}
}

func (r *RunConfig) WithEnv(key, value string) *RunConfig {
	r.env[key] = value
	return r
}

func (r *RunConfig) StdoutToFile(file string) *RunConfig {
	r.out = file
	return r
}

func (r *RunConfig) Exec() error {
	fd := os.Stdout
	if len(r.out) > 0 {
		var err error
		fd, err = os.Create(r.out)
		if err != nil {
			return fmt.Errorf("opening file: %s", err)
		}
		defer fd.Close()
	}

	cmd := exec.Command(r.cmd[0], r.cmd[1:]...)
	cmd.Stdout = fd

	env := os.Environ()
	for k, v := range r.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cmd start: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("cmd wait: %s", err)
	}
	return nil
}

var tmpDir = func() string {
	path := ".yabs"
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			log.Fatalf("creating tmp dir: %s", err)
		}
	}
	return ".yabs"
}()

func getTmp(prefix string) (string, error) {
	try := 0
	var err error
	for try < 10000 {
		rand := strconv.Itoa(int(rand.Uint32()))

		path := filepath.Join(tmpDir, "out", prefix+rand)

		_, err = os.Stat(path)

		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		try++
	}

	return "", err
}

func newTmpOut() (string, error) {
	return getTmp("yabs-out-")
}

func getFileChecksum(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %s", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("io copy: %s", err)
	}

	return h.Sum(nil), nil
}

func checksumFile(loc string) string {
	sum, err := getFileChecksum(loc)

	if err != nil {
		log.Fatalf("checksum file: %s", err)
	}

	return hex.EncodeToString(sum)
}

// func isDir(loc string) bool {
// 	f, err := os.Open(loc)
// 	if errors.Is(err, os.ErrNotExist) {
// 		return true
// 	}
// 	if err != nil {
// 		log.Fatalf("open dir: %s", err)
// 	}
// 	defer f.Close()

// 	_, err = f.Readdirnames(1)
// 	return err == io.EOF
// }

func checksumDir(loc string) string {

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
		log.Fatalf("file walk: %s", err)
	}

	return hex.EncodeToString(hsh.Sum(nil))
}

type BuildCtx struct {
	Run func(name string, args ...string) *RunConfig
	Out string
	Dep map[string]string
}

func NewBuildCtx() BuildCtx {
	tmpOut, err := newTmpOut()
	if err != nil {
		log.Fatal(err)
	}
	return BuildCtx{
		Run: NewRunConfig,
		Out: tmpOut,
		Dep: map[string]string{},
	}
}

type TaskRecord struct {
	Checksum string
	Time     uint64
}

type Task struct {
	Name     string
	Fn       BuildCtxFunc
	Dep      []string
	Out      string
	Checksum string
}

type OutType int

const (
	None OutType = iota
	File
	Dir
)

func (t *Task) cache(outType OutType) {
	loc := filepath.Join(tmpDir, "cache", t.Checksum[:2], t.Checksum[2:])
	if err := os.MkdirAll(filepath.Dir(loc), 0770); err != nil {
		if !errors.Is(err, os.ErrExist) {
			log.Fatalf("creating parent dir: %s", err)
		}
	}

	_, err := os.Lstat(loc)
	if err == nil {
		t.Out = loc
		return
	}

	switch outType {
	case File:
		if err = os.Link(t.Out, loc); err != nil {
			log.Fatalf("creating link: %s\n", err)
		}
	case Dir:
		if err = os.Symlink(t.Out, loc); err != nil {
			log.Fatalf("creating link: %s\n", err)
		}
	}

	t.Out = loc
}

func (t *Task) checksumEntries(ctx BuildCtx) {
	outType := None
	fd, err := os.Stat(t.Out)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		log.Fatalf("stat tmp out: %s", err)
	}

	if fd.IsDir() {
		f, err := os.Open(t.Out)
		if err != nil {
			log.Fatalf("open dir: %s", err)
		}
		defer f.Close()

		_, err = f.Readdirnames(1)
		if err == io.EOF {
			return
		}
		outType = Dir
	} else {
		if fd.Size() == 0 {
			return
		}
		outType = File
	}

	switch outType {
	case File:
		t.Checksum = checksumFile(t.Out)
	case Dir:
		t.Checksum = checksumDir(t.Out)
	case None:
		return
	}

	t.cache(outType)
}

func (t *Task) execConcurrent() <-chan *Task {
	ch := make(chan *Task, 1)

	go func() {
		ctx := NewBuildCtx()
		tasks := []<-chan *Task{}
		for _, dep := range t.Dep {
			if task, ok := taskKV[dep]; ok {
				tasks = append(tasks, task.execConcurrent())
			}
		}
		for _, task := range tasks {
			t := <-task
			ctx.Dep[t.Name] = t.Out
		}

		log.Printf("running %q", t.Name)
		t.Fn(ctx)
		t.Out = ctx.Out
		t.checksumEntries(ctx)
		fmt.Println(t.Checksum)

		ch <- t
	}()

	return ch
}

type BuildCtxFunc func(BuildCtx)

var taskKV map[string]*Task = map[string]*Task{}

func Register(name string, deps []string, fn BuildCtxFunc) {
	task := &Task{Dep: deps, Fn: fn, Name: name}
	taskKV[name] = task
}

func ExecWithDefault(def string) error {
	if task, ok := taskKV[def]; ok {
		<-task.execConcurrent()
	} else {
		return fmt.Errorf("%q task not found", def)
	}
	return nil
}
