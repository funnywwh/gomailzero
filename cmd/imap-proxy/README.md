# IMAP 透传代理工具

这是一个用于调试和分析 IMAP 客户端（如 Foxmail）与服务器交互的透传代理工具。

## 功能特性

- ✅ 双向数据转发：客户端 ↔ 服务器
- ✅ 完整的交互日志记录
- ✅ 自动隐藏敏感信息（密码）
- ✅ 支持 TLS 连接
- ✅ 详细模式（解析并显示 IMAP 命令）
- ✅ 多连接并发支持

## 使用方法

### 1. 编译程序

```bash
cd cmd/imap-proxy
go build -o imap-proxy main.go
```

或者从项目根目录：

```bash
go build -o bin/imap-proxy ./cmd/imap-proxy
```

### 2. 启动透传代理

```bash
# 默认配置（监听 1993，转发到 localhost:993）
./imap-proxy

# 自定义配置
./imap-proxy -listen :1993 -target localhost:993 -tls -log imap-proxy.log -v
```

### 3. 配置 Foxmail

在 Foxmail 中配置 IMAP 服务器：

- **服务器地址**: `localhost`（或你的代理服务器地址）
- **端口**: `1993`（代理监听端口）
- **加密方式**: 根据代理配置选择
  - 如果代理使用 TLS 连接目标服务器，Foxmail 可以选择"SSL/TLS"或"STARTTLS"
  - 如果代理不使用 TLS，Foxmail 选择"无加密"

### 4. 查看日志

所有交互数据都会记录到日志文件中（默认：`imap-proxy.log`），格式如下：

```
2025-01-11 10:30:45 [20250111-103045.123] 新客户端连接: 127.0.0.1:54321
2025-01-11 10:30:45 [20250111-103045.123] 连接到目标服务器: localhost:993
2025-01-11 10:30:45 [20250111-103045.123] 已连接到目标服务器
2025-01-11 10:30:45 [20250111-103045.123] 开始双向转发数据...
2025-01-11 10:30:45 [20250111-103045.123] --------------------------------------------------------------------------------
2025-01-11 10:30:45 [20250111-103045.123] S->C [1] * OK [CAPABILITY IMAP4rev1 AUTH=PLAIN] GoMailZero IMAP server ready
2025-01-11 10:30:46 [20250111-103045.123] C->S [1] A001 LOGIN user@example.com ***
2025-01-11 10:30:46 [20250111-103045.123] S->C [2] A001 OK LOGIN completed
...
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-listen` | `:1993` | 监听地址（客户端连接地址） |
| `-target` | `localhost:993` | 目标 IMAP 服务器地址 |
| `-tls` | `true` | 是否使用 TLS 连接目标服务器 |
| `-client-tls` | `false` | 是否接受客户端的 TLS 连接（TLS-in-TLS 模式） |
| `-client-cert` | `""` | 客户端 TLS 证书文件（用于 -client-tls） |
| `-client-key` | `""` | 客户端 TLS 密钥文件（用于 -client-tls） |
| `-insecure` | `false` | 跳过 TLS 证书验证（仅用于调试） |
| `-log` | `""` | 日志文件路径（留空自动生成：logs/imap-proxy-YYYYMMDD-HHMMSS.log） |
| `-log-dir` | `logs` | 日志目录（自动创建） |
| `-auto-log` | `true` | 自动保存日志到文件（默认启用） |
| `-v` | `false` | 详细输出模式（解析并显示 IMAP 命令） |

## 使用示例

### 示例 1：基本使用

```bash
# 启动代理，监听 1993 端口，转发到本地 993 端口
./imap-proxy -listen :1993 -target localhost:993
```

### 示例 2：自动保存日志（默认）

```bash
# 自动保存到 logs/imap-proxy-YYYYMMDD-HHMMSS.log
./imap-proxy

# 指定日志目录
./imap-proxy -log-dir /var/log/imap-proxy
```

### 示例 3：禁用自动保存

```bash
# 只输出到控制台，不保存到文件
./imap-proxy -auto-log=false
```

### 示例 4：详细模式

```bash
# 启用详细模式，会解析并显示 IMAP 命令（自动保存日志）
./imap-proxy -v

# 详细模式 + 自定义日志文件
./imap-proxy -v -log debug.log
```

