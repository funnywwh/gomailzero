package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomailzero/gmz/internal/migrate"
	_ "modernc.org/sqlite"
)

// SQLiteDriver SQLite 存储驱动
type SQLiteDriver struct {
	db *sql.DB
}

// NewSQLiteDriver 创建 SQLite 驱动
func NewSQLiteDriver(dsn string) (*SQLiteDriver, error) {
	// 对于非内存数据库，确保目录存在
	if dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
		dir := filepath.Dir(dsn)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0750); err != nil { // 使用 0750 权限（仅所有者可读写执行，组可读执行）
				return nil, fmt.Errorf("创建数据库目录失败: %w", err)
			}
		}
	}

	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 设置连接参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	driver := &SQLiteDriver{db: db}

	return driver, nil
}

// RunMigrations 执行数据库迁移
func (d *SQLiteDriver) RunMigrations(ctx context.Context, migrationsDir string, autoMigrate bool) error {
	if !autoMigrate {
		// 如果未启用自动迁移，使用旧的 initSchema 方法（向后兼容）
		return d.initSchema()
	}

	// 使用 goose 执行迁移
	if err := migrate.Migrate(ctx, d.db, migrationsDir, "up"); err != nil {
		return fmt.Errorf("执行数据库迁移失败: %w", err)
	}

	return nil
}

// GetDB 获取数据库连接（用于迁移）
func (d *SQLiteDriver) GetDB() *sql.DB {
	return d.db
}

// initSchema 初始化数据库表结构
func (d *SQLiteDriver) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		quota INTEGER DEFAULT 0,
		active INTEGER DEFAULT 1,
		is_admin INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS domains (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS aliases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_addr TEXT UNIQUE NOT NULL,
		to_addr TEXT NOT NULL,
		domain TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS mails (
		id TEXT PRIMARY KEY,
		user_email TEXT NOT NULL,
		folder TEXT NOT NULL,
		from_addr TEXT NOT NULL,
		to_addrs TEXT NOT NULL,
		cc_addrs TEXT,
		bcc_addrs TEXT,
		subject TEXT,
		size INTEGER NOT NULL,
		flags TEXT,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS totp_secrets (
		user_email TEXT PRIMARY KEY,
		secret TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_email) REFERENCES users(email) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_mails_user_folder ON mails(user_email, folder);
	CREATE INDEX IF NOT EXISTS idx_mails_received_at ON mails(received_at);
	CREATE INDEX IF NOT EXISTS idx_aliases_from ON aliases(from_addr);
	CREATE INDEX IF NOT EXISTS idx_aliases_domain ON aliases(domain);
	`

	_, err := d.db.Exec(schema)
	return err
}

// CreateUser 创建用户
func (d *SQLiteDriver) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (email, password_hash, quota, active, is_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	active := 0
	if user.Active {
		active = 1
	}
	isAdmin := 0
	if user.IsAdmin {
		isAdmin = 1
	}
	_, err := d.db.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Quota,
		active,
		isAdmin,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// GetUser 获取用户
func (d *SQLiteDriver) GetUser(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, quota, active, is_admin, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	row := d.db.QueryRowContext(ctx, query, email)

	var user User
	var active, isAdmin int
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Quota,
		&active,
		&isAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户不存在: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	user.Active = active == 1
	user.IsAdmin = isAdmin == 1
	return &user, nil
}

// UpdateUser 更新用户
func (d *SQLiteDriver) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET email = ?, password_hash = ?, quota = ?, active = ?, is_admin = ?, updated_at = ?
		WHERE id = ?
	`
	active := 0
	if user.Active {
		active = 1
	}
	isAdmin := 0
	if user.IsAdmin {
		isAdmin = 1
	}
	_, err := d.db.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Quota,
		active,
		isAdmin,
		time.Now(),
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("更新用户失败: %w", err)
	}
	return nil
}

