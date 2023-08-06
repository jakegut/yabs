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
	cmd.Stderr = os.Stderr

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

func getTmp(loc, prefix string) (string, error) {
	try := 0
	var err error
	for try < 10000 {
		rand := strconv.Itoa(int(rand.Uint32()))

		path := filepath.Join(loc, "out", prefix+rand)

		_, err = os.Stat(path)

		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		try++
	}

	return "", err
}

func (y *Yabs) newTmpOut() (string, error) {
	tmp, err := getTmp(y.tmpDir, "yabs-out-")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(tmp), 0700); err != nil && !os.IsExist(err) {
		log.Fatalf("tmpout: %s", err)
	}
	return tmp, nil
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

func NewBuildCtx(out string) BuildCtx {
	return BuildCtx{
		Run: NewRunConfig,
		Out: out,
		Dep: map[string]string{},
	}
}

type TaskRecord struct {
	Name     string
	Checksum string
	Deps     []string
}

type Task struct {
	Name     string
	Fn       BuildCtxFunc
	Dep      []string
	Out      string
	Checksum string
	Dirty    bool
}

type OutType int

const (
	None OutType = iota
	File
	Dir
)

func (y *Yabs) getCacheLoc(checksum string) string {
	return filepath.Join(y.tmpDir, "cache", checksum[:2], checksum[2:])
}

func removeDir(path string) {
	if !strings.HasPrefix(path, ".yabs/out") {
		log.Fatalf("about to remove a non-out dir: %q", path)
	}

	if err := os.RemoveAll(path); err != nil {
		log.Fatalf("remove dir: %s", err)
	}
}

func (t *Task) cache(y *Yabs, outType OutType) {
	loc := y.getCacheLoc(t.Checksum)
	if err := os.MkdirAll(filepath.Dir(loc), 0770); err != nil && !os.IsExist(err) {
		log.Fatalf("creating parent dir: %s", err)
	}

	_, err := os.Lstat(loc)
	if err == nil {
		t.Out = loc
		return
	}

	if err = os.Symlink(t.Out, loc); err != nil {
		log.Fatalf("creating link: %s", err)
	}

	t.Out = loc
}

func isEmptyDir(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("isEmptyDir: %s", err)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	return err == io.EOF
}

func (t *Task) checksumEntries(y *Yabs, ctx BuildCtx) {
	outType := None
	fd, err := os.Stat(t.Out)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("stat tmp out: %s", err)
	} else if fd != nil {
		if fd.IsDir() {
			if isEmptyDir(t.Out) {
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
		removeDir(t.Out)
		t.Out = y.getCacheLoc(t.Checksum)
		return
	} else {
		t.Checksum = checksum
	}

	t.cache(y, outType)
}

type BuildCtxFunc func(BuildCtx)

type Yabs struct {
	scheduler     *Scheduler
	taskKV        map[string]*Task
	taskRecordLoc string
	tmpDir        string
}

func (y *Yabs) getTaskRecords() []TaskRecord {
	taskRecords := []TaskRecord{}
	for name, task := range y.taskKV {
		if task.Checksum == "" && len(task.Dep) == 0 {
			continue
		}
		slices.Sort(task.Dep)
		taskRecords = append(taskRecords, TaskRecord{Checksum: task.Checksum, Name: name, Deps: task.Dep})
	}

	slices.SortFunc(taskRecords, func(a, b TaskRecord) int {
		return strings.Compare(a.Name, b.Name)
	})

	return taskRecords
}

func New() *Yabs {
	tmpDir := func() string {
		path := ".yabs"
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				log.Fatalf("creating tmp dir: %s", err)
			}
		}
		return ".yabs"
	}()
	y := &Yabs{
		scheduler:     NewScheduler(),
		taskKV:        map[string]*Task{},
		taskRecordLoc: filepath.Join(tmpDir, ".records.json"),
		tmpDir:        tmpDir,
	}
	// TODO: resolve this circular reference (seperate taskKV/TaskStore struct?)
	y.scheduler.y = y
	return y
}

func (y *Yabs) SaveTasks() {
	taskRecords := y.getTaskRecords()

	bs, err := json.MarshalIndent(taskRecords, "", "	")
	if err != nil {
		log.Fatalf("marshing records: %s", err)
	}

	fd, err := os.Create(y.taskRecordLoc)
	if err != nil {
		log.Fatalf("opening file: %s", err)
	}
	defer fd.Close()

	if _, err = fd.Write(bs); err != nil {
		log.Fatalf("writing to file: %s", err)
	}
}

func (y *Yabs) RestoreTasks() {
	fd, err := os.Open(y.taskRecordLoc)
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
		task, ok := y.taskKV[rec.Name]
		if !ok {
			continue
		}
		if len(rec.Checksum) > 0 {
			loc := y.getCacheLoc(rec.Checksum)
			if _, err := os.Lstat(loc); err == nil {
				task.Checksum = rec.Checksum
				task.Out = loc
			}
		}

		task.Dirty = len(task.Dep) != len(rec.Deps)
		if !task.Dirty {
			for i := range rec.Deps {
				if task.Dep[i] != rec.Deps[i] {
					task.Dirty = true
					break
				}
			}
		}

	}
}

func (y *Yabs) Prune() {
	validOuts := map[string]bool{}
	for _, t := range y.taskKV {
		if len(t.Checksum) == 0 {
			continue
		}
		cacheLoc := y.getCacheLoc(t.Checksum)
		validOuts[cacheLoc] = true
		path, err := os.Readlink(cacheLoc)
		if err != nil {
			log.Fatalf("prune: %s", err)
		}
		validOuts[path] = true
	}

	toDelete := []string{}
	if err := filepath.WalkDir(".yabs/out", func(path string, d fs.DirEntry, err error) error {
		if path == ".yabs/out" {
			return nil
		}
		if !validOuts[path] {
			toDelete = append(toDelete, path)
		}
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}); err != nil {
		log.Fatalf("prune: %s", err)
	}

	if err := filepath.WalkDir(".yabs/cache", func(path string, d fs.DirEntry, err error) error {
		if path == ".yabs/cache" || d.IsDir() {
			return nil
		}
		if !validOuts[path] {
			toDelete = append(toDelete, path)
		}
		return nil
	}); err != nil {
		log.Fatalf("prune: %s", err)
	}

	for _, path := range toDelete {
		if err := os.RemoveAll(path); err != nil {
			log.Fatalf("prune: %s", err)
		}
		dir := filepath.Dir(path)
		if isEmptyDir(filepath.Dir(path)) {
			if err := os.Remove(dir); err != nil {
				log.Fatalf("removing parent: %s", err)
			}
		}
	}
}

func (y *Yabs) Register(name string, deps []string, fn BuildCtxFunc) {
	slices.Sort(deps)
	task := &Task{Dep: deps, Fn: fn, Name: name}
	y.taskKV[name] = task
}

func (y *Yabs) ExecWithDefault(def string) error {
	y.RestoreTasks()
	y.scheduler.Start()
	if task, ok := y.taskKV[def]; ok {
		<-y.scheduler.Schedule(task)
	} else {
		return fmt.Errorf("%q task not found", def)
	}
	y.SaveTasks()
	return nil
}
