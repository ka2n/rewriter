package parser

import "github.com/ka2n/rewriter/shellparse"

type Command = shellparse.Command

func ParseCommand(raw string) (Command, error) {
	return shellparse.ParseCommand(raw)
}
