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

type PodmanRuntime struct {
	command string
	labels  containerLabelRegistry
	pulls   imagePullRegistry
}

func NewPodmanRuntime() (*PodmanRuntime, error) {
	cmd, err := exec.LookPath("podman")
	if err != nil {
		return nil, fmt.Errorf("podman not found in PATH: %w", err)
	}
	return &PodmanRuntime{
		command: cmd,
		labels:  newContainerLabelRegistry(),
		pulls:   newImagePullRegistry(),
	}, nil
}

func (p *PodmanRuntime) Name() string {
	return "podman"
}

func (p *PodmanRuntime) runCommand(ctx context.Context, args ...string) (string, error) {
	log.Printf("Executing: %s %s", p.command, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, p.command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman %s failed: %s", strings.Join(args, " "), string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (p *PodmanRuntime) PullImage(ctx context.Context, image string) error {
	return pullImageIfMissing(ctx, &p.pulls, image, p.imageExists, func(ctx context.Context, image string) error {
		_, err := p.runCommand(ctx, "pull", image)
		return err
	})
}

// imageExists 检查本地是否存在指定镜像
func (p *PodmanRuntime) imageExists(ctx context.Context, image string) (bool, error) {
	// 使用 podman inspect 命令检查镜像是否存在
	// 通过退出码判断：0 表示存在，非0 表示不存在
	log.Printf("Checking if image '%s' exists locally...", image)
	cmd := exec.CommandContext(ctx, p.command, "inspect", "--format", "{{.Id}}", image)
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

func (p *PodmanRuntime) CreateContainer(ctx context.Context, cfg *ContainerConfig) (string, error) {
	args := []string{"create"}

	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}

	args = append(args, cfg.Image)
	args = append(args, cfg.Cmd...)

	containerID, err := p.runCommand(ctx, args...)
	if err != nil {
		return "", err
	}

	p.labels.set(containerID, cfg.BuildName)
	return containerID, nil
}

func (p *PodmanRuntime) StartContainer(ctx context.Context, id string) error {
	_, err := p.runCommand(ctx, "start", id)
	return err
}

func (p *PodmanRuntime) Exec(ctx context.Context, containerID string, cmd ...string) error {
	args := []string{"exec", containerID}
	args = append(args, cmd...)

	log.Printf("Executing: podman %s", strings.Join(args, " "))
	return runStreamingCommand(ctx, p.command, p.labels.get(containerID), containerID, args...)
}

func (p *PodmanRuntime) WaitContainer(ctx context.Context, id string) error {
	_, err := p.runCommand(ctx, "wait", id)
	return err
}

func (p *PodmanRuntime) RemoveContainer(ctx context.Context, id string) error {
	_, err := p.runCommand(ctx, "rm", "-f", id)
	if err == nil {
		p.labels.remove(id)
	}
	return err
}

func (p *PodmanRuntime) CopyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	_, err := p.runCommand(ctx, "cp", srcPath, fileutil.ContainerRef(containerID, dstPath))
	return err
}

func (p *PodmanRuntime) CopyFromContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	os.MkdirAll(dstPath, 0755)
	_, err := p.runCommand(ctx, "cp", fileutil.ContainerRef(containerID, srcPath), dstPath)
	return err
}

func (p *PodmanRuntime) Close() error {
	return nil
}
