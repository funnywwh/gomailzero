# GoMailZero 实施计划文档

## 项目概述

**GoMailZero** (gmz) 是一个生产级、安全、可扩展的最小邮件服务器，支持 SMTP/IMAP/TLS/DKIM/SPF/DMARC/反垃圾/双因子/WebMail，单二进制部署，内存 ≤ 256 MB，10 万邮箱可横向扩容。

### 核心目标

- 一个 `gmz` 二进制 + 一个 `gmz.yml` 配置文件，60 秒内拉起安全合规的邮件服务器
- 通过 Google/Yahoo 2024 反垃圾新规
- 单二进制部署，零依赖
- 内存占用 ≤ 256 MB（空载 ≤ 128 MB）
- 支持 10 万邮箱横向扩容

## 技术架构

### 技术栈选型

| 功能 | 选用库 | 版本要求 | 理由 |
|------|--------|----------|------|
| SMTP 协议 | github.com/emersion/go-smtp | latest | 活跃、生产验证 |
| IMAP 协议 | github.com/emersion/go-imap | latest | 活跃、生产验证 |
| TLS/ACME | crypto/tls + golang.org/x/crypto/acme | latest | 官方标准库 |
| SQLite | modernc.org/sqlite | latest | cgo-free，交叉编译友好 |
| Postgres | github.com/jackc/pgx/v5 | v5.x | 高性能驱动 |
| 加密 | golang.org/x/crypto/chacha20poly1305 | latest | 标准加密库 |
| 2FA TOTP | github.com/pquerna/otp | latest | 成熟稳定 |
| Web 框架 | github.com/gin-gonic/gin | latest | 高性能 |
| 配置管理 | github.com/spf13/viper | latest | 热更新支持 |
| 日志 | github.com/rs/zerolog | latest | 结构化、高性能 |
| 指标 | github.com/prometheus/client_golang | latest | 官方标准 |
| 数据库迁移 | github.com/pressly/goose/v3 | v3.x | 简单可靠 |

### 目录结构

```
gomailzero/
├── cmd/
│   └── gmz/
│       └── main.go                    # 主入口
├── internal/
│   ├── config/                        # 配置管理
│   │   ├── config.go
│   │   └── hotreload.go
│   ├── smtpd/                         # SMTP 服务器
│   │   ├── server.go
│   │   ├── handler.go
│   │   ├── auth.go
│   │   └── relay.go
│   ├── imapd/                         # IMAP 服务器
│   │   ├── server.go
│   │   ├── handler.go
│   │   └── auth.go
│   ├── storage/                       # 存储层
│   │   ├── driver.go
│   │   ├── sqlite.go
│   │   ├── postgres.go
│   │   ├── maildir.go
│   │   └── metadata.go
│   ├── crypto/                        # 加密模块
│   │   ├── encrypt.go
│   │   ├── keyderiv.go
│   │   └── argon2.go
│   ├── acme/                          # ACME 客户端
│   │   ├── client.go
│   │   ├── certmanager.go
│   │   └── dns.go
│   ├── antispam/                      # 反垃圾模块
│   │   ├── engine.go
│   │   ├── dkim.go
│   │   ├── spf.go
│   │   ├── dmarc.go
│   │   ├── greylist.go
│   │   ├── ratelimit.go
│   │   └── clamav.go
│   ├── web/                           # WebMail
│   │   ├── server.go
│   │   ├── api.go
│   │   └── embed.go
│   ├── api/                           # 管理 API
│   │   ├── server.go
│   │   ├── handlers.go
│   │   ├── auth.go
│   │   └── middleware.go
│   ├── auth/                          # 认证模块
│   │   ├── totp.go
│   │   └── webauthn.go
│   ├── metrics/                       # Prometheus 指标
│   │   └── exporter.go
│   ├── logger/                        # 结构化日志
│   │   └── logger.go
│   └── migrate/                       # 数据库迁移
│       └── migrate.go
├── webmail/                           # Vue3 前端源码
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── migrations/                        # SQL 迁移文件
│   ├── 00001_init.up.sql
│   └── 00001_init.down.sql
├── configs/
│   └── gmz.yml.example               # 配置示例
├── scripts/
│   ├── install.sh                    # systemd 安装
│   ├── upgrade.sh                    # 热升级脚本
│   └── ci/
│       ├── test.sh
│       └── build.sh
├── docker/
│   └── Dockerfile                    # 多阶段构建
├── docs/
│   ├── openapi.yaml                  # OpenAPI 文档
│   └── grafana.json                  # Grafana 仪表板
├── test/
│   ├── integration/                  # 集成测试
│   └── fixtures/                     # 测试数据
├── .cursorrules                      # Cursor 规则
├── PLAN.md                           # 本文档
├── UPGRADE.md                        # 升级文档
├── README.md                         # 中文 README
├── README_EN.md                      # 英文 README
├── CHANGELOG.md                      # 变更日志
├── docker-compose.yml                # 开发环境
├── go.mod
├── go.sum
└── Makefile                          # 构建脚本
```

