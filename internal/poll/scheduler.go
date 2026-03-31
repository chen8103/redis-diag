package poll

import (
	"context"
	"sync"
)

type Scheduler struct {
	mu    sync.Mutex
	state map[string]jobState
	wg    sync.WaitGroup
}

type jobState struct {
	running bool
	pending bool
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		state: make(map[string]jobState),
	}
}

func (s *Scheduler) Run(ctx context.Context, name string, fn func(ctx context.Context)) bool {
	s.mu.Lock()
	state := s.state[name]
	if state.running {
		state.pending = true
		s.state[name] = state
		s.mu.Unlock()
		return false
	}
	s.state[name] = jobState{running: true}
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		for {
			fn(ctx)

			s.mu.Lock()
			state = s.state[name]
			if state.pending {
				state.pending = false
				s.state[name] = state
				s.mu.Unlock()
				continue
			}
			delete(s.state, name)
			s.mu.Unlock()
			return
		}
	}()

	return true
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	for name := range s.state {
		delete(s.state, name)
	}
	s.mu.Unlock()
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}
