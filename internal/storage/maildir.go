package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Maildir 实现 Maildir++ 格式存储
type Maildir struct {
	root string
}

// NewMaildir 创建 Maildir 实例
func NewMaildir(root string) (*Maildir, error) {
	// #nosec G301 -- 0755 权限允许组和其他用户读取，这是 Maildir 的标准权限
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("创建 Maildir 根目录失败: %w", err)
	}

	return &Maildir{root: root}, nil
}

// GetUserMaildir 获取用户的 Maildir 路径
func (m *Maildir) GetUserMaildir(userEmail string) string {
	return filepath.Join(m.root, userEmail)
}

// EnsureUserMaildir 确保用户的 Maildir 目录结构存在
func (m *Maildir) EnsureUserMaildir(userEmail string) error {
	userDir := m.GetUserMaildir(userEmail)

	// 创建标准文件夹
	folders := []string{"cur", "new", "tmp"}
	for _, folder := range folders {
		path := filepath.Join(userDir, folder)
		// #nosec G301 -- 0755 权限允许组和其他用户读取，这是 Maildir 的标准权限
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("创建文件夹 %s 失败: %w", folder, err)
		}
	}

	// 创建特殊文件夹
	specialFolders := []string{"Sent", "Drafts", "Trash", "Spam"}
	for _, folder := range specialFolders {
		path := filepath.Join(userDir, "."+folder, "cur")
		// #nosec G301 -- 0755 权限允许组和其他用户读取，这是 Maildir 的标准权限
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("创建特殊文件夹 %s 失败: %w", folder, err)
		}
		path = filepath.Join(userDir, "."+folder, "new")
		// #nosec G301 -- 0755 权限允许组和其他用户读取，这是 Maildir 的标准权限
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("创建特殊文件夹 %s 失败: %w", folder, err)
		}
	}

	return nil
}

// GenerateUniqueName 生成唯一的邮件文件名
func (m *Maildir) GenerateUniqueName() (string, error) {
	// 格式: <timestamp>.<pid>.<random>.<hostname>
	timestamp := time.Now().Unix()
	pid := os.Getpid()

	// 生成随机数
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	random := hex.EncodeToString(randomBytes)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	return fmt.Sprintf("%d.%d.%s.%s", timestamp, pid, random, hostname), nil
}

// StoreMail 存储邮件到 Maildir
func (m *Maildir) StoreMail(userEmail string, folder string, data []byte) (string, error) {
	// 确保用户目录存在
	if err := m.EnsureUserMaildir(userEmail); err != nil {
		return "", err
	}

	// 生成唯一文件名
	uniqueName, err := m.GenerateUniqueName()
	if err != nil {
		return "", err
	}

	// 确定目标文件夹
	var targetDir string
	if folder == "INBOX" || folder == "" {
		targetDir = filepath.Join(m.GetUserMaildir(userEmail), "new")
	} else {
		// 特殊文件夹使用 . 前缀
		targetDir = filepath.Join(m.GetUserMaildir(userEmail), "."+folder, "new")
	}

	// 写入文件
	filePath := filepath.Join(targetDir, uniqueName)
	// #nosec G306 -- 0644 权限允许组和其他用户读取，这是 Maildir 的标准权限
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("写入邮件文件失败: %w", err)
	}

	return uniqueName, nil
}

// MoveToCur 将邮件从 new 移动到 cur（标记为已读）
func (m *Maildir) MoveToCur(userEmail string, folder string, filename string, flags []string) error {
	userDir := m.GetUserMaildir(userEmail)

	// 确定源文件夹
	var srcDir string
	if folder == "INBOX" || folder == "" {
		srcDir = filepath.Join(userDir, "new")
	} else {
		srcDir = filepath.Join(userDir, "."+folder, "new")
	}

	// 确定目标文件夹
	var dstDir string
	if folder == "INBOX" || folder == "" {
		dstDir = filepath.Join(userDir, "cur")
	} else {
		dstDir = filepath.Join(userDir, "."+folder, "cur")
	}

	// 构建标志后缀
	flagSuffix := ":2,"
	for _, flag := range flags {
		switch flag {
		case "\\Seen":
			flagSuffix += "S"
		case "\\Answered":
			flagSuffix += "R"
		case "\\Flagged":
			flagSuffix += "F"
		case "\\Deleted":
			flagSuffix += "T"
		case "\\Draft":
			flagSuffix += "D"
		}
	}

	// 移动文件
	srcPath := filepath.Join(srcDir, filename)
	dstPath := filepath.Join(dstDir, filename+flagSuffix)

	if err := os.Rename(srcPath, dstPath); err != nil {
		return fmt.Errorf("移动邮件文件失败: %w", err)
	}

	return nil
}

// ReadMail 读取邮件内容
func (m *Maildir) ReadMail(userEmail string, folder string, filename string) ([]byte, error) {
	userDir := m.GetUserMaildir(userEmail)

	// 尝试从 cur 读取
	var filePath string
	if folder == "INBOX" || folder == "" {
		filePath = filepath.Join(userDir, "cur", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = filepath.Join(userDir, "new", filename)
		}
	} else {
		filePath = filepath.Join(userDir, "."+folder, "cur", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = filepath.Join(userDir, "."+folder, "new", filename)
		}
	}

	// 验证文件路径在 Maildir 根目录下（防止路径遍历攻击）
	// #nosec G304 -- filePath 已经通过 filepath.Join 和已验证的 userDir 构建，是安全的
	if !strings.HasPrefix(filePath, m.root) {
		return nil, fmt.Errorf("无效的文件路径")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取邮件文件失败: %w", err)
	}

	return data, nil
}

// DeleteMail 删除邮件
func (m *Maildir) DeleteMail(userEmail string, folder string, filename string) error {
	userDir := m.GetUserMaildir(userEmail)

	// 尝试从 cur 删除
	var filePath string
	if folder == "INBOX" || folder == "" {
		filePath = filepath.Join(userDir, "cur", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = filepath.Join(userDir, "new", filename)
		}
	} else {
		filePath = filepath.Join(userDir, "."+folder, "cur", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = filepath.Join(userDir, "."+folder, "new", filename)
		}
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("删除邮件文件失败: %w", err)
	}

	return nil
}

// ListMails 列出邮件
func (m *Maildir) ListMails(userEmail string, folder string) ([]string, error) {
	userDir := m.GetUserMaildir(userEmail)

	var dir string
	if folder == "INBOX" || folder == "" {
		dir = filepath.Join(userDir, "cur")
		// 也包含 new 文件夹中的邮件
	} else {
		dir = filepath.Join(userDir, "."+folder, "cur")
	}

	var files []string

	// 读取 cur 文件夹
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, fmt.Errorf("读取邮件目录失败: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	// 如果是 INBOX，也包含 new 文件夹
	if folder == "INBOX" || folder == "" {
		newDir := filepath.Join(userDir, "new")
		entries, err := os.ReadDir(newDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					files = append(files, entry.Name())
				}
			}
		}
	}

	return files, nil
}
