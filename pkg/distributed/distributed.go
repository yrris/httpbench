package distributed

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"httpbench/pkg/benchmark"
	"httpbench/pkg/config"
)

// Master 主节点
type Master struct {
	config  *config.Config
	workers []*WorkerClient
	mu      sync.RWMutex
	
	// 结果聚合
	results     []*benchmark.Results
	resultsChan chan *benchmark.Results
	
	server *grpc.Server
}

// WorkerClient 工作节点客户端
type WorkerClient struct {
	address string
	conn    *grpc.ClientConn
	client  BenchmarkServiceClient
	id      string
}

// NewMaster 创建主节点
func NewMaster(cfg *config.Config) (*Master, error) {
	m := &Master{
		config:      cfg,
		workers:     make([]*WorkerClient, 0),
		results:     make([]*benchmark.Results, 0),
		resultsChan: make(chan *benchmark.Results, 100),
	}

	// 连接所有工作节点
	for i, addr := range cfg.Distributed.WorkerAddresses {
		worker, err := m.connectWorker(addr, fmt.Sprintf("worker-%d", i))
		if err != nil {
			log.Printf("连接工作节点 %s 失败: %v", addr, err)
			continue
		}
		m.workers = append(m.workers, worker)
	}

	if len(m.workers) == 0 {
		return nil, fmt.Errorf("没有可用的工作节点")
	}

	return m, nil
}

// connectWorker 连接工作节点
func (m *Master) connectWorker(address, id string) (*WorkerClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	client := NewBenchmarkServiceClient(conn)

	return &WorkerClient{
		address: address,
		conn:    conn,
		client:  client,
		id:      id,
	}, nil
}

// Run 运行分布式测试
func (m *Master) Run(ctx context.Context) (*benchmark.Results, error) {
	fmt.Printf("🌐 分布式模式: 1个主节点 + %d个工作节点\n", len(m.workers))

	// 分配负载
	workload := m.distributeWorkload()

	// 启动所有工作节点
	var wg sync.WaitGroup
	for i, worker := range m.workers {
		wg.Add(1)
		go func(w *WorkerClient, load *WorkloadConfig) {
			defer wg.Done()
			
			result, err := m.executeWorker(ctx, w, load)
			if err != nil {
				log.Printf("工作节点 %s 执行失败: %v", w.id, err)
				return
			}
			
			m.resultsChan <- result
		}(worker, workload[i])
	}

	// 等待所有工作节点完成
	go func() {
		wg.Wait()
		close(m.resultsChan)
	}()

	// 收集结果
	for result := range m.resultsChan {
		m.mu.Lock()
		m.results = append(m.results, result)
		m.mu.Unlock()
	}

	// 聚合结果
	return m.aggregateResults(), nil
}

// distributeWorkload 分配工作负载
func (m *Master) distributeWorkload() []*WorkloadConfig {
	workerCount := len(m.workers)
	concurrencyPerWorker := m.config.Load.Concurrency / workerCount
	remainder := m.config.Load.Concurrency % workerCount

	workloads := make([]*WorkloadConfig, workerCount)
	for i := 0; i < workerCount; i++ {
		concurrency := concurrencyPerWorker
		if i < remainder {
			concurrency++
		}

		workloads[i] = &WorkloadConfig{
			TargetURL:   m.config.Target.URL,
			Method:      m.config.Target.Method,
			Concurrency: int32(concurrency),
			Duration:    int64(m.config.Load.Duration),
			RateLimit:   int32(m.config.Load.RateLimit / workerCount),
		}
	}

	return workloads
}

// executeWorker 执行工作节点
func (m *Master) executeWorker(ctx context.Context, worker *WorkerClient, load *WorkloadConfig) (*benchmark.Results, error) {
	fmt.Printf("  → 启动工作节点 %s (并发: %d)\n", worker.id, load.Concurrency)

	resp, err := worker.client.RunBenchmark(ctx, &BenchmarkRequest{
		Workload: load,
	})
	if err != nil {
		return nil, err
	}

	fmt.Printf("  ✓ 工作节点 %s 完成 (请求: %d)\n", worker.id, resp.TotalRequests)

	return m.convertProtoResult(resp), nil
}

