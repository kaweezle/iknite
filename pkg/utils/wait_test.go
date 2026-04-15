// cSpell: words stretchr
package utils_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/kaweezle/iknite/pkg/utils"
)

func TestWaitOptions_DefaultsAndInitialization(t *testing.T) {
	t.Parallel()

	t.Run("new defaults", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := utils.NewWaitOptions()
		req.Zero(waitOptions.Timeout)
		req.Equal(10*time.Second, waitOptions.CheckTimeout)
		req.Equal(2*time.Second, waitOptions.Interval)
		req.Equal(3, waitOptions.Retries)
		req.Equal(1, waitOptions.OkResponses)
		req.True(waitOptions.Wait)
		req.False(waitOptions.Watch)
		req.True(waitOptions.Immediate)
	})

	t.Run("initialize resets values", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := &utils.WaitOptions{
			Timeout:      20 * time.Second,
			CheckTimeout: 3 * time.Second,
			Interval:     20 * time.Millisecond,
			Retries:      9,
			OkResponses:  4,
			Wait:         false,
			Watch:        true,
			Immediate:    false,
		}

		utils.InitializeWaitOptions(waitOptions)
		req.Zero(waitOptions.Timeout)
		req.Equal(10*time.Second, waitOptions.CheckTimeout)
		req.Equal(2*time.Second, waitOptions.Interval)
		req.Equal(3, waitOptions.Retries)
		req.Equal(1, waitOptions.OkResponses)
		req.True(waitOptions.Wait)
		req.False(waitOptions.Watch)
		req.True(waitOptions.Immediate)
	})
}

func TestWaitOptions_AddFlags(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	waitOptions := utils.NewWaitOptions()
	flags := pflag.NewFlagSet("wait", pflag.ContinueOnError)
	utils.AddWaitOptionsFlags(flags, waitOptions)

	err := flags.Parse([]string{
		"--timeout", "3s",
		"--check-timeout", "4s",
		"--check-interval", "150ms",
		"--check-retries", "7",
		"--check-ok-responses", "2",
		"--watch",
		"--wait=false",
		"--check-immediate=false",
	})
	req.NoError(err)

	req.Equal(3*time.Second, waitOptions.Timeout)
	req.Equal(4*time.Second, waitOptions.CheckTimeout)
	req.Equal(150*time.Millisecond, waitOptions.Interval)
	req.Equal(7, waitOptions.Retries)
	req.Equal(2, waitOptions.OkResponses)
	req.True(waitOptions.Watch)
	req.False(waitOptions.Wait)
	req.False(waitOptions.Immediate)
}

func TestWaitOptions_ValidateAndHasLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       utils.WaitOptions
		expected    utils.WaitOptions
		hasLoopWant bool
	}{
		{
			name: "watch overrides timeout and wait",
			input: utils.WaitOptions{
				Timeout: 8 * time.Second,
				Watch:   true,
				Wait:    true,
			},
			expected: utils.WaitOptions{
				Timeout: 0,
				Watch:   true,
				Wait:    false,
			},
			hasLoopWant: true,
		},
		{
			name: "wait clears timeout",
			input: utils.WaitOptions{
				Timeout: 9 * time.Second,
				Wait:    true,
			},
			expected: utils.WaitOptions{
				Timeout: 0,
				Wait:    true,
			},
			hasLoopWant: true,
		},
		{
			name: "timeout enables loop",
			input: utils.WaitOptions{
				Timeout: 2 * time.Second,
			},
			expected: utils.WaitOptions{
				Timeout: 2 * time.Second,
			},
			hasLoopWant: true,
		},
		{
			name: "no watch wait or timeout",
			input: utils.WaitOptions{
				Wait:  false,
				Watch: false,
			},
			expected: utils.WaitOptions{
				Wait:  false,
				Watch: false,
			},
			hasLoopWant: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := require.New(t)

			waitOptions := tt.input
			err := waitOptions.Validate()
			req.NoError(err)
			req.Equal(tt.expected.Timeout, waitOptions.Timeout)
			req.Equal(tt.expected.Wait, waitOptions.Wait)
			req.Equal(tt.expected.Watch, waitOptions.Watch)
			req.Equal(tt.hasLoopWant, waitOptions.HasLoop())
			req.Contains(waitOptions.String(), "WaitOptions{")
		})
	}
}

func TestWaitOptions_Poll(t *testing.T) {
	t.Parallel()

	t.Run("no loop executes condition once", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := &utils.WaitOptions{Wait: false, Watch: false, Timeout: 0}
		calls := int32(0)
		err := waitOptions.Poll(context.Background(), func(_ context.Context) (bool, error) {
			atomic.AddInt32(&calls, 1)
			return false, nil
		})

		req.NoError(err)
		req.Equal(int32(1), atomic.LoadInt32(&calls))
	})

	t.Run("no loop propagates error", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := &utils.WaitOptions{Wait: false, Watch: false, Timeout: 0}
		err := waitOptions.Poll(context.Background(), func(_ context.Context) (bool, error) {
			return false, errors.New("boom")
		})

		req.EqualError(err, "boom")
	})

	t.Run("retry allows transient errors", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := &utils.WaitOptions{
			Wait:      false,
			Watch:     false,
			Timeout:   200 * time.Millisecond,
			Interval:  10 * time.Millisecond,
			Retries:   2,
			Immediate: true,
		}

		calls := int32(0)
		err := waitOptions.Poll(context.Background(), func(_ context.Context) (bool, error) {
			attempt := atomic.AddInt32(&calls, 1)
			if attempt == 1 {
				return false, errors.New("temporary")
			}
			return true, nil
		})

		req.NoError(err)
		req.GreaterOrEqual(atomic.LoadInt32(&calls), int32(2))
	})

	t.Run("ok responses require consecutive successes", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := &utils.WaitOptions{
			Wait:        false,
			Watch:       false,
			Timeout:     200 * time.Millisecond,
			Interval:    10 * time.Millisecond,
			Retries:     0,
			OkResponses: 2,
			Immediate:   true,
		}

		calls := int32(0)
		err := waitOptions.Poll(context.Background(), func(_ context.Context) (bool, error) {
			attempt := atomic.AddInt32(&calls, 1)
			switch attempt {
			case 1:
				return true, nil
			case 2:
				return false, nil
			default:
				return true, nil
			}
		})

		req.NoError(err)
		req.GreaterOrEqual(atomic.LoadInt32(&calls), int32(4))
	})

	t.Run("watch ends on context cancellation", func(t *testing.T) {
		t.Parallel()
		req := require.New(t)

		waitOptions := &utils.WaitOptions{
			Wait:      false,
			Watch:     true,
			Interval:  5 * time.Millisecond,
			Retries:   0,
			Immediate: true,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := waitOptions.Poll(ctx, func(_ context.Context) (bool, error) {
			return true, nil
		})

		req.Error(err)
		req.Contains(err.Error(), "condition not met within the specified options")
	})
}
