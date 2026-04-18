package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	awmv1 "github.com/enriquepascalin/awm-cli/internal/proto/awm/v1"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Manage workflow definitions and instances",
}

// ── workflow create ───────────────────────────────────────────────────────���───

var (
	wfCreateFile      string
	wfCreateName      string
	wfCreateTenant    string
	wfCreateCreatedBy string
)

var workflowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a workflow definition from a YAML file",
	Example: `  awm-cli workflow create --file invoice.yaml --name "Invoice Approval" --tenant acme`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfCreateFile == "" {
			return fmt.Errorf("--file is required")
		}
		data, err := os.ReadFile(wfCreateFile)
		if err != nil {
			return fmt.Errorf("cannot read file: %w", err)
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		def, err := mc.CreateWorkflowDefinition(background(), wfCreateName, wfCreateTenant, wfCreateCreatedBy, string(data))
		if err != nil {
			return err
		}
		fmt.Printf("Created: id=%s  name=%s  version=%d\n", def.Id, def.Name, def.Version)
		return nil
	},
}

// ── workflow update ───────────────────────────────────────────────────────────

var (
	wfUpdateID   string
	wfUpdateFile string
)

var workflowUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Replace YAML content of an existing workflow definition",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfUpdateID == "" || wfUpdateFile == "" {
			return fmt.Errorf("--id and --file are required")
		}
		data, err := os.ReadFile(wfUpdateFile)
		if err != nil {
			return fmt.Errorf("cannot read file: %w", err)
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		def, err := mc.UpdateWorkflowDefinition(background(), wfUpdateID, string(data))
		if err != nil {
			return err
		}
		fmt.Printf("Updated: id=%s  version=%d\n", def.Id, def.Version)
		return nil
	},
}

// ── workflow delete ───────────────────────────────────────────────────────────

var wfDeleteID string

var workflowDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a workflow definition",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfDeleteID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.DeleteWorkflowDefinition(background(), wfDeleteID)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Deleted.")
		}
		return nil
	},
}

// ── workflow list (definitions) ───────────────────────────────────────────────

var (
	wfListTenant     string
	wfListNameFilter string
	wfListPageSize   int32
)

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflow definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		defs, err := mc.ListWorkflowDefinitions(background(), wfListTenant, wfListNameFilter, wfListPageSize)
		if err != nil {
			return err
		}
		if len(defs) == 0 {
			fmt.Println("No definitions found.")
			return nil
		}
		fmt.Printf("%-36s  %-30s  %s\n", "ID", "NAME", "VERSION")
		for _, d := range defs {
			fmt.Printf("%-36s  %-30s  %d\n", d.Id, d.Name, d.Version)
		}
		return nil
	},
}

// ── workflow get ──────────────────────────────────────────────────────────────

var wfGetID string

var workflowGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a workflow definition by ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfGetID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		def, err := mc.GetWorkflowDefinition(background(), wfGetID)
		if err != nil {
			return err
		}
		printJSON(def)
		return nil
	},
}

// ── workflow start (instance) ─────────────────────────────────────────────────

var (
	wfStartDefID  string
	wfStartTenant string
	wfStartInput  string
)

var workflowStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new workflow instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfStartDefID == "" {
			return fmt.Errorf("--def is required")
		}
		input, err := parseJSON(wfStartInput)
		if err != nil {
			return err
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		instanceID, endpoint, err := mc.StartWorkflow(background(), wfStartDefID, wfStartTenant, input)
		if err != nil {
			return err
		}
		fmt.Printf("Started: instance_id=%s  endpoint=%s\n", instanceID, endpoint)
		return nil
	},
}

// ── workflow status ───────────────────────────────────────────────────────────

var wfStatusID string

var workflowStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of a workflow instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfStatusID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		state, err := mc.GetWorkflowState(background(), wfStatusID)
		if err != nil {
			return err
		}
		fmt.Printf("Instance:     %s\n", state.WorkflowInstanceId)
		fmt.Printf("Definition:   %s\n", state.WorkflowDefinitionId)
		fmt.Printf("Status:       %s\n", state.Status.String())
		fmt.Printf("Current state: %s\n", state.CurrentState)
		return nil
	},
}

// ── workflow instances ────────────────────────────────────────────────────────

var (
	wfInstancesDefID  string
	wfInstancesTenant string
	wfInstancesStatus string
	wfInstancesLimit  int32
)

var workflowInstancesCmd = &cobra.Command{
	Use:   "instances",
	Short: "List workflow instances",
	RunE: func(cmd *cobra.Command, args []string) error {
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		statusEnum := workflowStatusStringToEnum(wfInstancesStatus)
		workflows, err := mc.ListWorkflows(background(), wfInstancesTenant, wfInstancesDefID, statusEnum, wfInstancesLimit)
		if err != nil {
			return err
		}
		if len(workflows) == 0 {
			fmt.Println("No instances found.")
			return nil
		}
		fmt.Printf("%-36s  %-36s  %-12s  %s\n", "INSTANCE_ID", "DEFINITION_ID", "STATUS", "CURRENT_STATE")
		for _, w := range workflows {
			fmt.Printf("%-36s  %-36s  %-12s  %s\n", w.WorkflowInstanceId, w.WorkflowDefinitionId, w.Status.String(), w.CurrentState)
		}
		return nil
	},
}

// ── workflow stop ─────────────────────────────────────────────────────────────

