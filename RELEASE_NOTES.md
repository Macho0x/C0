# Goop 1.2.3

Fix VS Code / Cursor embed-tag highlighting so amber `@[go]` / `@[c]` works on any color theme (including Dracula).

## Highlights

- **Extension 0.3.7:** `keyword.embed.goop` → `#D7BA7D` and other Goop TextMate rules live in top-level `configurationDefaults.editor.tokenColorCustomizations` (not under `[goop]`, which VS Code ignores for token colors)
- Reinstall with `./scripts/install-editor-extension.sh`, then **Developer: Reload Window**

## Verification

- Inspect `@[c]` in `docs/examples/cgo_demo.goop`: scope `keyword.embed.goop`, foreground amber
