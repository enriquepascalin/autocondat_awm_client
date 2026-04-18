package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage workflow tasks",
}

// ── task list ─────────────────────────────────────────────────────────────────

var (
	taskListInstance string
	taskListStatus   string
	taskListLimit    int32
)

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks for a workflow instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskListInstance == "" {
			return fmt.Errorf("--instance is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		tasks, err := mc.ListTasks(background(), taskListInstance, taskListStatus, taskListLimit)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}
		fmt.Printf("%-36s  %-20s  %-12s  %s\n", "TASK_ID", "ACTIVITY", "STATUS", "AGENT")
		for _, t := range tasks {
			fmt.Printf("%-36s  %-20s  %-12s  %s\n", t.TaskId, t.ActivityName, t.Status, t.AssignedAgentId)
		}
		return nil
	},
}

// ── task get ──────────────────────────────────────────────────────────────────

var taskGetID string

var taskGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get details of a single task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskGetID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		task, err := mc.GetTask(background(), taskGetID)
		if err != nil {
			return err
		}
		printJSON(task)
		return nil
	},
}

// ── task my ───────────────────────────────────────────────────────────────────

var (
	taskMyAgentID string
	taskMyTenant  string
	taskMyStatus  string
)

var taskMyCmd = &cobra.Command{
	Use:   "my",
	Short: "List tasks currently assigned to an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskMyAgentID == "" {
			return fmt.Errorf("--agent is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		tasks, err := mc.ListMyTasks(background(), taskMyAgentID, taskMyTenant, taskMyStatus)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks.")
			return nil
		}
		fmt.Printf("%-36s  %-20s  %s\n", "TASK_ID", "ACTIVITY", "STATUS")
		for _, t := range tasks {
			fmt.Printf("%-36s  %-20s  %s\n", t.TaskId, t.ActivityName, t.Status)
		}
		return nil
	},
}

// ── task claim ────────────────────────────────────────────────────────────────

var (
	taskClaimID    string
	taskClaimAgent string
)

var taskClaimCmd = &cobra.Command{
	Use:   "claim",
	Short: "Claim a pending task for an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskClaimID == "" || taskClaimAgent == "" {
			return fmt.Errorf("--id and --agent are required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		resp, err := mc.ClaimTask(background(), taskClaimID, taskClaimAgent)
		if err != nil {
			return err
		}
		if resp.Claimed {
			fmt.Printf("Claimed. Activity: %s\n", resp.Task.GetActivityName())
		} else {
			fmt.Printf("Not claimed: %s\n", resp.Reason)
		}
		return nil
	},
}

// ── task release ──────────────────────────────────────────────────────────────

var (
	taskReleaseID     string
	taskReleaseAgent  string
	taskReleaseReason string
)

var taskReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release a task back to the pending pool",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskReleaseID == "" || taskReleaseAgent == "" {
			return fmt.Errorf("--id and --agent are required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.ReleaseTask(background(), taskReleaseID, taskReleaseAgent, taskReleaseReason)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Released.")
		}
		return nil
	},
}

// ── task complete ─────────────────────────────────────────────────────────────

var (
	taskCompleteID      string
	taskCompleteAgent   string
	taskCompleteSummary string
	taskCompleteOutput  string
)

var taskCompleteCmd = &cobra.Command{
	Use:   "complete",
	Short: "Mark a task as completed with evidence",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskCompleteID == "" || taskCompleteAgent == "" {
			return fmt.Errorf("--id and --agent are required")
		}
		output, err := parseJSON(taskCompleteOutput)
		if err != nil {
			return err
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		resp, err := mc.SubmitTaskResult(background(), taskCompleteID, taskCompleteAgent, taskCompleteSummary, output)
		if err != nil {
			return err
		}
		fmt.Printf("Completed. Effect: %s\n", resp.Effect.String())
		return nil
	},
}

// ── task fail ─────────────────────────────────────────────────────────────────

var (
	taskFailID      string
	taskFailAgent   string
	taskFailCode    string
	taskFailMessage string
)

var taskFailCmd = &cobra.Command{
	Use:   "fail",
	Short: "Mark a task as failed",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskFailID == "" || taskFailAgent == "" {
			return fmt.Errorf("--id and --agent are required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.SubmitTaskFailure(background(), taskFailID, taskFailAgent, taskFailCode, taskFailMessage)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Failed.")
		}
		return nil
	},
}

// ── task progress ─────────────────────────────────────────────────────────────

var (
	taskProgressID      string
	taskProgressAgent   string
	taskProgressMessage string
	taskProgressPercent int32
)

var taskProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Update task progress",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskProgressID == "" || taskProgressAgent == "" {
			return fmt.Errorf("--id and --agent are required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.UpdateTaskProgress(background(), taskProgressID, taskProgressAgent, taskProgressMessage, taskProgressPercent)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Progress updated.")
		}
		return nil
	},
}

// ── task reassign ─────────────────────────────────────────────────────────────

