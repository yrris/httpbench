package stats

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// TimePoint 时间点数据
type TimePoint struct {
	Timestamp  time.Time
	RPS        float64
	AvgLatency time.Duration
	ErrorRate  float64
}

// Collector 统计收集器
type Collector struct {
	// 基础计数
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	totalErrors     atomic.Int64

	bytesReceived atomic.Int64
	bytesSent     atomic.Int64

	// 延迟直方图 (使用HDR Histogram)
	latencyHistogram *hdrhistogram.Histogram
	histogramMu      sync.RWMutex

	// 错误分类
	errorsByType map[string]*atomic.Int64
	errorsMu     sync.RWMutex

	// 状态码统计
	statusCodes map[int]*atomic.Int64
	statusMu    sync.RWMutex

	// 时间序列数据
	timeSeries   []TimePoint
	timeSeriesMu sync.RWMutex
	lastSnapshot time.Time

	startTime time.Time
}

// LatencyStats 延迟统计
type LatencyStats struct {
	Min    time.Duration
	Max    time.Duration
	Mean   time.Duration
	StdDev time.Duration

	P50  time.Duration
	P75  time.Duration
	P90  time.Duration
	P95  time.Duration
	P99  time.Duration
	P999 time.Duration
}

// Snapshot 快照数据
type Snapshot struct {
	TotalRequests   int64
	SuccessRequests int64
	TotalErrors     int64

	BytesReceived int64
	BytesSent     int64

	Latency LatencyStats

	AvgLatency time.Duration
	P99Latency time.Duration

	ErrorsByType map[string]int64
	StatusCodes  map[int]int64

	Timestamp time.Time
}

// NewCollector 创建统计收集器
func NewCollector() *Collector {
	// HDR Histogram: 1微秒到1小时的范围,精度3位有效数字
	histogram := hdrhistogram.New(1, 3600000000, 3)

	return &Collector{
		latencyHistogram: histogram,
		errorsByType:     make(map[string]*atomic.Int64),
		statusCodes:      make(map[int]*atomic.Int64),
		timeSeries:       make([]TimePoint, 0),
		startTime:        time.Now(),
		lastSnapshot:     time.Now(),
	}
}

// RecordRequest 记录请求
func (c *Collector) RecordRequest(latency time.Duration, bytesReceived, bytesSent int64, success bool) {
	c.totalRequests.Add(1)

	if success {
		c.successRequests.Add(1)
	} else {
		c.totalErrors.Add(1)
	}

	c.bytesReceived.Add(bytesReceived)
	c.bytesSent.Add(bytesSent)

	// 记录延迟到直方图 (转换为微秒)
	c.histogramMu.Lock()
	c.latencyHistogram.RecordValue(latency.Microseconds())
	c.histogramMu.Unlock()
}

// RecordError 记录错误
func (c *Collector) RecordError(errorType string, err error) {
	c.totalErrors.Add(1)

	c.errorsMu.Lock()
	counter, exists := c.errorsByType[errorType]
	if !exists {
		counter = &atomic.Int64{}
		c.errorsByType[errorType] = counter
	}
	c.errorsMu.Unlock()

	counter.Add(1)
}

// RecordStatusCode 记录状态码
func (c *Collector) RecordStatusCode(code int) {
	c.statusMu.Lock()
	counter, exists := c.statusCodes[code]
	if !exists {
		counter = &atomic.Int64{}
		c.statusCodes[code] = counter
	}
	c.statusMu.Unlock()

	counter.Add(1)
}

// Snapshot 获取当前快照
func (c *Collector) Snapshot() Snapshot {
	snapshot := Snapshot{
		TotalRequests:   c.totalRequests.Load(),
		SuccessRequests: c.successRequests.Load(),
		TotalErrors:     c.totalErrors.Load(),
		BytesReceived:   c.bytesReceived.Load(),
		BytesSent:       c.bytesSent.Load(),
		ErrorsByType:    make(map[string]int64),
		StatusCodes:     make(map[int]int64),
		Timestamp:       time.Now(),
	}

	// 复制错误统计
	c.errorsMu.RLock()
	for errType, counter := range c.errorsByType {
		snapshot.ErrorsByType[errType] = counter.Load()
	}
	c.errorsMu.RUnlock()

	// 复制状态码统计
	c.statusMu.RLock()
	for code, counter := range c.statusCodes {
		snapshot.StatusCodes[code] = counter.Load()
	}
	c.statusMu.RUnlock()

	// 计算延迟统计
	c.histogramMu.RLock()
	snapshot.Latency = c.calculateLatencyStats()
	snapshot.AvgLatency = time.Duration(c.latencyHistogram.Mean()) * time.Microsecond
	snapshot.P99Latency = time.Duration(c.latencyHistogram.ValueAtQuantile(99.0)) * time.Microsecond
	c.histogramMu.RUnlock()

	// 记录时间点
	c.recordTimePoint(snapshot)

	return snapshot
}

