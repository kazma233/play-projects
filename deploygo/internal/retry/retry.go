package retry

import (
	"fmt"
	"log"
	"time"
)

// 重试配置（硬编码）
const (
	DefaultRetryCount    = 3
	DefaultRetryInterval = 2 * time.Second
	MaxRetryInterval     = 30 * time.Second
)

// WithBackoff 指数退避重试
// fn 返回 (error, shouldRetry): error 是错误信息，shouldRetry 表示是否重试
func WithBackoff(operation string, fn func() (error, bool)) error {
	var lastErr error
	interval := DefaultRetryInterval

	for i := 0; i <= DefaultRetryCount; i++ {
		if i > 0 {
			log.Printf("%s 失败，第 %d 次重试，等待 %v...", operation, i, interval)
			time.Sleep(interval)
			interval *= 2
			if interval > MaxRetryInterval {
				interval = MaxRetryInterval
			}
		}

		err, shouldRetry := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if !shouldRetry {
			return err
		}
	}
	return fmt.Errorf("%s 重试 %d 次后仍然失败: %w", operation, DefaultRetryCount, lastErr)
}
