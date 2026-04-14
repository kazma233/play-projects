package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"deploygo/internal/config"
	"deploygo/internal/fileutil"
)

type projectContext struct {
	Name       string
	ProjectDir string
	ConfigPath string
	Config     *config.Config
}

func loadSelectedProjectConfig() (*projectContext, error) {
	if err := ensureProjectSelected(); err != nil {
		return nil, err
	}

	return loadProjectConfig(projectName, fileutil.WorkspaceDir)
}

func loadProjectConfig(name, workspaceDir string) (*projectContext, error) {
	configPath := filepath.Join(workspaceDir, name, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project '%s' not found (file: %s)", name, configPath)
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat config file '%s': %w", configPath, err)
	}

	cfg, basicPath, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return &projectContext{
		Name:       name,
		ProjectDir: basicPath,
		ConfigPath: configPath,
		Config:     cfg,
	}, nil
}
