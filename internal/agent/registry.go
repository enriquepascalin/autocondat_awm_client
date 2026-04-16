package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/enriquepascalin/awm-cli/internal/client"
	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
	"github.com/enriquepascalin/awm-cli/internal/logger"
)

const (
	heartbeatInterval  = 30 * time.Second
	reconnectBaseDelay = 1 * time.Second
	reconnectMaxDelay  = 60 * time.Second
)

// Registry manages the lifecycle of an agent's connection to a worker endpoint.
// It delegates stream/polling mechanics to the client package and focuses
// solely on task dispatch and result reporting.
type Registry struct {
	agent  Agent
	client *client.GRPCClient
	log    *logger.Logger
}

// NewRegistry creates a new Registry for the given agent and gRPC client.
func NewRegistry(agent Agent, grpcClient *client.GRPCClient, log *logger.Logger) *Registry {
	return &Registry{
		agent:  agent,
		client: grpcClient,
		log:    log.With("agent_id", agent.ID(), "tenant", agent.Tenant(), "agent_type", agent.Type()),
	}
}

// Register validates the agent before it starts consuming tasks.
func (r *Registry) Register(ctx context.Context) error {
	if err := r.agent.Validate(); err != nil {
		return fmt.Errorf("agent validation failed: %w", err)
	}
	return nil
}

// Unregister performs any cleanup needed before the agent disconnects.
func (r *Registry) Unregister(_ context.Context) error {
	return nil
}

// RunStream establishes a bidirectional stream and processes tasks continuously.
// It reconnects automatically with exponential backoff on any stream failure.
// It blocks until the context is canceled.
func (r *Registry) RunStream(ctx context.Context) error {
	delay := reconnectBaseDelay
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := r.runStreamOnce(ctx)
		if ctx.Err() != nil {
			// Context canceled — clean shutdown, not an error worth logging.
			return ctx.Err()
		}

		r.log.Warn("stream disconnected, reconnecting", "delay", delay, "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay = min(delay*2, reconnectMaxDelay)
	}
}

// runStreamOnce opens one stream session. Returns when the stream ends or errors.
func (r *Registry) runStreamOnce(ctx context.Context) error {
	handler, err := r.client.NewStreamHandler(
		ctx,
		r.agent.ID(),
		r.agent.Tenant(),
		r.agent.Type(),
		r.agent.Capabilities(),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer handler.Close()

	r.log.Info("stream connected")

	// Heartbeat loop runs until the stream is closed.
	go handler.HeartbeatLoop(ctx, heartbeatInterval)

	// Receive loop — blocks until error or context cancellation.
	for {
		msg, err := handler.Recv()
		if err != nil {
			return err
		}
		switch m := msg.Message.(type) {
		case *awmv1.ServerToAgent_TaskAssignment:
			go r.handleTask(ctx, handler, m.TaskAssignment)
		case *awmv1.ServerToAgent_Ack:
			r.log.Info("registration acknowledged")
		case *awmv1.ServerToAgent_Signal:
			r.log.Info("received signal", "signal", m.Signal.SignalName)
		}
	}
}

// RunPolling uses the Poller (with exponential backoff) to fetch and execute tasks.
// It blocks until the context is canceled.
func (r *Registry) RunPolling(ctx context.Context) error {
	poller := client.NewPoller(r.client, r.agent.ID(), r.agent.Tenant())
	r.log.Info("polling started")
	poller.Run(ctx, func(task *awmv1.TaskAssignment) {
		go r.handleTask(ctx, nil, task)
	})
	return ctx.Err()
}

// handleTask executes a single task and reports the result back to the worker.
// streamHandler is nil when operating in polling mode; results are reported via
// the unary CompleteTask/FailTask RPCs in that case.
func (r *Registry) handleTask(ctx context.Context, handler *client.StreamHandler, task *awmv1.TaskAssignment) {
	log := r.log.With("task_id", task.TaskId, "activity", task.ActivityName)
	log.Info("task received")

	taskCtx := ctx
	if task.Deadline != nil {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithDeadline(ctx, task.Deadline.AsTime())
		defer cancel()
	}

	result, err := r.agent.Execute(taskCtx, task)
	if err != nil {
		log.Error("task execution failed", "error", err)
		r.reportFailure(ctx, handler, task.TaskId, err)
		return
	}

	if err := r.reportSuccess(ctx, handler, task.TaskId, result); err != nil {
		log.Error("failed to report task completion", "error", err)
		return
	}
	log.Info("task completed")
}

// reportSuccess sends the task result via stream (if available) or unary RPC.
func (r *Registry) reportSuccess(ctx context.Context, handler *client.StreamHandler, taskID string, result map[string]interface{}) error {
	if handler != nil {
		return handler.SendTaskResult(taskID, true, result, nil)
	}
	return r.client.CompleteTask(ctx, taskID, result)
}

// reportFailure sends the task error via stream (if available) or unary RPC.
func (r *Registry) reportFailure(ctx context.Context, handler *client.StreamHandler, taskID string, execErr error) {
	errDetails := &awmv1.TaskError{Message: execErr.Error()}
	var reportErr error
	if handler != nil {
		reportErr = handler.SendTaskResult(taskID, false, nil, errDetails)
	} else {
		reportErr = r.client.FailTask(ctx, taskID, errDetails)
	}
	if reportErr != nil {
		r.log.Error("failed to report task failure", "task_id", taskID, "error", reportErr)
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
