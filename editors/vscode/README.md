# Install the Goop VS Code extension

Syntax highlighting, `.goop` file icons, and LSP diagnostics (via `goop lsp`).

## Prerequisites

1. Build the compiler and ensure `goop` is on your `PATH`:

```bash
cd src && go build -o ../goop ./cmd/goop
export PATH="$PWD/..:$PATH"
```

2. Install Node dependencies for the extension (once):

```bash
cd editors/vscode
npm install
```

## Install locally

1. Open VS Code.
2. Run **Developer: Install Extension from Location…**
3. Select this directory: `editors/vscode`
4. Reload the window.

Open any `.goop` file — you should see:

- **Goop icon** in the file explorer (from `assets/goop-icon-square.png`)
- **Syntax highlighting** (keywords, `match`, `@golang` blocks, etc.)
- **LSP diagnostics** if `goop` is on `PATH` (errors from `goop check` pipeline)

## Troubleshooting

| Problem | Fix |
|---|---|
| No highlighting | Confirm the extension is enabled; check language mode is **Goop** (bottom-right) |
| No icons | Reload window after install; icons require VS Code 1.64+ |
| LSP not working | Run `which goop`; rebuild compiler; check **Output → Goop Language Server** |
| Wrong colors after grammar change | Reload window; grammar is `syntaxes/goop.tmLanguage.json` (synced from repo root `syntaxes/`) |

## Grammar source

Canonical grammar: [`../../syntaxes/goop.tmLanguage.json`](../../syntaxes/goop.tmLanguage.json).  
After editing it, copy to `editors/vscode/syntaxes/` and `editors/zed/grammars/`.
