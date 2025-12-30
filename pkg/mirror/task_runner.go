package mirror

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type Task interface {
	Execute() error
}

type TaskFunc func() error

func (tf TaskFunc) Execute() error {
	return tf()
}

type TaskRunner struct {
	tasks  chan Task
	wg     sync.WaitGroup
	closed chan struct{}
}

func NewTaskRunner() *TaskRunner {
	const bufferSize = 1024
	return &TaskRunner{
		tasks:  make(chan Task, bufferSize),
		closed: make(chan struct{}),
	}
}

func (s *TaskRunner) Start(workerCount int) {
	s.wg.Add(workerCount)

	for range workerCount {
		go func() {
			for task := range s.tasks {
				err := task.Execute()
				if err != nil {
					log.Error().Err(err).Msg("Task execution failed")
				}
			}
			s.wg.Done()
		}()
	}
}

func (s *TaskRunner) Schedule(task Task) error {
	select {
	case <-s.closed:
		return nil
	case s.tasks <- task:
	}
	return nil
}

func (s *TaskRunner) Stop() {
	close(s.closed)
	close(s.tasks)
	s.wg.Wait()
}
