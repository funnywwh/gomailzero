# GoMailZero 测试账号指南

## 可用的测试账号

从数据库中查询到的账号：

### 1. admin@example.com
- **状态**: ✅ 已激活
- **邮件数量**:
  - INBOX: 2 封
  - Sent: 6 封
- **用途**: 管理员账号，适合测试管理功能

### 2. funnywwh@example.com（推荐）
- **状态**: ✅ 已激活
- **邮件数量**:
  - INBOX: 15 封
  - Sent: 7 封
  - Drafts: 2 封
- **用途**: 普通用户账号，**推荐用于测试**（邮件较多，便于测试）

## 密码问题

⚠️ **注意**: 密码是哈希存储的，无法直接查看。你需要：

### 方案 1：使用已知密码

如果你知道这些账号的密码，直接使用即可。

### 方案 2：重置密码（通过 API）

如果不知道密码，可以通过管理 API 重置：

```bash
# 1. 获取 API Key（从环境变量或配置文件）
export GMZ_API_KEY="your-api-key"

# 2. 重置密码（需要 API Key）
curl -X PUT http://localhost:8081/api/v1/users/funnywwh@example.com/password \
  -H "X-API-Key: $GMZ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"password": "newpassword123"}'
```

### 方案 3：通过 WebMail 重置

1. 访问 WebMail: http://localhost:8080/webmail
2. 使用"忘记密码"功能（如果已实现）
3. 或通过管理员界面重置

### 方案 4：直接修改数据库（不推荐，仅用于测试）

```bash
# 生成新密码的哈希值（需要 Go 环境）
cd /home/winger/gowk/gomailzero
go run -c 'package main; import ("fmt"; "github.com/gomailzero/gmz/internal/crypto"); func main() { hash, _ := crypto.HashPassword("test123456"); fmt.Println(hash) }'

# 更新数据库（替换 YOUR_HASH 为上面生成的哈希值）
sqlite3 data/data.db "UPDATE users SET password_hash = 'YOUR_HASH' WHERE email = 'funnywwh@example.com';"
```

## 推荐的测试流程

### 步骤 1：确认密码

如果你不知道密码，先重置：

```bash
# 使用管理 API 重置密码
curl -X PUT http://localhost:8081/api/v1/users/funnywwh@example.com/password \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"password": "test123456"}'
```

### 步骤 2：启动透传代理

```bash
# 连接到本地 GoMailZero 服务器（使用 -insecure 跳过证书验证）
./bin/imap-proxy -target localhost:993 -insecure -v -log foxmail-gmz-debug.log

# 如果 Foxmail 使用 SSL/TLS，还需要添加 -client-tls
./bin/imap-proxy -target localhost:993 -client-tls -insecure -v -log foxmail-gmz-debug.log
```

**重要**：本地服务器通常使用自签名证书，必须使用 `-insecure` 选项跳过证书验证。

### 步骤 3：在 Foxmail 中配置

1. **服务器地址**: `localhost`
2. **端口**: `1993`（代理监听端口）
3. **加密方式**: 
   - 如果使用 `-client-tls`：选择 **SSL/TLS**
   - 如果不使用 `-client-tls`：选择 **无加密** 或 **STARTTLS**
4. **用户名**: `funnywwh@example.com`
5. **密码**: 你设置的密码（如 `test123456`）

### 步骤 4：测试连接

1. 在 Foxmail 中点击"收信"
2. 观察代理日志输出
3. 检查是否有错误

## 测试账号对比

| 账号 | INBOX 邮件 | Sent 邮件 | 推荐度 |
|------|-----------|-----------|--------|
| admin@example.com | 2 | 6 | ⭐⭐ |
| funnywwh@example.com | 15 | 7 | ⭐⭐⭐⭐⭐ |

**推荐使用 `funnywwh@example.com`**，因为：
- ✅ 邮件数量多（15封），便于测试各种场景
- ✅ 有草稿邮件，可以测试 Drafts 文件夹
- ✅ 有已发送邮件，可以测试 Sent 文件夹

## 快速测试命令

```bash
# 1. 重置密码（如果需要）
curl -X PUT http://localhost:8081/api/v1/users/funnywwh@example.com/password \
  -H "X-API-Key: $GMZ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"password": "test123456"}'

# 2. 启动代理
./bin/imap-proxy -target localhost:993 -v -log foxmail-gmz-debug.log

# 3. 在 Foxmail 中配置并测试
# 服务器: localhost:1993
# 用户名: funnywwh@example.com
# 密码: test123456
```

## 验证账号状态

```bash
# 检查账号是否存在且激活
sqlite3 data/data.db "SELECT email, active, is_admin FROM users WHERE email = 'funnywwh@example.com';"

# 检查邮件数量
sqlite3 data/data.db "SELECT folder, COUNT(*) FROM mails WHERE user_email = 'funnywwh@example.com' GROUP BY folder;"
```

## 注意事项

1. **密码安全**: 测试环境可以使用简单密码，生产环境必须使用强密码
2. **TLS 证书**: 如果使用 `-client-tls`，Foxmail 会提示证书警告，选择"接受"即可
3. **日志文件**: 所有交互都会记录到日志文件，便于分析问题

