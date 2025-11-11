package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // SQLite driver
)

// Migrate 执行数据库迁移
func Migrate(ctx context.Context, db *sql.DB, migrationsDir string, direction string) error {
	// 设置 goose 方言
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	// 检查迁移目录
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		return fmt.Errorf("迁移目录不存在: %s", migrationsDir)
	}

	// 执行迁移
	switch direction {
	case "up":
		if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
			return fmt.Errorf("执行迁移失败: %w", err)
		}
	case "down":
		if err := goose.DownContext(ctx, db, migrationsDir); err != nil {
			return fmt.Errorf("回滚迁移失败: %w", err)
		}
	case "up-to":
		// 需要版本号参数
		return fmt.Errorf("up-to 需要版本号参数")
	case "down-to":
		// 需要版本号参数
		return fmt.Errorf("down-to 需要版本号参数")
	default:
		return fmt.Errorf("未知的迁移方向: %s", direction)
	}

	return nil
}

// MigrateTo 迁移到指定版本
func MigrateTo(ctx context.Context, db *sql.DB, migrationsDir string, version int64) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	if err := goose.UpToContext(ctx, db, migrationsDir, version); err != nil {
		return fmt.Errorf("迁移到版本 %d 失败: %w", version, err)
	}

	return nil
}

// MigrateDownTo 回滚到指定版本
func MigrateDownTo(ctx context.Context, db *sql.DB, migrationsDir string, version int64) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	if err := goose.DownToContext(ctx, db, migrationsDir, version); err != nil {
		return fmt.Errorf("回滚到版本 %d 失败: %w", version, err)
	}

	return nil
}

// GetStatus 获取迁移状态（打印到标准输出）
func GetStatus(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	// StatusContext 直接打印状态到标准输出
	if err := goose.StatusContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("获取迁移状态失败: %w", err)
	}

	return nil
}

// GetCurrentVersion 获取当前迁移版本
func GetCurrentVersion(ctx context.Context, db *sql.DB) (int64, error) {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, fmt.Errorf("设置数据库方言失败: %w", err)
	}

	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, fmt.Errorf("获取当前版本失败: %w", err)
	}

	return version, nil
}

// GetMigrationsDir 获取迁移目录路径
func GetMigrationsDir() (string, error) {
	// 尝试从工作目录查找
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}

	// 检查多个可能的位置
	possibleDirs := []string{
		filepath.Join(wd, "migrations"),
		filepath.Join(wd, "..", "migrations"),
		filepath.Join(wd, "..", "..", "migrations"),
	}

	for _, dir := range possibleDirs {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("找不到迁移目录")
}

