package stage

import (
	"context"
	"io"
	"strings"
	"testing"

	"deploygo/internal/config"
	"deploygo/internal/container"
)

func TestRunCleanupRejectsAbsoluteDir(t *testing.T) {
	projectDir := t.TempDir()
	err := RunCleanup(&config.CleanupConfig{
		Enable: true,
		Dirs:   []string{"/tmp"},
	}, projectDir)
	if err == nil {
		t.Fatalf("expected cleanup with absolute path to fail")
	}
}

func TestRunTransferStepRejectsTraversalSource(t *testing.T) {
	projectDir := t.TempDir()
	server := &config.ServerConfig{Host: "127.0.0.1", User: "u", Port: 22}
	step := &config.DeploymentStep{
		Name:   "upload",
		From:   "../outside",
		To:     "/opt/app/",
		Server: "prod",
	}

	err := runTransferStep(server, step, projectDir)
	if err == nil {
		t.Fatalf("expected deploy source traversal to fail")
	}
}

func TestRunBuildsRejectsEmptyToDirProjectRoot(t *testing.T) {
	projectDir := t.TempDir()
	runtime := &testRuntime{}
	builds := []config.StageConfig{
		{
			Name:  "b1",
			Image: "alpine:latest",
			CopyToLocal: []config.CopyToLocalPath{
				{
					From:       "/tmp/out",
					ToDir:      ".",
					EmptyToDir: true,
				},
			},
		},
	}

	err := RunBuilds(runtime, &config.Config{}, builds, projectDir)
	if err == nil {
		t.Fatalf("expected empty_to_dir on project root to fail")
	}
}

type testRuntime struct{}

func (f *testRuntime) Name() string { return "fake" }
func (f *testRuntime) PullImage(ctx context.Context, image string) error {
	return nil
}
func (f *testRuntime) CreateContainer(ctx context.Context, cfg *container.ContainerConfig) (string, error) {
	return "1234567890abcdef", nil
}
func (f *testRuntime) StartContainer(ctx context.Context, id string) error { return nil }
func (f *testRuntime) Exec(ctx context.Context, containerID string, cmd ...string) error {
	return nil
}
func (f *testRuntime) WaitContainer(ctx context.Context, id string) error { return nil }
func (f *testRuntime) RemoveContainer(ctx context.Context, id string) error {
	return nil
}
func (f *testRuntime) CopyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	return nil
}
func (f *testRuntime) CopyFromContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	return nil
}
func (f *testRuntime) GetContainerLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *testRuntime) Close() error { return nil }
