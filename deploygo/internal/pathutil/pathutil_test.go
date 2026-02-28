package pathutil

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResolveProjectPathRejectsEscapes(t *testing.T) {
	projectDir := t.TempDir()

	_, err := ResolveProjectPath(projectDir, "../secrets", false)
	if err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}

	_, err = ResolveProjectPath(projectDir, "/tmp/evil", false)
	if err == nil {
		t.Fatalf("expected absolute path to be rejected")
	}
}

func TestResolveProjectPathRejectsNormalizedEscapePath(t *testing.T) {
	projectDir := t.TempDir()

	_, err := ResolveProjectPath(projectDir, "./a/../../b", true)
	if err == nil {
		t.Fatalf("expected normalized escape path to be rejected")
	}
}

func TestResolveProjectPathAllowsNormalizedInProjectPath(t *testing.T) {
	projectDir := t.TempDir()

	got, err := ResolveProjectPath(projectDir, "./a/../b", true)
	t.Logf("got=%v", got)
	if err != nil {
		t.Fatalf("expected normalized in-project path to be allowed, got error: %v", err)
	}
	want := filepath.Join(projectDir, "b")
	if got != want {
		t.Fatalf("unexpected resolved path, got %q want %q", got, want)
	}
}

func TestGlobFilesResolvesProjectRelativePatterns(t *testing.T) {
	projectDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "src", "a.go"), []byte("package src"), 0644); err != nil {
		t.Fatal(err)
	}

	rootMatches, err := GlobFiles("*.go", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(rootMatches, "main.go") {
		t.Fatalf("expected main.go in matches, got %v", rootMatches)
	}

	srcMatches, err := GlobFiles("src/*.go", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(srcMatches, "src/a.go") {
		t.Fatalf("expected src/a.go in matches, got %v", srcMatches)
	}

	dirMatches, err := GlobFiles("src/", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Contains(dirMatches, "src") {
		t.Fatalf("expected src directory match, got %v", dirMatches)
	}

	_, err = GlobFiles("missing/*.go", projectDir)
	if err == nil {
		t.Fatalf("expected no-match pattern to return error")
	}

	_, err = GlobFiles("./a/../../../b", projectDir)
	if err == nil {
		t.Fatalf("expected traversal-like pattern to return error")
	}
	if !strings.Contains(err.Error(), "escapes project directory") {
		t.Fatalf("expected traversal rejection error, got %v", err)
	}
}
