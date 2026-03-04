package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	// 计算文件SHA（SHA-256）
	sha := computeSHA(file.Content)

	url := s.buildURL(file.Path)

	slog.Info("文件上传成功", "path", file.Path, "size", len(file.Content))

	return &UploadResult{
		Path: file.Path,
		URL:  url,
		SHA:  sha,
	}, nil
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

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
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

		fileSHA, err := computeFileSHA(filePath)
		if err != nil {
			return fmt.Errorf("计算文件SHA失败: %w", err)
		}

		files = append(files, &RepositoryFile{
			Path:        relPath,
			SHA:         fileSHA,
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

// computeSHA 计算内容的 SHA-256
func computeSHA(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func computeFileSHA(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
