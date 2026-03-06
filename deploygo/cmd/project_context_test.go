package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectConfig(t *testing.T) {
	workspaceDir := t.TempDir()
	projectDir := filepath.Join(workspaceDir, "demo")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project directory: %v", err)
	}

	configPath := filepath.Join(projectDir, "config.yaml")
	configContent := "container:\n  type: docker\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	projectCtx, err := loadProjectConfig("demo", workspaceDir)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if projectCtx.Name != "demo" {
		t.Fatalf("expected project name demo, got %s", projectCtx.Name)
	}
	if projectCtx.ProjectDir != projectDir {
		t.Fatalf("expected project dir %s, got %s", projectDir, projectCtx.ProjectDir)
	}
	if projectCtx.ConfigPath != configPath {
		t.Fatalf("expected config path %s, got %s", configPath, projectCtx.ConfigPath)
	}
	if projectCtx.Config == nil {
		t.Fatal("expected loaded config, got nil")
	}
	if projectCtx.Config.Container.Type != "docker" {
		t.Fatalf("expected container type docker, got %s", projectCtx.Config.Container.Type)
	}
}

func TestLoadProjectConfigMissingFile(t *testing.T) {
	workspaceDir := t.TempDir()

	_, err := loadProjectConfig("demo", workspaceDir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "project 'demo' not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
