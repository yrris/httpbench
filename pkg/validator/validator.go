package validator

import (
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"httpbench/pkg/config"
)

// Validator 响应验证器
type Validator struct {
	config           config.ValidationConfig
	contentPatterns  []*regexp.Regexp
	statusCodeMap    map[int]bool
}

// ValidationError 验证错误
type ValidationError struct {
	Type    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// New 创建验证器
func New(cfg config.ValidationConfig) *Validator {
	v := &Validator{
		config:        cfg,
		statusCodeMap: make(map[int]bool),
	}

	// 编译内容匹配模式
	v.contentPatterns = make([]*regexp.Regexp, 0, len(cfg.ContentPatterns))
	for _, pattern := range cfg.ContentPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			v.contentPatterns = append(v.contentPatterns, re)
		}
	}

	// 构建状态码映射
	for _, code := range cfg.StatusCodes {
		v.statusCodeMap[code] = true
	}

	return v
}

// Validate 验证响应
func (v *Validator) Validate(resp *http.Response, body []byte) error {
	// 验证状态码
	if err := v.validateStatusCode(resp.StatusCode); err != nil {
		return err
	}

	// 验证响应头
	if err := v.validateHeaders(resp.Header); err != nil {
		return err
	}

	// 验证响应体
	if err := v.validateBody(body); err != nil {
		return err
	}

	return nil
}

// ValidateWithLatency 验证响应和延迟
func (v *Validator) ValidateWithLatency(resp *http.Response, body []byte, latency time.Duration) error {
	// 先验证响应
	if err := v.Validate(resp, body); err != nil {
		return err
	}

	// 验证响应时间
	if v.config.ResponseTimeMax > 0 && latency > v.config.ResponseTimeMax {
		return &ValidationError{
			Type:    "response_time",
			Message: fmt.Sprintf("响应时间 %v 超过阈值 %v", latency, v.config.ResponseTimeMax),
		}
	}

	return nil
}

// validateStatusCode 验证状态码
func (v *Validator) validateStatusCode(code int) error {
	if len(v.statusCodeMap) == 0 {
		return nil // 未配置状态码验证
	}

	if !v.statusCodeMap[code] {
		return &ValidationError{
			Type:    "status_code",
			Message: fmt.Sprintf("状态码 %d 不在允许列表中: %v", code, v.config.StatusCodes),
		}
	}

	return nil
}

// validateHeaders 验证响应头
func (v *Validator) validateHeaders(headers http.Header) error {
	for key, expectedValue := range v.config.HeaderValidation {
		actualValue := headers.Get(key)
		if actualValue != expectedValue {
			return &ValidationError{
				Type: "header",
				Message: fmt.Sprintf("响应头 %s 不匹配: 期望 %s, 实际 %s",
					key, expectedValue, actualValue),
			}
		}
	}

	return nil
}

// validateBody 验证响应体
func (v *Validator) validateBody(body []byte) error {
	bodyLen := len(body)

	// 验证大小
	if v.config.BodyValidation.MinSize > 0 && bodyLen < v.config.BodyValidation.MinSize {
		return &ValidationError{
			Type:    "body_size",
			Message: fmt.Sprintf("响应体大小 %d 小于最小值 %d", bodyLen, v.config.BodyValidation.MinSize),
		}
	}

	if v.config.BodyValidation.MaxSize > 0 && bodyLen > v.config.BodyValidation.MaxSize {
		return &ValidationError{
			Type:    "body_size",
			Message: fmt.Sprintf("响应体大小 %d 大于最大值 %d", bodyLen, v.config.BodyValidation.MaxSize),
		}
	}

	// 验证必须包含的内容
	for _, content := range v.config.BodyValidation.Contains {
		if !bytes.Contains(body, []byte(content)) {
			return &ValidationError{
				Type:    "body_content",
				Message: fmt.Sprintf("响应体不包含必需内容: %s", content),
			}
		}
	}

	// 验证不应包含的内容
	for _, content := range v.config.BodyValidation.NotContains {
		if bytes.Contains(body, []byte(content)) {
			return &ValidationError{
				Type:    "body_content",
				Message: fmt.Sprintf("响应体包含禁止内容: %s", content),
			}
		}
	}

	// 验证正则表达式模式
	for _, pattern := range v.contentPatterns {
		if !pattern.Match(body) {
			return &ValidationError{
				Type:    "content_pattern",
				Message: fmt.Sprintf("响应体不匹配模式: %s", pattern.String()),
			}
		}
	}

	return nil
}

// IsValid 快速检查是否有效
func (v *Validator) IsValid(resp *http.Response) bool {
	if len(v.statusCodeMap) == 0 {
		return true
	}
	return v.statusCodeMap[resp.StatusCode]
}

// GetExpectedStatusCodes 获取期望的状态码列表
func (v *Validator) GetExpectedStatusCodes() []int {
	return v.config.StatusCodes
}
