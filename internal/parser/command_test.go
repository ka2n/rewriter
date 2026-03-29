package parser

import (
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input      string
		name       string
		subcommand string
		envPrefix  []string
		redirects  []string
		args       []string
	}{
		{
			input:      "git status",
			name:       "git",
			subcommand: "status",
			args:       []string{"status"},
		},
		{
			input:      "git -C /tmp status --short",
			name:       "git",
			subcommand: "/tmp", // First non-flag token; git-specific flag semantics are not parsed.
			args:       []string{"-C", "/tmp", "status", "--short"},
		},
		{
			input:      "RUST_LOG=debug cargo test",
			name:       "cargo",
			subcommand: "test",
			envPrefix:  []string{"RUST_LOG=debug"},
			args:       []string{"test"},
		},
		{
			input:      "FOO=bar BAZ=qux git push",
			name:       "git",
			subcommand: "push",
			envPrefix:  []string{"FOO=bar", "BAZ=qux"},
			args:       []string{"push"},
		},
		{
			input:     "git status 2>&1",
			name:      "git",
			subcommand: "status",
			redirects: []string{"2>&1"},
			args:      []string{"status"},
		},
		{
			input:     "echo hello > output.txt",
			name:      "echo",
			subcommand: "hello",
			redirects: []string{">", "output.txt"},
			args:      []string{"hello"},
		},
		{
			input:      "cat file.txt",
			name:       "cat",
			subcommand: "file.txt",
			args:       []string{"file.txt"},
		},
		{
			input:      "docker compose up -d",
			name:       "docker",
			subcommand: "compose",
			args:       []string{"compose", "up", "-d"},
		},
		{
			input: "--version",
			name:  "--version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseCommand(%q) error: %v", tt.input, err)
			}
			if got.Name != tt.name {
				t.Errorf("Name: got %q, want %q", got.Name, tt.name)
			}
			if got.Subcommand != tt.subcommand {
				t.Errorf("Subcommand: got %q, want %q", got.Subcommand, tt.subcommand)
			}
			if !sliceEqual(got.EnvPrefix, tt.envPrefix) {
				t.Errorf("EnvPrefix: got %v, want %v", got.EnvPrefix, tt.envPrefix)
			}
			if !sliceEqual(got.Redirects, tt.redirects) {
				t.Errorf("Redirects: got %v, want %v", got.Redirects, tt.redirects)
			}
			if !sliceEqual(got.Args, tt.args) {
				t.Errorf("Args: got %v, want %v", got.Args, tt.args)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
