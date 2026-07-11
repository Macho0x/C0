# Goop 1.2.0

Lang embeds: hard-break rename to `import go` / `@[go]`, plus cgo-shaped `@[c]`.

## Highlights

- **Breaking:** `import golang`, `@golang`, and the `golang` keyword are removed — use `import go` and `@[go]`
- **`@[c]`:** inline C via cgo (preamble + `import "C"` + primitive `val` wrappers)
- **Docs:** [15-lang-embeds.md](docs/design/15-lang-embeds.md), tutorial interop chapter, [`cgo_demo.goop`](docs/examples/cgo_demo.goop)
- **Tests:** `c_embed_*` e2e; renamed `import_go` / `go_embed`; TextMate scopes for `@[go]` / `@[c]`

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` — 72 passed
- `goop check` on all `docs/examples/*.goop`
