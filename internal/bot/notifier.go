package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ExhaustedNotifier interface {
	NotifyExhausted(ctx context.Context, record DispatchRecord) error
}

type WebhookNotifier struct {
	URL    string
	Client *http.Client
}

type WebhookPayload struct {
	Event       string        `json:"event"`
	BoardItemID string        `json:"board_item_id"`
	Attempt     int           `json:"attempt"`
	MaxRetries  int           `json:"max_retries"`
	State       DispatchState `json:"state"`
	BoardItem   BoardItem     `json:"board_item"`
	Timestamp   time.Time     `json:"timestamp"`
}

func (n WebhookNotifier) NotifyExhausted(ctx context.Context, record DispatchRecord) error {
	if n.URL == "" {
		return nil
	}
	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	payload := WebhookPayload{
		Event:       "renkin.bot.dispatch.exhausted",
		BoardItemID: record.BoardItemID,
		Attempt:     record.Attempt,
		MaxRetries:  record.MaxRetries,
		State:       record.State,
		BoardItem:   record.BoardItem,
		Timestamp:   time.Now().UTC(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}
