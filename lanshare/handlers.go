package main

import (
	"log"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type Handlers struct {
	storage Storage
	config  *Config
}

func NewHandlers(storage Storage, config *Config) *Handlers {
	return &Handlers{
		storage: storage,
		config:  config,
	}
}

func (h *Handlers) GetMessages(c fiber.Ctx) error {
	return c.JSON(h.storage.List())
}

func (h *Handlers) AddMessage(c fiber.Ctx) error {
	var item Item
	if err := c.Bind().Body(&item); err != nil {
		log.Printf("Bind JSON error: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	if len(item.Content) > 10000 {
		return c.Status(400).JSON(fiber.Map{"error": "Text content too long"})
	}

	newItem := NewItem(TEXT, item.Content)
	h.storage.Add(newItem)
	return c.JSON(fiber.Map{"id": newItem.ID, "message": "Message added successfully"})
}

func (h *Handlers) DeleteMessage(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "ID is required"})
	}

	item := h.storage.Get(id)
	if item == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
	}

	// 删除物理文件
	if item.Type != TEXT {
		if saveName, ok := item.Meta["saveName"].(string); ok {
			filePath := filepath.Join(h.config.DownloadPath, saveName)
			if err := removeFile(filePath); err != nil {
				log.Printf("Failed to delete file %s: %v", filePath, err)
			}
		}
	}

	h.storage.Remove(id)
	return c.JSON(fiber.Map{"message": "Item deleted successfully"})
}

func (h *Handlers) UploadFile(c fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请选择要上传的文件"})
	}

	// 保存文件
	saveName := uuid.New().String() + filepath.Ext(fh.Filename)
	savePath := filepath.Join(h.config.DownloadPath, saveName)

	if err := c.SaveFile(fh, savePath); err != nil {
		log.Printf("Failed to save file: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "文件保存失败"})
	}

	item := NewItem(getItemType(fh), fh.Filename)
	item.Meta = map[string]interface{}{
		"saveName":     saveName,
		"path":         h.config.DownloadPath,
		"originalName": fh.Filename,
		"size":         fh.Size,
		"contentType":  fh.Header.Get("Content-Type"),
	}

	h.storage.Add(item)
	return c.JSON(fiber.Map{"message": "File uploaded successfully", "id": item.ID})
}

func (h *Handlers) DownloadFile(c fiber.Ctx) error {
	item := h.storage.Get(c.Params("*"))
	if item == nil {
		return c.SendStatus(404)
	}

	saveName, _ := item.Meta["saveName"].(string)
	realPath := filepath.Join(h.config.DownloadPath, saveName)

	// 检查文件是否存在
	if _, err := statFile(realPath); err != nil {
		return c.SendStatus(404)
	}

	// 设置响应头
	if ct, ok := item.Meta["contentType"].(string); ok {
		c.Set("Content-Type", ct)
	}
	disposition := "attachment"
	if item.Type == IMAGE {
		disposition = "inline"
	}
	if name, ok := item.Meta["originalName"].(string); ok {
		c.Set("Content-Disposition", disposition+`; filename="`+name+`"`)
	}

	return c.SendFile(realPath)
}

// 辅助函数
func getItemType(fh *multipart.FileHeader) Type {
	ext := filepath.Ext(fh.Filename)
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true}
	if imageExts[ext] {
		return IMAGE
	}
	return OTHER
}

func removeFile(path string) error {
	return os.Remove(path)
}

func statFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
