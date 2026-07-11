# Goop 1.2.1

Editor highlighting for lang embeds plus minor compiler hygiene.

## Highlights

- **`@[go]` / `@[c]` tags:** unified amber embed-marker color (`keyword.embed.goop`); inline Go/C bodies use normal grammar highlighting
- **VS Code extension 0.3.4** — reload window after update to pick up theme + grammar
- **Compiler:** remove dead `desugarIs` / `desugarAs`, unused `parseADTTypeKind`

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` — 72 passed
- `goop check` on all `docs/examples/*.goop`
