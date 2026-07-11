# Goop 1.2.2

Bring the VS Code / Cursor extension and LSP in line with recent language + formatter work.

## Highlights

- **Grammar:** keywords `continue`, `discontinue`, `downto`, `functor`, `not`; type `array`; deprecated markers for removed surface
- **Format Document:** LSP advertises `documentFormattingProvider` (same engine as `goop fmt`)
- **LSP framing fix:** reliable Content-Length reads + stdout flush (fixes hung/partial client sessions)
- **Extension 0.3.6:** default formatter `goop.goop`; reinstall via `./scripts/install-editor-extension.sh`

## Verification

- `go test ./...` (from `src/`) including `TestLSPDocumentFormatting`
- `goop test tests/`
- `goop check` on all `docs/examples/*.goop`
