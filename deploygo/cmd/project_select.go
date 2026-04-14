package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"deploygo/internal/config"
	"deploygo/internal/fileutil"
)

type configInfoLoader func(string) ([]config.ConfigInfo, error)

func ensureProjectSelected() error {
	selected, err := chooseProjectName(
		projectName,
		fileutil.WorkspaceDir,
		config.LoadConfigInfo,
		isInteractiveInput(),
		os.Stdin,
		os.Stdout,
	)
	if err != nil {
		return err
	}

	projectName = selected
	return nil
}

func chooseProjectName(currentName, workspaceDir string, load configInfoLoader, interactive bool, in io.Reader, out io.Writer) (string, error) {
	if currentName != "" {
		return currentName, nil
	}

	projects, err := load(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("failed to load projects: %w", err)
	}

	switch len(projects) {
	case 0:
		return "", fmt.Errorf("no projects found in %s/", workspaceDir)
	case 1:
		selected := projects[0].DirName
		fmt.Fprintf(out, "Using project: %s\n", selected)
		return selected, nil
	}

	if !interactive {
		return "", fmt.Errorf("project is required in non-interactive mode; available projects:\n%s", formatProjectList(projects))
	}

	return promptProjectSelection(projects, in, out)
}

func promptProjectSelection(projects []config.ConfigInfo, in io.Reader, out io.Writer) (string, error) {
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "Available projects:")
	for i, project := range projects {
		fmt.Fprintf(out, "  %d) %s\n", i+1, project.DirName)
	}

	for {
		fmt.Fprintf(out, "Select project [1-%d]: ", len(projects))

		line, err := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)

		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read project selection: %w", err)
		}

		index, convErr := strconv.Atoi(trimmed)
		if convErr == nil && index >= 1 && index <= len(projects) {
			selected := projects[index-1].DirName
			fmt.Fprintf(out, "Using project: %s\n", selected)
			return selected, nil
		}

		if err == io.EOF {
			return "", fmt.Errorf("invalid project selection: %q", trimmed)
		}

		fmt.Fprintf(out, "Invalid selection %q, please enter a number between 1 and %d.\n", trimmed, len(projects))
	}
}

func formatProjectList(projects []config.ConfigInfo) string {
	lines := make([]string, 0, len(projects))
	for _, project := range projects {
		lines = append(lines, fmt.Sprintf("  - %s", project.DirName))
	}
	return strings.Join(lines, "\n")
}

func isInteractiveInput() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
