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

1. Build the compiler (extension auto-detects `${workspaceFolder}/goop`):

```bash
cd src && go build -o ../goop ./cmd/goop
```

2. Install extension dependencies (once):

```bash
cd editors/vscode && npm install
```

3. Open this repository in VS Code — you should be prompted to install the recommended **Goop Language Support** extension, or run **Developer: Install Extension from Location…** and select `editors/vscode`.

4. Reload the window.

## Settings

| Setting | Default | Description |
|---|---|---|
| `goop.path` | *(auto)* | Path to `goop` binary. Empty = try `goop` in workspace root, then `PATH`. |

The workspace ships `.vscode/settings.json` pointing at `${workspaceFolder}/goop`.

## What you should see

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
