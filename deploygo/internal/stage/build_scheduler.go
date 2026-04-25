package stage

import (
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
		if err := scheduler.Schedule(entry); err != nil {
			return err
		}
	}

	return scheduler.Flush()
}

func (s *buildScheduler) Schedule(entry queuedBuild) error {
	if entry.build == nil {
		return fmt.Errorf("build is nil")
	}

	if !entry.build.Sync {
		s.asyncBatch = append(s.asyncBatch, entry)
		return nil
	}

	if err := s.Flush(); err != nil {
		return err
	}

	return s.runEntry(entry)
}

func (s *buildScheduler) Flush() error {
	if len(s.asyncBatch) == 0 {
		return nil
	}

	batch := s.asyncBatch
	s.asyncBatch = nil

	return s.runAsyncBatch(batch)
}

func (s *buildScheduler) runAsyncBatch(batch []queuedBuild) error {
	if len(batch) == 1 {
		return s.runEntry(batch[0])
	}

	log.Printf("Running async build batch with %d builds", len(batch))

	errCh := make(chan error, len(batch))
	var wg sync.WaitGroup

	for _, entry := range batch {
		wg.Add(1)
		go func(entry queuedBuild) {
			defer wg.Done()
			if err := s.runEntry(entry); err != nil {
				errCh <- err
			}
		}(entry)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *buildScheduler) runEntry(entry queuedBuild) error {
	return runBuildEntry(s.runtime, entry.build, s.projectDir, entry.index, s.total)
}
