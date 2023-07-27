package yabs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"strings"
	"sync"

	"golang.org/x/exp/slices"
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
	Name     string
	Checksum string
}

type Task struct {
	Name     string
	Fn       BuildCtxFunc
	Dep      []string
	Out      string
	Checksum string
	Dirty    bool
	mu       sync.Mutex
	chQueue  []chan *Task
}

type OutType int

const (
	None OutType = iota
	File
	Dir
)

func getCacheLoc(checksum string) string {
	return filepath.Join(tmpDir, "cache", checksum[:2], checksum[2:])
}

func (t *Task) cache(outType OutType) {
	loc := getCacheLoc(t.Checksum)
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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("stat tmp out: %s", err)
	} else if fd != nil {
		if fd.IsDir() {
			f, err := os.Open(t.Out)
			if err != nil {
				log.Fatalf("open dir: %s", err)
			}
			defer f.Close()

			_, err = f.Readdirnames(1)
			if err == io.EOF {
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
		checksum = checksumFile(t.Out)
	case Dir:
		checksum = checksumDir(t.Out)
	case None:
		t.Out = ""
		return
	}

	if checksum == t.Checksum {
		t.Dirty = false
		return
	} else {
		t.Checksum = checksum
	}

	t.cache(outType)
}

type BuildCtxFunc func(BuildCtx)

var taskKV map[string]*Task = map[string]*Task{}

var scheduler = NewScheduler()

var taskRecordLoc = tmpDir + "/.records.json"

func SaveTasks() {
	taskRecords := []TaskRecord{}
	for name, task := range taskKV {
		if task.Checksum == "" {
			continue
		}
		taskRecords = append(taskRecords, TaskRecord{Checksum: task.Checksum, Name: name})
	}

	slices.SortFunc(taskRecords, func(a, b TaskRecord) int {
		return strings.Compare(a.Name, b.Name)
	})

	if len(taskRecords) == 0 {
		return
	}

	bs, err := json.MarshalIndent(taskRecords, "", "	")
	if err != nil {
		log.Fatalf("marshing records: %s", err)
	}

	fd, err := os.OpenFile(taskRecordLoc, os.O_CREATE|os.O_RDWR, 0775)
	if err != nil {
		log.Fatalf("opening file: %s", err)
	}
	defer fd.Close()

	if _, err = fd.Write(bs); err != nil {
		log.Fatalf("writing to file: %s", err)
	}
}

func RestoreTasks() {
	fd, err := os.Open(taskRecordLoc)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Fatalf("opening file: %s", err)
	}
	defer fd.Close()

	bs, err := io.ReadAll(fd)
	if err != nil {
		log.Fatalf("reading file: %s", err)
	}
	var taskRecords = []TaskRecord{}
	if err = json.Unmarshal(bs, &taskRecords); err != nil {
		log.Fatalf("unmarshing: %s", err)
	}

	for _, rec := range taskRecords {
		task, ok := taskKV[rec.Name]
		if !ok {
			continue
		}
		task.Checksum = rec.Checksum
		task.Out = getCacheLoc(task.Checksum)

		if _, err := os.Lstat(task.Out); os.IsNotExist(err) {
			task.Checksum = ""
			task.Out = ""
		}
	}
}

func Register(name string, deps []string, fn BuildCtxFunc) {
	task := &Task{Dep: deps, Fn: fn, Name: name, Dirty: true, mu: sync.Mutex{}}
	taskKV[name] = task
}

func ExecWithDefault(def string) error {
	RestoreTasks()
	scheduler.Start()
	if task, ok := taskKV[def]; ok {
		<-scheduler.Schedule(task)
	} else {
		return fmt.Errorf("%q task not found", def)
	}
	SaveTasks()
	return nil
}
