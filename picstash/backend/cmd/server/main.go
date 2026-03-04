package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"picstash/internal/api"
	"picstash/internal/api/middleware"
	"picstash/internal/auth"
	"picstash/internal/config"
	"picstash/internal/database"
	"picstash/internal/service"
	"picstash/internal/storage"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	if len(cfg.Auth.AllowedEmails) == 0 {
		slog.Error("未配置 allowed_emails，必须至少配置一个邮箱才能启动服务")
		os.Exit(1)
	}

	if err := config.InitLogger(cfg); err != nil {
		slog.Error("初始化日志失败", "error", err)
		os.Exit(1)
	}

	if err := database.Init(cfg.Database.Path); err != nil {
		slog.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.AutoMigrate(database.GetDB()); err != nil {
		slog.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}

	jwtService := auth.NewJWTService(cfg)
	emailService := auth.NewEmailService(cfg)
	verificationService := auth.NewVerificationCodeService(database.GetDB())

	// 创建存储实例
	storageInstance := createStorage(cfg)

	// 确定路径前缀（统一从 storage.path_prefix 读取，支持为空）
	pathPrefix := cfg.Storage.PathPrefix

	imageService := service.NewImageService(database.GetDB(), storageInstance, pathPrefix)
	imageSyncService := service.NewImageSyncService(database.GetDB(), storageInstance, pathPrefix)
	tagService := service.NewTagService(database.GetDB())

	app := fiber.New(fiber.Config{
		AppName: "Picstash Backend", ErrorHandler: defaultErrorHandler,
		BodyLimit: parseBodySize(cfg.Server.MaxBodySize),
	})

	app.Use(middleware.Recovery())
	app.Use(middleware.CORS())
	app.Use(middleware.Logger())

	api.SetupRoutes(app, cfg, jwtService, emailService, verificationService, imageService, imageSyncService, tagService, database.GetDB())

	// 设置本地存储的静态文件服务
	setupStaticFiles(app, cfg, storageInstance)

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		slog.Info("接收到关闭信号，正在关闭服务器...")
		app.Shutdown()
	}()

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("服务器启动", "port", cfg.Server.Port, "mode", cfg.Server.Mode)

	if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: false}); err != nil {
		slog.Error("服务器启动失败", "error", err)
		os.Exit(1)
	}
}

func defaultErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	slog.Error("请求错误", "path", c.Path(), "error", err)

	return c.Status(code).JSON(fiber.Map{
		"error": err.Error(),
	})
}

func parseBodySize(size string) int {
	size = strings.TrimSpace(size)
	if size == "" {
		return 100 * 1024 * 1024
	}

	unit := ""
	number := size
	for i := 0; i < len(size); i++ {
		if size[i] >= '0' && size[i] <= '9' {
			continue
		}
		unit = size[i:]
		number = size[:i]
		break
	}

	value, err := strconv.Atoi(number)
	if err != nil {
		return 100 * 1024 * 1024
	}

	switch strings.ToLower(unit) {
	case "kb":
		return value * 1024
	case "mb":
		return value * 1024 * 1024
	case "gb":
		return value * 1024 * 1024 * 1024
	default:
		return value
	}
}

// createStorage 根据配置创建对应的存储实例
func createStorage(cfg *config.Config) storage.Storage {
	storageType := cfg.Storage.Type
	if storageType == "" {
		storageType = "local" // 默认使用 local 存储
	}

	switch storageType {
	case "local":
		// 构建服务器地址
		serverAddr := cfg.Storage.Local.ServerAddr
		if serverAddr == "" {
			serverAddr = fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
		}
		slog.Info("使用本地文件系统存储",
			"base_path", cfg.Storage.Local.BasePath,
			"url_path", cfg.Storage.Local.URLPath,
			"server_addr", serverAddr,
		)
		return storage.NewLocalStorage(
			cfg.Storage.Local.BasePath,
			cfg.Storage.Local.URLPath,
			serverAddr,
		)
	case "github":
		githubCfg := cfg.Storage.GitHub
		slog.Info("使用 GitHub 存储",
			"owner", githubCfg.Owner,
			"repo", githubCfg.Repo,
			"branch", githubCfg.Branch,
		)
		return storage.NewGitHubStorage(githubCfg.Token, githubCfg.Owner, githubCfg.Repo, githubCfg.Branch)
	default:
		panic("storage unkonw")
	}
}

// setupStaticFiles 为本地存储设置静态文件服务
func setupStaticFiles(app *fiber.App, cfg *config.Config, storageInstance storage.Storage) {
	// 根据配置判断是否为本地存储
	if cfg.Storage.Type == "local" {
		urlPath := cfg.Storage.Local.URLPath
		basePath := cfg.Storage.Local.BasePath

		slog.Info("注册静态文件服务", "url_path", urlPath, "base_path", basePath)

		// 注册静态文件路由，将 urlPath 映射到 basePath
		// 例如：/files/images/xxx.jpg -> ./data/files/images/xxx.jpg
		app.Get(urlPath+"/*", static.New(basePath))
	}
}
