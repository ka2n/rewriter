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
func (c *Cursor) RunHook(rs *rules.RuleSet, chains []string) {
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

	command := input.Command
	changed := false

	if rewritten, ok := tryRewrite(command, rs); ok {
		command = rewritten
		changed = true
	}

	if chainCmd, ok := runChains(chains, data, command, cursorExtractCommand, cursorBuildInput); ok {
		command = chainCmd
		changed = true
	}

	if !changed {
		exitSilent()
	}

	writeJSON(map[string]any{
		"permission":    "allow",
		"updated_input": map[string]string{"command": command},
	})
}

func cursorExtractCommand(output []byte) (string, bool) {
	var resp struct {
		UpdatedInput struct {
			Command string `json:"command"`
		} `json:"updated_input"`
	}
	if json.Unmarshal(output, &resp) == nil && resp.UpdatedInput.Command != "" {
		return resp.UpdatedInput.Command, true
	}
	return "", false
}

func cursorBuildInput(original []byte, command string) []byte {
	var obj map[string]any
	json.Unmarshal(original, &obj)
	obj["command"] = command
	out, _ := json.Marshal(obj)
	return out
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
