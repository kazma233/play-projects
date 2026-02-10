package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port         int
	DownloadPath string
	MaxFileSize  int64
	AllowedTypes []string
}

func LoadConfig() *Config {
	const envPrefix = "LC_"

	getEnv := func(key string, defaultVal string) string {
		if v := os.Getenv(envPrefix + key); v != "" {
			return v
		}
		return defaultVal
	}

	getEnvInt := func(key string, defaultVal int) int {
		if v := os.Getenv(envPrefix + key); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
			log.Printf("Warning: Invalid %s%s, using default: %d", envPrefix, key, defaultVal)
		}
		return defaultVal
	}

	getEnvInt64 := func(key string, defaultVal int64) int64 {
		if v := os.Getenv(envPrefix + key); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
			log.Printf("Warning: Invalid %s%s, using default: %d", envPrefix, key, defaultVal)
		}
		return defaultVal
	}

	config := &Config{
		Port:         getEnvInt("PORT", 5666),
		DownloadPath: getEnv("DOWNLOAD_PATH", "./download"),
		MaxFileSize:  getEnvInt64("MAX_FILE_SIZE", 1024*1024*1024),
	}

	if v := os.Getenv(envPrefix + "ALLOWED_TYPES"); v != "" {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		config.AllowedTypes = parts
	} else {
		config.AllowedTypes = []string{"*"}
	}

	// 简单的验证和回退
	if config.Port < 1 || config.Port > 65535 {
		log.Printf("Warning: Invalid port %d, using default 5666", config.Port)
		config.Port = 5666
	}
	if config.MaxFileSize <= 0 {
		log.Printf("Warning: Invalid max file size, using default 1GB")
		config.MaxFileSize = 1024 * 1024 * 1024
	}
	if config.DownloadPath == "" {
		log.Printf("Warning: Empty download path, using default ./download")
		config.DownloadPath = "./download"
	}

	log.Printf("Config loaded: Port=%d, DownloadPath=%s, MaxFileSize=%dMB, AllowedTypes=%v",
		config.Port, config.DownloadPath, config.MaxFileSize/(1024*1024), config.AllowedTypes)

	return config
}
