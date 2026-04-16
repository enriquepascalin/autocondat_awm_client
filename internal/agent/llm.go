package agent

import (
	"context"
	"fmt"

	"github.com/enriquepascalin/awm-cli/internal/executor"
	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

// LLMAgent implements the Agent interface for an AI language model participant.
type LLMAgent struct {
	id           string
	tenant       string
	capabilities []string
	executor     *executor.LLMExecutor
}

// NewLLMAgent creates a new LLM agent with the given identity and executor.
func NewLLMAgent(id, tenant string, capabilities []string, exec *executor.LLMExecutor) *LLMAgent {
	return &LLMAgent{
		id:           id,
		tenant:       tenant,
		capabilities: capabilities,
		executor:     exec,
	}
}

// ID returns the agent's unique identifier.
func (a *LLMAgent) ID() string {
	return a.id
}

// Type returns the agent type ("ai").
func (a *LLMAgent) Type() string {
	return "ai"
}

// Tenant returns the agent's tenant.
func (a *LLMAgent) Tenant() string {
	return a.tenant
}

// Capabilities returns the list of capabilities this agent can handle.
func (a *LLMAgent) Capabilities() []string {
	return a.capabilities
}

// Execute delegates task execution to the LLM executor and returns the result.
func (a *LLMAgent) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("LLM executor not initialized")
	}
	return a.executor.Execute(ctx, task)
}

// Validate ensures the agent is properly configured.
func (a *LLMAgent) Validate() error {
	if a.id == "" {
		return fmt.Errorf("agent ID is required")
	}
	if a.tenant == "" {
		return fmt.Errorf("tenant is required")
	}
	if a.executor == nil {
		return fmt.Errorf("executor is required")
	}
	return nil
}
