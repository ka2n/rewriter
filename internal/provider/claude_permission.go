package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ka2n/rewriter/internal/rules"
	"github.com/ka2n/rewriter/shellparse"
)

type claudePermissionVerdict int

const (
	claudePermissionAllow claudePermissionVerdict = iota
	claudePermissionDeny
	claudePermissionAsk
)

// permMatcher holds pre-compiled regexes for fast matching.
type permMatcher struct {
	denyExact map[string]bool     // exact/prefix patterns for O(1) lookup
	askExact  map[string]bool
	denyRe    *regexp.Regexp      // combined wildcard regex (nil = none)
	askRe     *regexp.Regexp
}

// permCache is the JSON-serializable form stored on disk.
type permCache struct {
	Key  string   `json:"k"`
	Deny []string `json:"d,omitempty"`
	Ask  []string `json:"a,omitempty"`
}

// claudeCheckPermission checks cmd against cached deny/ask rules.
// rs is used to pre-filter irrelevant rules on cache miss.
func claudeCheckPermission(cmd string, rs *rules.RuleSet) claudePermissionVerdict {
	if strings.ContainsAny(cmd, "\n\r") {
		return claudePermissionDeny
	}
	m := loadMatcher(rs)
	if m == nil {
		return claudePermissionAllow
	}
	return matchCmd(cmd, m)
}

// claudeCheckPermissionWithRules is the testable core.
func claudeCheckPermissionWithRules(cmd string, deny, ask []string) claudePermissionVerdict {
	m := buildMatcher(deny, ask)
	if m == nil {
		return claudePermissionAllow
	}
	return matchCmd(cmd, m)
}

func matchCmd(cmd string, m *permMatcher) claudePermissionVerdict {
	segs := shellparse.SplitCompound(cmd)
	anyAsk := false
	for _, seg := range segs {
		s := strings.TrimSpace(seg.Raw)
		if s == "" {
			continue
		}
		if matchSet(s, m.denyExact, m.denyRe) {
			return claudePermissionDeny
		}
		if !anyAsk && matchSet(s, m.askExact, m.askRe) {
			anyAsk = true
		}
	}
	if anyAsk {
		return claudePermissionAsk
	}
	return claudePermissionAllow
}

func matchSet(cmd string, exact map[string]bool, re *regexp.Regexp) bool {
	// Check exact/prefix patterns: try cmd itself, then progressively shorter prefixes
	if exact != nil {
		if exact[cmd] {
			return true
		}
		// Check if any exact pattern is a prefix of cmd (with word boundary)
		for p := range exact {
			if strings.HasPrefix(cmd, p+" ") {
				return true
			}
		}
	}
	if re != nil {
		return re.MatchString(cmd)
	}
	return false
}

func buildMatcher(deny, ask []string) *permMatcher {
	if len(deny) == 0 && len(ask) == 0 {
		return nil
	}
	m := &permMatcher{denyExact: map[string]bool{}, askExact: map[string]bool{}}
	var denyWild, askWild []string
	for _, p := range deny {
		rt, eff := parsePermRule(p)
		if rt == 2 {
			denyWild = append(denyWild, p)
		} else {
			m.denyExact[eff] = true
		}
	}
	for _, p := range ask {
		rt, eff := parsePermRule(p)
		if rt == 2 {
			askWild = append(askWild, p)
		} else {
			m.askExact[eff] = true
		}
	}
	m.denyRe = compileWildcards(denyWild)
	m.askRe = compileWildcards(askWild)
	return m
}

func compileWildcards(patterns []string) *regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}
	parts := make([]string, len(patterns))
	for i, p := range patterns {
		parts[i] = wildToRegex(p)
	}
	re, err := regexp.Compile("^(?:" + strings.Join(parts, "|") + ")$")
	if err != nil {
		return nil
	}
	return re
}

// parsePermRule returns (0=exact, 1=prefix, 2=wildcard) and the effective value.
func parsePermRule(p string) (int, string) {
	if strings.HasSuffix(p, ":*") {
		if v := p[:len(p)-2]; v != "" {
			return 1, v
		}
	}
	if hasWild(p) {
		return 2, p
	}
	return 0, p
}

func hasWild(p string) bool {
	if strings.HasSuffix(p, ":*") {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] == '*' {
			bs := 0
			for j := i - 1; j >= 0 && p[j] == '\\'; j-- {
				bs++
			}
			if bs%2 == 0 {
				return true
			}
		}
	}
	return false
}

