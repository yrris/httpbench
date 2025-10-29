.PHONY: all build clean test install run help

# 变量定义
BINARY_NAME=httpbench
VERSION=1.0.0
BUILD_DIR=build
GO_FILES=$(shell find . -name '*.go' -type f)

# 编译标志
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"
GCFLAGS=-gcflags "all=-trimpath=$(PWD)"
ASMFLAGS=-asmflags "all=-trimpath=$(PWD)"

all: clean build

help: ## 显示帮助信息
	@echo "HTTP Benchmark Tool - Makefile"
	@echo ""
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 编译项目
	@echo "编译 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) $(GCFLAGS) $(ASMFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "编译完成: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: ## 编译所有平台
	@echo "编译多平台版本..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	
	@echo "多平台编译完成!"

clean: ## 清理编译产物
	@echo "清理编译产物..."
	@rm -rf $(BUILD_DIR)
	@rm -f *.log *.csv *.json *.html
	@echo "清理完成"

test: ## 运行测试
	@echo "运行测试..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "测试完成"

test-benchmark: ## 运行性能测试
	@echo "运行性能测试..."
	go test -bench=. -benchmem ./...

install: build ## 安装到系统
	@echo "安装 $(BINARY_NAME)..."
	go install $(LDFLAGS)
	@echo "安装完成"

run: build ## 编译并运行
	@echo "运行 $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) -url https://httpbin.org/get -c 10 -d 5s

run-example: build ## 运行示例测试
	@echo "运行示例测试..."
	./$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml

run-http2: build ## 运行HTTP/2测试
	@echo "运行HTTP/2测试..."
	./$(BUILD_DIR)/$(BINARY_NAME) -url https://httpbin.org/get -c 50 -d 10s -http2

deps: ## 下载依赖
	@echo "下载依赖..."
	go mod download
	go mod tidy
	@echo "依赖下载完成"

fmt: ## 格式化代码
	@echo "格式化代码..."
	go fmt ./...
	@echo "格式化完成"

lint: ## 代码检查
	@echo "运行代码检查..."
	@which golangci-lint > /dev/null || (echo "请先安装 golangci-lint"; exit 1)
	golangci-lint run ./...
	@echo "代码检查完成"

vet: ## 代码静态分析
	@echo "运行静态分析..."
	go vet ./...
	@echo "静态分析完成"

docker-build: ## 构建Docker镜像
	@echo "构建Docker镜像..."
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "Docker镜像构建完成"

docker-run: docker-build ## 运行Docker容器
	@echo "运行Docker容器..."
	docker run --rm $(BINARY_NAME):$(VERSION) -url https://httpbin.org/get -c 10 -d 5s

benchmark-simple: build ## 简单基准测试
	@echo "执行简单基准测试..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
		-url https://httpbin.org/get \
		-c 50 \
		-d 30s \
		-output json \
		-report benchmark-simple.json

benchmark-stress: build ## 压力测试
	@echo "执行压力测试..."
	./$(BUILD_DIR)/$(BINARY_NAME) \
		-url https://httpbin.org/post \
		-c 200 \
		-d 60s \
		-rps 2000 \
		-output csv \
		-report benchmark-stress.csv

version: ## 显示版本信息
	@echo "$(BINARY_NAME) version $(VERSION)"

info: ## 显示构建信息
	@echo "Binary Name: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Dir: $(BUILD_DIR)"
	@echo "Go Version: $(shell go version)"
	@echo "OS: $(shell go env GOOS)"
	@echo "Arch: $(shell go env GOARCH)"

release: clean build-all ## 创建发布包
	@echo "创建发布包..."
	@mkdir -p $(BUILD_DIR)/release
	@cd $(BUILD_DIR) && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64 && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64 && \
		zip release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "发布包创建完成: $(BUILD_DIR)/release/"

watch: ## 监控文件变化并自动编译
	@which fswatch > /dev/null || (echo "请先安装 fswatch"; exit 1)
	@echo "监控文件变化..."
	@fswatch -o $(GO_FILES) | xargs -n1 -I{} make build
