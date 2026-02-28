package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create test file structure
	files := []string{
		"main.go",
		"utils.go",
		"README.md",
		"config.yaml",
		"src/app.go",
		"src/lib/helper.go",
		"src/lib/utils.go",
		"test/main_test.go",
		"configs/dev.yaml",
		"configs/prod.yaml",
		"docs/guide.md",
		"docs/api/reference.md",
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", path, err)
		}
	}

	return tmpDir
}

func TestGlobFiles_SimpleWildcard(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("*.go", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(matches)
	expected := []string{"main.go", "utils.go"}
	if len(matches) != len(expected) {
		t.Errorf("expected %v, got %v", expected, matches)
	}
	for i, exp := range expected {
		if i >= len(matches) || matches[i] != exp {
			t.Errorf("expected %s at position %d, got %v", exp, i, matches)
		}
	}
}

func TestGlobFiles_DoubleStar(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("**/*.go", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(matches)
	expected := []string{
		"main.go",
		"src/app.go",
		"src/lib/helper.go",
		"src/lib/utils.go",
		"test/main_test.go",
		"utils.go",
	}
	if len(matches) != len(expected) {
		t.Errorf("expected %d matches, got %d: %v", len(expected), len(matches), matches)
	}
	for i, exp := range expected {
		if i >= len(matches) || matches[i] != exp {
			t.Errorf("expected %s at position %d, got %v", exp, i, matches)
		}
	}
}

func TestGlobFiles_DirectoryPattern(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("src/*.go", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(matches) != 1 || matches[0] != "src/app.go" {
		t.Errorf("expected [src/app.go], got %v", matches)
	}
}

func TestGlobFiles_DirectorySuffix(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("configs/", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(matches) != 1 || matches[0] != "configs/" {
		t.Errorf("expected [configs/], got %v", matches)
	}
}

func TestGlobFiles_InvalidGlobFallback(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	invalidPattern := "[invalid"
	err := GlobFiles(invalidPattern, baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid glob should fall back to calling fn with original pattern
	if len(matches) != 1 || matches[0] != invalidPattern {
		t.Errorf("expected [%s] as fallback, got %v", invalidPattern, matches)
	}
}

func TestGlobFiles_NoMatchFallback(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("*.nonexistent", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No matches should fall back to calling fn with original pattern
	if len(matches) != 1 || matches[0] != "*.nonexistent" {
		t.Errorf("expected [*.nonexistent] as fallback, got %v", matches)
	}
}

func TestGlobFiles_CallbackError(t *testing.T) {
	baseDir := setupTestDir(t)

	testErr := errors.New("callback error")
	err := GlobFiles("*.go", baseDir, func(path string) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
}

func TestGlobFiles_NestedDoubleStar(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("**/*.yaml", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(matches)
	expected := []string{
		"config.yaml",
		"configs/dev.yaml",
		"configs/prod.yaml",
	}
	if len(matches) != len(expected) {
		t.Errorf("expected %d matches, got %d: %v", len(expected), len(matches), matches)
	}
	for i, exp := range expected {
		if i >= len(matches) || matches[i] != exp {
			t.Errorf("expected %s at position %d, got %v", exp, i, matches)
		}
	}
}

func TestGlobFiles_MarkdownFiles(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("**/*.md", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(matches)
	expected := []string{
		"README.md",
		"docs/api/reference.md",
		"docs/guide.md",
	}
	if len(matches) != len(expected) {
		t.Errorf("expected %d matches, got %d: %v", len(expected), len(matches), matches)
	}
	for i, exp := range expected {
		if i >= len(matches) || matches[i] != exp {
			t.Errorf("expected %s at position %d, got %v", exp, i, matches)
		}
	}
}

func TestGlobFiles_SingleFile(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	err := GlobFiles("config.yaml", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(matches) != 1 || matches[0] != "config.yaml" {
		t.Errorf("expected [config.yaml], got %v", matches)
	}
}

func TestGlobFiles_NonExistentDirectorySuffix(t *testing.T) {
	baseDir := setupTestDir(t)

	var matches []string
	// Directory doesn't exist, so it should fall back to glob matching
	err := GlobFiles("nonexistent/", baseDir, func(path string) error {
		matches = append(matches, path)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Since no files match "nonexistent/", it falls back to calling fn with pattern
	if len(matches) != 1 || matches[0] != "nonexistent/" {
		t.Errorf("expected [nonexistent/] as fallback, got %v", matches)
	}
}
