# ADT constructor shadowing (discovered via treelog)

Goop's builtin `option` uses `None` / `Some`. A user ADT that also declares
`| None` will confuse type inference when `None` / `Some X` appear in the same
module (e.g. `strategy option` returning `Some Full`).

**Guidance:** avoid constructor names `None` and `Some` in user ADTs; prefer
`NoMask`, `Absent`, etc.

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
