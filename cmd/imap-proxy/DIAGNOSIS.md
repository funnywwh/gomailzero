# Foxmail 收邮件问题诊断指南

## 当前日志分析

从日志文件 `imap-proxy-20251113-173141.log` 来看：
- ✅ 代理成功启动
- ❌ **没有客户端连接记录** - 这说明 Foxmail 可能没有连接到代理

## 问题可能的原因

### 1. Foxmail 没有连接到代理（最可能）

**症状**：日志中只有启动信息，没有客户端连接

**原因**：
- Foxmail 可能直接连接到 IMAP 服务器（993端口），而不是代理（1993端口）
- Foxmail 配置的服务器地址或端口不正确

**解决方案**：
1. 检查 Foxmail 的 IMAP 服务器配置
2. 确保服务器地址是 `localhost`（或代理服务器 IP）
3. 确保端口是 `1993`（代理监听端口）

### 2. TLS 握手失败

**症状**：Foxmail 提示 "SSL ShakeHand error:00002746"

**原因**：
- Foxmail 配置为使用 SSL/TLS，但代理没有启用客户端 TLS 支持

**解决方案**：
```bash
# 使用 -client-tls 选项启动代理
./bin/imap-proxy -client-tls -v
```

### 3. 连接被拒绝

**症状**：Foxmail 提示"连接失败"或"无法连接到服务器"

**原因**：
- 代理没有运行
- 防火墙阻止连接
- 端口被占用

**解决方案**：
```bash
# 检查代理是否运行
ps aux | grep imap-proxy

# 检查端口是否监听
netstat -tlnp | grep 1993

# 检查防火墙
sudo ufw status
```

## 正确的诊断流程

### 步骤 1：启动透传代理（带详细日志）

```bash
# 方式 1：如果 Foxmail 使用 SSL/TLS
./bin/imap-proxy -client-tls -v

# 方式 2：如果 Foxmail 使用无加密或 STARTTLS
./bin/imap-proxy -v
```

**重要**：使用 `-v` 选项启用详细模式，可以看到解析后的命令。

### 步骤 2：配置 Foxmail

1. 打开 Foxmail
2. 进入账户设置 → IMAP 设置
3. 修改配置：
   - **服务器地址**: `localhost`（或代理服务器 IP）
   - **端口**: `1993`
   - **加密方式**: 
     - 如果使用 `-client-tls`：选择 **SSL/TLS**
     - 如果不使用 `-client-tls`：选择 **无加密** 或 **STARTTLS**

### 步骤 3：在 Foxmail 中测试

1. 点击"收信"或"同步"
2. 观察 Foxmail 的提示信息
3. 查看代理的控制台输出

### 步骤 4：分析日志

查看日志文件中的关键信息：

```bash
# 查看最新的日志文件
ls -lt logs/imap-proxy-*.log | head -1

# 实时查看日志
tail -f logs/imap-proxy-*.log

# 查看客户端连接
grep "新客户端连接" logs/imap-proxy-*.log

# 查看登录命令
grep "LOGIN" logs/imap-proxy-*.log

# 查看错误信息
grep -i "error\|fail\|拒绝" logs/imap-proxy-*.log
```

## 常见问题分析

### 问题 1：日志中没有客户端连接

**检查清单**：
- [ ] Foxmail 配置的服务器地址是 `localhost` 吗？
- [ ] Foxmail 配置的端口是 `1993` 吗？
- [ ] 代理是否正在运行？
- [ ] 是否有防火墙阻止连接？

**验证方法**：
```bash
# 检查代理是否监听
netstat -tlnp | grep 1993

# 测试连接
telnet localhost 1993
# 或
nc -zv localhost 1993
```

### 问题 2：TLS 握手失败

**检查清单**：
- [ ] 是否使用了 `-client-tls` 选项？
- [ ] Foxmail 的加密设置是否匹配？
- [ ] 是否接受了证书警告？

**验证方法**：
```bash
# 使用 -client-tls 启动
./bin/imap-proxy -client-tls -v

# 在 Foxmail 中：
# 1. 选择 SSL/TLS
# 2. 接受证书警告
```

### 问题 3：认证失败

