# GoMailZero 升级文档

## 概述

本文档描述 GoMailZero (gmz) 的升级流程，包括热升级、数据迁移、回滚等操作。

## 升级类型

### 1. 热升级（零中断）

热升级支持在不中断服务的情况下升级二进制文件和配置。

**适用场景**:
- 二进制文件更新
- 配置文件更新
- 证书轮换

**限制**:
- 数据库 schema 变更需要重启（但可通过迁移脚本最小化中断）
- 某些重大协议变更可能需要重启

### 2. 冷升级（需要重启）

冷升级需要停止服务、更新、重启。

**适用场景**:
- 数据库 schema 重大变更
- 协议版本变更
- 重大安全更新

## 升级前准备

### 1. 备份数据

```bash
# 使用内置备份脚本
./scripts/backup.sh

# 或手动备份
cp -r /var/lib/gmz /var/lib/gmz.backup.$(date +%Y%m%d)
cp /etc/gmz/gmz.yml /etc/gmz/gmz.yml.backup.$(date +%Y%m%d)
```

### 2. 检查当前版本

```bash
./gmz version
```

### 3. 查看变更日志

```bash
# 查看 CHANGELOG.md
cat CHANGELOG.md

# 或查看 Release Notes
curl https://api.github.com/repos/gomailzero/gmz/releases/latest
```

### 4. 检查数据库迁移

```bash
# 查看待执行的迁移
./gmz migrate status

# 预览迁移 SQL（不执行）
./gmz migrate preview
```

## 升级流程

### 方法一：使用升级脚本（推荐）

```bash
# 下载新版本
wget https://github.com/gomailzero/gmz/releases/download/v0.9.1/gmz-linux-amd64 -O /tmp/gmz-new

# 执行升级脚本
./scripts/upgrade.sh v0.9.1 /tmp/gmz-new

# 或指定配置文件
./scripts/upgrade.sh v0.9.1 /tmp/gmz-new --config /etc/gmz/gmz.yml
```

**升级脚本功能**:
1. 备份当前二进制和配置
2. 检查新版本兼容性
3. 执行数据库迁移（如需要）
4. 替换二进制文件
5. 重载配置（热升级）或重启服务（冷升级）
6. 验证服务健康
7. 如有问题，自动回滚

### 方法二：手动升级

#### 热升级步骤

```bash
# 1. 停止接收新连接（graceful shutdown）
systemctl reload gmz

# 2. 备份
./scripts/backup.sh

# 3. 下载新版本
wget https://github.com/gomailzero/gmz/releases/download/v0.9.1/gmz-linux-amd64 -O /tmp/gmz-new
chmod +x /tmp/gmz-new

# 4. 检查新版本
/tmp/gmz-new version
/tmp/gmz-new config validate --config /etc/gmz/gmz.yml

# 5. 执行数据库迁移（如需要）
/tmp/gmz-new migrate up

# 6. 替换二进制
mv /usr/local/bin/gmz /usr/local/bin/gmz.old
mv /tmp/gmz-new /usr/local/bin/gmz

# 7. 重载配置（零中断）
systemctl reload gmz

# 8. 验证服务
systemctl status gmz
./gmz health

# 9. 验证功能
# 发送测试邮件
swaks --server localhost:587 --auth-user test@example.com --auth-password *** --to test@example.com

# 10. 如果一切正常，删除旧二进制
rm /usr/local/bin/gmz.old
```

#### 冷升级步骤

```bash
# 1. 停止服务
systemctl stop gmz

# 2. 备份
./scripts/backup.sh

# 3. 下载新版本
wget https://github.com/gomailzero/gmz/releases/download/v0.9.1/gmz-linux-amd64 -O /tmp/gmz-new
chmod +x /tmp/gmz-new

# 4. 检查新版本
/tmp/gmz-new version
/tmp/gmz-new config validate --config /etc/gmz/gmz.yml

# 5. 执行数据库迁移
/tmp/gmz-new migrate up

# 6. 替换二进制
mv /usr/local/bin/gmz /usr/local/bin/gmz.old
mv /tmp/gmz-new /usr/local/bin/gmz

# 7. 启动服务
systemctl start gmz

# 8. 验证服务
systemctl status gmz
./gmz health

# 9. 验证功能
swaks --server localhost:587 --auth-user test@example.com --auth-password *** --to test@example.com

# 10. 如果一切正常，删除旧二进制
rm /usr/local/bin/gmz.old
```

### 方法三：Docker 升级

