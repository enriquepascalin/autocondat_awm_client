package config

import (
	"fmt"
	"os"
	"time"

	"github.com/enriquepascalin/awm-cli/internal/executor"
	"gopkg.in/yaml.v3"
)

// Config represents the complete CLI configuration.
type Config struct {
	Agent   AgentConfig          `yaml:"agent"`
	Workers []WorkerConfig       `yaml:"workers"`
	Auth    AuthConfig           `yaml:"auth"`
	LLM     executor.LLMConfig   `yaml:"llm,omitempty"`
	Log     LogConfig            `yaml:"log,omitempty"`
}

// AgentConfig holds the identity and behaviour of this client process.
// All worker connections share the same identity.
type AgentConfig struct {
	ID           string   `yaml:"id"`
	Type         string   `yaml:"type"`      // "human", "ai", "service"
	Tenant       string   `yaml:"tenant"`
	Capabilities []string `yaml:"capabilities"`
	UseStream    bool     `yaml:"use_stream"`
}

// WorkerConfig describes a single worker endpoint this agent connects to.
type WorkerConfig struct {
	Name    string     `yaml:"name"`    // human-readable label, e.g. "jira-workflow"
	Address string     `yaml:"address"` // host:port of the worker gRPC server
	Auth    AuthConfig `yaml:"auth,omitempty"` // overrides top-level auth when set
}

// AuthConfig holds authentication credentials.
type AuthConfig struct {
	Token        string `yaml:"token,omitempty"`
	ClientID     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
	TokenURL     string `yaml:"token_url,omitempty"`
}

// LogConfig controls log output format and verbosity.
type LogConfig struct {
	Format string `yaml:"format"` // "json" (default) or "text"
	Level  string `yaml:"level"`  // "debug", "info" (default), "warn", "error"
}

// Load reads the configuration from a YAML file (path from AWM_CONFIG env var,
// defaulting to configs/agent.yaml) and applies environment variable overrides.
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

	applyEnvOverrides(&cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyEnvOverrides replaces config values with environment variables when set.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("AWM_AGENT_ID"); v != "" {
		cfg.Agent.ID = v
	}
	if v := os.Getenv("AWM_AGENT_TYPE"); v != "" {
		cfg.Agent.Type = v
	}
	if v := os.Getenv("AWM_AGENT_TENANT"); v != "" {
		cfg.Agent.Tenant = v
	}
	if v := os.Getenv("AWM_AUTH_TOKEN"); v != "" {
		cfg.Auth.Token = v
	}
	if v := os.Getenv("AWM_LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := os.Getenv("AWM_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("AWM_LLM_ENDPOINT"); v != "" {
		cfg.LLM.Endpoint = v
	}
	if v := os.Getenv("AWM_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("AWM_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("AWM_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
}

// Validate ensures the configuration is complete and internally consistent.
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
	if len(c.Workers) == 0 {
		return fmt.Errorf("at least one entry under workers is required")
	}
	for i, w := range c.Workers {
		if w.Address == "" {
			return fmt.Errorf("workers[%d].address is required", i)
		}
	}
	if c.Agent.Type == "ai" {
		if c.LLM.Provider == "" {
			return fmt.Errorf("llm.provider is required for ai agent")
		}
		if c.LLM.Model == "" {
			return fmt.Errorf("llm.model is required for ai agent")
		}
		if c.LLM.Endpoint == "" {
			return fmt.Errorf("llm.endpoint is required for ai agent")
		}
		if c.LLM.Timeout == 0 {
			c.LLM.Timeout = 60 * time.Second
		}
	}
	// Apply log defaults
	if c.Log.Format == "" {
		c.Log.Format = "json"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	return nil
}

// AuthForWorker returns the auth config for a specific worker, falling back
// to the top-level auth when the worker has no override.
func (c *Config) AuthForWorker(w WorkerConfig) AuthConfig {
	if w.Auth.Token != "" || w.Auth.ClientID != "" {
		return w.Auth
	}
	return c.Auth
}
