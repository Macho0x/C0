# C0 Roadmap

This file presents the same task list as `TODO.md`, organized by development phase.

## Phase 0: Design and bootstrap

- [x] Establish design principles and language name.
- [x] Write core design documents.
- [x] Write specification drafts.
- [x] Bootstrap compiler skeleton in Go.
- [x] Tokenizer and lexer.
- [x] Parser producing an AST.
- [x] Pretty-printer for the AST.
- [x] CLI: `c0 lex`, `c0 parse`, `c0 check`, `c0 compile`, `c0 build`, `c0 resolve`, `c0 test`.

## Phase 1: Minimal viable compiler

- [x] Type checker with HM inference.
- [x] ADT and pattern matching support.
- [x] `option` and `result` built-ins.
- [x] `?` error propagation.
- [x] Basic Go code generation.
- [x] End-to-end compile of `hello.c0` to runnable Go binary.
- [x] Source map generation.
- [x] Exhaustive pattern-match checking.

## Phase 2: Usable language

- [x] Modules and imports.
- [x] Records and tuples.
- [x] Lists.
- [x] Standard library prelude.
- [x] `extern` Go interop.
- [x] Active patterns.
- [x] Match macros (`is`, `as`, `guard`).
- [x] Feature flags via `c0.toml`.
- [x] Test runner.
- [x] Mutable fields and bindings.

## Phase 3: Production features

- [x] Generics and type constructors (parametric polymorphism; HKT deferred).
- [x] Concurrency primitives (goroutines, channels, select).
- [x] `using` for resource cleanup.
- [x] Row polymorphism.
- [x] F#-style computation expressions (`result { ... }`; `async` reserved).
- [x] Layered lambda type inference (bidirectional inference + optional `go/types` fallback).
- [x] Effect rows (erased, row-polymorphic effect tracking in types).
- [x] Linear resource types (modal linearity, opt-in for resource-kinded types).
- [x] Region scopes (computation expression for scoped resource cleanup).
- [x] Runtime refinement contracts (`where` clauses lowering to runtime asserts).
- [x] Source locations for type errors (file:line:col in all error messages).
- [x] Goroutine sharing analysis (compile-time data race detection for mutable captures).
- [x] Runtime-safe `Chan.close` (closed flag wrapper, clear panic messages).
- [x] `OwnedChan` linear channel wrapper (compile-time close safety via linear discharge checking).
- [x] Built-in refinement solver (compile-time VC checking for integer arithmetic).
- [x] Nil channel detection (flow-sensitive initialization checking).
- [x] Unified import syntax (`import golang` / `import c0`).
- [x] Package manager (`c0 get`, `c0.lock`).

## Phase 4: Maturity

- [ ] Self-hosting compiler in C0.
- [x] IDE support (LSP) - full implementation with diagnostics, hover, definition, completion
- [x] Formatter (`c0 fmt` command)
- [ ] Comprehensive standard library.
- [ ] Documentation generator.
- [ ] Stable 1.0 release.

## Documentation

- [x] README
- [x] Design documents
- [x] Specification drafts
- [x] Examples
- [x] `c0.toml` project configuration
- [ ] Language tutorial
- [ ] Standard library reference
- [ ] Contributing guide

## Deferred or rejected

- Full dependent types (Idris/Agda style) — see `docs/design/08-deferred-features-analysis.md`.
- Borrow checker with lifetimes (Rust style) — modal linearity for resources is the right level for a GC'd target.
- Resumptive effect system — Go has no continuations; see `docs/design/08-deferred-features-analysis.md`.
- QTT 0-quantity erasure — premature without dependent types or proof terms.
- Capabilities that cannot be enforced at the Go boundary.
- Runtime layer or bytecode VM.
