.PHONY: build test clean install docker run help

# 变量
BINARY_NAME=gmz
VERSION?=v0.9.0
BUILD_DIR=bin
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_TEST=$(GO_CMD) test
GO_LINT=golangci-lint

# 构建标志
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

help: ## 显示帮助信息
	@echo "可用目标:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: build-frontend ## 构建二进制文件
	@echo "构建 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gmz

build-frontend: ## 构建前端（WebMail 和管理界面）
	@echo "构建 WebMail 前端..."
	@cd webmail && npm install && npm run build
	@echo "构建管理界面前端..."
	@cd admin && npm install && npm run build

build-linux-amd64: ## 构建 Linux x86_64 二进制
	@echo "构建 Linux x86_64 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/gmz

build-linux-arm64: ## 构建 Linux arm64 二进制
	@echo "构建 Linux arm64 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/gmz

build-all: build-linux-amd64 build-linux-arm64 ## 构建所有架构

test: ## 运行单元测试
	@echo "运行单元测试..."
	$(GO_TEST) -v -cover ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "运行测试并生成覆盖率报告..."
	$(GO_TEST) -v -coverprofile=coverage.out ./...
	$(GO_CMD) tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

test-integration: ## 运行集成测试
	@echo "运行集成测试..."
	$(GO_TEST) -v -tags=integration ./test/integration/...

lint: ## 运行代码检查
	@echo "运行代码检查..."
	@if command -v $(GO_LINT) > /dev/null; then \
		$(GO_LINT) run ./...; \
	else \
		echo "golangci-lint 未安装，跳过检查"; \
	fi

fmt: ## 格式化代码
	@echo "格式化代码..."
	$(GO_CMD) fmt ./...
	@if command -v goimports > /dev/null; then \
		goimports -w .; \
	fi

clean: ## 清理构建文件
	@echo "清理构建文件..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

deps: ## 下载依赖
	@echo "下载依赖..."
	$(GO_CMD) mod download
	$(GO_CMD) mod tidy

deps-update: ## 更新依赖
	@echo "更新依赖..."
	$(GO_CMD) get -u ./...
	$(GO_CMD) mod tidy

install: build ## 安装到系统
	@echo "安装 $(BINARY_NAME)..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

docker-build: ## 构建 Docker 镜像
	@echo "构建 Docker 镜像..."
	docker build -t gomailzero/gmz:$(VERSION) -f docker/Dockerfile .
	docker tag gomailzero/gmz:$(VERSION) gomailzero/gmz:latest

docker-run: ## 运行 Docker 容器
	@echo "运行 Docker 容器..."
	docker-compose up -d

run: build ## 构建并运行
	@echo "运行 $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) -c configs/gmz.yml.example

migrate-up: ## 执行数据库迁移（向上）
	@echo "执行数据库迁移..."
	$(GO_CMD) run ./cmd/gmz migrate up

migrate-down: ## 回滚数据库迁移
	@echo "回滚数据库迁移..."
	$(GO_CMD) run ./cmd/gmz migrate down

migrate-status: ## 查看迁移状态
	@echo "查看迁移状态..."
	$(GO_CMD) run ./cmd/gmz migrate status

security: ## 运行安全扫描
	@echo "运行安全扫描..."
	@if command -v gosec > /dev/null; then \
		gosec ./...; \
	else \
		echo "gosec 未安装，跳过安全扫描"; \
	fi

ci: deps fmt lint test test-coverage security ## CI 完整流程

