package pathutil

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// ResolveProjectPath 将用户输入的相对路径解析到 projectDir 下。
// 约束：
// 1. 禁止绝对路径
// 2. 禁止通过 .. 越界到 projectDir 外
// 3. allowProjectRoot=false 时，禁止解析为项目根目录本身
func ResolveProjectPath(projectDir, rel string, allowProjectRoot bool) (string, error) {
	if rel == "" {
		if allowProjectRoot {
			return filepath.Clean(projectDir), nil
		}
		return "", fmt.Errorf("path must not be empty")
	}

	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path is not allowed: %q", rel)
	}

	// 先做一次规范化，消除 "./"、重复分隔符等噪声。
	cleanRel := filepath.Clean(rel)
	if cleanRel == "." {
		if allowProjectRoot {
			return filepath.Clean(projectDir), nil
		}
		return "", fmt.Errorf("project root path is not allowed: %q", rel)
	}

	// 快速拦截显式越界场景。
	parentPrefix := ".." + string(filepath.Separator)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, parentPrefix) {
		return "", fmt.Errorf("path escapes project directory: %q", rel)
	}

	baseAbs, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project directory: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, cleanRel))
	if err != nil {
		return "", fmt.Errorf("failed to resolve target path: %w", err)
	}

	// 再做一次相对关系校验，防止 Join/Clean 后出现隐式越界。
	relToBase, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	if relToBase == ".." || strings.HasPrefix(relToBase, parentPrefix) {
		return "", fmt.Errorf("path escapes project directory: %q", rel)
	}
	if relToBase == "." && !allowProjectRoot {
		return "", fmt.Errorf("project root path is not allowed: %q", rel)
	}

	return targetAbs, nil
}

// GlobFiles 在 projectDir 内展开相对 glob pattern，并返回匹配到的相对路径列表。
// 约束：
// 1. pattern 必须是相对路径
// 2. pattern 不允许越界（例如 ./a/../../../b）
// 3. 未匹配到任何文件会返回错误，避免静默回退
func GlobFiles(pattern, projectDir string) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("pattern must not be empty")
	}
	if filepath.IsAbs(pattern) {
		return nil, fmt.Errorf("absolute path is not allowed: %q", pattern)
	}

	// 规范化后检查是否越界，覆盖诸如 "./a/../../../b" 这类场景。
	normalizedPattern := filepath.ToSlash(pattern)
	cleanPattern := filepath.ToSlash(filepath.Clean(normalizedPattern))
	if cleanPattern == ".." || strings.HasPrefix(cleanPattern, "../") {
		return nil, fmt.Errorf("pattern escapes project directory: %q", pattern)
	}

	g, err := glob.Compile(normalizedPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	var matches []string
	// 统一遍历 projectDir，保证匹配基准恒定为“项目相对路径”。
	err = filepath.WalkDir(projectDir, func(fullPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(projectDir, fullPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			// 同时支持 "src/" 和 "src" 两种目录写法。
			if g.Match(rel+"/") || g.Match(rel) {
				matches = append(matches, rel)
			}
			return nil
		}

		if g.Match(rel) {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no files matched pattern %q", pattern)
	}
	return matches, nil
}
