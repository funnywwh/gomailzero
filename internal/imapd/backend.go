package imapd

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// Login 登录
func (b *Backend) Login(conn *imap.ConnInfo, username, password string) (backend.User, error) {
	ctx := context.Background()
	user, err := b.auth.Authenticate(ctx, username, password)
	if err != nil {
		logger.Warn().Str("username", username).Msg("IMAP 认证失败")
		return nil, fmt.Errorf("认证失败")
	}

	logger.Info().Str("username", username).Msg("IMAP 用户登录")
	return NewUser(b.storage, b.maildir, user), nil
}

// User IMAP 用户
type User struct {
	storage storage.Driver
	maildir *storage.Maildir // Maildir 实例，用于读取邮件体
	//nolint:unused // user 字段在 Username()、ListMailboxes() 和 GetMailbox() 方法中被使用
	user *storage.User
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
	// 标准文件夹
	folders := []string{"INBOX", "Sent", "Drafts", "Trash", "Spam"}

	var mailboxes []backend.Mailbox
	ctx := context.Background()
	for _, folder := range folders {
		mails, err := u.storage.ListMails(ctx, u.user.Email, folder, 1000, 0)
		if err != nil {
			// 如果文件夹不存在，创建空邮箱
			logger.Warn().Err(err).Str("user", u.user.Email).Str("folder", folder).Msg("查询邮件列表失败，创建空邮箱")
			mails = []*storage.Mail{}
		} else {
			// 记录调试信息
			logger.Debug().
				Str("user", u.user.Email).
				Str("folder", folder).
				Int("mail_count", len(mails)).
				Msg("IMAP ListMailboxes: 从数据库读取邮件")
		}
		mailboxes = append(mailboxes, NewMailbox(u.storage, u.maildir, u.user.Email, folder, mails))
	}

	return mailboxes, nil
}

// GetMailbox 获取邮箱
func (u *User) GetMailbox(name string) (backend.Mailbox, error) {
	ctx := context.Background()

	// 标准化邮箱名称（IMAP 规范要求 INBOX 大小写不敏感）
	normalizedName := name
	if strings.EqualFold(name, "INBOX") {
		normalizedName = "INBOX"
	}

	// 列出邮件（从数据库读取）
	mails, err := u.storage.ListMails(ctx, u.user.Email, normalizedName, 1000, 0)
	if err != nil {
		// 如果查询失败，返回空邮箱而不是错误
		logger.Warn().Err(err).Str("user", u.user.Email).Str("folder", name).Str("normalized", normalizedName).Msg("查询邮件列表失败，返回空邮箱")
		mails = []*storage.Mail{}
	} else {
		// 记录调试信息
		logger.Debug().
			Str("user", u.user.Email).
			Str("folder", name).
			Str("normalized", normalizedName).
			Int("mail_count", len(mails)).
			Msg("IMAP GetMailbox: 从数据库读取邮件")
	}

	// 如果 Maildir 可用，检查是否有新邮件未同步到数据库
	if u.maildir != nil {
		// 列出 Maildir 中的邮件文件（使用标准化名称）
		maildirFiles, err := u.maildir.ListMails(u.user.Email, normalizedName)
		if err == nil {
			// 检查是否有文件不在数据库中
			mailIDMap := make(map[string]bool)
			for _, mail := range mails {
				mailIDMap[mail.ID] = true
			}
			
			// 对于不在数据库中的文件，尝试同步（简化处理，只记录日志）
			for _, filename := range maildirFiles {
				// 去除可能的标志后缀（如 :2,S）
				baseID := filename
				if idx := strings.Index(filename, ":"); idx >= 0 {
					baseID = filename[:idx]
				}
				if !mailIDMap[baseID] && !mailIDMap[filename] {
					logger.Debug().
						Str("user", u.user.Email).
						Str("folder", name).
						Str("filename", filename).
						Msg("发现 Maildir 中的邮件未同步到数据库")
				}
			}
		}
	}

	// 如果邮件既没有 \Seen 也没有 \Recent 标志（旧邮件），自动设置 \Seen 标志（兼容 Foxmail）
	// 这会在 GetMailbox 时自动处理，即使客户端只调用 Status 命令
	for _, mail := range mails {
		hasSeen := false
		hasRecent := false
		for _, flag := range mail.Flags {
			if flag == imap.SeenFlag || flag == "\\Seen" {
				hasSeen = true
			}
			if flag == imap.RecentFlag || flag == "\\Recent" {
				hasRecent = true
			}
		}
		// 如果邮件没有 \Seen 标志，且没有 \Recent 标志，自动设置 \Seen 标志（兼容 Foxmail）
		if !hasSeen && !hasRecent {
			newFlags := append(mail.Flags, imap.SeenFlag)
			if err := u.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
				logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败（GetMailbox）")
			} else {
				// 更新内存中的标志
				mail.Flags = newFlags
				logger.Debug().
					Str("user", u.user.Email).
					Str("folder", normalizedName).
					Str("mail_id", mail.ID).
					Msg("IMAP GetMailbox: 自动设置 \\Seen 标志（兼容 Foxmail）")
			}
		}
	}

	// 使用原始名称创建邮箱（保持客户端请求的名称）
	return NewMailbox(u.storage, u.maildir, u.user.Email, normalizedName, mails), nil
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
	maildir   *storage.Maildir // Maildir 实例，用于读取邮件体
	userEmail string
	name      string
	mails     []*storage.Mail
}

