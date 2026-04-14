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
}

func NewPodmanRuntime() (*PodmanRuntime, error) {
	cmd, err := exec.LookPath("podman")
	if err != nil {
		return nil, fmt.Errorf("podman not found in PATH: %w", err)
	}
	return &PodmanRuntime{command: cmd}, nil
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
	// 检查本地是否已存在该镜像
	exists, err := p.imageExists(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to check image existence: %w", err)
	}
	if exists {
		log.Printf("Image '%s' already exists locally, skipping pull", image)
		return nil
	}

	log.Printf("Pulling image '%s'...", image)
	_, err = p.runCommand(ctx, "pull", image)
	return err
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

	return p.runCommand(ctx, args...)
}

func (p *PodmanRuntime) StartContainer(ctx context.Context, id string) error {
	_, err := p.runCommand(ctx, "start", id)
	return err
}

func (p *PodmanRuntime) Exec(ctx context.Context, containerID string, cmd ...string) error {
	args := []string{"exec", containerID}
	args = append(args, cmd...)

	log.Printf("Executing: podman %s", strings.Join(args, " "))
	execCmd := exec.CommandContext(ctx, p.command, args...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}

func (p *PodmanRuntime) WaitContainer(ctx context.Context, id string) error {
	_, err := p.runCommand(ctx, "wait", id)
	return err
}

func (p *PodmanRuntime) RemoveContainer(ctx context.Context, id string) error {
	_, err := p.runCommand(ctx, "rm", "-f", id)
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
