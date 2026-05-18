package testutil

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/kaweezle/iknite/pkg/constants"
)

func NewLogger(out io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func TestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return NewLogger(os.Stderr)
}

type Hook struct {
	Entries []*slog.Record
	mu      sync.RWMutex
}

func NewHook() *Hook {
	hook := new(Hook)
	return hook
}

func (t *Hook) LastEntry() *slog.Record {
	t.mu.RLock()
	defer t.mu.RUnlock()
	i := len(t.Entries) - 1
	if i < 0 {
		return nil
	}
	return t.Entries[i]
}

// AllEntries returns all entries that were logged.
func (t *Hook) AllEntries() []*slog.Record {
	t.mu.RLock()
	defer t.mu.RUnlock()
	// Make a copy so the returned value won't race with future log requests
	entries := make([]*slog.Record, len(t.Entries))
	copy(entries, t.Entries)
	return entries
}

// Reset removes all Entries from this test hook.
func (t *Hook) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Entries = make([]*slog.Record, 0)
}

type HookHandler struct {
	hook    *Hook
	handler slog.Handler
}

// Enabled implements [slog.Handler].
func (h *HookHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements [slog.Handler].
//
//nolint:gocritic // implementing interface
func (h *HookHandler) Handle(ctx context.Context, record slog.Record) error {
	h.hook.mu.Lock()
	defer h.hook.mu.Unlock()
	h.hook.Entries = append(h.hook.Entries, &record)
	return h.handler.Handle(ctx, record) //nolint:wrapcheck // we don't need to wrap this error
}

// WithAttrs implements [slog.Handler].
func (h *HookHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &HookHandler{
		hook:    h.hook,
		handler: h.handler.WithAttrs(attrs),
	}
}

// WithGroup implements [slog.Handler].
func (h *HookHandler) WithGroup(name string) slog.Handler {
	return &HookHandler{
		hook:    h.hook,
		handler: h.handler.WithGroup(name),
	}
}

var _ slog.Handler = (*HookHandler)(nil)

func TestLoggerWithHook(t *testing.T) (*slog.Logger, *Hook) {
	t.Helper()
	hook := NewHook()
	t.Cleanup(func() {
		hook.Reset()
	})

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	return slog.New(&HookHandler{
		hook:    hook,
		handler: handler,
	}), hook
}

type holder struct {
	logger *slog.Logger
}

func (h *holder) Logger() *slog.Logger {
	return h.logger
}

func WithTestLogger(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	logger := TestLogger(t)
	h := &holder{logger: logger}
	return context.WithValue(ctx, constants.LoggerContextKey{}, h)
}
