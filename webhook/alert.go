// Package webhook delivers anomaly alerts to a configurable HTTP endpoint.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Alerter sends anomaly alerts to an HTTP webhook.
type Alerter struct {
	url    string
	client *http.Client
	logger *slog.Logger
}

// Alert is the payload sent to the webhook.
type Alert struct {
	EventID   string    `json:"event_id"`
	Partition int32     `json:"partition"`
	Offset    int64     `json:"offset"`
	Score     float64   `json:"score"`
	ZScore    float64   `json:"z_score"`
	Timestamp time.Time `json:"timestamp"`
}

// New creates an Alerter. If url is empty, Send is a no-op.
func New(url string, logger *slog.Logger) *Alerter {
	return &Alerter{
		url:    url,
		logger: logger,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Send POSTs the alert to the configured webhook endpoint.
// It is safe to call concurrently.
func (a *Alerter) Send(ctx context.Context, alert Alert) error {
	if a.url == "" {
		return nil // webhook not configured — log only mode
	}

	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook POST: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	a.logger.Debug("alert delivered", "event_id", alert.EventID, "status", resp.StatusCode)
	return nil
}
