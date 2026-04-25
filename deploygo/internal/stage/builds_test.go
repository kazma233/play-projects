package stage

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"deploygo/internal/config"
	"deploygo/internal/container"
)

type blockingRuntime struct {
	mu             sync.Mutex
	nextID         int
	containerBuild map[string]string
	gates          map[string]chan struct{}
	events         []string
	active         int
	maxActive      int
	started        chan string
}

func newBlockingRuntime(gates map[string]chan struct{}) *blockingRuntime {
	return &blockingRuntime{
		containerBuild: make(map[string]string),
		gates:          gates,
		started:        make(chan string, len(gates)),
	}
}

func (r *blockingRuntime) Name() string {
	return "fake"
}

func (r *blockingRuntime) PullImage(context.Context, string) error {
	return nil
}

func (r *blockingRuntime) CreateContainer(_ context.Context, cfg *container.ContainerConfig) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := fmt.Sprintf("container-%d", r.nextID)
	r.nextID++
	r.containerBuild[id] = cfg.Image
	return id, nil
}

func (r *blockingRuntime) StartContainer(context.Context, string) error {
	return nil
}

func (r *blockingRuntime) Exec(ctx context.Context, containerID string, cmd ...string) error {
	if len(cmd) >= 2 && cmd[0] == "mkdir" && cmd[1] == "-p" {
		return nil
	}

	if len(cmd) < 3 || cmd[0] != "sh" || cmd[1] != "-c" {
		return nil
	}

	buildName := r.lookupBuild(containerID)
	r.recordEvent("start", buildName)

	select {
	case r.started <- buildName:
	case <-ctx.Done():
		return ctx.Err()
	}

	if gate := r.lookupGate(buildName); gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.recordEvent("end", buildName)
	return nil
}

func (r *blockingRuntime) WaitContainer(context.Context, string) error {
	return nil
}

func (r *blockingRuntime) RemoveContainer(context.Context, string) error {
	return nil
}

func (r *blockingRuntime) CopyToContainer(context.Context, string, string, string) error {
	return nil
}

func (r *blockingRuntime) CopyFromContainer(context.Context, string, string, string) error {
	return nil
}

func (r *blockingRuntime) Close() error {
	return nil
}

func (r *blockingRuntime) lookupBuild(containerID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.containerBuild[containerID]
}

func (r *blockingRuntime) lookupGate(buildName string) chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gates[buildName]
}

func (r *blockingRuntime) recordEvent(kind, buildName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch kind {
	case "start":
		r.active++
		if r.active > r.maxActive {
			r.maxActive = r.active
		}
	case "end":
		r.active--
	}

	r.events = append(r.events, fmt.Sprintf("%s:%s", kind, buildName))
}

func (r *blockingRuntime) snapshotEvents() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.events)
}

func (r *blockingRuntime) maxConcurrent() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxActive
}

func TestRunBuildsHonorsSyncBuilds(t *testing.T) {
	gates := map[string]chan struct{}{
		"frontend": make(chan struct{}),
		"backend":  make(chan struct{}),
		"package":  make(chan struct{}),
		"docs":     make(chan struct{}),
	}
	runtime := newBlockingRuntime(gates)
	builds := []config.StageConfig{
		{Name: "frontend", Image: "frontend", WorkingDir: "/work", Commands: []string{"build frontend"}},
		{Name: "backend", Image: "backend", WorkingDir: "/work", Commands: []string{"build backend"}},
		{Name: "package", Image: "package", WorkingDir: "/work", Commands: []string{"build package"}, Sync: true},
		{Name: "docs", Image: "docs", WorkingDir: "/work", Commands: []string{"build docs"}},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunBuilds(runtime, builds, t.TempDir())
	}()

	firstTwo := []string{
		waitForBuildStart(t, runtime.started),
		waitForBuildStart(t, runtime.started),
	}
	slices.Sort(firstTwo)
	if !slices.Equal(firstTwo, []string{"backend", "frontend"}) {
		t.Fatalf("first async batch = %v, want [backend frontend]", firstTwo)
	}

	assertNoBuildStart(t, runtime.started)

	close(gates["frontend"])
	close(gates["backend"])

	if got := waitForBuildStart(t, runtime.started); got != "package" {
		t.Fatalf("sync build started as %q, want package", got)
	}

	eventsBeforePackage := runtime.snapshotEvents()
	assertEventOrder(t, eventsBeforePackage, "end:frontend", "start:package")
	assertEventOrder(t, eventsBeforePackage, "end:backend", "start:package")
	assertNoBuildStart(t, runtime.started)

	close(gates["package"])

	if got := waitForBuildStart(t, runtime.started); got != "docs" {
		t.Fatalf("post-sync async build started as %q, want docs", got)
	}

	close(gates["docs"])

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunBuilds() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunBuilds() did not finish")
	}

	if got := runtime.maxConcurrent(); got != 2 {
		t.Fatalf("max concurrent builds = %d, want 2", got)
	}

	events := runtime.snapshotEvents()
	assertEventOrder(t, events, "start:frontend", "start:package")
	assertEventOrder(t, events, "start:backend", "start:package")
	assertEventOrder(t, events, "end:package", "start:docs")
}

func waitForBuildStart(t *testing.T, started <-chan string) string {
	t.Helper()

	select {
	case buildName := <-started:
		return buildName
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for build start")
		return ""
	}
}

func assertNoBuildStart(t *testing.T, started <-chan string) {
	t.Helper()

	select {
	case buildName := <-started:
		t.Fatalf("unexpected build start: %s", buildName)
	case <-time.After(150 * time.Millisecond):
	}
}

func assertEventOrder(t *testing.T, events []string, earlier, later string) {
	t.Helper()

	earlierIndex := slices.Index(events, earlier)
	if earlierIndex < 0 {
		t.Fatalf("event %q not found in %v", earlier, events)
	}

	laterIndex := slices.Index(events, later)
	if laterIndex < 0 {
		t.Fatalf("event %q not found in %v", later, events)
	}

	if earlierIndex >= laterIndex {
		t.Fatalf("event order %q before %q not satisfied: %v", earlier, later, events)
	}
}
