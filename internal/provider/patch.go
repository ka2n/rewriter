package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// patchJSONFile reads a JSON file, applies a mutation function, and writes it back.
// Creates the file (and parent dirs) if it doesn't exist, starting with an empty object.
func patchJSONFile(path string, mutate func(obj map[string]any) (map[string]any, error)) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	obj := make(map[string]any)

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &obj); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	obj, err = mutate(obj)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// hookEntry checks if a hook command already exists in a hooks array.
func hookExists(hooks []any, command string) bool {
	for _, h := range hooks {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		// Check nested hooks array (Claude Code / Gemini style)
		if innerHooks, ok := hm["hooks"].([]any); ok {
			for _, ih := range innerHooks {
				ihm, ok := ih.(map[string]any)
				if !ok {
					continue
				}
				if cmd, ok := ihm["command"].(string); ok && cmd == command {
					return true
				}
			}
		}
		// Check direct command field (Cursor style)
		if cmd, ok := hm["command"].(string); ok && cmd == command {
			return true
		}
	}
	return false
}
