package yabs

import (
	"context"
	"fmt"
	"log"
	"sync"

	"golang.org/x/sync/semaphore"
)

const POOL_SIZE = 5

type Scheduler struct {
	taskQueue map[string][]chan *Task
	taskDone  map[string]bool
	mu        *sync.Mutex
	y         *Yabs
	sema      *semaphore.Weighted
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		taskQueue: make(map[string][]chan *Task),
		taskDone:  make(map[string]bool),
		mu:        &sync.Mutex{},
	}
}

func (s *Scheduler) execTask(t *Task) {
	out, err := s.y.newTmpOut()
	if err != nil {
		log.Fatalf("creating tmp out: %s", err)
	}
	ctx := NewBuildCtx(out)
	tasks := []<-chan *Task{}
	for _, dep := range t.Dep {
		if task, ok := s.y.taskKV[dep]; ok {
			tasks = append(tasks, s.Schedule(task))
		} else {
			fmt.Println("dep not found", dep)
		}
	}

	dirty := len(tasks) == 0 || t.Dirty
	maxTime := t.Time
	for _, task := range tasks {
		tmpTask := <-task
		ctx.Dep[tmpTask.Name] = tmpTask.Out
		dirty = dirty || tmpTask.Dirty
		if tmpTask.Time > maxTime {
			maxTime = tmpTask.Time
		}
	}
	dirty = dirty || maxTime > t.Time

	t.Dirty = dirty
	if dirty {
		s.sema.Acquire(context.Background(), 1)
		log.Printf("running %q", t.Name)
		t.Fn(ctx)
		s.sema.Release(1)
		t.Out = ctx.Out
		t.checksumEntries(s.y, ctx)
	} else {
		log.Printf("no actions for %q", t.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.taskQueue[t.Name] {
		ch <- t
	}
	s.taskDone[t.Name] = true
}

func (s *Scheduler) Schedule(t *Task) chan *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan *Task, 1)
	if s.taskDone[t.Name] {
		ch <- t
		return ch
	}

	if _, ok := s.taskQueue[t.Name]; ok {
		s.taskQueue[t.Name] = append(s.taskQueue[t.Name], ch)
	} else {
		s.taskQueue[t.Name] = make([]chan *Task, 1)
		s.taskQueue[t.Name][0] = ch
		go s.execTask(t)
	}

	return ch
}

func (s *Scheduler) Start() {
	s.sema = semaphore.NewWeighted(POOL_SIZE)
}
