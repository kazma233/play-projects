package retry

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
)

const (
	DefaultAttempts     = 3
	DefaultInitialDelay = 2 * time.Second
	DefaultMaxDelay     = 30 * time.Second
)

type Policy struct {
	Attempts     int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Jitter       bool
	IsRetryable  func(error) bool
}

// NewPolicy 创建一份带默认参数的重试策略。
// 是否允许重试由调用方显式传入；如果不传，则默认视为不可重试。
func NewPolicy(isRetryable func(error) bool) Policy {
	return Policy{
		Attempts:     DefaultAttempts,
		InitialDelay: DefaultInitialDelay,
		MaxDelay:     DefaultMaxDelay,
		Jitter:       true,
		IsRetryable:  normalizeRetryable(isRetryable),
	}
}

// Do 适用于只关心 error 的场景。
// 它只是 DoValue 的薄封装，用来避免无返回值调用点重复写占位值。
func Do(ctx context.Context, operation string, policy Policy, fn func() error) error {
	_, err := DoValue(ctx, operation, policy, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// DoValue 适用于既需要返回结果、又需要统一重试控制的场景。
// 如果最终失败，会返回 T 的零值以及对应错误。
func DoValue[T any](ctx context.Context, operation string, policy Policy, fn func() (T, error)) (T, error) {
	ctx = ensureContext(ctx)
	policy = normalizePolicy(policy)

	var zero T
	var lastErr error
	delay := policy.InitialDelay
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 1; attempt <= policy.Attempts; attempt++ {
		if attempt == 1 {
			log.Printf("[%s] 开始执行...", operation)
		}

		value, err := fn()
		if err == nil {
			log.Printf("[%s] 执行成功", operation)
			return value, nil
		}

		lastErr = err
		if !policy.IsRetryable(err) {
			log.Printf("[%s] 第 %d/%d 次尝试失败: %v, 是否重试: false", operation, attempt, policy.Attempts, err)
			return zero, err
		}

		if attempt == policy.Attempts {
			break
		}

		wait := waitDuration(delay, policy.Jitter, rng)
		log.Printf("[%s] 第 %d/%d 次尝试失败: %v, 等待 %v 后重试", operation, attempt, policy.Attempts, err, wait)

		if err := sleep(ctx, wait); err != nil {
			return zero, fmt.Errorf("%s 重试等待被取消: %w", operation, err)
		}

		delay = nextDelay(delay, policy.MaxDelay)
	}

	log.Printf("[%s] 尝试 %d 次后仍然失败: %v", operation, policy.Attempts, lastErr)
	return zero, fmt.Errorf("%s 尝试 %d 次后仍然失败: %w", operation, policy.Attempts, lastErr)
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// normalizePolicy 负责补齐策略默认值。
// 尤其要保证未提供 IsRetryable 时，不会意外进入自动重试。
func normalizePolicy(policy Policy) Policy {
	if policy.Attempts <= 0 {
		policy.Attempts = DefaultAttempts
	}
	if policy.InitialDelay < 0 {
		policy.InitialDelay = 0
	}
	if policy.MaxDelay < 0 {
		policy.MaxDelay = 0
	}
	if policy.MaxDelay > 0 && policy.MaxDelay < policy.InitialDelay {
		policy.MaxDelay = policy.InitialDelay
	}
	if policy.IsRetryable == nil {
		policy.IsRetryable = normalizeRetryable(nil)
	}
	return policy
}

// normalizeRetryable 统一处理重试判定函数的默认值。
// 默认策略是保守模式：不明确可重试，就不重试。
func normalizeRetryable(isRetryable func(error) bool) func(error) bool {
	if isRetryable == nil {
		return func(error) bool {
			return false
		}
	}
	return isRetryable
}

func waitDuration(delay time.Duration, jitter bool, rng *rand.Rand) time.Duration {
	if delay <= 0 {
		return 0
	}
	if !jitter {
		return delay
	}
	return delay + time.Duration(rng.Int63n(int64(delay)))
}

func nextDelay(delay, maxDelay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	next := delay * 2
	if maxDelay > 0 && next > maxDelay {
		return maxDelay
	}
	return next
}

func sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
