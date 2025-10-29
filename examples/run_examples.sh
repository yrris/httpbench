#!/bin/bash

# HTTP Benchmark Tool - 示例脚本
# 演示各种使用场景

set -e

BINARY="./build/httpbench"
RESULTS_DIR="./results"

# 创建结果目录
mkdir -p $RESULTS_DIR

echo "🚀 HTTP Benchmark Tool - 示例测试"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 1. 简单GET请求测试
echo "📝 示例 1: 简单GET请求测试"
echo "测试目标: httpbin.org"
echo "并发: 10, 持续时间: 5秒"
$BINARY \
  -url https://httpbin.org/get \
  -c 10 \
  -d 5s \
  -output json \
  -report $RESULTS_DIR/example1-simple-get.json

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 2. POST请求测试
echo "📝 示例 2: POST请求测试"
echo "测试目标: httpbin.org/post"
echo "并发: 20, 持续时间: 10秒"

cat > /tmp/post-config.yaml <<EOF
target:
  url: "https://httpbin.org/post"
  method: "POST"
  headers:
    Content-Type: "application/json"

load:
  concurrency: 20
  duration: 10s

request:
  template:
    enabled: true
  dynamic_body: true
  body_template: |
    {
      "request_id": "{{random_uuid}}",
      "timestamp": {{timestamp}},
      "user_id": {{random_int 1 1000}},
      "message": "{{random_string 50}}"
    }

output:
  format: "json"
  report_file: "$RESULTS_DIR/example2-post-request.json"
EOF

$BINARY -config /tmp/post-config.yaml

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 3. HTTP/2测试
echo "📝 示例 3: HTTP/2协议测试"
echo "测试目标: httpbin.org"
echo "并发: 50, 持续时间: 15秒"
$BINARY \
  -url https://httpbin.org/get \
  -c 50 \
  -d 15s \
  -http2 \
  -output csv \
  -report $RESULTS_DIR/example3-http2.csv

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 4. 速率限制测试
echo "📝 示例 4: 速率限制测试"
echo "测试目标: httpbin.org"
echo "并发: 100, RPS限制: 500"
$BINARY \
  -url https://httpbin.org/get \
  -c 100 \
  -d 20s \
  -rps 500 \
  -output json \
  -report $RESULTS_DIR/example4-rate-limit.json

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 5. 渐进式负载测试
echo "📝 示例 5: 渐进式负载测试"
echo "测试目标: httpbin.org"
echo "并发: 10 -> 100, 步长: 10"

cat > /tmp/rampup-config.yaml <<EOF
target:
  url: "https://httpbin.org/get"
  method: "GET"

load:
  load_pattern: "ramp_up"
  
  ramp_up:
    enabled: true
    start_concurrency: 10
    end_concurrency: 100
    duration: 30s
    steps: 10

output:
  format: "json"
  report_file: "$RESULTS_DIR/example5-ramp-up.json"
  realtime_monitor: true
EOF

$BINARY -config /tmp/rampup-config.yaml

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 6. 突发负载测试
echo "📝 示例 6: 突发负载测试"
echo "测试目标: httpbin.org"
echo "基准并发: 50, 突发并发: 200"

cat > /tmp/burst-config.yaml <<EOF
target:
  url: "https://httpbin.org/get"
  method: "GET"

load:
  load_pattern: "burst"
  
  burst_mode:
    enabled: true
    base_concurrency: 50
    burst_concurrency: 200
    burst_duration: 5s
    burst_interval: 15s

output:
  format: "json"
  report_file: "$RESULTS_DIR/example6-burst.json"
  realtime_monitor: true
EOF

$BINARY -config /tmp/burst-config.yaml

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 7. 响应验证测试
echo "📝 示例 7: 响应验证测试"
echo "测试目标: httpbin.org/json"

cat > /tmp/validation-config.yaml <<EOF
target:
  url: "https://httpbin.org/json"
  method: "GET"

load:
  concurrency: 30
  duration: 10s

validation:
  status_codes: [200]
  response_time_max: 2s
  content_patterns:
    - '"slideshow"'
    - '"title"'
  body_validation:
    min_size: 100

output:
  format: "json"
  report_file: "$RESULTS_DIR/example7-validation.json"
EOF

$BINARY -config /tmp/validation-config.yaml

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 8. 压力测试
echo "📝 示例 8: 压力测试"
echo "测试目标: httpbin.org"
echo "并发: 500, 持续时间: 60秒"
$BINARY \
  -url https://httpbin.org/get \
  -c 500 \
  -d 60s \
  -output json \
  -report $RESULTS_DIR/example8-stress-test.json

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 清理临时文件
rm -f /tmp/*-config.yaml

echo "✅ 所有示例测试完成!"
echo ""
echo "📊 测试报告保存在: $RESULTS_DIR/"
ls -lh $RESULTS_DIR/

echo ""
echo "💡 提示:"
echo "  - 查看JSON报告: cat $RESULTS_DIR/example1-simple-get.json | jq"
echo "  - 查看CSV报告: column -t -s, $RESULTS_DIR/example3-http2.csv"
echo "  - 比较不同测试: 使用您喜欢的数据分析工具"
