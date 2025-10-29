package reporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"httpbench/pkg/benchmark"
	"httpbench/pkg/stats"
)

// Reporter 报告生成器
type Reporter interface {
	Generate(results *benchmark.Results, outputPath string) error
}

// New 创建报告生成器
func New(format string) Reporter {
	switch format {
	case "json":
		return &JSONReporter{}
	case "csv":
		return &CSVReporter{}
	default:
		return &ConsoleReporter{}
	}
}

// ConsoleReporter 控制台报告
type ConsoleReporter struct{}

func (r *ConsoleReporter) Generate(results *benchmark.Results, outputPath string) error {
	// 控制台输出在main.go中处理
	return nil
}

// JSONReporter JSON报告
type JSONReporter struct{}

func (r *JSONReporter) Generate(results *benchmark.Results, outputPath string) error {
	// 防止除以零
	successRate := 0.0
	if results.TotalRequests > 0 {
		successRate = float64(results.SuccessRequests) / float64(results.TotalRequests) * 100
	}

	receiveRate := 0.0
	sendRate := 0.0
	if results.Duration.Seconds() > 0 {
		receiveRate = float64(results.BytesReceived) / results.Duration.Seconds()
		sendRate = float64(results.BytesSent) / results.Duration.Seconds()
	}

	report := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_requests":   results.TotalRequests,
			"success_requests": results.SuccessRequests,
			"failed_requests":  results.FailedRequests,
			"success_rate":     successRate,
			"duration_seconds": results.Duration.Seconds(),
			"throughput_rps":   results.Throughput,
		},
		"latency": map[string]interface{}{
			"min_ms":    results.Latency.Min.Milliseconds(),
			"max_ms":    results.Latency.Max.Milliseconds(),
			"mean_ms":   results.Latency.Mean.Milliseconds(),
			"stddev_ms": results.Latency.StdDev.Milliseconds(),
			"p50_ms":    results.Latency.P50.Milliseconds(),
			"p75_ms":    results.Latency.P75.Milliseconds(),
			"p90_ms":    results.Latency.P90.Milliseconds(),
			"p95_ms":    results.Latency.P95.Milliseconds(),
			"p99_ms":    results.Latency.P99.Milliseconds(),
			"p999_ms":   results.Latency.P999.Milliseconds(),
		},
		"transfer": map[string]interface{}{
			"bytes_received":   results.BytesReceived,
			"bytes_sent":       results.BytesSent,
			"receive_rate_bps": receiveRate,
			"send_rate_bps":    sendRate,
		},
		"errors":       results.ErrorsByType,
		"status_codes": results.StatusCodes,
		"time_series":  r.formatTimeSeries(results.TimeSeries),
		"generated_at": time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
		fmt.Printf("\n📄 JSON报告已保存: %s\n", outputPath)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func (r *JSONReporter) formatTimeSeries(series []stats.TimePoint) []map[string]interface{} {
	result := make([]map[string]interface{}, len(series))
	for i, point := range series {
		result[i] = map[string]interface{}{
			"timestamp":      point.Timestamp.Format(time.RFC3339),
			"rps":            point.RPS,
			"avg_latency_ms": point.AvgLatency.Milliseconds(),
			"error_rate":     point.ErrorRate,
		}
	}
	return result
}

// CSVReporter CSV报告
type CSVReporter struct{}

