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
			if err := os.MkdirAll(dir, 0755); err != nil {
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
		INSERT INTO users (email, password_hash, quota, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	active := 0
	if user.Active {
		active = 1
	}
	_, err := d.db.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Quota,
		active,
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
		SELECT id, email, password_hash, quota, active, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	row := d.db.QueryRowContext(ctx, query, email)

	var user User
	var active int
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Quota,
		&active,
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
	return &user, nil
}

// UpdateUser 更新用户
func (d *SQLiteDriver) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET email = ?, password_hash = ?, quota = ?, active = ?, updated_at = ?
		WHERE id = ?
	`
	active := 0
	if user.Active {
		active = 1
	}
	_, err := d.db.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Quota,
		active,
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
		SELECT id, email, password_hash, quota, active, created_at, updated_at
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
		var active int
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.Quota,
			&active,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描用户失败: %w", err)
		}
		user.Active = active == 1
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
		mail.ReceivedAt,
		now,
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
	var toAddrs, flags string
	err := row.Scan(
		&mail.ID,
		&mail.UserEmail,
		&mail.Folder,
		&mail.From,
		&toAddrs,
		&mail.Cc,
		&mail.Bcc,
		&mail.Subject,
		&mail.Size,
		&flags,
		&mail.ReceivedAt,
		&mail.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("邮件不存在: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("查询邮件失败: %w", err)
	}

	// 解析字符串为切片
	if toAddrs != "" {
		mail.To = []string{toAddrs} // 简化处理
	}
	if flags != "" {
		mail.Flags = []string{flags} // 简化处理
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

	var mails []*Mail
	for rows.Next() {
		var mail Mail
		var toAddrs, flags string
		if err := rows.Scan(
			&mail.ID,
			&mail.UserEmail,
			&mail.Folder,
			&mail.From,
			&toAddrs,
			&mail.Cc,
			&mail.Bcc,
			&mail.Subject,
			&mail.Size,
			&flags,
			&mail.ReceivedAt,
			&mail.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描邮件失败: %w", err)
		}

		if toAddrs != "" {
			mail.To = []string{toAddrs}
		}
		if flags != "" {
			mail.Flags = []string{flags}
		}

		mails = append(mails, &mail)
	}

	return mails, nil
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

	var mails []*Mail
	for rows.Next() {
		var mail Mail
		var toAddrs, flags string
		if err := rows.Scan(
			&mail.ID,
			&mail.UserEmail,
			&mail.Folder,
			&mail.From,
			&toAddrs,
			&mail.Cc,
			&mail.Bcc,
			&mail.Subject,
			&mail.Size,
			&flags,
			&mail.ReceivedAt,
			&mail.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描邮件失败: %w", err)
		}

		if toAddrs != "" {
			mail.To = strings.Split(toAddrs, ",")
		}
		if flags != "" {
			mail.Flags = strings.Split(flags, ",")
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
