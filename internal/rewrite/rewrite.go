// Package rewrite applies rules to rewrite shell commands.
package rewrite

import (
	"strings"

	"github.com/buildkite/shellwords"
	"github.com/ka2n/rewriter/internal/parser"
	"github.com/ka2n/rewriter/internal/rules"
)

// Result represents the outcome of rewriting a command.
type Result struct {
	Rewritten string // The rewritten command string.
	Changed   bool   // Whether the command was modified.
}

// Rewrite takes a full command line and a RuleSet, and returns the rewritten command.
func Rewrite(line string, rs *rules.RuleSet) Result {
	segments := parser.SplitCompound(line)
	if len(segments) == 0 {
		return Result{Rewritten: line}
	}

	changed := false
	var parts []string

	for _, seg := range segments {
		rewritten := rewriteSegment(seg.Raw, rs)
		if rewritten != seg.Raw {
			changed = true
		}

		part := rewritten
		if seg.Op != parser.OpNone {
			part += " " + string(seg.Op)
		}
		parts = append(parts, part)
	}

	return Result{
		Rewritten: strings.Join(parts, " "),
		Changed:   changed,
	}
}

func rewriteSegment(raw string, rs *rules.RuleSet) string {
	cmd, err := parser.ParseCommand(raw)
	if err != nil || cmd.Name == "" {
		return raw
	}

	rule := rs.Match(cmd.Name, cmd.Subcommand)
	if rule == nil {
		return raw
	}

	var parts []string

	// Env prefix
	parts = append(parts, cmd.EnvPrefix...)

	// Replace command (+ optional subcommand).
	if rule.Replace != "" {
		parts = append(parts, rule.Replace)
		replaceParts, _ := shellwords.SplitPosix(rule.Replace)
		skipSub := len(replaceParts) > 1 && rule.Subcommand != ""
		for _, a := range cmd.Args {
			if skipSub && a == cmd.Subcommand {
				skipSub = false
				continue
			}
			parts = append(parts, quoteIfNeeded(a))
		}
	} else {
		parts = append(parts, cmd.Name)
		for _, a := range cmd.Args {
			parts = append(parts, quoteIfNeeded(a))
		}
	}

	// Append flags from rule.
	for _, f := range rule.AppendFlags {
		parts = append(parts, quoteIfNeeded(f))
	}

	// Redirects at the end.
	parts = append(parts, cmd.Redirects...)

	return strings.Join(parts, " ")
}

// quoteIfNeeded adds quoting if a string contains spaces or special characters.
func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, " \t\n\"'\\$`|&;<>()") {
		return shellwords.QuotePosix(s)
	}
	return s
}
