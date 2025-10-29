package template

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"text/template"
	"time"

	"httpbench/pkg/config"
)

// Engine 模板引擎
type Engine struct {
	config    config.TemplateConfig
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// New 创建模板引擎
func New(cfg config.TemplateConfig) *Engine {
	e := &Engine{
		config:    cfg,
		templates: make(map[string]*template.Template),
		funcMap:   createFuncMap(),
	}

	return e
}

// Render 渲染模板
func (e *Engine) Render(templateStr string, vars map[string]interface{}) (string, error) {
	if !e.config.Enabled {
		return templateStr, nil
	}

	// 合并配置的变量
	allVars := make(map[string]interface{})
	for k, v := range e.config.Variables {
		allVars[k] = v
	}
	for k, v := range vars {
		allVars[k] = v
	}

	// 创建一个新的函数映射,包含原有函数和变量函数
	funcMap := make(template.FuncMap)
	for k, v := range e.funcMap {
		funcMap[k] = v
	}

	// 将变量也注册为无参数函数,这样既支持 {{.var}} 也支持 {{var}}
	for k, v := range allVars {
		value := v // 捕获变量
		funcMap[k] = func() interface{} {
			return value
		}
	}

	// 创建并解析模板
	tmpl, err := template.New("request").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	// 执行模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, allVars); err != nil {
		return "", fmt.Errorf("执行模板失败: %w", err)
	}

	return buf.String(), nil
}

// RenderBytes 渲染模板为字节数组
func (e *Engine) RenderBytes(templateStr string, vars map[string]interface{}) ([]byte, error) {
	result, err := e.Render(templateStr, vars)
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

// createFuncMap 创建模板函数映射
func createFuncMap() template.FuncMap {
	return template.FuncMap{
		// 随机函数
		"random_int": func(min, max int) int {
			return rand.Intn(max-min+1) + min
		},
		"random_string": func(length int) string {
			const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			b := make([]byte, length)
			for i := range b {
				b[i] = charset[rand.Intn(len(charset))]
			}
			return string(b)
		},
		"random_uuid": func() string {
			return generateUUID()
		},

		// 时间函数
		"timestamp": func() int64 {
			return time.Now().Unix()
		},
		"timestamp_ms": func() int64 {
			return time.Now().UnixMilli()
		},
		"timestamp_ns": func() int64 {
			return time.Now().UnixNano()
		},
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
		"date": func(format string) string {
			return time.Now().Format(format)
		},

		// 字符串函数
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"trim": func(s string) string {
			return strings.TrimSpace(s)
		},
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"substr": func(s string, start, length int) string {
			if start < 0 || start >= len(s) {
				return ""
			}
			end := start + length
			if end > len(s) {
				end = len(s)
			}
			return s[start:end]
		},

		// 数学函数
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},

		// 格式化函数
		"printf": func(format string, args ...interface{}) string {
			return fmt.Sprintf(format, args...)
		},
		"json": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},

		// 序列函数
		"seq": func(start, end int) []int {
			result := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
		"range": func(n int) []int {
			result := make([]int, n)
			for i := 0; i < n; i++ {
				result[i] = i
			}
			return result
		},

		// 条件函数
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},
		"ternary": func(condition bool, trueVal, falseVal interface{}) interface{} {
			if condition {
				return trueVal
			}
			return falseVal
		},
	}
}

// generateUUID 生成简单的UUID
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// 内置变量提取器
type VariableExtractor struct {
	workerID  int
	requestID int64
}

// NewVariableExtractor 创建变量提取器
func NewVariableExtractor(workerID int) *VariableExtractor {
	return &VariableExtractor{
		workerID:  workerID,
		requestID: 0,
	}
}

// Extract 提取变量
func (ve *VariableExtractor) Extract() map[string]interface{} {
	ve.requestID++

	return map[string]interface{}{
		"worker_id":    ve.workerID,
		"request_id":   ve.requestID,
		"timestamp":    time.Now().Unix(),
		"timestamp_ms": time.Now().UnixMilli(),
		"random_int":   rand.Intn(1000000),
		"uuid":         generateUUID(),
	}
}

// TemplateBuilder 模板构建器
type TemplateBuilder struct {
	parts []string
}

// NewTemplateBuilder 创建模板构建器
func NewTemplateBuilder() *TemplateBuilder {
	return &TemplateBuilder{
		parts: make([]string, 0),
	}
}

// AddText 添加文本
func (tb *TemplateBuilder) AddText(text string) *TemplateBuilder {
	tb.parts = append(tb.parts, text)
	return tb
}

// AddVariable 添加变量
func (tb *TemplateBuilder) AddVariable(name string) *TemplateBuilder {
	tb.parts = append(tb.parts, "{{."+name+"}}")
	return tb
}

// AddFunction 添加函数调用
func (tb *TemplateBuilder) AddFunction(funcName string, args ...string) *TemplateBuilder {
	argsStr := strings.Join(args, " ")
	if argsStr != "" {
		tb.parts = append(tb.parts, "{{"+funcName+" "+argsStr+"}}")
	} else {
		tb.parts = append(tb.parts, "{{"+funcName+"}}")
	}
	return tb
}

// AddRandomInt 添加随机整数
func (tb *TemplateBuilder) AddRandomInt(min, max int) *TemplateBuilder {
	tb.parts = append(tb.parts, fmt.Sprintf("{{random_int %d %d}}", min, max))
	return tb
}

// AddRandomString 添加随机字符串
func (tb *TemplateBuilder) AddRandomString(length int) *TemplateBuilder {
	tb.parts = append(tb.parts, fmt.Sprintf("{{random_string %d}}", length))
	return tb
}

// AddTimestamp 添加时间戳
func (tb *TemplateBuilder) AddTimestamp() *TemplateBuilder {
	tb.parts = append(tb.parts, "{{timestamp}}")
	return tb
}

// AddUUID 添加UUID
func (tb *TemplateBuilder) AddUUID() *TemplateBuilder {
	tb.parts = append(tb.parts, "{{random_uuid}}")
	return tb
}

// Build 构建模板字符串
func (tb *TemplateBuilder) Build() string {
	return strings.Join(tb.parts, "")
}

// ParseInt 解析整数
func ParseInt(s string, defaultValue int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultValue
}

// ParseBool 解析布尔值
func ParseBool(s string, defaultValue bool) bool {
	if v, err := strconv.ParseBool(s); err == nil {
		return v
	}
	return defaultValue
}
