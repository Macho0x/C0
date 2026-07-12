# Goop 1.8.0

Goop 1.8.0 delivers a **full Goop build**: edit `.goop`, run `goop build` /
`goop test`, and never see generated `.go` or `.map.json` in your project tree.

## Highlights

- **Cache-only artifacts:** `goop compile` and `goop build` write under
  `$GOOP_HOME/build` (default `~/.cache/goop/build`).
- **Real `goop build`:** transitive `import goop` dependencies are compiled
  into the sandbox and linked via `replace` — the same wiring `goop test`
  already used.
- **Source maps opt-in:** `--emit-map`; `--in-tree` remains for inspecting
  generated Go or mixed Go+Goop packages.
- **Clean trees:** no leaked `go.mod` beside sources; CI asserts examples stay
  free of generated `.go`.

## Workflow

```bash
goop check main.goop
goop build main.goop    # → ./goop-out for module main
goop test examples/
```

See [docs/design/20-cli-artifacts.md](docs/design/20-cli-artifacts.md).

## Verification

- `go test ./...` (from `src/`) — all packages green
- `goop test tests/` → 101+ passed
- `goop build docs/examples/hello.goop` with no `.go` left in `docs/examples/`
- Treelog: `goop test examples/` with the 1.8.0 binary
