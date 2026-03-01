package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

// Upload 上传文件
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

	var uploaded []string
	var errors []string

	for i, file := range files {
		// 获取相对路径（如果有）
		relativePath := file.Filename
		if i < len(relativePaths) && relativePaths[i] != "" {
			relativePath = relativePaths[i]
		}

		log.Printf("Processing file %d: %s (size: %d), relativePath: %s", i, file.Filename, file.Size, relativePath)

		// 检查文件大小
		if file.Size > h.config.MaxFileSize {
			log.Printf("File too large: %s (size: %d, limit: %d)", file.Filename, file.Size, h.config.MaxFileSize)
			errors = append(errors, fmt.Sprintf("%s: file too large", file.Filename))
			continue
		}

		// 打开上传的文件
		src, err := file.Open()
		if err != nil {
			log.Printf("Failed to open file: %s, error: %v", file.Filename, err)
			errors = append(errors, fmt.Sprintf("%s: %v", file.Filename, err))
			continue
		}

		// 保存文件（使用相对路径）
		// 需要将路径分隔符转换为系统特定的分隔符
		normalizedPath := filepath.FromSlash(relativePath)
		filePath := filepath.Join(path, normalizedPath)
		log.Printf("Saving file to: %s", filePath)

		if err := h.fs.SaveFile(filePath, src); err != nil {
			log.Printf("Failed to save file: %s, error: %v", filePath, err)
			errors = append(errors, fmt.Sprintf("%s: %v", file.Filename, err))
			src.Close()
			continue
		}

		// 立即关闭文件句柄以避免资源泄漏
		src.Close()

		log.Printf("Successfully saved file: %s", filePath)
		uploaded = append(uploaded, relativePath)
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

// Download 下载文件
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

	// 获取文件信息
	info, err := h.fs.GetFile(path)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "file not found",
		})
	}

	if info.IsDir {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "cannot download directory directly",
		})
	}

	// 构建完整文件路径
	fullPath := filepath.Join(h.config.RootPath, path)

	// 检查是否为内联预览模式
	inline := c.Query("inline", "false") == "true"

	// 设置内容类型
	c.Set("Content-Type", info.MimeType)

	if inline {
		// 内联模式：在浏览器中显示内容（用于预览）
		c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", info.Name))
	} else {
		// 下载模式：强制下载文件
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", info.Name))
	}

	return c.SendFile(fullPath)
}

// DownloadZip 批量下载为ZIP
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

	// 创建临时ZIP文件
	tempFile, err := os.CreateTemp("", "download-*.zip")
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create temp file",
		})
	}
	defer os.Remove(tempFile.Name())

	// 创建ZIP文件
	if err := h.fs.CreateZip(paths, tempFile); err != nil {
		tempFile.Close()
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	tempFile.Close()

	// 设置下载头并发送文件
	c.Set("Content-Disposition", "attachment; filename=\"download.zip\"")
	c.Set("Content-Type", "application/zip")

	return c.SendFile(tempFile.Name())
}