## 实施里程碑

### M1: 基础协议 + TLS + 存储 (Day 1-3)

**目标**: SMTP/IMAP 裸跑 + TLS + SQLite 存储

#### 任务清单

1. **项目初始化**
   - [ ] 创建 go.mod，设置模块路径
   - [ ] 创建完整目录结构
   - [ ] 初始化 Makefile
   - [ ] 设置 CI/CD 基础配置

2. **配置管理** (`internal/config`)
   - [ ] 定义配置结构体（YAML 映射）
   - [ ] 实现 viper 配置加载
   - [ ] 实现配置热更新机制
   - [ ] 支持环境变量覆盖
   - [ ] 配置验证和默认值

3. **存储层** (`internal/storage`)
   - [ ] 定义存储接口（Driver）
   - [ ] 实现 SQLite 驱动（modernc.org/sqlite）
   - [ ] 实现数据库迁移（goose）
   - [ ] 实现 Maildir++ 存储
   - [ ] 实现邮件元数据管理（SQLite）
   - [ ] 实现用户/域名/别名管理

4. **SMTP 服务器** (`internal/smtpd`)
   - [ ] 实现 SMTP 服务器基础框架
   - [ ] 监听端口 25/465/587
   - [ ] 实现 STARTTLS 支持
   - [ ] 实现 SMTPS (465) 支持
   - [ ] 实现 SMTP-AUTH (PLAIN/LOGIN)
   - [ ] 实现 RCPT TO 检查（拒绝开放中继）
   - [ ] 实现邮件接收和存储
   - [ ] 实现队列管理（内存队列）

5. **IMAP 服务器** (`internal/imapd`)
   - [ ] 实现 IMAP 服务器基础框架
   - [ ] 监听端口 993（TLS 强制）
   - [ ] 实现 PLAIN 认证
   - [ ] 实现基础命令（LOGIN, SELECT, FETCH, LIST）
   - [ ] 实现 IDLE 支持
   - [ ] 实现 QUOTA 支持
   - [ ] 实现 SPECIAL-USE 文件夹

6. **TLS 集成**
   - [ ] 实现 TLS 配置加载
   - [ ] 强制 TLS 1.3（最低 TLS 1.2）
   - [ ] 实现证书链验证
   - [ ] 集成到 SMTP/IMAP 服务器

7. **测试**
   - [ ] 编写配置管理单测
   - [ ] 编写存储层单测
   - [ ] 编写 SMTP 集成测试
   - [ ] 编写 IMAP 集成测试
   - [ ] 编写 TLS 测试

#### 验收标准

- ✅ `swaks --to external@gmail.com` 返回 `550 Relay denied`
- ✅ `swaks --server localhost:587 --auth-user test@example.com --auth-password *** --to local@example.com` 成功投递
- ✅ IMAP 连接 993 端口，TLS 握手成功
- ✅ 单测覆盖率 ≥ 60%

#### 关键文件

- `cmd/gmz/main.go` - 主入口，服务启动
- `internal/config/config.go` - 配置结构和管理
- `internal/storage/sqlite.go` - SQLite 存储驱动
- `internal/storage/maildir.go` - Maildir 实现
- `internal/smtpd/server.go` - SMTP 服务器
- `internal/imapd/server.go` - IMAP 服务器

---

### M2: 安全增强 + ACME + 邮件安全 (Day 4-6)

**目标**: 存储加密 + ACME 自动证书 + DKIM/SPF/DMARC

#### 任务清单

1. **存储加密** (`internal/crypto`)
   - [ ] 实现 XChaCha20-Poly1305 加密/解密
   - [ ] 实现 Argon2id 密钥派生（从用户密码）
   - [ ] 实现密钥管理（内存缓存）
   - [ ] 集成到邮件存储层
   - [ ] 实现加密邮件读取/写入

