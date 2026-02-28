package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithin(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{"normal relative path", "subdir/file.txt", false},
		{"simple filename", "file.txt", false},
		{"nested path", "a/b/c/d.txt", false},
		{"dot-slash prefix", "./subdir/file.txt", false},
		{"traversal with ../", "../outside", true},
		{"traversal nested", "subdir/../../outside", true},
		{"absolute path", "/etc/passwd", true},
		{"empty path", "", true},
		{"double traversal", "../../etc/passwd", true},
		{"traversal after valid", "valid/../../../etc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveWithin(baseDir, tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for path %q, got result %q", tt.rel, result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for path %q: %v", tt.rel, err)
				}
				abs, _ := filepath.Abs(baseDir)
				if !IsWithin(abs, result) {
					t.Errorf("result %q is not within base %q", result, abs)
				}
			}
		})
	}
}

func TestEnsurePatternWithin(t *testing.T) {
	baseDir := t.TempDir()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"simple glob", "*.txt", false},
		{"subdir glob", "src/*.go", false},
		{"nested glob", "src/pkg/**/*.go", false},
		{"traversal glob", "../*.txt", true},
		{"traversal nested glob", "src/../../*.go", true},
		{"absolute glob", "/etc/*.conf", true},
		{"empty pattern", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsurePatternWithin(baseDir, tt.pattern)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for pattern %q", tt.pattern)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for pattern %q: %v", tt.pattern, err)
			}
		})
	}
}

func TestIsWithin(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		target string
		want   bool
	}{
		{"same dir", "/project", "/project", true},
		{"subdir", "/project", "/project/sub", true},
		{"deeply nested", "/project", "/project/a/b/c", true},
		{"parent dir", "/project", "/project/..", false},
		{"sibling dir", "/project", "/other", false},
		{"partial prefix", "/project", "/projectx/sub", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWithin(tt.base, tt.target)
			if got != tt.want {
				t.Errorf("IsWithin(%q, %q) = %v, want %v", tt.base, tt.target, got, tt.want)
			}
		})
	}
}

func TestResolveWithinCreatesCorrectAbsPath(t *testing.T) {
	baseDir := t.TempDir()
	// Create a subdirectory to ensure the resolved path is correct
	subdir := filepath.Join(baseDir, "sub")
	os.MkdirAll(subdir, 0755)

	result, err := ResolveWithin(baseDir, "sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(baseDir, "sub")
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