### 示例 5：TLS-in-TLS 模式（解决 TLS 握手错误）

```bash
# 使用自签名证书（自动生成）
./imap-proxy -client-tls

# 使用自定义证书
./imap-proxy -client-tls -client-cert cert.pem -client-key key.pem
```

### 示例 6：调试自签名证书

```bash
# 跳过 TLS 证书验证（仅用于调试）
./imap-proxy -insecure
```

### 示例 7：转发到远程服务器

```bash
# 转发到远程 IMAP 服务器
./imap-proxy -target imap.example.com:993
```

## 日志格式说明

### 连接信息

- `[时间戳]` - 连接唯一标识符
- `新客户端连接` - 客户端地址
- `连接到目标服务器` - 目标服务器地址
- `已连接到目标服务器` - 连接成功

### 数据交互

- `C->S [行号]` - 客户端发送到服务器的数据
- `S->C [行号]` - 服务器发送到客户端的数据

### 详细模式（-v）

启用详细模式后，还会显示：

- `>>> 命令: COMMAND args` - 解析的客户端命令
- `<<< 响应: STATUS message` - 解析的服务器响应

## 安全注意事项

1. **密码保护**：日志中会自动隐藏密码，但建议在生产环境中谨慎使用
2. **日志文件**：确保日志文件权限设置正确，避免敏感信息泄露
3. **TLS 证书**：生产环境不要使用 `-insecure` 选项

## 故障排查

### 问题 1：无法连接到目标服务器

**症状**：日志显示"连接目标服务器失败"

**解决方案**：
- 检查目标服务器地址和端口是否正确
- 检查目标服务器是否运行
- 检查防火墙设置

### 问题 2：TLS 握手失败

**症状**：日志显示 TLS 相关错误

**解决方案**：
- 检查目标服务器是否支持 TLS
- 如果使用自签名证书，使用 `-insecure` 选项（仅用于调试）
- 检查证书是否有效

### 问题 3：Foxmail 无法连接

**症状**：Foxmail 提示连接失败

**解决方案**：
- 检查代理是否正在运行
- 检查 Foxmail 配置的服务器地址和端口是否正确
- 检查 Foxmail 的加密设置是否与代理配置匹配

### 问题 4：看不到完整交互

**症状**：日志不完整或缺少数据

**解决方案**：
- 使用 `-v` 选项启用详细模式
- 检查日志文件是否有写入权限
- 确保代理程序有足够的权限

## 常见 IMAP 命令

透传代理会记录所有 IMAP 命令，常见命令包括：

- `LOGIN` - 用户登录
- `SELECT` - 选择邮箱
- `FETCH` - 获取邮件
- `SEARCH` - 搜索邮件
- `STORE` - 修改邮件标志
- `LIST` - 列出邮箱
- `STATUS` - 获取邮箱状态
- `IDLE` - 空闲等待新邮件

## 日志文件位置

### 自动保存（默认）

- **默认目录**：`logs/`（自动创建）
- **文件名格式**：`imap-proxy-YYYYMMDD-HHMMSS.log`
- **示例**：`logs/imap-proxy-20250111-103045.log`
- **特点**：
  - 每次启动自动生成新的日志文件（带时间戳）
  - 日志同时输出到文件和控制台
  - 自动创建日志目录

### 自定义日志

- 使用 `-log` 参数指定完整路径
- 使用 `-log-dir` 参数指定日志目录
- 使用 `-auto-log=false` 禁用自动保存

### 查看日志

```bash
# 查看最新的日志文件
ls -lt logs/imap-proxy-*.log | head -1

# 实时查看日志
tail -f logs/imap-proxy-*.log

# 查看指定时间的日志
cat logs/imap-proxy-20250111-103045.log
```

## 技术实现

- 使用 Go 标准库实现 TCP 代理
- 支持 TLS 透传（TLS-in-TLS）
- 使用 `bufio.Reader` 按行读取 IMAP 协议数据
- 自动处理 CRLF 行结束符
- 并发处理多个客户端连接
- 自动保存日志到文件（带时间戳）

## 许可证

与 GoMailZero 项目相同。

