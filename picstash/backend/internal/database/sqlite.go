package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
)

func Init(dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite3", sqliteDSN(dbPath))
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// SQLite 对并发写入比较敏感，保守连接池配置更稳妥。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	slog.Info("数据库连接成功", "path", dbPath)

	return nil
}

func sqliteDSN(dbPath string) string {
	values := url.Values{}
	values.Set("_busy_timeout", "5000")
	values.Set("_foreign_keys", "on")
	values.Set("_journal_mode", "WAL")

	return fmt.Sprintf("file:%s?%s", dbPath, values.Encode())
}

func GetDB() *sql.DB {
	return db
}

func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
