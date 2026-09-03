// Package httpx provides a small HTTP helper with timeouts and retries.
package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with retry semantics for transient failures.
type Client struct {
	HTTP    *http.Client
	Retries int
}

// New returns a Client with a sensible timeout.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		HTTP: &http.Client{
			Timeout: timeout,
		},
		Retries: 3,
	}
}

// Do performs the request, retrying on 429/502/503/504.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	var lastErr error
	attempts := c.Retries
	if attempts < 1 {
		attempts = 1
	}
	for i := 0; i < attempts; i++ {
		cloned := req.Clone(ctx)
		resp, err := c.HTTP.Do(cloned)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("transient HTTP %d", resp.StatusCode)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		default:
			return resp, nil
		}
	}
	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

// GetJSON fetches URL and returns the response body bytes when status is 200.
func (c *Client) GetJSON(ctx context.Context, url string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
