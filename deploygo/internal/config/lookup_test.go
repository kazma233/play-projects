package config

import "testing"

func TestConfigFindBuild(t *testing.T) {
	cfg := &Config{
		Builds: []StageConfig{
			{Name: "frontend"},
			{Name: "backend"},
		},
	}

	tests := []struct {
		name   string
		build  string
		want   string
		wantOK bool
	}{
		{name: "find existing build", build: "backend", want: "backend", wantOK: true},
		{name: "missing build", build: "worker", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := cfg.FindBuild(tt.build)
			if tt.wantOK && build == nil {
				t.Fatalf("FindBuild(%q) = nil, want build", tt.build)
			}
			if !tt.wantOK && build != nil {
				t.Fatalf("FindBuild(%q) = %#v, want nil", tt.build, build)
			}
			if tt.wantOK && build.Name != tt.want {
				t.Fatalf("FindBuild(%q) name = %q, want %q", tt.build, build.Name, tt.want)
			}
		})
	}
}

func TestConfigFindDeployStep(t *testing.T) {
	cfg := &Config{
		Deploys: []DeploymentStep{
			{Name: "upload"},
			{Name: "restart"},
		},
	}

	tests := []struct {
		name   string
		step   string
		want   string
		wantOK bool
	}{
		{name: "find existing deploy step", step: "restart", want: "restart", wantOK: true},
		{name: "missing deploy step", step: "cleanup", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := cfg.FindDeployStep(tt.step)
			if tt.wantOK && step == nil {
				t.Fatalf("FindDeployStep(%q) = nil, want step", tt.step)
			}
			if !tt.wantOK && step != nil {
				t.Fatalf("FindDeployStep(%q) = %#v, want nil", tt.step, step)
			}
			if tt.wantOK && step.Name != tt.want {
				t.Fatalf("FindDeployStep(%q) name = %q, want %q", tt.step, step.Name, tt.want)
			}
		})
	}
}

func TestConfigFindServer(t *testing.T) {
	cfg := &Config{
		Servers: map[string]ServerConfig{
			"staging": {Host: "staging.example.com"},
			"prod":    {Host: "prod.example.com"},
		},
	}

	tests := []struct {
		name   string
		server string
		want   string
		wantOK bool
	}{
		{name: "find existing server", server: "prod", want: "prod.example.com", wantOK: true},
		{name: "missing server", server: "dev", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := cfg.FindServer(tt.server)
			if tt.wantOK && server == nil {
				t.Fatalf("FindServer(%q) = nil, want server", tt.server)
			}
			if !tt.wantOK && server != nil {
				t.Fatalf("FindServer(%q) = %#v, want nil", tt.server, server)
			}
			if tt.wantOK && server.Host != tt.want {
				t.Fatalf("FindServer(%q) host = %q, want %q", tt.server, server.Host, tt.want)
			}
		})
	}
}

func TestConfigFindMethodsHandleNilReceiver(t *testing.T) {
	var cfg *Config

	if build := cfg.FindBuild("frontend"); build != nil {
		t.Fatalf("FindBuild on nil receiver = %#v, want nil", build)
	}

	if step := cfg.FindDeployStep("upload"); step != nil {
		t.Fatalf("FindDeployStep on nil receiver = %#v, want nil", step)
	}

	if server := cfg.FindServer("prod"); server != nil {
		t.Fatalf("FindServer on nil receiver = %#v, want nil", server)
	}
}
