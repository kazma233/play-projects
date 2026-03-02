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
// 返回：宽度、高度、格式、错误
func GetImageInfo(imgData []byte) (int, int, string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(imgData))
	if err != nil {
		return 0, 0, "", err
	}
	return config.Width, config.Height, format, nil
}

// GetMimeType 根据文件名获取 MIME 类型
func GetMimeType(filename string) string {
	ext := filename[strings.LastIndex(filename, "."):]
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
