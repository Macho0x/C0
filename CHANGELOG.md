# Changelog

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
