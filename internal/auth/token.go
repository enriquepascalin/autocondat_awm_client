package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// TokenProvider defines the interface for obtaining and refreshing authentication tokens.
type TokenProvider interface {
	// GetToken returns a valid token, refreshing it if necessary.
	GetToken(ctx context.Context) (string, error)
	// Stop releases any background resources (e.g., refresh goroutines).
	Stop()
}

// StaticTokenProvider returns a fixed token that never expires.
type StaticTokenProvider struct {
	token string
}

// NewStaticTokenProvider creates a provider from a static token string.
// If the token begins with "file://", the remainder is treated as a file path
// and the token is read from that file.
func NewStaticTokenProvider(token string) (*StaticTokenProvider, error) {
	if strings.HasPrefix(token, "file://") {
		path := strings.TrimPrefix(token, "file://")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read token file %s: %w", path, err)
		}
		token = strings.TrimSpace(string(data))
	}
	return &StaticTokenProvider{token: token}, nil
}

// GetToken returns the static token.
func (p *StaticTokenProvider) GetToken(ctx context.Context) (string, error) {
	return p.token, nil
}

// Stop is a no‑op for static tokens.
func (p *StaticTokenProvider) Stop() {}

// RefreshingTokenProvider periodically refreshes a token using a refresh function.
type RefreshingTokenProvider struct {
	mu              sync.RWMutex
	currentToken    string
	refreshFn       func(ctx context.Context) (string, time.Duration, error)
	refreshInterval time.Duration
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewRefreshingTokenProvider creates a provider that calls refreshFn to obtain a new token.
// The refreshFn should return the new token and its time‑to‑live (TTL). The provider will
// schedule the next refresh at TTL/2 or after the returned interval if specified.
func NewRefreshingTokenProvider(initialToken string, refreshFn func(ctx context.Context) (string, time.Duration, error)) *RefreshingTokenProvider {
	p := &RefreshingTokenProvider{
		currentToken: initialToken,
		refreshFn:    refreshFn,
		stopCh:       make(chan struct{}),
	}
	// Start background refresh if a refresh function is provided
	if refreshFn != nil {
		p.wg.Add(1)
		go p.refreshLoop()
	}
	return p
}

// refreshLoop runs in the background and periodically refreshes the token.
func (p *RefreshingTokenProvider) refreshLoop() {
	defer p.wg.Done()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		token, ttl, err := p.refreshFn(ctx)
		cancel()
		if err != nil {
			// On failure, retry after a short delay
			time.Sleep(10 * time.Second)
			continue
		}
		p.mu.Lock()
		p.currentToken = token
		p.mu.Unlock()

		// Refresh at half TTL or after ttl/2
		refreshAfter := ttl / 2
		if refreshAfter < 5*time.Second {
			refreshAfter = 5 * time.Second
		}
		select {
		case <-p.stopCh:
			return
		case <-time.After(refreshAfter):
		}
	}
}

// GetToken returns the current valid token.
func (p *RefreshingTokenProvider) GetToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.currentToken == "" {
		return "", fmt.Errorf("no token available")
	}
	return p.currentToken, nil
}

// Stop shuts down the background refresh goroutine.
func (p *RefreshingTokenProvider) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

// NewTokenProviderFromConfig creates the appropriate TokenProvider based on configuration.
// If a static token is provided, it is used directly.
// If OAuth2 client credentials are configured, it creates a refreshing provider.
func NewTokenProviderFromConfig(staticToken, clientID, clientSecret, tokenURL string) (TokenProvider, error) {
	if staticToken != "" {
		return NewStaticTokenProvider(staticToken)
	}
	if clientID != "" && clientSecret != "" && tokenURL != "" {
		// Initial token fetch
		initialToken, ttl, err := fetchOAuth2Token(context.Background(), clientID, clientSecret, tokenURL)
		if err != nil {
			return nil, fmt.Errorf("initial OAuth2 token fetch: %w", err)
		}
		refreshFn := func(ctx context.Context) (string, time.Duration, error) {
			return fetchOAuth2Token(ctx, clientID, clientSecret, tokenURL)
		}
		return NewRefreshingTokenProvider(initialToken, refreshFn), nil
	}
	// No authentication configured – return a provider that returns an empty string
	return NewStaticTokenProvider("")
}

// fetchOAuth2Token performs the client credentials grant.
func fetchOAuth2Token(ctx context.Context, clientID, clientSecret, tokenURL string) (string, time.Duration, error) {
	// Simplified OAuth2 implementation; in production, use golang.org/x/oauth2
	// For brevity, we'll assume a standard token endpoint returning JSON.
	// Placeholder – actual implementation would use http.Client.
	return "", 0, fmt.Errorf("OAuth2 not fully implemented; use static token or implement fetch")
}
