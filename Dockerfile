# 多阶段构建
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o httpbench \
    main.go

# 运行阶段
FROM alpine:latest

# 安装运行依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1000 httpbench && \
    adduser -D -u 1000 -G httpbench httpbench

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/httpbench .
COPY --from=builder /build/config.yaml .

# 设置权限
RUN chown -R httpbench:httpbench /app

# 切换到非root用户
USER httpbench

# 暴露gRPC端口(用于分布式模式)
EXPOSE 50051

# 入口点
ENTRYPOINT ["./httpbench"]

# 默认命令
CMD ["-config", "config.yaml"]
