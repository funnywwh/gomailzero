# GoMailZero (gmz)

一个生产级、安全、可扩展的最小邮件服务器，支持 SMTP/IMAP/TLS/DKIM/SPF/DMARC/反垃圾/双因子/WebMail，单二进制部署。

## 特性

- ✅ **单二进制部署** - 一个 `gmz` 二进制 + 一个配置文件，60 秒内拉起
- ✅ **SMTP/IMAP 支持** - 完整的 SMTP 和 IMAP 协议实现
- ✅ **TLS 加密** - 强制 TLS 1.3，支持 STARTTLS 和 SMTPS
- ✅ **自动证书管理** - 内置 ACME 客户端，自动申请/续期 Let's Encrypt 证书
- ✅ **存储加密** - 邮件体使用 XChaCha20-Poly1305 加密，密钥从用户密码派生
- ✅ **反垃圾邮件** - SPF/DKIM/DMARC 检查，灰名单，速率限制
- ✅ **双因子认证** - 支持 TOTP 和 WebAuthn
- ✅ **WebMail** - 现代化的 Web 邮件界面
- ✅ **管理 API** - RESTful API，支持 OpenAPI 3.1
- ✅ **监控指标** - Prometheus 指标导出，Grafana 仪表板

## 快速开始

### 安装

```bash
# 下载二进制
wget https://github.com/gomailzero/gmz/releases/download/v0.9.0/gmz-linux-amd64 -O /usr/local/bin/gmz
chmod +x /usr/local/bin/gmz

# 或使用 Docker
docker pull gomailzero/gmz:latest
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
./gmz -c /etc/gmz/gmz.yml

# 或使用 systemd
systemctl start gmz
```

## 配置说明

详细配置说明请参考 [configs/gmz.yml.example](configs/gmz.yml.example)

主要配置项：

- `domain`: 主域名
- `tls.acme.enabled`: 启用自动证书管理
- `storage.driver`: 存储驱动（sqlite 或 postgres）
- `smtp.ports`: SMTP 监听端口
- `imap.port`: IMAP 监听端口

## 开发

### 构建

```bash
make build
```

### 测试

```bash
make test
make test-coverage
```

### 运行

```bash
make run
```

## 文档

- [实施计划](PLAN.md)
- [升级文档](UPGRADE.md)
- [Cursor 规则](.cursorrules)

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