2. **ACME 客户端** (`internal/acme`)
   - [ ] 实现 ACME 客户端（golang.org/x/crypto/acme）
   - [ ] 实现 HTTP-01 挑战
   - [ ] 实现 DNS-01 挑战（可选）
   - [ ] 实现证书自动申请
   - [ ] 实现证书自动续期（30 天检查）
   - [ ] 实现 ECDSA P-256 证书生成
   - [ ] 实现证书热重载（零中断）

3. **DKIM** (`internal/antispam/dkim`)
   - [ ] 实现 DKIM 签名生成
   - [ ] 实现 DKIM 验证
   - [ ] 实现密钥对管理（RSA/Ed25519）
   - [ ] 集成到 SMTP 发送流程
   - [ ] 集成到 SMTP 接收流程

4. **SPF** (`internal/antispam/spf`)
   - [ ] 实现 SPF 记录解析
   - [ ] 实现 SPF 检查逻辑
   - [ ] 实现 DNS 查询缓存
   - [ ] 集成到 SMTP 接收流程

5. **DMARC** (`internal/antispam/dmarc`)
   - [ ] 实现 DMARC 记录解析
   - [ ] 实现 DMARC 策略评估
   - [ ] 实现 DMARC 报告生成（可选）
   - [ ] 集成到邮件接收流程

6. **灰名单** (`internal/antispam/greylist`)
   - [ ] 实现灰名单存储（SQLite）
   - [ ] 实现灰名单检查逻辑
   - [ ] 实现自动白名单机制

7. **速率限制** (`internal/antispam/ratelimit`)
   - [ ] 实现令牌桶算法
   - [ ] 实现 IP 级别限速
   - [ ] 实现用户级别限速
   - [ ] 实现 HELO 检查

8. **测试**
   - [ ] 编写加密存储单测
   - [ ] 编写 ACME 客户端测试（使用测试环境）
   - [ ] 编写 DKIM/SPF/DMARC 单测
   - [ ] 编写集成测试

#### 验收标准

- ✅ SSL Labs 评级 A+
- ✅ mail-tester.com ≥ 9/10
- ✅ 拔掉磁盘挂载到别机，无法读取邮件明文
- ✅ DKIM 签名验证通过
- ✅ SPF/DMARC 检查正常工作

#### 关键文件

- `internal/crypto/encrypt.go` - 邮件体加密
- `internal/crypto/keyderiv.go` - 密钥派生
- `internal/acme/client.go` - ACME 客户端
- `internal/acme/certmanager.go` - 证书管理
- `internal/antispam/dkim.go` - DKIM 实现
- `internal/antispam/spf.go` - SPF 实现
- `internal/antispam/dmarc.go` - DMARC 实现

---

### M3: 反垃圾 + WebMail + 2FA (Day 7-9)

**目标**: 完整反垃圾引擎 + WebMail 界面 + 双因子认证

#### 任务清单

1. **反垃圾引擎** (`internal/antispam/engine`)
   - [ ] 实现评分系统（0-100 分）
   - [ ] 实现规则链（Rule Chain）
   - [ ] 集成 SPF/DKIM/DMARC 评分
   - [ ] 实现灰名单评分
   - [ ] 实现速率限制评分
   - [ ] 实现 HELO 检查评分
   - [ ] 实现内容扫描（基础关键词，可选）

2. **ClamAV 集成** (`internal/antispam/clamav`)
   - [ ] 实现 ClamAV Unix Socket 客户端
   - [ ] 实现 ClamAV HTTP API 客户端
   - [ ] 实现病毒扫描集成
   - [ ] 实现扫描结果缓存

3. **TOTP 双因子** (`internal/auth/totp`)
   - [ ] 实现 TOTP 生成/验证
   - [ ] 实现密钥管理（加密存储）
   - [ ] 扩展 SMTP-AUTH 支持 TOTP
   - [ ] 扩展 IMAP 支持 TOTP
   - [ ] 实现恢复码机制

4. **WebAuthn** (`internal/auth/webauthn`)
   - [ ] 实现 WebAuthn 注册流程
   - [ ] 实现 WebAuthn 认证流程
   - [ ] 实现密钥存储

