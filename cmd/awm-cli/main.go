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

	// Create task executor based on agent type
	var exec executor.Executor
	switch cfg.Agent.Type {
	case "human":
		exec = executor.NewHumanExecutor()
	case "ai":
		exec = executor.NewLLMExecutor(cfg.AI)
	case "service":
		exec = executor.NewServiceExecutor()
	default:
		log.Fatalf("Unknown agent type: %s", cfg.Agent.Type)
	}

	// Create agent registry and register capabilities
	reg := agent.NewRegistry(grpcClient, exec, cfg.Agent)
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