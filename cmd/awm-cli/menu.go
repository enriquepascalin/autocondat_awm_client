package main

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
)

const menuBanner = `
  ██████╗ ██╗    ██╗███╗   ███╗
  ██╔══██╗██║    ██║████╗ ████║
  ███████║██║ █╗ ██║██╔████╔██║
  ██╔══██║██║███╗██║██║╚██╔╝██║
  ██║  ██║╚███╔███╔╝██║ ╚═╝ ██║
  ╚═╝  ╚═╝ ╚══╝╚══╝ ╚═╝     ╚═╝  CLI
`

var menuActions = []string{
	"── Workflow Definitions ──────────────────────────",
	"  Create workflow definition from YAML file",
	"  Update workflow definition",
	"  Delete workflow definition",
	"  List workflow definitions",
	"  Get workflow definition details",
	"── Workflow Instances ────────────────────────────",
	"  Start a workflow instance",
	"  Get workflow instance status",
	"  List workflow instances",
	"  Stop an instance",
	"  Cancel a workflow",
	"  Send signal to workflow",
	"── Tasks ─────────────────────────────────────────",
	"  List tasks for an instance",
	"  Get task details",
	"  List my tasks (by agent)",
	"  Claim a task",
	"  Release a task",
	"  Complete a task",
	"  Fail a task",
	"  Update task progress",
	"  Reassign a task",
	"  Get task history",
	"── Agent ─────────────────────────────────────────",
	"  Start agent worker",
	"────────────────────────────────────────────────── ",
	"  Exit",
}

func runInteractiveMenu() {
	fmt.Print(menuBanner)
	fmt.Printf("  Connected to: %s\n\n", flagAddress)

	for {
		var selected string
		prompt := &survey.Select{
			Message:  "Select an action:",
			Options:  menuActions,
			PageSize: 20,
		}
		if err := survey.AskOne(prompt, &selected); err != nil {
			// ctrl-c or error
			fmt.Println("\nGoodbye.")
			return
		}

		if err := dispatchMenuAction(selected); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n\n", err)
		} else {
			fmt.Println()
		}
	}
}

func ask(label string) string {
	var val string
	_ = survey.AskOne(&survey.Input{Message: label + ":"}, &val)
	return val
}

func askWithDefault(label, def string) string {
	var val string
	_ = survey.AskOne(&survey.Input{Message: label + ":", Default: def}, &val)
	return val
}

func askConfirm(label string) bool {
	var ok bool
	_ = survey.AskOne(&survey.Confirm{Message: label}, &ok)
	return ok
}

