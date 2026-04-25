package container

import (
	"bytes"
	"testing"
)

type recordingWriter struct {
	writes [][]byte
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.writes = append(w.writes, append([]byte(nil), p...))
	return len(p), nil
}

func TestPrefixedLineWriterPrefixesEachLine(t *testing.T) {
	var buf bytes.Buffer
	writer := newPrefixedLineWriter(&buf, ">>> [docker:abc123] ")

	if _, err := writer.Write([]byte("hello\nworld\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	want := ">>> [docker:abc123] hello\n>>> [docker:abc123] world\n"
	if got := buf.String(); got != want {
		t.Fatalf("prefixed output = %q, want %q", got, want)
	}
}

func TestPrefixedLineWriterFlushesPartialLine(t *testing.T) {
	var buf bytes.Buffer
	writer := newPrefixedLineWriter(&buf, ">>> [docker:abc123] ")

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	want := ">>> [docker:abc123] hello"
	if got := buf.String(); got != want {
		t.Fatalf("prefixed output = %q, want %q", got, want)
	}
}

func TestContainerLogPrefixUsesShortContainerID(t *testing.T) {
	got := containerLogPrefix("", "1234567890abcdef")
	want := "[1234567890ab] "
	if got != want {
		t.Fatalf("containerLogPrefix() = %q, want %q", got, want)
	}
}

func TestContainerLogPrefixIncludesBuildName(t *testing.T) {
	got := containerLogPrefix("frontend", "1234567890abcdef")
	want := "[frontend:1234567890ab] "
	if got != want {
		t.Fatalf("containerLogPrefix() = %q, want %q", got, want)
	}
}

func TestPrefixedLineWriterFlushWritesSingleCombinedLine(t *testing.T) {
	dest := &recordingWriter{}
	writer := newPrefixedLineWriter(dest, "[frontend:1234567890ab] ")

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if len(dest.writes) != 1 {
		t.Fatalf("write count = %d, want 1", len(dest.writes))
	}

	want := "[frontend:1234567890ab] hello"
	if got := string(dest.writes[0]); got != want {
		t.Fatalf("written output = %q, want %q", got, want)
	}
}
