package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWithin 在基础目录内安全解析相对路径，防止路径遍历攻击（path traversal）。
// 它将相对路径与基础目录拼接，然后验证最终路径是否仍在基础目录内。
//
// 参数：
//   - baseDir: 基础目录的路径（可以是相对路径或绝对路径）
//   - relPath: 相对于 baseDir 的目标路径
//
// 返回值：
//   - 解析后的绝对路径（如果安全）
//   - 错误（如果路径为空、是绝对路径、或逃逸出基础目录）
func ResolveWithin(baseDir, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute path is not allowed: %s", relPath)
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}
	baseAbs = filepath.Clean(baseAbs)

	targetAbs := filepath.Clean(filepath.Join(baseAbs, relPath))
	if !IsWithin(baseAbs, targetAbs) {
		return "", fmt.Errorf("path escapes config directory: %s", relPath)
	}

	return targetAbs, nil
}

// EnsurePatternWithin 验证文件匹配模式（如 "subdir/*.yaml"）是否在基础目录内。
// 它提取模式中的目录部分并调用 ResolveWithin 进行验证。
//
// 参数：
//   - baseDir: 基础目录的路径
//   - pattern: 文件匹配模式（如 "dir/*.json"）
//
// 返回值：
//   - 错误（如果模式为空、是绝对路径、或目录部分逃逸出基础目录）
func EnsurePatternWithin(baseDir, pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern is empty")
	}
	if filepath.IsAbs(pattern) {
		return fmt.Errorf("absolute pattern is not allowed: %s", pattern)
	}

	patternDir := filepath.Dir(pattern)
	if patternDir == "." {
		return nil
	}

	_, err := ResolveWithin(baseDir, patternDir)
	return err
}

// IsWithin 判断目标绝对路径是否在基础目录内。
// 使用 filepath.Rel 计算相对路径：如果结果以 ".." 开头，则表示路径逃逸。
//
// 参数：
//   - baseAbs: 基础目录的绝对路径（应已清理）
//   - targetAbs: 目标文件的绝对路径（应已清理）
//
// 返回值：
//   - true: targetAbs 在 baseAbs 目录内
//   - false: targetAbs 不在 baseAbs 目录内或发生错误
func IsWithin(baseAbs, targetAbs string) bool {
	baseClean := filepath.Clean(baseAbs)
	targetClean := filepath.Clean(targetAbs)

	rel, err := filepath.Rel(baseClean, targetClean)
	if err != nil {
		return false
	}

	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