//nolint:cyclop
func dispatchMenuAction(action string) error {
	mc, err := mgmtClient()
	if err != nil {
		return fmt.Errorf("cannot connect: %w", err)
	}
	defer mc.Close()
	ctx := background()

	switch action {
	case "  Create workflow definition from YAML file":
		filePath := ask("YAML file path")
		name := ask("Workflow name")
		tenant := ask("Tenant")
		createdBy := ask("Created by (agent/user ID)")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		def, err := mc.CreateWorkflowDefinition(ctx, name, tenant, createdBy, string(data))
		if err != nil {
			return err
		}
		fmt.Printf("  Created: id=%s  name=%s  version=%d\n", def.Id, def.Name, def.Version)

	case "  Update workflow definition":
		id := ask("Definition UUID")
		filePath := ask("New YAML file path")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		def, err := mc.UpdateWorkflowDefinition(ctx, id, string(data))
		if err != nil {
			return err
		}
		fmt.Printf("  Updated: id=%s  version=%d\n", def.Id, def.Version)

	case "  Delete workflow definition":
		id := ask("Definition UUID")
		if !askConfirm("Are you sure?") {
			return nil
		}
		ok, err := mc.DeleteWorkflowDefinition(ctx, id)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Deleted.")
		}

	case "  List workflow definitions":
		tenant := ask("Tenant (leave blank for all)")
		name := ask("Name filter (leave blank for all)")
		defs, err := mc.ListWorkflowDefinitions(ctx, tenant, name, 50)
		if err != nil {
			return err
		}
		if len(defs) == 0 {
			fmt.Println("  No definitions found.")
			return nil
		}
		fmt.Printf("  %-36s  %-30s  %s\n", "ID", "NAME", "VER")
		for _, d := range defs {
			fmt.Printf("  %-36s  %-30s  %d\n", d.Id, d.Name, d.Version)
		}

	case "  Get workflow definition details":
		id := ask("Definition UUID")
		def, err := mc.GetWorkflowDefinition(ctx, id)
		if err != nil {
			return err
		}
		printJSON(def)

	case "  Start a workflow instance":
		defID := ask("Workflow definition ID")
		tenant := ask("Tenant")
		inputStr := ask("Initial input JSON (leave blank for none)")
		input, err := parseJSON(inputStr)
		if err != nil {
			return err
		}
		instanceID, endpoint, err := mc.StartWorkflow(ctx, defID, tenant, input)
		if err != nil {
			return err
		}
		fmt.Printf("  Started: instance_id=%s  endpoint=%s\n", instanceID, endpoint)

	case "  Get workflow instance status":
		id := ask("Instance UUID")
		state, err := mc.GetWorkflowState(ctx, id)
		if err != nil {
			return err
		}
		fmt.Printf("  Instance:      %s\n", state.WorkflowInstanceId)
		fmt.Printf("  Definition:    %s\n", state.WorkflowDefinitionId)
		fmt.Printf("  Status:        %s\n", state.Status.String())
		fmt.Printf("  Current state: %s\n", state.CurrentState)

	case "  List workflow instances":
		defID := ask("Definition ID filter (leave blank for all)")
		statusStr := ask("Status filter (RUNNING|COMPLETED|FAILED|CANCELED, blank=all)")
		statusEnum := workflowStatusStringToEnum(statusStr)
		workflows, err := mc.ListWorkflows(ctx, "", defID, statusEnum, 50)
		if err != nil {
			return err
		}
		if len(workflows) == 0 {
			fmt.Println("  No instances found.")
			return nil
		}
		fmt.Printf("  %-36s  %-12s  %s\n", "INSTANCE_ID", "STATUS", "CURRENT_STATE")
		for _, w := range workflows {
			fmt.Printf("  %-36s  %-12s  %s\n", w.WorkflowInstanceId, w.Status.String(), w.CurrentState)
		}

	case "  Stop an instance":
		id := ask("Instance UUID")
		archive := askConfirm("Archive event log and task history?")
		ok, err := mc.StopInstance(ctx, id, archive)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Stopped.")
		}

	case "  Cancel a workflow":
		id := ask("Instance UUID")
		ok, err := mc.CancelWorkflow(ctx, id)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Canceled.")
		}

	case "  Send signal to workflow":
		id := ask("Instance UUID")
		sigName := ask("Signal name (e.g. approval-granted)")
		payloadStr := ask("Payload JSON (leave blank for none)")
		payload, err := parseJSON(payloadStr)
		if err != nil {
			return err
		}
		ok, err := mc.SignalWorkflow(ctx, id, sigName, payload)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Signal sent.")
		}

	case "  List tasks for an instance":
		instanceID := ask("Instance UUID")
		statusStr := ask("Status filter (PENDING|ASSIGNED|COMPLETED|FAILED, blank=all)")
		tasks, err := mc.ListTasks(ctx, instanceID, statusStr, 50)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("  No tasks.")
			return nil
		}
		fmt.Printf("  %-36s  %-20s  %-12s  %s\n", "TASK_ID", "ACTIVITY", "STATUS", "AGENT")
		for _, t := range tasks {
			fmt.Printf("  %-36s  %-20s  %-12s  %s\n", t.TaskId, t.ActivityName, t.Status, t.AssignedAgentId)
		}

	case "  Get task details":
		id := ask("Task UUID")
		task, err := mc.GetTask(ctx, id)
		if err != nil {
			return err
		}
		printJSON(task)

	case "  List my tasks (by agent)":
		agentID := ask("Agent ID")
		tenant := ask("Tenant (leave blank for all)")
		tasks, err := mc.ListMyTasks(ctx, agentID, tenant, "")
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("  No tasks.")
			return nil
		}
		fmt.Printf("  %-36s  %-20s  %s\n", "TASK_ID", "ACTIVITY", "STATUS")
		for _, t := range tasks {
			fmt.Printf("  %-36s  %-20s  %s\n", t.TaskId, t.ActivityName, t.Status)
		}

	case "  Claim a task":
		taskID := ask("Task UUID")
		agentID := ask("Agent ID")
		resp, err := mc.ClaimTask(ctx, taskID, agentID)
		if err != nil {
			return err
		}
		if resp.Claimed {
			fmt.Printf("  Claimed. Activity: %s\n", resp.Task.GetActivityName())
		} else {
			fmt.Printf("  Not claimed: %s\n", resp.Reason)
		}

	case "  Release a task":
		taskID := ask("Task UUID")
		agentID := ask("Agent ID")
		reason := ask("Reason (optional)")
		ok, err := mc.ReleaseTask(ctx, taskID, agentID, reason)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Released.")
		}

	case "  Complete a task":
		taskID := ask("Task UUID")
		agentID := ask("Agent ID")
		summary := ask("Evidence summary")
		outputStr := ask("Output JSON (leave blank for none)")
		output, err := parseJSON(outputStr)
		if err != nil {
			return err
		}
		resp, err := mc.SubmitTaskResult(ctx, taskID, agentID, summary, output)
		if err != nil {
			return err
		}
		fmt.Printf("  Completed. Effect: %s\n", resp.Effect.String())

	case "  Fail a task":
		taskID := ask("Task UUID")
		agentID := ask("Agent ID")
		code := askWithDefault("Error code", "ERR")
		message := ask("Error message")
		ok, err := mc.SubmitTaskFailure(ctx, taskID, agentID, code, message)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Failed.")
		}

	case "  Update task progress":
		taskID := ask("Task UUID")
		agentID := ask("Agent ID")
		message := ask("Progress note")
		percentStr := ask("Percent complete (0-100, leave blank to skip)")
		var percent int32
		if percentStr != "" {
			fmt.Sscanf(percentStr, "%d", &percent)
		}
		ok, err := mc.UpdateTaskProgress(ctx, taskID, agentID, message, percent)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Progress updated.")
		}

	case "  Reassign a task":
		taskID := ask("Task UUID")
		fromAgent := ask("From agent ID")
		toAgent := ask("To agent ID (leave blank to return to pool)")
		reason := ask("Reason (optional)")
		ok, err := mc.ReassignTask(ctx, taskID, fromAgent, toAgent, reason)
		if err != nil {
			return err
		}
		if ok {
			fmt.Println("  Reassigned.")
		}

	case "  Get task history":
		taskID := ask("Task UUID")
		entries, err := mc.GetTaskHistory(ctx, taskID)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("  No history.")
			return nil
		}
		fmt.Printf("  %-26s  %-12s  %-20s  %s\n", "TIMESTAMP", "EVENT", "AGENT", "MESSAGE")
		for _, e := range entries {
			ts := ""
			if e.Timestamp != nil {
				ts = e.Timestamp.AsTime().Format("2006-01-02T15:04:05Z")
			}
			fmt.Printf("  %-26s  %-12s  %-20s  %s\n", ts, e.EventType, e.AgentId, e.Message)
		}

	case "  Start agent worker":
		if err := runAgentStart(); err != nil {
			return err
		}

	case "  Exit":
		fmt.Println("Goodbye.")
		os.Exit(0)

	default:
		// separator lines — do nothing
	}

	return nil
}
