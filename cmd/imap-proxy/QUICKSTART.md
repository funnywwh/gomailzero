# IMAP 透传代理 - 快速开始

## 问题场景

Foxmail 无法正常收邮件，需要查看 Foxmail 和 IMAP 服务器之间的交互流程来诊断问题。

### 常见错误

- **SSL ShakeHand error:00002746** - TLS 握手失败，通常是因为客户端和代理之间的 TLS 配置不匹配

## 快速使用

### 步骤 1：启动透传代理

根据 Foxmail 的配置选择不同的启动方式：

#### 方式 A：Foxmail 使用 SSL/TLS（推荐，解决 TLS 握手错误）

如果 Foxmail 配置为使用 SSL/TLS，需要使用 `-client-tls` 选项：

```bash
# 使用自签名证书（自动生成）
./bin/imap-proxy -client-tls

# 或使用自定义证书
./bin/imap-proxy -client-tls -client-cert cert.pem -client-key key.pem
```

**注意**：使用自签名证书时，Foxmail 会提示证书不受信任，需要选择"接受"或"继续"。

#### 方式 B：Foxmail 使用无加密或 STARTTLS

如果 Foxmail 配置为无加密或 STARTTLS：

```bash
# 普通 TCP 模式
./bin/imap-proxy

# 或使用启动脚本
cd cmd/imap-proxy
./run.sh
```

#### 方式 C：详细模式（推荐用于调试）

```bash
# TLS-in-TLS 模式 + 详细日志（自动保存到 logs/ 目录）
./bin/imap-proxy -client-tls -v

# 或指定自定义日志文件
./bin/imap-proxy -client-tls -v -log foxmail-debug.log
```

**注意**：默认情况下，日志会自动保存到 `logs/imap-proxy-YYYYMMDD-HHMMSS.log` 文件，同时也会输出到控制台，方便实时查看。

### 步骤 2：配置 Foxmail

1. 打开 Foxmail
2. 进入账户设置
3. 修改 IMAP 服务器配置：
   - **服务器地址**: `localhost`（或代理服务器 IP）
   - **端口**: `1993`（代理监听端口）
   - **加密方式**: 
     - **如果使用 `-client-tls`**：选择 **SSL/TLS**（会提示证书警告，选择接受）
     - **如果不使用 `-client-tls`**：选择 **无加密** 或 **STARTTLS**

### 步骤 3：测试连接

在 Foxmail 中尝试：
- 收邮件
- 查看邮件列表
- 同步邮箱

### 步骤 4：查看日志

日志会自动保存到文件，同时也会输出到控制台。

```bash
# 查看最新的日志文件（自动生成在 logs/ 目录）
ls -lt logs/imap-proxy-*.log | head -1

# 实时查看日志
tail -f logs/imap-proxy-*.log

# 或查看指定日志文件
tail -f logs/imap-proxy-20250111-103045.log
```

**日志文件命名规则**：
- 默认目录：`logs/`
- 文件名格式：`imap-proxy-YYYYMMDD-HHMMSS.log`
- 例如：`logs/imap-proxy-20250111-103045.log`

**自定义日志设置**：
```bash
# 指定日志目录
./bin/imap-proxy -log-dir /var/log/imap-proxy

# 指定日志文件
./bin/imap-proxy -log /path/to/custom.log

# 禁用自动保存（只输出到控制台）
./bin/imap-proxy -auto-log=false
```

## 日志分析

### 查看连接信息

```bash
grep "新客户端连接" imap-proxy.log
```

### 查看客户端命令

```bash
grep "C->S" imap-proxy.log
```

### 查看服务器响应

```bash
grep "S->C" imap-proxy.log
```

### 查看错误信息

```bash
grep -i "error\|fail\|拒绝" imap-proxy.log
```

## 常见问题诊断

### 问题 1：Foxmail 提示"连接失败"

**检查项**：
1. 代理是否正在运行
2. Foxmail 配置的端口是否正确（应该是 1993）
3. 查看日志中的错误信息

**日志示例**：
```
[20250111-103045.123] 连接目标服务器失败: dial tcp: connect: connection refused
```

**解决方案**：检查目标 IMAP 服务器是否运行在 993 端口

### 问题 2：Foxmail 提示"认证失败"