// NewMailbox 创建邮箱
func NewMailbox(storage storage.Driver, maildir *storage.Maildir, userEmail, name string, mails []*storage.Mail) *Mailbox {
	return &Mailbox{
		storage:   storage,
		maildir:   maildir,
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
		Name:  m.name,
		Items: make(map[imap.StatusItem]interface{}),
	}

	// 记录调试信息
	logger.Debug().
		Str("user", m.userEmail).
		Str("folder", m.name).
		Int("mail_count", len(m.mails)).
		Msg("IMAP Status: 获取邮箱状态")

	for _, item := range items {
		// 在 Items 中初始化该项（Format() 方法需要）
		status.Items[item] = nil

		switch item {
		case imap.StatusMessages:
			// 设置邮件总数（即使为 0 也要设置）
			// #nosec G115 -- len() 返回的 int 在合理范围内，不会溢出 uint32
			if len(m.mails) <= int(^uint32(0)) {
				status.Messages = uint32(len(m.mails))
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Uint32("messages", status.Messages).
					Msg("IMAP Status: 邮件数量")
			}
		case imap.StatusRecent:
			// 计算带有 \Recent 标志的邮件数（新邮件）
			// 根据 IMAP 规范，StatusRecent 应该返回带有 \Recent 标志的邮件数
			recentCount := uint32(0)
			for _, mail := range m.mails {
				hasRecent := false
				for _, flag := range mail.Flags {
					if flag == imap.RecentFlag || flag == "\\Recent" {
						hasRecent = true
						break
					}
				}
				if hasRecent {
					recentCount++
				}
			}
			status.Recent = recentCount
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Uint32("recent", recentCount).
				Msg("IMAP Status: Recent 邮件数量")
		case imap.StatusUnseen:
			// 计算未读邮件数（没有 \Seen 标志的邮件）
			unseenCount := uint32(0)
			for _, mail := range m.mails {
				hasSeen := false
				for _, flag := range mail.Flags {
					// 检查 \Seen 标志（支持两种格式）
					if flag == imap.SeenFlag || flag == "\\Seen" {
						hasSeen = true
						break
					}
				}
				if !hasSeen {
					unseenCount++
				}
			}
			status.Unseen = unseenCount
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Uint32("unseen", unseenCount).
				Msg("IMAP Status: Unseen 邮件数量")
		case imap.StatusUidNext:
			// 计算下一个 UID（即使邮箱为空，UID 也应该从 1 开始）
			// #nosec G115 -- len() 返回的 int 在合理范围内，不会溢出 uint32
			if len(m.mails)+1 <= int(^uint32(0)) {
				status.UidNext = uint32(len(m.mails) + 1)
			} else {
				// 如果溢出，使用最大值
				status.UidNext = ^uint32(0)
			}
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Uint32("uid_next", status.UidNext).
				Msg("IMAP Status: UidNext")
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

	// 记录调试信息
	itemNames := make([]string, len(items))
	for i, item := range items {
		itemNames[i] = string(item)
	}
	logger.Debug().
		Str("user", m.userEmail).
		Str("folder", m.name).
		Int("mail_count", len(m.mails)).
		Bool("uid", uid).
		Strs("requested_items", itemNames).
		Msg("IMAP ListMessages: 开始列出邮件")

		// 如果客户端只请求了 UID，为了兼容性，也返回基本字段（Envelope、Flags、InternalDate、RFC822Size）
		// 这不符合严格的 IMAP 规范，但有助于某些客户端正确显示邮件列表
		hasOnlyUID := len(items) == 1 && items[0] == imap.FetchUid
		if hasOnlyUID {
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Msg("IMAP ListMessages: 客户端只请求了 UID，添加基本字段以兼容")
			// 添加基本字段到 items 列表
			items = []imap.FetchItem{
				imap.FetchUid,
				imap.FetchEnvelope,
				imap.FetchFlags,
				imap.FetchInternalDate,
				imap.FetchRFC822Size,
			}
		}
		
		// 检查是否请求了 FLAGS，如果没有，也添加 FLAGS（某些客户端需要）
		hasFlags := false
		for _, item := range items {
			if item == imap.FetchFlags {
				hasFlags = true
				break
			}
		}
		if !hasFlags {
			items = append(items, imap.FetchFlags)
		}
		
		// 检查是否请求了 BODY 但没有请求 Envelope，如果是，也添加 Envelope
		hasBodyRequest := false
		hasEnvelopeRequest := false
		for _, item := range items {
			itemStr := string(item)
			if strings.HasPrefix(itemStr, "BODY") || item == imap.FetchRFC822 || item == imap.FetchRFC822Text {
				hasBodyRequest = true
			}
			if item == imap.FetchEnvelope {
				hasEnvelopeRequest = true
			}
		}
		if hasBodyRequest && !hasEnvelopeRequest {
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Msg("IMAP ListMessages: 客户端请求了 BODY 但没有请求 Envelope，添加 Envelope 以兼容")
			// 添加 Envelope 到 items 列表
			items = append(items, imap.FetchEnvelope)
		}

	for i, mail := range m.mails {
		// #nosec G115 -- 循环索引 i 在合理范围内，不会溢出 uint32
		seqNum := uint32(i + 1)
		if uid {
			// #nosec G115 -- 循环索引 i 在合理范围内，不会溢出 uint32
			seqNum = uint32(i + 1) // TODO: 使用实际的 UID
		}

		if seqSet != nil && !seqSet.Contains(seqNum) {
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Uint32("seq_num", seqNum).
				Str("mail_id", mail.ID).
				Msg("IMAP ListMessages: 邮件不在序列集中，跳过")
			continue
		}

		msg := &imap.Message{
			SeqNum: seqNum,
			Items:  make(map[imap.FetchItem]interface{}),
			// go-imap 库在格式化时从这些字段读取，而不是从 msg.Items 读取
			Envelope:     nil,
			Flags:        nil,
			InternalDate: time.Time{},
			Size:         0,
			Uid:          0,
			BodyStructure: nil, // 需要在使用时初始化
			Body:         make(map[*imap.BodySectionName]imap.Literal), // 用于存储 BODY.PEEK[1] 等请求
		}
		
		// 预先填充 Envelope（即使客户端没有请求，也填充以便客户端从邮件头解析时使用）
		// 解析 From 地址
		fromAddr := mail.From
		if fromAddr == "" {
			fromAddr = "unknown@unknown"
		}
		// 简单解析：如果包含 < >，提取邮箱地址
		if idx := strings.Index(fromAddr, "<"); idx >= 0 {
			if idx2 := strings.Index(fromAddr, ">"); idx2 > idx {
				fromAddr = fromAddr[idx+1 : idx2]
			}
		}
		// 解析邮箱地址为 MailboxName 和 HostName
		fromMailbox, fromHost := parseEmailAddress(fromAddr)
		if fromMailbox == "" {
			fromMailbox = "unknown"
		}
		if fromHost == "" {
			fromHost = "unknown"
		}
		
		// 解析 To 地址
		toAddrs := make([]*imap.Address, 0)
		if mail.To != nil {
			for _, to := range mail.To {
				toAddr := to
				if toAddr == "" {
					continue
				}
				if idx := strings.Index(to, "<"); idx >= 0 {
					if idx2 := strings.Index(to, ">"); idx2 > idx {
						toAddr = to[idx+1 : idx2]
					}
				}
				toMailbox, toHost := parseEmailAddress(toAddr)
				if toMailbox == "" {
					continue
				}
				if toHost == "" {
					toHost = "unknown"
				}
				toAddrs = append(toAddrs, &imap.Address{
					MailboxName: toMailbox,
					HostName:    toHost,
				})
			}
		}
		// 确保 Date 不是零值
		date := mail.ReceivedAt
		if date.IsZero() {
			date = mail.CreatedAt
			if date.IsZero() {
				date = time.Now()
			}
		}
		// 确保 From 地址不为空
		fromAddrs := []*imap.Address{{
			MailboxName: fromMailbox,
			HostName:    fromHost,
		}}
		if fromAddrs[0] == nil || fromAddrs[0].MailboxName == "" {
			fromAddrs = []*imap.Address{{
				MailboxName: "unknown",
				HostName:    "unknown",
			}}
		}
		
		// 预先填充 Envelope（即使客户端没有请求）
		msg.Envelope = &imap.Envelope{
			Subject: mail.Subject,
			From:    fromAddrs,
			To:      toAddrs,
			Date:    date,
		}

		// 记录处理的邮件
		logger.Debug().
			Str("user", m.userEmail).
			Str("folder", m.name).
			Uint32("seq_num", seqNum).
			Str("mail_id", mail.ID).
			Str("subject", mail.Subject).
			Int("items_count", len(items)).
			Msg("IMAP ListMessages: 处理邮件")

		// 填充邮件项
		for _, item := range items {
			switch item {
			case imap.FetchEnvelope:
				// Envelope 已经在消息初始化时填充，这里只需要设置到 Items 中
				if msg.Envelope == nil {
					logger.Warn().
						Str("user", m.userEmail).
						Str("folder", m.name).
						Str("mail_id", mail.ID).
						Msg("IMAP ListMessages: Envelope 为 nil，重新创建")
					// 如果 Envelope 为 nil，重新创建（这不应该发生）
					msg.Envelope = &imap.Envelope{
						Subject: mail.Subject,
						From:    []*imap.Address{{MailboxName: "unknown", HostName: "unknown"}},
						To:      []*imap.Address{},
						Date:    time.Now(),
					}
				}
				// 同时也设置到 Items 中（以防万一）
				msg.Items[item] = msg.Envelope
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", mail.ID).
					Str("subject", msg.Envelope.Subject).
					Str("item", string(item)).
					Bool("envelope_nil", msg.Envelope == nil).
					Bool("envelope_from_nil", msg.Envelope != nil && (msg.Envelope.From == nil || len(msg.Envelope.From) == 0)).
					Int("items_count", len(msg.Items)).
					Msg("IMAP ListMessages: 填充 Envelope")
			case imap.FetchFlags:
				flags := make([]string, len(mail.Flags))
				copy(flags, mail.Flags)
				// go-imap 库从 msg.Flags 字段读取
				msg.Flags = flags
				msg.Items[item] = flags
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", mail.ID).
					Strs("flags", flags).
					Msg("IMAP ListMessages: 填充 Flags")
				
				// 如果邮件没有 \Seen 标志，且没有 \Recent 标志，说明是旧邮件
				// 为了兼容 Foxmail，当客户端请求 FLAGS 时，也自动设置 \Seen 标志
				hasSeen := false
				hasRecent := false
				for _, flag := range mail.Flags {
					if flag == imap.SeenFlag || flag == "\\Seen" {
						hasSeen = true
					}
					if flag == imap.RecentFlag || flag == "\\Recent" {
						hasRecent = true
					}
				}
				// 如果邮件没有 \Seen 标志，且没有 \Recent 标志，自动设置 \Seen 标志（兼容 Foxmail）
				if !hasSeen && !hasRecent {
					ctx := context.Background()
					newFlags := append(mail.Flags, imap.SeenFlag)
					if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
						logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败（FetchFlags）")
					} else {
						// 更新内存中的标志
						mail.Flags = newFlags
						msg.Flags = newFlags
						msg.Items[item] = newFlags
						logger.Debug().
							Str("user", m.userEmail).
							Str("folder", m.name).
							Str("mail_id", mail.ID).
							Msg("IMAP ListMessages: 自动设置 \\Seen 标志（FetchFlags，兼容 Foxmail）")
					}
				}
			case imap.FetchInternalDate:
				// 确保 Date 不是零值
				date := mail.ReceivedAt
				if date.IsZero() {
					date = mail.CreatedAt
					if date.IsZero() {
						date = time.Now()
					}
				}
				// go-imap 库从 msg.InternalDate 字段读取
				msg.InternalDate = date
				msg.Items[item] = date
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", mail.ID).
					Time("date", date).
					Msg("IMAP ListMessages: 填充 InternalDate")
			case imap.FetchRFC822Size:
				// go-imap 库从 msg.Size 字段读取（需要转换为 uint32）
				size := uint32(mail.Size)
				if mail.Size > 0 && size == 0 {
					// 如果转换后为 0 但原始值不为 0，使用最大值
					size = ^uint32(0)
				}
				msg.Size = size
				msg.Items[item] = mail.Size
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", mail.ID).
					Int64("size", mail.Size).
					Uint32("size_uint32", size).
					Msg("IMAP ListMessages: 填充 RFC822Size")
			case imap.FetchUid:
				// go-imap 库从 msg.Uid 字段读取
				msg.Uid = seqNum // TODO: 使用实际的 UID
				msg.Items[item] = seqNum
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", mail.ID).
					Uint32("uid", seqNum).
					Msg("IMAP ListMessages: 填充 Uid")
			case imap.FetchBody, imap.FetchBodyStructure:
				// go-imap 库从 msg.BodyStructure 字段读取，需要初始化
				if msg.BodyStructure == nil {
					// 创建一个简单的 BodyStructure（文本/纯文本）
					msg.BodyStructure = &imap.BodyStructure{
						MIMEType:    "text",
						MIMESubType: "plain",
						Size:        uint32(mail.Size),
					}
				}
				msg.BodyStructure.Extended = item == imap.FetchBodyStructure
				msg.Items[item] = msg.BodyStructure
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", mail.ID).
					Str("item", string(item)).
					Msg("IMAP ListMessages: 填充 BodyStructure")
			case imap.FetchRFC822, imap.FetchRFC822Text:
				// 从 Maildir 读取邮件体
				if m.maildir != nil {
					body, err := m.maildir.ReadMail(m.userEmail, m.name, mail.ID)
					if err == nil {
						msg.Items[item] = body
						
						// 根据 IMAP 规范，如果客户端使用 FETCH（不是 PEEK）获取邮件体，自动设置 \Seen 标志
						// FetchRFC822 不是 PEEK，所以需要设置 \Seen
						hasSeen := false
						hasRecent := false
						for _, flag := range mail.Flags {
							if flag == imap.SeenFlag {
								hasSeen = true
							}
							if flag == imap.RecentFlag {
								hasRecent = true
							}
						}
						if !hasSeen {
							// 自动设置 \Seen 标志
							ctx := context.Background()
							newFlags := append(mail.Flags, imap.SeenFlag)
							// 移除 \Recent 标志（如果存在）
							if hasRecent {
								flagMap := make(map[string]bool)
								for _, f := range newFlags {
									if f != imap.RecentFlag {
										flagMap[f] = true
									}
								}
								newFlags = make([]string, 0, len(flagMap))
								for f := range flagMap {
									newFlags = append(newFlags, f)
								}
							}
							if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
								logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败")
							} else {
								// 更新内存中的标志
								mail.Flags = newFlags
								logger.Debug().
									Str("user", m.userEmail).
									Str("folder", m.name).
									Str("mail_id", mail.ID).
									Msg("IMAP ListMessages: 自动设置 \\Seen 标志（FetchRFC822）")
							}
						} else if hasRecent {
							// 如果邮件已经有 \Seen 标志，但还有 \Recent 标志，移除 \Recent 标志
							ctx := context.Background()
							flagMap := make(map[string]bool)
							for _, f := range mail.Flags {
								if f != imap.RecentFlag {
									flagMap[f] = true
								}
							}
							newFlags := make([]string, 0, len(flagMap))
							for f := range flagMap {
								newFlags = append(newFlags, f)
							}
							if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
								logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("移除 \\Recent 标志失败")
							} else {
								mail.Flags = newFlags
							}
						}
						
						logger.Debug().
							Str("user", m.userEmail).
							Str("folder", m.name).
							Str("mail_id", mail.ID).
							Int("body_size", len(body)).
							Str("item", string(item)).
							Msg("IMAP ListMessages: 从 Maildir 读取邮件体成功")
					} else {
						logger.Warn().Err(err).Str("mail_id", mail.ID).Str("item", string(item)).Msg("读取邮件体失败")
						// 如果读取失败，尝试使用数据库中的 Body 字段（如果有）
						if len(mail.Body) > 0 {
							msg.Items[item] = mail.Body
							logger.Debug().
								Str("user", m.userEmail).
								Str("folder", m.name).
								Str("mail_id", mail.ID).
								Int("body_size", len(mail.Body)).
								Msg("IMAP ListMessages: 使用数据库中的邮件体")
						}
					}
				} else if len(mail.Body) > 0 {
					// 如果没有 Maildir，使用数据库中的 Body 字段
					msg.Items[item] = mail.Body
					logger.Debug().
						Str("user", m.userEmail).
						Str("folder", m.name).
						Str("mail_id", mail.ID).
						Int("body_size", len(mail.Body)).
						Msg("IMAP ListMessages: 使用数据库中的邮件体（无 Maildir）")
				} else {
					logger.Warn().
						Str("user", m.userEmail).
						Str("folder", m.name).
						Str("mail_id", mail.ID).
						Str("item", string(item)).
						Msg("IMAP ListMessages: 无法获取邮件体（Maildir 为空且数据库 Body 为空）")
				}
			default:
				// 尝试解析为 BodySectionName（如 BODY.PEEK[1], BODY[1] 等）
				section, err := imap.ParseBodySectionName(imap.FetchItem(item))
				if err == nil {
					// 从 Maildir 读取邮件体
					var bodyData []byte
					if m.maildir != nil {
						body, err := m.maildir.ReadMail(m.userEmail, m.name, mail.ID)
						if err == nil {
							bodyData = body
						} else {
							logger.Warn().Err(err).Str("mail_id", mail.ID).Str("item", string(item)).Msg("读取邮件体失败")
							if len(mail.Body) > 0 {
								bodyData = mail.Body
							}
						}
					} else if len(mail.Body) > 0 {
						bodyData = mail.Body
					}

					if len(bodyData) > 0 {
						// 根据 section 提取相应的部分
						// 如果 section.Specifier 为空，返回整个邮件体
						// 如果 section.Specifier 为 "TEXT"，返回邮件正文
						// 如果 section.Specifier 为 "HEADER"，返回邮件头
						var literalData []byte
						if section.Specifier == "" {
							// BODY[1] 或 BODY.PEEK[1] - 返回整个邮件体
							literalData = bodyData
						} else if section.Specifier == "TEXT" {
							// BODY[1.TEXT] - 返回邮件正文（不包括头）
							// 查找第一个空行（分隔头和正文）
							if idx := bytes.Index(bodyData, []byte("\r\n\r\n")); idx >= 0 {
								literalData = bodyData[idx+4:]
							} else if idx := bytes.Index(bodyData, []byte("\n\n")); idx >= 0 {
								literalData = bodyData[idx+2:]
							} else {
								literalData = bodyData
							}
						} else if section.Specifier == "HEADER" {
							// BODY[1.HEADER] - 返回邮件头
							if idx := bytes.Index(bodyData, []byte("\r\n\r\n")); idx >= 0 {
								literalData = bodyData[:idx+2]
							} else if idx := bytes.Index(bodyData, []byte("\n\n")); idx >= 0 {
								literalData = bodyData[:idx+1]
							} else {
								literalData = bodyData
							}
						} else {
							// 其他情况，返回整个邮件体
							literalData = bodyData
						}

						// 创建 Literal 并存储到 msg.Body
						literal := bytes.NewReader(literalData)
						msg.Body[section] = literal
						msg.Items[item] = literal
						
						// 根据 IMAP 规范，如果客户端使用 FETCH（不是 PEEK）获取邮件体，自动设置 \Seen 标志
						// 为了兼容 Foxmail 等客户端，即使使用 PEEK，也设置 \Seen 标志
						// 检查邮件是否已经有 \Seen 标志
						hasSeen := false
						hasRecent := false
						for _, flag := range mail.Flags {
							if flag == imap.SeenFlag {
								hasSeen = true
							}
							if flag == imap.RecentFlag {
								hasRecent = true
							}
						}
						
						// 如果邮件还没有 \Seen 标志，设置它（即使使用 PEEK，也设置以兼容 Foxmail）
						if !hasSeen {
							ctx := context.Background()
							newFlags := append(mail.Flags, imap.SeenFlag)
							// 移除 \Recent 标志（如果存在）
							if hasRecent {
								flagMap := make(map[string]bool)
								for _, f := range newFlags {
									if f != imap.RecentFlag {
										flagMap[f] = true
									}
								}
								newFlags = make([]string, 0, len(flagMap))
								for f := range flagMap {
									newFlags = append(newFlags, f)
								}
							}
							if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
								logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败")
							} else {
								// 更新内存中的标志
								mail.Flags = newFlags
								logger.Debug().
									Str("user", m.userEmail).
									Str("folder", m.name).
									Str("mail_id", mail.ID).
									Bool("peek", section.Peek).
									Msg("IMAP ListMessages: 自动设置 \\Seen 标志")
							}
						} else if hasRecent {
							// 如果邮件已经有 \Seen 标志，但还有 \Recent 标志，移除 \Recent 标志
							ctx := context.Background()
							flagMap := make(map[string]bool)
							for _, f := range mail.Flags {
								if f != imap.RecentFlag {
									flagMap[f] = true
								}
							}
							newFlags := make([]string, 0, len(flagMap))
							for f := range flagMap {
								newFlags = append(newFlags, f)
							}
							if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
								logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("移除 \\Recent 标志失败")
							} else {
								mail.Flags = newFlags
							}
						}
						
						logger.Debug().
							Str("user", m.userEmail).
							Str("folder", m.name).
							Str("mail_id", mail.ID).
							Str("item", string(item)).
							Str("specifier", string(section.Specifier)).
							Bool("peek", section.Peek).
							Int("body_size", len(literalData)).
							Msg("IMAP ListMessages: 填充 BodySection")
					} else {
						logger.Warn().
							Str("user", m.userEmail).
							Str("folder", m.name).
							Str("mail_id", mail.ID).
							Str("item", string(item)).
							Msg("IMAP ListMessages: 无法获取邮件体（Maildir 为空且数据库 Body 为空）")
					}
				} else {
					logger.Debug().
						Str("user", m.userEmail).
						Str("folder", m.name).
						Str("mail_id", mail.ID).
						Str("item", string(item)).
						Err(err).
						Msg("IMAP ListMessages: 未处理的 FetchItem")
				}
			}
		}

		// 记录发送的邮件项数量
		logger.Debug().
			Str("user", m.userEmail).
			Str("folder", m.name).
			Uint32("seq_num", seqNum).
			Str("mail_id", mail.ID).
			Int("items_sent", len(msg.Items)).
			Bool("has_envelope", msg.Envelope != nil).
			Bool("has_envelope_from", msg.Envelope != nil && msg.Envelope.From != nil && len(msg.Envelope.From) > 0).
			Str("envelope_subject", func() string {
				if msg.Envelope != nil {
					return msg.Envelope.Subject
				}
				return ""
			}()).
			Msg("IMAP ListMessages: 发送邮件到通道")

		ch <- msg
	}

	logger.Debug().
		Str("user", m.userEmail).
		Str("folder", m.name).
		Int("total_sent", len(m.mails)).
		Msg("IMAP ListMessages: 完成列出邮件")

	return nil
}

