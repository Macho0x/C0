# Goop 1.6.0

Goop 1.6.0 removes the last codegen gaps that forced multi-package libraries
(like Treelog) to ship lowercase “bridge” wrappers. Cross-package record
literals, Capitalized calls, multiline apps, Buffer pointer FFI, richer
`gosig`, and LSP path decoding now work as OCaml-style Goop expects.

## Highlights

- **Cross-package records:** `{ service = … }` after `import goop . "pkg"`
  lowers to `pkg.Options{…}` (including option fields as
  `pkg.NewOptionTSome` / `None`).
- **Cross-package Capitalized calls:** `NewScope log attrs` →
  `treelog.NewScope(log, attrs)` — full apps, partial apps, and unit elision.
- **Multiline apps:** juxtaposition may continue on the next line when the
  argument is parenthesized (or indented past the callee).
- **Buffer ptr:** heap/mutable Go types and pointer-receiver methods use
  `T ptr` (e.g. `Buffer ptr` for `*bytes.Buffer`).
- **gosig:** named Go types, `LookupVar`, and `packages.Config.Dir` from the
  project root (graceful fallback when load fails).
- **LSP:** `file://` URIs are URL-decoded; workspace root backs up
  `module_root` when the file path alone cannot find `goop.toml`.

## Libraries

Drop lowercase bridges that only existed for 1.5 codegen. Prefer Capitalized
public APIs and record literals at the call site.

## Verification

- `go test ./...` (from `src/`)
- `goop test tests/` → 94 passed, 0 failed
- Treelog: `goop test examples/` → 6 passed, 0 failed; `go build ./...`
