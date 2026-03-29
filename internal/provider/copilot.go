package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ka2n/rewriter/internal/rules"
)

type Copilot struct{}

func (c *Copilot) Name() string { return "copilot" }

func (c *Copilot) Scopes() ([]Scope, Scope) {
	return []Scope{ScopeProject}, ScopeProject
}

// RunHook handles GitHub Copilot (VS Code) PreToolUse hook protocol.
// Input:  {"tool_name":"runTerminalCommand","tool_input":{"command":"..."}}
// Output: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{"command":"..."}}}
func (c *Copilot) RunHook(rs *rules.RuleSet) {
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

	switch input.ToolName {
	case "runTerminalCommand", "Bash", "bash":
	default:
		exitSilent()
	}
	if input.ToolInput.Command == "" {
		exitSilent()
	}

	rewritten, changed := tryRewrite(input.ToolInput.Command, rs)
	if !changed {
		exitSilent()
	}

	writeJSON(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":      "PreToolUse",
			"permissionDecision": "allow",
			"updatedInput":       map[string]string{"command": rewritten},
		},
	})
}

// Init creates .github/hooks/rewriter.json for VS Code Copilot (project scope only).
func (c *Copilot) Init(rewriterPath string, _ Scope) error {
	hookCmd := rewriterPath + " --copilot"
	hooksDir := filepath.Join(".github", "hooks")
	hookFile := filepath.Join(hooksDir, "rewriter.json")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", hooksDir, err)
	}

	config := map[string]any{
		"hooks": map[string]any{
			"preToolUse": []any{
				map[string]any{
					"matcher": "runTerminalCommand|Bash|bash",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": hookCmd,
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(hookFile, append(data, '\n'), 0644); err != nil {
		return err
	}

	fmt.Printf("copilot: created %s\n", hookFile)
	return nil
}
