package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/enriquepascalin/awm-cli/internal/agent"
	"github.com/enriquepascalin/awm-cli/internal/client"
	"github.com/enriquepascalin/awm-cli/internal/config"
	"github.com/enriquepascalin/awm-cli/internal/executor"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create gRPC client connection to the orchestrator
	grpcClient, err := client.NewGRPCClient(cfg.Orchestrator.Address, cfg.Auth.Token)
	if err != nil {
		log.Fatalf("Failed to create gRPC client: %v", err)
	}
	defer grpcClient.Close()

	// Build typed executor and agent wrapper based on agent type.
	// The agent carries identity (ID, tenant, capabilities); the executor carries
	// the execution logic. Both must be wired together before creating the Registry.
	var ag agent.Agent
	var exec executor.Executor
	switch cfg.Agent.Type {
	case "human":
		humanExec := executor.NewHumanExecutor()
		exec = humanExec
		ag = agent.NewHumanAgent(cfg.Agent.ID, cfg.Agent.Tenant, cfg.Agent.Capabilities, humanExec)
	case "ai":
		llmExec := executor.NewLLMExecutor(executor.LLMConfig{
			Provider: cfg.AI.Provider,
			Model:    cfg.AI.Model,
			Endpoint: cfg.AI.Endpoint,
			APIKey:   cfg.AI.APIKey,
			Timeout:  cfg.AI.Timeout,
		})
		exec = llmExec
		ag = agent.NewLLMAgent(cfg.Agent.ID, cfg.Agent.Tenant, cfg.Agent.Capabilities, llmExec)
	case "service":
		svcExec := executor.NewServiceExecutor()
		exec = svcExec
		ag = agent.NewServiceAgent(cfg.Agent.ID, cfg.Agent.Tenant, cfg.Agent.Capabilities, svcExec)
	default:
		log.Fatalf("Unknown agent type: %s", cfg.Agent.Type)
	}

	// Create agent registry and register capabilities
	reg := agent.NewRegistry(ag, grpcClient, exec)
	if err := reg.Register(context.Background()); err != nil {
		log.Fatalf("Failed to register agent: %v", err)
	}
	defer reg.Unregister(context.Background())

	// Start the appropriate task consumption mode
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	log.Printf("AWM-CLI started as %s agent (ID: %s)", cfg.Agent.Type, cfg.Agent.ID)

	if cfg.Agent.UseStream {
		err = reg.RunStream(ctx)
	} else {
		err = reg.RunPolling(ctx)
	}
	if err != nil && err != context.Canceled {
		log.Fatalf("Agent loop error: %v", err)
	}
}