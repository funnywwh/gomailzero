package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SaveTOTPSecret 保存 TOTP 密钥
func (d *SQLiteDriver) SaveTOTPSecret(ctx context.Context, userEmail string, secret string) error {
	query := `
		INSERT INTO totp_secrets (user_email, secret, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_email) DO UPDATE SET
			secret = excluded.secret,
			updated_at = excluded.updated_at
	`
	now := time.Now()
	_, err := d.db.ExecContext(ctx, query, userEmail, secret, now, now)
	if err != nil {
		return fmt.Errorf("保存 TOTP 密钥失败: %w", err)
	}
	return nil
}

// GetTOTPSecret 获取 TOTP 密钥
func (d *SQLiteDriver) GetTOTPSecret(ctx context.Context, userEmail string) (string, error) {
	query := `
		SELECT secret
		FROM totp_secrets
		WHERE user_email = ?
	`
	var secret string
	err := d.db.QueryRowContext(ctx, query, userEmail).Scan(&secret)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("用户未启用 TOTP")
		}
		return "", fmt.Errorf("获取 TOTP 密钥失败: %w", err)
	}
	return secret, nil
}

// DeleteTOTPSecret 删除 TOTP 密钥
func (d *SQLiteDriver) DeleteTOTPSecret(ctx context.Context, userEmail string) error {
	query := `
		DELETE FROM totp_secrets
		WHERE user_email = ?
	`
	_, err := d.db.ExecContext(ctx, query, userEmail)
	if err != nil {
		return fmt.Errorf("删除 TOTP 密钥失败: %w", err)
	}
	return nil
}

// IsTOTPEnabled 检查用户是否启用了 TOTP
func (d *SQLiteDriver) IsTOTPEnabled(ctx context.Context, userEmail string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM totp_secrets
		WHERE user_email = ?
	`
	var count int
	err := d.db.QueryRowContext(ctx, query, userEmail).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("检查 TOTP 状态失败: %w", err)
	}
	return count > 0, nil
}

