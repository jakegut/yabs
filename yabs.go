package yabs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jakegut/yabs/internal"
	"github.com/jakegut/yabs/task"
	"golang.org/x/exp/slices"
)

func getTmp(loc, prefix string) (string, error) {
	try := 0
	var err error
	for try < 10000 {
		rand := strconv.Itoa(int(rand.Uint32()))

		path := filepath.Join(loc, "out", prefix+rand)

		_, err = os.Stat(path)

		if errors.Is(err, os.ErrNotExist) {
			abs, _ := filepath.Abs(path)
			return abs, nil
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
	if err := os.MkdirAll(filepath.Dir(tmp), os.ModePerm); err != nil && !os.IsExist(err) {
		log.Fatalf("tmpout: %s", err)
	}
	return tmp, nil
}

type TaskRecord struct {
	Name     string
	Checksum string
	Deps     []string
	Time     int64
}

func (y *Yabs) getCacheLoc(checksum string) string {
	return filepath.Join(y.tmpDir, "cache", checksum[:2], checksum[2:])
}

type Yabs struct {
	scheduler     *Scheduler
	taskKV        map[string]*task.Task
	taskRecordLoc string
	tmpDir        string
	time          int64
}

func (y *Yabs) getTaskRecords() []TaskRecord {
	taskRecords := []TaskRecord{}
	for name, task := range y.taskKV {
		if task.Checksum == "" && len(task.Dep) == 0 {
			continue
		}

		slices.Sort(task.Dep)
		if y.scheduler.taskDone[task.Name] && task.Dirty {
			task.Time = y.time
		}
		taskRecords = append(taskRecords, TaskRecord{Checksum: task.Checksum, Name: name, Deps: task.Dep, Time: task.Time})
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
		taskKV:        map[string]*task.Task{},
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
				path, err := os.Readlink(loc)
				if err != nil {
					log.Fatalf("restoring tasks: %s", err)
				}
				task.Out = path
			}
		}

		task.Time = rec.Time
		if task.Time > y.time {
			y.time = task.Time
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
		if filepath.IsAbs(path) {
			wd, _ := os.Getwd()
			path, _ = filepath.Rel(wd, path)
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
		if internal.IsEmptyDir(filepath.Dir(path)) {
			if err := os.Remove(dir); err != nil {
				log.Fatalf("removing parent: %s", err)
			}
		}
	}
}

func (y *Yabs) Register(name string, deps []string, fn task.BuildCtxFunc) {
	if _, ok := y.taskKV[name]; ok {
		return
	}

	slices.Sort(deps)
	y.taskKV[name] = &task.Task{Dep: deps, Fn: fn, Name: name, GetCacheLoc: y.getCacheLoc}
}

func (y *Yabs) ExecWithDefault(def string) error {
	y.RestoreTasks()
	y.time = y.time + 1
	y.scheduler.Start()
	if task, ok := y.taskKV[def]; ok {
		<-y.scheduler.Schedule(task)
	} else {
		return fmt.Errorf("%q task not found", def)
	}
	y.SaveTasks()
	return nil
}

func (y *Yabs) GetTaskNames() []string {
	names := []string{}
	for name := range y.taskKV {
		names = append(names, name)
	}
	return names
}
