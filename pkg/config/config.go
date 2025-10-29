package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 主配置结构
type Config struct {
	Target       TargetConfig       `yaml:"target"`
	Load         LoadConfig         `yaml:"load"`
	Protocol     ProtocolConfig     `yaml:"protocol"`
	Request      RequestConfig      `yaml:"request"`
	Validation   ValidationConfig   `yaml:"validation"`
	TLS          TLSConfig          `yaml:"tls"`
	Output       OutputConfig       `yaml:"output"`
	Distributed  DistributedConfig  `yaml:"distributed"`
}

// TargetConfig 目标配置
type TargetConfig struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Timeout time.Duration     `yaml:"timeout"`
}

// LoadConfig 负载配置
type LoadConfig struct {
	Concurrency   int           `yaml:"concurrency"`
	Duration      time.Duration `yaml:"duration"`
	TotalRequests int           `yaml:"total_requests"`
	RateLimit     int           `yaml:"rate_limit"`
	
	// 负载模式
	LoadPattern   LoadPattern   `yaml:"load_pattern"`
	RampUp        RampUpConfig  `yaml:"ramp_up"`
	BurstMode     BurstConfig   `yaml:"burst_mode"`
}

// LoadPattern 负载模式
type LoadPattern string

const (
	LoadPatternConstant  LoadPattern = "constant"   // 恒定负载
	LoadPatternRampUp    LoadPattern = "ramp_up"    // 渐进式
	LoadPatternBurst     LoadPattern = "burst"      // 突发式
)

// RampUpConfig 渐进式负载配置
type RampUpConfig struct {
	Enabled       bool          `yaml:"enabled"`
	StartConcurrency int        `yaml:"start_concurrency"`
	EndConcurrency   int        `yaml:"end_concurrency"`
	Duration      time.Duration `yaml:"duration"`
	Steps         int           `yaml:"steps"`
}

// BurstConfig 突发负载配置
type BurstConfig struct {
	Enabled       bool          `yaml:"enabled"`
	BaseConcurrency int         `yaml:"base_concurrency"`
	BurstConcurrency int        `yaml:"burst_concurrency"`
	BurstDuration time.Duration `yaml:"burst_duration"`
	BurstInterval time.Duration `yaml:"burst_interval"`
}

// ProtocolConfig 协议配置
type ProtocolConfig struct {
	HTTP2Enabled bool `yaml:"http2_enabled"`
	HTTP3Enabled bool `yaml:"http3_enabled"`
	
	// HTTP/2 特定配置
	HTTP2Config HTTP2Config `yaml:"http2"`
	
	// HTTP/3 特定配置
	HTTP3Config HTTP3Config `yaml:"http3"`
	
	KeepAlive   bool          `yaml:"keep_alive"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
}

// HTTP2Config HTTP/2配置
type HTTP2Config struct {
	MaxConcurrentStreams uint32 `yaml:"max_concurrent_streams"`
	InitialWindowSize    uint32 `yaml:"initial_window_size"`
	MaxFrameSize         uint32 `yaml:"max_frame_size"`
}

// HTTP3Config HTTP/3配置
type HTTP3Config struct {
	MaxStreamBuffer int64         `yaml:"max_stream_buffer"`
	HandshakeTimeout time.Duration `yaml:"handshake_timeout"`
}

// RequestConfig 请求配置
type RequestConfig struct {
	Headers      map[string]string `yaml:"headers"`
	Cookies      []Cookie          `yaml:"cookies"`
	Template     TemplateConfig    `yaml:"template"`
	
	// 动态内容
	DynamicBody  bool   `yaml:"dynamic_body"`
	BodyTemplate string `yaml:"body_template"`
}

// Cookie Cookie配置
type Cookie struct {
	Name     string    `yaml:"name"`
	Value    string    `yaml:"value"`
	Domain   string    `yaml:"domain"`
	Path     string    `yaml:"path"`
	Expires  time.Time `yaml:"expires"`
	Secure   bool      `yaml:"secure"`
	HttpOnly bool      `yaml:"http_only"`
}

// TemplateConfig 模板配置
type TemplateConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Variables map[string]string `yaml:"variables"`
	Functions []string          `yaml:"functions"`
}

// ValidationConfig 验证配置
type ValidationConfig struct {
	StatusCodes       []int             `yaml:"status_codes"`
	ContentPatterns   []string          `yaml:"content_patterns"`
	ResponseTimeMax   time.Duration     `yaml:"response_time_max"`
	HeaderValidation  map[string]string `yaml:"header_validation"`
	BodyValidation    BodyValidation    `yaml:"body_validation"`
}

// BodyValidation 响应体验证
type BodyValidation struct {
	MinSize    int      `yaml:"min_size"`
	MaxSize    int      `yaml:"max_size"`
	Contains   []string `yaml:"contains"`
	NotContains []string `yaml:"not_contains"`
	JSONSchema string   `yaml:"json_schema"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled            bool     `yaml:"enabled"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
	MinVersion         string   `yaml:"min_version"`
	MaxVersion         string   `yaml:"max_version"`
	CipherSuites       []string `yaml:"cipher_suites"`
	
	// 证书配置
	ClientCertFile string `yaml:"client_cert_file"`
	ClientKeyFile  string `yaml:"client_key_file"`
	CAFile         string `yaml:"ca_file"`
	
	// 双向认证
	MutualTLS      bool   `yaml:"mutual_tls"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	Format     string `yaml:"format"`      // console, json, csv
	ReportFile string `yaml:"report_file"`
	
	// 实时监控
	RealtimeMonitor bool   `yaml:"realtime_monitor"`
	MonitorInterval time.Duration `yaml:"monitor_interval"`
	
	// 详细程度
	Verbose bool `yaml:"verbose"`
	Debug   bool `yaml:"debug"`
}

