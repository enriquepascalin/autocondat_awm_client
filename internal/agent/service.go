package agent

import (
	"context"
	"fmt"

	"github.com/enriquepascalin/awm-cli/internal/executor"
	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

// ServiceAgent implements the Agent interface for automated service connectors
// (e.g., Jira, GitHub, telemetry) that operate without direct human or AI interaction.
type ServiceAgent struct {
	id           string
	tenant       string
	capabilities []string
	executor     *executor.ServiceExecutor
}

// NewServiceAgent creates a new service agent with the given identity and executor.
func NewServiceAgent(id, tenant string, capabilities []string, exec *executor.ServiceExecutor) *ServiceAgent {
	return &ServiceAgent{
		id:           id,
		tenant:       tenant,
		capabilities: capabilities,
		executor:     exec,
	}
}

// ID returns the agent's unique identifier.
func (a *ServiceAgent) ID() string {
	return a.id
}

// Type returns the agent type ("service").
func (a *ServiceAgent) Type() string {
	return "service"
}

// Tenant returns the agent's tenant.
func (a *ServiceAgent) Tenant() string {
	return a.tenant
}

// Capabilities returns the list of capabilities this agent can handle.
func (a *ServiceAgent) Capabilities() []string {
	return a.capabilities
}

// Execute delegates task execution to the service executor and returns the result.
func (a *ServiceAgent) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("service executor not initialized")
	}
	return a.executor.Execute(ctx, task)
}

// Validate ensures the agent is properly configured.
func (a *ServiceAgent) Validate() error {
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
