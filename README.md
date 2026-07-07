<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo.png">
    <img alt="C0 logo" src="assets/logo.png" width="160">
  </picture>
</p>

<h1 align="center">C0</h1>

C0 is a statically typed systems language with ML-family syntax that compiles to readable, idiomatic Go.

It combines the type safety and expressiveness of OCaml/F# with Go's runtime, ecosystem, and deployment story. The goal is not to wrap Go in a thin syntax layer, but to give programmers a genuinely safer and more expressive source language whose output is still the kind of Go a senior engineer would write by hand.

> **Core design constraint:** The Go emitted by the C0 compiler must be readable, idiomatic, and debuggable by Go programmers who have never seen C0.

## Status

C0 is in early design and bootstrap implementation. The compiler is being written in Go and will self-host once the language is mature enough.

Error reporting uses Lisette-style graphical diagnostics with source context, precise spans, and actionable help (see `internal/report`).

`c0 build` supports true mixed `.c0` + `.go` projects in the same directory (including existing `go.mod` and hand-written Go files).

## Quick example

```c0
module Main

open Std.IO

type shape =
  | Circle of { radius: float }
  | Rect of { width: float; height: float }

let area (s: shape) : float =
  match s with
  | Circle { radius } -> 3.14159 *. radius *. radius
  | Rect { width; height } -> width *. height

let main () =
  let s = Circle { radius = 2.0 } in
  Console.print_line (Float.to_string (area s))
```

Compiles to a plain Go package with interface-based sum types and type switches.

## Project structure

```
C0/
├── docs/
│   ├── design/         # Design rationale and decisions
│   ├── spec/           # Formal grammar, semantics, and lowering rules
│   └── examples/       # Example C0 programs
├── src/                # Bootstrap compiler
│   ├── cmd/c0/         # CLI entry point (includes LSP)
│   └── internal/       # Compiler packages
│       ├── color/      # CLI color output
│       └── ...         # lexer, parser, typecheck, codegen, etc.
├── syntaxes/           # TextMate grammar (JSON)
├── editors/            # Editor extensions
│   ├── vscode/         # VSCode/Cursor extension
│   └── zed/            # Zed extension
├── tests/              # Compiler and end-to-end tests
├── README.md
└── TODO.md
```

## Design documents

- [Language overview](docs/design/01-overview.md)
- [Type system](docs/design/02-type-system.md)
- [Syntax](docs/design/03-syntax.md)
- [Go lowering strategy](docs/design/04-go-lowering.md)
- [Modules and packages](docs/design/05-modules-and-packages.md)
- [Effects and safety](docs/design/06-effects-and-safety.md)
- [Roadmap](docs/design/07-roadmap.md)

## Specification

- [Grammar](docs/spec/grammar.md)
- [Semantics](docs/spec/semantics.md)
- [Lowering to Go](docs/spec/lowering.md)

## Building the bootstrap compiler

```bash
cd src
go build ./cmd/c0
```

## Editor Support

### Syntax Highlighting

C0 uses OCaml/F# style syntax. Syntax highlighting is available for:

- **VSCode/Cursor**: See `editors/vscode/` for the extension (TextMate grammar)
- **Zed**: See `editors/zed/` for the extension configuration
- **CLI**: Use `c0 lex --color` for terminal colorization

### LSP Server

The LSP server is built into the `c0` binary. Run `c0 lsp` to start the language server:
```bash
c0 lsp
```

Provides:
- Real-time syntax error diagnostics
- Graphical error reporting with source context
- Future: hover, completion, go-to-definition

Configure your editor to use the LSP binary for `.c0` files.

## License

MIT / Apache-2.0 dual license (to be finalized).
