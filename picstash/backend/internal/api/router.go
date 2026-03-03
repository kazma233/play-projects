package api

import (
	"database/sql"

	"picstash/internal/api/handler"
	"picstash/internal/api/middleware"
	"picstash/internal/auth"
	"picstash/internal/config"
	"picstash/internal/service"

	"github.com/gofiber/fiber/v3"
)

func SetupRoutes(
	app *fiber.App,
	cfg *config.Config,
	jwtService *auth.JWTService,
	emailService *auth.EmailService,
	verificationService *auth.VerificationCodeService,
	imageService *service.ImageService,
	tagService *service.TagService,
	db *sql.DB,
) {
	authHandler := handler.NewAuthHandler(emailService, verificationService, jwtService, &cfg.Auth)
	imageHandler := handler.NewImageHandler(imageService)
	tagHandler := handler.NewTagHandler(tagService)
	syncLogHandler := handler.NewSyncLogHandler(db)
	configHandler := handler.NewConfigHandler(cfg.Auth.HomeAuth)

	api := app.Group("/api")

	// 完全公开接口
	public := api.Group("")
	public.Get("/config", configHandler.GetConfig)

	authGroup := api.Group("/auth")
	authGroup.Post("/send-code", authHandler.SendCode)
	authGroup.Post("/verify", authHandler.VerifyCode)

	// 根据 home_auth 配置决定是否保护图片相关接口
	contentGroup := api.Group("")
	if cfg.Auth.HomeAuth {
		contentGroup.Use(middleware.JWTAuth(jwtService))
	}
	contentGroup.Get("/images", imageHandler.GetList)
	contentGroup.Get("/images/:id", imageHandler.GetByID)
	contentGroup.Get("/tags", tagHandler.GetAll)
	contentGroup.Get("/tags/:id/images", tagHandler.GetByImageID)
	contentGroup.Get("/sync/logs", syncLogHandler.GetList)
	contentGroup.Get("/sync/logs/:id", syncLogHandler.GetByID)
	contentGroup.Get("/sync/logs/:id/files", syncLogHandler.GetFileLogs)

	// 受保护接口（始终需要登录）
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(jwtService))
	protected.Post("/images/upload", imageHandler.Upload)
	protected.Delete("/images/:id", imageHandler.Delete)
	protected.Put("/images/:id/tags", imageHandler.UpdateTags)
	protected.Post("/tags", tagHandler.Create)
	protected.Put("/tags/:id", tagHandler.Update)
	protected.Delete("/tags/:id", tagHandler.Delete)
	protected.Post("/images/sync", imageHandler.SyncFromStorage)
}
