package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// JiraConfig holds the configuration for the Jira connector.
type JiraConfig struct {
	BaseURL  string        // e.g. "https://acme.atlassian.net"
	Email    string        // account email for basic auth
	APIToken string        // Jira API token
	Timeout  time.Duration
}

// JiraConnector creates, updates, and transitions Jira issues.
// Supported actions:
//   - "create-issue":     creates a new issue
//   - "update-issue":     updates fields on an existing issue
//   - "transition-issue": moves an issue to a new status
//   - "add-comment":      adds a comment to an issue
//   - "get-issue":        retrieves issue details
type JiraConnector struct {
	cfg        JiraConfig
	httpClient *http.Client
}

// NewJiraConnector creates a configured Jira connector.
func NewJiraConnector(cfg JiraConfig) *JiraConnector {
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &JiraConnector{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (j *JiraConnector) Name() string { return "jira" }

// Execute dispatches to the appropriate Jira action.
func (j *JiraConnector) Execute(ctx context.Context, action string, params map[string]interface{}) (map[string]interface{}, error) {
	switch action {
	case "create-issue":
		return j.createIssue(ctx, params)
	case "update-issue":
		return j.updateIssue(ctx, params)
	case "transition-issue":
		return j.transitionIssue(ctx, params)
	case "add-comment":
		return j.addComment(ctx, params)
	case "get-issue", "":
		return j.getIssue(ctx, params)
	default:
		return nil, fmt.Errorf("jira connector: unknown action %q", action)
	}
}

func (j *JiraConnector) createIssue(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	project := stringParam(params, "project", "")
	summary := stringParam(params, "summary", "")
	issueType := stringParam(params, "issue_type", "Task")
	if project == "" || summary == "" {
		return nil, fmt.Errorf("jira create-issue: project and summary are required")
	}

	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":   map[string]string{"key": project},
			"summary":   summary,
			"issuetype": map[string]string{"name": issueType},
		},
	}
	if desc := stringParam(params, "description", ""); desc != "" {
		body["fields"].(map[string]interface{})["description"] = desc
	}

	return j.request(ctx, http.MethodPost, "/rest/api/3/issue", body)
}

func (j *JiraConnector) updateIssue(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	issueKey := stringParam(params, "issue_key", "")
	if issueKey == "" {
		return nil, fmt.Errorf("jira update-issue: issue_key is required")
	}
	fields, ok := params["fields"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("jira update-issue: fields map is required")
	}
	_, err := j.request(ctx, http.MethodPut, "/rest/api/3/issue/"+issueKey, map[string]interface{}{"fields": fields})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"updated": issueKey}, nil
}

func (j *JiraConnector) transitionIssue(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	issueKey := stringParam(params, "issue_key", "")
	transitionID := stringParam(params, "transition_id", "")
	if issueKey == "" || transitionID == "" {
		return nil, fmt.Errorf("jira transition-issue: issue_key and transition_id are required")
	}
	body := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	_, err := j.request(ctx, http.MethodPost, "/rest/api/3/issue/"+issueKey+"/transitions", body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"transitioned": issueKey, "transition_id": transitionID}, nil
}

func (j *JiraConnector) addComment(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	issueKey := stringParam(params, "issue_key", "")
	text := stringParam(params, "text", "")
	if issueKey == "" || text == "" {
		return nil, fmt.Errorf("jira add-comment: issue_key and text are required")
	}
	body := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{"type": "paragraph", "content": []map[string]interface{}{
					{"type": "text", "text": text},
				}},
			},
		},
	}
	return j.request(ctx, http.MethodPost, "/rest/api/3/issue/"+issueKey+"/comment", body)
}

func (j *JiraConnector) getIssue(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	issueKey := stringParam(params, "issue_key", "")
	if issueKey == "" {
		return nil, fmt.Errorf("jira get-issue: issue_key is required")
	}
	return j.request(ctx, http.MethodGet, "/rest/api/3/issue/"+issueKey, nil)
}

// request performs an authenticated REST call against the Jira API.
func (j *JiraConnector) request(ctx context.Context, method, path string, body map[string]interface{}) (map[string]interface{}, error) {
	url := strings.TrimSuffix(j.cfg.BaseURL, "/") + path

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("jira: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("jira: create request: %w", err)
	}
	req.SetBasicAuth(j.cfg.Email, j.cfg.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jira: API returned %d: %s", resp.StatusCode, string(respBytes))
	}

	// 204 No Content (successful transitions, updates)
	if resp.StatusCode == http.StatusNoContent || len(respBytes) == 0 {
		return map[string]interface{}{"status_code": resp.StatusCode}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("jira: decode response: %w", err)
	}
	return result, nil
}
