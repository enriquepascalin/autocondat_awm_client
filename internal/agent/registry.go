package agent

import (
	"context"
	"log"
	"time"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
	"github.com/enriquepascalin/awm-cli/internal/client"
	"github.com/enriquepascalin/awm-cli/internal/config"
	"github.com/enriquepascalin/awm-cli/internal/executor"
)

type Registry struct {
	client *client.GRPCClient
	exec   executor.Executor
	cfg    config.AgentConfig
}

func NewRegistry(c *client.GRPCClient, e executor.Executor, cfg config.AgentConfig) *Registry {
	return &Registry{client: c, exec: e, cfg: cfg}
}

func (r *Registry) Register(ctx context.Context) error {
	// Registration happens automatically on first Connect/Poll
	return nil
}

func (r *Registry) Unregister(ctx context.Context) error {
	return nil
}

func (r *Registry) RunStream(ctx context.Context) error {
	stream, err := r.client.Connect(ctx)
	if err != nil {
		return err
	}
	// Send registration
	if err := stream.Send(&awmv1.AgentToServer{
		Message: &awmv1.AgentToServer_Register{
			Register: &awmv1.RegisterAgent{
				AgentId:      r.cfg.ID,
				Tenant:       r.cfg.Tenant,
				AgentType:    r.cfg.Type,
				Capabilities: r.cfg.Capabilities,
			},
		},
	}); err != nil {
		return err
	}

	// Receive loop
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				log.Printf("Stream receive error: %v", err)
				return
			}
			switch m := msg.Message.(type) {
			case *awmv1.ServerToAgent_TaskAssignment:
				go r.handleTask(ctx, m.TaskAssignment)
			case *awmv1.ServerToAgent_Ack:
				log.Printf("Registration acknowledged")
			}
		}
	}()

	// Heartbeat ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := stream.Send(&awmv1.AgentToServer{
				Message: &awmv1.AgentToServer_Heartbeat{
					Heartbeat: &awmv1.Heartbeat{AgentId: r.cfg.ID},
				},
			}); err != nil {
				return err
			}
		}
	}
}

func (r *Registry) RunPolling(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			task, err := r.client.PollTask(ctx, r.cfg.ID, r.cfg.Tenant)
			if err != nil {
				log.Printf("Poll error: %v", err)
				continue
			}
			if task != nil && task.TaskId != "" {
				go r.handleTask(ctx, task)
			}
		}
	}
}

func (r *Registry) handleTask(ctx context.Context, task *awmv1.TaskAssignment) {
	log.Printf("Received task %s: %s", task.TaskId, task.ActivityName)
	result, err := r.exec.Execute(ctx, task)
	if err != nil {
		log.Printf("Task %s failed: %v", task.TaskId, err)
		_ = r.client.FailTask(ctx, task.TaskId, &awmv1.TaskError{Message: err.Error()})
		return
	}
	if err := r.client.CompleteTask(ctx, task.TaskId, result); err != nil {
		log.Printf("Failed to report completion for task %s: %v", task.TaskId, err)
	}
}