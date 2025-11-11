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
	var results []uint32

	for i, mail := range m.mails {
		seqNum := uint32(i + 1)
		matched := true

		// 检查搜索条件
		if criteria != nil {
			// 检查标志
			if len(criteria.Flag) > 0 {
				hasFlag := false
				for _, searchFlag := range criteria.Flag {
					for _, mailFlag := range mail.Flags {
						if mailFlag == searchFlag {
							hasFlag = true
							break
						}
					}
					if hasFlag {
						break
					}
				}
				if !hasFlag {
					matched = false
				}
			}

			// 检查未读（\Seen 标志）
			if criteria.NotFlag != nil {
				for _, notFlag := range criteria.NotFlag {
					if notFlag == imap.SeenFlag {
						hasSeen := false
						for _, mailFlag := range mail.Flags {
							if mailFlag == imap.SeenFlag {
								hasSeen = true
								break
							}
						}
						if hasSeen {
							matched = false
							break
						}
					}
				}
			}

			// 检查主题（简化实现）
			if criteria.Header != nil {
				for key, values := range criteria.Header {
					if key == "Subject" {
						subjectMatched := false
						for _, value := range values {
							if contains(mail.Subject, value) {
								subjectMatched = true
								break
							}
						}
						if !subjectMatched {
							matched = false
							break
						}
					}
				}
			}
		}

		if matched {
			results = append(results, seqNum)
		}
	}

	return results, nil
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	// 使用简单的字符串包含检查（区分大小写）
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// CreateMessage 创建邮件
func (m *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	// TODO: 实现创建邮件功能
	return fmt.Errorf("未实现")
}

// AddFlags 添加标志
func (m *Mailbox) AddFlags(uid bool, seqSet *imap.SeqSet, flags []string) error {
	return m.UpdateMessagesFlags(uid, seqSet, imap.AddFlags, flags)
}

// SetFlags 设置标志
func (m *Mailbox) SetFlags(uid bool, seqSet *imap.SeqSet, flags []string) error {
	return m.UpdateMessagesFlags(uid, seqSet, imap.SetFlags, flags)
}

// StoreFlags 存储标志
func (m *Mailbox) StoreFlags(uid bool, seqSet *imap.SeqSet, flags []string, op imap.FlagsOp) error {
	return m.UpdateMessagesFlags(uid, seqSet, op, flags)
}

// UpdateMessagesFlags 更新消息标志
func (m *Mailbox) UpdateMessagesFlags(uid bool, seqSet *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	ctx := context.Background()

	// 遍历序列集
	for i, mail := range m.mails {
		seqNum := uint32(i + 1)
		if seqSet != nil && !seqSet.Contains(seqNum) {
			continue
		}

		var newFlags []string
		switch op {
		case imap.AddFlags:
			// 添加标志
			flagMap := make(map[string]bool)
			for _, f := range mail.Flags {
				flagMap[f] = true
			}
			for _, f := range flags {
				flagMap[f] = true
			}
			newFlags = make([]string, 0, len(flagMap))
			for f := range flagMap {
				newFlags = append(newFlags, f)
			}
		case imap.SetFlags:
			// 设置标志
			newFlags = flags
		case imap.RemoveFlags:
			// 移除标志
			flagMap := make(map[string]bool)
			for _, f := range mail.Flags {
				flagMap[f] = true
			}
			for _, f := range flags {
				delete(flagMap, f)
			}
			newFlags = make([]string, 0, len(flagMap))
			for f := range flagMap {
				newFlags = append(newFlags, f)
			}
		}

		// 更新存储
		if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
			return fmt.Errorf("更新邮件标志失败: %w", err)
		}

		// 更新内存中的标志
		mail.Flags = newFlags
	}

	return nil
}

// CopyMessages 复制邮件到目标邮箱
func (m *Mailbox) CopyMessages(uid bool, seqSet *imap.SeqSet, dest string) error {
	ctx := context.Background()

	// 获取目标邮箱的邮件列表
	destMails, err := m.storage.ListMails(ctx, m.userEmail, dest, 1000, 0)
	if err != nil {
		// 如果目标邮箱不存在，创建空列表
		destMails = []*storage.Mail{}
	}

	// 复制选中的邮件
	for i, mail := range m.mails {
		seqNum := uint32(i + 1)
		if seqSet != nil && !seqSet.Contains(seqNum) {
			continue
		}

		// 创建新邮件副本
		newMail := &storage.Mail{
			UserEmail:  mail.UserEmail,
			Folder:     dest,
			From:       mail.From,
			To:         mail.To,
			Cc:         mail.Cc,
			Bcc:        mail.Bcc,
			Subject:    mail.Subject,
			Body:       mail.Body,
			Size:       mail.Size,
			Flags:      []string{}, // 新邮件没有标志
			ReceivedAt: mail.ReceivedAt,
			CreatedAt:  time.Now(),
		}

		// 生成新 ID
		newMail.ID = fmt.Sprintf("%s-%d", dest, len(destMails)+1)

		// 存储到目标邮箱
		if err := m.storage.StoreMail(ctx, newMail); err != nil {
			return fmt.Errorf("复制邮件失败: %w", err)
		}
	}

	return nil
}

// Expunge 删除邮件（标记为 \Deleted 的邮件）
func (m *Mailbox) Expunge() error {
	ctx := context.Background()

	var toDelete []string
	for _, mail := range m.mails {
		// 检查是否有 \Deleted 标志
		for _, flag := range mail.Flags {
			if flag == imap.DeletedFlag {
				toDelete = append(toDelete, mail.ID)
				break
			}
		}
	}

	// 删除邮件
	for _, id := range toDelete {
		if err := m.storage.DeleteMail(ctx, id); err != nil {
			return fmt.Errorf("删除邮件失败: %w", err)
		}
	}

	// 从内存中移除
	var remaining []*storage.Mail
	for _, mail := range m.mails {
		hasDeleted := false
		for _, flag := range mail.Flags {
			if flag == imap.DeletedFlag {
				hasDeleted = true
				break
			}
		}
		if !hasDeleted {
			remaining = append(remaining, mail)
		}
	}
	m.mails = remaining

	return nil
}
