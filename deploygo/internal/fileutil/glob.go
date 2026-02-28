package fileutil

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// GlobFiles 使用 glob 模式匹配文件，并为每个匹配项调用 fn 函数。
// 如果 pattern 不是有效的 glob 模式，则直接将 pattern 作为参数调用 fn。
// 如果 pattern 以 "/" 结尾且是一个有效的目录，则将该目录路径作为参数调用 fn。
func GlobFiles(pattern, baseDir string, fn func(string) error) error {
	normalizedPattern := filepath.ToSlash(pattern)

	// 处理目录模式（以 "/" 结尾）
	if strings.HasSuffix(normalizedPattern, "/") {
		fullPath := filepath.Join(baseDir, normalizedPattern)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			return fn(normalizedPattern)
		}
	}

	// 编译 glob 模式
	g, err := glob.Compile(normalizedPattern)
	if err != nil {
		return fn(pattern)
	}

	// 对于 ** 模式，同时编译去掉 **/ 前缀的版本，用于匹配根目录下的文件
	var rootGlob glob.Glob
	if strings.HasPrefix(normalizedPattern, "**/") {
		rootPattern := strings.TrimPrefix(normalizedPattern, "**/")
		rootGlob, _ = glob.Compile(rootPattern)
	}

	// 确定目录深度限制
	// 像 "src/*.go" 这样的模式应该只匹配直接子文件，而不是 "src/lib/*.go"
	slashCount := strings.Count(normalizedPattern, "/")
	hasDoubleStar := strings.Contains(normalizedPattern, "**")

	// 遍历目录树并匹配文件
	matched := false
	err = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		// 对非 ** 模式检查深度限制
		if !hasDoubleStar {
			actualSlashCount := strings.Count(relPath, "/")
			// 模式 "src/*.go" 有 1 个斜杠，应该只匹配恰好有 1 个斜杠的文件
			// （即 src/ 的直接子文件）
			if actualSlashCount != slashCount {
				return nil
			}
		}

		// 尝试使用编译好的 glob 进行匹配
		if g.Match(relPath) {
			matched = true
			if err := fn(relPath); err != nil {
				return err
			}
		} else if rootGlob != nil && rootGlob.Match(relPath) {
			// 对于 ** 模式，也尝试去掉 **/ 前缀进行匹配
			// 这样可以处理根目录下的文件（如 "main.go"）匹配 "**/*.go" 的情况
			matched = true
			if err := fn(relPath); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 如果没有文件匹配，则使用原始模式调用 fn（回退行为）
	if !matched {
		return fn(pattern)
	}
	return nil
}
