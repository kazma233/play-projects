package cmd

import (
	"bytes"
	"deploygo/internal/config"
	"deploygo/internal/fileutil"
	"errors"
	"strings"
	"testing"
)

func TestChooseProjectNameKeepsExplicitProject(t *testing.T) {
	selected, err := chooseProjectName("demo", fileutil.WorkspaceDir, func(string) ([]config.ConfigInfo, error) {
		return nil, errors.New("should not be called")
	}, true, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if selected != "demo" {
		t.Fatalf("expected demo, got %s", selected)
	}
}

func TestChooseProjectNameNoProjects(t *testing.T) {
	_, err := chooseProjectName("", fileutil.WorkspaceDir, func(string) ([]config.ConfigInfo, error) {
		return nil, nil
	}, true, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no projects found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChooseProjectNameAutoSelectsSingleProject(t *testing.T) {
	var output bytes.Buffer

	selected, err := chooseProjectName("", fileutil.WorkspaceDir, func(string) ([]config.ConfigInfo, error) {
		return []config.ConfigInfo{{DirName: "demo"}}, nil
	}, true, strings.NewReader(""), &output)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if selected != "demo" {
		t.Fatalf("expected demo, got %s", selected)
	}
	if !strings.Contains(output.String(), "Using project: demo") {
		t.Fatalf("expected auto selection output, got %q", output.String())
	}
}

func TestChooseProjectNameErrorsInNonInteractiveMode(t *testing.T) {
	_, err := chooseProjectName("", fileutil.WorkspaceDir, func(string) ([]config.ConfigInfo, error) {
		return []config.ConfigInfo{{DirName: "alpha"}, {DirName: "beta"}}, nil
	}, false, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "project is required in non-interactive mode") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Fatalf("expected project list in error, got %v", err)
	}
}

func TestChooseProjectNamePromptsForSelection(t *testing.T) {
	var output bytes.Buffer

	selected, err := chooseProjectName("", fileutil.WorkspaceDir, func(string) ([]config.ConfigInfo, error) {
		return []config.ConfigInfo{{DirName: "alpha"}, {DirName: "beta"}}, nil
	}, true, strings.NewReader("2\n"), &output)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if selected != "beta" {
		t.Fatalf("expected beta, got %s", selected)
	}
	if !strings.Contains(output.String(), "Available projects:") {
		t.Fatalf("expected prompt output, got %q", output.String())
	}
	if !strings.Contains(output.String(), "Using project: beta") {
		t.Fatalf("expected selection confirmation, got %q", output.String())
	}
}

func TestChooseProjectNameRepromptsAfterInvalidSelection(t *testing.T) {
	var output bytes.Buffer

	selected, err := chooseProjectName("", fileutil.WorkspaceDir, func(string) ([]config.ConfigInfo, error) {
		return []config.ConfigInfo{{DirName: "alpha"}, {DirName: "beta"}}, nil
	}, true, strings.NewReader("9\n1\n"), &output)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if selected != "alpha" {
		t.Fatalf("expected alpha, got %s", selected)
	}
	if !strings.Contains(output.String(), "Invalid selection") {
		t.Fatalf("expected invalid selection hint, got %q", output.String())
	}
}
