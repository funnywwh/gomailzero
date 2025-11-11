#!/bin/bash
set -e

# GoMailZero 备份脚本
# 用法: ./scripts/backup.sh [备份目录]

BACKUP_DIR=${1:-/var/lib/gmz/backups}
DATA_DIR=/var/lib/gmz
CONFIG_DIR=/etc/gmz
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="gmz_backup_$TIMESTAMP"
BACKUP_PATH="$BACKUP_DIR/$BACKUP_NAME"

echo "=========================================="
echo "GoMailZero 备份脚本"
echo "=========================================="

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo "错误: 请使用 root 权限运行此脚本"
    echo "使用: sudo $0 $@"
    exit 1
fi

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 创建临时备份目录
TEMP_BACKUP=$(mktemp -d)
trap "rm -rf $TEMP_BACKUP" EXIT

echo "开始备份..."

# 备份数据库
if [ -f "$DATA_DIR/data.db" ]; then
    echo "备份数据库..."
    cp "$DATA_DIR/data.db" "$TEMP_BACKUP/data.db"
fi

# 备份邮件目录
if [ -d "$DATA_DIR/mail" ]; then
    echo "备份邮件目录..."
    cp -r "$DATA_DIR/mail" "$TEMP_BACKUP/mail"
fi

# 备份证书
if [ -d "$DATA_DIR/certs" ]; then
    echo "备份证书..."
    cp -r "$DATA_DIR/certs" "$TEMP_BACKUP/certs"
fi

# 备份配置文件
if [ -f "$CONFIG_DIR/gmz.yml" ]; then
    echo "备份配置文件..."
    cp "$CONFIG_DIR/gmz.yml" "$TEMP_BACKUP/gmz.yml"
fi

# 创建压缩包
echo "创建压缩包..."
cd "$TEMP_BACKUP"
tar -czf "$BACKUP_PATH.tar.gz" .

# 计算文件大小
BACKUP_SIZE=$(du -h "$BACKUP_PATH.tar.gz" | cut -f1)

echo ""
echo "=========================================="
echo "备份完成！"
echo "=========================================="
echo ""
echo "备份文件: $BACKUP_PATH.tar.gz"
echo "文件大小: $BACKUP_SIZE"
echo ""
echo "备份内容:"
echo "  - 数据库 (data.db)"
echo "  - 邮件目录 (mail/)"
echo "  - 证书目录 (certs/)"
echo "  - 配置文件 (gmz.yml)"
echo ""

# 清理旧备份（保留最近 7 天）
echo "清理旧备份（保留最近 7 天）..."
find "$BACKUP_DIR" -name "gmz_backup_*.tar.gz" -mtime +7 -delete 2>/dev/null || true

echo "完成！"

