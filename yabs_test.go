package yabs

import (
	"testing"

	"github.com/jakegut/yabs/task"
	"golang.org/x/exp/slices"
)

type getRecordsTest struct {
	name  string
	input func(*Yabs)
	final []TaskRecord
}

const hiChecksum = "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4"

func TestGetTaskRecords(t *testing.T) {
	tests := []getRecordsTest{
		{
			name: "no-op target returns empty TaskRecord",
			input: func(y *Yabs) {
				y.Register("default", []string{}, func(bc task.BuildCtx) {})
			},
			final: []TaskRecord{},
		},
		{
			name: "one target produces out, no task dep",
			input: func(y *Yabs) {
				y.Register("default", []string{}, func(bc task.BuildCtx) {
					if err := bc.Run("echo", "hi").StdoutToFile(bc.Out).Exec(); err != nil {
						t.Fatal(err)
					}
				})
			},
			final: []TaskRecord{{Name: "default", Checksum: hiChecksum}},
		},
		{
			name: "two targets",
			input: func(y *Yabs) {
				y.Register("echo", []string{}, func(bc task.BuildCtx) {
					if err := bc.Run("echo", "hi").StdoutToFile(bc.Out).Exec(); err != nil {
						t.Fatal(err)
					}
				})
				y.Register("default", []string{"echo"}, func(bc task.BuildCtx) {})
			},
			final: []TaskRecord{{Name: "default", Deps: []string{"echo"}}, {Name: "echo", Checksum: hiChecksum}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y := New()

			tt.input(y)

			y.scheduler.Start()
			if task, ok := y.taskKV["default"]; ok {
				<-y.scheduler.Schedule(task)
			} else {
				t.Fatalf("%q task not found", "default")
			}

			if !compareTaskRecSlice(tt.final, y.getTaskRecords()) {
				t.Fatalf("got two different []TaskRecord\n\t*got =%+v\n\t*want=%+v", tt.final, y.getTaskRecords())
			}
		})
	}
}

func compareTaskRecSlice(a, b []TaskRecord) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		recA := a[i]
		recB := b[i]

		if recA.Name != recB.Name {
			return false
		}

		if recA.Checksum != recB.Checksum {
			return false
		}

		if slices.Compare(recA.Deps, recB.Deps) != 0 {
			return false
		}
	}

	return true
}
