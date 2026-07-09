package fmt

import "strings"

// Printer accumulates formatted Goop source with indentation.
type Printer struct {
	buf    strings.Builder
	indent int
}

func (p *Printer) Write(s string) {
	p.buf.WriteString(s)
}

func (p *Printer) WriteIndent() {
	p.buf.WriteString(strings.Repeat("  ", p.indent))
}

func (p *Printer) Newline() {
	p.buf.WriteByte('\n')
}

func (p *Printer) Indent() { p.indent++ }

func (p *Printer) Dedent() {
	if p.indent > 0 {
		p.indent--
	}
}

func (p *Printer) String() string {
	return p.buf.String()
}