// aggregateResults 聚合结果
func (m *Master) aggregateResults() *benchmark.Results {
	if len(m.results) == 0 {
		return &benchmark.Results{}
	}

	aggregated := &benchmark.Results{
		ErrorsByType: make(map[string]int64),
		StatusCodes:  make(map[int]int64),
	}

	// 聚合基础指标
	for _, result := range m.results {
		aggregated.TotalRequests += result.TotalRequests
		aggregated.SuccessRequests += result.SuccessRequests
		aggregated.FailedRequests += result.FailedRequests
		aggregated.BytesReceived += result.BytesReceived
		aggregated.BytesSent += result.BytesSent

		// 聚合错误
		for errType, count := range result.ErrorsByType {
			aggregated.ErrorsByType[errType] += count
		}

		// 聚合状态码
		for code, count := range result.StatusCodes {
			aggregated.StatusCodes[code] += count
		}

		// 使用最长持续时间
		if result.Duration > aggregated.Duration {
			aggregated.Duration = result.Duration
		}
	}

	// 计算平均延迟 (简化实现)
	totalLatency := time.Duration(0)
	for _, result := range m.results {
		totalLatency += result.Latency.Mean
	}
	aggregated.Latency.Mean = totalLatency / time.Duration(len(m.results))

	// 计算吞吐量
	if aggregated.Duration.Seconds() > 0 {
		aggregated.Throughput = float64(aggregated.TotalRequests) / aggregated.Duration.Seconds()
	}

	return aggregated
}

// convertProtoResult 转换Proto结果
func (m *Master) convertProtoResult(resp *BenchmarkResponse) *benchmark.Results {
	return &benchmark.Results{
		TotalRequests:   resp.TotalRequests,
		SuccessRequests: resp.SuccessRequests,
		FailedRequests:  resp.FailedRequests,
		Duration:        time.Duration(resp.DurationMs) * time.Millisecond,
		BytesReceived:   resp.BytesReceived,
		BytesSent:       resp.BytesSent,
	}
}

// Close 关闭主节点
func (m *Master) Close() error {
	for _, worker := range m.workers {
		if worker.conn != nil {
			worker.conn.Close()
		}
	}
	if m.server != nil {
		m.server.GracefulStop()
	}
	return nil
}

// Worker 工作节点
type Worker struct {
	config *config.Config
	server *grpc.Server
	
	UnimplementedBenchmarkServiceServer
}

// NewWorker 创建工作节点
func NewWorker(cfg *config.Config) (*Worker, error) {
	return &Worker{
		config: cfg,
	}, nil
}

// Start 启动工作节点服务
func (w *Worker) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	w.server = grpc.NewServer()
	RegisterBenchmarkServiceServer(w.server, w)

	fmt.Printf("🔧 工作节点启动在端口 %d\n", port)
	return w.server.Serve(lis)
}

// RunBenchmark 执行基准测试
func (w *Worker) RunBenchmark(ctx context.Context, req *BenchmarkRequest) (*BenchmarkResponse, error) {
	// 创建临时配置
	cfg := *w.config
	cfg.Target.URL = req.Workload.TargetURL
	cfg.Target.Method = req.Workload.Method
	cfg.Load.Concurrency = int(req.Workload.Concurrency)
	cfg.Load.Duration = time.Duration(req.Workload.Duration)
	cfg.Load.RateLimit = int(req.Workload.RateLimit)

	// 执行测试
	bench, err := benchmark.New(&cfg)
	if err != nil {
		return nil, err
	}
	defer bench.Close()

	results, err := bench.Run(ctx)
	if err != nil {
		return nil, err
	}

	// 转换结果
	return &BenchmarkResponse{
		TotalRequests:   results.TotalRequests,
		SuccessRequests: results.SuccessRequests,
		FailedRequests:  results.FailedRequests,
		DurationMs:      results.Duration.Milliseconds(),
		BytesReceived:   results.BytesReceived,
		BytesSent:       results.BytesSent,
	}, nil
}

// Stop 停止工作节点
func (w *Worker) Stop() {
	if w.server != nil {
		w.server.GracefulStop()
	}
}

// 简化的gRPC定义(实际应该用proto文件生成)
type BenchmarkServiceClient interface {
	RunBenchmark(ctx context.Context, req *BenchmarkRequest, opts ...grpc.CallOption) (*BenchmarkResponse, error)
}

type BenchmarkRequest struct {
	Workload *WorkloadConfig
}

type WorkloadConfig struct {
	TargetURL   string
	Method      string
	Concurrency int32
	Duration    int64
	RateLimit   int32
}

type BenchmarkResponse struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	DurationMs      int64
	BytesReceived   int64
	BytesSent       int64
}

type UnimplementedBenchmarkServiceServer struct{}

func RegisterBenchmarkServiceServer(s *grpc.Server, srv interface{}) {}

func NewBenchmarkServiceClient(conn *grpc.ClientConn) BenchmarkServiceClient {
	return &benchmarkServiceClient{conn: conn}
}

type benchmarkServiceClient struct {
	conn *grpc.ClientConn
}

func (c *benchmarkServiceClient) RunBenchmark(ctx context.Context, req *BenchmarkRequest, opts ...grpc.CallOption) (*BenchmarkResponse, error) {
	// 简化实现
	return &BenchmarkResponse{}, nil
}
