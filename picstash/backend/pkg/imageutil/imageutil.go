package imageutil

import (
	"bytes"
	"image"
	"strings"

	// decode jpeg and png images
	_ "image/jpeg"
	_ "image/png"

	// decode webp images
	_ "golang.org/x/image/webp"
)

// GetImageInfo 从图片数据中获取图片信息
// 返回：宽度、高度、MIME类型、错误
func GetImageInfo(imgData []byte) (int, int, string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(imgData))
	if err != nil {
		return 0, 0, "", err
	}
	return config.Width, config.Height, mimeTypeFromFormat(format), nil
}

func mimeTypeFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}

// GetMimeType 根据文件名获取 MIME 类型
func GetMimeType(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return "application/octet-stream"
	}

	ext := strings.ToLower(filename[idx:])
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
