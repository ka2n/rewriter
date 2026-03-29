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
// Input:  {"tool_name":"Bash","tool_input":{"command":"..."}}
// Output: {"permissionDecision":"allow","updatedInput":{"command":"..."}}
func (c *Claude) RunHook(rs *rules.RuleSet) {
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

	rewritten, changed := tryRewrite(input.ToolInput.Command, rs)
	if !changed {
		exitSilent()
	}

	writeJSON(map[string]any{
		"permissionDecision": "allow",
		"updatedInput":       map[string]string{"command": rewritten},
	})
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