var (
	taskReassignID     string
	taskReassignFrom   string
	taskReassignTo     string
	taskReassignReason string
)

var taskReassignCmd = &cobra.Command{
	Use:   "reassign",
	Short: "Reassign a task from one agent to another",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskReassignID == "" || taskReassignFrom == "" {
			return fmt.Errorf("--id and --from are required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		ok, err := mc.ReassignTask(background(), taskReassignID, taskReassignFrom, taskReassignTo, taskReassignReason)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("Reassigned.")
		}
		return nil
	},
}

// ── task history ──────────────────────────────────────────────────────────────

var taskHistoryID string

var taskHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Get the full audit history of a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if taskHistoryID == "" {
			return fmt.Errorf("--id is required")
		}
		mc, err := mgmtClient()
		if err != nil {
			return err
		}
		defer mc.Close()
		entries, err := mc.GetTaskHistory(background(), taskHistoryID)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No audit entries.")
			return nil
		}
		fmt.Printf("%-26s  %-12s  %-20s  %s\n", "TIMESTAMP", "EVENT", "AGENT", "MESSAGE")
		for _, e := range entries {
			ts := ""
			if e.Timestamp != nil {
				ts = e.Timestamp.AsTime().Format("2006-01-02T15:04:05Z")
			}
			fmt.Printf("%-26s  %-12s  %-20s  %s\n", ts, e.EventType, e.AgentId, e.Message)
		}
		return nil
	},
}

func init() {
	taskListCmd.Flags().StringVar(&taskListInstance, "instance", "", "Workflow instance UUID (required)")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "Filter by status (PENDING|ASSIGNED|COMPLETED|FAILED)")
	taskListCmd.Flags().Int32Var(&taskListLimit, "limit", 50, "Maximum results")

	taskGetCmd.Flags().StringVar(&taskGetID, "id", "", "Task UUID (required)")

	taskMyCmd.Flags().StringVar(&taskMyAgentID, "agent", "", "Agent ID (required)")
	taskMyCmd.Flags().StringVar(&taskMyTenant, "tenant", "", "Tenant name")
	taskMyCmd.Flags().StringVar(&taskMyStatus, "status", "", "Filter by status")

	taskClaimCmd.Flags().StringVar(&taskClaimID, "id", "", "Task UUID (required)")
	taskClaimCmd.Flags().StringVar(&taskClaimAgent, "agent", "", "Agent ID claiming the task (required)")

	taskReleaseCmd.Flags().StringVar(&taskReleaseID, "id", "", "Task UUID (required)")
	taskReleaseCmd.Flags().StringVar(&taskReleaseAgent, "agent", "", "Agent ID (required)")
	taskReleaseCmd.Flags().StringVar(&taskReleaseReason, "reason", "", "Reason for release")

	taskCompleteCmd.Flags().StringVar(&taskCompleteID, "id", "", "Task UUID (required)")
	taskCompleteCmd.Flags().StringVar(&taskCompleteAgent, "agent", "", "Agent ID (required)")
	taskCompleteCmd.Flags().StringVar(&taskCompleteSummary, "summary", "", "Evidence summary text")
	taskCompleteCmd.Flags().StringVar(&taskCompleteOutput, "output", "", "Result data as JSON string")

	taskFailCmd.Flags().StringVar(&taskFailID, "id", "", "Task UUID (required)")
	taskFailCmd.Flags().StringVar(&taskFailAgent, "agent", "", "Agent ID (required)")
	taskFailCmd.Flags().StringVar(&taskFailCode, "code", "ERR", "Error code")
	taskFailCmd.Flags().StringVar(&taskFailMessage, "message", "", "Error message")

	taskProgressCmd.Flags().StringVar(&taskProgressID, "id", "", "Task UUID (required)")
	taskProgressCmd.Flags().StringVar(&taskProgressAgent, "agent", "", "Agent ID (required)")
	taskProgressCmd.Flags().StringVar(&taskProgressMessage, "message", "", "Progress note")
	taskProgressCmd.Flags().Int32Var(&taskProgressPercent, "percent", 0, "Completion percentage (0-100)")

	taskReassignCmd.Flags().StringVar(&taskReassignID, "id", "", "Task UUID (required)")
	taskReassignCmd.Flags().StringVar(&taskReassignFrom, "from", "", "Current agent ID (required)")
	taskReassignCmd.Flags().StringVar(&taskReassignTo, "to", "", "New agent ID (empty = return to pool)")
	taskReassignCmd.Flags().StringVar(&taskReassignReason, "reason", "", "Reason for reassignment")

	taskHistoryCmd.Flags().StringVar(&taskHistoryID, "id", "", "Task UUID (required)")

	taskCmd.AddCommand(
		taskListCmd,
		taskGetCmd,
		taskMyCmd,
		taskClaimCmd,
		taskReleaseCmd,
		taskCompleteCmd,
		taskFailCmd,
		taskProgressCmd,
		taskReassignCmd,
		taskHistoryCmd,
	)
	rootCmd.AddCommand(taskCmd)
}
