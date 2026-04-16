package client

import (
	"context"
	"log"
	"math"
	"time"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// Poller periodically polls the orchestrator for pending tasks using the unary PollTask RPC.
// It implements exponential backoff on consecutive errors to avoid overwhelming the server.
type Poller struct {
	client  *GRPCClient
	agentID string
	tenant  string

	// configurable settings
	pollInterval  time.Duration
	maxBackoff    time.Duration
	backoffFactor float64
}

// NewPoller creates a new Poller with sensible defaults.
func NewPoller(client *GRPCClient, agentID, tenant string) *Poller {
	return &Poller{
		client:        client,
		agentID:       agentID,
		tenant:        tenant,
		pollInterval:  5 * time.Second,
		maxBackoff:    60 * time.Second,
		backoffFactor: 2.0,
	}
}

// WithPollInterval sets the base interval between successful polls.
func (p *Poller) WithPollInterval(interval time.Duration) *Poller {
	p.pollInterval = interval
	return p
}

// WithMaxBackoff sets the maximum backoff duration after repeated errors.
func (p *Poller) WithMaxBackoff(max time.Duration) *Poller {
	p.maxBackoff = max
	return p
}

// Run starts the polling loop. It blocks until the context is canceled.
// For each poll cycle, it fetches a task and passes it to the provided handler function.
// The handler should be non‑blocking; for long‑running tasks, it should spawn a goroutine.
func (p *Poller) Run(ctx context.Context, handler func(*awmv1.TaskAssignment)) {
	var consecutiveErrors int
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Poller stopping due to context cancellation")
			return
		case <-timer.C:
			task, err := p.client.PollTask(ctx, p.agentID, p.tenant)
			if err != nil {
				consecutiveErrors++
				backoff := p.calculateBackoff(consecutiveErrors)
				log.Printf("PollTask error (attempt %d): %v; backing off for %v", consecutiveErrors, err, backoff)
				timer.Reset(backoff)
				continue
			}
			// Successful poll resets error counter
			consecutiveErrors = 0
			timer.Reset(p.pollInterval)

			if task == nil || task.TaskId == "" {
				continue
			}
			// Invoke handler for the task
			handler(task)
		}
	}
}

// PollOnce performs a single poll attempt and returns any task found.
func (p *Poller) PollOnce(ctx context.Context) (*awmv1.TaskAssignment, error) {
	return p.client.PollTask(ctx, p.agentID, p.tenant)
}

// calculateBackoff computes exponential backoff with jitter.
func (p *Poller) calculateBackoff(consecutiveErrors int) time.Duration {
	if consecutiveErrors <= 0 {
		return p.pollInterval
	}
	backoff := float64(p.pollInterval) * math.Pow(p.backoffFactor, float64(consecutiveErrors-1))
	if backoff > float64(p.maxBackoff) {
		backoff = float64(p.maxBackoff)
	}
	// Add ±10% jitter
	jitter := backoff * 0.1 * (float64(time.Now().UnixNano()%100)/100 - 0.5)
	return time.Duration(backoff + jitter)
}
