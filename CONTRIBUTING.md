# Contributing

## Development

```bash
git clone https://github.com/ka2n/rewriter.git
cd rewriter
go test ./...
go build -o rewriter ./cmd/rewriter/
```

If you have Nix installed:

```bash
nix develop
```

## Adding a new provider

1. Create `internal/provider/<name>.go`
2. Implement the `Provider` interface: `Name()`, `Scopes()`, `RunHook()`, `Init()`
3. Register in `provider.go` `All()` and `Names()`
4. Add `--<name>` flag in `cmd/rewriter/main.go`

## Adding rules

Rules go in `~/.config/rewriter/rules.toml`. See README for format.

## Testing

```bash
go test ./...
```

## Pull requests

- Keep changes focused
- Add tests for new functionality
- Run `go test ./...` before submitting
