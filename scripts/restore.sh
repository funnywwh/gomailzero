#!/bin/bash
set -e

# GoMailZero 恢复脚本
# 用法: ./scripts/restore.sh [备份文件路径]

BACKUP_FILE=${1}
DATA_DIR=/var/lib/gmz
CONFIG_DIR=/etc/gmz

echo "=========================================="
echo "GoMailZero 恢复脚本"
echo "=========================================="

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo "错误: 请使用 root 权限运行此脚本"
    echo "使用: sudo $0 $@"
    exit 1
fi

# 检查备份文件
if [ -z "$BACKUP_FILE" ]; then
    echo "错误: 请指定备份文件路径"
    echo "用法: $0 <备份文件路径>"
    echo ""
    echo "可用的备份文件:"
    ls -lh /var/lib/gmz/backups/*.tar.gz 2>/dev/null || echo "  无"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "错误: 备份文件不存在: $BACKUP_FILE"
    exit 1
fi

# 确认恢复
echo "警告: 此操作将覆盖当前数据！"
echo "备份文件: $BACKUP_FILE"
read -p "确定要继续吗? (yes/no) " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "恢复已取消"
    exit 1
fi

# 停止服务
echo "停止服务..."
systemctl stop gmz || true

# 创建临时目录
TEMP_RESTORE=$(mktemp -d)
trap "rm -rf $TEMP_RESTORE" EXIT

# 解压备份
echo "解压备份..."
tar -xzf "$BACKUP_FILE" -C "$TEMP_RESTORE"

# 备份当前数据（以防万一）
CURRENT_BACKUP="$DATA_DIR/backups/pre_restore_$(date +%Y%m%d_%H%M%S).tar.gz"
mkdir -p "$DATA_DIR/backups"
if [ -f "$DATA_DIR/data.db" ] || [ -d "$DATA_DIR/mail" ]; then
    echo "备份当前数据..."
    tar -czf "$CURRENT_BACKUP" -C "$DATA_DIR" data.db mail 2>/dev/null || true
fi

# 恢复数据库
if [ -f "$TEMP_RESTORE/data.db" ]; then
    echo "恢复数据库..."
    cp "$TEMP_RESTORE/data.db" "$DATA_DIR/data.db"
    chmod 600 "$DATA_DIR/data.db"
fi

# 恢复邮件目录
if [ -d "$TEMP_RESTORE/mail" ]; then
    echo "恢复邮件目录..."
    rm -rf "$DATA_DIR/mail"
    cp -r "$TEMP_RESTORE/mail" "$DATA_DIR/mail"
    chmod -R 700 "$DATA_DIR/mail"
fi

# 恢复证书
if [ -d "$TEMP_RESTORE/certs" ]; then
    echo "恢复证书..."
    rm -rf "$DATA_DIR/certs"
    cp -r "$TEMP_RESTORE/certs" "$DATA_DIR/certs"
    chmod -R 600 "$DATA_DIR/certs"
fi

# 恢复配置文件（可选）
if [ -f "$TEMP_RESTORE/gmz.yml" ]; then
    read -p "是否恢复配置文件? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "恢复配置文件..."
        cp "$TEMP_RESTORE/gmz.yml" "$CONFIG_DIR/gmz.yml"
        chmod 600 "$CONFIG_DIR/gmz.yml"
    fi
fi

# 设置权限
echo "设置权限..."
chown -R root:root "$DATA_DIR"
chmod 755 "$DATA_DIR"

# 启动服务
echo "启动服务..."
systemctl start gmz

# 等待服务启动
sleep 2

# 检查服务状态
if systemctl is-active --quiet gmz; then
    echo ""
    echo "=========================================="
    echo "恢复成功！"
    echo "=========================================="
    echo ""
    echo "当前数据备份: $CURRENT_BACKUP"
    echo ""
    echo "服务状态:"
    systemctl status gmz --no-pager -l
    echo ""
else
    echo ""
    echo "=========================================="
    echo "警告: 服务启动失败"
    echo "=========================================="
    echo ""
    echo "查看日志: journalctl -u gmz -n 50"
    echo ""
    exit 1
fi

