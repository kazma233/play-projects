package main

import (
	"path/filepath"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	root := t.TempDir()
	fsys := NewFileSystem(root)

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "root slash",
			input: "/",
			want:  fsys.root,
		},
		{
			name:  "empty path means root",
			input: "",
			want:  fsys.root,
		},
		{
			name:  "normal nested file",
			input: "/docs/readme.txt",
			want:  filepath.Join(fsys.root, "docs", "readme.txt"),
		},
		{
			name:  "clean inside root",
			input: "/docs/../safe.txt",
			want:  filepath.Join(fsys.root, "safe.txt"),
		},
		{
			name:  "double leading slash stays inside root",
			input: "//safe/file.txt",
			want:  filepath.Join(fsys.root, "safe", "file.txt"),
		},
		{
			name:  "dotdot prefix in filename allowed",
			input: "/..backup/file.txt",
			want:  filepath.Join(fsys.root, "..backup", "file.txt"),
		},
		{
			name:    "reject parent traversal",
			input:   "../escape.txt",
			wantErr: true,
		},
		{
			name:    "reject traversal with leading slash",
			input:   "/../escape.txt",
			wantErr: true,
		},
		{
			name:    "reject windows traversal",
			input:   `\\..\\escape.txt`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fsys.normalizePath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got path %q", tc.input, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}

			if got != tc.want {
				t.Fatalf("normalizePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseSingleRange(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		fileSize  int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{
			name:      "explicit start and end",
			header:    "bytes=0-99",
			fileSize:  1000,
			wantStart: 0,
			wantEnd:   99,
		},
		{
			name:      "open ended range",
			header:    "bytes=100-",
			fileSize:  1000,
			wantStart: 100,
			wantEnd:   999,
		},
		{
			name:      "suffix range",
			header:    "bytes=-200",
			fileSize:  1000,
			wantStart: 800,
			wantEnd:   999,
		},
		{
			name:      "suffix larger than file",
			header:    "bytes=-2000",
			fileSize:  1000,
			wantStart: 0,
			wantEnd:   999,
		},
		{
			name:      "end clamps to file size",
			header:    "bytes=900-2000",
			fileSize:  1000,
			wantStart: 900,
			wantEnd:   999,
		},
		{
			name:      "range with spaces",
			header:    "bytes= 10-19",
			fileSize:  1000,
			wantStart: 10,
			wantEnd:   19,
		},
		{
			name:     "invalid unit",
			header:   "items=0-1",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "empty range spec",
			header:   "bytes=",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "multiple ranges unsupported",
			header:   "bytes=0-1,2-3",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "invalid start",
			header:   "bytes=abc-10",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "invalid end",
			header:   "bytes=10-abc",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "start beyond file size",
			header:   "bytes=1000-1001",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "end before start",
			header:   "bytes=50-10",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "invalid suffix zero",
			header:   "bytes=-0",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "dash only",
			header:   "bytes=-",
			fileSize: 1000,
			wantErr:  true,
		},
		{
			name:     "empty file invalid",
			header:   "bytes=0-1",
			fileSize: 0,
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStart, gotEnd, err := parseSingleRange(tc.header, tc.fileSize)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for header %q and fileSize %d, got %d-%d", tc.header, tc.fileSize, gotStart, gotEnd)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for header %q and fileSize %d: %v", tc.header, tc.fileSize, err)
			}

			if gotStart != tc.wantStart || gotEnd != tc.wantEnd {
				t.Fatalf("parseSingleRange(%q, %d) = %d-%d, want %d-%d", tc.header, tc.fileSize, gotStart, gotEnd, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
