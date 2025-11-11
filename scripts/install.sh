#!/bin/bash
set -e

# GoMailZero 安装脚本
# 用法: ./scripts/install.sh [二进制路径] [配置文件路径]

BINARY_PATH=${1:-./bin/gmz}
CONFIG_PATH=${2:-/etc/gmz/gmz.yml}
INSTALL_DIR=/usr/local/bin
SERVICE_DIR=/etc/systemd/system
DATA_DIR=/var/lib/gmz
CONFIG_DIR=/etc/gmz

echo "=========================================="
echo "GoMailZero 安装脚本"
echo "=========================================="

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo "错误: 请使用 root 权限运行此脚本"
    echo "使用: sudo $0 $@"
    exit 1
fi

# 检查二进制文件
if [ ! -f "$BINARY_PATH" ]; then
    echo "错误: 二进制文件不存在: $BINARY_PATH"
    echo "请先构建二进制: make build"
    exit 1
fi

# 检查二进制是否可执行
if [ ! -x "$BINARY_PATH" ]; then
    chmod +x "$BINARY_PATH"
fi

# 创建目录
echo "创建目录..."
mkdir -p "$DATA_DIR"
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR/mail"
mkdir -p "$DATA_DIR/certs"

# 复制二进制文件
echo "安装二进制文件..."
cp "$BINARY_PATH" "$INSTALL_DIR/gmz"
chmod +x "$INSTALL_DIR/gmz"

# 创建配置文件（如果不存在）
if [ ! -f "$CONFIG_PATH" ]; then
    echo "创建配置文件..."
    cat > "$CONFIG_PATH" <<EOF
node_id: mx1
domain: example.com

storage:
  driver: sqlite
  dsn: $DATA_DIR/data.db
  maildir_root: $DATA_DIR/mail
  auto_migrate: true

smtp:
  enabled: true
  ports: [25, 465, 587]
  hostname: ""
  max_size: 50MB

imap:
  enabled: true
  port: 993

tls:
  enabled: true
  min_version: "1.3"
  acme:
    enabled: true
    provider: letsencrypt
    email: admin@example.com
    dir: $DATA_DIR/certs

webmail:
  enabled: true
  port: 8080

admin:
  port: 8081
  api_key: "$(openssl rand -hex 32)"
  jwt_secret: "$(openssl rand -hex 32)"

log:
  level: info
  format: json
  output: stdout

metrics:
  enabled: true
  port: 9090
EOF
    echo "配置文件已创建: $CONFIG_PATH"
    echo "请编辑配置文件并设置正确的域名和邮箱地址"
else
    echo "使用现有配置文件: $CONFIG_PATH"
fi

# 创建 systemd 服务文件
echo "创建 systemd 服务..."
cat > "$SERVICE_DIR/gmz.service" <<EOF
[Unit]
Description=GoMailZero Email Server
After=network.target

[Service]
Type=simple
User=root
ExecStart=$INSTALL_DIR/gmz -c $CONFIG_PATH
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR $CONFIG_DIR

[Install]
WantedBy=multi-user.target
EOF

# 设置权限
echo "设置权限..."
chown -R root:root "$DATA_DIR"
chmod 755 "$DATA_DIR"
chmod 600 "$CONFIG_PATH" 2>/dev/null || chmod 644 "$CONFIG_PATH"

# 重载 systemd
echo "重载 systemd..."
systemctl daemon-reload

# 启用服务
echo "启用服务..."
systemctl enable gmz

echo ""
echo "=========================================="
echo "安装完成！"
echo "=========================================="
echo ""
echo "下一步："
echo "1. 编辑配置文件: $CONFIG_PATH"
echo "2. 启动服务: systemctl start gmz"
echo "3. 查看状态: systemctl status gmz"
echo "4. 查看日志: journalctl -u gmz -f"
echo ""

