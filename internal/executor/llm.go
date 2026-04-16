package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
	"github.com/enriquepascalin/awm-cli/internal/config"
)

type LLMExecutor struct {
	cfg config.AIConfig
}

func NewLLMExecutor(cfg config.AIConfig) *LLMExecutor {
	return &LLMExecutor{cfg: cfg}
}

func (l *LLMExecutor) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	prompt := fmt.Sprintf("You are an AI agent. Perform the following task: %s\nInput: %v\nProvide a JSON result.", task.ActivityName, task.Input.AsMap())
	var response string
	var err error
	switch l.cfg.Provider {
	case "ollama":
		response, err = l.callOllama(ctx, prompt)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", l.cfg.Provider)
	}
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// fallback: treat response as plain text
		result = map[string]interface{}{"output": response}
	}
	return result, nil
}

func (l *LLMExecutor) callOllama(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  l.cfg.Model,
		"prompt": prompt,
		"stream": false,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", l.cfg.Endpoint+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var ollamaResp struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return "", err
	}
	return ollamaResp.Response, nil
}