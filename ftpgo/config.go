package main

import (
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	RootPath    string // 文件根目录
	Port        int    // 服务端口
	MaxFileSize int64  // 最大文件大小（字节）
	AuthUser    string // Basic Auth 用户名
	AuthPass    string // Basic Auth 密码
}

// LoadConfig 加载配置
func LoadConfig() *Config {
	config := &Config{
		RootPath:    getEnv("FTPGO_ROOT", "./data"),
		Port:        getEnvAsInt("FTPGO_PORT", 7300),
		MaxFileSize: getEnvAsInt64("FTPGO_MAX_SIZE", 1024*1024*1024), // 默认1GB
		AuthUser:    getEnv("FTPGO_AUTH_USER", ""),
		AuthPass:    getEnv("FTPGO_AUTH_PASS", ""),
	}
	return config
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 获取环境变量并转换为int
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsInt64 获取环境变量并转换为int64
func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}
