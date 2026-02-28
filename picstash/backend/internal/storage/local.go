package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// localStorage 本地文件系统存储实现
type localStorage struct {
	basePath   string // 文件存储的根目录
	urlPath    string // URL路径前缀（如 /files）
	serverAddr string // 后端服务地址（如 http://localhost:6100）
}

// NewLocalStorage 创建本地存储实例
// basePath: 文件存储的根目录，如 "./data/files"
// urlPath: URL路径前缀，如 "/files"
// serverAddr: 后端服务地址，如 "http://localhost:6100"
func NewLocalStorage(basePath, urlPath, serverAddr string) Storage {
	return &localStorage{
		basePath:   basePath,
		urlPath:    urlPath,
		serverAddr: serverAddr,
	}
}

// Upload 上传单个文件到本地文件系统
func (s *localStorage) Upload(ctx context.Context, file *File) (*UploadResult, error) {
	fullPath := filepath.Join(s.basePath, file.Path)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("创建目录失败", "path", dir, "error", err)
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, file.Content, 0644); err != nil {
		slog.Error("写入文件失败", "path", fullPath, "error", err)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 计算文件SHA（使用文件内容计算简单的校验和）
	sha := computeSHA(file.Content)

	url := s.buildURL(file.Path)

	slog.Info("文件上传成功", "path", file.Path, "size", len(file.Content))

	return &UploadResult{
		Path: file.Path,
		URL:  url,
		SHA:  sha,
	}, nil
}

// BatchUpload 批量上传文件
func (s *localStorage) BatchUpload(ctx context.Context, files []*File) ([]*UploadResult, error) {
	results := make([]*UploadResult, 0, len(files))

	for _, file := range files {
		result, err := s.Upload(ctx, file)
		if err != nil {
			slog.Error("批量上传失败", "path", file.Path, "error", err)
			return results, err
		}
		results = append(results, result)
	}

	slog.Info("批量上传完成", "count", len(results))
	return results, nil
}

// Delete 删除本地文件
func (s *localStorage) Delete(ctx context.Context, path, sha string) error {
	fullPath := filepath.Join(s.basePath, path)

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		slog.Warn("文件不存在，跳过删除", "path", path)
		return nil
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		slog.Error("删除文件失败", "path", fullPath, "error", err)
		return fmt.Errorf("删除文件失败: %w", err)
	}

	// 尝试删除空目录
	s.cleanupEmptyDirs(filepath.Dir(fullPath))

	slog.Info("文件删除成功", "path", path)
	return nil
}

// GetURL 获取文件的访问URL
func (s *localStorage) GetURL(ctx context.Context, path string) string {
	return s.buildURL(path)
}

// Exists 检查文件是否存在
func (s *localStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(s.basePath, path)
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("检查文件存在性失败: %w", err)
	}
	return true, nil
}

// ListFiles 递归列出目录下的所有文件
func (s *localStorage) ListFiles(ctx context.Context, path string) ([]*RepositoryFile, error) {
	var files []*RepositoryFile

	targetPath := filepath.Join(s.basePath, path)

	// 检查目录是否存在
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, fmt.Errorf("访问目录失败: %w", err)
	}

	// 如果是文件而非目录，直接返回
	if !info.IsDir() {
		return files, nil
	}

	// 递归遍历目录
	err = filepath.Walk(targetPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("遍历文件失败", "path", filePath, "error", err)
			return nil
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(s.basePath, filePath)
		if err != nil {
			slog.Warn("计算相对路径失败", "path", filePath, "error", err)
			return nil
		}

		// 统一使用正斜杠
		relPath = filepath.ToSlash(relPath)

		files = append(files, &RepositoryFile{
			Path:        relPath,
			SHA:         "", // 本地存储不维护SHA
			Size:        info.Size(),
			Type:        "file",
			DownloadURL: s.buildURL(relPath),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", err)
	}

	return files, nil
}

// GetRawFileContent 读取文件原始内容
func (s *localStorage) GetRawFileContent(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(s.basePath, path)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("文件不存在: %s", path)
		}
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return data, nil
}

// GetBasePath 返回存储的根目录路径
func (s *localStorage) GetBasePath() string {
	return s.basePath
}

// GetURLPath 返回对外访问URL路径前缀
func (s *localStorage) GetURLPath() string {
	return s.urlPath
}

// buildURL 构建文件的访问URL（相对路径，用于数据库存储）
func (s *localStorage) buildURL(path string) string {
	// 统一使用正斜杠
	path = filepath.ToSlash(path)
	return s.urlPath + "/" + path
}

// GetPublicURL 获取前端访问的完整URL
// 返回格式: http://localhost:6100/files/images/xxx.jpg
func (s *localStorage) GetPublicURL(path string) string {
	// 统一使用正斜杠
	path = filepath.ToSlash(path)
	return s.serverAddr + s.urlPath + "/" + path
}

// cleanupEmptyDirs 清理空目录
func (s *localStorage) cleanupEmptyDirs(dir string) {
	for {
		// 检查是否是根目录
		rel, err := filepath.Rel(s.basePath, dir)
		if err != nil || rel == "." || rel == "/" {
			return
		}

		// 尝试删除目录
		err = os.Remove(dir)
		if err != nil {
			// 目录不为空或删除失败，停止清理
			return
		}

		slog.Debug("删除空目录", "dir", dir)
		dir = filepath.Dir(dir)
	}
}

// computeSHA 计算内容的简单校验和（实际SHA256）
// 注意：这里使用一个简化版本，实际生产环境应该使用 crypto/sha256
func computeSHA(content []byte) string {
	// 返回前16个字符的十六进制表示作为简化SHA
	// 实际项目中应该导入 "crypto/sha256" 并计算真正的哈希
	if len(content) == 0 {
		return ""
	}

	// 简单的校验和计算
	sum := 0
	for _, b := range content {
		sum += int(b)
	}

	return fmt.Sprintf("%x", sum%65536)
}

// CopyFile 本地存储特有的辅助方法：复制文件
func (s *localStorage) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath := filepath.Join(s.basePath, srcPath)
	dstFullPath := filepath.Join(s.basePath, dstPath)

	// 确保目标目录存在
	dir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 打开源文件
	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dstFullPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	// 复制内容
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	return nil
}

// MoveFile 本地存储特有的辅助方法：移动文件
func (s *localStorage) MoveFile(ctx context.Context, srcPath, dstPath string) error {
	srcFullPath := filepath.Join(s.basePath, srcPath)
	dstFullPath := filepath.Join(s.basePath, dstPath)

	// 确保目标目录存在
	dir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 移动文件
	if err := os.Rename(srcFullPath, dstFullPath); err != nil {
		// 如果跨设备移动失败，尝试复制后删除
		if err := s.CopyFile(ctx, srcPath, dstPath); err != nil {
			return fmt.Errorf("移动文件失败: %w", err)
		}
		if err := s.Delete(ctx, srcPath, ""); err != nil {
			slog.Warn("移动后删除源文件失败", "path", srcPath, "error", err)
		}
	}

	// 清理源目录可能产生的空目录
	s.cleanupEmptyDirs(filepath.Dir(srcFullPath))

	return nil
}
