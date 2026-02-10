package main

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/static"
)

//go:embed templates css js
var assetsFS embed.FS

func main() {
	// 加载配置
	config := LoadConfig()

	// 初始化下载目录
	mustInitDir(config.DownloadPath)

	// 初始化存储系统
	ms, err := NewMemoryStorage(config.DownloadPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	err = ms.Load()
	if err != nil {
		log.Fatalf("Failed to load data: %v", err)
	}

	// 启动时清理过期数据（包含物理文件删除）
	if err := ms.CleanupExpired(30); err != nil {
		log.Printf("Initial cleanup failed: %v", err)
	}

	// 启动时执行一次 compact（在 Load 之后）
	log.Println("Performing initial compact...")
	if err := ms.Compact(); err != nil {
		log.Printf("Initial compact failed: %v", err)
	}

	// 创建处理器
	handlers := NewHandlers(ms, config)

	// 创建 Fiber 应用
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			log.Printf("URL %s has Unknow Error:  %v", c.OriginalURL(), err)
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// 中间件
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(requestid.New())
	app.Use(NewCORSMiddleware())
	app.Use(FileValidationMiddleware(config))

	// 静态文件服务
	app.Use("/fs", static.New(config.DownloadPath)) // 文件上传目录从文件系统读取

	// 从 embed.FS 提供静态资源
	cssSubFS, err := fs.Sub(assetsFS, "css")
	if err != nil {
		log.Fatalf("Failed to create CSS sub filesystem: %v", err)
	}
	jsSubFS, err := fs.Sub(assetsFS, "js")
	if err != nil {
		log.Fatalf("Failed to create JS sub filesystem: %v", err)
	}
	templatesSubFS, err := fs.Sub(assetsFS, "templates")
	if err != nil {
		log.Fatalf("Failed to create templates sub filesystem: %v", err)
	}

	app.Use("/css", static.New("", static.Config{FS: cssSubFS}))
	app.Use("/js", static.New("", static.Config{FS: jsSubFS}))

	// 路由
	templatesFile, err := templatesSubFS.Open("index.html")
	if err != nil {
		log.Fatalf("Failed to open templates/index.html: %v", err)
	}
	templatesContent, err := io.ReadAll(templatesFile)
	templatesFile.Close()
	if err != nil {
		log.Fatalf("Failed to read templates/index.html: %v", err)
	}

	app.Get("/", func(ctx fiber.Ctx) error {
		ctx.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return ctx.Send(templatesContent)
	})

	// API 路由
	app.Get("/msg", handlers.GetMessages)
	app.Post("/msg", handlers.AddMessage)
	app.Delete("/msg/:id", handlers.DeleteMessage)
	app.Post("/fs", handlers.UploadFile)
	app.Get("/download/*", handlers.DownloadFile)

	// 启动信息
	printEndpointInfo(config.Port)

	// 优雅关闭
	go func() {
		if err := app.Listen(":" + strconv.Itoa(config.Port)); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 优雅关闭，超时 30 秒
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// 关闭存储系统
	if err := ms.Close(); err != nil {
		log.Printf("Failed to close storage: %v", err)
	}

	log.Println("Server exited")
}

func printEndpointInfo(port int) {
	log.Printf("Starting server on port %d", port)

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get interfaces: %v", err)
		return
	}

	// 遍历所有网络接口
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("Failed to get addresses for %s: %v", iface.Name, err)
			continue
		}

		// 遍历该接口下的所有IP地址
		for _, addr := range addrs {
			// 检查IP地址是否为IPv4地址
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				log.Printf("Local IP address: http://%s:%d", ipnet.IP.String(), port)
			}
		}
	}
}

func mustInitDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		log.Printf("Created directory: %s", dir)
	}
}