```bash
# 1. 备份数据卷
docker exec gmz ./scripts/backup.sh
# 或
docker cp gmz:/var/lib/gmz ./backup-$(date +%Y%m%d)

# 2. 拉取新镜像
docker pull gomailzero/gmz:v0.9.1

# 3. 停止容器
docker stop gmz

# 4. 执行数据库迁移（使用新镜像）
docker run --rm \
  -v gmz-data:/var/lib/gmz \
  -v gmz-config:/etc/gmz \
  gomailzero/gmz:v0.9.1 \
  migrate up

# 5. 启动新容器
docker run -d \
  --name gmz \
  --restart unless-stopped \
  -p 25:25 -p 465:465 -p 587:587 -p 993:993 \
  -v gmz-data:/var/lib/gmz \
  -v gmz-config:/etc/gmz \
  -v gmz-certs:/var/lib/gmz/certs \
  gomailzero/gmz:v0.9.1

# 6. 验证服务
docker logs gmz
docker exec gmz ./gmz health
```

## 数据库迁移

### 自动迁移

gmz 启动时会自动检查并执行待执行的迁移：

```yaml
# gmz.yml
storage:
  auto_migrate: true  # 默认 true
```

### 手动迁移

```bash
# 查看迁移状态
./gmz migrate status

# 执行迁移
./gmz migrate up

# 回滚一个版本
./gmz migrate down

# 回滚到指定版本
./gmz migrate down-to VERSION

# 执行到指定版本
./gmz migrate up-to VERSION

# 查看迁移 SQL（不执行）
./gmz migrate preview
```

### 迁移文件位置

```
migrations/
├── 00001_init.up.sql
├── 00001_init.down.sql
├── 00002_add_quota.up.sql
├── 00002_add_quota.down.sql
└── ...
```

## 配置升级

### 配置热更新

gmz 支持配置热更新，无需重启：

```bash
# 修改配置文件
vim /etc/gmz/gmz.yml

# 重载配置（零中断）
systemctl reload gmz
# 或
kill -HUP $(pgrep gmz)

# 验证配置
./gmz config validate
```

### 配置版本兼容性

如果新版本引入了新的配置项，旧配置仍然兼容，新配置项使用默认值。

如果新版本移除了配置项，会在启动时警告，但不影响运行。

### 配置迁移工具

```bash
# 自动迁移配置到新版本格式
./gmz config migrate --input /etc/gmz/gmz.yml --output /etc/gmz/gmz.yml.new

# 验证新配置
./gmz config validate --config /etc/gmz/gmz.yml.new

# 如果验证通过，替换配置
mv /etc/gmz/gmz.yml.new /etc/gmz/gmz.yml
systemctl reload gmz
```

## 回滚流程

### 自动回滚

升级脚本会在以下情况自动回滚：

1. 新版本启动失败
2. 健康检查失败（30 秒内）
3. 数据库迁移失败

### 手动回滚

```bash
# 1. 停止服务
systemctl stop gmz

# 2. 恢复二进制
mv /usr/local/bin/gmz /usr/local/bin/gmz.failed
mv /usr/local/bin/gmz.old /usr/local/bin/gmz

# 3. 回滚数据库迁移（如需要）
./gmz migrate down

# 4. 恢复配置（如需要）
cp /etc/gmz/gmz.yml.backup.$(date +%Y%m%d) /etc/gmz/gmz.yml

# 5. 启动服务
systemctl start gmz

# 6. 验证服务
systemctl status gmz
./gmz health
```

### 数据回滚

```bash
# 恢复数据备份
./scripts/restore.sh /var/lib/gmz.backup.20241219

# 或手动恢复
systemctl stop gmz
rm -rf /var/lib/gmz
cp -r /var/lib/gmz.backup.20241219 /var/lib/gmz
systemctl start gmz
```

## 版本兼容性

### 主版本号（Major）

主版本号变更（如 v1.0.0 → v2.0.0）可能包含：
- 不兼容的 API 变更
- 数据库 schema 重大变更
- 配置文件格式变更

**升级建议**: 仔细阅读 Release Notes，可能需要手动迁移。

### 次版本号（Minor）

次版本号变更（如 v0.9.0 → v0.10.0）可能包含：
- 新功能
- 数据库 schema 新增（向后兼容）
- 新配置项（可选）

**升级建议**: 通常可以直接升级，建议先测试。

### 修订版本号（Patch）

修订版本号变更（如 v0.9.0 → v0.9.1）通常包含：
- Bug 修复
- 安全补丁
- 性能优化

**升级建议**: 建议尽快升级。

## 升级检查清单

### 升级前

- [ ] 阅读 CHANGELOG.md 和 Release Notes
- [ ] 备份数据和配置
- [ ] 检查磁盘空间（至少 2x 数据大小）
- [ ] 检查新版本系统要求
- [ ] 在测试环境验证升级流程

### 升级中

- [ ] 执行数据库迁移（如需要）
- [ ] 替换二进制文件
- [ ] 验证配置文件兼容性
- [ ] 重载或重启服务

### 升级后

