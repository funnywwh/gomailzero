package imapd

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-message"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Backend IMAP 后端
type Backend struct {
	storage storage.Driver
	maildir *storage.Maildir // Maildir 实例，用于读取邮件体
	auth    Authenticator
}

// NewBackend 创建后端
func NewBackend(storage storage.Driver, maildir *storage.Maildir, auth Authenticator) *Backend {
	return &Backend{
		storage: storage,
		maildir: maildir,
		auth:    auth,
	}
}

// stableUIDFromID 将字符串 ID 映射为 uint32（临时方案）
func stableUIDFromID(id string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(id))
	return h.Sum32()
}

// Login 登录
func (b *Backend) Login(conn *imap.ConnInfo, username, password string) (backend.User, error) {
	ctx := context.Background()
	user, err := b.auth.Authenticate(ctx, username, password)
	if err != nil {
		return nil, fmt.Errorf("认证失败")
	}

	return NewUser(b.storage, b.maildir, user), nil
}

// User IMAP 用户
type User struct {
	storage storage.Driver
	maildir *storage.Maildir
	user    *storage.User
}

// NewUser 创建用户
func NewUser(storage storage.Driver, maildir *storage.Maildir, user *storage.User) *User {
	return &User{
		storage: storage,
		maildir: maildir,
		user:    user,
	}
}

// Username 返回用户名
func (u *User) Username() string {
	return u.user.Email
}

// ListMailboxes 列出邮箱
func (u *User) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	...
}

// GetMailbox 获取邮箱
func (u *User) GetMailbox(name string) (backend.Mailbox, error) {
	...
}

// CreateMailbox 创建邮箱
func (u *User) CreateMailbox(name string) error {
	return nil
}