5. **WebMail 前端** (`webmail/`)
   - [ ] 初始化 Vue3 + Vite 项目
   - [ ] 实现登录页面（TOTP/WebAuthn）
   - [ ] 实现邮件列表页面
   - [ ] 实现邮件阅读页面
   - [ ] 实现邮件编写页面
   - [ ] 集成 OpenPGP.js（PGP 加密/签名）
   - [ ] 实现文件夹管理
   - [ ] 实现搜索功能
   - [ ] 优化性能（Lighthouse ≥ 90）

6. **WebMail 后端** (`internal/web`)
   - [ ] 实现 Web 服务器（Gin）
   - [ ] 实现登录 API
   - [ ] 实现邮件列表 API
   - [ ] 实现邮件读取 API
   - [ ] 实现邮件发送 API
   - [ ] 实现文件夹管理 API
   - [ ] 实现搜索 API
   - [ ] 实现静态文件嵌入（go:embed）

7. **测试**
   - [ ] 编写反垃圾引擎单测
   - [ ] 编写 TOTP/WebAuthn 单测
   - [ ] 编写 WebMail API 测试
   - [ ] 编写前端 E2E 测试（可选）

#### 验收标准

- ✅ mail-tester.com ≥ 9/10，无 false-positive
- ✅ WebMail 登录支持 TOTP/WebAuthn
- ✅ Lighthouse 性能 ≥ 90
- ✅ PGP 加密/签名功能正常
- ✅ 反垃圾引擎评分准确

#### 关键文件

- `internal/antispam/engine.go` - 反垃圾引擎
- `internal/antispam/clamav.go` - ClamAV 集成
- `internal/auth/totp.go` - TOTP 实现
- `internal/auth/webauthn.go` - WebAuthn 实现
- `internal/web/api.go` - WebMail API
- `internal/web/embed.go` - 静态文件嵌入
- `webmail/` - Vue3 前端源码

---

### M4: 管理 API + 监控 (Day 10-12)

**目标**: REST API + Prometheus 指标 + Grafana 仪表板

#### 任务清单

1. **管理 API** (`internal/api`)
   - [ ] 实现 REST API 服务器（Gin）
   - [ ] 实现 JWT 认证中间件
   - [ ] 实现 API 密钥认证
   - [ ] 实现域名管理 API（CRUD）
   - [ ] 实现用户管理 API（CRUD）
   - [ ] 实现别名管理 API（CRUD）
   - [ ] 实现配额管理 API
   - [ ] 实现日志查询 API
   - [ ] 实现队列管理 API
   - [ ] 实现 OpenAPI 3.1 文档生成

2. **结构化日志** (`internal/logger`)
   - [ ] 实现 zerolog 集成
   - [ ] 实现 JSON 格式输出
   - [ ] 实现 trace_id 生成和传播
   - [ ] 实现日志级别管理
   - [ ] 实现 Loki/Promtail 兼容格式

3. **Prometheus 指标** (`internal/metrics`)
   - [ ] 实现指标注册
   - [ ] 实现 SMTP 指标（连接数、消息数、错误数）
   - [ ] 实现 IMAP 指标（连接数、操作数、错误数）
   - [ ] 实现队列指标（堆积数、处理速度）
   - [ ] 实现 TLS 指标（握手失败、证书过期）
   - [ ] 实现认证指标（失败次数、暴力破解检测）
   - [ ] 实现存储指标（磁盘使用、邮件数）
   - [ ] 暴露 `/metrics` 端点

4. **告警规则**
   - [ ] 定义队列堆积告警（> 1000）
   - [ ] 定义 TLS 握手失败告警（> 10/min）
   - [ ] 定义认证暴力破解告警（> 5/min from same IP）
   - [ ] 定义证书过期告警（< 7 days）

5. **Grafana 仪表板** (`docs/grafana.json`)
   - [ ] 创建 SMTP 监控面板
   - [ ] 创建 IMAP 监控面板
   - [ ] 创建队列监控面板
   - [ ] 创建安全监控面板
   - [ ] 创建存储监控面板
   - [ ] 导出 JSON 配置

6. **配置热更新**
   - [ ] 实现配置文件监听
   - [ ] 实现配置变更通知
   - [ ] 实现零重启配置重载

7. **测试**
   - [ ] 编写 API 单测
   - [ ] 编写 API 集成测试
   - [ ] 创建 Postman 集合
   - [ ] 编写 CI 自动测试

#### 验收标准

- ✅ 提供完整的 OpenAPI 文档
- ✅ 提供 Postman 集合
- ✅ CI 自动测试通过
- ✅ Grafana 仪表板可一键导入
- ✅ 配置热更新零重启

