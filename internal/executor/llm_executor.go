package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// LLMConfig holds the configuration for the LLM executor.
type LLMConfig struct {
	Provider string // "ollama", "openai", etc.
	Model    string
	Endpoint string
	APIKey   string // optional
	Timeout  time.Duration
}

// LLMExecutor executes tasks by calling a configured large language model.
type LLMExecutor struct {
	cfg        LLMConfig
	httpClient *http.Client
}

// NewLLMExecutor creates a new LLM executor with the given configuration.
func NewLLMExecutor(cfg LLMConfig) *LLMExecutor {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &LLMExecutor{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Execute sends the task to the LLM and returns the parsed result.
func (e *LLMExecutor) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	switch strings.ToLower(e.cfg.Provider) {
	case "ollama":
		return e.callOllama(ctx, task)
	case "openai":
		return e.callOpenAI(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", e.cfg.Provider)
	}
}

// callOllama invokes the Ollama generate API.
func (e *LLMExecutor) callOllama(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	prompt := e.buildPrompt(task)
	reqBody := map[string]interface{}{
		"model":  e.cfg.Model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.2,
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(e.cfg.Endpoint, "/") + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return e.parseResponse(ollamaResp.Response)
}

// callOpenAI invokes the OpenAI chat completions API.
func (e *LLMExecutor) callOpenAI(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	prompt := e.buildPrompt(task)
	reqBody := map[string]interface{}{
		"model": e.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are an AI agent executing tasks. Return a JSON object with the result."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(e.cfg.Endpoint, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	return e.parseResponse(openAIResp.Choices[0].Message.Content)
}

// buildPrompt constructs a prompt that describes the task and asks for a JSON result.
func (e *LLMExecutor) buildPrompt(task *awmv1.TaskAssignment) string {
	inputMap := task.Input.AsMap()
	inputJSON, _ := json.MarshalIndent(inputMap, "", "  ")
	return fmt.Sprintf(`Task: %s

Input:
%s

Please perform this task and return a JSON object with the result. Only output valid JSON.`, task.ActivityName, string(inputJSON))
}

// parseResponse attempts to extract a JSON object from the LLM's response.
func (e *LLMExecutor) parseResponse(raw string) (map[string]interface{}, error) {
	raw = strings.TrimSpace(raw)
	// Find JSON object boundaries
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || start > end {
		// Fallback: wrap the whole response as a text output
		return map[string]interface{}{"output": raw}, nil
	}
	jsonStr := raw[start : end+1]
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Fallback to text output
		return map[string]interface{}{"output": raw}, nil
	}
	return result, nil
}
