// Package parser provides shell command parsing utilities.
package parser

// Operator represents a shell compound operator.
type Operator string

const (
	OpNone Operator = ""
	OpAnd  Operator = "&&"
	OpOr   Operator = "||"
	OpPipe Operator = "|"
	OpSeq  Operator = ";"
	OpBg   Operator = "&"
)

// Segment represents a single command in a compound command line.
type Segment struct {
	Raw string   // The raw command string for this segment.
	Op  Operator // The operator that follows this segment (empty for the last one).
}

// SplitCompound splits a command line into segments separated by shell operators
// (&&, ||, |, ;, &). It respects single and double quotes, backslash escapes,
// and $(...) / `...` subshell syntax.
func SplitCompound(line string) []Segment {
	var segments []Segment
	var buf []byte
	runes := []byte(line)
	n := len(runes)
	i := 0

	for i < n {
		ch := runes[i]

		switch {
		// Single quote: consume until closing quote
		case ch == '\'':
			j := i + 1
			for j < n && runes[j] != '\'' {
				j++
			}
			if j < n {
				j++ // include closing quote
			}
			buf = append(buf, runes[i:j]...)
			i = j

		// Double quote: consume until closing quote, respecting backslash
		case ch == '"':
			j := i + 1
			for j < n && runes[j] != '"' {
				if runes[j] == '\\' && j+1 < n {
					j += 2
				} else {
					j++
				}
			}
			if j < n {
				j++ // include closing quote
			}
			buf = append(buf, runes[i:j]...)
			i = j

		// Backslash escape
		case ch == '\\' && i+1 < n:
			buf = append(buf, runes[i:i+2]...)
			i += 2

		// $(...) subshell
		case ch == '$' && i+1 < n && runes[i+1] == '(':
			depth := 1
			j := i + 2
			for j < n && depth > 0 {
				if runes[j] == '(' {
					depth++
				} else if runes[j] == ')' {
					depth--
				} else if runes[j] == '\'' || runes[j] == '"' {
					// skip quoted content inside subshell
					q := runes[j]
					j++
					for j < n && runes[j] != q {
						if runes[j] == '\\' && q == '"' && j+1 < n {
							j++
						}
						j++
					}
				}
				j++
			}
			buf = append(buf, runes[i:j]...)
			i = j

		// Backtick subshell
		case ch == '`':
			j := i + 1
			for j < n && runes[j] != '`' {
				if runes[j] == '\\' && j+1 < n {
					j++
				}
				j++
			}
			if j < n {
				j++
			}
			buf = append(buf, runes[i:j]...)
			i = j

		// && or & (but not >& which is a redirect)
		case ch == '&':
			// Check if this & is part of a redirect (e.g., >&, 2>&1)
			if len(buf) > 0 && buf[len(buf)-1] == '>' {
				buf = append(buf, ch)
				i++
			} else if i+1 < n && runes[i+1] == '&' {
				segments = append(segments, Segment{Raw: trimRight(buf), Op: OpAnd})
				buf = buf[:0]
				i += 2
			} else if i+1 < n && runes[i+1] == '>' {
				// &> redirect (e.g., &>file, &>>file)
				buf = append(buf, ch)
				i++
			} else {
				segments = append(segments, Segment{Raw: trimRight(buf), Op: OpBg})
				buf = buf[:0]
				i++
			}

		// ||
		case ch == '|' && i+1 < n && runes[i+1] == '|':
			segments = append(segments, Segment{Raw: trimRight(buf), Op: OpOr})
			buf = buf[:0]
			i += 2

		// |
		case ch == '|':
			segments = append(segments, Segment{Raw: trimRight(buf), Op: OpPipe})
			buf = buf[:0]
			i++

		// ;
		case ch == ';':
			segments = append(segments, Segment{Raw: trimRight(buf), Op: OpSeq})
			buf = buf[:0]
			i++

		default:
			// Skip leading whitespace after operator
			if len(buf) == 0 && (ch == ' ' || ch == '\t') {
				i++
				continue
			}
			buf = append(buf, ch)
			i++
		}
	}

	// Last segment
	if s := trimRight(buf); s != "" {
		segments = append(segments, Segment{Raw: s, Op: OpNone})
	}

	return segments
}

func trimRight(b []byte) string {
	i := len(b) - 1
	for i >= 0 && (b[i] == ' ' || b[i] == '\t') {
		i--
	}
	return string(b[:i+1])
}
