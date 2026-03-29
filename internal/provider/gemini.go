package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ka2n/rewriter/internal/rules"
)

type Gemini struct{}

func (g *Gemini) Name() string { return "gemini" }

func (g *Gemini) Scopes() ([]Scope, Scope) {
	return []Scope{ScopeUser, ScopeProject}, ScopeUser
}

// RunHook handles Gemini CLI BeforeTool hook protocol.
// See: https://geminicli.com/docs/hooks/reference/
// Input:  {"tool_name":"run_shell_command","tool_input":{"command":"..."}}
// Output: {"decision":"allow","hookSpecificOutput":{"tool_input":{"command":"..."}}}
func (g *Gemini) RunHook(rs *rules.RuleSet) {
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

	if input.ToolName != "run_shell_command" || input.ToolInput.Command == "" {
		exitSilent()
	}

	rewritten, changed := tryRewrite(input.ToolInput.Command, rs)
	if !changed {
		exitSilent()
	}

	writeJSON(map[string]any{
		"decision": "allow",
		"hookSpecificOutput": map[string]any{
			"tool_input": map[string]string{"command": rewritten},
		},
	})
}

func (g *Gemini) Init(rewriterPath string, scope Scope) error {
	var settingsPath string
	switch scope {
	case ScopeUser:
		settingsPath = filepath.Join(os.Getenv("HOME"), ".gemini", "settings.json")
	case ScopeProject:
		settingsPath = filepath.Join(".gemini", "settings.json")
	}

	hookCmd := rewriterPath + " --gemini"

	return patchJSONFile(settingsPath, func(obj map[string]any) (map[string]any, error) {
		hooks, _ := obj["hooks"].(map[string]any)
		if hooks == nil {
			hooks = make(map[string]any)
		}

		beforeTool, _ := hooks["BeforeTool"].([]any)
		if hookExists(beforeTool, hookCmd) {
			fmt.Printf("gemini: hook already installed in %s\n", settingsPath)
			return obj, nil
		}

		entry := map[string]any{
			"matcher": "run_shell_command",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCmd,
				},
			},
		}
		beforeTool = append(beforeTool, entry)
		hooks["BeforeTool"] = beforeTool
		obj["hooks"] = hooks

		fmt.Printf("gemini: patched %s\n", settingsPath)
		return obj, nil
	})
}
