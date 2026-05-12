package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

type HTTPTool struct {
	client *http.Client
	logger *pkglogger.Logger
}

func NewHTTPTool(client *http.Client, logger *pkglogger.Logger) *HTTPTool {
	return &HTTPTool{client: client, logger: logger}
}

func (t *HTTPTool) Name() string { return "http_request" }

func (t *HTTPTool) Description() string { return "发起外部 HTTP 请求" }

func (t *HTTPTool) ParameterSchema() []ParamSchema {
	return []ParamSchema{
		schemaItem("method", "string", false, "http method", "GET", "POST"),
		schemaItem("url", "string", true, "request url", nil, "https://example.com"),
		schemaItem("body", "string", false, "request body", nil, ""),
		schemaItem("headers", "object", false, "request headers", nil, map[string]any{"Authorization": "Bearer ..."}),
	}
}

func (t *HTTPTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	method := strings.ToUpper(strings.TrimSpace(readString(params, "method")))
	if method == "" {
		method = "GET"
	}
	rawURL := readString(params, "url")
	if strings.TrimSpace(rawURL) == "" {
		t.logger.Warn("[Tool] http_request invalid params: url is empty")
		return nil, fmt.Errorf("url is required")
	}
	t.logger.Info("[Tool] http_request start", "method", method, "url", rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.logger.Error("[Tool] http_request parse url failed", "url", rawURL, "error", err)
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		t.logger.Warn("[Tool] http_request unsupported scheme", "scheme", parsed.Scheme, "url", rawURL)
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}
	body := readString(params, "body")
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(body))
	if err != nil {
		t.logger.Error("[Tool] http_request build request failed", "error", err)
		return nil, err
	}
	if headers, ok := params["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	client := t.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		t.logger.Error("[Tool] http_request do request failed", "method", method, "url", rawURL, "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.logger.Error("[Tool] http_request read response failed", "method", method, "url", rawURL, "error", err)
		return nil, err
	}
	result := map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
	}
	t.logger.Info("[Tool] http_request success", "method", method, "url", rawURL, "status_code", resp.StatusCode)
	return result, nil
}
