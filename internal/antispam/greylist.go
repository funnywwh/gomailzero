package antispam

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Greylist 灰名单
type Greylist struct {
	db *sql.DB
}

// NewGreylist 创建灰名单
func NewGreylist(dsn string) (*Greylist, error) {
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	g := &Greylist{db: db}

	// 初始化表
	if err := g.initSchema(); err != nil {
		return nil, fmt.Errorf("初始化表结构失败: %w", err)
	}

	return g, nil
}

// initSchema 初始化表结构
func (g *Greylist) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS greylist (
		ip TEXT NOT NULL,
		sender TEXT NOT NULL,
		recipient TEXT NOT NULL,
		first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		count INTEGER DEFAULT 1,
		PRIMARY KEY (ip, sender, recipient)
	);

	CREATE INDEX IF NOT EXISTS idx_greylist_ip ON greylist(ip);
	CREATE INDEX IF NOT EXISTS idx_greylist_first_seen ON greylist(first_seen);
	`
	_, err := g.db.Exec(schema)
	return err
}

// Check 检查灰名单
func (g *Greylist) Check(ctx context.Context, ip, sender, recipient string) (bool, error) {
	now := time.Now()
	delay := 5 * time.Minute  // 延迟时间
	window := 4 * time.Hour    // 时间窗口

	// 查询记录
	var firstSeen, lastSeen time.Time
	var count int

	query := `
		SELECT first_seen, last_seen, count
		FROM greylist
		WHERE ip = ? AND sender = ? AND recipient = ?
	`

	err := g.db.QueryRowContext(ctx, query, ip, sender, recipient).Scan(&firstSeen, &lastSeen, &count)
	if err == sql.ErrNoRows {
		// 新记录，添加到灰名单
		_, err := g.db.ExecContext(ctx, `
			INSERT INTO greylist (ip, sender, recipient, first_seen, last_seen, count)
			VALUES (?, ?, ?, ?, ?, 1)
		`, ip, sender, recipient, now, now)
		if err != nil {
			return false, fmt.Errorf("插入灰名单记录失败: %w", err)
		}
		return false, nil // 拒绝（灰名单）
	}
	if err != nil {
		return false, fmt.Errorf("查询灰名单失败: %w", err)
	}

	// 更新记录
	_, err = g.db.ExecContext(ctx, `
		UPDATE greylist
		SET last_seen = ?, count = count + 1
		WHERE ip = ? AND sender = ? AND recipient = ?
	`, now, ip, sender, recipient)
	if err != nil {
		return false, fmt.Errorf("更新灰名单记录失败: %w", err)
	}

	// 检查是否在时间窗口内
	timeSinceFirst := now.Sub(firstSeen)
	if timeSinceFirst < delay {
		return false, nil // 仍在延迟期内，拒绝
	}

	if timeSinceFirst > window {
		// 超过时间窗口，重新开始
		_, err := g.db.ExecContext(ctx, `
			UPDATE greylist
			SET first_seen = ?, last_seen = ?, count = 1
			WHERE ip = ? AND sender = ? AND recipient = ?
		`, now, now, ip, sender, recipient)
		if err != nil {
			return false, fmt.Errorf("重置灰名单记录失败: %w", err)
		}
		return false, nil // 拒绝
	}

	// 在时间窗口内且超过延迟期，允许通过
	return true, nil
}

// Cleanup 清理过期记录
func (g *Greylist) Cleanup(ctx context.Context, maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	_, err := g.db.ExecContext(ctx, `
		DELETE FROM greylist
		WHERE last_seen < ?
	`, cutoff)
	return err
}

// Close 关闭连接
func (g *Greylist) Close() error {
	return g.db.Close()
}

