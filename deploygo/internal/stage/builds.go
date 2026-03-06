package stage

import (
	"context"
	"deploygo/internal/config"
	"deploygo/internal/container"
	"deploygo/internal/fileutil"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func RunBuilds(runtime container.ContainerRuntime, builds []config.StageConfig, projectDir string) error {
	for i := range builds {
		if err := runBuildEntry(runtime, &builds[i], projectDir, i+1, len(builds)); err != nil {
			return err
		}
	}

	return nil
}

func RunBuild(runtime container.ContainerRuntime, build *config.StageConfig, projectDir string) error {
	return runBuildEntry(runtime, build, projectDir, 0, 0)
}

func runBuildEntry(runtime container.ContainerRuntime, build *config.StageConfig, projectDir string, index, total int) error {
	if build == nil {
		return fmt.Errorf("build is nil")
	}

	if total > 0 {
		log.Printf("Running build %d/%d: %s (runtime: %s, image: %s)", index, total, build.Name, runtime.Name(), build.Image)
	} else {
		log.Printf("Running build: %s (runtime: %s, image: %s)", build.Name, runtime.Name(), build.Image)
	}

	if err := runBuild(runtime, *build, projectDir); err != nil {
		return fmt.Errorf("build %q failed: %w", build.Name, err)
	}

	return nil
}

func runBuild(runtime container.ContainerRuntime, build config.StageConfig, projectDir string) error {
	ctx := context.Background()

	if err := runtime.PullImage(ctx, build.Image); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", build.Image, err)
	}

	containerCfg := &container.ContainerConfig{
		Image:      build.Image,
		Cmd:        []string{"sleep", "infinity"},
		WorkingDir: build.WorkingDir,
		Env:        build.Environment,
	}

	log.Printf("Creating container with image: %s", build.Image)
	containerID, err := runtime.CreateContainer(ctx, containerCfg)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	defer runtime.RemoveContainer(ctx, containerID)

	log.Printf("Starting container %s", containerID[:12])
	if err := runtime.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	if build.WorkingDir != "" {
		log.Printf("Creating working directory: %s", build.WorkingDir)
		if err := runtime.Exec(ctx, containerID, "mkdir", "-p", build.WorkingDir); err != nil {
			return fmt.Errorf("failed to create working directory: %w", err)
		}
	}

	for _, cp := range build.CopyToContainer {
		dir := cp.ToDir
		if dir != "" && dir != "." && dir != "/" {
			log.Printf("Creating directory: %s", dir)
			if err := runtime.Exec(ctx, containerID, "mkdir", "-p", dir); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	for _, cp := range build.CopyToContainer {
		if err := fileutil.EnsurePatternWithin(projectDir, cp.From); err != nil {
			return fmt.Errorf("invalid copy_to_container path: %w", err)
		}
		if err := fileutil.GlobFiles(cp.From, projectDir, func(src string) error {
			srcAbs := filepath.Join(projectDir, src)
			dst := path.Join(cp.ToDir, filepath.Base(src))
			log.Printf("Copying %s -> %s:%s", srcAbs, containerID[:12], dst)
			if err := runtime.CopyToContainer(ctx, containerID, srcAbs, dst); err != nil {
				return fmt.Errorf("failed to copy to container: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	log.Printf("Executing: %s", strings.Join(build.Commands, " && "))
	cmd := fmt.Sprintf("cd %s && %s && exit", build.WorkingDir, strings.Join(build.Commands, " && "))
	if err := runtime.Exec(ctx, containerID, "sh", "-c", cmd); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	for _, cp := range build.CopyToLocal {
		toAbs, err := fileutil.ResolveWithin(projectDir, cp.ToDir)
		if err != nil {
			return fmt.Errorf("invalid copy_to_local path: %w", err)
		}
		if err := os.MkdirAll(toAbs, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		if cp.EmptyToDir {
			log.Printf("Emptying directory: %s", toAbs)
			if err := os.RemoveAll(toAbs); err != nil {
				return fmt.Errorf("failed to empty directory: %w", err)
			}
			if err := os.MkdirAll(toAbs, 0755); err != nil {
				return fmt.Errorf("failed to recreate directory: %w", err)
			}
		}

		log.Printf("Copying %s:%s -> %s", containerID[:12], cp.From, toAbs)
		if err := runtime.CopyFromContainer(ctx, containerID, cp.From, toAbs); err != nil {
			return fmt.Errorf("failed to copy from container: %w", err)
		}
	}

	return nil
}
