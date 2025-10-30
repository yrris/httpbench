package benchmark

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http2"

	"httpbench/pkg/config"
	"httpbench/pkg/stats"
	"httpbench/pkg/template"
	"httpbench/pkg/validator"
)

// Benchmark 基准测试器
type Benchmark struct {
	config    *config.Config
	client    *http.Client
	stats     *stats.Collector
	validator *validator.Validator
	template  *template.Engine

	// 状态管理
	running   atomic.Bool
	startTime time.Time

	// 速率限制
	rateLimiter *RateLimiter
}

// Results 测试结果
type Results struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64

	Duration   time.Duration
	Throughput float64

	BytesReceived int64
	BytesSent     int64

	Latency      stats.LatencyStats
	ErrorsByType map[string]int64
	StatusCodes  map[int]int64

	// 时间序列数据
	TimeSeries []stats.TimePoint
}

// New 创建基准测试器
func New(cfg *config.Config) (*Benchmark, error) {
	// 创建HTTP客户端
	client, err := createHTTPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP客户端失败: %w", err)
	}

	// 创建统计收集器
	statsCollector := stats.NewCollector()

	// 创建验证器
	val := validator.New(cfg.Validation)

	// 创建模板引擎
	tmpl := template.New(cfg.Request.Template)

	b := &Benchmark{
		config:    cfg,
		client:    client,
		stats:     statsCollector,
		validator: val,
		template:  tmpl,
	}

	// 初始化速率限制器
	if cfg.Load.RateLimit > 0 {
		b.rateLimiter = NewRateLimiter(cfg.Load.RateLimit)
	}

	return b, nil
}

// Run 执行基准测试
func (b *Benchmark) Run(ctx context.Context) (*Results, error) {
	b.running.Store(true)
	b.startTime = time.Now()
	defer b.running.Store(false)

	// 创建工作上下文
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 根据负载模式执行测试
	switch b.config.Load.LoadPattern {
	case config.LoadPatternRampUp:
		return b.runRampUp(workCtx)
	case config.LoadPatternBurst:
		return b.runBurst(workCtx)
	default:
		return b.runConstant(workCtx)
	}
}

// runConstant 恒定负载测试
func (b *Benchmark) runConstant(ctx context.Context) (*Results, error) {
	var wg sync.WaitGroup
	requestChan := make(chan struct{}, b.config.Load.Concurrency)

	// 时间或请求数限制
	var timeoutCtx context.Context
	var timeoutCancel context.CancelFunc

	if b.config.Load.Duration > 0 {
		timeoutCtx, timeoutCancel = context.WithTimeout(ctx, b.config.Load.Duration)
	} else {
		timeoutCtx, timeoutCancel = context.WithCancel(ctx)
	}
	defer timeoutCancel()

	// 启动工作协程
	for i := 0; i < b.config.Load.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			b.worker(timeoutCtx, workerID, requestChan)
		}(i)
	}

	// 生成请求
	go b.generateRequests(timeoutCtx, requestChan)

	// 实时监控
	if b.config.Output.RealtimeMonitor {
		go b.realtimeMonitor(timeoutCtx)
	}

	// 等待完成
	wg.Wait()

	return b.generateResults(), nil
}

