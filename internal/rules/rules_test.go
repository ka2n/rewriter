package rules

import (
	"os"
	"path/filepath"
	"testing"
)

const testConfig = `
[[rules]]
command = "docker"
subcommand = "compose"
replace = "podman compose"

[[rules]]
command = "git"
replace = "rtk git"

[[rules]]
command = "cat"
replace = "bat"

[[rules]]
command = "ssh"
append_flags = ["-o", "StrictHostKeyChecking=no"]
`

func TestLoadAndMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.toml")
	if err := os.WriteFile(path, []byte(testConfig), 0644); err != nil {
		t.Fatal(err)
	}

	rs, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		cmd  string
		sub  string
		want *string // nil = no match, otherwise expected Replace value
	}{
		{"docker", "compose", strPtr("podman compose")},
		{"docker", "run", nil},                        // No rule for docker without subcommand
		{"git", "status", strPtr("rtk git")},          // Command-only match
		{"git", "push", strPtr("rtk git")},            // Command-only match
		{"cat", "file.txt", strPtr("bat")},
		{"ssh", "host", strPtr("")},                   // No replace, but has append_flags
		{"unknown", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.cmd+"/"+tt.sub, func(t *testing.T) {
			r := rs.Match(tt.cmd, tt.sub)
			if tt.want == nil {
				if r != nil {
					t.Errorf("expected no match, got %+v", r)
				}
				return
			}
			if r == nil {
				t.Fatal("expected match, got nil")
			}
			if r.Replace != *tt.want {
				t.Errorf("Replace: got %q, want %q", r.Replace, *tt.want)
			}
		})
	}
}

func TestLoadMissing(t *testing.T) {
	rs, err := LoadFrom("/nonexistent/rules.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules()) != 0 {
		t.Errorf("expected empty ruleset, got %d rules", len(rs.Rules()))
	}
}

func strPtr(s string) *string { return &s }
