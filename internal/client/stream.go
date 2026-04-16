package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// StreamHandler manages a bidirectional gRPC stream to the orchestrator.
// It handles registration, heartbeats, and incoming task assignments.
type StreamHandler struct {
	stream    awmv1.Orchestrator_ConnectClient
	sendMutex sync.Mutex
	recvMutex sync.Mutex
	agentID   string
	tenant    string
}

// NewStreamHandler establishes a connection and returns a handler.
func (c *GRPCClient) NewStreamHandler(ctx context.Context, agentID, tenant, agentType string, capabilities []string) (*StreamHandler, error) {
	stream, err := c.client.Connect(c.withAuth(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	handler := &StreamHandler{
		stream:  stream,
		agentID: agentID,
		tenant:  tenant,
	}

	// Send registration immediately
	if err := handler.sendRegister(agentID, tenant, agentType, capabilities); err != nil {
		stream.CloseSend()
		return nil, fmt.Errorf("failed to send registration: %w", err)
	}

	return handler, nil
}

// sendRegister sends the initial registration message.
func (s *StreamHandler) sendRegister(agentID, tenant, agentType string, capabilities []string) error {
	s.sendMutex.Lock()
	defer s.sendMutex.Unlock()
	return s.stream.Send(&awmv1.AgentToServer{
		Message: &awmv1.AgentToServer_Register{
			Register: &awmv1.RegisterAgent{
				AgentId:      agentID,
				Tenant:       tenant,
				AgentType:    agentType,
				Capabilities: capabilities,
			},
		},
	})
}

// SendHeartbeat sends a periodic heartbeat.
func (s *StreamHandler) SendHeartbeat() error {
	s.sendMutex.Lock()
	defer s.sendMutex.Unlock()
	return s.stream.Send(&awmv1.AgentToServer{
		Message: &awmv1.AgentToServer_Heartbeat{
			Heartbeat: &awmv1.Heartbeat{
				AgentId: s.agentID,
			},
		},
	})
}

// SendTaskResult reports a task outcome.
func (s *StreamHandler) SendTaskResult(taskID string, success bool, result map[string]interface{}, errDetails *awmv1.TaskError) error {
	s.sendMutex.Lock()
	defer s.sendMutex.Unlock()

	var outcome awmv1.TaskResult
	if success {
		s, _ := structpb.NewStruct(result)
		outcome = awmv1.TaskResult{
			TaskId: taskID,
			Outcome: &awmv1.TaskResult_Success{
				Success: s,
			},
		}
	} else {
		outcome = awmv1.TaskResult{
			TaskId: taskID,
			Outcome: &awmv1.TaskResult_Error{
				Error: errDetails,
			},
		}
	}

	return s.stream.Send(&awmv1.AgentToServer{
		Message: &awmv1.AgentToServer_TaskResult{
			TaskResult: &outcome,
		},
	})
}

// Recv waits for and returns the next message from the orchestrator.
func (s *StreamHandler) Recv() (*awmv1.ServerToAgent, error) {
	s.recvMutex.Lock()
	defer s.recvMutex.Unlock()
	return s.stream.Recv()
}

// Close gracefully closes the send direction of the stream.
func (s *StreamHandler) Close() error {
	return s.stream.CloseSend()
}

// HeartbeatLoop sends periodic heartbeats until the context is canceled.
func (s *StreamHandler) HeartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.SendHeartbeat(); err != nil {
				if err != io.EOF {
					log.Printf("Heartbeat failed: %v", err)
				}
				return
			}
		}
	}
}
