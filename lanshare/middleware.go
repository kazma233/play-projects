package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// 文件验证中间件：检查文件大小
func FileValidationMiddleware(config *Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() == "POST" && strings.HasPrefix(c.Path(), "/fs") {
			size, err := strconv.ParseInt(c.Get("Content-Length"), 10, 64)
			if err == nil && size > config.MaxFileSize {
				maxMB := config.MaxFileSize / (1024 * 1024)
				return c.Status(413).JSON(fiber.Map{
					"error": fmt.Sprintf("文件太大，最大支持 %d MB", maxMB),
				})
			}
		}
		return c.Next()
	}
}

// NewCORSMiddleware 返回 Fiber 内置的 CORS 中间件配置
func NewCORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return true // 允许所有来源
		},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type"},
	})
}
