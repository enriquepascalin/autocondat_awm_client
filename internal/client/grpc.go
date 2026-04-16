package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	awmv1 "github.com/enriquepascalin/awm-orchestrator/internal/proto/awm/v1"
)

type GRPCClient struct {
	conn   *grpc.ClientConn
	client awmv1.OrchestratorClient
	token  string
}

func NewGRPCClient(address, token string) (*GRPCClient, error) {
	var opts []grpc.DialOption
	if token != "" {
		// Use TLS if token is provided (assuming mTLS setup)
		// For simplicity, we'll use insecure in dev; production should use TLS.
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, err
	}
	return &GRPCClient{
		conn:   conn,
		client: awmv1.NewOrchestratorClient(conn),
		token:  token,
	}, nil
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}

func (c *GRPCClient) Connect(ctx context.Context) (awmv1.Orchestrator_ConnectClient, error) {
	ctx = c.withAuth(ctx)
	return c.client.Connect(ctx)
}

func (c *GRPCClient) PollTask(ctx context.Context, agentID, tenant string) (*awmv1.TaskAssignment, error) {
	ctx = c.withAuth(ctx)
	resp, err := c.client.PollTask(ctx, &awmv1.PollTaskRequest{
		AgentId: agentID,
		Tenant:  tenant,
	})
	if err != nil {
		return nil, err
	}
	return resp.Task, nil
}

func (c *GRPCClient) CompleteTask(ctx context.Context, taskID string, result map[string]interface{}) error {
	ctx = c.withAuth(ctx)
	_, err := c.client.CompleteTask(ctx, &awmv1.CompleteTaskRequest{
		TaskId: taskID,
		Result: mustStruct(result),
	})
	return err
}

func (c *GRPCClient) FailTask(ctx context.Context, taskID string, errDetails *awmv1.TaskError) error {
	ctx = c.withAuth(ctx)
	_, err := c.client.FailTask(ctx, &awmv1.FailTaskRequest{
		TaskId: taskID,
		Error:  errDetails,
	})
	return err
}

func (c *GRPCClient) Heartbeat(ctx context.Context, agentID string, activeTaskIDs []string) error {
	ctx = c.withAuth(ctx)
	_, err := c.client.Heartbeat(ctx, &awmv1.HeartbeatRequest{
		AgentId:        agentID,
		ActiveTaskIds: activeTaskIDs,
	})
	return err
}

func (c *GRPCClient) withAuth(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
}

func mustStruct(m map[string]interface{}) *structpb.Struct {
	s, _ := structpb.NewStruct(m)
	return s
}