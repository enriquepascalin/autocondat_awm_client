package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// GRPCClient wraps the gRPC connection to the orchestrator and provides
// high‑level methods for agent operations.
type GRPCClient struct {
	conn      *grpc.ClientConn
	client    awmv1.OrchestratorClient
	authToken string
}

// NewGRPCClient creates a new gRPC client connected to the given address.
// If authToken is non‑empty, it will be sent as a Bearer token in metadata.
// TLS is enabled by default when the address does not explicitly use plaintext.
func NewGRPCClient(address, authToken string) (*GRPCClient, error) {
	var opts []grpc.DialOption

	// Use insecure for local development unless TLS is explicitly configured.
	// In production, set AWM_TLS_CA_CERT and AWM_TLS_CLIENT_CERT/KEY.
	if caCert := os.Getenv("AWM_TLS_CA_CERT"); caCert != "" {
		creds, err := loadTLSCredentials(caCert)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	return &GRPCClient{
		conn:      conn,
		client:    awmv1.NewOrchestratorClient(conn),
		authToken: authToken,
	}, nil
}

// loadTLSCredentials loads the CA certificate and optional client certificate/key.
func loadTLSCredentials(caCertPath string) (credentials.TransportCredentials, error) {
	caPem, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
	}

	// Optional client certificate for mTLS
	if clientCert := os.Getenv("AWM_TLS_CLIENT_CERT"); clientCert != "" {
		clientKey := os.Getenv("AWM_TLS_CLIENT_KEY")
		if clientKey == "" {
			return nil, fmt.Errorf("AWM_TLS_CLIENT_KEY must be set when AWM_TLS_CLIENT_CERT is provided")
		}
		cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

// Close terminates the underlying gRPC connection.
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}

// Connect establishes a bidirectional stream with the orchestrator.
func (c *GRPCClient) Connect(ctx context.Context) (awmv1.Orchestrator_ConnectClient, error) {
	ctx = c.withAuth(ctx)
	return c.client.Connect(ctx)
}

// PollTask performs a unary poll for a pending task.
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

// CompleteTask reports successful task completion to the orchestrator.
func (c *GRPCClient) CompleteTask(ctx context.Context, taskID string, result map[string]interface{}) error {
	ctx = c.withAuth(ctx)
	resultStruct, err := structpb.NewStruct(result)
	if err != nil {
		return fmt.Errorf("failed to convert result to struct: %w", err)
	}
	_, err = c.client.CompleteTask(ctx, &awmv1.CompleteTaskRequest{
		TaskId: taskID,
		Result: resultStruct,
	})
	return err
}

// FailTask reports task failure to the orchestrator.
func (c *GRPCClient) FailTask(ctx context.Context, taskID string, errDetails *awmv1.TaskError) error {
	ctx = c.withAuth(ctx)
	_, err := c.client.FailTask(ctx, &awmv1.FailTaskRequest{
		TaskId: taskID,
		Error:  errDetails,
	})
	return err
}

// Heartbeat sends a heartbeat to the orchestrator (unary fallback).
func (c *GRPCClient) Heartbeat(ctx context.Context, agentID string, activeTaskIDs []string) error {
	ctx = c.withAuth(ctx)
	_, err := c.client.Heartbeat(ctx, &awmv1.HeartbeatRequest{
		AgentId:       agentID,
		ActiveTaskIds: activeTaskIDs,
	})
	return err
}

// withAuth attaches the Bearer token to outgoing gRPC metadata if configured.
func (c *GRPCClient) withAuth(ctx context.Context) context.Context {
	if c.authToken == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.authToken)
}
