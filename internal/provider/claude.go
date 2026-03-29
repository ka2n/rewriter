package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ka2n/rewriter/internal/rules"
)

type Claude struct{}

func (c *Claude) Name() string { return "claude" }

func (c *Claude) Scopes() ([]Scope, Scope) {
	return []Scope{ScopeUser, ScopeProject, ScopeLocal}, ScopeUser
}

// RunHook handles Claude Code PreToolUse hook protocol.
// See: https://docs.anthropic.com/en/docs/claude-code/hooks
// Input:  {"tool_name":"Bash","tool_input":{"command":"..."}}
// Output: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"...","updatedInput":{"command":"..."}}}
func (c *Claude) RunHook(rs *rules.RuleSet, chains []string) {
	data := readStdin()

	var input struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing input: %v\n", err)
		os.Exit(1)
	}

	if input.ToolName != "Bash" || input.ToolInput.Command == "" {
		exitSilent()
	}

	command := input.ToolInput.Command
	changed := false

	if rewritten, ok := tryRewrite(command, rs); ok {
		command = rewritten
		changed = true
	}

	if chainCmd, ok := runChains(chains, data, command, claudeExtractCommand, claudeBuildInput); ok {
		command = chainCmd
		changed = true
	}

	if !changed {
		exitSilent()
	}

	writeJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":           "PreToolUse",
			"permissionDecision":       "allow",
			"permissionDecisionReason": "rewriter auto-rewrite",
			"updatedInput":             map[string]string{"command": command},
		},
	})
}

func claudeExtractCommand(output []byte) (string, bool) {
	// Chain hooks may return either:
	// {"hookSpecificOutput":{"updatedInput":{"command":"..."}}}  (wrapped)
	// {"permissionDecision":"allow","updatedInput":{"command":"..."}}  (legacy flat)
	var wrapped struct {
		HookSpecificOutput struct {
			UpdatedInput struct {
				Command string `json:"command"`
			} `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if json.Unmarshal(output, &wrapped) == nil && wrapped.HookSpecificOutput.UpdatedInput.Command != "" {
		return wrapped.HookSpecificOutput.UpdatedInput.Command, true
	}
	var flat struct {
		UpdatedInput struct {
			Command string `json:"command"`
		} `json:"updatedInput"`
	}
	if json.Unmarshal(output, &flat) == nil && flat.UpdatedInput.Command != "" {
		return flat.UpdatedInput.Command, true
	}
	return "", false
}

func claudeBuildInput(original []byte, command string) []byte {
	var obj map[string]any
	json.Unmarshal(original, &obj)
	ti, _ := obj["tool_input"].(map[string]any)
	if ti == nil {
		ti = make(map[string]any)
	}
	ti["command"] = command
	obj["tool_input"] = ti
	out, _ := json.Marshal(obj)
	return out
}

func (c *Claude) Init(rewriterPath string, scope Scope) error {
	var settingsPath string
	switch scope {
	case ScopeUser:
		settingsPath = filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")
	case ScopeProject:
		settingsPath = filepath.Join(".claude", "settings.json")
	case ScopeLocal:
		settingsPath = filepath.Join(".claude", "settings.local.json")
	}

	hookCmd := rewriterPath + " --claude"

	return patchJSONFile(settingsPath, func(obj map[string]any) (map[string]any, error) {
		hooks, _ := obj["hooks"].(map[string]any)
		if hooks == nil {
			hooks = make(map[string]any)
		}

		preToolUse, _ := hooks["PreToolUse"].([]any)
		if hookExists(preToolUse, hookCmd) {
			fmt.Printf("claude: hook already installed in %s\n", settingsPath)
			return obj, nil
		}

		entry := map[string]any{
			"matcher": "Bash",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCmd,
				},
			},
		}
		preToolUse = append(preToolUse, entry)
		hooks["PreToolUse"] = preToolUse
		obj["hooks"] = hooks

		fmt.Printf("claude: patched %s\n", settingsPath)
		return obj, nil
	})
}
