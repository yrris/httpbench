package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"httpbench/pkg/benchmark"
	"httpbench/pkg/config"
	"httpbench/pkg/reporter"
)

var (
	configFile   = flag.String("config", "config.yaml", "配置文件路径")
	url          = flag.String("url", "", "目标URL")
	concurrency  = flag.Int("c", 10, "并发数")
	duration     = flag.Duration("d", 10*time.Second, "测试持续时间")
	requests     = flag.Int("n", 0, "总请求数(0表示基于时间)")
	rps          = flag.Int("rps", 0, "每秒请求数限制(0表示无限制)")
	http2        = flag.Bool("http2", false, "启用HTTP/2")
	http3        = flag.Bool("http3", false, "启用HTTP/3 (QUIC)")
	outputFormat = flag.String("output", "console", "输出格式: console, json, csv")
	reportFile   = flag.String("report", "", "报告输出文件")
	distributed  = flag.Bool("distributed", false, "分布式模式")
	masterAddr   = flag.String("master", "", "主节点地址(分布式模式)")
	workerMode   = flag.Bool("worker", false, "作为工作节点运行")
)

func main() {
	flag.Parse()

	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	// 创建上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("收到中断信号,正在优雅退出...")
		cancel()
	}()

	// 运行基准测试
	if err := runBenchmark(ctx, cfg); err != nil {
		log.Fatalf("基准测试执行失败: %v", err)
	}
}

func loadConfig() (*config.Config, error) {
	var cfg *config.Config
	var err error

	if *configFile != "" && fileExists(*configFile) {
		cfg, err = config.LoadFromFile(*configFile)
		if err != nil {
			return nil, err
		}
	} else {
		cfg = config.NewDefault()
	}

	// 命令行参数覆盖配置文件
	if *url != "" {
		cfg.Target.URL = *url
	}
	if *concurrency > 0 {
		cfg.Load.Concurrency = *concurrency
	}
	if *duration > 0 {
		cfg.Load.Duration = *duration
	}
	if *requests > 0 {
		cfg.Load.TotalRequests = *requests
	}
	if *rps > 0 {
		cfg.Load.RateLimit = *rps
	}
	if *http2 {
		cfg.Protocol.HTTP2Enabled = true
	}
	if *http3 {
		cfg.Protocol.HTTP3Enabled = true
	}
	if *outputFormat != "" {
		cfg.Output.Format = *outputFormat
	}
	if *reportFile != "" {
		cfg.Output.ReportFile = *reportFile
	}
	if *distributed {
		cfg.Distributed.Enabled = true
	}
	if *masterAddr != "" {
		cfg.Distributed.MasterAddress = *masterAddr
	}
	if *workerMode {
		cfg.Distributed.WorkerMode = true
	}

	return cfg, nil
}

func runBenchmark(ctx context.Context, cfg *config.Config) error {
	fmt.Printf("🚀 HTTP 基准测试工具 v1.0\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("目标: %s\n", cfg.Target.URL)
	fmt.Printf("并发: %d\n", cfg.Load.Concurrency)
	fmt.Printf("持续时间: %v\n", cfg.Load.Duration)
	if cfg.Protocol.HTTP2Enabled {
		fmt.Printf("协议: HTTP/2\n")
	} else if cfg.Protocol.HTTP3Enabled {
		fmt.Printf("协议: HTTP/3 (QUIC)\n")
	} else {
		fmt.Printf("协议: HTTP/1.1\n")
	}
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// 创建基准测试执行器
	bench, err := benchmark.New(cfg)
	if err != nil {
		return fmt.Errorf("创建基准测试器失败: %w", err)
	}
	defer bench.Close()

	// 执行测试
	fmt.Println("⏳ 开始测试...")
	startTime := time.Now()

	results, err := bench.Run(ctx)
	if err != nil {
		return fmt.Errorf("执行测试失败: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("\n✅ 测试完成 (耗时: %v)\n\n", duration)

	// 生成报告
	rep := reporter.New(cfg.Output.Format)
	if err := rep.Generate(results, cfg.Output.ReportFile); err != nil {
		return fmt.Errorf("生成报告失败: %w", err)
	}

	// 控制台输出摘要
	if cfg.Output.Format == "console" || cfg.Output.ReportFile != "" {
		printSummary(results)
	}

	return nil
}

func printSummary(results *benchmark.Results) {
	fmt.Printf("📊 测试结果摘要\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("总请求数:     %d\n", results.TotalRequests)
	fmt.Printf("成功请求:     %d\n", results.SuccessRequests)
	fmt.Printf("失败请求:     %d\n", results.FailedRequests)

	// 防止除以零
	successRate := 0.0
	if results.TotalRequests > 0 {
		successRate = float64(results.SuccessRequests) / float64(results.TotalRequests) * 100
	}
	fmt.Printf("成功率:       %.2f%%\n", successRate)
	fmt.Printf("\n")
	fmt.Printf("📈 性能指标\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("吞吐量:       %.2f req/s\n", results.Throughput)
	fmt.Printf("平均延迟:     %v\n", results.Latency.Mean)
	fmt.Printf("P50延迟:      %v\n", results.Latency.P50)
	fmt.Printf("P90延迟:      %v\n", results.Latency.P90)
	fmt.Printf("P95延迟:      %v\n", results.Latency.P95)
	fmt.Printf("P99延迟:      %v\n", results.Latency.P99)
	fmt.Printf("最小延迟:     %v\n", results.Latency.Min)
	fmt.Printf("最大延迟:     %v\n", results.Latency.Max)
	fmt.Printf("\n")
	fmt.Printf("📦 数据传输\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("总接收:       %s\n", formatBytes(results.BytesReceived))
	fmt.Printf("总发送:       %s\n", formatBytes(results.BytesSent))

	// 防止除以零
	receiveRate := int64(0)
	if results.Duration.Seconds() > 0 {
		receiveRate = int64(float64(results.BytesReceived) / results.Duration.Seconds())
	}
	fmt.Printf("接收速率:     %s/s\n", formatBytes(receiveRate))

	if len(results.ErrorsByType) > 0 {
		fmt.Printf("\n")
		fmt.Printf("❌ 错误统计\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		for errType, count := range results.ErrorsByType {
			fmt.Printf("%-20s: %d\n", errType, count)
		}
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
