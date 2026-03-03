package main

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileInfo 文件信息
type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"isDir"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
	MimeType string    `json:"mimeType"`
}

// FileSystem 文件系统操作
type FileSystem struct {
	root string
}

// NewFileSystem 创建文件系统操作实例
func NewFileSystem(root string) *FileSystem {
	return &FileSystem{root: root}
}

// normalizePath 规范化路径，防止目录遍历攻击
func (fs *FileSystem) normalizePath(path string) (string, error) {
	// 移除开头的 /
	path = strings.TrimPrefix(path, "/")

	// 清理路径
	path = filepath.Clean(path)

	// 标准化路径分隔符以确保跨平台一致性
	path = strings.ReplaceAll(path, "\\", "/")

	// 确保路径不以 .. 开头或包含 .. 作为路径组件（防止目录遍历）
	if strings.HasPrefix(path, "..") || strings.Contains(path, "/../") || strings.Contains(path, "/..") {
		return "", fmt.Errorf("invalid path")
	}

	// 拼接完整路径
	fullPath := filepath.Join(fs.root, path)

	// 确保路径在根目录内
	cleanFullPath := filepath.Clean(fullPath)
	cleanRoot := filepath.Clean(fs.root)
	if !strings.HasPrefix(cleanFullPath, cleanRoot) {
		return "", fmt.Errorf("path outside root directory")
	}

	return fullPath, nil
}

// ListDirectory 列出目录内容
func (fs *FileSystem) ListDirectory(path string) ([]FileInfo, error) {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		log.Printf("normalizePath error for path %s: %v", path, err)
		return nil, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(path, entry.Name())
		mimeType := ""
		if !entry.IsDir() {
			mimeType = mime.TypeByExtension(filepath.Ext(entry.Name()))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
		}

		files = append(files, FileInfo{
			Name:     entry.Name(),
			Path:     filePath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			MimeType: mimeType,
		})
	}

	return files, nil
}

// CreateDirectory 创建目录
func (fs *FileSystem) CreateDirectory(path string) error {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		log.Printf("normalizePath error for CreateDirectory %s: %v", path, err)
		return err
	}

	return os.MkdirAll(fullPath, 0755)
}

// Delete 删除文件或目录
func (fs *FileSystem) Delete(path string) error {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		log.Printf("normalizePath error for Delete %s: %v", path, err)
		return err
	}

	return os.RemoveAll(fullPath)
}

// Rename 重命名文件或目录
func (fs *FileSystem) Rename(oldPath, newPath string) error {
	oldFullPath, err := fs.normalizePath(oldPath)
	if err != nil {
		log.Printf("normalizePath error for Rename oldPath %s: %v", oldPath, err)
		return err
	}

	newFullPath, err := fs.normalizePath(newPath)
	if err != nil {
		log.Printf("normalizePath error for Rename newPath %s: %v", newPath, err)
		return err
	}

	return os.Rename(oldFullPath, newFullPath)
}

// GetFile 获取文件信息
func (fs *FileSystem) GetFile(path string) (*FileInfo, error) {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		log.Printf("normalizePath error for GetFile %s: %v", path, err)
		return nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	mimeType := ""
	if !info.IsDir() {
		mimeType = mime.TypeByExtension(filepath.Ext(info.Name()))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	return &FileInfo{
		Name:     info.Name(),
		Path:     path,
		IsDir:    info.IsDir(),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		MimeType: mimeType,
	}, nil
}

// OpenFile 打开文件
func (fs *FileSystem) OpenFile(path string) (*os.File, error) {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		log.Printf("normalizePath error for OpenFile %s: %v", path, err)
		return nil, err
	}

	return os.Open(fullPath)
}

// SaveFile 保存上传的文件
func (fs *FileSystem) SaveFile(path string, reader io.Reader) error {
	log.Printf("SaveFile called with path: %s", path)

	fullPath, err := fs.normalizePath(path)
	if err != nil {
		log.Printf("normalizePath failed: %v", err)
		return err
	}

	log.Printf("fullPath: %s", fullPath)

	// 确保所有父目录存在
	dirPath := filepath.Dir(fullPath)
	log.Printf("Creating directory: %s", dirPath)
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		log.Printf("MkdirAll failed: %v", err)
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		log.Printf("Create file failed: %v", err)
		return err
	}
	defer file.Close()

	log.Printf("Starting to copy data to file...")
	written, err := io.Copy(file, reader)
	log.Printf("Copied %d bytes, error: %v", written, err)
	return err
}

// CreateZip 创建ZIP压缩文件 - 流式处理，支持并发
func (fs *FileSystem) CreateZip(paths []string, w io.Writer) error {
	zipWriter := zip.NewWriter(w)

	// 设置压缩级别为最快（平衡速度和压缩率，大文件时更快）
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})
	defer zipWriter.Close()

	// 使用有界并发池控制同时处理的文件数
	const maxWorkers = 10
	semaphore := make(chan struct{}, maxWorkers)
	var mu sync.Mutex
	var firstErr error

	for _, path := range paths {
		fullPath, err := fs.normalizePath(path)
		if err != nil {
			log.Printf("normalizePath error in CreateZip for path %s: %v", path, err)
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			log.Printf("Stat error for path %s: %v", fullPath, err)
			continue
		}

		if info.IsDir() {
			// 使用 WalkDir 更高效（不调用 Stat）
			err = filepath.WalkDir(fullPath, func(filePath string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}

				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				relPath, err := filepath.Rel(fs.root, filePath)
				if err != nil {
					log.Printf("filepath.Rel error for %s: %v", filePath, err)
					return err
				}

				err = addFileToZip(zipWriter, filePath, relPath)
				if err != nil && firstErr == nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
				return nil // 继续处理其他文件
			})
			if err != nil {
				log.Printf("WalkDir error for path %s: %v", fullPath, err)
				continue
			}
		} else {
			semaphore <- struct{}{}
			relPath, err := filepath.Rel(fs.root, fullPath)
			if err != nil {
				log.Printf("filepath.Rel error for %s: %v", fullPath, err)
				<-semaphore
				continue
			}
			err = addFileToZip(zipWriter, fullPath, relPath)
			<-semaphore
			if err != nil {
				log.Printf("addFileToZip error for %s: %v", fullPath, err)
			}
		}
	}

	return firstErr
}

// addFileToZip 添加文件到ZIP
func addFileToZip(zipWriter *zip.Writer, filePath, zipPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

// ServeFile 提供文件下载
func (fs *FileSystem) ServeFile(path string, w http.ResponseWriter, r *http.Request) error {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		return err
	}

	http.ServeFile(w, r, fullPath)
	return nil
}
