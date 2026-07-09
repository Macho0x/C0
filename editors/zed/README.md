# Goop Support for Zed

Syntax highlighting, file icons, and LSP integration for the [Goop language](https://github.com/Macho0x/Goop).

## Features

- **File icon** — `.goop` files show the Goop logo in the file tree
- **Syntax highlighting** — full TextMate grammar for keywords, types, operators, literals, comments
- **LSP integration** — diagnostics, completions, and hover via the `c0` compiler
- **Block comments** — `(* ... *)` support
- **Language config** — proper tab/comment settings for `.goop` files

## Installation

### From source

```bash
cd editors/zed
make install
```

This builds the WASM extension and installs it to `~/.local/share/zed/extensions/installed/c0/`.

### From marketplace

Search for "Goop" in the Zed extensions panel.

After installing, restart Zed or run **zed: reload extensions** from the command palette.

## Development

- **Grammar**: edit `grammars/goop.tmLanguage.json`
- **Icons**: edit `icons/` SVG files and `icons/theme.json`
- **LSP adapter**: edit `src/lib.rs`
- **Language config**: edit `languages/c0/config.toml`

Run `make install` to rebuild and deploy changes.
