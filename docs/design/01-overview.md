# Goop Language Overview

## What Goop is

Goop is a compiled, statically typed programming language with ML-family syntax that targets Go. It is designed for programmers who want the type safety, pattern matching, and inference of OCaml or F#, but who also want Go's runtime, concurrency model, standard library, and deployment characteristics.

## Core principles

1. **Type safety first.** Null safety, exhaustive pattern matching, sound inference, and explicit error handling are defaults, not opt-in features.
2. **Compiles to idiomatic Go.** The output must be readable and maintainable by Go programmers. Goop is not a runtime layer; it is a source-to-source compiler.
3. **Eat our own dog food.** The Goop compiler itself is written in Go, demonstrating that the target ecosystem is suitable for real software.
4. **Pragmatic, not academic.** Borrow proven ideas from ML, F#, Rust, and Go. Skip research-grade features unless they can be implemented soundly and lowered honestly.
5. **Interop is the point.** Goop code must be able to call Go libraries, and Go code must be able to call the Go emitted by Goop.
6. **Incremental adoption.** Teams can introduce Goop file-by-file and feature-by-feature.

## Relationship to other languages

| Language | What Goop borrows | What Goop rejects |
|---|---|---|
| **OCaml** | Syntax, modules, type inference, ADTs, pattern matching | Objects, class types, full module functors |
| **F#** | Computation-expression style (`result { ... }`), active patterns, pipeline syntax | .NET runtime dependence, F#'s object model |
| **Rust** | Result/Option discipline, ownership vocabulary for resources, exhaustive match | Borrow checker complexity, lifetime annotations as a user-facing feature |
| **Go** | Runtime, goroutines, channels, standard library, deployment model | Nil everywhere, verbose error handling, lack of sum types |
| **Kit** | Active patterns, match macros (`is`/`as`/`guard`), closed-by-default records | Overclaimed refinement/linear/effect features, capability model (no Go enforcement) |
| **Dingo** | Interface-based sum type lowering, `?` error propagation, source maps, feature flags | Rust/TS syntax, partial null-safety implementation |

## The design constraint

> **The Go emitted by the Goop compiler must be the kind of Go a senior Go engineer would write by hand if asked to express the same program in idiomatic Go.**

This constraint governs every language decision. It means:

- Sum types lower to Go interfaces + per-variant structs (the pattern used by `go/ast`).
- `match` lowers to Go type switches.
- `?` lowers to `if err != nil { return ... }`.
- Goop does not ship features that evaporate at the Go boundary (capabilities, effects, ownership checks that don't survive lowering).

## Non-goals for v1

- Self-hosting (the bootstrap compiler is written in Go).
- Full dependent types (lightweight runtime refinement contracts with `where` clauses are implemented; full SMT-based dependent types are deferred — see `docs/design/08-deferred-features-analysis.md`).
- A borrow checker with lifetimes (opt-in linear resource types with discharge checking are implemented; full ownership/lifetimes are deferred).
- A resumptive effect system (erased row-polymorphic effect tracking is implemented; resumptive handlers are rejected — see `docs/design/08-deferred-features-analysis.md`).
- Direct machine-code output.
- Replacing Go entirely.

## Target audience

- Go teams that want stronger types without leaving the ecosystem.
- ML/Rust developers who need Go's deployment and library story.
- Domain-specific tooling where correctness matters (trading systems, infrastructure, protocols).