// DeleteUser 删除用户
func (d *SQLiteDriver) DeleteUser(ctx context.Context, email string) error {
	query := `DELETE FROM users WHERE email = ?`
	_, err := d.db.ExecContext(ctx, query, email)
	if err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}
	return nil
}

// ListUsers 列出用户
func (d *SQLiteDriver) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, email, password_hash, quota, active, is_admin, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := d.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var active, isAdmin int
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.Quota,
			&active,
			&isAdmin,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描用户失败: %w", err)
		}
		user.Active = active == 1
		user.IsAdmin = isAdmin == 1
		users = append(users, &user)
	}

	return users, nil
}

// CreateDomain 创建域名
func (d *SQLiteDriver) CreateDomain(ctx context.Context, domain *Domain) error {
	query := `
		INSERT INTO domains (name, active, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`
	now := time.Now()
	active := 0
	if domain.Active {
		active = 1
	}
	_, err := d.db.ExecContext(ctx, query,
		domain.Name,
		active,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("创建域名失败: %w", err)
	}
	return nil
}

// GetDomain 获取域名
func (d *SQLiteDriver) GetDomain(ctx context.Context, name string) (*Domain, error) {
	query := `
		SELECT id, name, active, created_at, updated_at
		FROM domains
		WHERE name = ?
	`
	row := d.db.QueryRowContext(ctx, query, name)

	var domain Domain
	var active int
	err := row.Scan(
		&domain.ID,
		&domain.Name,
		&active,
		&domain.CreatedAt,
		&domain.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("域名不存在: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("查询域名失败: %w", err)
	}

	domain.Active = active == 1
	return &domain, nil
}

// UpdateDomain 更新域名
func (d *SQLiteDriver) UpdateDomain(ctx context.Context, domain *Domain) error {
	query := `
		UPDATE domains
		SET name = ?, active = ?, updated_at = ?
		WHERE id = ?
	`
	active := 0
	if domain.Active {
		active = 1
	}
	_, err := d.db.ExecContext(ctx, query,
		domain.Name,
		active,
		time.Now(),
		domain.ID,
	)
	if err != nil {
		return fmt.Errorf("更新域名失败: %w", err)
	}
	return nil
}

// DeleteDomain 删除域名
func (d *SQLiteDriver) DeleteDomain(ctx context.Context, name string) error {
	query := `DELETE FROM domains WHERE name = ?`
	_, err := d.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("删除域名失败: %w", err)
	}
	return nil
}

// ListDomains 列出域名
func (d *SQLiteDriver) ListDomains(ctx context.Context) ([]*Domain, error) {
	query := `
		SELECT id, name, active, created_at, updated_at
		FROM domains
		ORDER BY name
	`
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询域名列表失败: %w", err)
	}
	defer rows.Close()

	var domains []*Domain
	for rows.Next() {
		var domain Domain
		var active int
		if err := rows.Scan(
			&domain.ID,
			&domain.Name,
			&active,
			&domain.CreatedAt,
			&domain.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描域名失败: %w", err)
		}
		domain.Active = active == 1
		domains = append(domains, &domain)
	}

	return domains, nil
}

