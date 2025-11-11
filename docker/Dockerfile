# 多阶段构建 Dockerfile for GoMailZero

# 构建阶段
FROM golang:1.24-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git make

# 设置工作目录
WORKDIR /build

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖（先 tidy 确保依赖正确）
RUN go mod tidy && go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-w -s -X main.Version=${VERSION:-dev} -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o gmz ./cmd/gmz

# 运行阶段
# 使用 distroless 镜像以减小体积和提高安全性
FROM gcr.io/distroless/static:nonroot

# 或者使用 alpine（如果需要 shell 等工具）
# FROM alpine:latest
# RUN apk add --no-cache ca-certificates tzdata && \
#     update-ca-certificates

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/gmz /app/gmz

# 复制配置示例
COPY --from=builder /build/configs/gmz.yml.example /app/configs/gmz.yml.example

# distroless 镜像已经使用 nonroot 用户（uid:gid = 65532:65532）
# 不需要创建用户或设置权限

# 暴露端口
EXPOSE 25 465 587 993 8080 9090

# 健康检查（如果管理 API 启用）
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 设置入口点
ENTRYPOINT ["/app/gmz"]

# 默认命令
CMD ["-c", "/app/configs/gmz.yml.example"]

