package storage

import (
	"context"
	"time"
)

// Driver 存储驱动接口
type Driver interface {
	// 用户管理
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, email string) error
	ListUsers(ctx context.Context, limit, offset int) ([]*User, error)

	// 域名管理
	CreateDomain(ctx context.Context, domain *Domain) error
	GetDomain(ctx context.Context, name string) (*Domain, error)
	UpdateDomain(ctx context.Context, domain *Domain) error
	DeleteDomain(ctx context.Context, name string) error
	ListDomains(ctx context.Context) ([]*Domain, error)

	// 别名管理
	CreateAlias(ctx context.Context, alias *Alias) error
	GetAlias(ctx context.Context, from string) (*Alias, error)
	DeleteAlias(ctx context.Context, from string) error
	ListAliases(ctx context.Context, domain string) ([]*Alias, error)

	// 邮件管理
	StoreMail(ctx context.Context, mail *Mail) error
	GetMail(ctx context.Context, id string) (*Mail, error)
	ListMails(ctx context.Context, userEmail string, folder string, limit, offset int) ([]*Mail, error)
	DeleteMail(ctx context.Context, id string) error
	UpdateMailFlags(ctx context.Context, id string, flags []string) error

	// 配额管理
	GetQuota(ctx context.Context, userEmail string) (*Quota, error)
	UpdateQuota(ctx context.Context, userEmail string, quota *Quota) error

	// 关闭连接
	Close() error
}

// User 用户
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`     // 不序列化
	Quota        int64     `json:"quota"` // 字节数，0 表示无限制
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Active       bool      `json:"active"`
}

// Domain 域名
type Domain struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Alias 别名
type Alias struct {
	ID        int64     `json:"id"`
	From      string    `json:"from"` // 源地址
	To        string    `json:"to"`   // 目标地址
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

// Mail 邮件
type Mail struct {
	ID         string    `json:"id"`
	UserEmail  string    `json:"user_email"`
	Folder     string    `json:"folder"` // INBOX, Sent, Drafts, etc.
	From       string    `json:"from"`
	To         []string  `json:"to"`
	Cc         []string  `json:"cc"`
	Bcc        []string  `json:"bcc"`
	Subject    string    `json:"subject"`
	Body       []byte    `json:"-"` // 邮件体（加密存储）
	Size       int64     `json:"size"`
	Flags      []string  `json:"flags"` // \Seen, \Answered, \Flagged, etc.
	ReceivedAt time.Time `json:"received_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// Quota 配额
type Quota struct {
	UserEmail string `json:"user_email"`
	Used      int64  `json:"used"`  // 已使用字节数
	Limit     int64  `json:"limit"` // 限制字节数，0 表示无限制
}