// CreateAlias 创建别名
func (d *SQLiteDriver) CreateAlias(ctx context.Context, alias *Alias) error {
	query := `
		INSERT INTO aliases (from_addr, to_addr, domain, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := d.db.ExecContext(ctx, query,
		alias.From,
		alias.To,
		alias.Domain,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("创建别名失败: %w", err)
	}
	return nil
}

// GetAlias 获取别名
func (d *SQLiteDriver) GetAlias(ctx context.Context, from string) (*Alias, error) {
	query := `
		SELECT id, from_addr, to_addr, domain, created_at
		FROM aliases
		WHERE from_addr = ?
	`
	row := d.db.QueryRowContext(ctx, query, from)

	var alias Alias
	err := row.Scan(
		&alias.ID,
		&alias.From,
		&alias.To,
		&alias.Domain,
		&alias.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("别名不存在: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("查询别名失败: %w", err)
	}

	return &alias, nil
}

// DeleteAlias 删除别名
func (d *SQLiteDriver) DeleteAlias(ctx context.Context, from string) error {
	query := `DELETE FROM aliases WHERE from_addr = ?`
	_, err := d.db.ExecContext(ctx, query, from)
	if err != nil {
		return fmt.Errorf("删除别名失败: %w", err)
	}
	return nil
}

// ListAliases 列出别名
func (d *SQLiteDriver) ListAliases(ctx context.Context, domain string) ([]*Alias, error) {
	query := `
		SELECT id, from_addr, to_addr, domain, created_at
		FROM aliases
		WHERE domain = ?
		ORDER BY from_addr
	`
	rows, err := d.db.QueryContext(ctx, query, domain)
	if err != nil {
		return nil, fmt.Errorf("查询别名列表失败: %w", err)
	}
	defer rows.Close()

	var aliases []*Alias
	for rows.Next() {
		var alias Alias
		if err := rows.Scan(
			&alias.ID,
			&alias.From,
			&alias.To,
			&alias.Domain,
			&alias.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描别名失败: %w", err)
		}
		aliases = append(aliases, &alias)
	}

	return aliases, nil
}

// StoreMail 存储邮件（仅元数据，邮件体由 Maildir 存储）
func (d *SQLiteDriver) StoreMail(ctx context.Context, mail *Mail) error {
	query := `
		INSERT INTO mails (id, user_email, folder, from_addr, to_addrs, cc_addrs, bcc_addrs, subject, size, flags, received_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// 将切片转换为字符串（简单实现，实际应该使用 JSON）
	toAddrs := ""
	if len(mail.To) > 0 {
		toAddrs = mail.To[0]
		for i := 1; i < len(mail.To); i++ {
			toAddrs += "," + mail.To[i]
		}
	}

	flags := ""
	if len(mail.Flags) > 0 {
		flags = mail.Flags[0]
		for i := 1; i < len(mail.Flags); i++ {
			flags += "," + mail.Flags[i]
		}
	}

	now := time.Now()
	// 将时间格式化为 SQLite 兼容的格式（RFC3339）
	receivedAtStr := mail.ReceivedAt.Format(time.RFC3339)
	createdAtStr := now.Format(time.RFC3339)

	_, err := d.db.ExecContext(ctx, query,
		mail.ID,
		mail.UserEmail,
		mail.Folder,
		mail.From,
		toAddrs,
		"", // cc_addrs
		"", // bcc_addrs
		mail.Subject,
		mail.Size,
		flags,
		receivedAtStr,
		createdAtStr,
	)
	if err != nil {
		return fmt.Errorf("存储邮件失败: %w", err)
	}
	return nil
}

// GetMail 获取邮件
func (d *SQLiteDriver) GetMail(ctx context.Context, id string) (*Mail, error) {
	query := `
		SELECT id, user_email, folder, from_addr, to_addrs, cc_addrs, bcc_addrs, subject, size, flags, received_at, created_at
		FROM mails
		WHERE id = ?
	`
	row := d.db.QueryRowContext(ctx, query, id)

	var mail Mail
	var toAddrs, ccAddrs, bccAddrs, flags string
	var receivedAtStr, createdAtStr string
	err := row.Scan(
		&mail.ID,
		&mail.UserEmail,
		&mail.Folder,
		&mail.From,
		&toAddrs,
		&ccAddrs,
		&bccAddrs,
		&mail.Subject,
		&mail.Size,
		&flags,
		&receivedAtStr,
		&createdAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("邮件不存在: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("查询邮件失败: %w", err)
	}

	// 解析 to_addrs（用逗号分割）
	if toAddrs != "" {
		mail.To = strings.Split(toAddrs, ",")
		// 去除空格
		for i := range mail.To {
			mail.To[i] = strings.TrimSpace(mail.To[i])
		}
	}
	// 解析 cc_addrs（用逗号分割）
	if ccAddrs != "" {
		mail.Cc = strings.Split(ccAddrs, ",")
		// 去除空格
		for i := range mail.Cc {
			mail.Cc[i] = strings.TrimSpace(mail.Cc[i])
		}
	}
	// 解析 bcc_addrs（用逗号分割）
	if bccAddrs != "" {
		mail.Bcc = strings.Split(bccAddrs, ",")
		// 去除空格
		for i := range mail.Bcc {
			mail.Bcc[i] = strings.TrimSpace(mail.Bcc[i])
		}
	}
	// 解析 flags（用逗号分割）
	if flags != "" {
		mail.Flags = strings.Split(flags, ",")
		// 去除空格
		for i := range mail.Flags {
			mail.Flags[i] = strings.TrimSpace(mail.Flags[i])
		}
	}

	// 解析时间字符串
	if receivedAtStr != "" {
		if t := parseTimeString(receivedAtStr); !t.IsZero() {
			mail.ReceivedAt = t
		}
	}
	if createdAtStr != "" {
		if t := parseTimeString(createdAtStr); !t.IsZero() {
			mail.CreatedAt = t
		}
	}

	return &mail, nil
}

// GetMailBody 获取邮件体（从 Maildir 读取）
// 注意：SQLite 驱动不直接存储邮件体，需要从 Maildir 读取
// 这个方法需要 Maildir 实例，但当前架构中 Maildir 是独立的
// 暂时返回错误，实际应该通过组合或依赖注入的方式访问 Maildir
func (d *SQLiteDriver) GetMailBody(ctx context.Context, userEmail string, folder string, mailID string) ([]byte, error) {
	// TODO: 需要 Maildir 实例来读取邮件体
	// 当前实现返回错误，实际应该：
	// 1. 通过依赖注入获取 Maildir 实例
	// 2. 或者将 Maildir 作为 SQLiteDriver 的字段
	return nil, fmt.Errorf("GetMailBody 需要 Maildir 实例，当前未实现")
}

// ListMails 列出邮件
func (d *SQLiteDriver) ListMails(ctx context.Context, userEmail string, folder string, limit, offset int) ([]*Mail, error) {
	query := `
		SELECT id, user_email, folder, from_addr, to_addrs, cc_addrs, bcc_addrs, subject, size, flags, received_at, created_at
		FROM mails
		WHERE user_email = ? AND folder = ?
		ORDER BY received_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := d.db.QueryContext(ctx, query, userEmail, folder, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询邮件列表失败: %w", err)
	}
	defer rows.Close()

	mails := make([]*Mail, 0) // 初始化为空切片，而不是 nil
	for rows.Next() {
		var mail Mail
		var toAddrs, ccAddrs, bccAddrs, flags string
		var receivedAtStr, createdAtStr string
		if err := rows.Scan(
			&mail.ID,
			&mail.UserEmail,
			&mail.Folder,
			&mail.From,
			&toAddrs,
			&ccAddrs,
			&bccAddrs,
			&mail.Subject,
			&mail.Size,
			&flags,
			&receivedAtStr,
			&createdAtStr,
		); err != nil {
			return nil, fmt.Errorf("扫描邮件失败: %w", err)
		}

		// 解析 to_addrs（用逗号分割）
		if toAddrs != "" {
			mail.To = strings.Split(toAddrs, ",")
			// 去除空格
			for i := range mail.To {
				mail.To[i] = strings.TrimSpace(mail.To[i])
			}
		}
		// 解析 cc_addrs（用逗号分割）
		if ccAddrs != "" {
			mail.Cc = strings.Split(ccAddrs, ",")
			// 去除空格
			for i := range mail.Cc {
				mail.Cc[i] = strings.TrimSpace(mail.Cc[i])
			}
		}
		// 解析 bcc_addrs（用逗号分割）
		if bccAddrs != "" {
			mail.Bcc = strings.Split(bccAddrs, ",")
			// 去除空格
			for i := range mail.Bcc {
				mail.Bcc[i] = strings.TrimSpace(mail.Bcc[i])
			}
		}
		// 解析 flags（用逗号分割）
		if flags != "" {
			mail.Flags = strings.Split(flags, ",")
			// 去除空格
			for i := range mail.Flags {
				mail.Flags[i] = strings.TrimSpace(mail.Flags[i])
			}
		}

		// 解析时间字符串
		if receivedAtStr != "" {
			if t := parseTimeString(receivedAtStr); !t.IsZero() {
				mail.ReceivedAt = t
			}
		}
		if createdAtStr != "" {
			if t := parseTimeString(createdAtStr); !t.IsZero() {
				mail.CreatedAt = t
			}
		}

		mails = append(mails, &mail)
	}

	return mails, nil
}

// parseTimeString 解析时间字符串，支持多种格式（向后兼容）
func parseTimeString(timeStr string) time.Time {
	// 尝试 RFC3339 格式（标准格式）
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t
	}

	// 尝试 RFC3339Nano 格式
	if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
		return t
	}

	// 尝试 Go time.Time.String() 格式（包含 m=+xxx 调试信息）
	// 格式：2006-01-02 15:04:05.999999999 -0700 MST m=+xxx
	if idx := strings.Index(timeStr, " m="); idx > 0 {
		timeStr = timeStr[:idx]
		if t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", timeStr); err == nil {
			return t
		}
	}

	// 尝试标准格式（无时区）
	if t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", timeStr); err == nil {
		return t
	}

	// 尝试简单格式
	if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
		return t
	}

	// 如果所有格式都失败，返回零值时间
	return time.Time{}
}

// SearchMails 搜索邮件
func (d *SQLiteDriver) SearchMails(ctx context.Context, userEmail string, query string, folder string, limit, offset int) ([]*Mail, error) {
	sqlQuery := `
		SELECT id, user_email, folder, from_addr, to_addrs, cc_addrs, bcc_addrs, subject, size, flags, received_at, created_at
		FROM mails
		WHERE user_email = ? AND (subject LIKE ? OR from_addr LIKE ? OR to_addrs LIKE ?)
	`
	args := []interface{}{userEmail, "%" + query + "%", "%" + query + "%", "%" + query + "%"}

	if folder != "" {
		sqlQuery += " AND folder = ?"
		args = append(args, folder)
	}

	sqlQuery += " ORDER BY received_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := d.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("搜索邮件失败: %w", err)
	}
	defer rows.Close()

	mails := make([]*Mail, 0) // 初始化为空切片，而不是 nil
	for rows.Next() {
		var mail Mail
		var toAddrs, ccAddrs, bccAddrs, flags string
		var receivedAtStr, createdAtStr string
		if err := rows.Scan(
			&mail.ID,
			&mail.UserEmail,
			&mail.Folder,
			&mail.From,
			&toAddrs,
			&ccAddrs,
			&bccAddrs,
			&mail.Subject,
			&mail.Size,
			&flags,
			&receivedAtStr,
			&createdAtStr,
		); err != nil {
			return nil, fmt.Errorf("扫描邮件失败: %w", err)
		}

		// 解析 to_addrs（用逗号分割）
		if toAddrs != "" {
			mail.To = strings.Split(toAddrs, ",")
			// 去除空格
			for i := range mail.To {
				mail.To[i] = strings.TrimSpace(mail.To[i])
			}
		}
		// 解析 cc_addrs（用逗号分割）
		if ccAddrs != "" {
			mail.Cc = strings.Split(ccAddrs, ",")
			// 去除空格
			for i := range mail.Cc {
				mail.Cc[i] = strings.TrimSpace(mail.Cc[i])
			}
		}
		// 解析 bcc_addrs（用逗号分割）
		if bccAddrs != "" {
			mail.Bcc = strings.Split(bccAddrs, ",")
			// 去除空格
			for i := range mail.Bcc {
				mail.Bcc[i] = strings.TrimSpace(mail.Bcc[i])
			}
		}
		// 解析 flags（用逗号分割）
		if flags != "" {
			mail.Flags = strings.Split(flags, ",")
			// 去除空格
			for i := range mail.Flags {
				mail.Flags[i] = strings.TrimSpace(mail.Flags[i])
			}
		}

		// 解析时间字符串
		if receivedAtStr != "" {
			if t := parseTimeString(receivedAtStr); !t.IsZero() {
				mail.ReceivedAt = t
			}
		}
		if createdAtStr != "" {
			if t := parseTimeString(createdAtStr); !t.IsZero() {
				mail.CreatedAt = t
			}
		}

		mails = append(mails, &mail)
	}

	return mails, nil
}

// ListFolders 列出文件夹
func (d *SQLiteDriver) ListFolders(ctx context.Context, userEmail string) ([]string, error) {
	query := `
		SELECT DISTINCT folder
		FROM mails
		WHERE user_email = ?
		ORDER BY folder
	`
	rows, err := d.db.QueryContext(ctx, query, userEmail)
	if err != nil {
		return nil, fmt.Errorf("查询文件夹列表失败: %w", err)
	}
	defer rows.Close()

	var folders []string
	// 添加默认文件夹
	folders = append(folders, "INBOX", "Sent", "Drafts", "Trash", "Spam")

	folderMap := make(map[string]bool)
	for _, f := range folders {
		folderMap[f] = true
	}

	for rows.Next() {
		var folder string
		if err := rows.Scan(&folder); err != nil {
			return nil, fmt.Errorf("扫描文件夹失败: %w", err)
		}
		if !folderMap[folder] {
			folders = append(folders, folder)
			folderMap[folder] = true
		}
	}

	return folders, nil
}

// DeleteMail 删除邮件
func (d *SQLiteDriver) DeleteMail(ctx context.Context, id string) error {
	query := `DELETE FROM mails WHERE id = ?`
	_, err := d.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除邮件失败: %w", err)
	}
	return nil
}

// UpdateMailFlags 更新邮件标志
func (d *SQLiteDriver) UpdateMailFlags(ctx context.Context, id string, flags []string) error {
	flagsStr := ""
	if len(flags) > 0 {
		flagsStr = flags[0]
		for i := 1; i < len(flags); i++ {
			flagsStr += "," + flags[i]
		}
	}

	query := `UPDATE mails SET flags = ? WHERE id = ?`
	_, err := d.db.ExecContext(ctx, query, flagsStr, id)
	if err != nil {
		return fmt.Errorf("更新邮件标志失败: %w", err)
	}
	return nil
}

// GetQuota 获取配额
func (d *SQLiteDriver) GetQuota(ctx context.Context, userEmail string) (*Quota, error) {
	query := `
		SELECT quota, COALESCE(SUM(size), 0) as used
		FROM users
		LEFT JOIN mails ON users.email = mails.user_email
		WHERE users.email = ?
		GROUP BY users.email, users.quota
	`
	row := d.db.QueryRowContext(ctx, query, userEmail)

	var quota Quota
	quota.UserEmail = userEmail
	err := row.Scan(&quota.Limit, &quota.Used)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户不存在: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("查询配额失败: %w", err)
	}

	return &quota, nil
}

// UpdateQuota 更新配额
func (d *SQLiteDriver) UpdateQuota(ctx context.Context, userEmail string, quota *Quota) error {
	query := `UPDATE users SET quota = ? WHERE email = ?`
	_, err := d.db.ExecContext(ctx, query, quota.Limit, userEmail)
	if err != nil {
		return fmt.Errorf("更新配额失败: %w", err)
	}
	return nil
}

// Close 关闭连接
func (d *SQLiteDriver) Close() error {
	return d.db.Close()
}

// ErrNotFound 未找到错误
var ErrNotFound = fmt.Errorf("not found")