#### 关键文件

- `internal/api/server.go` - REST API 服务器
- `internal/api/handlers.go` - API 处理器
- `internal/api/auth.go` - JWT 认证
- `internal/metrics/exporter.go` - Prometheus 导出
- `internal/logger/logger.go` - 结构化日志
- `docs/openapi.yaml` - OpenAPI 文档
- `docs/grafana.json` - Grafana 配置

---

### M5: 测试 + 文档 (Day 13-14)

**目标**: 单测覆盖率 ≥ 80%，集成测试 ≥ 50 条场景，文档补齐

#### 任务清单

1. **单元测试**
   - [ ] 补充所有模块单测
   - [ ] 确保覆盖率 ≥ 80%
   - [ ] 使用 `go test -cover` 验证

2. **集成测试** (`test/integration/`)
   - [ ] SMTP 测试场景（10+）
   - [ ] IMAP 测试场景（10+）
   - [ ] TLS 测试场景（5+）
   - [ ] 认证测试场景（5+）
   - [ ] 反垃圾测试场景（10+）
   - [ ] 加密存储测试场景（5+）
   - [ ] API 测试场景（10+）
   - [ ] 总计 ≥ 50 场景

3. **数据库迁移** (`migrations/`)
   - [ ] 创建初始迁移文件
   - [ ] 实现迁移脚本（goose）
   - [ ] 实现回滚脚本
   - [ ] 集成到启动流程（自动迁移）

4. **文档编写**
   - [ ] 编写中文 README.md
     - 项目介绍
     - 快速开始
     - 配置说明
     - 部署指南
     - 故障排查
   - [ ] 编写英文 README_EN.md
   - [ ] 编写 CHANGELOG.md
   - [ ] 更新 OpenAPI 文档

5. **部署脚本**
   - [ ] 创建 `scripts/install.sh`（systemd 集成）
   - [ ] 创建 `scripts/upgrade.sh`（热升级）
   - [ ] 创建 `scripts/backup.sh`（数据备份）
   - [ ] 创建 `scripts/restore.sh`（数据恢复）

6. **Docker 构建**
   - [ ] 创建多阶段 Dockerfile
   - [ ] 使用 distroless 基础镜像
   - [ ] 优化镜像大小（< 100MB）
   - [ ] 创建 docker-compose.yml（开发环境）

7. **性能测试和优化**
   - [ ] 单核 SMTP 1k msg/s 测试
   - [ ] IMAP 登录 < 50ms 测试
   - [ ] 内存占用测试（空载 ≤ 128MB，10k 连接 ≤ 256MB）
   - [ ] 10 万邮件查询 < 200ms 测试
   - [ ] 性能优化（如有必要）

#### 验收标准

- ✅ 单测覆盖率 ≥ 80%
- ✅ 集成测试 ≥ 50 场景，全部通过
- ✅ 文档完整，中英双语
- ✅ 一键安装脚本可用
- ✅ Docker 镜像构建成功
- ✅ 性能指标达标

#### 关键文件

- `*_test.go` - 各模块测试文件
- `test/integration/` - 集成测试
- `migrations/` - SQL 迁移文件
- `README.md` / `README_EN.md` - 文档
- `scripts/install.sh` / `scripts/upgrade.sh` - 部署脚本
- `docker/Dockerfile` - Docker 构建
- `docker-compose.yml` - 开发环境

---

### M6: 发布准备 (Day 15)

**目标**: Release v0.9.0 二进制 + Docker 镜像

#### 任务清单

1. **构建多架构二进制**
   - [ ] Linux x86_64 构建
   - [ ] Linux arm64 构建
   - [ ] 验证二进制大小（≤ 60MB）
   - [ ] 验证二进制可执行性

2. **构建 Docker 镜像**
   - [ ] 多架构 Docker 镜像构建
   - [ ] 推送到 Docker Hub / GitHub Container Registry
   - [ ] 验证镜像可运行

3. **发布文档**
   - [ ] 创建 Release Notes
   - [ ] 更新 CHANGELOG.md
   - [ ] 创建 GitHub Release

4. **最终验证**
   - [ ] 验证所有验收标准
   - [ ] 验证一键安装流程
   - [ ] 验证热升级流程
   - [ ] 验证数据备份/恢复

