package container

import (
	"context"
	"sync"
)

type imagePullCall struct {
	done chan struct{}
	err  error
}

type imagePullRegistry struct {
	mu    sync.Mutex
	calls map[string]*imagePullCall
}

func newImagePullRegistry() imagePullRegistry {
	return imagePullRegistry{
		calls: make(map[string]*imagePullCall),
	}
}

func (r *imagePullRegistry) do(ctx context.Context, image string, fn func(context.Context) error) error {
	if r == nil || image == "" {
		return fn(ctx)
	}

	r.mu.Lock()
	if r.calls == nil {
		r.calls = make(map[string]*imagePullCall)
	}
	if call, ok := r.calls[image]; ok {
		r.mu.Unlock()
		select {
		case <-call.done:
			return call.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	call := &imagePullCall{done: make(chan struct{})}
	r.calls[image] = call
	r.mu.Unlock()

	err := fn(ctx)

	r.mu.Lock()
	call.err = err
	delete(r.calls, image)
	close(call.done)
	r.mu.Unlock()

	return err
}
