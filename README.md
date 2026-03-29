# rewriter

A generic command rewriting tool for AI coding agents. Rewrites shell commands based on configurable rules before execution.

## Install

```bash
go install github.com/ka2n/rewriter/cmd/rewriter@latest
```

## Usage

### Direct rewrite

```bash
rewriter "git status"           # → rtk git status
rewriter "docker compose up"    # → podman compose up
```

Exit code 0 + rewritten command on stdout if matched, exit code 1 if no match.

### Hook mode

Run as a pre-execution hook for AI coding agents:

```bash
rewriter --claude     # Claude Code PreToolUse hook
rewriter --copilot    # GitHub Copilot (VS Code) PreToolUse hook
rewriter --cursor     # Cursor beforeShellExecution hook
rewriter --gemini     # Gemini CLI BeforeTool hook
```

Each flag implements the agent's native hook protocol (JSON stdin/stdout).

### Setup

Install the hook into an agent's settings:

```bash
rewriter init claude            # patch ~/.claude/settings.json
rewriter init copilot           # create .github/hooks/rewriter.json
rewriter init cursor            # patch ~/.cursor/hooks.json
rewriter init gemini            # patch ~/.gemini/settings.json
```

Use `-s` to control scope:

```bash
rewriter init claude -s user      # global (default)
rewriter init claude -s project   # .claude/settings.json (shared via git)
rewriter init claude -s local     # .claude/settings.local.json (private)
```

Available scopes per agent:

| Agent | user | project | local |
|-------|------|---------|-------|
| claude | default | yes | yes |
| copilot | — | default | — |
| cursor | default | yes | — |
| gemini | default | yes | — |

## Rules

Rules are defined in `~/.config/rewriter/rules.toml` (respects `$XDG_CONFIG_HOME`):

```toml
[[rules]]
command = "git"
replace = "rtk git"

[[rules]]
command = "docker"
subcommand = "compose"
replace = "podman compose"

[[rules]]
command = "cat"
replace = "bat"

[[rules]]
command = "ssh"
append_flags = ["-o", "StrictHostKeyChecking=no"]
```

### Rule fields

| Field | Required | Description |
|-------|----------|-------------|
| `command` | yes | Command name to match (exact) |
| `subcommand` | no | Subcommand to match (exact, higher priority) |
| `replace` | no | Replace command prefix |
| `append_flags` | no | Flags to append |

### Matching

- Lookup is O(1) by command name (no regex)
- A rule with `subcommand` takes priority over command-only
- Compound commands (`&&`, `||`, `|`, `;`) are split and each segment is rewritten independently
- Environment prefixes (`VAR=val cmd`) and redirections (`2>&1`) are preserved

### Examples

```
git status                     → rtk git status
git status && cargo test       → rtk git status && cargo test
RUST_LOG=debug git push        → RUST_LOG=debug rtk git push
docker compose up -d           → podman compose up -d
git status 2>&1                → rtk git status 2>&1
ssh myhost                     → ssh myhost -o StrictHostKeyChecking=no
```

## Supported agents

| Agent | Hook protocol | Rewrite method |
|-------|--------------|----------------|
| Claude Code | PreToolUse (JSON stdin/stdout) | `updatedInput` |
| GitHub Copilot (VS Code) | preToolUse (JSON stdin/stdout) | `hookSpecificOutput.updatedInput` |
| Cursor | beforeShellExecution (JSON stdin/stdout) | `updated_input` |
| Gemini CLI | BeforeTool (JSON stdin/stdout) | `hookSpecificOutput.tool_input` |
