// cSpell: words sirupsen paralleltest unwrapable
package utils

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	testHook "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
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
			m := &HookManager{}

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
	m := &HookManager{}
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
	m := &HookManager{}
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

//nolint:paralleltest  // messing with global logger
func TestHookManager_ErrorsLogged(t *testing.T) {
	hooks := make(logrus.LevelHooks)
	hook := &testHook.Hook{}
	hooks.Add(hook)
	oldHooks := logrus.StandardLogger().ReplaceHooks(hooks)
	t.Cleanup(func() {
		_ = logrus.StandardLogger().ReplaceHooks(oldHooks)
	})

	m := &HookManager{}
	m.Register("failing", func() error { return errors.New("test fail") })

	err := m.Run()
	require.Error(t, err)

	require.Len(t, hook.AllEntries(), 1)
	entry := hook.LastEntry()
	require.Equal(t, logrus.ErrorLevel, entry.Level)
	require.Equal(t, "Error while running hook", entry.Message)
	require.Equal(t, "failing", entry.Data["hook"])
	errorEntry := entry.Data["error"]
	require.NotNil(t, errorEntry)
	err, ok := errorEntry.(error)
	require.True(t, ok)
	require.Contains(t, err.Error(), "test fail")
}