func (r *CSVReporter) Generate(results *benchmark.Results, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("benchmark_report_%s.csv", time.Now().Format("20060102_150405"))
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入摘要部分
	writer.Write([]string{"Metric", "Value"})
	writer.Write([]string{"Total Requests", fmt.Sprintf("%d", results.TotalRequests)})
	writer.Write([]string{"Success Requests", fmt.Sprintf("%d", results.SuccessRequests)})
	writer.Write([]string{"Failed Requests", fmt.Sprintf("%d", results.FailedRequests)})

	// 防止除以零
	successRate := 0.0
	if results.TotalRequests > 0 {
		successRate = float64(results.SuccessRequests) / float64(results.TotalRequests) * 100
	}
	writer.Write([]string{"Success Rate", fmt.Sprintf("%.2f%%", successRate)})
	writer.Write([]string{"Duration (seconds)", fmt.Sprintf("%.2f", results.Duration.Seconds())})
	writer.Write([]string{"Throughput (req/s)", fmt.Sprintf("%.2f", results.Throughput)})
	writer.Write([]string{})

	// 延迟统计
	writer.Write([]string{"Latency Metric", "Value (ms)"})
	writer.Write([]string{"Min", fmt.Sprintf("%.2f", float64(results.Latency.Min.Microseconds())/1000)})
	writer.Write([]string{"Max", fmt.Sprintf("%.2f", float64(results.Latency.Max.Microseconds())/1000)})
	writer.Write([]string{"Mean", fmt.Sprintf("%.2f", float64(results.Latency.Mean.Microseconds())/1000)})
	writer.Write([]string{"StdDev", fmt.Sprintf("%.2f", float64(results.Latency.StdDev.Microseconds())/1000)})
	writer.Write([]string{"P50", fmt.Sprintf("%.2f", float64(results.Latency.P50.Microseconds())/1000)})
	writer.Write([]string{"P75", fmt.Sprintf("%.2f", float64(results.Latency.P75.Microseconds())/1000)})
	writer.Write([]string{"P90", fmt.Sprintf("%.2f", float64(results.Latency.P90.Microseconds())/1000)})
	writer.Write([]string{"P95", fmt.Sprintf("%.2f", float64(results.Latency.P95.Microseconds())/1000)})
	writer.Write([]string{"P99", fmt.Sprintf("%.2f", float64(results.Latency.P99.Microseconds())/1000)})
	writer.Write([]string{"P99.9", fmt.Sprintf("%.2f", float64(results.Latency.P999.Microseconds())/1000)})
	writer.Write([]string{})

	// 传输统计
	writer.Write([]string{"Transfer Metric", "Value"})
	writer.Write([]string{"Bytes Received", fmt.Sprintf("%d", results.BytesReceived)})
	writer.Write([]string{"Bytes Sent", fmt.Sprintf("%d", results.BytesSent)})

	// 防止除以零
	receiveRate := 0.0
	if results.Duration.Seconds() > 0 {
		receiveRate = float64(results.BytesReceived) / results.Duration.Seconds()
	}
	writer.Write([]string{"Receive Rate (bytes/s)", fmt.Sprintf("%.2f", receiveRate)})
	writer.Write([]string{})

	// 错误统计
	if len(results.ErrorsByType) > 0 {
		writer.Write([]string{"Error Type", "Count"})
		for errType, count := range results.ErrorsByType {
			writer.Write([]string{errType, fmt.Sprintf("%d", count)})
		}
		writer.Write([]string{})
	}

	// 状态码统计
	if len(results.StatusCodes) > 0 {
		writer.Write([]string{"Status Code", "Count"})
		for code, count := range results.StatusCodes {
			writer.Write([]string{fmt.Sprintf("%d", code), fmt.Sprintf("%d", count)})
		}
		writer.Write([]string{})
	}

	// 时间序列数据
	if len(results.TimeSeries) > 0 {
		writer.Write([]string{"Timestamp", "RPS", "Avg Latency (ms)", "Error Rate"})
		for _, point := range results.TimeSeries {
			writer.Write([]string{
				point.Timestamp.Format(time.RFC3339),
				fmt.Sprintf("%.2f", point.RPS),
				fmt.Sprintf("%.2f", float64(point.AvgLatency.Microseconds())/1000),
				fmt.Sprintf("%.4f", point.ErrorRate),
			})
		}
	}

	fmt.Printf("\n📊 CSV报告已保存: %s\n", outputPath)
	return nil
}

// HTMLReporter HTML报告 (简化版)
type HTMLReporter struct{}

func (r *HTMLReporter) Generate(results *benchmark.Results, outputPath string) error {
	if outputPath == "" {
		outputPath = fmt.Sprintf("benchmark_report_%s.html", time.Now().Format("20060102_150405"))
	}

	html := r.generateHTML(results)

	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	fmt.Printf("\n🌐 HTML报告已保存: %s\n", outputPath)
	return nil
}

func (r *HTMLReporter) generateHTML(results *benchmark.Results) string {
	// 防止除以零
	successRate := 0.0
	if results.TotalRequests > 0 {
		successRate = float64(results.SuccessRequests) / float64(results.TotalRequests) * 100
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>HTTP Benchmark Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #4CAF50; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .metric { display: inline-block; width: 200px; margin: 10px; padding: 15px; background: #f9f9f9; border-left: 4px solid #4CAF50; }
        .metric-label { font-size: 12px; color: #666; }
        .metric-value { font-size: 24px; font-weight: bold; color: #333; }
        table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #4CAF50; color: white; }
        tr:hover { background-color: #f5f5f5; }
        .success { color: #4CAF50; }
        .error { color: #f44336; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 HTTP Benchmark Report</h1>
        <p>Generated: %s</p>
        
        <h2>📊 Summary</h2>
        <div class="metric">
            <div class="metric-label">Total Requests</div>
            <div class="metric-value">%d</div>
        </div>
        <div class="metric">
            <div class="metric-label">Success Rate</div>
            <div class="metric-value">%.2f%%</div>
        </div>
        <div class="metric">
            <div class="metric-label">Throughput</div>
            <div class="metric-value">%.2f req/s</div>
        </div>
        <div class="metric">
            <div class="metric-label">Avg Latency</div>
            <div class="metric-value">%v</div>
        </div>
        
        <h2>📈 Latency Percentiles</h2>
        <table>
            <tr><th>Percentile</th><th>Latency</th></tr>
            <tr><td>P50</td><td>%v</td></tr>
            <tr><td>P75</td><td>%v</td></tr>
            <tr><td>P90</td><td>%v</td></tr>
            <tr><td>P95</td><td>%v</td></tr>
            <tr><td>P99</td><td>%v</td></tr>
            <tr><td>P99.9</td><td>%v</td></tr>
        </table>
    </div>
</body>
</html>`,
		time.Now().Format("2006-01-02 15:04:05"),
		results.TotalRequests,
		successRate,
		results.Throughput,
		results.Latency.Mean,
		results.Latency.P50,
		results.Latency.P75,
		results.Latency.P90,
		results.Latency.P95,
		results.Latency.P99,
		results.Latency.P999,
	)
}