// runRampUp 渐进式负载测试
func (b *Benchmark) runRampUp(ctx context.Context) (*Results, error) {
	rampCfg := b.config.Load.RampUp
	if !rampCfg.Enabled {
		return b.runConstant(ctx)
	}

	stepDuration := rampCfg.Duration / time.Duration(rampCfg.Steps)
	concurrencyStep := (rampCfg.EndConcurrency - rampCfg.StartConcurrency) / rampCfg.Steps

	fmt.Printf("📈 渐进式负载: %d -> %d (步长: %d, 每步: %v)\n",
		rampCfg.StartConcurrency, rampCfg.EndConcurrency, rampCfg.Steps, stepDuration)

	var wg sync.WaitGroup
	requestChan := make(chan struct{}, rampCfg.EndConcurrency)
	workerControl := make(chan int, rampCfg.EndConcurrency)

	// 启动工作协程池
	for i := 0; i < rampCfg.EndConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 等待激活信号
			<-workerControl
			b.worker(ctx, workerID, requestChan)
		}(i)
	}

	// 渐进增加并发
	go func() {
		currentConcurrency := rampCfg.StartConcurrency

		// 激活初始工作协程
		for i := 0; i < currentConcurrency; i++ {
			workerControl <- i
		}

		ticker := time.NewTicker(stepDuration)
		defer ticker.Stop()

		for step := 0; step < rampCfg.Steps && ctx.Err() == nil; step++ {
			<-ticker.C

			// 增加并发
			newConcurrency := currentConcurrency + concurrencyStep
			if newConcurrency > rampCfg.EndConcurrency {
				newConcurrency = rampCfg.EndConcurrency
			}

			for i := currentConcurrency; i < newConcurrency; i++ {
				workerControl <- i
			}

			fmt.Printf("  ↑ 并发调整: %d -> %d\n", currentConcurrency, newConcurrency)
			currentConcurrency = newConcurrency
		}
	}()

	// 生成请求
	go b.generateRequests(ctx, requestChan)

	// 实时监控
	if b.config.Output.RealtimeMonitor {
		go b.realtimeMonitor(ctx)
	}

	wg.Wait()
	return b.generateResults(), nil
}

// runBurst 突发负载测试
func (b *Benchmark) runBurst(ctx context.Context) (*Results, error) {
	burstCfg := b.config.Load.BurstMode
	if !burstCfg.Enabled {
		return b.runConstant(ctx)
	}

	fmt.Printf("💥 突发负载模式: 基准 %d, 突发 %d (持续: %v, 间隔: %v)\n",
		burstCfg.BaseConcurrency, burstCfg.BurstConcurrency,
		burstCfg.BurstDuration, burstCfg.BurstInterval)

	var wg sync.WaitGroup
	requestChan := make(chan struct{}, burstCfg.BurstConcurrency)

	// 启动基准工作协程
	for i := 0; i < burstCfg.BaseConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			b.worker(ctx, workerID, requestChan)
		}(i)
	}

	// 突发协程池
	// burstWorkers := make(chan struct{}, burstCfg.BurstConcurrency-burstCfg.BaseConcurrency)

	// 突发控制
	go func() {
		ticker := time.NewTicker(burstCfg.BurstInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 触发突发
				fmt.Printf("  💥 触发突发: +%d 并发\n", burstCfg.BurstConcurrency-burstCfg.BaseConcurrency)

				burstCtx, burstCancel := context.WithTimeout(ctx, burstCfg.BurstDuration)

				for i := 0; i < burstCfg.BurstConcurrency-burstCfg.BaseConcurrency; i++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						b.worker(burstCtx, burstCfg.BaseConcurrency+workerID, requestChan)
					}(i)
				}

				<-burstCtx.Done()
				burstCancel()
				fmt.Printf("  ✓ 突发结束\n")
			}
		}
	}()

	// 生成请求
	go b.generateRequests(ctx, requestChan)

	// 实时监控
	if b.config.Output.RealtimeMonitor {
		go b.realtimeMonitor(ctx)
	}

	wg.Wait()
	return b.generateResults(), nil
}

// worker 工作协程
func (b *Benchmark) worker(ctx context.Context, workerID int, requestChan <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-requestChan:
			if !ok {
				return
			}
			b.executeRequest(ctx, workerID)
		}
	}
}

// generateRequests 生成请求
func (b *Benchmark) generateRequests(ctx context.Context, requestChan chan<- struct{}) {
	defer close(requestChan)

	totalRequests := b.config.Load.TotalRequests
	requestCount := int64(0)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 检查请求数限制
			if totalRequests > 0 && requestCount >= int64(totalRequests) {
				return
			}

			// 速率限制
			if b.rateLimiter != nil {
				b.rateLimiter.Wait(ctx)
			}

			select {
			case requestChan <- struct{}{}:
				requestCount++
			case <-ctx.Done():
				return
			}
		}
	}
}