func wildToRegex(pattern string) string {
	t := strings.TrimSpace(pattern)
	const sPH, bPH, wPH = "\x00S", "\x00B", "\x00W"
	var b strings.Builder
	stars := 0
	for i := 0; i < len(t); i++ {
		if t[i] == '\\' && i+1 < len(t) {
			if t[i+1] == '*' {
				b.WriteString(sPH)
				i++
				continue
			}
			if t[i+1] == '\\' {
				b.WriteString(bPH)
				i++
				continue
			}
		}
		if t[i] == '*' {
			stars++
		}
		b.WriteByte(t[i])
	}
	p := b.String()
	var buf strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '*' {
			buf.WriteString(wPH)
		} else {
			buf.WriteByte(p[i])
		}
	}
	e := regexp.QuoteMeta(buf.String())
	r := strings.ReplaceAll(e, regexp.QuoteMeta(wPH), ".*")
	r = strings.ReplaceAll(r, regexp.QuoteMeta(sPH), "\\*")
	r = strings.ReplaceAll(r, regexp.QuoteMeta(bPH), "\\\\")
	if strings.HasSuffix(r, " .*") && stars == 1 {
		r = r[:len(r)-3] + "( .*)?"
	}
	return "(?s)" + r
}

// --- Cache layer ---

func loadMatcher(rs *rules.RuleSet) *permMatcher {
	paths := claudeSettingsPaths()
	key := cacheKey(paths)

	// Try disk cache
	if c := readCache(key); c != nil {
		return buildMatcher(c.Deny, c.Ask)
	}

	// Load from settings files
	deny, ask := loadRulesFromFiles(paths)
	if len(deny) == 0 && len(ask) == 0 {
		writeCache(&permCache{Key: key})
		return nil
	}

	// Pre-filter by rewrite rule commands
	if rs != nil {
		cmds := rs.Commands()
		deny = filterByCommands(deny, cmds)
		ask = filterByCommands(ask, cmds)
	}

	writeCache(&permCache{Key: key, Deny: deny, Ask: ask})
	return buildMatcher(deny, ask)
}

// filterByCommands keeps only patterns that could match commands in cmds.
func filterByCommands(patterns, cmds []string) []string {
	if len(cmds) == 0 {
		return patterns
	}
	var out []string
	for _, p := range patterns {
		rt, eff := parsePermRule(p)
		switch rt {
		case 2: // wildcard — keep (could match anything)
			out = append(out, p)
		default: // exact or prefix
			first, _, _ := strings.Cut(eff, " ")
			for _, c := range cmds {
				if first == c {
					out = append(out, p)
					break
				}
			}
		}
	}
	return out
}

func loadRulesFromFiles(paths []string) (deny, ask []string) {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var s map[string]any
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		perms, ok := s["permissions"].(map[string]any)
		if !ok {
			continue
		}
		deny = appendBash(perms["deny"], deny)
		ask = appendBash(perms["ask"], ask)
	}
	return
}

func appendBash(v any, target []string) []string {
	arr, ok := v.([]any)
	if !ok {
		return target
	}
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			continue
		}
		if inner, ok := strings.CutPrefix(s, "Bash("); ok {
			if pat, ok := strings.CutSuffix(inner, ")"); ok {
				target = append(target, pat)
			}
		}
	}
	return target
}

func cacheKey(paths []string) string {
	h := sha256.New()
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(h, "%s:missing;", p)
		} else {
			fmt.Fprintf(h, "%s:%d:%d;", p, info.Size(), info.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func cachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "rewriter", "claude-perm.json")
}

func readCache(key string) *permCache {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return nil
	}
	var c permCache
	if json.Unmarshal(data, &c) != nil || c.Key != key {
		return nil
	}
	// Cache is valid for 1 hour max as safety net
	info, err := os.Stat(cachePath())
	if err != nil || time.Since(info.ModTime()) > time.Hour {
		return nil
	}
	return &c
}

func writeCache(c *permCache) {
	p := cachePath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	data, _ := json.Marshal(c)
	os.WriteFile(p, data, 0o644)
}

func claudeSettingsPaths() []string {
	var paths []string
	if root := claudeFindProjectRoot(); root != "" {
		paths = append(paths, filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".claude", "settings.local.json"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".claude", "settings.local.json"))
	}
	return paths
}

func claudeFindProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, ".claude")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
