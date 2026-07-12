# ADT constructor shadowing (discovered via treelog)

Goop's builtin `option` uses `None` / `Some`. A user ADT that also declares
`| None` will confuse type inference when `None` / `Some X` appear in the same
module (e.g. `strategy option` returning `Some Full`).

**Guidance:** avoid constructor names `None` and `Some` in user ADTs; prefer
`NoMask`, `Absent`, etc.

## Codegen: builtin option match (1.5.1-dev)

`match opts.pretty with | Some p -> … | None -> …` used the ADT
type-switch path, which is invalid for concrete `Option*` structs.

**Fixed:** `isOptionMatch` / `emitOptionMatch` lower to `IsSome` /
`MustSome`, parallel to result matches. Unit-returning Go methods
(`Lock`/`Unlock`/`Info`) and `let _ =` are emitted as statements /
`_ =`, not value bindings.

Regression: `tests/option_match_test.goop`.

## Codegen: call lowering for native slog libraries (1.5.0)

Treelog migration away from `@[go]` needed:

1. Capitalized multi-arg apps (`Emit_request a b c d` → one Go call)
2. Unit erasure (`Now ()`, `NewID ()`, `colors.gray ()`)
3. If-as-expression on let RHS (`let service = if …`)

These shipped in **1.5.0** ([19-call-lowering.md](19-call-lowering.md)).
Handle/sanitize can move to native Goop next; Options should use `'a option`
and Scope’s error API should be `Fail` (not `Error`) to avoid the builtin
`error` clash.

Goop 1.3.0's `implements` declaration can emit the pointer-receiver method set
needed by `slog.Handler`, so Treelog can define a native handler without an
`@[go]` wrapper for the handler methods. See
[`go_implements_slog_handler.goop`](../examples/go_implements_slog_handler.goop).

## Cross-package libraries (1.6.0)

1.5 same-package lowering was not enough for consumers of a published Goop
package. Gaps forced lowercase bridges (`scope_new`, `opts_stdout_dev`, …).

**Fixed in 1.6.0:**

1. Imported record literals → `pkg.Options{…}` (option fields →
   `pkg.NewOptionTSome` / `None`)
2. Capitalized open-export calls flatten and qualify (`treelog.NewScope(…)`)
3. Multiline parenthesized apps (readable `appendAttr` chains)
4. `Buffer ptr` for `*bytes.Buffer` / pointer receivers
5. LSP URI decode so workspace paths with spaces resolve `module_root`

Libraries should expose Capitalized APIs and record types directly; do **not**
add lowercase bridges solely for codegen workarounds.

## Codegen: `let main` must stay `func main`

Export capitalization of top-level lets turned `let main () = ...` into
`func Main()`, which breaks `go build` / `goop test` (no program entry point).

**Fixed:** keep the Goop name `main` as Go `main`.

## Codegen: `if c then false else true` in return position

Nested `else if header then false else true` is recognized as the `not`
desugar and lowers to `!(c)`. When that if is itself in return position,
codegen used to emit a bare expression statement (`!(c)` with no `return`),
which is invalid Go.

**Fixed:** `emitReturnExpr` prefixes `return` for not-pattern ifs.

Regression: `tests/if_not_return_test.goop`.


`else begin match ... end` used to lower to an invalid IIFE:

```go
return func() string {
  return _s2 := Classify_key(key)  // illegal
  switch ...
}()
```

**Fixed:** `emitReturnExpr` for `BeginExpr` emits a statement block whose
last statement uses `emitReturnExpr` (see `emitBeginReturnBlock`). The
expression-IIFE path in `emitBegin` likewise routes statement-shaped tails
through `emitReturnExpr` instead of prefixing `return`.

Regression: `tests/begin_match_return_test.goop`.
