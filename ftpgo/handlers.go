package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

// Handlers HTTP处理器
type Handlers struct {
	fs     *FileSystem
	config *Config
}

// NewHandlers 创建处理器
func NewHandlers(fs *FileSystem, config *Config) *Handlers {
	return &Handlers{fs: fs, config: config}
}

// BrowseRequest 浏览请求
type BrowseRequest struct {
	Path string `query:"path"`
}

// BrowseResponse 浏览响应
type BrowseResponse struct {
	Path  string     `json:"path"`
	Files []FileInfo `json:"files"`
}

// Browse 浏览目录
func (h *Handlers) Browse(c fiber.Ctx) error {
	path := c.Query("path", "/")

	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	files, err := h.fs.ListDirectory(path)
	if err != nil {
		log.Printf("ListDirectory error for path %s: %v", path, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list directory",
		})
	}

	return c.JSON(BrowseResponse{
		Path:  path,
		Files: files,
	})
}

// MkdirRequest 创建目录请求
type MkdirRequest struct {
	Path string `json:"path"`
}

// Mkdir 创建目录
func (h *Handlers) Mkdir(c fiber.Ctx) error {
	var req MkdirRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	if req.Path == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "path is required",
		})
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}

	if err := h.fs.CreateDirectory(req.Path); err != nil {
		log.Printf("CreateDirectory error for path %s: %v", req.Path, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create directory",
		})
	}

	return c.JSON(fiber.Map{
		"message": "directory created",
		"path":    req.Path,
	})
}

// Upload 上传文件 - 并发处理
func (h *Handlers) Upload(c fiber.Ctx) error {
	path := c.Query("path", "/")

	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	log.Printf("Upload request - path: %s", path)

	// 获取上传的文件
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid form data",
		})
	}

	files := form.File["files"]
	relativePaths := form.Value["relativePaths"]
	if len(files) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "no files uploaded",
		})
	}

	// 使用并发处理文件上传
	type uploadResult struct {
		filename string
		err      error
	}

	// 控制并发数（最多同时处理5个文件）
	const maxWorkers = 5
	semaphore := make(chan struct{}, maxWorkers)
	results := make(chan uploadResult, len(files))
	var wg sync.WaitGroup

	for i, file := range files {
		wg.Add(1)
		go func(idx int, f *multipart.FileHeader) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 获取相对路径
			relativePath := f.Filename
			if idx < len(relativePaths) && relativePaths[idx] != "" {
				relativePath = relativePaths[idx]
			}

			// 检查文件大小
			if f.Size > h.config.MaxFileSize {
				results <- uploadResult{
					filename: f.Filename,
					err:      fmt.Errorf("file too large"),
				}
				return
			}

			// 打开上传的文件
			src, err := f.Open()
			if err != nil {
				results <- uploadResult{
					filename: f.Filename,
					err:      err,
				}
				return
			}
			defer src.Close()

			// 保存文件
			normalizedPath := filepath.FromSlash(relativePath)
			filePath := filepath.Join(path, normalizedPath)

			if err := h.fs.SaveFile(filePath, src); err != nil {
				results <- uploadResult{
					filename: f.Filename,
					err:      err,
				}
				return
			}

			results <- uploadResult{
				filename: relativePath,
				err:      nil,
			}
		}(i, file)
	}

	// 等待所有上传完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var uploaded []string
	var errors []string
	for res := range results {
		if res.err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", res.filename, res.err))
		} else {
			uploaded = append(uploaded, res.filename)
		}
	}

	response := fiber.Map{
		"message":  "upload complete",
		"uploaded": uploaded,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	return c.JSON(response)
}

// DeleteRequest 删除请求
type DeleteRequest struct {
	Path string `json:"path"`
}

// Delete 删除文件或目录
func (h *Handlers) Delete(c fiber.Ctx) error {
	var req DeleteRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	if req.Path == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "path is required",
		})
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}

	if err := h.fs.Delete(req.Path); err != nil {
		log.Printf("Delete error for path %s: %v", req.Path, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to delete",
		})
	}

	return c.JSON(fiber.Map{
		"message": "deleted",
		"path":    req.Path,
	})
}

// RenameRequest 重命名请求
type RenameRequest struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
}

// Rename 重命名文件或目录
func (h *Handlers) Rename(c fiber.Ctx) error {
	var req RenameRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	if req.OldPath == "" || req.NewPath == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "oldPath and newPath are required",
		})
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(req.OldPath, "/") {
		req.OldPath = "/" + req.OldPath
	}
	if !strings.HasPrefix(req.NewPath, "/") {
		req.NewPath = "/" + req.NewPath
	}

	if err := h.fs.Rename(req.OldPath, req.NewPath); err != nil {
		log.Printf("Rename error from %s to %s: %v", req.OldPath, req.NewPath, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to rename",
		})
	}

	return c.JSON(fiber.Map{
		"message": "renamed",
		"oldPath": req.OldPath,
		"newPath": req.NewPath,
	})
}

