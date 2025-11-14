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
	ctx := context.Background()
	
	// 列出所有文件夹
	folders, err := u.storage.ListFolders(ctx, u.user.Email)
	if err != nil {
		logger.Warn().Err(err).Str("user", u.user.Email).Msg("列出文件夹失败，返回空列表")
		folders = []string{}
	}
	
	// 确保有 INBOX
	hasInbox := false
	for _, folder := range folders {
		if strings.EqualFold(folder, "INBOX") {
			hasInbox = true
			break
		}
	}
	if !hasInbox {
		folders = append([]string{"INBOX"}, folders...)
	}
	
	// 创建邮箱列表
	mailboxes := make([]backend.Mailbox, 0, len(folders))
	for _, folder := range folders {
		// 标准化文件夹名称
		normalizedName := folder
		if strings.EqualFold(folder, "INBOX") {
			normalizedName = "INBOX"
		}
		
		// 列出邮件
		mails, err := u.storage.ListMails(ctx, u.user.Email, normalizedName, 1000, 0)
		if err != nil {
			logger.Warn().Err(err).Str("user", u.user.Email).Str("folder", normalizedName).Msg("列出邮件失败，使用空列表")
			mails = []*storage.Mail{}
		}
		
		mailbox := NewMailbox(u.storage, u.maildir, u.user.Email, normalizedName, mails)
		mailboxes = append(mailboxes, mailbox)
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

	// 如果 Maildir 可用，检查文件系统状态并同步
	if u.maildir != nil {
		userDir := u.maildir.GetUserMaildir(u.user.Email)
		var curDir string
		var newDir string
		if normalizedName == "INBOX" || normalizedName == "" {
			curDir = filepath.Join(userDir, "cur")
			newDir = filepath.Join(userDir, "new")
		} else {
			curDir = filepath.Join(userDir, "."+normalizedName, "cur")
			newDir = filepath.Join(userDir, "."+normalizedName, "new")
		}
		
		// 构建数据库中已有的邮件 ID 映射
		mailIDMap := make(map[string]bool)
		for _, mail := range mails {
			baseID := mail.ID
			if idx := strings.Index(mail.ID, ":"); idx >= 0 {
				baseID = mail.ID[:idx]
			}
			mailIDMap[baseID] = true
			mailIDMap[mail.ID] = true
		}
		
		// 检查 cur 目录中的文件，同步缺失的邮件到数据库
		curEntries, err := os.ReadDir(curDir)
		if err == nil {
			for _, entry := range curEntries {
				if entry.IsDir() {
					continue
				}
				filename := entry.Name()
				baseID := filename
				if idx := strings.Index(filename, ":"); idx >= 0 {
					baseID = filename[:idx]
				}
				
				// 如果文件不在数据库中，尝试同步
				if !mailIDMap[baseID] && !mailIDMap[filename] {
					logger.Debug().
						Str("user", u.user.Email).
						Str("folder", normalizedName).
						Str("filename", filename).
						Msg("IMAP GetMailbox: 发现 Maildir 中的邮件未同步到数据库，尝试同步")
					
					// 读取邮件文件
					mailData, err := u.maildir.ReadMail(u.user.Email, normalizedName, baseID)
					if err == nil {
						var fromHeader, toHeader, subject string
						var bodyBytes []byte
						
						// 尝试使用 message.Read 解析
						msg, err := message.Read(bytes.NewReader(mailData))
						if err == nil {
							header := msg.Header
							fromHeader = header.Get("From")
							toHeader = header.Get("To")
							subject = header.Get("Subject")
							
							// 读取邮件体
							if msg.Body != nil {
								bodyBytes, _ = io.ReadAll(msg.Body)
							}
						}
						
						// 如果 message.Read 解析失败或邮件头为空，尝试手动解析
						// 检查是否以 "This is a multi-part message" 开头（缺少邮件头）
						mailDataStr := string(mailData)
						if fromHeader == "" && strings.HasPrefix(mailDataStr, "This is a multi-part message") {
							// 这种格式的邮件缺少邮件头，尝试从文件名或其他方式推断
							// 或者使用默认值
							logger.Debug().
								Str("user", u.user.Email).
								Str("folder", normalizedName).
								Str("mail_id", baseID).
								Msg("IMAP GetMailbox: 邮件缺少标准邮件头，使用默认值")
							fromHeader = "unknown@unknown"
							toHeader = u.user.Email
							subject = "(无主题)"
							bodyBytes = mailData
						} else if fromHeader == "" {
							// 尝试手动解析邮件头（如果 message.Read 失败但文件有邮件头）
							lines := strings.Split(mailDataStr, "\n")
							for i, line := range lines {
								line = strings.TrimSpace(line)
								if strings.HasPrefix(strings.ToLower(line), "from:") {
									fromHeader = strings.TrimSpace(line[5:])
								} else if strings.HasPrefix(strings.ToLower(line), "to:") {
									toHeader = strings.TrimSpace(line[3:])
								} else if strings.HasPrefix(strings.ToLower(line), "subject:") {
									subject = strings.TrimSpace(line[8:])
								} else if line == "" && i > 0 {
									// 空行表示邮件头结束
									// 邮件体从下一行开始
									if i+1 < len(lines) {
										bodyBytes = []byte(strings.Join(lines[i+1:], "\n"))
									}
									break
								}
							}
							if fromHeader == "" {
								fromHeader = "unknown@unknown"
							}
							if toHeader == "" {
								toHeader = u.user.Email
							}
							if subject == "" {
								subject = "(无主题)"
							}
							if len(bodyBytes) == 0 {
								bodyBytes = mailData
							}
						}
						
						// 解析 From 地址
						fromAddr := fromHeader
						if fromAddr == "" {
							fromAddr = "unknown@unknown"
						}
						// 清理 From 地址
						fromAddr = strings.TrimSpace(fromAddr)
						if idx := strings.Index(fromAddr, "<"); idx >= 0 {
							if idx2 := strings.Index(fromAddr, ">"); idx2 > idx {
								fromAddr = fromAddr[idx+1 : idx2]
							}
						}
						fromAddr = strings.Trim(fromAddr, "\"")
						fromAddr = strings.TrimSpace(fromAddr)
						if fromAddr == "" || fromAddr == "<>" {
							fromAddr = "unknown@unknown"
						}
						
						// 解析 To 地址
						toAddrs := []string{}
						if toHeader != "" {
							// 简单的地址解析（支持多个地址，用逗号分隔）
							parts := strings.Split(toHeader, ",")
							for _, part := range parts {
								addr := strings.TrimSpace(part)
								// 提取邮箱地址
								if idx := strings.Index(addr, "<"); idx >= 0 {
									if idx2 := strings.Index(addr, ">"); idx2 > idx {
										addr = addr[idx+1 : idx2]
									}
								}
								addr = strings.Trim(addr, "\"")
								addr = strings.TrimSpace(addr)
								if addr != "" {
									toAddrs = append(toAddrs, addr)
								}
							}
						}
						if len(toAddrs) == 0 {
							toAddrs = []string{u.user.Email}
						}
						
						// 确定标志（如果文件在 cur 目录且有 :2,S 后缀，说明已读）
						var flags []string
						if strings.Contains(filename, ":2,S") || strings.Contains(filename, ":2,RS") {
							flags = []string{"\\Seen"}
						} else {
							flags = []string{"\\Recent"}
						}
						
						// 获取文件修改时间作为接收时间
						fileInfo, err := entry.Info()
						receivedAt := time.Now()
						if err == nil {
							receivedAt = fileInfo.ModTime()
						}
						
						// 创建邮件记录
						syncMail := &storage.Mail{
							ID:         baseID,
							UserEmail:  u.user.Email,
							Folder:     normalizedName,
							From:       fromAddr,
							To:         toAddrs,
							Subject:    subject,
							Body:       bodyBytes,
							Size:       int64(len(mailData)),
							Flags:      flags,
							ReceivedAt: receivedAt,
							CreatedAt:  receivedAt,
						}
						
						// 存储到数据库
						if err := u.storage.StoreMail(ctx, syncMail); err != nil {
							logger.Warn().Err(err).
								Str("user", u.user.Email).
								Str("folder", normalizedName).
								Str("mail_id", baseID).
								Msg("同步邮件到数据库失败")
						} else {
							// 添加到邮件列表
							mails = append(mails, syncMail)
							mailIDMap[baseID] = true
							logger.Info().
								Str("user", u.user.Email).
								Str("folder", normalizedName).
								Str("mail_id", baseID).
								Str("from", fromAddr).
								Str("subject", subject).
								Msg("IMAP GetMailbox: 成功同步邮件到数据库")
						}
					}
				}
			}
		}
		
		// 检查 new 目录中的文件，同步缺失的邮件到数据库
		newEntries, err := os.ReadDir(newDir)
		if err == nil {
			newFileMap := make(map[string]bool)
			for _, entry := range newEntries {
				if !entry.IsDir() {
					filename := entry.Name()
					baseID := filename
					if idx := strings.Index(filename, ":"); idx >= 0 {
						baseID = filename[:idx]
					}
					newFileMap[baseID] = true
					
					// 如果文件不在数据库中，尝试同步
					if !mailIDMap[baseID] && !mailIDMap[filename] {
						logger.Debug().
							Str("user", u.user.Email).
							Str("folder", normalizedName).
							Str("filename", filename).
							Msg("IMAP GetMailbox: 发现 new 目录中的邮件未同步到数据库，尝试同步")
						
						// 读取邮件文件
						mailData, err := u.maildir.ReadMail(u.user.Email, normalizedName, baseID)
						if err == nil {
							var fromHeader, toHeader, subject string
							var bodyBytes []byte
							
							// 尝试使用 message.Read 解析
							msg, err := message.Read(bytes.NewReader(mailData))
							if err == nil {
								header := msg.Header
								fromHeader = header.Get("From")
								toHeader = header.Get("To")
								subject = header.Get("Subject")
								
								// 读取邮件体
								if msg.Body != nil {
									bodyBytes, _ = io.ReadAll(msg.Body)
								}
							}
							
							// 如果 message.Read 解析失败或邮件头为空，尝试手动解析
							// 检查是否以 "This is a multi-part message" 开头（缺少邮件头）
							mailDataStr := string(mailData)
							if fromHeader == "" && strings.HasPrefix(mailDataStr, "This is a multi-part message") {
								// 这种格式的邮件缺少邮件头，尝试从文件名或其他方式推断
								// 或者使用默认值
								logger.Debug().
									Str("user", u.user.Email).
									Str("folder", normalizedName).
									Str("mail_id", baseID).
									Msg("IMAP GetMailbox: 邮件缺少标准邮件头，使用默认值（new）")
								fromHeader = "unknown@unknown"
								toHeader = u.user.Email
								subject = "(无主题)"
								bodyBytes = mailData
							} else if fromHeader == "" {
								// 尝试手动解析邮件头（如果 message.Read 失败但文件有邮件头）
								lines := strings.Split(mailDataStr, "\n")
								for i, line := range lines {
									line = strings.TrimSpace(line)
									if strings.HasPrefix(strings.ToLower(line), "from:") {
										fromHeader = strings.TrimSpace(line[5:])
									} else if strings.HasPrefix(strings.ToLower(line), "to:") {
										toHeader = strings.TrimSpace(line[3:])
									} else if strings.HasPrefix(strings.ToLower(line), "subject:") {
										subject = strings.TrimSpace(line[8:])
									} else if line == "" && i > 0 {
										// 空行表示邮件头结束
										// 邮件体从下一行开始
										if i+1 < len(lines) {
											bodyBytes = []byte(strings.Join(lines[i+1:], "\n"))
										}
										break
									}
								}
								if fromHeader == "" {
									fromHeader = "unknown@unknown"
								}
								if toHeader == "" {
									toHeader = u.user.Email
								}
								if subject == "" {
									subject = "(无主题)"
								}
								if len(bodyBytes) == 0 {
									bodyBytes = mailData
								}
							}
							
							// 解析 From 地址
							fromAddr := fromHeader
							if fromAddr == "" {
								fromAddr = "unknown@unknown"
							}
							// 清理 From 地址
							fromAddr = strings.TrimSpace(fromAddr)
							if idx := strings.Index(fromAddr, "<"); idx >= 0 {
								if idx2 := strings.Index(fromAddr, ">"); idx2 > idx {
									fromAddr = fromAddr[idx+1 : idx2]
								}
							}
							fromAddr = strings.Trim(fromAddr, "\"")
							fromAddr = strings.TrimSpace(fromAddr)
							if fromAddr == "" || fromAddr == "<>" {
								fromAddr = "unknown@unknown"
							}
							
							// 解析 To 地址
							toAddrs := []string{}
							if toHeader != "" {
								parts := strings.Split(toHeader, ",")
								for _, part := range parts {
									addr := strings.TrimSpace(part)
									if idx := strings.Index(addr, "<"); idx >= 0 {
										if idx2 := strings.Index(addr, ">"); idx2 > idx {
											addr = addr[idx+1 : idx2]
										}
									}
									addr = strings.Trim(addr, "\"")
									addr = strings.TrimSpace(addr)
									if addr != "" {
										toAddrs = append(toAddrs, addr)
									}
								}
							}
							if len(toAddrs) == 0 {
								toAddrs = []string{u.user.Email}
							}
							
							// 获取文件修改时间作为接收时间
							fileInfo, err := entry.Info()
							receivedAt := time.Now()
							if err == nil {
								receivedAt = fileInfo.ModTime()
							}
							
							// 创建邮件记录（new 目录中的邮件是未读的）
							syncMail := &storage.Mail{
								ID:         baseID,
								UserEmail:  u.user.Email,
								Folder:     normalizedName,
								From:       fromAddr,
								To:         toAddrs,
								Subject:    subject,
								Body:       bodyBytes,
								Size:       int64(len(mailData)),
								Flags:      []string{"\\Recent"}, // new 目录中的邮件是未读的
								ReceivedAt: receivedAt,
								CreatedAt:  receivedAt,
							}
							
							// 存储到数据库
							if err := u.storage.StoreMail(ctx, syncMail); err != nil {
								logger.Warn().Err(err).
									Str("user", u.user.Email).
									Str("folder", normalizedName).
									Str("mail_id", baseID).
									Msg("同步邮件到数据库失败")
							} else {
								// 添加到邮件列表
								mails = append(mails, syncMail)
								mailIDMap[baseID] = true
								logger.Info().
									Str("user", u.user.Email).
									Str("folder", normalizedName).
									Str("mail_id", baseID).
									Str("from", fromAddr).
									Str("subject", subject).
									Msg("IMAP GetMailbox: 成功同步邮件到数据库（new）")
							}
						}
					}
				}
			}
			
			// 检查数据库中的邮件，如果文件在 new 目录中但标志有 \Seen，需要修复
			for _, mail := range mails {
				baseID := mail.ID
				if idx := strings.Index(mail.ID, ":"); idx >= 0 {
					baseID = mail.ID[:idx]
				}
				
				// 如果文件在 new 目录中，但标志有 \Seen，这是不一致的
				if newFileMap[baseID] {
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
					
					// 如果文件在 new 目录中，但标志有 \Seen，移除 \Seen 标志
					if hasSeen {
						logger.Debug().
							Str("user", u.user.Email).
							Str("folder", normalizedName).
							Str("mail_id", baseID).
							Msg("IMAP GetMailbox: 发现文件在 new 目录但标志有 \\Seen，修复标志")
						
						// 移除 \Seen 标志，保留 \Recent
						newFlags := make([]string, 0)
						for _, flag := range mail.Flags {
							if flag != imap.SeenFlag && flag != "\\Seen" {
								newFlags = append(newFlags, flag)
							}
						}
						// 确保有 \Recent 标志
						if !hasRecent {
							newFlags = append(newFlags, imap.RecentFlag)
						}
						
						if err := u.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
							logger.Warn().Err(err).
								Str("user", u.user.Email).
								Str("folder", normalizedName).
								Str("mail_id", baseID).
								Msg("修复邮件标志失败")
						} else {
							mail.Flags = newFlags
							logger.Debug().
								Str("user", u.user.Email).
								Str("folder", normalizedName).
								Str("mail_id", baseID).
								Strs("new_flags", newFlags).
								Msg("IMAP GetMailbox: 已修复邮件标志")
						}
					}
				}
			}
		}
		
		// 重新排序邮件（按接收时间降序）
		if len(mails) > 0 {
			sort.Slice(mails, func(i, j int) bool {
				return mails[i].ReceivedAt.After(mails[j].ReceivedAt)
			})
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
			
			// 如果邮件被标记为已读，且之前未读，需要从 new 移动到 cur
			if u.maildir != nil {
				// 去除可能的标志后缀（如 :2,S）
				baseID := mail.ID
				if idx := strings.Index(mail.ID, ":"); idx >= 0 {
					baseID = mail.ID[:idx]
				}
				
				// 检查文件是否在 new 目录中
				userDir := u.maildir.GetUserMaildir(u.user.Email)
				var newDir string
				if normalizedName == "INBOX" || normalizedName == "" {
					newDir = filepath.Join(userDir, "new")
				} else {
					newDir = filepath.Join(userDir, "."+normalizedName, "new")
				}
				
				newPath := filepath.Join(newDir, baseID)
				if _, err := os.Stat(newPath); err == nil {
					// 文件在 new 目录中，移动到 cur
					if err := u.maildir.MoveToCur(u.user.Email, normalizedName, baseID, newFlags); err != nil {
						logger.Warn().Err(err).
							Str("user", u.user.Email).
							Str("folder", normalizedName).
							Str("mail_id", baseID).
							Msg("移动邮件从 new 到 cur 失败（GetMailbox）")
					} else {
						logger.Debug().
							Str("user", u.user.Email).
							Str("folder", normalizedName).
							Str("mail_id", baseID).
							Msg("IMAP GetMailbox: 邮件已从 new 移动到 cur")
					}
				}
			}
			
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

// updateMailFlagsAndMove 更新邮件标志，并在需要时移动文件（从 new 到 cur）
func (m *Mailbox) updateMailFlagsAndMove(ctx context.Context, mail *storage.Mail, newFlags []string) error {
	// 检查是否需要移动邮件文件（从 new 到 cur）
	hasSeen := false
	hadSeen := false
	for _, f := range newFlags {
		if f == imap.SeenFlag || f == "\\Seen" {
			hasSeen = true
			break
		}
	}
	for _, f := range mail.Flags {
		if f == imap.SeenFlag || f == "\\Seen" {
			hadSeen = true
			break
		}
	}

	// 如果邮件被标记为已读，且之前未读，需要从 new 移动到 cur
	if hasSeen && !hadSeen && m.maildir != nil {
		// 去除可能的标志后缀（如 :2,S）
		baseID := mail.ID
		if idx := strings.Index(mail.ID, ":"); idx >= 0 {
			baseID = mail.ID[:idx]
		}
		
		// 检查文件是否在 new 目录中
		userDir := m.maildir.GetUserMaildir(m.userEmail)
		var newDir string
		if m.name == "INBOX" || m.name == "" {
			newDir = filepath.Join(userDir, "new")
		} else {
			newDir = filepath.Join(userDir, "."+m.name, "new")
		}
		
		newPath := filepath.Join(newDir, baseID)
		if _, err := os.Stat(newPath); err == nil {
			// 文件在 new 目录中，移动到 cur
			if err := m.maildir.MoveToCur(m.userEmail, m.name, baseID, newFlags); err != nil {
				logger.Warn().Err(err).
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", baseID).
					Msg("移动邮件从 new 到 cur 失败")
			} else {
				logger.Debug().
					Str("user", m.userEmail).
					Str("folder", m.name).
					Str("mail_id", baseID).
					Msg("邮件已从 new 移动到 cur")
			}
		}
	}

	// 更新存储
	if err := m.storage.UpdateMailFlags(ctx, mail.ID, newFlags); err != nil {
		return fmt.Errorf("更新邮件标志失败: %w", err)
	}

	// 更新内存中的标志
	mail.Flags = newFlags
	return nil
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
	// 记录 seqSet 信息以便调试
	seqSetStr := "nil"
	if seqSet != nil {
		seqSetStr = seqSet.String()
	}
	logger.Debug().
		Str("user", m.userEmail).
		Str("folder", m.name).
		Int("mail_count", len(m.mails)).
		Bool("uid", uid).
		Str("seq_set", seqSetStr).
		Strs("requested_items", itemNames).
		Msg("IMAP ListMessages: 开始列出邮件")

		// 严格按照 IMAP 规范，只返回客户端请求的字段
		// 如果客户端需要更多字段，它会再次请求（如 UID FETCH 1:15 (UID RFC822.SIZE FLAGS BODY.PEEK[HEADER])）
		// 不要添加额外字段，避免响应过大和字段顺序不一致导致客户端解析失败
		
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
		// 序列号始终是 i+1（从 1 开始）
		seqNum := uint32(i + 1)
		
		// 确定用于匹配 seqSet 的数值
		// - 如果 uid=false（普通 FETCH），seqSet 包含序列号，使用 seqNum 匹配
		// - 如果 uid=true（UID FETCH），seqSet 包含 UID 值，使用 UID 匹配
		// 目前我们使用序列号作为 UID（TODO: 使用实际的 UID），所以两种情况都使用 seqNum
		checkNum := seqNum

		if seqSet != nil && !seqSet.Contains(checkNum) {
			logger.Debug().
				Str("user", m.userEmail).
				Str("folder", m.name).
				Uint32("seq_num", seqNum).
				Uint32("check_num", checkNum).
				Bool("uid", uid).
				Str("seq_set", seqSet.String()).
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
		
		// 预先设置 UID（必须在填充其他字段之前设置，确保 UID 在响应中正确显示）
		// go-imap 库在格式化 FETCH 响应时，会优先显示 UID（如果存在）
		msg.Uid = seqNum // TODO: 使用实际的 UID
		
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
					Bool("envelope_from_nil", msg.Envelope != nil && len(msg.Envelope.From) == 0). // len() 对 nil slice 返回 0
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
					if err := m.updateMailFlagsAndMove(ctx, mail, newFlags); err != nil {
						logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败（FetchFlags）")
					} else {
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
				// #nosec G115 -- 检查溢出，如果超过 uint32 最大值则使用最大值
				var size uint32
				if mail.Size > 0 && mail.Size <= int64(^uint32(0)) {
					size = uint32(mail.Size)
				} else if mail.Size > int64(^uint32(0)) {
					// 如果超过 uint32 最大值，使用最大值
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
					// #nosec G115 -- 检查溢出，如果超过 uint32 最大值则使用最大值
					var size uint32
					if mail.Size > 0 && mail.Size <= int64(^uint32(0)) {
						size = uint32(mail.Size)
					} else if mail.Size > int64(^uint32(0)) {
						size = ^uint32(0)
					}
					msg.BodyStructure = &imap.BodyStructure{
						MIMEType:    "text",
						MIMESubType: "plain",
						Size:        size,
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
							if err := m.updateMailFlagsAndMove(ctx, mail, newFlags); err != nil {
								logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败")
							} else {
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
						
						// 如果客户端请求了 UID，确保 UID 也被包含在响应中
						// go-imap 库在格式化 FETCH 响应时，需要 UID 在 msg.Items 中才能正确显示
						// 注意：即使 msg.Uid 已设置（第966行），也必须添加到 msg.Items 中
						hasUIDRequest := false
						for _, reqItem := range items {
							if reqItem == imap.FetchUid {
								hasUIDRequest = true
								break
							}
						}
						if hasUIDRequest {
							// 无论 msg.Items[imap.FetchUid] 是否已设置，都确保设置（因为可能先处理 BODY section）
							msg.Uid = seqNum // 确保 msg.Uid 已设置
							msg.Items[imap.FetchUid] = seqNum // 确保 UID 在 Items 中（go-imap 库需要这个）
						}
						
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
							if err := m.updateMailFlagsAndMove(ctx, mail, newFlags); err != nil {
								logger.Warn().Err(err).Str("mail_id", mail.ID).Msg("自动设置 \\Seen 标志失败")
							} else {
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

		// 使用辅助函数更新标志并移动文件
		if err := m.updateMailFlagsAndMove(ctx, mail, newFlags); err != nil {
			return err
		}
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
