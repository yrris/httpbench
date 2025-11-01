# HTTP Benchmark Tool

🚀 一个高性能 Go 语言 HTTP 基准测试工具,支持 HTTP/1.1、HTTP/2 和 HTTP/3 协议

## ✨ 核心特性

### 1. 并发请求生成

- ✅ 可配置线程数控制并发级别
- ✅ 支持动态调整并发梯度
- ✅ 智能速率限制

### 2. 协议支持

- ✅ 完整支持 HTTP/1.1 协议栈
- ✅ 原生集成 HTTP/2 多路复用
- ✅ 实现 HTTP/3 QUIC 传输层

### 3. 请求配置

- ✅ 自定义请求头管理模块
- ✅ Cookie 会话持久化支持
- ✅ 动态内容模板引擎
  - 变量插值
  - 循环构造
  - 丰富的内置函数

### 4. 响应验证

- ✅ 状态码校验器
- ✅ 正则表达式内容匹配
- ✅ 响应时间阈值告警

### 5. 安全配置

- ✅ 完整 TLS 参数套件
  - 证书链验证
  - 双向认证支持
  - 协议版本控制
- ✅ 自定义 CA 信任库

### 6. 负载模式

- ✅ 渐进式压力爬升阶段
- ✅ 稳态持续负载阶段
- ✅ 突发流量模式模拟

### 7. 统计指标

- ✅ 毫秒级延迟统计
  - P50/P90/P99 分位值
  - HDR 直方图分布
- ✅ 实时吞吐量监控
- ✅ 错误率分类统计
  - 网络错误
  - 业务错误
  - 验证失败
    
<!--
### 8. 高级特性

- ✅ 分布式测试模式
  - 集群节点协同
  - 结果聚合
- ✅ CSV/JSON/HTML 报告导出
- ✅ 实时监控仪表盘
-->

## 📦 安装

### 前置要求

- Go 1.21+
- Git

### 编译安装

```bash
# 克隆仓库
git clone https://github.com/yrris/httpbench.git
cd httpbench

# 下载依赖
go mod download

# 编译
go build -o httpbench main.go

# 安装到系统
go install
```

## 🚀 快速开始

### 基础用法

```bash
# 简单测试
httpbench -url https://api.example.com -c 10 -d 30s

# 指定请求数
httpbench -url https://api.example.com -n 10000 -c 100

# 启用HTTP/2
httpbench -url https://api.example.com -c 50 -http2

# 启用HTTP/3
httpbench -url https://api.example.com -c 50 -http3

# 速率限制
httpbench -url https://api.example.com -c 100 -rps 1000
```

### 使用配置文件

```bash
# 使用默认配置文件 (config.yaml)
httpbench

# 指定配置文件
httpbench -config custom-config.yaml

# 导出JSON报告
httpbench -config config.yaml -output json -report report.json

# 导出CSV报告
httpbench -config config.yaml -output csv -report report.csv
```

## 📖 详细使用

### 命令行参数

| 参数           | 类型     | 默认值      | 说明                         |
| -------------- | -------- | ----------- | ---------------------------- |
| `-url`         | string   | -           | 目标 URL                     |
| `-c`           | int      | 10          | 并发数                       |
| `-d`           | duration | 10s         | 测试持续时间                 |
| `-n`           | int      | 0           | 总请求数(0 表示基于时间)     |
| `-rps`         | int      | 0           | 每秒请求数限制(0 表示无限制) |
| `-http2`       | bool     | false       | 启用 HTTP/2                  |
| `-http3`       | bool     | false       | 启用 HTTP/3                  |
| `-output`      | string   | console     | 输出格式: console, json, csv |
| `-report`      | string   | -           | 报告输出文件                 |
| `-config`      | string   | config.yaml | 配置文件路径                 |
| `-distributed` | bool     | false       | 分布式模式                   |
| `-master`      | string   | -           | 主节点地址                   |
| `-worker`      | bool     | false       | 作为工作节点运行             |

### 配置文件示例