// calculateLatencyStats 计算延迟统计
func (c *Collector) calculateLatencyStats() LatencyStats {
	hist := c.latencyHistogram

	return LatencyStats{
		Min:    time.Duration(hist.Min()) * time.Microsecond,
		Max:    time.Duration(hist.Max()) * time.Microsecond,
		Mean:   time.Duration(hist.Mean()) * time.Microsecond,
		StdDev: time.Duration(hist.StdDev()) * time.Microsecond,
		P50:    time.Duration(hist.ValueAtQuantile(50.0)) * time.Microsecond,
		P75:    time.Duration(hist.ValueAtQuantile(75.0)) * time.Microsecond,
		P90:    time.Duration(hist.ValueAtQuantile(90.0)) * time.Microsecond,
		P95:    time.Duration(hist.ValueAtQuantile(95.0)) * time.Microsecond,
		P99:    time.Duration(hist.ValueAtQuantile(99.0)) * time.Microsecond,
		P999:   time.Duration(hist.ValueAtQuantile(99.9)) * time.Microsecond,
	}
}

// recordTimePoint 记录时间点数据
func (c *Collector) recordTimePoint(snapshot Snapshot) {
	now := time.Now()
	duration := now.Sub(c.lastSnapshot).Seconds()

	if duration < 1.0 {
		return // 至少1秒间隔
	}

	// 计算RPS
	requestsDelta := snapshot.TotalRequests - c.getLastTotalRequests()
	rps := float64(requestsDelta) / duration

	// 计算错误率
	errorRate := 0.0
	if snapshot.TotalRequests > 0 {
		errorRate = float64(snapshot.TotalErrors) / float64(snapshot.TotalRequests)
	}

	point := TimePoint{
		Timestamp:  now,
		RPS:        rps,
		AvgLatency: snapshot.AvgLatency,
		ErrorRate:  errorRate,
	}

	c.timeSeriesMu.Lock()
	c.timeSeries = append(c.timeSeries, point)
	c.timeSeriesMu.Unlock()

	c.lastSnapshot = now
}

// GetTimeSeries 获取时间序列数据
func (c *Collector) GetTimeSeries() []TimePoint {
	c.timeSeriesMu.RLock()
	defer c.timeSeriesMu.RUnlock()

	result := make([]TimePoint, len(c.timeSeries))
	copy(result, c.timeSeries)
	return result
}

// getLastTotalRequests 获取上次快照的总请求数
func (c *Collector) getLastTotalRequests() int64 {
	c.timeSeriesMu.RLock()
	defer c.timeSeriesMu.RUnlock()

	if len(c.timeSeries) == 0 {
		return 0
	}

	// 这里简化实现,实际应该保存上次快照
	return 0
}

// Reset 重置统计
func (c *Collector) Reset() {
	c.totalRequests.Store(0)
	c.successRequests.Store(0)
	c.totalErrors.Store(0)
	c.bytesReceived.Store(0)
	c.bytesSent.Store(0)

	c.histogramMu.Lock()
	c.latencyHistogram.Reset()
	c.histogramMu.Unlock()

	c.errorsMu.Lock()
	c.errorsByType = make(map[string]*atomic.Int64)
	c.errorsMu.Unlock()

	c.statusMu.Lock()
	c.statusCodes = make(map[int]*atomic.Int64)
	c.statusMu.Unlock()

	c.timeSeriesMu.Lock()
	c.timeSeries = make([]TimePoint, 0)
	c.timeSeriesMu.Unlock()

	c.startTime = time.Now()
	c.lastSnapshot = time.Now()
}

// GetLatencyDistribution 获取延迟分布
func (c *Collector) GetLatencyDistribution() *hdrhistogram.Snapshot {
	c.histogramMu.RLock()
	defer c.histogramMu.RUnlock()

	// 返回副本快照
	return c.latencyHistogram.Export()
}

// GetLatencyPercentiles 获取指定百分位的延迟
func (c *Collector) GetLatencyPercentiles(percentiles []float64) map[float64]time.Duration {
	c.histogramMu.RLock()
	defer c.histogramMu.RUnlock()

	result := make(map[float64]time.Duration)
	for _, p := range percentiles {
		value := c.latencyHistogram.ValueAtQuantile(p)
		result[p] = time.Duration(value) * time.Microsecond
	}

	return result
}
