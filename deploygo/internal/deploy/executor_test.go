package deploy

import (
	"errors"
	"net"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIsRetryableSSHError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "network timeout",
			err: &net.DNSError{
				Err:       "i/o timeout",
				IsTimeout: true,
			},
			want: true,
		},
		{
			name: "wrapped connection reset",
			err:  errors.New("dial tcp: connection reset by peer"),
			want: true,
		},
		{
			name: "syscall connection refused",
			err:  syscall.ECONNREFUSED,
			want: true,
		},
		{
			name: "authentication failure",
			err:  errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"),
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("permission denied"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableSSHError(tt.err)
			if got != tt.want {
				t.Fatalf("isRetryableSSHError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestBuildAuthMethods(t *testing.T) {
	tests := []struct {
		name    string
		cfg     connectionConfig
		wantLen int
		wantErr bool
	}{
		{
			name: "password auth",
			cfg: connectionConfig{
				Password: "secret",
			},
			wantLen: 1,
		},
		{
			name:    "missing auth",
			cfg:     connectionConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methods, err := buildAuthMethods(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(methods) != tt.wantLen {
				t.Fatalf("expected %d auth methods, got %d", tt.wantLen, len(methods))
			}
		})
	}
}

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
			want: "/opt/app/.app.tar.gz.deploygo-upload-123456789.tmp",
		},
		{
			name: "relative path",
			dest: "release/app.tar.gz",
			want: "release/.app.tar.gz.deploygo-upload-123456789.tmp",
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
		})
	}
}
