package executor

import (
	"context"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// Executor defines the contract for task execution implementations.
// Each agent type (human, ai, service) will have a corresponding executor
// that implements this interface, encapsulating the specific logic required
// to perform tasks for that agent type.
type Executor interface {
	// Execute performs the given task and returns a result map or an error.
	// The result map should contain structured data that can be serialized
	// and sent back to the orchestrator.
	Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error)
}
