package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/migrate"
	_ "modernc.org/sqlite"
)

// handleMigrateCommand 处理迁移命令
func handleMigrateCommand(cmd, version, configPath string) error {
	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite", cfg.Storage.DSN+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer db.Close()

	// 获取迁移目录
	migrationsDir, err := migrate.GetMigrationsDir()
	if err != nil {
		return fmt.Errorf("获取迁移目录失败: %w", err)
	}

	ctx := context.Background()

	switch cmd {
	case "up":
		fmt.Println("执行数据库迁移...")
		if err := migrate.Migrate(ctx, db, migrationsDir, "up"); err != nil {
			return err
		}
		fmt.Println("迁移完成")

	case "down":
		fmt.Println("回滚数据库迁移...")
		if err := migrate.Migrate(ctx, db, migrationsDir, "down"); err != nil {
			return err
		}
		fmt.Println("回滚完成")

	case "status":
		if err := migrate.GetStatus(ctx, db, migrationsDir); err != nil {
			return err
		}

		currentVersion, err := migrate.GetCurrentVersion(ctx, db)
		if err != nil {
			fmt.Printf("当前版本: 无法获取\n")
		} else {
			fmt.Printf("当前版本: %d\n", currentVersion)
		}

	case "up-to":
		if version == "" {
			return fmt.Errorf("up-to 需要指定版本号")
		}
		ver, err := strconv.ParseInt(version, 10, 64)
		if err != nil {
			return fmt.Errorf("无效的版本号: %s", version)
		}
		fmt.Printf("迁移到版本 %d...\n", ver)
		if err := migrate.MigrateTo(ctx, db, migrationsDir, ver); err != nil {
			return err
		}
		fmt.Println("迁移完成")

	case "down-to":
		if version == "" {
			return fmt.Errorf("down-to 需要指定版本号")
		}
		ver, err := strconv.ParseInt(version, 10, 64)
		if err != nil {
			return fmt.Errorf("无效的版本号: %s", version)
		}
		fmt.Printf("回滚到版本 %d...\n", ver)
		if err := migrate.MigrateDownTo(ctx, db, migrationsDir, ver); err != nil {
			return err
		}
		fmt.Println("回滚完成")

	default:
		return fmt.Errorf("未知的迁移命令: %s (支持: up, down, status, up-to, down-to)", cmd)
	}

	return nil
}