// DistributedConfig 分布式配置
type DistributedConfig struct {
	Enabled       bool     `yaml:"enabled"`
	WorkerMode    bool     `yaml:"worker_mode"`
	MasterAddress string   `yaml:"master_address"`
	WorkerAddresses []string `yaml:"worker_addresses"`
	
	// 同步配置
	SyncInterval  time.Duration `yaml:"sync_interval"`
	GRPCPort      int           `yaml:"grpc_port"`
}

// NewDefault 创建默认配置
func NewDefault() *Config {
	return &Config{
		Target: TargetConfig{
			Method:  "GET",
			Headers: make(map[string]string),
			Timeout: 30 * time.Second,
		},
		Load: LoadConfig{
			Concurrency:   10,
			Duration:      10 * time.Second,
			TotalRequests: 0,
			RateLimit:     0,
			LoadPattern:   LoadPatternConstant,
		},
		Protocol: ProtocolConfig{
			HTTP2Enabled: false,
			HTTP3Enabled: false,
			KeepAlive:    true,
			IdleTimeout:  90 * time.Second,
			HTTP2Config: HTTP2Config{
				MaxConcurrentStreams: 100,
				InitialWindowSize:    65535,
				MaxFrameSize:         16384,
			},
			HTTP3Config: HTTP3Config{
				MaxStreamBuffer:  1 << 20, // 1MB
				HandshakeTimeout: 10 * time.Second,
			},
		},
		Request: RequestConfig{
			Headers:     make(map[string]string),
			Cookies:     []Cookie{},
			DynamicBody: false,
		},
		Validation: ValidationConfig{
			StatusCodes:     []int{200},
			ContentPatterns: []string{},
			ResponseTimeMax: 5 * time.Second,
		},
		TLS: TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: false,
			MinVersion:         "TLS1.2",
			MaxVersion:         "TLS1.3",
		},
		Output: OutputConfig{
			Format:          "console",
			RealtimeMonitor: false,
			MonitorInterval: 1 * time.Second,
			Verbose:         false,
			Debug:           false,
		},
		Distributed: DistributedConfig{
			Enabled:      false,
			WorkerMode:   false,
			SyncInterval: 1 * time.Second,
			GRPCPort:     50051,
		},
	}
}

// LoadFromFile 从文件加载配置
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := NewDefault()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Target.URL == "" {
		return fmt.Errorf("目标URL不能为空")
	}

	if c.Load.Concurrency <= 0 {
		return fmt.Errorf("并发数必须大于0")
	}

	if c.Load.Duration <= 0 && c.Load.TotalRequests <= 0 {
		return fmt.Errorf("必须指定持续时间或总请求数")
	}

	if c.Protocol.HTTP2Enabled && c.Protocol.HTTP3Enabled {
		return fmt.Errorf("不能同时启用HTTP/2和HTTP/3")
	}

	if c.Distributed.Enabled && !c.Distributed.WorkerMode && len(c.Distributed.WorkerAddresses) == 0 {
		return fmt.Errorf("分布式模式需要至少一个工作节点地址")
	}

	return nil
}

// SaveToFile 保存配置到文件
func (c *Config) SaveToFile(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}
