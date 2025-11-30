package heartbeat

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Client sends heartbeats to Obsrvr Console for billing tracking
type Client struct {
	consoleURL    string
	pipelineID    string
	sessionID     string
	webhookSecret string
	httpClient    *http.Client
	ledgerCount   int64 // Atomic counter
}

// HeartbeatPayload is sent to console API
type HeartbeatPayload struct {
	SessionID        string                 `json:"session_id"`
	LedgersProcessed int64                  `json:"ledgers_processed"`
	CheckpointData   map[string]interface{} `json:"checkpoint_data,omitempty"`
	Timestamp        string                 `json:"timestamp"`
}

// NewClient creates a new console heartbeat client
func NewClient(consoleURL, pipelineID, sessionID, webhookSecret string) *Client {
	return &Client{
		consoleURL:    consoleURL,
		pipelineID:    pipelineID,
		sessionID:     sessionID,
		webhookSecret: webhookSecret,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		ledgerCount:   0,
	}
}

// SetLedgerCount atomically updates the ledger counter
func (c *Client) SetLedgerCount(count int64) {
	atomic.StoreInt64(&c.ledgerCount, count)
}

// GetLedgerCount atomically reads the ledger counter
func (c *Client) GetLedgerCount() int64 {
	return atomic.LoadInt64(&c.ledgerCount)
}

// SendHeartbeat sends a heartbeat to the console
func (c *Client) SendHeartbeat(ctx context.Context, checkpointData map[string]interface{}) error {
	payload := HeartbeatPayload{
		SessionID:        c.sessionID,
		LedgersProcessed: c.GetLedgerCount(),
		CheckpointData:   checkpointData,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Generate HMAC signature
	signature := c.generateSignature(payloadBytes)

	// Create request
	url := fmt.Sprintf("%s/flow/api/pipelines/%s/heartbeat/", c.consoleURL, c.pipelineID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pipeline-Signature", signature)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}

// generateSignature creates HMAC-SHA256 signature
func (c *Client) generateSignature(payload []byte) string {
	h := hmac.New(sha256.New, []byte(c.webhookSecret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// StartHeartbeatLoop runs in a goroutine, sending heartbeats at intervals
func (c *Client) StartHeartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.SendHeartbeat(ctx, nil); err != nil {
				// Log but don't crash - next heartbeat in N minutes
				fmt.Printf("Heartbeat error: %v\n", err)
			} else {
				fmt.Printf("Heartbeat sent: %d ledgers processed\n", c.GetLedgerCount())
			}
		}
	}
}
