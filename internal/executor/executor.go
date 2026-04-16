package executor

import (
	"context"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

type Executor interface {
	Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error)
}