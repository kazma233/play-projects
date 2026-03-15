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
	type uploadJob struct {
		idx  int
		file *multipart.FileHeader
	}

	// 固定 worker 池，避免每个文件都创建 goroutine
	const maxWorkers = 5
	workerCount := maxWorkers
	if len(files) < workerCount {
		workerCount = len(files)
	}

	jobs := make(chan uploadJob, len(files))
	results := make(chan uploadResult, len(files))
	var wg sync.WaitGroup

	for workerIdx := 0; workerIdx < workerCount; workerIdx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for job := range jobs {
				idx := job.idx
				f := job.file

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
					continue
				}

				// 打开上传的文件
				src, err := f.Open()
				if err != nil {
					results <- uploadResult{
						filename: f.Filename,
						err:      err,
					}
					continue
				}

				// 保存文件
				normalizedPath := filepath.FromSlash(relativePath)
				filePath := filepath.Join(path, normalizedPath)

				err = h.fs.SaveFile(filePath, src)
				src.Close()
				if err != nil {
					results <- uploadResult{
						filename: f.Filename,
						err:      err,
					}
					continue
				}

				results <- uploadResult{
					filename: relativePath,
					err:      nil,
				}
			}
		}()
	}

	for i, file := range files {
		jobs <- uploadJob{idx: i, file: file}
	}
	close(jobs)

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

	fullPath, err := h.fs.normalizePath(path)
	if err != nil {
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
	c.Set("Cache-Control", "private, no-store")

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
	start, end, err := parseSingleRange(rangeHeader, fileSize)
	if err != nil {
		c.Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		return c.Status(http.StatusRequestedRangeNotSatisfiable).SendString("Invalid Range")
	}

	contentLength := end - start + 1

	file, err := os.Open(filePath)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("Failed to open file")
	}

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		file.Close()
		return c.Status(http.StatusInternalServerError).SendString("Failed to seek file")
	}

	c.Status(http.StatusPartialContent)
	c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Set("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Set("Cache-Control", "private, no-store")

	// SendStream sends asynchronously after the handler returns, so Fiber must own the file.
	return c.SendStream(file, int(contentLength))
}

func parseSingleRange(rangeHeader string, fileSize int64) (int64, int64, error) {
	if fileSize <= 0 {
		return 0, 0, fmt.Errorf("empty file")
	}

	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range unit")
	}

	rangeSpec := strings.TrimSpace(strings.TrimPrefix(rangeHeader, "bytes="))
	if rangeSpec == "" || strings.Contains(rangeSpec, ",") {
		return 0, 0, fmt.Errorf("multiple ranges not supported")
	}

	parts := strings.SplitN(rangeSpec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	var start, end int64

	switch {
	case parts[0] == "" && parts[1] == "":
		return 0, 0, fmt.Errorf("invalid range value")
	case parts[0] == "":
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLen <= 0 {
			return 0, 0, fmt.Errorf("invalid suffix range")
		}
		if suffixLen > fileSize {
			suffixLen = fileSize
		}
		start = fileSize - suffixLen
		end = fileSize - 1
	case parts[1] == "":
		parsedStart, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || parsedStart < 0 || parsedStart >= fileSize {
			return 0, 0, fmt.Errorf("invalid start range")
		}
		start = parsedStart
		end = fileSize - 1
	default:
		parsedStart, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || parsedStart < 0 || parsedStart >= fileSize {
			return 0, 0, fmt.Errorf("invalid start range")
		}
		parsedEnd, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || parsedEnd < parsedStart {
			return 0, 0, fmt.Errorf("invalid end range")
		}
		if parsedEnd >= fileSize {
			parsedEnd = fileSize - 1
		}
		start = parsedStart
		end = parsedEnd
	}

	if start < 0 || end < start || end >= fileSize {
		return 0, 0, fmt.Errorf("range outside file")
	}

	return start, end, nil
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
