# Changelog

## 0.8.0

### OCaml surface syntax
- Arrays via `Array.make`, `Array.length`, `arr.(i)`, `arr.(i) <- v`, and `'a array` types
- `for i = lo to hi do ... done` loops
- `begin ...; ... end` sequencing blocks
- Qualified constructors `Type.Ctor`

### Codegen
- Fix top-level record literal thunks, float paren precedence, option-in-record prescan, tuple match

### Tests & docs
- OCaml-style trading decision LUT test (375-cell universe)
- Eight new integration tests; design doc 13 and std-array reference

## 0.7.1

### Parser & LSP
- Allow `let _` discard bindings (idiomatic for `go` fire-and-forget)
- Fix LSP hover/definition/completion returning invalid JSON-RPC when result is null
- Check `scanner.Err()` after LSP header reads

### Cleanup
- Remove dead code: `parseExternDecl`, `localGoopPathForImport`, `writeOpenDependencies`
- Drop unused parameters in `parseLetDecl` and `diagnosticFromError`
- Promote `golang.org/x/tools` to a direct module dependency

## 0.7.0

### Formatter
- New `src/internal/fmt` package; `goop fmt` formats the parse tree (no desugar)
- Offside-rule layout for `match`, `if`, `let`; full `GoExpr` / `go (move ...)` support

### Concurrency safety
- Channel-mediated race tracking (`LINEAR008`, under `[check] concurrent`)
- Narrow static deadlock lint (`DEADLOCK001`, `[check] deadlock`)

## 0.6.0

### Compile-time safety
- Refinement call-site codegen: proven VCs skip guards; unproven emit call-site checks; exported entry guards for FFI
- Arithmetic refinement solver (interval arithmetic, overflow-safe)
- Linear `go` handoff for owned resources
- `go (move ...)` syntax for explicit goroutine transfer
- `goop.toml`: `concurrent` and `refinement_unproven` severity knobs

### Examples & docs
- `go_move.goop`, `linear_go_handoff.goop`, extended `refinement_solving.goop`
- Tutorial and README updated for new check config keys

## 0.5.5-dev

### README
- Language comparison matrix (Go / Rust / OCaml / F# / Goop)
- Trading bot safety summary table with example links
- Banner renders on light and dark mode via `<picture>` element
- Renamed banner asset to `goop-banner-whitebg.jpg`

### Editor (0.5.3)
- Goopher branding assets and Zed file icons
- Linguist language color updated to `#62c52e`

## 0.5.2

- Language tutorial and stdlib reference
- VS Code highlighting/icons, GitHub Linguist package
- Unified safety pipeline, NIL001, EXHAUST003, effect inference, newtypes
