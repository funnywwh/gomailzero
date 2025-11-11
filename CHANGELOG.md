# Changelog

所有重要的变更都会记录在这个文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 计划中
- ACME 证书自动续期和热重载优化
- DKIM/SPF/DMARC 完整验证流程优化
- WebMail 前端完整功能（文件夹管理、搜索、PGP 支持）
- WebAuthn 密钥存储实现
- OpenAPI 文档自动生成
- 性能测试和优化

## [0.9.0] - 2025-01-20

### 新增
- **核心功能**
  - SMTP 服务器（支持 AUTH、STARTTLS、SMTPS）
  - IMAP 服务器（支持登录、邮箱管理、邮件操作）
  - TLS 配置和加载（支持 TLS 1.3，强制加密）
  - 邮件加密（XChaCha20-Poly1305）
  - 密码哈希（Argon2id）
  - 结构化日志系统（zerolog，支持 trace_id）

- **存储系统**
  - SQLite 存储驱动（支持 WAL 模式，高性能）
  - Maildir++ 邮件存储实现
  - 数据库迁移系统（goose，支持自动迁移）

- **安全功能**
  - TOTP 双因子认证（支持 SMTP/IMAP/WebMail/API）
  - JWT 认证系统
  - WebAuthn 基础实现（注册和认证流程）
  - 反垃圾邮件引擎（评分系统、规则链、灰名单、速率限制）
  - DKIM/SPF/DMARC 基础实现

- **WebMail**
  - Vue3 + Vite 前端项目
  - 登录页面（支持 TOTP 2FA）
  - 邮件列表页面（文件夹导航）
  - 邮件阅读和编写页面
  - WebMail 后端 API（登录、邮件列表、发送、删除、标志更新）

- **管理 API**
  - RESTful API（域名、用户、别名、配额管理）
  - 支持 API Key 和 JWT 两种认证方式
  - TOTP 中间件（敏感操作需要 2FA）

- **ACME 证书管理**
  - ACME 客户端基础实现
  - Let's Encrypt 证书申请支持

- **监控和指标**
  - Prometheus 指标导出
  - 结构化日志（支持 Loki/Promtail）

- **部署和维护**
  - 一键安装脚本（systemd 集成）
  - 热升级脚本（支持数据库迁移）
  - 数据备份脚本（数据库、邮件、证书、配置）
  - 数据恢复脚本（支持选择性恢复）

- **Docker 支持**
  - 多阶段 Dockerfile（使用 distroless 基础镜像）
  - docker-compose.yml（生产环境）
  - docker-compose.dev.yml（开发环境，支持热重载）

### 改进
- 优化 Docker 镜像大小（使用 distroless）
- 完善错误处理和日志记录
- 改进安全配置（文件权限、systemd 安全设置）
- 完善文档（README、UPGRADE、维护指南）

### 修复
- 修复 TOTP 中间件逻辑（API Key 认证不需要 TOTP）
- 修复 WebMail 服务器初始化（JWT 密钥来源）
- 修复迁移系统编译错误
- 修复安全扫描发现的问题（gosec、golangci-lint）

### 文档
- 完善 README.md（安装、配置、维护、Docker 部署）
- 创建 UPGRADE.md（升级和迁移指南）
- 创建 .cursorrules（TDD 开发规范）
- 更新 CHANGELOG.md

## [0.1.0] - 2025-11-11

### 新增
- 初始版本发布
- 基础项目框架
- SMTP/IMAP 服务器核心功能
- 存储和加密模块

[Unreleased]: https://github.com/funnywwh/gomailzero/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/funnywwh/gomailzero/compare/v0.1.0...v0.9.0
[0.1.0]: https://github.com/funnywwh/gomailzero/releases/tag/v0.1.0

