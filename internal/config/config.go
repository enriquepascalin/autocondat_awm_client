package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent        AgentConfig        `yaml:"agent"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator"`
	Auth         AuthConfig         `yaml:"auth"`
	AI           AIConfig           `yaml:"ai,omitempty"`
	Dashboard    DashboardConfig    `yaml:"dashboard,omitempty"`
}

type AgentConfig struct {
	ID           string   `yaml:"id"`
	Type         string   `yaml:"type"` // human, ai, service
	Tenant       string   `yaml:"tenant"`
	Capabilities []string `yaml:"capabilities"`
	UseStream    bool     `yaml:"use_stream"`
}

type OrchestratorConfig struct {
	Address string `yaml:"address"`
}

type AuthConfig struct {
	Token string `yaml:"token"`
}

type AIConfig struct {
	Provider string `yaml:"provider"` // ollama, openai, etc.
	Model    string `yaml:"model"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"api_key,omitempty"`
}

type DashboardConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
}

func Load() (*Config, error) {
	path := os.Getenv("AWM_CONFIG")
	if path == "" {
		path = "configs/agent.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// Override with environment variables
	if v := os.Getenv("AWM_AGENT_ID"); v != "" {
		cfg.Agent.ID = v
	}
	if v := os.Getenv("AWM_ORCHESTRATOR_ADDR"); v != "" {
		cfg.Orchestrator.Address = v
	}
	if v := os.Getenv("AWM_AUTH_TOKEN"); v != "" {
		cfg.Auth.Token = v
	}
	if cfg.Agent.ID == "" {
		return nil, fmt.Errorf("agent.id is required")
	}
	if cfg.Orchestrator.Address == "" {
		return nil, fmt.Errorf("orchestrator.address is required")
	}
	return &cfg, nil
}