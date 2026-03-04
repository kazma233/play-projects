package main

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
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
	cleanRoot := filepath.Clean(root)
	if absRoot, err := filepath.Abs(cleanRoot); err == nil {
		cleanRoot = absRoot
	}

	return &FileSystem{root: cleanRoot}
}

// normalizePath 规范化路径，防止目录遍历攻击
func (fs *FileSystem) normalizePath(path string) (string, error) {
	normalizedPath := strings.ReplaceAll(path, "\\", "/")
	normalizedPath = strings.TrimLeft(normalizedPath, "/")
	normalizedPath = filepath.Clean(normalizedPath)

	if normalizedPath == "." {
		return fs.root, nil
	}

	if filepath.IsAbs(normalizedPath) {
		return "", fmt.Errorf("absolute path is not allowed")
	}

	fullPath := filepath.Join(fs.root, normalizedPath)
	relPath, err := filepath.Rel(fs.root, fullPath)
	if err != nil {
		return "", err
	}

	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
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

// SaveFile 保存上传的文件
func (fs *FileSystem) SaveFile(path string, reader io.Reader) error {
	fullPath, err := fs.normalizePath(path)
	if err != nil {
		return err
	}

	// 确保所有父目录存在
	dirPath := filepath.Dir(fullPath)
	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

// CreateZip 创建 ZIP 压缩文件（流式输出）
func (fs *FileSystem) CreateZip(paths []string, w io.Writer) error {
	zipWriter := zip.NewWriter(w)

	// 设置压缩级别为最快（平衡速度和压缩率，大文件时更快）
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})
	defer zipWriter.Close()

	var firstErr error
	recordErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	for _, path := range paths {
		fullPath, err := fs.normalizePath(path)
		if err != nil {
			log.Printf("normalizePath error in CreateZip for path %s: %v", path, err)
			recordErr(err)
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			log.Printf("Stat error for path %s: %v", fullPath, err)
			recordErr(err)
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

				relPath, err := filepath.Rel(fs.root, filePath)
				if err != nil {
					log.Printf("filepath.Rel error for %s: %v", filePath, err)
					return err
				}

				err = addFileToZip(zipWriter, filePath, relPath)
				if err != nil {
					log.Printf("addFileToZip error for %s: %v", filePath, err)
					recordErr(err)
				}
				return nil // 继续处理其他文件
			})
			if err != nil {
				log.Printf("WalkDir error for path %s: %v", fullPath, err)
				recordErr(err)
				continue
			}
		} else {
			relPath, err := filepath.Rel(fs.root, fullPath)
			if err != nil {
				log.Printf("filepath.Rel error for %s: %v", fullPath, err)
				recordErr(err)
				continue
			}
			err = addFileToZip(zipWriter, fullPath, relPath)
			if err != nil {
				log.Printf("addFileToZip error for %s: %v", fullPath, err)
				recordErr(err)
			}
		}
	}

	return firstErr
}

// addFileToZip 添加文件到ZIP
func addFileToZip(zipWriter *zip.Writer, filePath, zipPath string) error {
	zipPath = filepath.ToSlash(filepath.Clean(zipPath))
	zipPath = strings.TrimPrefix(zipPath, "/")
	if zipPath == "." || zipPath == "" || strings.HasPrefix(zipPath, "../") {
		return fmt.Errorf("invalid zip path")
	}

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
