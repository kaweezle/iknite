// cSpell: words unsub
package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sampleEvent struct {
	Name string
	ID   int
}

func TestNew(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()
	require.NotNil(t, bus)
	require.NotNil(t, bus.subs)
	require.Empty(t, bus.subs)
}

func TestSubscribeAndPublish(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()

	ch, unsubscribe := bus.Subscribe(1)
	require.NotNil(t, ch)

	evt := sampleEvent{ID: 1, Name: "alpha"}
	bus.Publish(evt)

	select {
	case got, ok := <-ch:
		require.True(t, ok)
		require.Equal(t, evt, got)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event")
	}

	unsubscribe()

	_, ok := <-ch
	require.False(t, ok)
}

func TestMultipleSubscribersReceiveSameEvent(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()

	ch1, unsub1 := bus.Subscribe(1)
	defer unsub1()

	ch2, unsub2 := bus.Subscribe(1)
	defer unsub2()

	evt := sampleEvent{ID: 2, Name: "beta"}
	bus.Publish(evt)

	select {
	case got := <-ch1:
		require.Equal(t, evt, got)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first subscriber")
	}

	select {
	case got := <-ch2:
		require.Equal(t, evt, got)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for second subscriber")
	}
}

func TestUnsubscribeRemovesSubscriber(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()

	ch, unsubscribe := bus.Subscribe(1)
	unsubscribe()

	require.Eventually(t, func() bool {
		_, ok := <-ch
		return !ok
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestPublishDropsWhenBufferFull(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()

	ch, unsubscribe := bus.Subscribe(1)
	defer unsubscribe()

	first := sampleEvent{ID: 1, Name: "first"}
	second := sampleEvent{ID: 2, Name: "second"}

	bus.Publish(first)
	bus.Publish(second)

	select {
	case got := <-ch:
		require.Equal(t, first, got)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first event")
	}

	select {
	case got := <-ch:
		t.Fatalf("unexpected extra event: %+v", got)
	default:
	}
}

func TestSubscribeContextUnsubscribesOnCancel(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()
	ctx, cancel := context.WithCancel(context.Background())

	ch := bus.SubscribeContext(ctx, 1)
	require.NotNil(t, ch)

	cancel()

	require.Eventually(t, func() bool {
		_, ok := <-ch
		return !ok
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestPublishWithNoSubscribers(t *testing.T) {
	t.Parallel()

	bus := New[sampleEvent]()
	require.NotPanics(t, func() {
		bus.Publish(sampleEvent{ID: 99, Name: "noop"})
	})
}
