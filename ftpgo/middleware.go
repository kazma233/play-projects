package main

import (
	"crypto/subtle"
	"encoding/base64"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// SetupMiddleware 设置中间件
func SetupMiddleware(app *fiber.App, config *Config) {
	// 恢复中间件 - 捕获panic
	app.Use(recover.New())

	// 日志中间件
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} ${latency}\n",
	}))

	if strings.TrimSpace(config.CORSOrigins) == "" {
		return
	}

	allowedOrigins := make([]string, 0)
	for origin := range strings.SplitSeq(config.CORSOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins = append(allowedOrigins, origin)
		}
	}

	if len(allowedOrigins) == 0 {
		return
	}

	// CORS中间件
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"*"},
	}))
}

// BasicAuthMiddleware Basic Auth 鉴权中间件
func BasicAuthMiddleware(config *Config) fiber.Handler {
	// 如果没有配置用户名和密码，则跳过鉴权
	if config.AuthUser == "" && config.AuthPass == "" {
		log.Println("Warning: Basic Auth is not configured (FTPGO_AUTH_USER and FTPGO_AUTH_PASS not set)")
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	log.Printf("Basic Auth enabled for user: %s", config.AuthUser)

	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization header required",
			})
		}

		// 检查是否为 Basic Auth 格式
		const prefix = "Basic "
		if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
			c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization format",
			})
		}

		// 解码 Base64 凭证
		encoded := authHeader[len(prefix):]
		decoded, err := parseBase64(encoded)
		if err != nil {
			c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid base64 encoding",
			})
		}

		// 分离用户名和密码
		credentials := string(decoded)
		for i := 0; i < len(credentials); i++ {
			if credentials[i] == ':' {
				user := credentials[:i]
				pass := credentials[i+1:]

				// 使用 constant-time 比较避免时序攻击
				userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(config.AuthUser)) == 1
				passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(config.AuthPass)) == 1

				if userMatch && passMatch {
					return c.Next()
				}
				break
			}
		}

		c.Set("WWW-Authenticate", `Basic realm="Restricted"`)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}
}

// parseBase64 简单的 Base64 解码
func parseBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
