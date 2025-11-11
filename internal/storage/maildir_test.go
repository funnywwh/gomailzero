package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaildir(t *testing.T) {
	// 创建临时目录
	tmpdir, err := os.MkdirTemp("", "maildir-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	maildir, err := NewMaildir(tmpdir)
	if err != nil {
		t.Fatalf("创建 Maildir 失败: %v", err)
	}

	t.Run("EnsureUserMaildir", func(t *testing.T) {
		err := maildir.EnsureUserMaildir("test@example.com")
		if err != nil {
			t.Fatalf("创建用户目录失败: %v", err)
		}

		// 检查目录是否存在
		userDir := maildir.GetUserMaildir("test@example.com")
		folders := []string{"cur", "new", "tmp"}
		for _, folder := range folders {
			path := filepath.Join(userDir, folder)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("文件夹 %s 不存在", folder)
			}
		}
	})

	t.Run("StoreMail", func(t *testing.T) {
		data := []byte("From: test@example.com\nTo: user@example.com\nSubject: Test\n\nBody")
		filename, err := maildir.StoreMail("test@example.com", "INBOX", data)
		if err != nil {
			t.Fatalf("存储邮件失败: %v", err)
		}

		if filename == "" {
			t.Error("文件名应该不为空")
		}

		// 验证文件存在
		filePath := filepath.Join(maildir.GetUserMaildir("test@example.com"), "new", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("邮件文件不存在: %s", filePath)
		}
	})

	t.Run("ReadMail", func(t *testing.T) {
		data := []byte("From: test@example.com\nTo: user@example.com\nSubject: Test\n\nBody")
		filename, err := maildir.StoreMail("test@example.com", "INBOX", data)
		if err != nil {
			t.Fatalf("存储邮件失败: %v", err)
		}

		readData, err := maildir.ReadMail("test@example.com", "INBOX", filename)
		if err != nil {
			t.Fatalf("读取邮件失败: %v", err)
		}

		if string(readData) != string(data) {
			t.Errorf("邮件内容不匹配")
		}
	})

	t.Run("MoveToCur", func(t *testing.T) {
		data := []byte("From: test@example.com\nTo: user@example.com\nSubject: Test\n\nBody")
		filename, err := maildir.StoreMail("test@example.com", "INBOX", data)
		if err != nil {
			t.Fatalf("存储邮件失败: %v", err)
		}

		err = maildir.MoveToCur("test@example.com", "INBOX", filename, []string{"\\Seen"})
		if err != nil {
			t.Fatalf("移动邮件失败: %v", err)
		}

		// 验证文件已移动到 cur
		curPath := filepath.Join(maildir.GetUserMaildir("test@example.com"), "cur", filename+":2,S")
		if _, err := os.Stat(curPath); os.IsNotExist(err) {
			t.Errorf("邮件文件未移动到 cur: %s", curPath)
		}
	})

	t.Run("ListMails", func(t *testing.T) {
		// 存储几封邮件
		for i := 0; i < 3; i++ {
			data := []byte("From: test@example.com\nTo: user@example.com\nSubject: Test\n\nBody")
			_, err := maildir.StoreMail("test@example.com", "INBOX", data)
			if err != nil {
				t.Fatalf("存储邮件失败: %v", err)
			}
		}

		files, err := maildir.ListMails("test@example.com", "INBOX")
		if err != nil {
			t.Fatalf("列出邮件失败: %v", err)
		}

		if len(files) < 3 {
			t.Errorf("邮件数量不匹配: got %d, want at least 3", len(files))
		}
	})

	t.Run("DeleteMail", func(t *testing.T) {
		data := []byte("From: test@example.com\nTo: user@example.com\nSubject: Test\n\nBody")
		filename, err := maildir.StoreMail("test@example.com", "INBOX", data)
		if err != nil {
			t.Fatalf("存储邮件失败: %v", err)
		}

		err = maildir.DeleteMail("test@example.com", "INBOX", filename)
		if err != nil {
			t.Fatalf("删除邮件失败: %v", err)
		}

		// 验证文件已删除
		filePath := filepath.Join(maildir.GetUserMaildir("test@example.com"), "new", filename)
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("邮件文件未被删除: %s", filePath)
		}
	})
}

