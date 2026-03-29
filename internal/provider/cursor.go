package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ka2n/rewriter/internal/rules"
)

type Cursor struct{}

func (c *Cursor) Name() string { return "cursor" }

func (c *Cursor) Scopes() ([]Scope, Scope) {
	return []Scope{ScopeUser, ScopeProject}, ScopeUser
}

// RunHook handles Cursor beforeShellExecution hook protocol.
// See: https://cursor.com/docs/hooks
// Input:  {"hook_event_name":"beforeShellExecution","command":"...","cwd":"..."}
// Output: {"permission":"allow","updated_input":{"command":"..."}}
func (c *Cursor) RunHook(rs *rules.RuleSet) {
	data := readStdin()

	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing input: %v\n", err)
		os.Exit(1)
	}

	if input.Command == "" {
		exitSilent()
	}

	rewritten, changed := tryRewrite(input.Command, rs)
	if !changed {
		exitSilent()
	}

	writeJSON(map[string]any{
		"permission":    "allow",
		"updated_input": map[string]string{"command": rewritten},
	})
}

func (c *Cursor) Init(rewriterPath string, scope Scope) error {
	var settingsPath string
	switch scope {
	case ScopeUser:
		settingsPath = filepath.Join(os.Getenv("HOME"), ".cursor", "hooks.json")
	case ScopeProject:
		settingsPath = filepath.Join(".cursor", "hooks.json")
	}

	hookCmd := rewriterPath + " --cursor"

	return patchJSONFile(settingsPath, func(obj map[string]any) (map[string]any, error) {
		if _, ok := obj["version"]; !ok {
			obj["version"] = 1
		}

		hooks, _ := obj["hooks"].(map[string]any)
		if hooks == nil {
			hooks = make(map[string]any)
		}

		beforeShell, _ := hooks["beforeShellExecution"].([]any)
		if hookExists(beforeShell, hookCmd) {
			fmt.Printf("cursor: hook already installed in %s\n", settingsPath)
			return obj, nil
		}

		entry := map[string]any{
			"command": hookCmd,
		}
		beforeShell = append(beforeShell, entry)
		hooks["beforeShellExecution"] = beforeShell
		obj["hooks"] = hooks

		fmt.Printf("cursor: patched %s\n", settingsPath)
		return obj, nil
	})
}
