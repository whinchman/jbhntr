// Package notifier sends push notifications via ntfy.sh.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/whinchman/jobhuntr/internal/models"
)

// Notifier is the interface for sending job notifications.
type Notifier interface {
	Notify(ctx context.Context, job models.Job, topic string) error
}

// ntfyPayload is the JSON body sent to ntfy.sh.
type ntfyPayload struct {
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Click    string   `json:"click"`
	Priority int      `json:"priority"`
	Tags     []string `json:"tags"`
}

// NtfyNotifier sends push notifications via ntfy.sh.
type NtfyNotifier struct {
	server  string
	baseURL string
	client  *http.Client
}

// NewNtfyNotifier creates a NtfyNotifier.
// server is the ntfy server base URL (e.g. "https://ntfy.sh"),
// baseURL is the jobhuntr web dashboard base URL used to build the click link.
func NewNtfyNotifier(server, baseURL string) *NtfyNotifier {
	return &NtfyNotifier{
		server:  server,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a push notification for the given job to the given ntfy topic.
// If topic is empty, Notify is a no-op and returns nil.
func (n *NtfyNotifier) Notify(ctx context.Context, job models.Job, topic string) error {
	if topic == "" {
		return nil
	}

	msg := job.Location
	if job.Salary != "" {
		msg += " · " + job.Salary
	}

	payload := ntfyPayload{
		Title:    fmt.Sprintf("New: %s at %s", job.Title, job.Company),
		Message:  msg,
		Click:    fmt.Sprintf("%s/jobs/%d", n.baseURL, job.ID),
		Priority: 3,
		Tags:     []string{"briefcase"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifier: marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s", n.server, topic)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifier: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("notifier: http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notifier: ntfy returned status %d", resp.StatusCode)
	}
	return nil
}
