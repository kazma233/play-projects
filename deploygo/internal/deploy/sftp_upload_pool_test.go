package deploy

import (
	"errors"
	"slices"
	"sync"
	"testing"
)

func TestUploadPoolProcessesAllTasks(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []string
	)

	pool := newUploadPool(2, func(task uploadTask) error {
		mu.Lock()
		seen = append(seen, task.dest)
		mu.Unlock()
		return nil
	})

	tasks := []uploadTask{
		{source: "a", dest: "one"},
		{source: "b", dest: "two"},
		{source: "c", dest: "three"},
	}

	for _, task := range tasks {
		if err := pool.Submit(task); err != nil {
			t.Fatalf("Submit(%+v) unexpected error: %v", task, err)
		}
	}

	if err := pool.Wait(); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}

	slices.Sort(seen)
	want := []string{"one", "three", "two"}
	if !slices.Equal(seen, want) {
		t.Fatalf("processed tasks = %v, want %v", seen, want)
	}
}

func TestUploadPoolReturnsFirstError(t *testing.T) {
	failErr := errors.New("upload failed")
	failed := make(chan struct{})

	pool := newUploadPool(1, func(task uploadTask) error {
		if task.dest == "fail" {
			close(failed)
			return failErr
		}
		return nil
	})

	if err := pool.Submit(uploadTask{source: "a", dest: "fail"}); err != nil {
		t.Fatalf("Submit(fail) unexpected error: %v", err)
	}

	<-failed

	if err := pool.Submit(uploadTask{source: "b", dest: "after"}); !errors.Is(err, failErr) {
		t.Fatalf("Submit(after) error = %v, want %v", err, failErr)
	}

	if err := pool.Wait(); !errors.Is(err, failErr) {
		t.Fatalf("Wait() error = %v, want %v", err, failErr)
	}
}

func TestUploadPoolWalkStopStoresError(t *testing.T) {
	failErr := errors.New("walk failed")
	pool := newUploadPool(1, func(task uploadTask) error {
		return nil
	})

	stopErr := pool.WalkStop(failErr)
	if !errors.Is(stopErr, errUploadWalkStopped) {
		t.Fatalf("WalkStop() error = %v, want %v", stopErr, errUploadWalkStopped)
	}

	if err := pool.Finish(stopErr); !errors.Is(err, failErr) {
		t.Fatalf("Finish() error = %v, want %v", err, failErr)
	}
}

func TestUploadPoolFinishPreservesWalkError(t *testing.T) {
	walkErr := errors.New("walk unexpected error")
	pool := newUploadPool(1, func(task uploadTask) error {
		return nil
	})

	if err := pool.Finish(walkErr); !errors.Is(err, walkErr) {
		t.Fatalf("Finish() error = %v, want %v", err, walkErr)
	}
}
