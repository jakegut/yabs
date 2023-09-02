package task

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TaskTestSuite struct {
	suite.Suite
	TmpRootDir  string
	GetCacheLoc CacheLocFunc
}

var _ suite.BeforeTest = new(TaskTestSuite)
var _ suite.AfterTest = new(TaskTestSuite)

func (ts *TaskTestSuite) BeforeTest(suiteName, testName string) {
	ts.TmpRootDir, _ = os.MkdirTemp("", "yabs-task-test-")
	ts.GetCacheLoc = func(s string) string {
		return filepath.Join(ts.TmpRootDir, "cache", s[:2], s[2:])
	}
}

func (ts *TaskTestSuite) AfterTest(suiteName, testName string) {
	os.RemoveAll(ts.TmpRootDir)
	ts.TmpRootDir = ""
}

func (ts *TaskTestSuite) TestTaskCacheFile() {
	name := filepath.Join(ts.TmpRootDir, "out", "tmp", "hi.txt")
	ts.Nil(os.MkdirAll(filepath.Dir(name), os.ModePerm))

	ts.Nil(os.WriteFile(name, []byte("hi"), os.ModePerm))

	strChecksum, err := checksumFile(name)
	ts.Nil(err)

	task := &Task{Out: name, GetCacheLoc: ts.GetCacheLoc}
	ts.Nil(task.ChecksumEntries())

	ts.Equal(strChecksum, task.Checksum)

	stat, err := os.Lstat(ts.GetCacheLoc(strChecksum))
	ts.Nil(err)

	ts.True(stat.Mode()&os.ModeSymlink != 0)

	lnk, err := filepath.EvalSymlinks(ts.GetCacheLoc(strChecksum))
	ts.Nil(err)

	ts.Equal(name, lnk)
}

func (ts *TaskTestSuite) TestTaskCacheDir() {
	name := filepath.Join(ts.TmpRootDir, "out", "tmp", "hi.txt")
	dir := filepath.Dir(name)
	ts.Nil(os.MkdirAll(dir, os.ModePerm))

	ts.Nil(os.WriteFile(name, []byte("hi"), os.ModePerm))

	checksum, err := checksumDir(dir)
	ts.Nil(err)

	task := &Task{Out: dir, GetCacheLoc: ts.GetCacheLoc}
	ts.Nil(task.ChecksumEntries())

	ts.Equal(checksum, task.Checksum)

	stat, err := os.Lstat(ts.GetCacheLoc(checksum))
	ts.Nil(err)

	ts.True(stat.Mode()&os.ModeSymlink != 0)
	lnk, err := filepath.EvalSymlinks(ts.GetCacheLoc(checksum))
	ts.Nil(err)

	ts.Equal(dir, lnk)
}

func (ts *TaskTestSuite) TestTaskCacheNone() {
	name := filepath.Join(ts.TmpRootDir, "out", "tmp")
	// intentionally don't make dir to show as it doesn't exist

	task := &Task{Out: name, GetCacheLoc: ts.GetCacheLoc}
	ts.Nil(task.ChecksumEntries())

	ts.Equal("", task.Out)
}

func (ts *TaskTestSuite) TestTaskCacheEmptyFile() {
	name := filepath.Join(ts.TmpRootDir, "out", "tmp", "hi.txt")
	ts.Nil(os.MkdirAll(filepath.Dir(name), os.ModePerm))

	ts.Nil(os.WriteFile(name, []byte{}, os.ModePerm))

	task := &Task{Out: name, GetCacheLoc: ts.GetCacheLoc}
	ts.Nil(task.ChecksumEntries())

	ts.Equal("", task.Out)
}

func (ts *TaskTestSuite) TestTaskCacheEmptyDir() {
	name := filepath.Join(ts.TmpRootDir, "out", "tmp")
	ts.Nil(os.MkdirAll(name, os.ModePerm))

	task := &Task{Out: name, GetCacheLoc: ts.GetCacheLoc}
	ts.Nil(task.ChecksumEntries())

	ts.Equal("", task.Out)
}

func (ts *TaskTestSuite) TestGetFileChecksumEmptyDirectory() {
	_, err := getFileChecksum(ts.TmpRootDir)
	ts.Equal(err, fmt.Errorf("file checksum: read %s: is a directory", ts.TmpRootDir))
}

func (ts *TaskTestSuite) TestGetFileChecksumNonFile() {
	name := filepath.Join(ts.TmpRootDir, "random")
	_, err := getFileChecksum(name)
	ts.Equal(err, fmt.Errorf("file checksum: lstat %s: no such file or directory", name))
}

const hiChecksum = "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4"

func (ts *TaskTestSuite) TestGetFileChecksumFollowsSymlinks() {
	oldname := filepath.Join(ts.TmpRootDir, "hi.txt")
	ts.Nil(os.WriteFile(oldname, []byte("hi\n"), os.ModePerm))

	newname := filepath.Join(ts.TmpRootDir, "lk")
	ts.Nil(os.Symlink(oldname, newname))

	bs, err := getFileChecksum(newname)
	ts.Nil(err)

	ts.Equal(hiChecksum, hex.EncodeToString(bs))
}

func (ts *TaskTestSuite) TestGetFileChecksumInvalidSymlink() {
	oldname := filepath.Join(ts.TmpRootDir, "hi.txt")
	ts.Nil(os.WriteFile(oldname, []byte("hi\n"), os.ModePerm))

	newname := filepath.Join(ts.TmpRootDir, "lk")
	ts.Nil(os.Symlink(oldname, newname))

	ts.Nil(os.RemoveAll(oldname))

	_, err := getFileChecksum(newname)
	ts.Equal(err, fmt.Errorf("file checksum: lstat %s: no such file or directory", oldname))
}

func (ts *TaskTestSuite) TestTaskUsesChache() {
	oldname := filepath.Join(ts.TmpRootDir, "hi.txt")
	ts.Nil(os.WriteFile(oldname, []byte("hi\n"), os.ModePerm))

	newname := ts.GetCacheLoc(hiChecksum)
	ts.Nil(os.MkdirAll(filepath.Dir(newname), os.ModePerm))
	ts.Nil(os.Symlink(oldname, newname))

	diffname := filepath.Join(ts.TmpRootDir, "out", "different.txt")
	ts.Nil(os.MkdirAll(filepath.Dir(diffname), os.ModePerm))
	ts.Nil(os.WriteFile(diffname, []byte("hi\n"), os.ModePerm))

	tsk := &Task{Out: diffname, Checksum: hiChecksum, GetCacheLoc: ts.GetCacheLoc}

	ts.Nil(tsk.ChecksumEntries())

	ts.False(tsk.Dirty)
	ts.Equal(oldname, tsk.Out)
	ts.Equal(hiChecksum, tsk.Checksum)
}

func TestTaskTestSuite(t *testing.T) {
	suite.Run(t, new(TaskTestSuite))
}
