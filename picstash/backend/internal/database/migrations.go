package database

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Migration struct {
	Name     string
	Checksum string
	Content  string
}

func AutoMigrate(db *sql.DB) error {
	slog.Info("开始数据库迁移")

	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("加载迁移文件失败: %w", err)
	}

	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("创建migrations表失败: %w", err)
	}

	for _, migration := range migrations {
		exists, err := isMigrationExecuted(db, migration.Name)
		if err != nil {
			return fmt.Errorf("检查迁移状态失败: %w", err)
		}

		if exists {
			slog.Debug("迁移已执行，跳过", "name", migration.Name)
			continue
		}

		slog.Info("执行迁移", "name", migration.Name)

		if err := executeMigration(db, migration); err != nil {
			return fmt.Errorf("执行迁移失败 %s: %w", migration.Name, err)
		}
	}

	slog.Info("数据库迁移完成")

	return nil
}

func loadMigrations() ([]Migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("读取迁移文件失败 %s: %w", entry.Name(), err)
		}

		checksum := fmt.Sprintf("%x", sha256.Sum256(content))

		migrations = append(migrations, Migration{
			Name:     entry.Name(),
			Checksum: checksum,
			Content:  string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

func executeMigration(db *sql.DB, migration Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(migration.Content); err != nil {
		return err
	}

	_, err = tx.Exec(
		"INSERT INTO migrations (name, checksum, executed_at) VALUES (?, ?, ?)",
		migration.Name,
		migration.Checksum,
		time.Now(),
	)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	slog.Info("迁移执行成功", "name", migration.Name)

	return nil
}

func createMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			checksum TEXT NOT NULL,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func isMigrationExecuted(db *sql.DB, name string) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM migrations WHERE name = ?)", name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
