// Package connector provides a named-connector registry for service agents.
//
// A Connector encapsulates all integration-specific logic for a single external
// service (Jira, Slack, GitHub, etc.): its authentication, base URL, retry
// policy, and action vocabulary.  The ServiceExecutor looks up connectors by
// name from the registry; if a task's input carries a "connector" field that
// matches a registered name, execution is delegated to that connector.
// Tasks without a "connector" field fall back to the generic HTTP/script path.
//
// Registering connectors at startup:
//
//	svcExec := executor.NewServiceExecutor()
//	svcExec.RegisterConnector(connector.NewSlackConnector(slackCfg))
//	svcExec.RegisterConnector(connector.NewJiraConnector(jiraCfg))
package connector

import (
	"context"
	"fmt"
	"sync"
)

// Connector is the contract every named integration must fulfil.
type Connector interface {
	// Name returns the canonical identifier used in task input to select this
	// connector (e.g. "slack", "jira", "github").  Must be lower-case.
	Name() string

	// Execute carries out a single action for this integration.
	// action is a verb meaningful to the connector (e.g. "post-message", "create-issue").
	// params carries action-specific parameters extracted from the task input.
	// Returns a result map that is forwarded to the orchestrator as task output.
	Execute(ctx context.Context, action string, params map[string]interface{}) (map[string]interface{}, error)
}

// Registry holds named connectors and provides thread-safe lookup.
type Registry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

// NewRegistry returns an empty connector registry.
func NewRegistry() *Registry {
	return &Registry{connectors: make(map[string]Connector)}
}

// Register adds a connector to the registry.
// Panics if a connector with the same name is already registered, which
// catches misconfiguration at startup rather than silently overwriting.
func (r *Registry) Register(c Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := c.Name()
	if _, exists := r.connectors[name]; exists {
		panic(fmt.Sprintf("connector %q already registered", name))
	}
	r.connectors[name] = c
}

// Get returns the connector for the given name, or an error if not found.
func (r *Registry) Get(name string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.connectors[name]
	if !ok {
		return nil, fmt.Errorf("no connector registered for %q", name)
	}
	return c, nil
}

// Has reports whether a connector with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.connectors[name]
	return ok
}