5. **文档审查**
   - [ ] 审查所有文档
   - [ ] 修复文档错误
   - [ ] 补充缺失内容

#### 验收标准

- ✅ 单二进制 `gmz` (≤ 60 MB) 可用
- ✅ `gmz.yml.example` 配置示例完整
- ✅ 文档完整（中英双语）
- ✅ Docker 镜像可用
- ✅ 所有验收标准通过

---

## 验收标准总览

### 功能验收

1. ✅ `swaks --to external@gmail.com` 返回 `550 Relay denied`
2. ✅ `swaks --server localhost:587 --auth-user test@example.com --auth-password *** --to local@example.com` 成功投递
3. ✅ iPhone 自动配置 `https://example.com/.well-known/autoconfig.xml` 零警告
4. ✅ mail-tester.com ≥ 9/10
5. ✅ SSL Labs 评级 A+
6. ✅ 拔掉磁盘挂载到别机，无法读取邮件明文
7. ✅ 10 万封随机邮件导入，查询 IMAP 列表 < 200 ms
8. ✅ 一键 `scripts/upgrade.sh v0.9.1` 热升级，零中断

### 性能验收

- ✅ 单核 1k SMTP msg/s
- ✅ IMAP 登录 < 50 ms
- ✅ 内存：空载 ≤ 128 MB，10k 连接 ≤ 256 MB
- ✅ 磁盘：每用户 1k 封邮件 ≤ 50 MB 元数据

### 质量验收

- ✅ 单测覆盖率 ≥ 80%
- ✅ 集成测试 ≥ 50 场景
- ✅ 无 cgo 依赖（使用 modernc.org/sqlite）
- ✅ 单二进制部署（≤ 60 MB）

---

## 关键设计决策

### 1. 存储加密

- **方案**: 邮件体使用 XChaCha20-Poly1305 加密，密钥从用户密码通过 Argon2id 派生
- **理由**: 确保即使磁盘被物理访问，也无法读取邮件明文
- **实现**: `internal/crypto/encrypt.go`

### 2. 证书管理

- **方案**: 内置 ACME 客户端，自动申请/续期 Let's Encrypt 证书
- **理由**: 零配置 TLS，自动续期，零中断
- **实现**: `internal/acme/client.go`

### 3. 反垃圾策略

- **方案**: 先实现简化版（SPF/DKIM/DMARC/灰名单），后续可扩展完整 Rspamd 兼容
- **理由**: 平衡开发时间和功能完整性
- **实现**: `internal/antispam/engine.go`

### 4. WebMail 前端

- **方案**: Vue3 前端编译后嵌入二进制，使用 `go:embed`
- **理由**: 单二进制部署，无需额外静态文件
- **实现**: `internal/web/embed.go`

### 5. 热更新机制

- **方案**: 配置和证书支持热重载，零中断服务
- **理由**: 生产环境零停机更新
- **实现**: `internal/config/hotreload.go`, `internal/acme/certmanager.go`

### 6. 日志追踪

- **方案**: 所有日志包含 `trace_id`，支持链路追踪
- **理由**: 便于问题排查和性能分析
- **实现**: `internal/logger/logger.go`

---

## 开发规范

### 代码规范

1. **禁止引入 cgo**: 使用 `modernc.org/sqlite` 替代 `database/sqlite`
2. **禁止硬编码**: 所有密码/密钥通过环境变量或配置文件注入
3. **必须包含 trace_id**: 所有日志必须包含 `trace_id` 字段
4. **TDD 驱动**: 每写一个功能先写单测，再写实现，最后写集成测试

### 测试规范

1. **单元测试**: 每个模块必须有对应的 `*_test.go` 文件
2. **集成测试**: 放在 `test/integration/` 目录
3. **覆盖率**: 单测覆盖率必须 ≥ 80%
4. **测试数据**: 使用 `test/fixtures/` 存放测试数据

### 文档规范

1. **代码注释**: 所有公开函数必须有 GoDoc 注释
2. **README**: 必须包含快速开始、配置说明、部署指南
3. **API 文档**: 使用 OpenAPI 3.1 规范
4. **变更日志**: 每个版本更新 CHANGELOG.md

---

## 后续扩展路线图

### 短期（v0.10.0）

- 分布式队列：内置 NATS JetStream，支持多 MX 横向扩容
- 搜索优化：嵌入 bleve 全文搜索

### 中期（v0.11.0）

