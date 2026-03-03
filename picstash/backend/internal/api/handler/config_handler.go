package handler

import (
	"github.com/gofiber/fiber/v3"
)

type ConfigHandler struct {
	HomeAuth bool
}

func NewConfigHandler(homeAuth bool) *ConfigHandler {
	return &ConfigHandler{
		HomeAuth: homeAuth,
	}
}

func (h *ConfigHandler) GetConfig(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"home_auth": h.HomeAuth,
	})
}