// executeRequest 执行单个请求
func (b *Benchmark) executeRequest(ctx context.Context, workerID int) {
	startTime := time.Now()

	// 创建请求
	req, err := b.createRequest(ctx, workerID)
	if err != nil {
		b.stats.RecordError("request_creation", err)
		fmt.Printf("err: %e \n", err)
		return
	}

	// 发送请求
	resp, err := b.client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		b.stats.RecordError("network", err)
		fmt.Printf("err: %e \n", err)
		b.stats.RecordRequest(latency, 0, 0, false)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b.stats.RecordError("body_read", err)
		fmt.Printf("err: %e \n", err)
		b.stats.RecordRequest(latency, 0, 0, false)
		return
	}

	// 验证响应
	validationErr := b.validator.Validate(resp, body)
	success := validationErr == nil

	if !success {
		b.stats.RecordError("validation", validationErr)
		fmt.Printf("err: %e \n", validationErr)
	}

	// 记录统计
	b.stats.RecordRequest(latency, int64(len(body)), int64(req.ContentLength), success)
	b.stats.RecordStatusCode(resp.StatusCode)
}

// createRequest 创建HTTP请求
func (b *Benchmark) createRequest(ctx context.Context, workerID int) (*http.Request, error) {
	// 应用模板
	url := b.config.Target.URL
	body := b.config.Target.Body

	if b.config.Request.Template.Enabled {
		vars := map[string]interface{}{
			"worker_id": workerID,
			"timestamp": time.Now().Unix(),
		}

		var err error
		url, err = b.template.Render(url, vars)
		if err != nil {
			return nil, fmt.Errorf("渲染URL模板失败: %w", err)
		}

		if b.config.Request.DynamicBody {
			body, err = b.template.Render(b.config.Request.BodyTemplate, vars)
			if err != nil {
				return nil, fmt.Errorf("渲染Body模板失败: %w", err)
			}
		}
	}

	// 创建请求
	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequestWithContext(ctx, b.config.Target.Method, url,
			strings.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, b.config.Target.Method, url, nil)
	}

	if err != nil {
		fmt.Printf("err: %e\n", err)
		return nil, err
	}

	// 设置请求头
	for key, value := range b.config.Target.Headers {
		req.Header.Set(key, value)
	}
	for key, value := range b.config.Request.Headers {
		req.Header.Set(key, value)
	}

	// 设置Cookie
	for _, cookie := range b.config.Request.Cookies {
		req.AddCookie(&http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  cookie.Expires,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
		})
	}

	return req, nil
}

// Close 关闭基准测试器
func (b *Benchmark) Close() error {
	b.running.Store(false)
	return nil
}

// createHTTPClient 创建HTTP客户端
func createHTTPClient(cfg *config.Config) (*http.Client, error) {
	// TLS配置
	tlsConfig, err := createTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	// HTTP/3 (QUIC)
	if cfg.Protocol.HTTP3Enabled {
		return &http.Client{
			Transport: &http3.RoundTripper{
				TLSClientConfig: tlsConfig,
			},
			Timeout: cfg.Target.Timeout,
		}, nil
	}

	// HTTP/1.1 和 HTTP/2
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        cfg.Load.Concurrency * 2,
		MaxIdleConnsPerHost: cfg.Load.Concurrency,
		IdleConnTimeout:     cfg.Protocol.IdleTimeout,
		DisableKeepAlives:   !cfg.Protocol.KeepAlive,
	}

	// HTTP/2
	if cfg.Protocol.HTTP2Enabled {
		http2.ConfigureTransport(transport)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.Target.Timeout,
	}, nil
}

// createTLSConfig 创建TLS配置
func createTLSConfig(cfg config.TLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	// TLS版本
	switch cfg.MinVersion {
	case "TLS1.0":
		tlsConfig.MinVersion = tls.VersionTLS10
	case "TLS1.1":
		tlsConfig.MinVersion = tls.VersionTLS11
	case "TLS1.2":
		tlsConfig.MinVersion = tls.VersionTLS12
	case "TLS1.3":
		tlsConfig.MinVersion = tls.VersionTLS13
	default:
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	switch cfg.MaxVersion {
	case "TLS1.2":
		tlsConfig.MaxVersion = tls.VersionTLS12
	case "TLS1.3":
		tlsConfig.MaxVersion = tls.VersionTLS13
	}

	// 客户端证书(双向认证)
	if cfg.MutualTLS && cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书失败: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// 继续实现剩余方法...
