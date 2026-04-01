package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeParsePermRule(t *testing.T) {
	tests := []struct {
		pattern  string
		wantType int
		wantVal  string
	}{
		{"git push --force", 0, "git push --force"},
		{"npm:*", 1, "npm"},
		{"sudo:*", 1, "sudo"},
		{"git push *", 2, "git push *"},
		{`git push \*`, 0, `git push \*`},
		{"*", 2, "*"},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			gotType, gotVal := parsePermRule(tt.pattern)
			if gotType != tt.wantType || gotVal != tt.wantVal {
				t.Errorf("parsePermRule(%q) = (%d, %q), want (%d, %q)", tt.pattern, gotType, gotVal, tt.wantType, tt.wantVal)
			}
		})
	}
}

func TestClaudeCommandMatchesPattern(t *testing.T) {
	// Test via claudeCheckPermissionWithRules since commandMatchesPattern is now internal
	tests := []struct {
		cmd     string
		pattern string
		want    claudePermissionVerdict
	}{
		{"git push --force", "git push --force", claudePermissionDeny},
		{"git push --force origin main", "git push --force", claudePermissionDeny},
		{"git status", "git push --force", claudePermissionAllow},
		{"git push --forceful", "git push --force", claudePermissionAllow},
		{"gws drive list", "gws", claudePermissionDeny},
		{"gws", "gws", claudePermissionDeny},
		{"gwsomething", "gws", claudePermissionAllow},
		// Legacy prefix :*
		{"sudo rm -rf /", "sudo:*", claudePermissionDeny},
		{"sudo", "sudo:*", claudePermissionDeny},
		{"sudoedit /etc/hosts", "sudo:*", claudePermissionAllow},
		{"npm install", "npm:*", claudePermissionDeny},
		{"npm", "npm:*", claudePermissionDeny},
		{"npmx foo", "npm:*", claudePermissionAllow},
		// Wildcard *
		{"anything at all", "*", claudePermissionDeny},
		// empty command with "*" → allow (SplitCompound returns no segments)
		{"", "*", claudePermissionAllow},
		{"git push origin main", "git push *", claudePermissionDeny},
		{"git push", "git push *", claudePermissionDeny},
		{"git pushx", "git push *", claudePermissionAllow},
		{"npm run build", "npm run *", claudePermissionDeny},
		{"npm run", "npm run *", claudePermissionDeny},
		// Escaped wildcard
		{`echo \*`, `echo \*`, claudePermissionDeny},
		{"echo hello", `echo \*`, claudePermissionAllow},
		// Double colon prefix
		{"foo: bar", "foo::*", claudePermissionDeny},
		{"foo bar", "foo::*", claudePermissionAllow},
	}
	for _, tt := range tests {
		t.Run(tt.cmd+"__"+tt.pattern, func(t *testing.T) {
			got := claudeCheckPermissionWithRules(tt.cmd, []string{tt.pattern}, nil)
			if got != tt.want {
				t.Errorf("check(%q, deny=[%q]) = %v, want %v", tt.cmd, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestClaudeMatchWildcard(t *testing.T) {
	// Test wildcard patterns via buildMatcher
	tests := []struct {
		pattern string
		cmd     string
		want    bool
	}{
		{"*", "anything", true},
		{"git *", "git push", true},
		{"git *", "git", true},
		{"* run *", "npm run build", true},
		{"* run *", "npm run", false},
		// `echo \*` is classified as exact (escaped *), matches literal "echo \*"
		{`echo \*`, `echo \*`, true},
		{`echo \*`, "echo hello", false},
		// `path\\to\\file` is exact (no unescaped wildcard)
		{`path\\to\\file`, `path\\to\\file`, true},
		{"git push --force*", "git push --force-with-lease", true},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"__"+tt.cmd, func(t *testing.T) {
			m := buildMatcher([]string{tt.pattern}, nil)
			got := matchCmd(tt.cmd, m)
			want := claudePermissionAllow
			if tt.want {
				want = claudePermissionDeny
			}
			if got != want {
				t.Errorf("matchCmd(%q) with deny=[%q] = %v, want %v", tt.cmd, tt.pattern, got, want)
			}
		})
	}
}

func TestClaudeCheckPermissionWithRules(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		deny []string
		ask  []string
		want claudePermissionVerdict
	}{
		{"no rules = allow", "gws drive list", nil, nil, claudePermissionAllow},
		{"deny exact", "gws drive list", []string{"gws"}, nil, claudePermissionDeny},
		{"ask exact", "gws drive list", nil, []string{"gws"}, claudePermissionAsk},
		{"deny > ask", "gws drive list", []string{"gws"}, []string{"gws"}, claudePermissionDeny},
		{"compound deny", "echo hello && gws drive list", []string{"gws"}, nil, claudePermissionDeny},
		{"compound ask", "echo hello && gws drive list", nil, []string{"gws"}, claudePermissionAsk},
		{"compound deny > ask", "echo hello && gws drive list", []string{"gws"}, []string{"echo"}, claudePermissionDeny},
		{"wildcard deny all", "gws drive list", []string{"*"}, nil, claudePermissionDeny},
		{"no match", "gws drive list", []string{"git push --force"}, []string{"sudo"}, claudePermissionAllow},
		{"prefix deny", "gws drive list", []string{"gws:*"}, nil, claudePermissionDeny},
		{"wildcard pattern deny", "gws drive list", []string{"gws *"}, nil, claudePermissionDeny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := claudeCheckPermissionWithRules(tt.cmd, tt.deny, tt.ask)
			if got != tt.want {
				t.Errorf("claudeCheckPermissionWithRules(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestClaudeCheckPermission_NewlineDeny(t *testing.T) {
	if got := claudeCheckPermission("gws drive list\nrm -rf /", nil); got != claudePermissionDeny {
		t.Errorf("with newline = %v, want deny", got)
	}
	if got := claudeCheckPermission("gws drive list\r\nrm -rf /", nil); got != claudePermissionDeny {
		t.Errorf("with CRLF = %v, want deny", got)
	}
}

func TestClaudeLoadDenyAskRulesFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{
		"permissions": {
			"deny": ["Bash(git push --force)", "Bash(git push -f)", "Read(**/.env*)"],
			"ask": ["Bash(git push)", "Bash(gh pr merge)"]
		}
	}`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	deny, ask := loadRulesFromFiles(claudeSettingsPaths())
	wantDeny := []string{"git push --force", "git push -f"}
	wantAsk := []string{"git push", "gh pr merge"}

	if len(deny) < len(wantDeny) {
		t.Fatalf("deny: got %d, want >= %d", len(deny), len(wantDeny))
	}
	for i, w := range wantDeny {
		if deny[i] != w {
			t.Errorf("deny[%d] = %q, want %q", i, deny[i], w)
		}
	}
	if len(ask) < len(wantAsk) {
		t.Fatalf("ask: got %d, want >= %d", len(ask), len(wantAsk))
	}
	for i, w := range wantAsk {
		if ask[i] != w {
			t.Errorf("ask[%d] = %q, want %q", i, ask[i], w)
		}
	}
}

func TestFilterByCommands(t *testing.T) {
	patterns := []string{"git push --force", "sudo:*", "npm run *", "docker compose"}
	cmds := []string{"git", "docker"}
	got := filterByCommands(patterns, cmds)
	// Should keep: "git push --force" (git), "npm run *" (wildcard, always kept), "docker compose" (docker)
	// Should drop: "sudo:*" (prefix for sudo, not in cmds)
	want := []string{"git push --force", "npm run *", "docker compose"}
	if len(got) != len(want) {
		t.Fatalf("filterByCommands: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filterByCommands[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
