// Package report renders Goop errors in Lisette-style graphical format.
package report

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Render turns a Goop error (already containing "file:line:col: msg") into
// a Lisette-style diagnostic using the provided source text.
// Ponytail: single function, no new error types, reuses existing locations.
func Render(err error, src []byte) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Parse "file:line:col: rest"
	parts := strings.SplitN(msg, ":", 4)
	if len(parts) < 4 {
		return msg // fallback for CLI errors without location
	}
	file := strings.TrimSpace(parts[0])
	line, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	col, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
	rest := strings.TrimSpace(parts[3])

	lines := strings.Split(string(src), "\n")
	if line < 1 || line > len(lines) {
		return msg
	}

	// Build the box (3-line window around the error)
	var b strings.Builder
	b.WriteString("✕ " + rest + "\n")
	b.WriteString(fmt.Sprintf("╭─[%s:%d:%d]\n", file, line, col))

	start := max(0, line-2)
	end := min(len(lines), line+1)
	for i := start; i < end; i++ {
		prefix := " "
		if i+1 == line {
			prefix = ">"
		}
		b.WriteString(fmt.Sprintf("%s %d │ %s\n", prefix, i+1, lines[i]))
		if i+1 == line {
			// underline
			indent := strings.Repeat(" ", col-1)
			b.WriteString("  · " + indent + "╰── " + rest + "\n")
		}
	}
	b.WriteString("╰────\n")
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RenderFromFile is the convenience form used by cmd/goop.
func RenderFromFile(err error, filename string) string {
	src, _ := os.ReadFile(filename)
	return Render(err, src)
}