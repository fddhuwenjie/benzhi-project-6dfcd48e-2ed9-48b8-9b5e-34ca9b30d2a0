package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type checkClient struct {
	baseURL string
	client  *http.Client
}

func newCheckClient(address string) *checkClient {
	return &checkClient{baseURL: "http://" + address, client: &http.Client{Timeout: 4 * time.Second}}
}

func (c *checkClient) request(ctx context.Context, method, path string, payload any, expectedStatus int, destination any) error {
	_, err := c.requestWithHeaders(ctx, method, path, payload, expectedStatus, destination, nil)
	return err
}

func (c *checkClient) requestWithHeaders(ctx context.Context, method, path string, payload any, expectedStatus int, destination any, headers map[string]string) (http.Header, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("编码自检请求: %w", err)
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("X-Correlation-ID", "self-check-correlation")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求 %s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != expectedStatus {
		return nil, fmt.Errorf("%s %s 返回 %d，期望 %d，响应 %s", method, path, response.StatusCode, expectedStatus, string(data))
	}
	if destination != nil && len(data) > 0 {
		if err := json.Unmarshal(data, destination); err != nil {
			return nil, fmt.Errorf("解析 %s %s 响应: %w", method, path, err)
		}
	}
	return response.Header.Clone(), nil
}
