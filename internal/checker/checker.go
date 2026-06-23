// Package checker — клиент xray-checker. Тянет публичный список прокси с их
// последним результатом проверки (online/latency). Сам ничего не тестирует —
// это делает xray-checker (см. ПЛАН §2).
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Proxy — одна запись из /api/v1/public/proxies.
type Proxy struct {
	StableID  string `json:"stableId"`
	Name      string `json:"name"`
	Online    bool   `json:"online"`
	LatencyMs int    `json:"latencyMs"`
}

type Client struct {
	base string
	http *http.Client
}

// Config — подмножество /api/v1/config чекера (read-only): нам нужен реальный
// интервал проверок, чтобы страница/опрос шли в его такт.
type Config struct {
	CheckInterval int `json:"checkInterval"`
}

type configEnvelope struct {
	Data Config `json:"data"`
}

// FetchConfig читает интервал проверок у чекера. Эндпоинт /api/v1/config открыт,
// когда METRICS_PROTECTED=false (дефолт); иначе вернёт ошибку и мы используем
// фолбэк POLL_INTERVAL.
func (c *Client) FetchConfig(ctx context.Context) (Config, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/v1/config", nil)
	if err != nil {
		return Config{}, err
	}
	req.Header.Set("User-Agent", "xray-status/1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return Config{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Config{}, fmt.Errorf("checker config status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Config{}, err
	}
	var env configEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	return env.Data, nil
}

func New(baseURL string) *Client {
	return &Client{
		base: baseURL,
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// envelope покрывает оба варианта ответа: {"data":[...]} и просто [...].
type envelope struct {
	Data []Proxy `json:"data"`
}

// Fetch повторяет fetch_proxies: 3 попытки с паузой 4с между ними.
func (c *Client) Fetch(ctx context.Context) ([]Proxy, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		proxies, err := c.fetchOnce(ctx)
		if err == nil {
			return proxies, nil
		}
		lastErr = err
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(4 * time.Second):
			}
		}
	}
	return nil, lastErr
}

func (c *Client) fetchOnce(ctx context.Context) ([]Proxy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/api/v1/public/proxies", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "xray-status/1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checker status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	// Сначала пробуем {"data":[...]}, затем голый массив.
	var env envelope
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil {
		return env.Data, nil
	}
	var arr []Proxy
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, fmt.Errorf("decode proxies: %w", err)
	}
	return arr, nil
}