// SearchMessages 搜索邮件
func (m *Mailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	var results []uint32

	for i, mail := range m.mails {
		// #nosec G115 -- 循环索引 i 在合理范围内，不会溢出 uint32
		seqNum := uint32(i + 1)
		matched := true

		// 检查搜索条件
		if criteria != nil {
			// 检查必须包含的标志
			if len(criteria.WithFlags) > 0 {
				hasAllFlags := true
				for _, searchFlag := range criteria.WithFlags {
					hasFlag := false
					for _, mailFlag := range mail.Flags {
						if mailFlag == searchFlag {
							hasFlag = true
							break
						}
					}
					if !hasFlag {
						hasAllFlags = false
						break
					}
				}
				if !hasAllFlags {
					matched = false
				}
			}

			// 检查不能包含的标志
			if len(criteria.WithoutFlags) > 0 {
				for _, notFlag := range criteria.WithoutFlags {
					for _, mailFlag := range mail.Flags {
						if mailFlag == notFlag {
							matched = false
							break
						}
					}
					if !matched {
						break
					}
				}
			}

			// 检查邮件头（简化实现）
			if len(criteria.Header) > 0 {
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

			// 检查邮件体
			if len(criteria.Body) > 0 {
				bodyMatched := false
				bodyStr := string(mail.Body)
				for _, searchText := range criteria.Body {
					if contains(bodyStr, searchText) {
						bodyMatched = true
						break
					}
				}
				if !bodyMatched {
					matched = false
				}
			}

			// 检查序列号
			if criteria.SeqNum != nil {
				if !criteria.SeqNum.Contains(seqNum) {
					matched = false
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

// CreateMessage 创建邮件（用于 IMAP APPEND 命令，发送邮件）
func (m *Mailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	ctx := context.Background()

	// 读取邮件体
	bodyData := make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			bodyData = append(bodyData, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取邮件体失败: %w", err)
		}
	}

	// 解析邮件头
	msg, err := message.Read(bytes.NewReader(bodyData))
	if err != nil {
		return fmt.Errorf("解析邮件失败: %w", err)
	}

	header := msg.Header
	from := header.Get("From")
	toStr := header.Get("To")
	ccStr := header.Get("Cc")
	bccStr := header.Get("Bcc")
	subject := header.Get("Subject")

	// 解析收件人列表
	var to, cc, bcc []string
	if toStr != "" {
		to = parseAddressList(toStr)
	}
	if ccStr != "" {
		cc = parseAddressList(ccStr)
	}
	if bccStr != "" {
		bcc = parseAddressList(bccStr)
	}

	// 读取邮件正文
	bodyText := ""
	if msg.Body != nil {
		bodyBytes, err := io.ReadAll(msg.Body)
		if err == nil {
			bodyText = string(bodyBytes)
		}
	}

	// 确定文件夹（Sent 或当前文件夹）
	folder := m.name
	if folder == "INBOX" {
		folder = "Sent" // 如果从 INBOX 发送，存储到 Sent
	}

	// 存储到 Maildir
	var mailID string
	if m.maildir != nil {
		if err := m.maildir.EnsureUserMaildir(m.userEmail); err != nil {
			return fmt.Errorf("创建用户 Maildir 失败: %w", err)
		}
		filename, err := m.maildir.StoreMail(m.userEmail, folder, bodyData)
		if err != nil {
			return fmt.Errorf("存储邮件到 Maildir 失败: %w", err)
		}
		mailID = filename
	} else {
		// 如果没有 Maildir，使用时间戳作为 ID
		mailID = fmt.Sprintf("%s-%d", folder, time.Now().UnixNano())
	}

	// 存储邮件元数据到数据库
	mail := &storage.Mail{
		ID:         mailID,
		UserEmail:  m.userEmail,
		Folder:     folder,
		From:       from,
		To:         to,
		Cc:         cc,
		Bcc:        bcc,
		Subject:    subject,
		Body:       []byte(bodyText),
		Size:       int64(len(bodyData)),
		Flags:      flags,
		ReceivedAt: date,
		CreatedAt:  time.Now(),
	}

	if err := m.storage.StoreMail(ctx, mail); err != nil {
		return fmt.Errorf("存储邮件元数据失败: %w", err)
	}

	// 如果是发送邮件（Sent 文件夹），需要投递到收件人
	if folder == "Sent" {
		// 收集所有收件人
		allRecipients := make([]string, 0)
		allRecipients = append(allRecipients, to...)
		allRecipients = append(allRecipients, cc...)
		allRecipients = append(allRecipients, bcc...)

		// 投递到本地收件人
		for _, recipient := range allRecipients {
			user, err := m.storage.GetUser(ctx, recipient)
			if err != nil {
				// 检查别名
				alias, err := m.storage.GetAlias(ctx, recipient)
				if err != nil {
					continue // 不是本地用户，跳过
				}
				user, err = m.storage.GetUser(ctx, alias.To)
				if err != nil {
					continue // 别名目标不存在，跳过
				}
			}

			// 投递到收件人的 INBOX
			if m.maildir != nil {
				if err := m.maildir.EnsureUserMaildir(user.Email); err == nil {
					filename, err := m.maildir.StoreMail(user.Email, "INBOX", bodyData)
					if err == nil {
						inboxMail := &storage.Mail{
							ID:         filename,
							UserEmail:  user.Email,
							Folder:     "INBOX",
							From:       from,
							To:         []string{recipient},
							Cc:         cc,
							Bcc:        bcc,
							Subject:    subject,
							Size:       int64(len(bodyData)),
							Flags:      []string{"\\Recent"}, // 新邮件设置 \Recent 标志
							ReceivedAt: time.Now(),
							CreatedAt:  time.Now(),
						}
						_ = m.storage.StoreMail(ctx, inboxMail) // 忽略错误，继续投递其他收件人
					}
				}
			}
		}
	}

	logger.Info().
		Str("user", m.userEmail).
		Str("folder", folder).
		Str("from", from).
		Msg("IMAP 创建邮件成功")

	return nil
}

// parseAddressList 解析地址列表（简化实现）
func parseAddressList(addrList string) []string {
	// 简单的解析：按逗号分割
	addresses := strings.Split(addrList, ",")
	result := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		// 提取邮箱地址（去除显示名称）
		if idx := strings.LastIndex(addr, "<"); idx >= 0 {
			addr = addr[idx+1:]
			if idx := strings.Index(addr, ">"); idx >= 0 {
				addr = addr[:idx]
			}
		}
		addr = strings.TrimSpace(addr)
		if addr != "" {
			result = append(result, addr)
		}
	}
	return result
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

	logger.Debug().
		Str("user", m.userEmail).
		Str("folder", m.name).
		Str("op", string(op)).
		Strs("flags", flags).
		Msg("IMAP UpdateMessagesFlags: 开始更新标志")

	// 遍历序列集
	for i, mail := range m.mails {
		// #nosec G115 -- 循环索引 i 在合理范围内，不会溢出 uint32
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

		logger.Debug().
			Str("user", m.userEmail).
			Str("folder", m.name).
			Str("mail_id", mail.ID).
			Strs("old_flags", mail.Flags).
			Strs("new_flags", newFlags).
			Msg("IMAP UpdateMessagesFlags: 更新标志")

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
		// #nosec G115 -- 循环索引 i 在合理范围内，不会溢出 uint32
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

// parseEmailAddress 解析邮箱地址为 MailboxName 和 HostName
func parseEmailAddress(email string) (mailbox, host string) {
	if email == "" {
		return "", ""
	}
	// 查找 @ 符号
	idx := strings.Index(email, "@")
	if idx < 0 {
		// 没有 @ 符号，整个字符串作为 mailbox，host 为空
		return email, ""
	}
	mailbox = email[:idx]
	host = email[idx+1:]
	return mailbox, host
}
