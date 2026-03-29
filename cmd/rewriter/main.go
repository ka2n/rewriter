package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ka2n/rewriter/internal/provider"
	"github.com/ka2n/rewriter/internal/rewrite"
	"github.com/ka2n/rewriter/internal/rules"
)

// chainFlags collects multiple -c flag values.
type chainFlags []string

func (c *chainFlags) String() string { return strings.Join(*c, ", ") }
func (c *chainFlags) Set(v string) error {
	*c = append(*c, v)
	return nil
}

func main() {
	claude := flag.Bool("claude", false, "Claude Code hook mode")
	copilot := flag.Bool("copilot", false, "GitHub Copilot (VS Code) hook mode")
	cursor := flag.Bool("cursor", false, "Cursor hook mode")
	gemini := flag.Bool("gemini", false, "Gemini CLI hook mode")
	version := flag.Bool("version", false, "print version")
	var chains chainFlags
	flag.Var(&chains, "c", "chain command to execute after rewrite (repeatable)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rewriter [flags] [command...]\n\n")
		fmt.Fprintf(os.Stderr, "Rewrite:\n")
		fmt.Fprintf(os.Stderr, "  rewriter <command>              rewrite a command string\n")
		fmt.Fprintf(os.Stderr, "  rewriter --claude               Claude Code hook mode\n")
		fmt.Fprintf(os.Stderr, "  rewriter --copilot              GitHub Copilot hook mode\n")
		fmt.Fprintf(os.Stderr, "  rewriter --cursor               Cursor hook mode\n")
		fmt.Fprintf(os.Stderr, "  rewriter --gemini               Gemini CLI hook mode\n")
		fmt.Fprintf(os.Stderr, "  -c <command>                    chain hook command (repeatable)\n\n")
		fmt.Fprintf(os.Stderr, "Setup:\n")
		fmt.Fprintf(os.Stderr, "  rewriter init [-s scope] <name> install hook for an agent\n")
		fmt.Fprintf(os.Stderr, "    names: %s\n", strings.Join(provider.Names(), ", "))
		fmt.Fprintf(os.Stderr, "    scopes: user, project, local\n\n")
		fmt.Fprintf(os.Stderr, "  rewriter --version              print version\n")
	}
	flag.Parse()

	if *version {
		fmt.Println("rewriter 0.1.0")
		return
	}

	// Hook modes
	var p provider.Provider
	switch {
	case *claude:
		p = provider.Get("claude")
	case *copilot:
		p = provider.Get("copilot")
	case *cursor:
		p = provider.Get("cursor")
	case *gemini:
		p = provider.Get("gemini")
	}
	if p != nil {
		rs, err := rules.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading rules: %v\n", err)
			os.Exit(1)
		}
		p.RunHook(rs, chains)
		return
	}

	args := flag.Args()

	// init subcommand
	if len(args) >= 1 && args[0] == "init" {
		runInit(args[1:])
		return
	}

	// Direct rewrite
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	runRewrite(strings.Join(args, " "))
}

func runRewrite(cmd string) {
	rs, err := rules.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading rules: %v\n", err)
		os.Exit(1)
	}

	result := rewrite.Rewrite(cmd, rs)
	if !result.Changed {
		os.Exit(1)
	}

	fmt.Print(result.Rewritten)
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	scopeFlag := fs.String("s", "", "scope: user, project, local (default depends on agent)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rewriter init [-s scope] <name>\n")
		fmt.Fprintf(os.Stderr, "  names: %s\n", strings.Join(provider.Names(), ", "))
		fmt.Fprintf(os.Stderr, "  scopes: user, project, local\n")
	}
	fs.Parse(args)

	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	name := fs.Arg(0)
	p := provider.Get(name)
	if p == nil {
		fmt.Fprintf(os.Stderr, "unknown provider: %s\n", name)
		fmt.Fprintf(os.Stderr, "  available: %s\n", strings.Join(provider.Names(), ", "))
		os.Exit(1)
	}

	// Determine scope
	_, defaultScope := p.Scopes()
	scope := defaultScope
	if *scopeFlag != "" {
		scope = provider.Scope(*scopeFlag)
	}
	if err := provider.ValidateScope(p, scope); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Find the rewriter binary path
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding executable path: %v\n", err)
		os.Exit(1)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving executable path: %v\n", err)
		os.Exit(1)
	}

	if err := p.Init(exe, scope); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
