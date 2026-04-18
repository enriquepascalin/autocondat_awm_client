package client

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

// ManagementClient wraps both Public and Orchestrator gRPC clients for CLI use.
type ManagementClient struct {
	conn      *grpc.ClientConn
	public    awmv1.PublicClient
	orch      awmv1.OrchestratorClient
	authToken string
}

// NewManagementClient connects to the orchestrator and returns a client that
// exposes both the management plane (Public) and orchestrator RPCs.
func NewManagementClient(address, authToken string) (*ManagementClient, error) {
	var opts []grpc.DialOption
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
	return &ManagementClient{
		conn:      conn,
		public:    awmv1.NewPublicClient(conn),
		orch:      awmv1.NewOrchestratorClient(conn),
		authToken: authToken,
	}, nil
}

func (m *ManagementClient) Close() error { return m.conn.Close() }

func (m *ManagementClient) withAuth(ctx context.Context) context.Context {
	if m.authToken == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+m.authToken)
}

// ── Workflow definitions ──────────────────────────────────────────────────────

func (m *ManagementClient) CreateWorkflowDefinition(ctx context.Context, name, tenant, createdBy, yamlContent string) (*awmv1.WorkflowDefinition, error) {
	return m.public.CreateWorkflowDefinition(m.withAuth(ctx), &awmv1.CreateWorkflowDefinitionRequest{
		Name:        name,
		Tenant:      tenant,
		CreatedBy:   createdBy,
		YamlContent: yamlContent,
	})
}

func (m *ManagementClient) UpdateWorkflowDefinition(ctx context.Context, id, yamlContent string) (*awmv1.WorkflowDefinition, error) {
	return m.public.UpdateWorkflowDefinition(m.withAuth(ctx), &awmv1.UpdateWorkflowDefinitionRequest{
		Id:          id,
		YamlContent: yamlContent,
	})
}

func (m *ManagementClient) DeleteWorkflowDefinition(ctx context.Context, id string) (bool, error) {
	resp, err := m.public.DeleteWorkflowDefinition(m.withAuth(ctx), &awmv1.DeleteWorkflowDefinitionRequest{Id: id})
	if err != nil {
		return false, err
	}
	return resp.Deleted, nil
}

