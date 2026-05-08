package utils

import (
	"context"
	"sync"
	"sync/atomic"
)

type Bus[T any] struct {
	subs   map[uint64]chan T
	mu     sync.RWMutex
	nextID atomic.Uint64
}

func New[T any]() *Bus[T] {
	return &Bus[T]{
		subs: make(map[uint64]chan T),
	}
}

func (b *Bus[T]) Subscribe(buf int) (<-chan T, func()) {
	c := make(chan T, buf)
	id := b.nextID.Add(1)

	b.mu.Lock()
	if b.subs == nil {
		b.subs = make(map[uint64]chan T)
	}
	b.subs[id] = c
	b.mu.Unlock()

	return c, func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if ch, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(ch)
		}
	}
}

func (b *Bus[T]) SubscribeContext(ctx context.Context, buf int) <-chan T {
	ch, unsubscribe := b.Subscribe(buf)
	go func() {
		<-ctx.Done()
		unsubscribe()
	}()
	return ch
}

func (b *Bus[T]) Publish(evt T) {
	b.mu.RLock()
	subs := make([]chan T, 0, len(b.subs))
	for _, ch := range b.subs {
		subs = append(subs, ch)
	}
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}
