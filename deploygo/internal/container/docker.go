package container

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"deploygo/internal/fileutil"
)

type DockerRuntime struct {
	command string
	labels  containerLabelRegistry
	pulls   imagePullRegistry
}

func NewDockerRuntime() (*DockerRuntime, error) {
	cmd, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("docker not found in PATH: %w", err)
	}
	return &DockerRuntime{
		command: cmd,
		labels:  newContainerLabelRegistry(),
		pulls:   newImagePullRegistry(),
	}, nil
}

func (d *DockerRuntime) Name() string {
	return "docker"
}

func (d *DockerRuntime) runCommand(ctx context.Context, args ...string) (string, error) {
	log.Printf("Executing: %s %s", d.command, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, d.command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s failed: %s", strings.Join(args, " "), string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (d *DockerRuntime) PullImage(ctx context.Context, image string) error {
	return pullImageIfMissing(ctx, &d.pulls, image, d.imageExists, func(ctx context.Context, image string) error {
		_, err := d.runCommand(ctx, "pull", image)
		return err
	})
}

// imageExists 检查本地是否存在指定镜像
func (d *DockerRuntime) imageExists(ctx context.Context, image string) (bool, error) {
	// 使用 docker inspect 命令检查镜像是否存在
	// 通过退出码判断：0 表示存在，非0 表示不存在
	log.Printf("Checking if image '%s' exists locally...", image)
	cmd := exec.CommandContext(ctx, d.command, "inspect", "--format", "{{.Id}}", image)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	exists := err == nil
	if exists {
		log.Printf("Image '%s' exists locally, ID: %s", image, outputStr)
	} else {
		log.Printf("Image '%s' does not exist locally, output: %s", image, outputStr)
	}
	return exists, nil
}

func (d *DockerRuntime) CreateContainer(ctx context.Context, cfg *ContainerConfig) (string, error) {
	args := []string{"create"}

	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}

	args = append(args, cfg.Image)
	args = append(args, cfg.Cmd...)

	containerID, err := d.runCommand(ctx, args...)
	if err != nil {
		return "", err
	}

	d.labels.set(containerID, cfg.BuildName)
	return containerID, nil
}

func (d *DockerRuntime) StartContainer(ctx context.Context, id string) error {
	_, err := d.runCommand(ctx, "start", id)
	return err
}

func (d *DockerRuntime) Exec(ctx context.Context, containerID string, cmd ...string) error {
	args := []string{"exec", containerID}
	args = append(args, cmd...)

	log.Printf("Executing: docker %s", strings.Join(args, " "))
	return runStreamingCommand(ctx, d.command, d.labels.get(containerID), containerID, args...)
}

func (d *DockerRuntime) WaitContainer(ctx context.Context, id string) error {
	_, err := d.runCommand(ctx, "wait", id)
	return err
}

func (d *DockerRuntime) RemoveContainer(ctx context.Context, id string) error {
	_, err := d.runCommand(ctx, "rm", "-f", id)
	if err == nil {
		d.labels.remove(id)
	}
	return err
}

func (d *DockerRuntime) CopyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	_, err := d.runCommand(ctx, "cp", srcPath, fileutil.ContainerRef(containerID, dstPath))
	return err
}

func (d *DockerRuntime) CopyFromContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	os.MkdirAll(dstPath, 0755)
	_, err := d.runCommand(ctx, "cp", fileutil.ContainerRef(containerID, srcPath), dstPath)
	return err
}

func (d *DockerRuntime) Close() error {
	return nil
}
