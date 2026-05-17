// cSpell: words sirupsen
package utils

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type hook struct {
	fn   func() error
	name string
}

type hookError struct {
	err  error
	name string
}

type HookManager struct {
	LogEnabled
	hooks []hook
	mu    sync.Mutex
}

func NewHookManager(log *slog.Logger) *HookManager {
	return &HookManager{
		LogEnabled: LogEnabled{LogEntry: log},
	}
}

func (m *HookManager) Register(name string, fn func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook{name: name, fn: fn})
}

func (m *HookManager) Run() error {
	m.mu.Lock()
	hooks := append([]hook{}, m.hooks...)
	m.mu.Unlock()

	errs := make(chan hookError, len(hooks))
	var wg sync.WaitGroup
	wg.Add(len(hooks))
	for _, h := range hooks {
		go func(h hook) {
			defer wg.Done()
			if err := h.fn(); err != nil {
				errs <- hookError{name: h.name, err: err}
				m.Logger().Error("Error while running hook", "hook", h.name, ErrorKey, err)
			}
		}(h)
	}
	wg.Wait()
	close(errs)

	var all []error
	for err := range errs {
		all = append(all, fmt.Errorf("hook %s: %w", err.name, err.err))
	}
	return errors.Join(all...)
}