// Download 下载文件 - 支持 Range 请求
func (h *Handlers) Download(c fiber.Ctx) error {
	path := c.Query("path", "")

	if path == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "path is required",
		})
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 安全检查路径（避免目录遍历）
	cleanPath := strings.TrimPrefix(path, "/")
	cleanPath = filepath.Clean(cleanPath)
	fullPath := filepath.Join(h.config.RootPath, cleanPath)

	// 确保路径在根目录内
	cleanFullPath := filepath.Clean(fullPath)
	cleanRoot := filepath.Clean(h.config.RootPath)
	if !strings.HasPrefix(cleanFullPath, cleanRoot) {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"error": "invalid path",
		})
	}

	// 检查文件是否存在且不是目录
	info, err := os.Stat(fullPath)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "file not found",
		})
	}

	if info.IsDir() {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "cannot download directory directly",
		})
	}

	// 检查是否为内联预览模式
	inline := c.Query("inline", "false") == "true"

	// 获取 MIME 类型
	mimeType := getMimeType(filepath.Ext(info.Name()))
	c.Set("Content-Type", mimeType)
	c.Set("Accept-Ranges", "bytes")

	if inline {
		c.Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", info.Name()))
	} else {
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	}

	// 处理 Range 请求（用于预览时只获取部分内容）
	rangeHeader := string(c.Request().Header.Peek("Range"))
	if rangeHeader != "" {
		return h.serveRange(fullPath, rangeHeader, info.Size(), c)
	}

	// 设置缓存头
	c.Set("Cache-Control", "public, max-age=3600")

	return c.SendFile(fullPath)
}

// getMimeType 获取文件的 MIME 类型
func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
	}
	if mt, ok := mimeTypes[strings.ToLower(ext)]; ok {
		return mt
	}
	return "application/octet-stream"
}

// serveRange 处理 Range 请求
func (h *Handlers) serveRange(filePath, rangeHeader string, fileSize int64, c fiber.Ctx) error {
	// 解析 Range: bytes=start-end
	parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(parts) != 2 {
		return c.Status(http.StatusRequestedRangeNotSatisfiable).SendString("Invalid Range")
	}

	start, _ := strconv.ParseInt(parts[0], 10, 64)
	var end int64
	if parts[1] == "" {
		end = fileSize - 1
	} else {
		end, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	// 如果请求的范围超出文件大小，调整 end 值（而不是返回 416）
	if start < 0 || start >= fileSize {
		return c.Status(http.StatusRequestedRangeNotSatisfiable).SendString("Invalid Range")
	}
	if end >= fileSize {
		end = fileSize - 1
	}
	if start > end {
		return c.Status(http.StatusRequestedRangeNotSatisfiable).SendString("Invalid Range")
	}

	contentLength := end - start + 1

	// 读取指定范围的数据到 buffer
	file, err := os.Open(filePath)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Failed to open file")
	}
	defer file.Close()

	_, err = file.Seek(start, 0)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Failed to seek file")
	}

	// 读取指定长度的数据到 buffer，避免流的问题
	buffer := make([]byte, contentLength)
	_, err = io.ReadFull(file, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return c.Status(http.StatusInternalServerError).SendString("Failed to read file")
	}

	c.Status(http.StatusPartialContent)
	c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Set("Cache-Control", "public, max-age=3600")

	return c.Send(buffer)
}

// DownloadZip 批量下载为ZIP - 流式传输，无需临时文件
func (h *Handlers) DownloadZip(c fiber.Ctx) error {
	pathsStr := c.Query("paths", "")

	if pathsStr == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "paths is required",
		})
	}

	var paths []string
	if err := json.Unmarshal([]byte(pathsStr), &paths); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid paths format",
		})
	}

	if len(paths) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "no paths provided",
		})
	}

	// 设置流式响应头
	c.Set("Content-Disposition", "attachment; filename=\"download.zip\"")
	c.Set("Content-Type", "application/zip")
	c.Set("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	// 使用流式 writer，边压缩边发送，不占用临时磁盘空间
	writer := c.Response().BodyWriter()
	if err := h.fs.CreateZip(paths, writer); err != nil {
		log.Printf("CreateZip error: %v", err)
		// 由于已经开始发送，无法返回错误 JSON，记录日志
	}

	return nil
}
