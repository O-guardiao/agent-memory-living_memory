package bootstrap

// Add coordinated worker shutdown hooks here when queues and background jobs are enabled.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
)

// Hook is a named stop function executed during coordinated shutdown.
type Hook struct {
	Name string
	Stop func(context.Context) error
}

// Shutdown runs registered hooks in reverse registration order so that
// consumers stop before the resources they depend on.
type Shutdown struct {
	mu    sync.Mutex
	hooks []Hook
}

func NewShutdown() *Shutdown {
	return &Shutdown{}
}

func (s *Shutdown) Register(name string, stop func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = append(s.hooks, Hook{Name: name, Stop: stop})
}

func (s *Shutdown) Run(ctx context.Context) error {
	s.mu.Lock()
	hooks := make([]Hook, len(s.hooks))
	copy(hooks, s.hooks)
	s.hooks = nil
	s.mu.Unlock()

	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		hook := hooks[i]
		if err := hook.Stop(ctx); err != nil {
			log.Printf("shutdown hook %s failed: %v", hook.Name, err)
			errs = append(errs, fmt.Errorf("%s: %w", hook.Name, err))
		}
	}
	return errors.Join(errs...)
}
