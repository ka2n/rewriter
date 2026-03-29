// Package provider implements agent-specific hook protocols and settings patching.
package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	// chains are additional hook commands to execute sequentially after
	// the rewriter's own rules are applied.
	RunHook(rs *rules.RuleSet, chains []string)

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

// runChains executes chain commands sequentially, passing inputJSON (with command
// updated) to each. extractCommand is a provider-specific function that extracts
// the updated command from a chain's JSON output. buildInput rebuilds the input
// JSON with the updated command for the next chain.
// Returns the final command and whether any chain changed it.
func runChains(chains []string, originalInput []byte, command string, extractCommand func(output []byte) (string, bool), buildInput func(input []byte, command string) []byte) (string, bool) {
	if len(chains) == 0 {
		return command, false
	}

	changed := false
	currentInput := buildInput(originalInput, command)

	for _, chainCmd := range chains {
		out, err := execChain(chainCmd, currentInput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rewriter: chain %q error: %v\n", chainCmd, err)
			continue
		}
		if len(bytes.TrimSpace(out)) == 0 {
			continue
		}

		if cmd, ok := extractCommand(out); ok {
			command = cmd
			changed = true
			currentInput = buildInput(originalInput, command)
		}
	}

	return command, changed
}

func execChain(command string, input []byte) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return nil, nil // non-zero exit = skip
		}
		return nil, err
	}
	return out, nil
}
