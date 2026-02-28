package stage

import (
	"deploygo/internal/pathutil"
	"fmt"
	"log"
	"os"

	"deploygo/internal/config"
)

func RunCleanup(cleanup *config.CleanupConfig, projectDir string) error {
	if cleanup == nil {
		return nil
	}

	// 如果enable为false，则不执行清理
	if !cleanup.Enable {
		return nil
	}

	// 总是清理source目录
	defaultDirs := []string{"source"}
	cleanupDirs := append(defaultDirs, cleanup.Dirs...)

	for _, dir := range cleanupDirs {
		targetDir, err := pathutil.ResolveProjectPath(projectDir, dir, false)
		if err != nil {
			return fmt.Errorf("invalid cleanup dir %q: %w", dir, err)
		}
		log.Printf("Removing directory: %s", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove directory %s: %w", targetDir, err)
		}
	}

	return nil
}
