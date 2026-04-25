package container

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestImagePullRegistryDeduplicatesConcurrentPulls(t *testing.T) {
	registry := newImagePullRegistry()
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	var count int32

	run := func() error {
		return registry.do(context.Background(), "golang:1.24", func(context.Context) error {
			if atomic.AddInt32(&count, 1) != 1 {
				t.Fatal("pull function ran more than once")
			}
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return nil
		})
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- run()
		}()
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pull to start")
	}

	close(release)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	}

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("pull function ran %d times, want 1", got)
	}
}

func TestImagePullRegistryWaiterRespectsContextCancellation(t *testing.T) {
	registry := newImagePullRegistry()
	release := make(chan struct{})
	started := make(chan struct{}, 1)

	go func() {
		_ = registry.do(context.Background(), "golang:1.24", func(context.Context) error {
			started <- struct{}{}
			<-release
			return nil
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for leading pull to start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := registry.do(ctx, "golang:1.24", func(context.Context) error {
		t.Fatal("waiting caller should not execute the pull")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context.Canceled", err)
	}

	close(release)
}
