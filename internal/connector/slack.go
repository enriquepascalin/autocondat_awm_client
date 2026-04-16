package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SlackConfig holds the configuration for the Slack connector.
type SlackConfig struct {
	BotToken   string        // xoxb-... bot token
	DefaultChannel string    // fallback channel when task params omit one
	Timeout    time.Duration
}

// SlackConnector sends messages and interacts with Slack via the Web API.
// Supported actions:
//   - "post-message" (default): posts a message to a channel
//   - "post-ephemeral":          posts an ephemeral message visible only to one user
type SlackConnector struct {
	cfg        SlackConfig
	httpClient *http.Client
}

// NewSlackConnector creates a configured Slack connector.
func NewSlackConnector(cfg SlackConfig) *SlackConnector {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &SlackConnector{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (s *SlackConnector) Name() string { return "slack" }

// Execute dispatches to the appropriate Slack action.
func (s *SlackConnector) Execute(ctx context.Context, action string, params map[string]interface{}) (map[string]interface{}, error) {
	switch action {
	case "", "post-message":
		return s.postMessage(ctx, params)
	case "post-ephemeral":
		return s.postEphemeral(ctx, params)
	default:
		return nil, fmt.Errorf("slack connector: unknown action %q", action)
	}
}

func (s *SlackConnector) postMessage(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	channel := stringParam(params, "channel", s.cfg.DefaultChannel)
	if channel == "" {
		return nil, fmt.Errorf("slack post-message: channel is required")
	}
	text := stringParam(params, "text", "")
	if text == "" {
		return nil, fmt.Errorf("slack post-message: text is required")
	}

	body := map[string]interface{}{"channel": channel, "text": text}
	// Optional: pass a blocks array for rich layout.
	if blocks, ok := params["blocks"]; ok {
		body["blocks"] = blocks
	}

	return s.callAPI(ctx, "https://slack.com/api/chat.postMessage", body)
}

func (s *SlackConnector) postEphemeral(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	channel := stringParam(params, "channel", s.cfg.DefaultChannel)
	user := stringParam(params, "user", "")
	text := stringParam(params, "text", "")
	if channel == "" || user == "" || text == "" {
		return nil, fmt.Errorf("slack post-ephemeral: channel, user, and text are required")
	}
	body := map[string]interface{}{"channel": channel, "user": user, "text": text}
	return s.callAPI(ctx, "https://slack.com/api/chat.postEphemeral", body)
}

func (s *SlackConnector) callAPI(ctx context.Context, url string, body map[string]interface{}) (map[string]interface{}, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("slack: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("slack: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.cfg.BotToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack: http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("slack: decode response: %w", err)
	}

	if ok, _ := result["ok"].(bool); !ok {
		errCode, _ := result["error"].(string)
		return nil, fmt.Errorf("slack API error: %s", errCode)
	}

	return result, nil
}

func stringParam(params map[string]interface{}, key, defaultVal string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}
