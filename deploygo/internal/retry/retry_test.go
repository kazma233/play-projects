package retry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDoRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	targetErr := errors.New("temporary failure")

	policy := Policy{
		Attempts:     3,
		InitialDelay: 0,
		MaxDelay:     0,
		IsRetryable: func(error) bool {
			return true
		},
	}

	err := Do(context.Background(), "test", policy, func() error {
		attempts++
		if attempts < 3 {
			return targetErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoStopsOnNonRetryableError(t *testing.T) {
	attempts := 0
	targetErr := errors.New("permanent failure")

	policy := Policy{
		Attempts:     3,
		InitialDelay: 0,
		MaxDelay:     0,
		IsRetryable: func(error) bool {
			return false
		},
	}

	_, err := DoValue(context.Background(), "test", policy, func() (struct{}, error) {
		attempts++
		return struct{}{}, targetErr
	})
	if !errors.Is(err, targetErr) {
		t.Fatalf("expected %v, got %v", targetErr, err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoValueDoesNotRetryWhenClassifierMissing(t *testing.T) {
	attempts := 0
	targetErr := errors.New("temporary failure")

	_, err := DoValue(context.Background(), "test", Policy{}, func() (struct{}, error) {
		attempts++
		return struct{}{}, targetErr
	})
	if !errors.Is(err, targetErr) {
		t.Fatalf("expected %v, got %v", targetErr, err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestNewPolicyUsesProvidedRetryClassifier(t *testing.T) {
	policy := NewPolicy(func(error) bool {
		return true
	})

	if policy.Attempts != DefaultAttempts {
		t.Fatalf("expected %d attempts, got %d", DefaultAttempts, policy.Attempts)
	}
	if policy.InitialDelay != DefaultInitialDelay {
		t.Fatalf("expected initial delay %v, got %v", DefaultInitialDelay, policy.InitialDelay)
	}
	if policy.MaxDelay != DefaultMaxDelay {
		t.Fatalf("expected max delay %v, got %v", DefaultMaxDelay, policy.MaxDelay)
	}
	if !policy.IsRetryable(errors.New("retry me")) {
		t.Fatal("expected retry classifier to be used")
	}
}

func TestDoWrapsErrorAfterExhaustingAttempts(t *testing.T) {
	attempts := 0
	targetErr := errors.New("temporary failure")

	policy := Policy{
		Attempts:     2,
		InitialDelay: 0,
		MaxDelay:     0,
		IsRetryable: func(error) bool {
			return true
		},
	}

	_, err := DoValue(context.Background(), "test", policy, func() (struct{}, error) {
		attempts++
		return struct{}{}, targetErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, targetErr) {
		t.Fatalf("expected wrapped error %v, got %v", targetErr, err)
	}
	if !strings.Contains(err.Error(), "尝试 2 次后仍然失败") {
		t.Fatalf("expected retry summary in error, got %q", err.Error())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoCancelsWaitWhenContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	policy := Policy{
		Attempts:     2,
		InitialDelay: time.Second,
		MaxDelay:     time.Second,
		Jitter:       false,
		IsRetryable: func(error) bool {
			return true
		},
	}

	targetErr := errors.New("temporary failure")
	attempts := 0
	_, err := DoValue(ctx, "test", policy, func() (struct{}, error) {
		attempts++
		cancel()
		return struct{}{}, targetErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "重试等待被取消") {
		t.Fatalf("expected cancellation error, got %q", err.Error())
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt before cancellation, got %d", attempts)
	}
}

func TestDoValueReturnsResultAfterRetries(t *testing.T) {
	attempts := 0
	targetErr := errors.New("temporary failure")

	policy := Policy{
		Attempts:     3,
		InitialDelay: 0,
		MaxDelay:     0,
		IsRetryable: func(error) bool {
			return true
		},
	}

	value, err := DoValue(context.Background(), "test", policy, func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, targetErr
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if value != 42 {
		t.Fatalf("expected value 42, got %d", value)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
