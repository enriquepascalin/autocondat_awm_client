package agent

import (
	"context"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

// Agent defines the contract that all agent types (human, AI, service) must fulfill.
type Agent interface {
	// ID returns the unique identifier of the agent.
	ID() string

	// Type returns the agent kind ("human", "ai", "service").
	Type() string

	// Tenant returns the tenant the agent belongs to.
	Tenant() string

	// Capabilities returns the list of task capabilities this agent can handle.
	Capabilities() []string

	// Execute performs the given task and returns a result map or an error.
	Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error)

	// Validate checks whether the agent is correctly configured.
	Validate() error
}
