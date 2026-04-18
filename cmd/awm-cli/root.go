package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/enriquepascalin/awm-cli/internal/client"
)

var (
	flagAddress string
	flagToken   string
)

var rootCmd = &cobra.Command{
	Use:   "awm-cli",
	Short: "AWM command-line interface for workflow and task management",
	Long: `awm-cli connects to an AWM Orchestrator and lets you manage
workflow definitions, start and monitor workflow instances,
and interact with tasks assigned to agents.

Run without arguments to enter the interactive shell menu.`,
	// No RunE here — sub-command dispatch is handled by cobra; no args → menu in main.
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagAddress, "address", "a", "localhost:9091", "Orchestrator gRPC address")
	rootCmd.PersistentFlags().StringVarP(&flagToken, "token", "t", "", "Bearer auth token")
}

// mgmtClient creates a short-lived ManagementClient for a single command invocation.
func mgmtClient() (*client.ManagementClient, error) {
	return client.NewManagementClient(flagAddress, flagToken)
}

// must prints the error and exits; used in simple command handlers.
func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// printJSON prints v as indented JSON.
func printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck
}

// parseJSON parses a JSON string into a map; returns empty map on empty input.
func parseJSON(s string) (map[string]interface{}, error) {
	if s == "" {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return m, nil
}

// background returns a plain background context (commands are short-lived).
func background() context.Context { return context.Background() }
