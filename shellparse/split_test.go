package shellparse

import (
	"testing"
)

func TestSplitCompound(t *testing.T) {
	tests := []struct {
		input string
		want  []Segment
	}{
		{
			input: "git status",
			want:  []Segment{{Raw: "git status", Op: OpNone}},
		},
		{
			input: "git status && cargo test",
			want: []Segment{
				{Raw: "git status", Op: OpAnd},
				{Raw: "cargo test", Op: OpNone},
			},
		},
		{
			input: "cmd1 || cmd2 && cmd3",
			want: []Segment{
				{Raw: "cmd1", Op: OpOr},
				{Raw: "cmd2", Op: OpAnd},
				{Raw: "cmd3", Op: OpNone},
			},
		},
		{
			input: "cat foo | grep bar",
			want: []Segment{
				{Raw: "cat foo", Op: OpPipe},
				{Raw: "grep bar", Op: OpNone},
			},
		},
		{
			input: "cmd1; cmd2; cmd3",
			want: []Segment{
				{Raw: "cmd1", Op: OpSeq},
				{Raw: "cmd2", Op: OpSeq},
				{Raw: "cmd3", Op: OpNone},
			},
		},
		{
			input: "server start &",
			want: []Segment{
				{Raw: "server start", Op: OpBg},
			},
		},
		{
			input: `echo "hello && world" && echo done`,
			want: []Segment{
				{Raw: `echo "hello && world"`, Op: OpAnd},
				{Raw: "echo done", Op: OpNone},
			},
		},
		{
			input: `echo 'pipes | here' | grep something`,
			want: []Segment{
				{Raw: `echo 'pipes | here'`, Op: OpPipe},
				{Raw: "grep something", Op: OpNone},
			},
		},
		{
			input: `echo $(git status && git diff) | cat`,
			want: []Segment{
				{Raw: `echo $(git status && git diff)`, Op: OpPipe},
				{Raw: "cat", Op: OpNone},
			},
		},
		{
			input: "echo `date` && echo done",
			want: []Segment{
				{Raw: "echo `date`", Op: OpAnd},
				{Raw: "echo done", Op: OpNone},
			},
		},
		{
			input: "git status 2>&1",
			want:  []Segment{{Raw: "git status 2>&1", Op: OpNone}},
		},
		{
			input: "git status 2>&1 && echo done",
			want: []Segment{
				{Raw: "git status 2>&1", Op: OpAnd},
				{Raw: "echo done", Op: OpNone},
			},
		},
		{
			input: "cmd &>output.txt",
			want:  []Segment{{Raw: "cmd &>output.txt", Op: OpNone}},
		},
		{
			input: `RUST_LOG=debug cargo test && echo ok`,
			want: []Segment{
				{Raw: "RUST_LOG=debug cargo test", Op: OpAnd},
				{Raw: "echo ok", Op: OpNone},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SplitCompound(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitCompound(%q):\ngot  %d segments: %+v\nwant %d segments: %+v",
					tt.input, len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i].Raw != tt.want[i].Raw || got[i].Op != tt.want[i].Op {
					t.Errorf("segment[%d]:\ngot  %+v\nwant %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
