package storage

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

func GetImageInfo(imgData []byte) (int, int, string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(imgData))
	if err != nil {
		return 0, 0, "", err
	}
	return config.Width, config.Height, format, nil
}

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
