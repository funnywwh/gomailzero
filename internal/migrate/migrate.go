package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	// 由于 goose v3.25.0 和 v3.26.0 都有重复版本检测的 bug
	// 我们创建一个临时目录，只包含 .up.sql 文件，避免 bug
	tmpDir, err := createFilteredMigrationsDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("创建过滤后的迁移目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 执行迁移
	switch direction {
	case "up":
		// 使用过滤后的目录（只包含 .up.sql 文件）
		if err := goose.UpContext(ctx, db, tmpDir); err != nil {
			return fmt.Errorf("执行迁移失败: %w", err)
		}
	case "down":
		// 对于 down，我们需要包含 .down.sql 文件
		// 但 goose 的 bug 会导致问题，所以暂时不支持 down
		return fmt.Errorf("由于 goose 的 bug，暂时不支持 down 操作，请使用 auto_migrate: false")
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

	// 创建过滤后的迁移目录
	tmpDir, err := createFilteredMigrationsDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("创建过滤后的迁移目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := goose.UpToContext(ctx, db, tmpDir, version); err != nil {
		return fmt.Errorf("迁移到版本 %d 失败: %w", version, err)
	}

	return nil
}

// MigrateDownTo 回滚到指定版本
func MigrateDownTo(ctx context.Context, db *sql.DB, migrationsDir string, version int64) error {
	// 由于 goose 的 bug，暂时不支持 down 操作
	return fmt.Errorf("由于 goose 的 bug，暂时不支持 down 操作")
}

// GetStatus 获取迁移状态（打印到标准输出）
func GetStatus(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("设置数据库方言失败: %w", err)
	}

	// 创建过滤后的迁移目录
	tmpDir, err := createFilteredMigrationsDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("创建过滤后的迁移目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// StatusContext 直接打印状态到标准输出
	if err := goose.StatusContext(ctx, db, tmpDir); err != nil {
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

// createFilteredMigrationsDir 创建只包含 .up.sql 文件的临时目录
// 这样可以避免 goose 的重复版本检测 bug
func createFilteredMigrationsDir(sourceDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "gmz-migrations-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 读取源目录中的所有文件
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("读取迁移目录失败: %w", err)
	}

	// 只复制 .up.sql 文件
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		// 只处理 .up.sql 文件
		if !strings.HasSuffix(fileName, ".up.sql") {
			continue
		}

		// 读取源文件
		sourcePath := filepath.Join(sourceDir, fileName)
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("读取迁移文件失败 %s: %w", sourcePath, err)
		}

		// 写入临时目录
		targetPath := filepath.Join(tmpDir, fileName)
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("写入临时文件失败 %s: %w", targetPath, err)
		}
	}

	return tmpDir, nil
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
