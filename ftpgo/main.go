package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/static"
)

//go:embed templates/* static/*
var embedFS embed.FS

func main() {
	// 加载配置
	config := LoadConfig()

	// 初始化文件系统
	fsys := NewFileSystem(config.RootPath)

	// 确保根目录存在
	if err := os.MkdirAll(config.RootPath, 0755); err != nil {
		log.Fatalf("Failed to create root directory: %v", err)
	}

	// 创建处理器
	handlers := NewHandlers(fsys, config)

	// 创建Fiber应用
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			message := "Internal Server Error"

			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				message = e.Message
			}

			log.Printf("Error: %v", err)

			return c.Status(code).JSON(fiber.Map{
				"error": message,
			})
		},
		BodyLimit:         int(config.MaxFileSize + 4*1024*1024),
		StreamRequestBody: true,
		ReadBufferSize:    65536, // 64KB 读取缓冲区
	})

	// 设置中间件
	SetupMiddleware(app, config)

	// 启用Gzip/Brotli压缩（对API响应和静态资源）
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // 最快压缩级别，减少CPU使用
	}))

	// 静态文件服务 - 从embed.FS提供
	templatesFS, err := fs.Sub(embedFS, "templates")
	if err != nil {
		log.Fatalf("Failed to create templates sub filesystem: %v", err)
	}

	staticFS, err := fs.Sub(embedFS, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub filesystem: %v", err)
	}

	// 提供静态资源
	app.Use("/static", static.New("", static.Config{FS: staticFS}))

	// API路由 - 需要鉴权
	api := app.Group("/api")
	api.Use(BasicAuthMiddleware(config))
	{
		api.Get("/browse", handlers.Browse)
		api.Post("/mkdir", handlers.Mkdir)
		api.Post("/upload", handlers.Upload)
		api.Post("/delete", handlers.Delete)
		api.Post("/rename", handlers.Rename)
		api.Get("/download", handlers.Download)
		api.Get("/download-zip", handlers.DownloadZip)
	}

	// 主页 - 提供前端应用（需要鉴权）
	app.Get("/", BasicAuthMiddleware(config), func(c fiber.Ctx) error {
		content, err := templatesFS.(fs.ReadFileFS).ReadFile("index.html")
		if err != nil {
			return c.Status(500).SendString("Failed to load template")
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Send(content)
	})

	// 启动服务器
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("Server starting on %s", addr)
	log.Printf("Root directory: %s", config.RootPath)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
