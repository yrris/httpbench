package benchmark

import (
	"context"
	"fmt"
	"time"
)

// realtimeMonitor 实时监控
func (b *Benchmark) realtimeMonitor(ctx context.Context) {
	ticker := time.NewTicker(b.config.Output.MonitorInterval)
	defer ticker.Stop()

	lastSnapshot := b.stats.Snapshot()
	lastTime := time.Now()

	fmt.Println("\n⏱️  实时监控已启动")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-10s %-10s %-15s %-15s %-10s\n", "时间", "RPS", "平均延迟", "P99延迟", "错误率")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentSnapshot := b.stats.Snapshot()
			currentTime := time.Now()

			// 计算间隔内的指标
			duration := currentTime.Sub(lastTime).Seconds()
			requestsDelta := currentSnapshot.TotalRequests - lastSnapshot.TotalRequests
			errorsDelta := currentSnapshot.TotalErrors - lastSnapshot.TotalErrors

			rps := float64(requestsDelta) / duration
			errorRate := 0.0
			if requestsDelta > 0 {
				errorRate = float64(errorsDelta) / float64(requestsDelta) * 100
			}

			elapsed := currentTime.Sub(b.startTime)
			fmt.Printf("%-10s %-10.2f %-15v %-15v %-10.2f%%\n",
				formatDuration(elapsed),
				rps,
				currentSnapshot.AvgLatency,
				currentSnapshot.P99Latency,
				errorRate,
			)

			lastSnapshot = currentSnapshot
			lastTime = currentTime
		}
	}
}

// generateResults 生成测试结果
func (b *Benchmark) generateResults() *Results {
	snapshot := b.stats.Snapshot()
	duration := time.Since(b.startTime)

	results := &Results{
		TotalRequests:   snapshot.TotalRequests,
		SuccessRequests: snapshot.SuccessRequests,
		FailedRequests:  snapshot.TotalErrors,
		Duration:        duration,
		BytesReceived:   snapshot.BytesReceived,
		BytesSent:       snapshot.BytesSent,
		ErrorsByType:    snapshot.ErrorsByType,
		StatusCodes:     snapshot.StatusCodes,
	}

	// 计算吞吐量
	if duration.Seconds() > 0 {
		results.Throughput = float64(results.TotalRequests) / duration.Seconds()
	}

	// 延迟统计
	results.Latency = snapshot.Latency

	// 时间序列数据
	results.TimeSeries = b.stats.GetTimeSeries()

	return results
}

// formatDuration 格式化持续时间
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// RateLimiter 速率限制器
type RateLimiter struct {
	rps      int
	interval time.Duration
	ticker   *time.Ticker
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rps int) *RateLimiter {
	interval := time.Second / time.Duration(rps)
	return &RateLimiter{
		rps:      rps,
		interval: interval,
		ticker:   time.NewTicker(interval),
	}
}

// Wait 等待速率限制
func (r *RateLimiter) Wait(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-r.ticker.C:
		return
	}
}

// Stop 停止速率限制器
func (r *RateLimiter) Stop() {
	if r.ticker != nil {
		r.ticker.Stop()
	}
}
