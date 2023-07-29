package yabs

import (
	"fmt"
	"log"
	"sync"
)

const POOL_SIZE = 10

type Scheduler struct {
	taskKV   map[string][]chan *Task
	taskDone map[string]bool
	mu       *sync.Mutex
	taskCh   chan *Task
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		taskKV:   make(map[string][]chan *Task),
		taskDone: make(map[string]bool),
		mu:       &sync.Mutex{},
		taskCh:   make(chan *Task),
	}
}

func (s *Scheduler) execTask(t *Task) {
	ctx := NewBuildCtx()
	tasks := []<-chan *Task{}
	for _, dep := range t.Dep {
		if task, ok := taskKV[dep]; ok {
			tasks = append(tasks, s.Schedule(task))
		} else {
			fmt.Println("dep not found", dep)
		}
	}

	dirty := len(tasks) == 0 || t.Dirty
	for _, task := range tasks {
		tmpTask := <-task
		ctx.Dep[tmpTask.Name] = tmpTask.Out
		dirty = dirty || tmpTask.Dirty
	}

	t.Dirty = dirty
	if dirty {
		log.Printf("running %q", t.Name)
		t.Fn(ctx)
		t.Out = ctx.Out
		t.checksumEntries(ctx)
	} else {
		log.Printf("no actions for %q", t.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.taskKV[t.Name] {
		ch <- t
	}
	s.taskDone[t.Name] = true
}

func (s *Scheduler) taskWorker() {
	for task := range s.taskCh {
		s.execTask(task)
	}
}

func (s *Scheduler) Schedule(t *Task) chan *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan *Task, 1)
	if s.taskDone[t.Name] {
		ch <- t
		return ch
	}

	if _, ok := s.taskKV[t.Name]; ok {
		s.taskKV[t.Name] = append(s.taskKV[t.Name], ch)
	} else {
		s.taskKV[t.Name] = make([]chan *Task, 1)
		s.taskKV[t.Name][0] = ch
		s.taskCh <- t
	}

	return ch
}

func (s *Scheduler) Start() {
	for i := 0; i < POOL_SIZE; i++ {
		go s.taskWorker()
	}
}
