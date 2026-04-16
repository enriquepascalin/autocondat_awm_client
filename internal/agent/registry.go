package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/enriquepascalin/awm-cli/internal/client"
	"github.com/enriquepascalin/awm-cli/internal/executor"
	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// Registry handles the lifecycle of an agent's connection to the orchestrator.
// It manages registration, task consumption, heartbeats, and result reporting.
type Registry struct {
	agent  Agent
	client *client.GRPCClient
	exec   executor.Executor
}

// NewRegistry creates a new Registry with the given dependencies.
func NewRegistry(agent Agent, grpcClient *client.GRPCClient, exec executor.Executor) *Registry {
	return &Registry{
		agent:  agent,
		client: grpcClient,
		exec:   exec,
	}
}

// Register validates the agent and performs initial registration.
// For streaming mode, registration happens during stream establishment.
// For polling mode, registration is implicit with the first PollTask.
func (r *Registry) Register(ctx context.Context) error {
	if err := r.agent.Validate(); err != nil {
		return fmt.Errorf("agent validation failed: %w", err)
	}
	// Registration is a no-op for polling; stream mode handles it in RunStream.
	return nil
}

// Unregister performs any cleanup needed before the agent disconnects.
func (r *Registry) Unregister(ctx context.Context) error {
	// Currently no explicit unregistration RPC exists.
	return nil
}

// RunStream establishes a bidirectional stream and processes tasks continuously.
// It blocks until the context is canceled or an irrecoverable error occurs.
func (r *Registry) RunStream(ctx context.Context) error {
	stream, err := r.client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect stream: %w", err)
	}
	defer stream.CloseSend()

	// Send registration message
	if err := stream.Send(&awmv1.AgentToServer{
		Message: &awmv1.AgentToServer_Register{
			Register: &awmv1.RegisterAgent{
				AgentId:      r.agent.ID(),
				Tenant:       r.agent.Tenant(),
				AgentType:    r.agent.Type(),
				Capabilities: r.agent.Capabilities(),
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}

	// Start receiver goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.receiveStream(ctx, stream)
	}()

	// Heartbeat ticker
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stream loop exiting due to context cancellation")
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("stream receive error: %w", err)
			}
			return nil
		case <-heartbeatTicker.C:
			if err := stream.Send(&awmv1.AgentToServer{
				Message: &awmv1.AgentToServer_Heartbeat{
					Heartbeat: &awmv1.Heartbeat{
						AgentId: r.agent.ID(),
					},
				},
			}); err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
			}
		}
	}
}

// receiveStream listens for incoming ServerToAgent messages and dispatches them.
func (r *Registry) receiveStream(ctx context.Context, stream awmv1.Orchestrator_ConnectClient) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		switch m := msg.Message.(type) {
		case *awmv1.ServerToAgent_TaskAssignment:
			go r.handleTask(ctx, m.TaskAssignment)
		case *awmv1.ServerToAgent_Ack:
			log.Println("Registration acknowledged by orchestrator")
		case *awmv1.ServerToAgent_Signal:
			log.Printf("Received signal: %s", m.Signal.SignalName)
			// Signals can be handled here if needed.
		}
	}
}

// RunPolling uses periodic PollTask requests to fetch and execute tasks.
// It blocks until the context is canceled.
func (r *Registry) RunPolling(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Polling loop exiting due to context cancellation")
			return ctx.Err()
		case <-ticker.C:
			task, err := r.client.PollTask(ctx, r.agent.ID(), r.agent.Tenant())
			if err != nil {
				log.Printf("PollTask error: %v", err)
				continue
			}
			if task == nil || task.TaskId == "" {
				continue
			}
			go r.handleTask(ctx, task)
		}
	}
}

// handleTask executes a single task and reports the result back to the orchestrator.
func (r *Registry) handleTask(ctx context.Context, task *awmv1.TaskAssignment) {
	log.Printf("Received task %s: %s", task.TaskId, task.ActivityName)

	// Create a child context with timeout if deadline is set.
	taskCtx := ctx
	if task.Deadline != nil {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithDeadline(ctx, task.Deadline.AsTime())
		defer cancel()
	}

	result, err := r.exec.Execute(taskCtx, task)
	if err != nil {
		log.Printf("Task %s execution failed: %v", task.TaskId, err)
		if reportErr := r.client.FailTask(ctx, task.TaskId, &awmv1.TaskError{
			Message: err.Error(),
		}); reportErr != nil {
			log.Printf("Failed to report task failure: %v", reportErr)
		}
		return
	}

	if err := r.client.CompleteTask(ctx, task.TaskId, result); err != nil {
		log.Printf("Failed to report task completion: %v", err)
	} else {
		log.Printf("Task %s completed successfully", task.TaskId)
	}
}