```yaml
target:
  url: "https://api.example.com/v1/users"
  method: "POST"
  timeout: 30s
  headers:
    Content-Type: "application/json"
    Authorization: "Bearer your-token"

load:
  concurrency: 100
  duration: 60s
  load_pattern: "ramp_up"

  ramp_up:
    enabled: true
    start_concurrency: 10
    end_concurrency: 100
    duration: 30s
    steps: 10

request:
  template:
    enabled: true
  dynamic_body: true
  body_template: |
    {
      "user_id": {{worker_id}},
      "timestamp": {{timestamp}},
      "email": "user-{{random_int 1 10000}}@example.com",
      "name": "{{random_string 10}}"
    }

validation:
  status_codes: [200, 201]
  response_time_max: 2s
  content_patterns:
    - '"status":"success"'

output:
  format: "json"
  report_file: "benchmark-report.json"
  realtime_monitor: true
```

## 🎯 使用场景

### 1. API 性能测试

```bash
httpbench \
  -url https://api.example.com/v1/users \
  -c 100 \
  -d 60s \
  -output json \
  -report api-benchmark.json
```

### 2. 压力测试

```yaml
load:
  concurrency: 500
  duration: 300s # 5分钟
  rate_limit: 5000
```

### 3. 渐进式负载测试

```yaml
load:
  load_pattern: "ramp_up"
  ramp_up:
    enabled: true
    start_concurrency: 50
    end_concurrency: 500
    duration: 120s
    steps: 10
```

### 4. 突发流量测试

```yaml
load:
  load_pattern: "burst"
  burst_mode:
    enabled: true
    base_concurrency: 100
    burst_concurrency: 1000
    burst_duration: 10s
    burst_interval: 30s
```
<!--
### 5. 分布式测试

```bash
# 启动工作节点
httpbench -worker -config worker-config.yaml

# 运行主节点
httpbench -distributed -config master-config.yaml
```
-->

## 📊 报告格式

### Console 输出

```
🚀 HTTP 基准测试工具 v1.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
目标: https://api.example.com
并发: 100
持续时间: 60s
协议: HTTP/2
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

⏱️  实时监控已启动
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
时间       RPS        平均延迟          P99延迟          错误率
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
10s        1523.45    65.3ms           156.2ms         0.12%
20s        1547.82    63.8ms           152.1ms         0.10%
...

📊 测试结果摘要
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
总请求数:     90000
成功请求:     89890
失败请求:     110
成功率:       99.88%

📈 性能指标
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
吞吐量:       1500.00 req/s
平均延迟:     64.2ms
P50延迟:      58.3ms
P90延迟:      98.5ms
P95延迟:      125.6ms
P99延迟:      154.2ms
最小延迟:     12.3ms
最大延迟:     523.1ms
```

### JSON 报告

```json
{
  "summary": {
    "total_requests": 90000,
    "success_requests": 89890,
    "failed_requests": 110,
    "success_rate": 99.88,
    "duration_seconds": 60.02,
    "throughput_rps": 1500.0
  },
  "latency": {
    "min_ms": 12,
    "max_ms": 523,
    "mean_ms": 64,
    "p50_ms": 58,
    "p90_ms": 98,
    "p95_ms": 125,
    "p99_ms": 154
  }
}
```
<!--

## 🔧 模板函数

工具提供丰富的模板函数用于动态内容生成:

### 随机函数

- `random_int min max` - 生成随机整数
- `random_string length` - 生成随机字符串
- `random_uuid` - 生成 UUID

### 时间函数

- `timestamp` - Unix 时间戳(秒)
- `timestamp_ms` - Unix 时间戳(毫秒)
- `now` - 当前时间(RFC3339)
- `date format` - 格式化日期

### 字符串函数

- `upper s` - 转大写
- `lower s` - 转小写
- `trim s` - 去除空白
- `replace s old new` - 替换字符串

### 数学函数

- `add a b` - 加法
- `sub a b` - 减法
- `mul a b` - 乘法
- `div a b` - 除法
-->

<!--
## 🤝 贡献

欢迎提交 Issue 和 Pull Request!
-->

## 📄 许可证

MIT License

## 🙏 致谢

- [HDR Histogram](https://github.com/HdrHistogram/hdrhistogram-go) - 高精度延迟统计
- [quic-go](https://github.com/quic-go/quic-go) - HTTP/3 支持
- [golang.org/x/net/http2](https://pkg.go.dev/golang.org/x/net/http2) - HTTP/2 支持