- [ ] 验证服务健康状态
- [ ] 测试 SMTP 发送/接收
- [ ] 测试 IMAP 连接
- [ ] 测试 WebMail 访问
- [ ] 检查日志是否有错误
- [ ] 检查监控指标是否正常
- [ ] 验证证书自动续期（如适用）

## 常见问题

### Q: 升级后服务无法启动？

**A**: 检查以下内容：

1. 查看日志: `journalctl -u gmz -n 100`
2. 验证配置: `./gmz config validate`
3. 检查数据库迁移: `./gmz migrate status`
4. 检查文件权限: `ls -la /var/lib/gmz`
5. 如果问题持续，执行回滚

### Q: 升级后数据库迁移失败？

**A**: 

1. 检查迁移 SQL 语法: `./gmz migrate preview`
2. 检查数据库连接: `./gmz health`
3. 手动执行迁移 SQL（谨慎操作）
4. 如有问题，回滚迁移: `./gmz migrate down`

### Q: 升级后配置不生效？

**A**:

1. 验证配置: `./gmz config validate`
2. 检查配置热更新是否成功: `systemctl status gmz`
3. 如果使用热更新，尝试重启服务: `systemctl restart gmz`

### Q: 如何跳过某个迁移？

**A**: 

**不推荐**，但如必须：

1. 手动修改迁移记录表
2. 或创建空迁移文件占位

**风险**: 可能导致数据不一致，建议联系维护者。

### Q: 升级后性能下降？

**A**:

1. 检查新版本 Release Notes 中的性能变更
2. 对比升级前后的监控指标
3. 检查是否有新的配置项影响性能
4. 如有问题，提交 Issue 或回滚

## 升级脚本示例

### scripts/upgrade.sh

```bash
#!/bin/bash
set -e

VERSION=$1
BINARY_PATH=$2
CONFIG_PATH=${3:-/etc/gmz/gmz.yml}

echo "开始升级 GoMailZero 到 $VERSION"

# 1. 备份
echo "备份当前版本..."
./scripts/backup.sh

# 2. 检查新版本
echo "检查新版本..."
$BINARY_PATH version
$BINARY_PATH config validate --config $CONFIG_PATH

# 3. 检查迁移
echo "检查数据库迁移..."
MIGRATIONS=$($BINARY_PATH migrate status | grep "pending" | wc -l)
if [ $MIGRATIONS -gt 0 ]; then
    echo "发现 $MIGRATIONS 个待执行迁移"
    read -p "是否继续? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 4. 执行迁移
if [ $MIGRATIONS -gt 0 ]; then
    echo "执行数据库迁移..."
    $BINARY_PATH migrate up
fi

# 5. 替换二进制
echo "替换二进制文件..."
mv /usr/local/bin/gmz /usr/local/bin/gmz.old
cp $BINARY_PATH /usr/local/bin/gmz
chmod +x /usr/local/bin/gmz

# 6. 重载服务
echo "重载服务..."
systemctl reload gmz || systemctl restart gmz

# 7. 等待服务就绪
echo "等待服务就绪..."
sleep 5

# 8. 健康检查
echo "健康检查..."
for i in {1..6}; do
    if ./gmz health > /dev/null 2>&1; then
        echo "服务健康"
        break
    fi
    if [ $i -eq 6 ]; then
        echo "健康检查失败，回滚..."
        ./scripts/rollback.sh
        exit 1
    fi
    sleep 5
done

# 9. 验证功能
echo "验证功能..."
# TODO: 添加功能测试

echo "升级完成！"
echo "旧版本备份在: /usr/local/bin/gmz.old"
```

### scripts/rollback.sh

```bash
#!/bin/bash
set -e

echo "开始回滚..."

# 1. 停止服务
systemctl stop gmz

# 2. 恢复二进制
if [ -f /usr/local/bin/gmz.old ]; then
    mv /usr/local/bin/gmz /usr/local/bin/gmz.failed
    mv /usr/local/bin/gmz.old /usr/local/bin/gmz
    echo "二进制已回滚"
else
    echo "错误: 找不到旧版本二进制"
    exit 1
fi

# 3. 回滚数据库迁移（如需要）
read -p "是否回滚数据库迁移? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./gmz migrate down
fi

# 4. 启动服务
systemctl start gmz

# 5. 验证
sleep 3
if ./gmz health > /dev/null 2>&1; then
    echo "回滚成功，服务已恢复"
else
    echo "警告: 服务可能未正常启动，请检查日志"
    journalctl -u gmz -n 50
fi
```

## 联系支持

如遇到升级问题：

1. 查看日志: `journalctl -u gmz -n 100`
2. 查看文档: `README.md`, `UPGRADE.md`
3. 提交 Issue: https://github.com/gomailzero/gmz/issues
4. 社区讨论: https://github.com/gomailzero/gmz/discussions

---

**最后更新**: 2024-12-19  
**版本**: v0.1.0 (升级文档)