var (
	wfStopID      string
	wfStopArchive bool
)

var workflowStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a workflow instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfStopID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.StopInstance(background(), wfStopID, wfStopArchive)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Stopped.")
		}
		return nil
	},
}

// ── workflow cancel ───────────────────────────────────────────────────────────

var wfCancelID string

var workflowCancelCmd = &cobra.Command{
	Use:   "cancel",
	Short: "Cancel a running workflow instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfCancelID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.CancelWorkflow(background(), wfCancelID)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Canceled.")
		}
		return nil
	},
}

// ── workflow signal ───────────────────────────────────────────────────────────

var (
	wfSignalID      string
	wfSignalName    string
	wfSignalPayload string
)

var workflowSignalCmd = &cobra.Command{
	Use:   "signal",
	Short: "Send a signal to a running workflow",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wfSignalID == "" || wfSignalName == "" {
			return fmt.Errorf("--id and --signal are required")
		}
		payload, err := parseJSON(wfSignalPayload)
		if err != nil {
			return err
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.SignalWorkflow(background(), wfSignalID, wfSignalName, payload)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Signal sent.")
		}
		return nil
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

func workflowStatusStringToEnum(s string) awmv1.WorkflowStatus {
	switch s {
	case "RUNNING":
		return awmv1.WorkflowStatus_WORKFLOW_STATUS_RUNNING
	case "COMPLETED":
		return awmv1.WorkflowStatus_WORKFLOW_STATUS_COMPLETED
	case "FAILED":
		return awmv1.WorkflowStatus_WORKFLOW_STATUS_FAILED
	case "CANCELED":
		return awmv1.WorkflowStatus_WORKFLOW_STATUS_CANCELED
	default:
		return awmv1.WorkflowStatus_WORKFLOW_STATUS_UNSPECIFIED
	}
}

func init() {
	// create
	workflowCreateCmd.Flags().StringVarP(&wfCreateFile, "file", "f", "", "Path to workflow YAML file (required)")
	workflowCreateCmd.Flags().StringVar(&wfCreateName, "name", "", "Human-readable workflow name")
	workflowCreateCmd.Flags().StringVar(&wfCreateTenant, "tenant", "", "Tenant name")
	workflowCreateCmd.Flags().StringVar(&wfCreateCreatedBy, "created-by", "", "Creator identifier")

	// update
	workflowUpdateCmd.Flags().StringVar(&wfUpdateID, "id", "", "Workflow definition UUID (required)")
	workflowUpdateCmd.Flags().StringVarP(&wfUpdateFile, "file", "f", "", "Path to new YAML file (required)")

	// delete
	workflowDeleteCmd.Flags().StringVar(&wfDeleteID, "id", "", "Workflow definition UUID (required)")

	// list
	workflowListCmd.Flags().StringVar(&wfListTenant, "tenant", "", "Filter by tenant name")
	workflowListCmd.Flags().StringVar(&wfListNameFilter, "name", "", "Filter by name substring")
	workflowListCmd.Flags().Int32Var(&wfListPageSize, "limit", 50, "Maximum results to return")

	// get
	workflowGetCmd.Flags().StringVar(&wfGetID, "id", "", "Workflow definition UUID (required)")

	// start
	workflowStartCmd.Flags().StringVar(&wfStartDefID, "def", "", "Workflow definition ID (required)")
	workflowStartCmd.Flags().StringVar(&wfStartTenant, "tenant", "", "Tenant name")
	workflowStartCmd.Flags().StringVar(&wfStartInput, "input", "", "Initial context as JSON string")

	// status
	workflowStatusCmd.Flags().StringVar(&wfStatusID, "id", "", "Workflow instance UUID (required)")

	// instances
	workflowInstancesCmd.Flags().StringVar(&wfInstancesDefID, "def", "", "Filter by definition ID")
	workflowInstancesCmd.Flags().StringVar(&wfInstancesTenant, "tenant", "", "Filter by tenant")
	workflowInstancesCmd.Flags().StringVar(&wfInstancesStatus, "status", "", "Filter by status (RUNNING|COMPLETED|FAILED|CANCELED)")
	workflowInstancesCmd.Flags().Int32Var(&wfInstancesLimit, "limit", 50, "Maximum results")

	// stop
	workflowStopCmd.Flags().StringVar(&wfStopID, "id", "", "Instance UUID (required)")
	workflowStopCmd.Flags().BoolVar(&wfStopArchive, "archive", false, "Archive event log and task history")

	// cancel
	workflowCancelCmd.Flags().StringVar(&wfCancelID, "id", "", "Instance UUID (required)")

	// signal
	workflowSignalCmd.Flags().StringVar(&wfSignalID, "id", "", "Instance UUID (required)")
	workflowSignalCmd.Flags().StringVar(&wfSignalName, "signal", "", "Signal name (required)")
	workflowSignalCmd.Flags().StringVar(&wfSignalPayload, "payload", "", "Payload as JSON string")

	workflowCmd.AddCommand(
		workflowCreateCmd,
		workflowUpdateCmd,
		workflowDeleteCmd,
		workflowListCmd,
		workflowGetCmd,
		workflowStartCmd,
		workflowStatusCmd,
		workflowInstancesCmd,
		workflowStopCmd,
		workflowCancelCmd,
		workflowSignalCmd,
	)
	rootCmd.AddCommand(workflowCmd)
}