**检查项**：
1. 查看日志中的 LOGIN 命令
2. 检查用户名和密码是否正确

**日志示例**：
```
[20250111-103045.123] C->S [1] A001 LOGIN user@example.com ***
[20250111-103045.123] S->C [2] A001 NO LOGIN failed
```

**解决方案**：检查用户名和密码是否正确

### 问题 3：Foxmail 提示"TLS 错误"或 "SSL ShakeHand error"

**症状**：
- `SSL ShakeHand error:00002746:lib(0):func(2):reason(1862)`
- TLS 握手失败

**原因**：
- Foxmail 配置为使用 SSL/TLS，但代理没有启用客户端 TLS 支持

**解决方案**：

1. **使用 `-client-tls` 选项启动代理**：
   ```bash
   ./bin/imap-proxy -client-tls
   ```

2. **在 Foxmail 中接受证书警告**：
   - 代理使用自签名证书时，Foxmail 会提示证书不受信任
   - 选择"接受"或"继续"即可

3. **如果不想看到证书警告，使用服务器证书**：
   ```bash
   # 使用服务器的证书文件
   ./bin/imap-proxy -client-tls \
     -client-cert /path/to/server/cert.pem \
     -client-key /path/to/server/key.pem
   ```

4. **检查 Foxmail 的加密设置**：
   - 必须选择 **SSL/TLS**（不是 STARTTLS）
   - 端口必须是 `1993`（代理监听端口）

### 问题 4：连接目标服务器失败 - 证书验证错误

**症状**：
```
连接目标服务器失败: tls: failed to verify certificate: x509: certificate signed by unknown authority
```

**原因**：
- 本地 GoMailZero 服务器使用自签名证书
- 代理默认会验证服务器证书，导致验证失败

**解决方案**：

使用 `-insecure` 选项跳过证书验证（**仅用于调试**）：

```bash
# 连接到本地服务器时，跳过证书验证
./bin/imap-proxy -target localhost:993 -insecure -v

# 如果同时需要客户端 TLS 支持
./bin/imap-proxy -target localhost:993 -client-tls -insecure -v
```

**注意**：
- `-insecure` 选项会跳过 TLS 证书验证，**仅用于本地调试**
- 生产环境或连接到远程服务器时，不要使用此选项
- 建议使用有效的证书或配置证书信任

### 问题 4：收不到邮件

**检查项**：
1. 查看 FETCH 命令和响应
2. 检查 SELECT 命令是否成功
3. 查看 SEARCH 命令的结果

**日志示例**：
```
[20250111-103045.123] C->S [5] A005 SELECT INBOX
[20250111-103045.123] S->C [6] A005 OK [READ-WRITE] SELECT completed
[20250111-103045.123] C->S [7] A006 SEARCH ALL
[20250111-103045.123] S->C [8] * SEARCH 1 2 3
[20250111-103045.123] S->C [9] A006 OK SEARCH completed
```

**分析**：
- 如果 SEARCH 返回空，说明邮箱中没有邮件
- 如果 FETCH 失败，查看具体的错误信息

## 高级用法

### 同时记录多个连接

代理支持并发连接，每个连接都有唯一的时间戳标识：

```
[20250111-103045.123] 新客户端连接: 127.0.0.1:54321
[20250111-103045.456] 新客户端连接: 127.0.0.1:54322
```

### 详细模式

使用 `-v` 选项可以查看解析后的命令：

```bash
./bin/imap-proxy -v
```

输出示例：
```
[20250111-103045.123] >>> 命令: LOGIN user@example.com ***
[20250111-103045.123] <<< 响应: A001 OK LOGIN completed
```

### 转发到远程服务器

```bash
./bin/imap-proxy -target imap.example.com:993
```

### 调试自签名证书

```bash
./bin/imap-proxy -insecure
```

## 日志文件位置

- 默认：`imap-proxy.log`（当前目录）
- 自定义：使用 `-log` 参数指定

## 停止代理

按 `Ctrl+C` 停止代理，所有连接会正常关闭。

## 下一步

根据日志分析结果：
1. 如果发现协议问题，检查 IMAP 服务器实现
2. 如果发现认证问题，检查用户配置
3. 如果发现连接问题，检查网络和防火墙设置

将日志文件保存，便于后续分析和问题追踪。