func (m *ManagementClient) ListWorkflowDefinitions(ctx context.Context, tenant, nameFilter string, pageSize int32) ([]*awmv1.WorkflowDefinition, error) {
	resp, err := m.public.ListWorkflowDefinitions(m.withAuth(ctx), &awmv1.ListWorkflowDefinitionsRequest{
		Tenant:     tenant,
		NameFilter: nameFilter,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, err
	}
	return resp.Definitions, nil
}

func (m *ManagementClient) GetWorkflowDefinition(ctx context.Context, id string) (*awmv1.WorkflowDefinition, error) {
	return m.public.GetWorkflowDefinition(m.withAuth(ctx), &awmv1.GetWorkflowDefinitionRequest{Id: id})
}

// ── Workflow instances ────────────────────────────────────────────────────────

func (m *ManagementClient) StartWorkflow(ctx context.Context, definitionID, tenant string, input map[string]interface{}) (string, string, error) {
	inputStruct, err := structpb.NewStruct(input)
	if err != nil {
		return "", "", fmt.Errorf("invalid input: %w", err)
	}
	resp, err := m.public.StartWorkflow(m.withAuth(ctx), &awmv1.StartWorkflowRequest{
		WorkflowDefinitionId: definitionID,
		Tenant:               tenant,
		Input:                inputStruct,
	})
	if err != nil {
		return "", "", err
	}
	return resp.WorkflowInstanceId, resp.OrchestratorEndpoint, nil
}

func (m *ManagementClient) GetWorkflowState(ctx context.Context, instanceID string) (*awmv1.GetWorkflowStateResponse, error) {
	return m.orch.GetWorkflowState(m.withAuth(ctx), &awmv1.GetWorkflowStateRequest{
		WorkflowInstanceId: instanceID,
	})
}

func (m *ManagementClient) ListWorkflows(ctx context.Context, tenant, definitionID string, statusFilter awmv1.WorkflowStatus, pageSize int32) ([]*awmv1.WorkflowSummary, error) {
	resp, err := m.orch.ListWorkflows(m.withAuth(ctx), &awmv1.ListWorkflowsRequest{
		Tenant:               tenant,
		WorkflowDefinitionId: definitionID,
		StatusFilter:         statusFilter,
		PageSize:             pageSize,
	})
	if err != nil {
		return nil, err
	}
	return resp.Workflows, nil
}

func (m *ManagementClient) StopInstance(ctx context.Context, id string, archive bool) (bool, error) {
	resp, err := m.public.StopInstance(m.withAuth(ctx), &awmv1.StopInstanceRequest{Id: id, Archive: archive})
	if err != nil {
		return false, err
	}
	return resp.Stopped, nil
}

func (m *ManagementClient) SignalWorkflow(ctx context.Context, instanceID, signalName string, payload map[string]interface{}) (bool, error) {
	payloadStruct, _ := structpb.NewStruct(payload)
	resp, err := m.public.SignalWorkflow(m.withAuth(ctx), &awmv1.SignalWorkflowRequest{
		WorkflowInstanceId: instanceID,
		SignalName:         signalName,
		Payload:            payloadStruct,
	})
	if err != nil {
		return false, err
	}
	return resp.Accepted, nil
}

func (m *ManagementClient) CancelWorkflow(ctx context.Context, instanceID string) (bool, error) {
	resp, err := m.public.CancelWorkflow(m.withAuth(ctx), &awmv1.CancelWorkflowRequest{
		WorkflowInstanceId: instanceID,
	})
	if err != nil {
		return false, err
	}
	return resp.Accepted, nil
}

// ── Tasks ─────────────────────────────────────────────────────────────────────

func (m *ManagementClient) ListTasks(ctx context.Context, instanceID, statusFilter string, pageSize int32) ([]*awmv1.TaskSummary, error) {
	resp, err := m.orch.ListTasks(m.withAuth(ctx), &awmv1.ListTasksRequest{
		WorkflowInstanceId: instanceID,
		StatusFilter:       statusFilter,
		PageSize:           pageSize,
	})
	if err != nil {
		return nil, err
	}
	return resp.Tasks, nil
}

func (m *ManagementClient) GetTask(ctx context.Context, taskID string) (*awmv1.GetTaskResponse, error) {
	return m.orch.GetTask(m.withAuth(ctx), &awmv1.GetTaskRequest{TaskId: taskID})
}

func (m *ManagementClient) ListMyTasks(ctx context.Context, agentID, tenant, statusFilter string) ([]*awmv1.TaskSummary, error) {
	resp, err := m.orch.ListMyTasks(m.withAuth(ctx), &awmv1.ListMyTasksRequest{
		AgentId:      agentID,
		Tenant:       tenant,
		StatusFilter: statusFilter,
	})
	if err != nil {
		return nil, err
	}
	return resp.Tasks, nil
}

func (m *ManagementClient) ClaimTask(ctx context.Context, taskID, agentID string) (*awmv1.ClaimTaskResponse, error) {
	return m.orch.ClaimTask(m.withAuth(ctx), &awmv1.ClaimTaskRequest{
		TaskId:  taskID,
		AgentId: agentID,
	})
}

func (m *ManagementClient) ReleaseTask(ctx context.Context, taskID, agentID, reason string) (bool, error) {
	resp, err := m.orch.ReleaseTask(m.withAuth(ctx), &awmv1.ReleaseTaskRequest{
		TaskId:  taskID,
		AgentId: agentID,
		Reason:  reason,
	})
	if err != nil {
		return false, err
	}
	return resp.Released, nil
}

func (m *ManagementClient) SubmitTaskResult(ctx context.Context, taskID, agentID, summary string, data map[string]interface{}) (*awmv1.SubmitTaskResultResponse, error) {
	dataStruct, _ := structpb.NewStruct(data)
	return m.orch.SubmitTaskResult(m.withAuth(ctx), &awmv1.SubmitTaskResultRequest{
		TaskId:  taskID,
		AgentId: agentID,
		Evidence: &awmv1.TaskEvidence{
			Summary: summary,
			AgentId: agentID,
			Data:    dataStruct,
		},
	})
}

func (m *ManagementClient) SubmitTaskFailure(ctx context.Context, taskID, agentID, code, message string) (bool, error) {
	resp, err := m.orch.SubmitTaskFailure(m.withAuth(ctx), &awmv1.SubmitTaskFailureRequest{
		TaskId:  taskID,
		AgentId: agentID,
		Error: &awmv1.TaskError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return false, err
	}
	return resp.Accepted, nil
}

func (m *ManagementClient) UpdateTaskProgress(ctx context.Context, taskID, agentID, message string, percent int32) (bool, error) {
	resp, err := m.orch.UpdateTaskProgress(m.withAuth(ctx), &awmv1.UpdateTaskProgressRequest{
		TaskId:  taskID,
		AgentId: agentID,
		Message: message,
		Percent: percent,
	})
	if err != nil {
		return false, err
	}
	return resp.Accepted, nil
}

func (m *ManagementClient) ReassignTask(ctx context.Context, taskID, fromAgentID, toAgentID, reason string) (bool, error) {
	resp, err := m.orch.ReassignTask(m.withAuth(ctx), &awmv1.ReassignTaskRequest{
		TaskId:      taskID,
		FromAgentId: fromAgentID,
		ToAgentId:   toAgentID,
		Reason:      reason,
	})
	if err != nil {
		return false, err
	}
	return resp.Accepted, nil
}

func (m *ManagementClient) GetTaskHistory(ctx context.Context, taskID string) ([]*awmv1.TaskAuditEntry, error) {
	resp, err := m.orch.GetTaskHistory(m.withAuth(ctx), &awmv1.GetTaskHistoryRequest{TaskId: taskID})
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}
