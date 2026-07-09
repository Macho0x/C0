# Goop TODO

This file tracks the remaining work to make Goop a usable language. It is kept in sync with `docs/design/07-roadmap.md`.

## Compiler (bootstrap in Go)

- [x] Project structure and documentation
- [x] Bootstrap compiler skeleton in Go
- [x] Lexer with nested block comments and source locations
- [x] Recursive-descent parser for the core grammar
- [x] AST and pretty-printer
- [x] CLI: lex, parse, check, compile, build, test, get, resolve, fmt, lsp
- [x] Type checker with HM inference
- [x] Exhaustive pattern-match checking
- [x] Go code generation
- [x] Source maps
- [x] Imports and module resolution
- [x] Standard library prelude
- [x] Layered lambda type inference (bidirectional inference + optional `go/types` fallback)
- [x] Build system and package manager (`goop get`, `goop.lock`)

## Language features

- [x] Core syntax design
- [x] ADTs and pattern matching
- [x] Records and tuples
- [x] Lists
- [x] `option` and `result`
- [x] `?` error propagation
- [x] Mutable fields and bindings
- [x] Active patterns
- [x] Match macros (`is`, `as`, `guard`)
- [x] F#-style computation expressions (`result { ... }`, `async { ... }`)
- [x] Row polymorphism
- [x] Concurrency primitives (`go`, `chan`, `select`)
- [x] `using` for resource cleanup
- [x] Go interop (`import golang`, `@golang { }` embed blocks)
- [x] Unified imports (`import golang` / `import goop`, dot and aliased forms)
- [x] `private` module visibility
- [x] `%` modulo operator
- [x] Golang import 2-tuple returns
- [x] Flow-sensitive goroutine liveness (fewer race false positives)
- [x] std/ modules (`std.io`, `std.list`, `std.option`, `std.result`)
- [x] Effect rows (erased, row-polymorphic effect tracking in types)
- [x] Linear resource types (modal linearity, opt-in for resource-kinded types)
- [x] Region scopes (computation expression for scoped resource cleanup)
- [x] Runtime refinement contracts (`where` clauses lowering to runtime asserts)

## Compile-time safety checks

- [x] Source locations for type errors (file:line:col in all error messages)
- [x] Goroutine sharing analysis (compile-time data race detection for mutable captures)
- [x] Runtime-safe `Chan.close` (closed flag wrapper, clear panic messages)
- [x] `OwnedChan` linear channel wrapper (compile-time close safety via linear discharge checking)
- [x] Built-in refinement solver (compile-time VC checking for integer arithmetic)
- [x] Refinement call-site codegen (proven VCs skip guards; exported entry guards)
- [x] Arithmetic refinement solver extensions
- [x] Linear `go` handoff for owned resources
- [x] `go (move ...)` syntax for explicit goroutine transfer
- [x] `goop.toml` severities: `concurrent`, `refinement_unproven`

## Tooling

- [x] LSP server (features: diagnostics, hover, definition, completion)
- [x] Formatter (`goop fmt` command)
- [x] Test runner
- [ ] Documentation generator (`goop doc` — not started)

## Documentation

- [x] README (safety-first layout; interop and `@golang` sections)
- [x] Design documents
- [x] Specification drafts
- [x] Examples (`docs/examples/`; CI `goop check` on all files)
- [x] `goop.toml` project configuration
- [x] Package manager (`goop get`, `goop.lock`; see `docs/design/11-package-manager.md`)
- [x] Language tutorial — [docs/tutorial/](docs/tutorial/) (6 chapters + examples)
- [x] Standard library reference — [docs/stdlib/](docs/stdlib/) (prelude, builtins, std.*)
- [x] Contributing guide — [CONTRIBUTING.md](CONTRIBUTING.md) (build, editors, doc accuracy)

## Long term

- [ ] Self-hosting compiler
- [ ] Comprehensive standard library
- [ ] Stable 1.0 release

## Deferred or rejected

- Full dependent types (Idris/Agda style) — see `docs/design/08-deferred-features-analysis.md`.
- Borrow checker with lifetimes (Rust style) — modal linearity for resources is the right level for a GC'd target.
- Resumptive effect system — Go has no continuations; see `docs/design/08-deferred-features-analysis.md`.
- QTT 0-quantity erasure — premature without dependent types or proof terms.
- Capabilities that cannot be enforced at the Go boundary.
- Runtime layer or bytecode VM.
