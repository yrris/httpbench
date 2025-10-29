package benchmark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"httpbench/pkg/config"
)

// TestBenchmarkCreation 测试基准测试器创建
func TestBenchmarkCreation(t *testing.T) {
	cfg := &config.Config{
		Target: config.TargetConfig{
			URL:     "http://example.com",
			Method:  "GET",
			Timeout: 30 * time.Second,
		},
		Load: config.LoadConfig{
			Concurrency: 10,
			Duration:    5 * time.Second,
		},
		Protocol: config.ProtocolConfig{
			KeepAlive: true,
		},
	}

	bench, err := New(cfg)
	if err != nil {
		t.Fatalf("创建基准测试器失败: %v", err)
	}
	defer bench.Close()

	if bench == nil {
		t.Fatal("基准测试器为nil")
	}
}

// TestSimpleRequest 测试简单请求
func TestSimpleRequest(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Target: config.TargetConfig{
			URL:     server.URL,
			Method:  "GET",
			Timeout: 5 * time.Second,
		},
		Load: config.LoadConfig{
			Concurrency:   2,
			Duration:      2 * time.Second,
			TotalRequests: 10,
		},
		Protocol: config.ProtocolConfig{
			KeepAlive: true,
		},
		Validation: config.ValidationConfig{
			StatusCodes: []int{200},
		},
	}

	bench, err := New(cfg)
	if err != nil {
		t.Fatalf("创建基准测试器失败: %v", err)
	}
	defer bench.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := bench.Run(ctx)
	if err != nil {
		t.Fatalf("运行基准测试失败: %v", err)
	}

	if results.TotalRequests == 0 {
		t.Error("没有发送任何请求")
	}

	if results.SuccessRequests != results.TotalRequests {
		t.Errorf("成功请求数不匹配: got %d, want %d", results.SuccessRequests, results.TotalRequests)
	}

	if results.Throughput <= 0 {
		t.Error("吞吐量应该大于0")
	}
}

// TestRateLimiter 测试速率限制
func TestRateLimiter(t *testing.T) {
	rps := 100
	limiter := NewRateLimiter(rps)
	defer limiter.Stop()

	start := time.Now()
	ctx := context.Background()

	// 执行RPS次请求
	for i := 0; i < rps; i++ {
		limiter.Wait(ctx)
	}

	elapsed := time.Since(start)

	// 应该接近1秒
	if elapsed < 900*time.Millisecond || elapsed > 1100*time.Millisecond {
		t.Errorf("速率限制不准确: 期望约1秒, 实际 %v", elapsed)
	}
}

// TestHTTP2Support 测试HTTP/2支持
func TestHTTP2Support(t *testing.T) {
	cfg := &config.Config{
		Target: config.TargetConfig{
			URL:     "https://http2.golang.org",
			Method:  "GET",
			Timeout: 10 * time.Second,
		},
		Load: config.LoadConfig{
			Concurrency:   5,
			Duration:      2 * time.Second,
			TotalRequests: 10,
		},
		Protocol: config.ProtocolConfig{
			HTTP2Enabled: true,
			KeepAlive:    true,
		},
	}

	bench, err := New(cfg)
	if err != nil {
		t.Fatalf("创建HTTP/2基准测试器失败: %v", err)
	}
	defer bench.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	results, err := bench.Run(ctx)
	if err != nil {
		t.Fatalf("运行HTTP/2测试失败: %v", err)
	}

	if results.TotalRequests == 0 {
		t.Error("HTTP/2测试没有发送任何请求")
	}
}

// BenchmarkRequestExecution 基准测试请求执行
func BenchmarkRequestExecution(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	cfg := &config.Config{
		Target: config.TargetConfig{
			URL:     server.URL,
			Method:  "GET",
			Timeout: 5 * time.Second,
		},
		Load: config.LoadConfig{
			Concurrency: 1,
		},
		Protocol: config.ProtocolConfig{
			KeepAlive: true,
		},
	}

	bench, err := New(cfg)
	if err != nil {
		b.Fatalf("创建基准测试器失败: %v", err)
	}
	defer bench.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bench.executeRequest(ctx, 0)
	}
}

// TestConcurrentExecution 测试并发执行
func TestConcurrentExecution(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		time.Sleep(10 * time.Millisecond) // 模拟处理时间
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		Target: config.TargetConfig{
			URL:     server.URL,
			Method:  "GET",
			Timeout: 5 * time.Second,
		},
		Load: config.LoadConfig{
			Concurrency:   10,
			TotalRequests: 100,
		},
		Protocol: config.ProtocolConfig{
			KeepAlive: true,
		},
	}

	bench, err := New(cfg)
	if err != nil {
		t.Fatalf("创建基准测试器失败: %v", err)
	}
	defer bench.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := bench.Run(ctx)
	if err != nil {
		t.Fatalf("运行并发测试失败: %v", err)
	}

	if results.TotalRequests != 100 {
		t.Errorf("请求数不匹配: got %d, want 100", results.TotalRequests)
	}

	// 验证确实是并发执行的(如果串行执行会超过1秒)
	if results.Duration > 2*time.Second {
		t.Errorf("并发执行时间过长: %v", results.Duration)
	}
}
