package stage

import (
	"context"
	"deploygo/internal/config"
	"deploygo/internal/container"
	"fmt"
	"log"
	"sync"
)

type queuedBuild struct {
	build *config.StageConfig
	index int
}

type buildScheduler struct {
	runtime    container.ContainerRuntime
	projectDir string
	total      int
	asyncBatch []queuedBuild
}

func RunBuilds(runtime container.ContainerRuntime, builds []config.StageConfig, projectDir string) error {
	scheduler := buildScheduler{
		runtime:    runtime,
		projectDir: projectDir,
		total:      len(builds),
	}

	for i := range builds {
		entry := queuedBuild{
			build: &builds[i],
			index: i + 1,
		}
		if err := scheduler.schedule(entry); err != nil {
			return err
		}
	}

	return scheduler.flush()
}

func (s *buildScheduler) schedule(entry queuedBuild) error {
	if entry.build == nil {
		return fmt.Errorf("build is nil")
	}

	if !entry.build.Sync {
		s.asyncBatch = append(s.asyncBatch, entry)
		return nil
	}

	if err := s.flush(); err != nil {
		return err
	}

	return s.runEntry(context.Background(), entry)
}

func (s *buildScheduler) flush() error {
	if len(s.asyncBatch) == 0 {
		return nil
	}

	batch := s.asyncBatch
	s.asyncBatch = nil

	return s.runAsyncBatch(batch)
}

func (s *buildScheduler) runAsyncBatch(batch []queuedBuild) error {
	if len(batch) == 1 {
		return s.runEntry(context.Background(), batch[0])
	}

	log.Printf("Running async build batch with %d builds", len(batch))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error

	for _, entry := range batch {
		wg.Add(1)
		go func(entry queuedBuild) {
			defer wg.Done()
			if err := s.runEntry(ctx, entry); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(entry)
	}

	wg.Wait()
	return firstErr
}

func (s *buildScheduler) runEntry(ctx context.Context, entry queuedBuild) error {
	return runBuildEntry(ctx, s.runtime, entry.build, s.projectDir, entry.index, s.total)
}
