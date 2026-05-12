package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	pkglogger "github.com/jiujuan/wukong/pkg/logger"
)

type WebSearchTool struct {
	client *http.Client
	logger *pkglogger.Logger
}

func NewWebSearchTool(client *http.Client, logger *pkglogger.Logger) *WebSearchTool {
	return &WebSearchTool{client: client, logger: logger}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string { return "联网搜索并返回结构化结果" }

func (t *WebSearchTool) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	query := readString(params, "query", "q", "keyword", "topic")
	if strings.TrimSpace(query) == "" {
		t.logger.Warn("[Tool] web_search invalid params: query is empty")
		return nil, fmt.Errorf("query is required")
	}
	t.logger.Info("[Tool] web_search start", "query", query)
	if t.client == nil {
		t.client = &http.Client{Timeout: 15 * time.Second}
	}
	api := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "GET", api, nil)
	if err != nil {
		t.logger.Error("[Tool] web_search build request failed", "error", err)
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		t.logger.Error("[Tool] web_search request failed", "error", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.logger.Error("[Tool] web_search read response failed", "error", err)
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.logger.Error("[Tool] web_search bad response", "status_code", resp.StatusCode)
		return nil, fmt.Errorf("search status=%d body=%s", resp.StatusCode, string(body))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.logger.Error("[Tool] web_search parse response failed", "error", err)
		return nil, err
	}
	related := make([]map[string]any, 0, 5)
	if raw, ok := payload["RelatedTopics"].([]any); ok {
		for _, item := range raw {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text := readString(obj, "Text")
			firstURL := readString(obj, "FirstURL")
			if strings.TrimSpace(text) == "" && strings.TrimSpace(firstURL) == "" {
				continue
			}
			related = append(related, map[string]any{
				"text":       text,
				"url":        firstURL,
				"source":     "duckduckgo",
				"confidence": "medium",
			})
			if len(related) >= 5 {
				break
			}
		}
	}
	result := map[string]any{
		"query":    query,
		"heading":  readString(payload, "Heading"),
		"abstract": readString(payload, "AbstractText"),
		"results":  related,
	}
	t.logger.Info("[Tool] web_search success", "query", query, "result_count", len(related))
	return result, nil
}
