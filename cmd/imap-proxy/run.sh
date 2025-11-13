#!/bin/bash
# IMAP 透传代理快速启动脚本

# 默认配置
LISTEN_PORT="${LISTEN_PORT:-1993}"
TARGET_ADDR="${TARGET_ADDR:-localhost:993}"
LOG_FILE="${LOG_FILE:-imap-proxy.log}"
VERBOSE="${VERBOSE:-false}"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${SCRIPT_DIR}/../../bin/imap-proxy"

# 如果二进制文件不存在，尝试编译
if [ ! -f "$BINARY" ]; then
    echo "二进制文件不存在，正在编译..."
    cd "$SCRIPT_DIR/../.." || exit 1
    go build -o bin/imap-proxy ./cmd/imap-proxy
    if [ $? -ne 0 ]; then
        echo "编译失败！"
        exit 1
    fi
    echo "编译成功！"
fi

# 构建参数
ARGS="-listen :${LISTEN_PORT} -target ${TARGET_ADDR} -log ${LOG_FILE}"

if [ "$VERBOSE" = "true" ]; then
    ARGS="${ARGS} -v"
fi

# 显示配置信息
echo "=========================================="
echo "IMAP 透传代理启动"
echo "=========================================="
echo "监听地址: :${LISTEN_PORT}"
echo "目标服务器: ${TARGET_ADDR}"
echo "日志文件: ${LOG_FILE}"
echo "详细模式: ${VERBOSE}"
echo "=========================================="
echo ""
echo "在 Foxmail 中配置："
echo "  服务器: localhost"
echo "  端口: ${LISTEN_PORT}"
echo "  加密: SSL/TLS 或 STARTTLS"
echo ""
echo "按 Ctrl+C 停止代理"
echo "=========================================="
echo ""

# 运行代理
exec "$BINARY" $ARGS

