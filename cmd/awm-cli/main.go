package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/enriquepascalin/awm-cli/internal/agent"
	"github.com/enriquepascalin/awm-cli/internal/client"
	"github.com/enriquepascalin/awm-cli/internal/config"
	"github.com/enriquepascalin/awm-cli/internal/executor"
	"github.com/enriquepascalin/awm-cli/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Logger not yet initialised; use fmt so the error is visible.
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Format, cfg.Log.Level).With(
		"agent_id", cfg.Agent.ID,
		"agent_type", cfg.Agent.Type,
		"tenant", cfg.Agent.Tenant,
	)

	// Build the agent once — identity is shared across all worker connections.
	ag, exec, err := buildAgent(cfg)
	if err != nil {
		log.Fatal("failed to build agent", "error", err)
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

	// One goroutine per worker endpoint.
	var wg sync.WaitGroup
	for _, w := range cfg.Workers {
		wg.Add(1)
		go func(wc config.WorkerConfig) {
			defer wg.Done()
			runWorkerConnection(ctx, cfg, wc, ag, exec, log)
		}(w)
	}

	wg.Wait()
	log.Info("awm-cli stopped")
}

// buildAgent constructs the typed executor and agent wrapper from configuration.
// The executor is returned separately so it satisfies executor.Executor where needed.
func buildAgent(cfg *config.Config) (agent.Agent, executor.Executor, error) {
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

// runWorkerConnection manages the full lifecycle of one agent ↔ worker connection.
// It registers the agent, then runs stream or polling mode until ctx is canceled.
func runWorkerConnection(
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
