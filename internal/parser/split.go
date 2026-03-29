// Package parser re-exports shellparse for internal use.
package parser

import "github.com/ka2n/rewriter/shellparse"

type Operator = shellparse.Operator

const (
	OpNone = shellparse.OpNone
	OpAnd  = shellparse.OpAnd
	OpOr   = shellparse.OpOr
	OpPipe = shellparse.OpPipe
	OpSeq  = shellparse.OpSeq
	OpBg   = shellparse.OpBg
)

type Segment = shellparse.Segment

func SplitCompound(line string) []Segment {
	return shellparse.SplitCompound(line)
}
