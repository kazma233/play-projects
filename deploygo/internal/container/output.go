package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

var prefixedOutputMu sync.Mutex

type prefixedLineWriter struct {
	dest   io.Writer
	prefix string
	buffer bytes.Buffer
}

func newPrefixedLineWriter(dest io.Writer, prefix string) *prefixedLineWriter {
	return &prefixedLineWriter{
		dest:   dest,
		prefix: prefix,
	}
}

func (w *prefixedLineWriter) Write(p []byte) (int, error) {
	total := len(p)

	for len(p) > 0 {
		newlineIndex := bytes.IndexByte(p, '\n')
		if newlineIndex < 0 {
			_, _ = w.buffer.Write(p)
			return total, nil
		}

		lineEnd := newlineIndex + 1
		_, _ = w.buffer.Write(p[:lineEnd])
		if err := w.flush(); err != nil {
			return 0, err
		}
		p = p[lineEnd:]
	}

	return total, nil
}

func (w *prefixedLineWriter) Flush() error {
	if w.buffer.Len() == 0 {
		return nil
	}

	return w.flush()
}

func (w *prefixedLineWriter) flush() error {
	prefixedOutputMu.Lock()
	defer prefixedOutputMu.Unlock()

	if _, err := io.WriteString(w.dest, w.prefix); err != nil {
		return err
	}
	if _, err := w.dest.Write(w.buffer.Bytes()); err != nil {
		return err
	}

	w.buffer.Reset()
	return nil
}

func containerLogPrefix(buildName, containerID string) string {
	if buildName != "" {
		return fmt.Sprintf("[%s:%s] ", buildName, shortContainerID(containerID))
	}
	return fmt.Sprintf("[%s] ", shortContainerID(containerID))
}

func shortContainerID(containerID string) string {
	if len(containerID) <= 12 {
		return containerID
	}
	return containerID[:12]
}

func runStreamingCommand(
	ctx context.Context,
	command string,
	buildName string,
	containerID string,
	args ...string,
) error {
	execCmd := exec.CommandContext(ctx, command, args...)
	prefix := containerLogPrefix(buildName, containerID)
	stdoutWriter := newPrefixedLineWriter(os.Stdout, prefix)
	stderrWriter := newPrefixedLineWriter(os.Stderr, prefix)

	execCmd.Stdout = stdoutWriter
	execCmd.Stderr = stderrWriter

	runErr := execCmd.Run()
	stdoutErr := stdoutWriter.Flush()
	stderrErr := stderrWriter.Flush()

	if runErr != nil {
		return runErr
	}
	if stdoutErr != nil {
		return stdoutErr
	}
	if stderrErr != nil {
		return stderrErr
	}

	return nil
}
