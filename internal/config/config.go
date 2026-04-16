package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete CLI configuration.
type Config struct {
	Agent        AgentConfig        `yaml:"agent"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator"`
	Auth         AuthConfig         `yaml:"auth"`
	AI           AIConfig           `yaml:"ai,omitempty"`
	Dashboard    DashboardConfig    `yaml:"dashboard,omitempty"`
}

// AgentConfig holds the identity and behavior of the agent.
type AgentConfig struct {
	ID           string   `yaml:"id"`
	Type         string   `yaml:"type"` // "human", "ai", "service"
	Tenant       string   `yaml:"tenant"`
	Capabilities []string `yaml:"capabilities"`
	UseStream    bool     `yaml:"use_stream"`
}

// OrchestratorConfig holds the connection details for the orchestrator.
type OrchestratorConfig struct {
	Address string `yaml:"address"`
}

// AuthConfig holds authentication credentials.
type AuthConfig struct {
	Token string `yaml:"token"`
}

// AIConfig holds configuration for AI agents.
type AIConfig struct {
	Provider string        `yaml:"provider"` // "ollama", "openai"
	Model    string        `yaml:"model"`
	Endpoint string        `yaml:"endpoint"`
	APIKey   string        `yaml:"api_key,omitempty"`
	Timeout  time.Duration `yaml:"timeout"`
}

// DashboardConfig holds configuration for the local web dashboard.
type DashboardConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// Load reads the configuration from a YAML file (path from AWM_CONFIG or default)
// and overrides with environment variables. It returns a validated Config.
func Load() (*Config, error) {
	path := os.Getenv("AWM_CONFIG")
	if path == "" {
		path = "configs/agent.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Environment variable overrides
	if v := os.Getenv("AWM_AGENT_ID"); v != "" {
		cfg.Agent.ID = v
	}
	if v := os.Getenv("AWM_AGENT_TYPE"); v != "" {
		cfg.Agent.Type = v
	}
	if v := os.Getenv("AWM_AGENT_TENANT"); v != "" {
		cfg.Agent.Tenant = v
	}
	if v := os.Getenv("AWM_ORCHESTRATOR_ADDR"); v != "" {
		cfg.Orchestrator.Address = v
	}
	if v := os.Getenv("AWM_AUTH_TOKEN"); v != "" {
		cfg.Auth.Token = v
	}
	if v := os.Getenv("AWM_AI_PROVIDER"); v != "" {
		cfg.AI.Provider = v
	}
	if v := os.Getenv("AWM_AI_MODEL"); v != "" {
		cfg.AI.Model = v
	}
	if v := os.Getenv("AWM_AI_ENDPOINT"); v != "" {
		cfg.AI.Endpoint = v
	}
	if v := os.Getenv("AWM_AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate ensures the configuration is complete and consistent.
func (c *Config) Validate() error {
	if c.Agent.ID == "" {
		return fmt.Errorf("agent.id is required")
	}
	if c.Agent.Type == "" {
		return fmt.Errorf("agent.type is required")
	}
	if c.Agent.Tenant == "" {
		return fmt.Errorf("agent.tenant is required")
	}
	if c.Orchestrator.Address == "" {
		return fmt.Errorf("orchestrator.address is required")
	}
	if c.Agent.Type == "ai" {
		if c.AI.Provider == "" {
			return fmt.Errorf("ai.provider is required for ai agent")
		}
		if c.AI.Model == "" {
			return fmt.Errorf("ai.model is required for ai agent")
		}
		if c.AI.Endpoint == "" {
			return fmt.Errorf("ai.endpoint is required for ai agent")
		}
		if c.AI.Timeout == 0 {
			c.AI.Timeout = 60 * time.Second
		}
	}
	if c.Dashboard.Enabled && c.Dashboard.Port == 0 {
		c.Dashboard.Port = 3000
	}
	return nil
}
