package shellparse

import (
	"strings"

	"github.com/buildkite/shellwords"
)

// Command represents a parsed shell command segment.
type Command struct {
	EnvPrefix  []string // Environment variable assignments (e.g., "VAR=val").
	Name       string   // The command name (e.g., "git").
	Subcommand string   // The first non-flag argument after the command name (e.g., "status").
	Args       []string // All tokens after the command name (including subcommand and flags).
	Redirects  []string // Redirect tokens (e.g., "2>&1", ">file").
	Raw        string   // Original raw string.
}

// ParseCommand parses a single command segment (no compound operators) into a
// structured Command. It extracts env prefixes, redirections, command name,
// and subcommand.
func ParseCommand(raw string) (Command, error) {
	cmd := Command{Raw: raw}

	tokens, err := shellwords.SplitPosix(raw)
	if err != nil {
		return cmd, err
	}

	if len(tokens) == 0 {
		return cmd, nil
	}

	// Pass 1: separate redirections from other tokens.
	var nonRedirect []string
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if isRedirect(t) {
			cmd.Redirects = append(cmd.Redirects, t)
			// If the redirect doesn't include the target (e.g., ">" "file"),
			// consume the next token too.
			if !hasRedirectTarget(t) && i+1 < len(tokens) {
				i++
				cmd.Redirects = append(cmd.Redirects, tokens[i])
			}
		} else {
			nonRedirect = append(nonRedirect, t)
		}
	}

	// Pass 2: extract env prefix (VAR=val at the start).
	idx := 0
	for idx < len(nonRedirect) {
		if isEnvAssignment(nonRedirect[idx]) {
			cmd.EnvPrefix = append(cmd.EnvPrefix, nonRedirect[idx])
			idx++
		} else {
			break
		}
	}

	if idx >= len(nonRedirect) {
		return cmd, nil
	}

	// Command name
	cmd.Name = nonRedirect[idx]
	idx++

	// Remaining args
	if idx < len(nonRedirect) {
		cmd.Args = nonRedirect[idx:]

		// Subcommand: first arg that doesn't start with '-'
		for _, a := range cmd.Args {
			if !strings.HasPrefix(a, "-") {
				cmd.Subcommand = a
				break
			}
		}
	}

	return cmd, nil
}

// IsEnvAssignment checks if a token looks like VAR=val.
func IsEnvAssignment(s string) bool {
	return isEnvAssignment(s)
}

// IsRedirect checks if a token is a shell redirection operator.
func IsRedirect(s string) bool {
	return isRedirect(s)
}

func isEnvAssignment(s string) bool {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	name := s[:eq]
	for i, ch := range name {
		if ch == '_' {
			continue
		}
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
			continue
		}
		if i > 0 && ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}

func isRedirect(s string) bool {
	trimmed := strings.TrimLeft(s, "0123456789")
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, ">") ||
		strings.HasPrefix(trimmed, "<") ||
		strings.HasPrefix(trimmed, "&>")
}

func hasRedirectTarget(s string) bool {
	trimmed := strings.TrimLeft(s, "0123456789")
	trimmed = strings.TrimLeft(trimmed, ">&<")
	return trimmed != ""
}
