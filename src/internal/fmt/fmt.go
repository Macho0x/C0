// Package fmt pretty-prints Goop source from parse trees.
package fmt

import (
	"goop.dev/compiler/internal/ast"
)

// FormatModule returns canonical Goop source for a parsed module (no desugar).
func FormatModule(mod *ast.Module) string {
	if mod == nil {
		return ""
	}
	var p Printer

	if mod.Name != "" {
		p.Write("module " + mod.Name)
		p.Newline()
	}

	if len(mod.Imports) > 0 {
		p.Newline()
		p.Write("import (")
		p.Newline()
		for _, spec := range mod.Imports {
			formatImportSpec(&p, spec)
		}
		p.Write(")")
		p.Newline()
	}

	for i, d := range mod.Decls {
		if i > 0 || mod.Name != "" || len(mod.Imports) > 0 {
			p.Newline()
		}
		formatDecl(&p, d)
	}

	return p.String()
}
