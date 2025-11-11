package imapd

import (
	"context"
	"fmt"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Backend IMAP 后端
type Backend struct {
	storage storage.Driver
	auth    Authenticator
}

// NewBackend 创建后端
func NewBackend(storage storage.Driver, auth Authenticator) *Backend {
	return &Backend{
		storage: storage,
		auth:    auth,
	}
}

// Login 登录
func (b *Backend) Login(conn *imap.ConnInfo, username, password string) (backend.User, error) {
	ctx := context.Background()
	user, err := b.auth.Authenticate(ctx, username, password)
	if err != nil {
		logger.Warn().Str("username", username).Msg("IMAP 认证失败")
		return nil, fmt.Errorf("认证失败")
	}

	logger.Info().Str("username", username).Msg("IMAP 用户登录")
	return NewUser(b.storage, user), nil
}

// User IMAP 用户
type User struct {
	storage storage.Driver
	user    *storage.User
}

// NewUser 创建用户
func NewUser(storage storage.Driver, user *storage.User) *User {
	return &User{
		storage: storage,
		user:    user,
	}
}

// Username 返回用户名
func (u *User) Username() string {
	return u.user.Email
}

// ListMailboxes 列出邮箱
func (u *User) ListMailboxes(subscribed bool) ([]backend.Mailbox, error) {
	// 标准文件夹
	folders := []string{"INBOX", "Sent", "Drafts", "Trash", "Spam"}

	var mailboxes []backend.Mailbox
	ctx := context.Background()
	for _, folder := range folders {
		mails, err := u.storage.ListMails(ctx, u.user.Email, folder, 1000, 0)
		if err != nil {
			// 如果文件夹不存在，创建空邮箱
			mails = []*storage.Mail{}
		}
		mailboxes = append(mailboxes, NewMailbox(u.storage, u.user.Email, folder, mails))
	}

	return mailboxes, nil
}

// GetMailbox 获取邮箱
func (u *User) GetMailbox(name string) (backend.Mailbox, error) {
	ctx := context.Background()

	// 列出邮件
	mails, err := u.storage.ListMails(ctx, u.user.Email, name, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("列出邮件失败: %w", err)
	}

	return NewMailbox(u.storage, u.user.Email, name, mails), nil
}

// CreateMailbox 创建邮箱
func (u *User) CreateMailbox(name string) error {
	// TODO: 实现创建邮箱功能
	return nil
}

// DeleteMailbox 删除邮箱
func (u *User) DeleteMailbox(name string) error {
	// TODO: 实现删除邮箱功能
	return nil
}

// Logout 登出
func (u *User) Logout() error {
	return nil
}

// RenameMailbox 重命名邮箱
func (u *User) RenameMailbox(existingName, newName string) error {
	// TODO: 实现重命名邮箱功能
	return nil
}

// Mailbox 邮箱
type Mailbox struct {
	storage   storage.Driver
	userEmail string
	name      string
	mails     []*storage.Mail
}

// NewMailbox 创建邮箱
func NewMailbox(storage storage.Driver, userEmail, name string, mails []*storage.Mail) *Mailbox {
	return &Mailbox{
		storage:   storage,
		userEmail: userEmail,
		name:      name,
		mails:     mails,
	}
}

// Name 返回邮箱名称
func (m *Mailbox) Name() string {
	return m.name
}

// Info 返回邮箱信息
func (m *Mailbox) Info() (*imap.MailboxInfo, error) {
	return &imap.MailboxInfo{
		Attributes: []string{imap.NoInferiorsAttr},
		Delimiter:  "/",
		Name:       m.name,
	}, nil
}

// Status 返回邮箱状态
func (m *Mailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := &imap.MailboxStatus{
		Name: m.name,
	}

	for _, item := range items {
		switch item {
		case imap.StatusMessages:
			status.Messages = uint32(len(m.mails))
		case imap.StatusRecent:
			// TODO: 计算未读邮件数
			status.Recent = 0
		case imap.StatusUnseen:
			// TODO: 计算未读邮件数
			status.Unseen = 0
		case imap.StatusUidNext:
			// TODO: 计算下一个 UID
			status.UidNext = uint32(len(m.mails) + 1)
		case imap.StatusUidValidity:
			status.UidValidity = 1
		}
	}

	return status, nil
}

// SetSubscribed 设置订阅状态
func (m *Mailbox) SetSubscribed(subscribed bool) error {
	// TODO: 实现订阅功能
	return nil
}

// Check 检查邮箱
func (m *Mailbox) Check() error {
	// TODO: 实现检查功能
	return nil
}

// ListMessages 列出邮件
func (m *Mailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	for i, mail := range m.mails {
		seqNum := uint32(i + 1)
		if uid {
			seqNum = uint32(i + 1) // TODO: 使用实际的 UID
		}

		if seqSet != nil && !seqSet.Contains(seqNum) {
			continue
		}

		msg := &imap.Message{
			SeqNum: seqNum,
			Items:  make(map[imap.FetchItem]interface{}),
		}

		// 填充邮件项
		for _, item := range items {
			switch item {
			case imap.FetchEnvelope:
				msg.Items[item] = &imap.Envelope{
					Subject: mail.Subject,
					From:    []*imap.Address{{MailboxName: mail.From}},
				}
			case imap.FetchFlags:
				flags := make([]string, len(mail.Flags))
				copy(flags, mail.Flags)
				msg.Items[item] = flags
			case imap.FetchInternalDate:
				msg.Items[item] = mail.ReceivedAt
			case imap.FetchRFC822Size:
				msg.Items[item] = mail.Size
			case imap.FetchUid:
				msg.Items[item] = seqNum // TODO: 使用实际的 UID
			}
		}

		ch <- msg
	}

	return nil
}

// SearchMessages 搜索邮件
func (m *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	// TODO: 实现搜索功能
	var results []uint32
	for i := range m.mails {
		results = append(results, uint32(i+1))
	}
	return results, nil
}

// CreateMessage 创建邮件
func (m *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	// TODO: 实现创建邮件功能
	return fmt.Errorf("未实现")
}

// AddFlags 添加标志
func (m *Mailbox) AddFlags(uid bool, seqSet *imap.SeqSet, flags []string) error {
	// TODO: 实现添加标志功能
	return fmt.Errorf("未实现")
}

// SetFlags 设置标志
func (m *Mailbox) SetFlags(uid bool, seqSet *imap.SeqSet, flags []string) error {
	// TODO: 实现设置标志功能
	return fmt.Errorf("未实现")
}

// StoreFlags 存储标志
func (m *Mailbox) StoreFlags(uid bool, seqSet *imap.SeqSet, flags []string, op imap.FlagsOp) error {
	// TODO: 实现存储标志功能
	return fmt.Errorf("未实现")
}

// UpdateMessagesFlags 更新消息标志
func (m *Mailbox) UpdateMessagesFlags(uid bool, seqSet *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	// TODO: 实现更新消息标志功能
	return fmt.Errorf("未实现")
}

// CopyMessages 复制邮件
func (m *Mailbox) CopyMessages(uid bool, seqSet *imap.SeqSet, dest string) error {
	// TODO: 实现复制邮件功能
	return fmt.Errorf("未实现")
}

// Expunge 删除邮件
func (m *Mailbox) Expunge() error {
	// TODO: 实现删除邮件功能
	return fmt.Errorf("未实现")
}
