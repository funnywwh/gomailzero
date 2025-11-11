# GoMailZero (gmz)

一个生产级、安全、可扩展的最小邮件服务器，支持 SMTP/IMAP/TLS/DKIM/SPF/DMARC/反垃圾/双因子/WebMail，单二进制部署。

## 特性

- ✅ **单二进制部署** - 一个 `gmz` 二进制 + 一个配置文件，60 秒内拉起
- ✅ **SMTP/IMAP 支持** - 完整的 SMTP 和 IMAP 协议实现
- ✅ **TLS 加密** - 强制 TLS 1.3，支持 STARTTLS 和 SMTPS
- ✅ **自动证书管理** - 内置 ACME 客户端，自动申请/续期 Let's Encrypt 证书
- ✅ **存储加密** - 邮件体使用 XChaCha20-Poly1305 加密，密钥从用户密码派生
- ✅ **反垃圾邮件** - SPF/DKIM/DMARC 检查，灰名单，速率限制
- ✅ **双因子认证** - 支持 TOTP 和 WebAuthn（基础实现）
- ✅ **WebMail** - 现代化的 Web 邮件界面（Vue3 + Vite）
- ✅ **管理 API** - RESTful API，支持 JWT 和 API Key 认证
- ✅ **监控指标** - Prometheus 指标导出

## 快速开始

### 安装

#### 方式一：使用安装脚本（推荐）

```bash
# 从源码构建
git clone https://github.com/funnywwh/gomailzero.git
cd gomailzero
make build

# 运行安装脚本
sudo ./scripts/install.sh ./bin/gmz
```

#### 方式二：手动安装

```bash
# 从源码构建
git clone https://github.com/funnywwh/gomailzero.git
cd gomailzero
make build

# 复制二进制
sudo cp bin/gmz /usr/local/bin/
sudo chmod +x /usr/local/bin/gmz

# 创建配置目录
sudo mkdir -p /etc/gmz
sudo cp configs/gmz.yml.example /etc/gmz/gmz.yml
```

#### 方式三：使用 Docker

```bash
# 使用 docker-compose（推荐）
docker-compose up -d

# 或直接运行
docker run -d \
  --name gomailzero \
  -p 25:25 -p 465:465 -p 587:587 -p 993:993 \
  -p 8080:8080 -p 8081:8081 -p 9090:9090 \
  -v gmz-data:/var/lib/gmz \
  -v gmz-config:/etc/gmz \
  funnywwh/gomailzero:latest
```

### 配置

```bash
# 复制配置示例
cp configs/gmz.yml.example /etc/gmz/gmz.yml

# 编辑配置
vim /etc/gmz/gmz.yml
```

### 运行

#### 使用 systemd 服务（推荐）

```bash
# 启动服务
sudo systemctl start gmz

# 查看状态
sudo systemctl status gmz

# 查看日志
sudo journalctl -u gmz -f

# 停止服务
sudo systemctl stop gmz
```

#### 直接运行

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
- `tls.acme.enabled`: 启用自动证书管理
- `admin.jwt_secret`: JWT 密钥（用于 WebMail 和管理 API 认证）
- `storage.driver`: 存储驱动（sqlite 或 postgres）
- `smtp.ports`: SMTP 监听端口（25, 465, 587）
- `imap.port`: IMAP 监听端口（993）

## 维护

### 备份

```bash
# 使用备份脚本
sudo ./scripts/backup.sh

# 备份文件保存在 /var/lib/gmz/backups/
```

### 恢复

```bash
# 使用恢复脚本
sudo ./scripts/restore.sh /var/lib/gmz/backups/gmz_backup_YYYYMMDD_HHMMSS.tar.gz
```

### 升级

```bash
# 构建新版本
make build

# 使用升级脚本
sudo ./scripts/upgrade.sh v0.9.1 ./bin/gmz
```

### 数据库迁移

```bash
# 查看迁移状态
./gmz -migrate status -c /etc/gmz/gmz.yml

# 执行迁移
./gmz -migrate up -c /etc/gmz/gmz.yml

# 回滚迁移
./gmz -migrate down -c /etc/gmz/gmz.yml
```

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
- WebMail 后端完整实现（登录、邮件列表、发送、删除、搜索、文件夹、草稿）
- WebMail 前端完整功能（邮件列表、查看、编写、搜索、文件夹导航、回复、转发、标记）
- Prometheus 指标导出
- CI/CD 配置（测试、构建、安全扫描）
- 安全扫描和修复（gosec、golangci-lint）

### 开发中 🚧

- ACME 证书自动续期和热重载优化
- DKIM/SPF/DMARC 完整验证流程优化
- WebMail PGP 加密/签名支持
- WebAuthn 密钥存储实现
- 集成测试完善（更多场景）
- OpenAPI 文档自动生成
- 性能测试和优化
- 邮件附件支持

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

## Docker 部署

### 使用 docker-compose

```bash
# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down

# 查看状态
docker-compose ps
```

### 开发环境

```bash
# 使用开发配置（挂载源代码，支持热重载）
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

### 数据持久化

Docker Compose 使用命名卷来持久化数据：
- `gmz-data`: 数据库和邮件数据
- `gmz-config`: 配置文件
- `gmz-certs`: TLS 证书

### 自定义配置

1. 创建 `docker-compose.override.yml`（不会被 git 跟踪）
2. 覆盖默认配置

```yaml
version: '3.8'
services:
  gmz:
    volumes:
      - ./my-config.yml:/etc/gmz/gmz.yml:ro
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
