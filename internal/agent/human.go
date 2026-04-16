package agent

import (
	"context"
	"fmt"

	"github.com/enriquepascalin/awm-cli/internal/executor"
	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// HumanAgent implements the Agent interface for a human participant.
type HumanAgent struct {
	id           string
	tenant       string
	capabilities []string
	executor     *executor.HumanExecutor
}

// NewHumanAgent creates a new human agent with the given identity and executor.
func NewHumanAgent(id, tenant string, capabilities []string, exec *executor.HumanExecutor) *HumanAgent {
	return &HumanAgent{
		id:           id,
		tenant:       tenant,
		capabilities: capabilities,
		executor:     exec,
	}
}

// ID returns the agent's unique identifier.
func (a *HumanAgent) ID() string {
	return a.id
}

// Type returns the agent type ("human").
func (a *HumanAgent) Type() string {
	return "human"
}

// Tenant returns the agent's tenant.
func (a *HumanAgent) Tenant() string {
	return a.tenant
}

// Capabilities returns the list of capabilities this agent can handle.
func (a *HumanAgent) Capabilities() []string {
	return a.capabilities
}

// Execute delegates task execution to the human executor and returns the result.
func (a *HumanAgent) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("human executor not initialized")
	}
	return a.executor.Execute(ctx, task)
}

// Validate ensures the agent is properly configured.
func (a *HumanAgent) Validate() error {
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
