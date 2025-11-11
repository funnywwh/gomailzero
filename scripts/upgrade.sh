#!/bin/bash
set -e

# GoMailZero 升级脚本
# 用法: ./scripts/upgrade.sh [版本] [新二进制路径] [配置文件路径]

VERSION=${1:-latest}
BINARY_PATH=${2:-./bin/gmz}
CONFIG_PATH=${3:-/etc/gmz/gmz.yml}
INSTALL_DIR=/usr/local/bin
BACKUP_DIR=/var/lib/gmz/backups

echo "=========================================="
echo "GoMailZero 升级脚本"
echo "版本: $VERSION"
echo "=========================================="

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo "错误: 请使用 root 权限运行此脚本"
    echo "使用: sudo $0 $@"
    exit 1
fi

# 检查新二进制文件
if [ ! -f "$BINARY_PATH" ]; then
    echo "错误: 新二进制文件不存在: $BINARY_PATH"
    exit 1
fi

# 检查当前版本
if [ -f "$INSTALL_DIR/gmz" ]; then
    CURRENT_VERSION=$($INSTALL_DIR/gmz -version 2>/dev/null || echo "unknown")
    echo "当前版本: $CURRENT_VERSION"
else
    echo "警告: 未找到当前安装的 gmz"
fi

# 创建备份目录
mkdir -p "$BACKUP_DIR"
BACKUP_TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# 备份当前二进制
if [ -f "$INSTALL_DIR/gmz" ]; then
    echo "备份当前二进制..."
    cp "$INSTALL_DIR/gmz" "$BACKUP_DIR/gmz.$BACKUP_TIMESTAMP"
fi

# 备份配置文件
if [ -f "$CONFIG_PATH" ]; then
    echo "备份配置文件..."
    cp "$CONFIG_PATH" "$BACKUP_DIR/gmz.yml.$BACKUP_TIMESTAMP"
fi

# 备份数据
echo "备份数据..."
BACKUP_FILE="$BACKUP_DIR/data.$BACKUP_TIMESTAMP.tar.gz"
tar -czf "$BACKUP_FILE" -C /var/lib/gmz data.db mail 2>/dev/null || true
echo "数据备份: $BACKUP_FILE"

# 检查新版本
echo "检查新版本..."
NEW_VERSION=$($BINARY_PATH -version 2>/dev/null || echo "unknown")
echo "新版本: $NEW_VERSION"

# 验证新二进制
echo "验证新二进制..."
if ! $BINARY_PATH -version > /dev/null 2>&1; then
    echo "错误: 新二进制文件无法运行"
    exit 1
fi

# 检查数据库迁移
echo "检查数据库迁移..."
if [ -f "$CONFIG_PATH" ]; then
    MIGRATIONS=$($BINARY_PATH -migrate status -c "$CONFIG_PATH" 2>/dev/null | grep -c "pending" || echo "0")
    if [ "$MIGRATIONS" -gt 0 ]; then
        echo "发现 $MIGRATIONS 个待执行迁移"
        read -p "是否继续? (y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "升级已取消"
            exit 1
        fi
    fi
fi

# 停止服务
echo "停止服务..."
systemctl stop gmz || true

# 执行数据库迁移（如果启用）
if [ -f "$CONFIG_PATH" ]; then
    echo "执行数据库迁移..."
    $BINARY_PATH -migrate up -c "$CONFIG_PATH" || echo "警告: 迁移失败，请手动检查"
fi

# 替换二进制
echo "替换二进制..."
cp "$BINARY_PATH" "$INSTALL_DIR/gmz"
chmod +x "$INSTALL_DIR/gmz"

# 启动服务
echo "启动服务..."
systemctl start gmz

# 等待服务启动
sleep 2

# 检查服务状态
if systemctl is-active --quiet gmz; then
    echo ""
    echo "=========================================="
    echo "升级成功！"
    echo "=========================================="
    echo ""
    echo "服务状态:"
    systemctl status gmz --no-pager -l
    echo ""
    echo "备份位置: $BACKUP_DIR"
    echo ""
else
    echo ""
    echo "=========================================="
    echo "警告: 服务启动失败"
    echo "=========================================="
    echo ""
    echo "查看日志: journalctl -u gmz -n 50"
    echo ""
    echo "如需回滚，请执行:"
    echo "  systemctl stop gmz"
    echo "  cp $BACKUP_DIR/gmz.$BACKUP_TIMESTAMP $INSTALL_DIR/gmz"
    echo "  systemctl start gmz"
    echo ""
    exit 1
fi

