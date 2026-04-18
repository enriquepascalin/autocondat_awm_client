package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/enriquepascalin/awm-cli/internal/agent"
	"github.com/enriquepascalin/awm-cli/internal/client"
	"github.com/enriquepascalin/awm-cli/internal/config"
	"github.com/enriquepascalin/awm-cli/internal/executor"
	"github.com/enriquepascalin/awm-cli/internal/logger"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage the background agent worker",
}

var agentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the agent worker(s) described in the config file",
	Long: `Reads agent.yaml (or the file specified by AWM_CONFIG_FILE), connects
to all configured worker endpoints, and starts processing tasks.
Runs until SIGINT or SIGTERM is received.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgentStart()
	},
}

func init() {
	agentCmd.AddCommand(agentStartCmd)
	rootCmd.AddCommand(agentCmd)
}

func runAgentStart() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	log := logger.New(cfg.Log.Format, cfg.Log.Level).With(
		"agent_id", cfg.Agent.ID,
		"agent_type", cfg.Agent.Type,
		"tenant", cfg.Agent.Tenant,
	)

	ag, exec, err := buildAgentFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to build agent: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("shutdown signal received", "signal", sig.String())
		cancel()
	}()

	log.Info("awm-cli starting", "workers", len(cfg.Workers))

	var wg sync.WaitGroup
	for _, w := range cfg.Workers {
		wg.Add(1)
		go func(wc config.WorkerConfig) {
			defer wg.Done()
			runWorkerConn(ctx, cfg, wc, ag, exec, log)
		}(w)
	}

	wg.Wait()
	log.Info("awm-cli stopped")
	return nil
}

func buildAgentFromConfig(cfg *config.Config) (agent.Agent, executor.Executor, error) {
	switch cfg.Agent.Type {
	case "human":
		exec := executor.NewHumanExecutor()
		ag := agent.NewHumanAgent(cfg.Agent.ID, cfg.Agent.Tenant, cfg.Agent.Capabilities, exec)
		return ag, exec, nil
	case "ai":
		exec := executor.NewLLMExecutor(cfg.LLM)
		ag := agent.NewLLMAgent(cfg.Agent.ID, cfg.Agent.Tenant, cfg.Agent.Capabilities, exec)
		return ag, exec, nil
	case "service":
		exec := executor.NewServiceExecutor()
		ag := agent.NewServiceAgent(cfg.Agent.ID, cfg.Agent.Tenant, cfg.Agent.Capabilities, exec)
		return ag, exec, nil
	default:
		return nil, nil, fmt.Errorf("unknown agent type %q; must be human, ai, or service", cfg.Agent.Type)
	}
}

func runWorkerConn(
	ctx context.Context,
	cfg *config.Config,
	wc config.WorkerConfig,
	ag agent.Agent,
	_ executor.Executor,
	log *logger.Logger,
) {
	log = log.With("worker", wc.Name, "address", wc.Address)

	auth := cfg.AuthForWorker(wc)
	grpcClient, err := client.NewGRPCClient(wc.Address, auth.Token)
	if err != nil {
		log.Error("failed to create gRPC client", "error", err)
		return
	}
	defer grpcClient.Close()

	reg := agent.NewRegistry(ag, grpcClient, log)
	if err := reg.Register(ctx); err != nil {
		log.Error("agent registration failed", "error", err)
		return
	}
	defer reg.Unregister(ctx) //nolint:errcheck

	log.Info("connected to worker")

	var runErr error
	if cfg.Agent.UseStream {
		runErr = reg.RunStream(ctx)
	} else {
		runErr = reg.RunPolling(ctx)
	}

	if runErr != nil && runErr != ctx.Err() {
		log.Error("agent loop exited with error", "error", runErr)
	}
}
