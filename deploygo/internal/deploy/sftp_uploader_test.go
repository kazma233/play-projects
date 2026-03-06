package deploy

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteTempPath(t *testing.T) {
	now := time.Unix(0, 123456789)
	tests := []struct {
		name string
		dest string
		want string
	}{
		{
			name: "absolute path",
			dest: "/opt/app/app.tar.gz",
			want: "/opt/app/.app.tar.gz-0.tmp",
		},
		{
			name: "relative path",
			dest: "release/app.tar.gz",
			want: "release/.app.tar.gz-0.tmp",
		},
		{
			name: "long filename",
			dest: "/opt/app/" + strings.Repeat("very-long-name-", 20) + ".tar.gz",
			want: "/opt/app/." + strings.Repeat("very-long-name-", 20) + ".tar.gz-0.tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remoteTempPath(tt.dest, now)
			if got != tt.want {
				t.Fatalf("remoteTempPath(%q) = %q, want %q", tt.dest, got, tt.want)
			}

			if filepath.Dir(got) != filepath.Dir(tt.dest) {
				t.Fatalf("expected temp path to stay in same directory, got %q for %q", got, tt.dest)
			}

			if !strings.HasPrefix(filepath.Base(got), ".") {
				t.Fatalf("expected temp file to be hidden, got %q", got)
			}

			if !strings.HasSuffix(filepath.Base(got), ".tmp") {
				t.Fatalf("expected temp file to use .tmp suffix, got %q", got)
			}
		})
	}
}
