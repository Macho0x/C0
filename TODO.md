# C0 TODO

This file tracks the remaining work to make C0 a usable language. It is kept in sync with `docs/design/07-roadmap.md`.

## Compiler (bootstrap in Go)

- [x] Project structure and documentation
- [x] Bootstrap compiler skeleton in Go
- [x] Lexer with nested block comments and source locations
- [x] Recursive-descent parser for the core grammar
- [x] AST and pretty-printer
- [x] CLI: lex, parse, check, compile, build, resolve, test
- [x] Type checker with HM inference
- [x] Exhaustive pattern-match checking
- [x] Go code generation
- [x] Source maps
- [x] Imports and module resolution
- [x] Standard library prelude
- [x] Layered lambda type inference (bidirectional inference + optional `go/types` fallback)
- [x] Build system and package manager (`c0 get`, `c0.lock`)

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
- [x] `extern` Go interop (`@golang { }` embed blocks)
- [x] `private` module visibility
- [x] `%` modulo operator
- [x] Extern 2-tuple returns
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
- [x] Nil channel detection (flow-sensitive initialization checking)

## Tooling

- [x] LSP server (features: diagnostics, hover, definition, completion)
- [x] Formatter (`c0 fmt` command)
- [x] Test runner
- [ ] Documentation generator
- [ ] Package manager (`c0 get`)

## Documentation

- [x] README
- [x] Design documents
- [x] Specification drafts
- [x] Examples
- [x] `c0.toml` project configuration
- [ ] Language tutorial
- [ ] Standard library reference
- [ ] Contributing guide

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
