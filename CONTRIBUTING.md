# Contributing to Goop

Thank you for your interest in Goop. The project is in active bootstrap; this guide will grow as the toolchain matures.

## Before you start

- Read the [design overview](docs/design/01-overview.md) and [roadmap](docs/design/07-roadmap.md).
- Check [TODO.md](TODO.md) for open work.
- Error codes and safety checks: [10-error-reference.md](docs/design/10-error-reference.md), [12-trading-bot-safety.md](docs/design/12-trading-bot-safety.md).

## Build and test

```bash
cd src
go build -o ../goop ./cmd/goop

# Unit tests (Go compiler implementation)
go test ./...

# End-to-end Goop tests
../goop test ../tests/

# Examples (also run in CI)
for f in ../docs/examples/*.goop; do ../goop check "$f"; done
```

## Making changes

1. **Compiler / language** — changes usually touch `src/internal/` (parser, typecheck, codegen, safety passes) and `tests/*.goop` e2e files.
2. **Examples** — add or update files under `docs/examples/`; CI runs `goop check` on all of them.
3. **Design docs** — keep `docs/design/` and `TODO.md` / `docs/design/07-roadmap.md` in sync when behavior changes.
4. **Formatting** — `goop fmt` for Goop sources; `gofmt` for Go in `src/`.

## Pull requests

- One logical change per PR when possible.
- Include or update e2e tests for user-visible behavior.
- Note breaking changes and migration steps in the PR description.

## Not yet available

- `goop doc` documentation generator
- Dedicated language tutorial series
- Generated standard library API reference

These are tracked in [TODO.md](TODO.md) and the roadmap.
