# Changelog

所有重要的变更都会记录在这个文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 新增
- 项目基础结构和构建系统
- 配置管理系统（支持热更新）
- SQLite 存储驱动（支持 WAL 模式）
- Maildir++ 邮件存储实现
- SMTP 服务器基础功能
- IMAP 服务器基础功能
- TLS 配置和加载支持
- 邮件加密（XChaCha20-Poly1305）
- 密码哈希（Argon2id）
- 结构化日志系统（zerolog）

### 开发中
- ACME 自动证书管理
- DKIM/SPF/DMARC 验证
- 反垃圾邮件引擎
- WebMail 前端
- 管理 API
- Prometheus 指标导出

## [0.1.0] - 2025-11-11

### 新增
- 初始版本发布
- 基础项目框架
- SMTP/IMAP 服务器核心功能
- 存储和加密模块

[Unreleased]: https://github.com/funnywwh/gomailzero/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/funnywwh/gomailzero/releases/tag/v0.1.0

