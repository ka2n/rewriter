package rewrite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ka2n/rewriter/internal/rules"
)

const testRules = `
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

func loadTestRules(t *testing.T) *rules.RuleSet {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.toml")
	if err := os.WriteFile(path, []byte(testRules), 0644); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestRewrite(t *testing.T) {
	rs := loadTestRules(t)

	tests := []struct {
		input   string
		want    string
		changed bool
	}{
		{"git status", "rtk git status", true},
		{"git status && cargo test", "rtk git status && cargo test", true},
		{"docker compose up -d", "podman compose up -d", true},
		{"cat file.txt", "bat file.txt", true},
		{"RUST_LOG=debug git push origin main", "RUST_LOG=debug rtk git push origin main", true},
		{"git status 2>&1", "rtk git status 2>&1", true},
		{"ssh myhost", "ssh myhost -o StrictHostKeyChecking=no", true},
		{"echo hello", "echo hello", false},
		{"git status | cat", "rtk git status | bat", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Rewrite(tt.input, rs)
			if got.Rewritten != tt.want {
				t.Errorf("Rewritten:\ngot  %q\nwant %q", got.Rewritten, tt.want)
			}
			if got.Changed != tt.changed {
				t.Errorf("Changed: got %v, want %v", got.Changed, tt.changed)
			}
		})
	}
}