- 零信任：OIDC + SSO + WebAuthn Passkey
- 移动端推送：Apple APNS / FCM 代理

### 长期（v0.12.0+）

- S3 兼容对象存储：邮件体冷热分层
- 多租户支持：SaaS 模式
- 高级反垃圾：机器学习模型集成

---

## 风险与应对

### 技术风险

1. **ACME 证书申请失败**
   - 风险: DNS 配置错误或网络问题
   - 应对: 提供手动证书配置选项，详细错误日志

2. **性能不达标**
   - 风险: 10 万邮件查询 > 200ms
   - 应对: 使用索引优化，考虑分页和缓存

3. **内存占用超标**
   - 风险: 10k 连接 > 256MB
   - 应对: 连接池优化，及时释放资源

### 合规风险

1. **反垃圾邮件规则变化**
   - 风险: Google/Yahoo 更新规则
   - 应对: 持续监控，及时更新规则

2. **TLS 版本要求**
   - 风险: 新 TLS 版本要求
   - 应对: 使用标准库，及时更新 Go 版本

---

## 时间线

| 阶段 | 时间 | 状态 | 完成度 |
|------|------|------|--------|
| M1: 基础协议 + TLS + 存储 | Day 1-3 | ✅ 已完成 | 100% |
| M2: 安全增强 + ACME + 邮件安全 | Day 4-6 | 🚧 进行中 | 80% |
| M3: 反垃圾 + WebMail + 2FA | Day 7-9 | 🚧 进行中 | 60% |
| M4: 管理 API + 监控 | Day 10-12 | 🚧 进行中 | 70% |
| M5: 测试 + 文档 | Day 13-14 | 🚧 进行中 | 50% |
| M6: 发布准备 | Day 15 | ⏳ 待开始 | 0% |

### 详细进度

#### M1: 基础协议 + TLS + 存储 ✅ (100%)
- ✅ 项目初始化和目录结构
- ✅ 配置管理系统（viper，支持热更新）
- ✅ SQLite 存储驱动（WAL 模式）
- ✅ Maildir++ 邮件存储
- ✅ SMTP 服务器（支持 AUTH、STARTTLS）
- ✅ IMAP 服务器（支持登录、邮箱管理、邮件操作）
- ✅ TLS 配置和加载
- ✅ 结构化日志系统

#### M2: 安全增强 + ACME + 邮件安全 🚧 (80%)
- ✅ 邮件加密（XChaCha20-Poly1305）
- ✅ 密码哈希（Argon2id）
- ✅ ACME 客户端基础实现
- ✅ DKIM 签名和验证基础
- ✅ SPF 记录解析和检查基础
- ✅ DMARC 策略解析基础
- 🚧 ACME 证书自动续期和热重载
- 🚧 DKIM/SPF/DMARC 完整验证流程优化

#### M3: 反垃圾 + WebMail + 2FA 🚧 (60%)
- ✅ 反垃圾邮件引擎（评分系统）
- ✅ 规则链实现
- ✅ 灰名单机制
- ✅ 速率限制
- ✅ TOTP 双因子认证基础
- ✅ JWT 认证系统
- ✅ WebMail 后端基础（API、中间件）
- 🚧 ClamAV 病毒扫描集成
- 🚧 WebAuthn 支持
- 🚧 WebMail 前端（Vue3）

#### M4: 管理 API + 监控 🚧 (70%)
- ✅ 管理 API 基础功能（域名、用户、别名、配额）
- ✅ Prometheus 指标导出
- ✅ API 认证（API Key + JWT）
- 🚧 API 2FA 认证
- 🚧 日志查询 API
- 🚧 队列管理 API
- 🚧 OpenAPI 文档生成

#### M5: 测试 + 文档 🚧 (50%)
- ✅ 单元测试框架
- ✅ 部分模块测试（config, storage, crypto, antispam, auth, api）
- ✅ CI/CD 配置
- ✅ 安全扫描（gosec, golangci-lint）
- 🚧 集成测试完善
- 🚧 测试覆盖率提升到 80%
- 🚧 文档完善（README, API 文档）

#### M6: 发布准备 ⏳ (0%)
- ⏳ 构建脚本优化
- ⏳ Docker 镜像构建
- ⏳ 安装脚本
- ⏳ 升级脚本
- ⏳ 发布文档

---

**最后更新**: 2025-11-11  
**版本**: v0.1.0 (计划文档)  
**总体进度**: 约 70% 完成

