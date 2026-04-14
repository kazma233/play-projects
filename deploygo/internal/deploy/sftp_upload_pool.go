package deploy

import (
	"errors"
	"sync"
)

const sftpUploadWorkers = 4

var errUploadWalkStopped = errors.New("stop walking upload tree")

type uploadTask struct {
	source string
	dest   string
}

type uploadPool struct {
	jobs      chan uploadTask
	upload    func(uploadTask) error
	wg        sync.WaitGroup
	mu        sync.Mutex
	err       error
	errOnce   sync.Once
	closeOnce sync.Once
	done      chan struct{}
}

func newUploadPool(workers int, upload func(uploadTask) error) *uploadPool {
	if workers < 1 {
		workers = 1
	}

	pool := &uploadPool{
		jobs:   make(chan uploadTask, workers*2),
		upload: upload,
		done:   make(chan struct{}),
	}

	for range workers {
		pool.wg.Add(1)
		go func() {
			defer pool.wg.Done()
			for task := range pool.jobs {
				if pool.Err() != nil {
					continue
				}
				if err := pool.upload(task); err != nil {
					pool.Fail(err)
				}
			}
		}()
	}

	return pool
}

func (p *uploadPool) Submit(task uploadTask) error {
	select {
	case <-p.done:
		return p.Err()
	case p.jobs <- task:
		return nil
	}
}

func (p *uploadPool) Fail(err error) {
	if err == nil {
		return
	}

	p.errOnce.Do(func() {
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	})
}

func (p *uploadPool) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *uploadPool) Stopped() bool {
	return p.Err() != nil
}

func (p *uploadPool) WalkStop(err error) error {
	p.Fail(err)
	return errUploadWalkStopped
}

func (p *uploadPool) Finish(walkErr error) error {
	if walkErr != nil && !errors.Is(walkErr, errUploadWalkStopped) {
		p.Fail(walkErr)
	}
	return p.Wait()
}

func (p *uploadPool) Wait() error {
	p.closeOnce.Do(func() {
		close(p.jobs)
	})
	p.wg.Wait()
	return p.Err()
}
