package executor

import (
	"context"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

type ServiceExecutor struct{}

func NewServiceExecutor() *ServiceExecutor {
	return &ServiceExecutor{}
}

func (s *ServiceExecutor) Execute(ctx context.Context, task *awmv1.TaskAssignment) (map[string]interface{}, error) {
	// For service agents, tasks are scripts or API calls.
	// This could be extended to run shell commands or call webhooks.
	// For now, return a placeholder success.
	return map[string]interface{}{"status": "ok"}, nil
}