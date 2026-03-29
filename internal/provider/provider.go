// Package provider implements agent-specific hook protocols and settings patching.
package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ka2n/rewriter/internal/rewrite"
	"github.com/ka2n/rewriter/internal/rules"
)

// Scope defines where the hook config is written.
type Scope string

const (
	ScopeUser    Scope = "user"    // Global (~/.agent/settings.json)
	ScopeProject Scope = "project" // Shared (.agent/settings.json in project root)
	ScopeLocal   Scope = "local"   // Private per-project (.agent/settings.local.json)
)

// Provider defines the interface for an AI coding agent integration.
type Provider interface {
	// Name returns the provider identifier (e.g., "claude", "copilot").
	Name() string

	// Scopes returns the supported scopes and the default scope.
	Scopes() (supported []Scope, defaultScope Scope)

	// RunHook reads JSON from stdin, rewrites commands, and writes the
	// agent-specific JSON response to stdout.
	RunHook(rs *rules.RuleSet)

	// Init patches the agent's settings file to register the rewriter hook.
	Init(rewriterPath string, scope Scope) error
}

// All returns all registered providers keyed by name.
func All() map[string]Provider {
	return map[string]Provider{
		"claude":  &Claude{},
		"copilot": &Copilot{},
		"cursor":  &Cursor{},
		"gemini":  &Gemini{},
	}
}

// Get returns a provider by name, or nil if not found.
func Get(name string) Provider {
	return All()[name]
}

// Names returns sorted provider names for display.
func Names() []string {
	return []string{"claude", "copilot", "cursor", "gemini"}
}

// ValidateScope checks if a scope is supported by the provider.
func ValidateScope(p Provider, scope Scope) error {
	supported, _ := p.Scopes()
	for _, s := range supported {
		if s == scope {
			return nil
		}
	}
	names := make([]string, len(supported))
	for i, s := range supported {
		names[i] = string(s)
	}
	return fmt.Errorf("%s does not support scope %q (available: %s)", p.Name(), scope, strings.Join(names, ", "))
}

// helpers shared across providers

func readStdin() []byte {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}
	return data
}

func tryRewrite(cmd string, rs *rules.RuleSet) (string, bool) {
	result := rewrite.Rewrite(cmd, rs)
	return result.Rewritten, result.Changed
}

func writeJSON(v any) {
	json.NewEncoder(os.Stdout).Encode(v)
}

func exitSilent() {
	os.Exit(0)
}
