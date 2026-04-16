package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/enriquepascalin/awm-cli/internal/connector"
	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// ServiceExecutor executes automated tasks such as shell scripts, HTTP requests,
// named integration connectors, or external API calls.
type ServiceExecutor struct {
	httpClient *http.Client
	connectors *connector.Registry
}

// NewServiceExecutor creates a new service executor with default settings.
func NewServiceExecutor() *ServiceExecutor {
	return &ServiceExecutor{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		connectors: connector.NewRegistry(),
	}
}

// RegisterConnector adds a named connector to this executor.
// Call at startup before the agent begins consuming tasks.
func (s *ServiceExecutor) RegisterConnector(c connector.Connector) {
	s.connectors.Register(c)
}

// Execute inspects the task input to determine the action type and performs it.
//
// If the input contains a "connector" field that matches a registered connector
// name, execution is delegated to that connector.  The "action" field (if any)
// is passed as the connector action; all other input fields become its params.
//
// Generic action types (when no connector is specified):
//   - "http" / "webhook": performs an HTTP request
//   - "script" / "shell" / "command": executes a shell script
//   - default: falls back to HTTP if a "url" is present, otherwise script
func (s *ServiceExecutor) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	inputMap := task.Input.AsMap()

	// Named connector path.
	if name, _ := inputMap["connector"].(string); name != "" {
		conn, err := s.connectors.Get(name)
		if err != nil {
			return nil, err
		}
		action, _ := inputMap["action"].(string)
		// Build params: everything except the routing fields themselves.
		params := make(map[string]interface{}, len(inputMap))
		for k, v := range inputMap {
			if k != "connector" && k != "action" {
				params[k] = v
			}
		}
		return conn.Execute(ctx, action, params)
	}

	action, _ := inputMap["action"].(string)
	if action == "" {
		action, _ = inputMap["type"].(string)
	}

	switch strings.ToLower(action) {
	case "http", "webhook":
		return s.executeHTTP(ctx, inputMap)
	case "script", "shell", "command":
		return s.executeScript(ctx, inputMap)
	default:
		// Fallback: try to run as a shell command if a "command" field exists
		if cmd, ok := inputMap["command"].(string); ok {
			inputMap["script"] = cmd
			return s.executeScript(ctx, inputMap)
		}
		// Default to HTTP if URL is present
		if _, ok := inputMap["url"]; ok {
			return s.executeHTTP(ctx, inputMap)
		}
		return nil, fmt.Errorf("unknown action type; specify 'action' (http, script) in task input")
	}
}

// executeHTTP performs an HTTP request based on the task input.
// Expected input fields:
//   - url (string, required)
//   - method (string, default GET)
//   - headers (map[string]string)
//   - body (object or string)
//   - timeout (duration string, e.g., "10s")
func (s *ServiceExecutor) executeHTTP(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	url, ok := input["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url is required for HTTP action")
	}

	method := strings.ToUpper(getString(input, "method", "GET"))
	headers := getStringMap(input, "headers")
	timeout := getDuration(input, "timeout", 30*time.Second)

	var body io.Reader
	if bodyData, exists := input["body"]; exists {
		switch v := bodyData.(type) {
		case string:
			body = strings.NewReader(v)
		case map[string]interface{}:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("marshal body: %w", err)
			}
			body = bytes.NewReader(jsonBytes)
			if headers["Content-Type"] == "" {
				headers["Content-Type"] = "application/json"
			}
		default:
			body = strings.NewReader(fmt.Sprintf("%v", v))
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	result := map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
	}

	// Try to parse JSON response if content-type indicates
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var jsonBody map[string]interface{}
		if err := json.Unmarshal(respBody, &jsonBody); err == nil {
			result["json"] = jsonBody
		}
	}

	return result, nil
}

// executeScript runs a shell script provided in the task input.
// Expected input fields:
//   - script (string, required) — the script content
//   - interpreter (string, default "/bin/sh")
//   - timeout (duration string, e.g., "10s")
//   - env (map[string]string) — environment variables
func (s *ServiceExecutor) executeScript(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	script, ok := input["script"].(string)
	if !ok || script == "" {
		return nil, fmt.Errorf("script is required for script action")
	}

	interpreter := getString(input, "interpreter", "/bin/sh")
	timeout := getDuration(input, "timeout", 60*time.Second)
	envMap := getStringMap(input, "env")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, interpreter, "-c", script)
	cmd.Env = append(cmd.Env, "PATH=/usr/local/bin:/usr/bin:/bin")
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := map[string]interface{}{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": 0,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else {
			result["exit_code"] = -1
		}
		result["error"] = err.Error()
		return result, fmt.Errorf("script execution failed: %w", err)
	}
	return result, nil
}

// Helper functions

func getString(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultValue
}

func getStringMap(m map[string]interface{}, key string) map[string]string {
	if v, ok := m[key].(map[string]interface{}); ok {
		result := make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
		return result
	}
	if v, ok := m[key].(map[string]string); ok {
		return v
	}
	return make(map[string]string)
}

func getDuration(m map[string]interface{}, key string, defaultValue time.Duration) time.Duration {
	if v, ok := m[key].(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}
