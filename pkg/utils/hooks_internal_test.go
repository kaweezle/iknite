// cSpell: words sirupsen paralleltest unwrapable testutil
package utils

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/testutil"
)

type namedHook struct {
	fn   func() error
	name string
}

func TestHookManager_Register_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hooks    []namedHook
		wantErrs []string // hook names that should error
	}{
		{
			name:     "no_hooks",
			hooks:    nil,
			wantErrs: nil,
		},
		{
			name: "success_hooks",
			hooks: []namedHook{
				{name: "good1", fn: func() error { return nil }},
				{name: "good2", fn: func() error { return nil }},
			},
			wantErrs: nil,
		},
		{
			name: "one_error",
			hooks: []namedHook{
				{name: "good", fn: func() error { return nil }},
				{name: "bad", fn: func() error { return errors.New("fail") }},
			},
			wantErrs: []string{"hook bad: fail"},
		},
		{
			name: "multiple_errors",
			hooks: []namedHook{
				{name: "good", fn: func() error { return nil }},
				{name: "err1", fn: func() error { return errors.New("one") }},
				{name: "err2", fn: func() error { return errors.New("two") }},
			},
			wantErrs: []string{"hook err1: one", "hook err2: two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger := testutil.TestLogger(t)
			m := NewHookManager(logger)

			for _, h := range tt.hooks {
				m.Register(h.name, h.fn)
			}

			err := m.Run()

			if len(tt.wantErrs) == 0 {
				require.NoError(t, err)
				return
			}

			unwrapable, ok := err.(interface{ Unwrap() []error })
			require.True(t, ok)
			errList := unwrapable.Unwrap()
			require.Len(t, errList, len(tt.wantErrs))
			var errStrings []string
			for _, e := range errList {
				errStrings = append(errStrings, e.Error())
			}
			require.ElementsMatch(t, tt.wantErrs, errStrings)
		})
	}
}

func TestHookManager_ConcurrentRegister(t *testing.T) {
	t.Parallel()
	logger := testutil.TestLogger(t)
	m := NewHookManager(logger)
	var wg sync.WaitGroup
	numHooks := 20
	for i := range numHooks {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Register(fmt.Sprintf("hook-%d", i), func() error { return nil })
		}(i)
	}
	wg.Wait()

	require.Len(t, m.hooks, numHooks)
	err := m.Run()
	require.NoError(t, err)
}

func TestHookManager_ConcurrentRun_Parallel(t *testing.T) {
	t.Parallel()
	logger := testutil.TestLogger(t)
	m := NewHookManager(logger)
	numHooks := 10
	for i := 0; i < numHooks; i++ {
		m.Register(fmt.Sprintf("slow-%d", i), func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
	}

	start := time.Now()
	err := m.Run()
	require.NoError(t, err)
	require.Less(t, time.Since(start), 200*time.Millisecond) // concurrent, ~50ms total
}

func TestHookManager_ErrorsLogged(t *testing.T) {
	t.Parallel()
	logger, hook := testutil.TestLoggerWithHook(t)
	m := NewHookManager(logger)
	m.Register("failing", func() error { return errors.New("test fail") })

	err := m.Run()
	require.Error(t, err)

	require.Len(t, hook.AllEntries(), 1)
	entry := hook.LastEntry()
	require.Equal(t, slog.LevelError, entry.Level)
	require.Equal(t, "Error while running hook", entry.Message)
	attrs := make(map[string]any)
	entry.Attrs(func(a slog.Attr) bool { attrs[a.Key] = a.Value.Any(); return true })
	require.Equal(t, "failing", attrs["hook"])
	errorEntry := attrs[ErrorKey]
	require.NotNil(t, errorEntry)
	err, ok := errorEntry.(error)
	require.True(t, ok)
	require.Contains(t, err.Error(), "test fail")
}
