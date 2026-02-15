package retry

import (
	"fmt"
	"log"
	"math/rand"
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < DefaultRetryCount; i++ {
		if i == 0 {
			log.Printf("[%s] 开始执行...", operation)
		}

		if i > 0 {
			jitter := time.Duration(r.Int63n(int64(interval)))
			actualWait := interval + jitter

			log.Printf("[%s] 执行失败，第 %d 次重试，等待 %v...", operation, i, actualWait)
			time.Sleep(actualWait)
			interval *= 2
			if interval > MaxRetryInterval {
				interval = MaxRetryInterval
			}
		}

		err, shouldRetry := fn()
		if err == nil {
			log.Printf("[%s] 执行成功", operation)
			return nil
		}

		log.Printf("[%s] 执行失败: %v, 是否重试: %v", operation, err, shouldRetry)
		lastErr = err
		if !shouldRetry {
			log.Printf("[%s] 不可重试，终止", operation)
			return err
		}
	}
	log.Printf("[%s] 重试 %d 次后仍然失败: %v", operation, DefaultRetryCount, lastErr)
	return fmt.Errorf("%s 重试 %d 次后仍然失败: %w", operation, DefaultRetryCount, lastErr)
}
