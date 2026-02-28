package stage

import (
	"context"
	"deploygo/internal/config"
	"deploygo/internal/container"
	"deploygo/internal/pathutil"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func RunBuilds(runtime container.ContainerRuntime, cfg *config.Config, builds []config.StageConfig, projectDir string) error {
	for i, build := range builds {
		ctx := context.Background()

		log.Printf("Running build %d/%d: %s (runtime: %s, image: %s)", i+1, len(builds), build.Name, runtime.Name(), build.Image)

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
			files, err := pathutil.GlobFiles(cp.From, projectDir)
			if err != nil {
				return fmt.Errorf("failed to resolve copy_to_container pattern %q: %w", cp.From, err)
			}
			for _, src := range files {
				srcAbs, err := pathutil.ResolveProjectPath(projectDir, src, false)
				if err != nil {
					return fmt.Errorf("invalid copy_to_container path %q: %w", src, err)
				}
				dst := path.Join(cp.ToDir, filepath.Base(src))
				log.Printf("Copying %s -> %s:%s", srcAbs, containerID[:12], dst)
				if err := runtime.CopyToContainer(ctx, containerID, srcAbs, dst); err != nil {
					return fmt.Errorf("failed to copy to container: %w", err)
				}
			}
		}

		cmd := strings.Join(build.Commands, " && ")
		if build.WorkingDir != "" {
			cmd = fmt.Sprintf("cd %s && %s", build.WorkingDir, cmd)
		}
		log.Printf("Executing: %s", cmd)

		if err := runtime.Exec(ctx, containerID, "sh", "-c", cmd); err != nil {
			logs, _ := runtime.GetContainerLogs(ctx, containerID)
			if logs != nil {
				data, _ := io.ReadAll(logs)
				logs.Close()
				log.Printf("Container logs:\n%s", string(data))
			}
			return fmt.Errorf("command failed: %w", err)
		}

		for _, cp := range build.CopyToLocal {
			toAbs, err := pathutil.ResolveProjectPath(projectDir, cp.ToDir, true)
			if err != nil {
				return fmt.Errorf("invalid copy_to_local.to_dir %q: %w", cp.ToDir, err)
			}
			if err := os.MkdirAll(toAbs, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			if cp.EmptyToDir {
				if filepath.Clean(toAbs) == filepath.Clean(projectDir) {
					return fmt.Errorf("invalid copy_to_local.to_dir %q: refusing to empty project root", cp.ToDir)
				}
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

		logs, err := runtime.GetContainerLogs(ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}
		defer logs.Close()

		logData, _ := io.ReadAll(logs)
		log.Printf("Output: %s", string(logData))
	}

	return nil
}
