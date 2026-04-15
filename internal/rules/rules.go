// Package rules loads and matches rewrite rules from TOML config.
package rules

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Rule defines a single rewrite rule.
type Rule struct {
	Command     string   `toml:"command"`               // Required: command name to match (exact).
	Subcommand  string   `toml:"subcommand,omitempty"`   // Optional: subcommand to match (exact).
	Replace     string   `toml:"replace,omitempty"`      // Replace command (+ subcommand) prefix.
	AppendFlags []string `toml:"append_flags,omitempty"` // Flags to append to the command.
}

type config struct {
	Rules []Rule `toml:"rules"`
}

// RuleSet holds loaded rules indexed by command name for fast lookup.
type RuleSet struct {
	byCommand map[string][]Rule
}

// Load reads rules from the TOML config file at the standard location.
// If the file doesn't exist, returns an empty RuleSet.
func Load() (*RuleSet, error) {
	path := configPath()
	return LoadFrom(path)
}

// LoadFrom reads rules from a specific TOML file.
func LoadFrom(path string) (*RuleSet, error) {
	rs := &RuleSet{byCommand: make(map[string][]Rule)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rs, nil
		}
		return nil, err
	}

	var cfg config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	for _, r := range cfg.Rules {
		rs.byCommand[r.Command] = append(rs.byCommand[r.Command], r)
	}

	return rs, nil
}

// Match finds the best matching rule for a given command name and subcommand.
// A rule with both command and subcommand match takes priority over command-only.
// Returns nil if no rule matches.
func (rs *RuleSet) Match(command, subcommand string) *Rule {
	rules, ok := rs.byCommand[command]
	if !ok {
		return nil
	}

	var commandOnly *Rule
	for i := range rules {
		r := &rules[i]
		if r.Subcommand != "" {
			if r.Subcommand == subcommand {
				return r // Exact match with subcommand wins.
			}
			continue
		}
		if commandOnly == nil {
			commandOnly = r
		}
	}
	return commandOnly
}

// Rules returns all loaded rules.
func (rs *RuleSet) Rules() []Rule {
	var all []Rule
	for _, rules := range rs.byCommand {
		all = append(all, rules...)
	}
	return all
}

// Commands returns the set of command names that have rewrite rules.
func (rs *RuleSet) Commands() []string {
	cmds := make([]string, 0, len(rs.byCommand))
	for cmd := range rs.byCommand {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// ConfigPath returns the path to the rules TOML config file.
func ConfigPath() string {
	return configPath()
}

func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "rewriter", "rules.toml")
}
