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
// See: https://code.visualstudio.com/docs/copilot/customization/hooks
// Input:  {"tool_name":"runTerminalCommand","tool_input":{"command":"..."}}
// Output: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"...","updatedInput":{"command":"..."}}}
func (c *Copilot) RunHook(rs *rules.RuleSet, chains []string) {
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

	command := input.ToolInput.Command
	changed := false

	if rewritten, ok := tryRewrite(command, rs); ok {
		command = rewritten
		changed = true
	}

	// Copilot uses the same JSON format as Claude for chain extraction
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
