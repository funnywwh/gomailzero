# GoMailZero (gmz)

一个生产级、安全、可扩展的最小邮件服务器，支持 SMTP/IMAP/TLS/DKIM/SPF/DMARC/反垃圾/双因子/WebMail，单二进制部署。

## 特性

- ✅ **单二进制部署** - 一个 `gmz` 二进制 + 一个配置文件，60 秒内拉起
- ✅ **SMTP/IMAP 支持** - 完整的 SMTP 和 IMAP 协议实现
- ✅ **TLS 加密** - 强制 TLS 1.3，支持 STARTTLS 和 SMTPS
- ✅ **自动证书管理** - 内置 ACME 客户端，自动申请/续期 Let's Encrypt 证书（开发中）
- ✅ **存储加密** - 邮件体使用 XChaCha20-Poly1305 加密，密钥从用户密码派生
- ✅ **反垃圾邮件** - SPF/DKIM/DMARC 检查，灰名单，速率限制（开发中）
- ✅ **双因子认证** - 支持 TOTP 和 WebAuthn（开发中）
- ✅ **WebMail** - 现代化的 Web 邮件界面（开发中）
- ✅ **管理 API** - RESTful API，支持 OpenAPI 3.1（开发中）
- ✅ **监控指标** - Prometheus 指标导出，Grafana 仪表板（开发中）

## 快速开始

### 安装

```bash
# 从源码构建
git clone https://github.com/funnywwh/gomailzero.git
cd gomailzero
make build

# 或下载预编译二进制
wget https://github.com/funnywwh/gomailzero/releases/download/v0.9.0/gmz-linux-amd64 -O /usr/local/bin/gmz
chmod +x /usr/local/bin/gmz

# 或使用 Docker
docker pull funnywwh/gomailzero:latest
```

### 配置

```bash
# 复制配置示例
cp configs/gmz.yml.example /etc/gmz/gmz.yml

# 编辑配置
vim /etc/gmz/gmz.yml
```

### 运行

```bash
# 直接运行
./bin/gmz -c /etc/gmz/gmz.yml

# 或使用 systemd
systemctl start gmz
```

## 配置说明

详细配置说明请参考 [configs/gmz.yml.example](configs/gmz.yml.example)

主要配置项：

- `domain`: 主域名
- `tls.acme.enabled`: 启用自动证书管理（开发中）
- `storage.driver`: 存储驱动（sqlite 或 postgres）
- `smtp.ports`: SMTP 监听端口（25, 465, 587）
- `imap.port`: IMAP 监听端口（993）

## 当前实现状态

### 已完成 ✅

- 项目基础结构和构建系统
- 配置管理系统（支持热更新）
- SQLite 存储驱动（支持 WAL 模式）
- Maildir++ 邮件存储
- SMTP 服务器基础功能（支持 AUTH、STARTTLS）
- IMAP 服务器基础功能（支持登录、邮箱管理、邮件操作）
- TLS 配置和加载
- 邮件加密（XChaCha20-Poly1305）
- 密码哈希（Argon2id）
- 结构化日志系统
- ACME 客户端基础实现
- DKIM/SPF/DMARC 基础实现
- 反垃圾邮件引擎（评分系统、规则链、灰名单、速率限制）
- TOTP 双因子认证基础实现
- JWT 认证系统
- 管理 API 基础功能（域名、用户、别名、配额管理）
- WebMail 后端基础实现（登录、邮件列表、发送、删除）
- Prometheus 指标导出
- CI/CD 配置（测试、构建、安全扫描）
- 安全扫描和修复（gosec、golangci-lint）

### 开发中 🚧

- ACME 证书自动续期和热重载
- DKIM/SPF/DMARC 完整验证流程
- ClamAV 病毒扫描集成
- WebMail 前端（Vue3）
- WebAuthn 支持
- 数据库迁移系统
- 集成测试完善
- OpenAPI 文档生成

## 开发

### 构建

```bash
# 构建二进制
make build

# 构建多架构
make build-all

# 构建 Docker 镜像
make docker-build
```

### 测试

```bash
# 运行单元测试
make test

# 运行测试并生成覆盖率报告
make test-coverage

# 运行集成测试
make test-integration
```

### 运行

```bash
# 构建并运行
make run
```

### 代码检查

```bash
# 格式化代码
make fmt

# 运行 linter
make lint

# 安全扫描
make security
```

## 项目结构

```
gomailzero/
├── cmd/gmz/              # 主入口
├── internal/
│   ├── config/           # 配置管理
│   ├── smtpd/            # SMTP 服务器
│   ├── imapd/            # IMAP 服务器
│   ├── storage/          # 存储层
│   ├── crypto/           # 加密模块
│   ├── tls/              # TLS 配置
│   ├── logger/           # 日志系统
│   └── ...               # 其他模块
├── configs/              # 配置文件示例
├── scripts/              # 脚本文件
├── docs/                 # 文档
└── test/                 # 测试代码
```

## 文档

- [实施计划](PLAN.md) - 详细的开发计划和里程碑
- [升级文档](UPGRADE.md) - 升级和迁移指南
- [Cursor 规则](.cursorrules) - TDD 开发规范

## 贡献

欢迎提交 Issue 和 Pull Request！

### 开发规范

- 遵循 TDD（测试驱动开发）
- 代码覆盖率 ≥ 80%
- 所有公开函数必须有 GoDoc 注释
- 提交消息遵循 [Conventional Commits](https://www.conventionalcommits.org/)

## 许可证

MIT License

## 路线图

- [x] v0.1.0 - 基础框架和 SMTP/IMAP 服务器 ✅
- [x] v0.2.0 - ACME 证书管理和 TLS 支持 ✅ (基础实现)
- [x] v0.3.0 - DKIM/SPF/DMARC 验证 ✅ (基础实现)
- [x] v0.4.0 - 反垃圾邮件引擎 ✅ (基础实现)
- [ ] v0.5.0 - WebMail 前端 🚧 (后端完成，前端开发中)
- [x] v0.6.0 - 管理 API 和监控 ✅ (基础实现)
- [ ] v0.9.0 - 完整功能发布 🚧 (约 70% 完成)