**日志特征**：
```
C->S [1] A001 LOGIN user@example.com ***
S->C [2] A001 NO LOGIN failed
```

**检查清单**：
- [ ] 用户名是否正确？
- [ ] 密码是否正确？
- [ ] 用户是否存在于数据库中？

**验证方法**：
```bash
# 检查用户是否存在
sqlite3 data/data.db "SELECT email FROM users WHERE email='user@example.com';"
```

### 问题 4：收不到邮件

**日志特征**：
```
C->S [5] A005 SELECT INBOX
S->C [6] A005 OK [READ-WRITE] SELECT completed
C->S [7] A006 SEARCH ALL
S->C [8] * SEARCH
S->C [9] A006 OK SEARCH completed
```

如果 SEARCH 返回空（没有数字），说明邮箱中没有邮件。

**检查清单**：
- [ ] 邮箱中是否有邮件？
- [ ] SELECT 命令是否成功？
- [ ] FETCH 命令是否成功？

**验证方法**：
```bash
# 检查邮件数量
sqlite3 data/data.db "SELECT COUNT(*) FROM mails WHERE user_email='user@example.com' AND folder='INBOX';"
```

## 完整的诊断命令

```bash
# 1. 启动代理（TLS-in-TLS 模式，详细日志）
./bin/imap-proxy -client-tls -v -log-dir logs

# 2. 在另一个终端查看日志
tail -f logs/imap-proxy-*.log

# 3. 在 Foxmail 中测试连接

# 4. 分析日志
grep -E "新客户端连接|LOGIN|SELECT|FETCH|SEARCH|ERROR|FAIL" logs/imap-proxy-*.log
```

## 预期的正常日志示例

```
2025/11/13 17:31:41 IMAP 透传代理启动（客户端 TLS 模式）
2025/11/13 17:31:41 监听地址: :1993
2025/11/13 17:31:41 目标服务器: localhost:993 (TLS: true)
2025/11/13 17:31:41 客户端连接: TLS (需要客户端配置 SSL/TLS)
2025/11/13 17:31:41 日志文件: /path/to/logs/imap-proxy-20251113-173141.log
2025/11/13 17:31:41 等待客户端连接...
2025/11/13 17:31:41 ================================================================================
2025/11/13 17:32:15 [20251113-173215.123] 新客户端连接: 127.0.0.1:54321
2025/11/13 17:32:15 [20251113-173215.123] 已连接到目标服务器
2025/11/13 17:32:15 [20251113-173215.123] 开始双向转发数据...
2025/11/13 17:32:15 [20251113-173215.123] --------------------------------------------------------------------------------
2025/11/13 17:32:15 [20251113-173215.123] S->C [1] * OK [CAPABILITY IMAP4rev1 AUTH=PLAIN] GoMailZero IMAP server ready
2025/11/13 17:32:16 [20251113-173215.123] C->S [1] A001 LOGIN user@example.com ***
2025/11/13 17:32:16 [20251113-173215.123] S->C [2] A001 OK LOGIN completed
2025/11/13 17:32:16 [20251113-173215.123] C->S [2] A002 SELECT INBOX
2025/11/13 17:32:16 [20251113-173215.123] S->C [3] * 15 EXISTS
2025/11/13 17:32:16 [20251113-173215.123] S->C [4] * 0 RECENT
2025/11/13 17:32:16 [20251113-173215.123] S->C [5] A002 OK [READ-WRITE] SELECT completed
2025/11/13 17:32:17 [20251113-173215.123] C->S [3] A003 SEARCH ALL
2025/11/13 17:32:17 [20251113-173215.123] S->C [6] * SEARCH 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
2025/11/13 17:32:17 [20251113-173215.123] S->C [7] A003 OK SEARCH completed
...
```

## 下一步行动

1. **重新启动代理并捕获日志**：
   ```bash
   ./bin/imap-proxy -client-tls -v
   ```

2. **在 Foxmail 中测试连接**，确保：
   - 服务器地址：`localhost`
   - 端口：`1993`
   - 加密方式：`SSL/TLS`（如果使用 `-client-tls`）

3. **查看新的日志文件**，分析实际的交互数据

4. **如果问题仍然存在**，将完整的日志文件发送给我进行分析

