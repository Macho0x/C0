#!/bin/sh
# Sync canonical TextMate grammar to editor extensions.
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/syntaxes/goop.tmLanguage.json"
cp "$SRC" "$ROOT/editors/vscode/syntaxes/goop.tmLanguage.json"
cp "$SRC" "$ROOT/editors/zed/grammars/goop.tmLanguage.json"
echo "Synced grammar to VS Code and Zed extensions."
