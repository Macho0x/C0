#!/usr/bin/env bash
# Install the Goop VS Code / Cursor extension from this repo.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXT_DIR="$ROOT/editors/vscode"

cd "$EXT_DIR"
npm install --silent

if command -v cursor >/dev/null 2>&1; then
  CLI=cursor
elif command -v code >/dev/null 2>&1; then
  CLI=code
else
  echo "error: neither 'cursor' nor 'code' CLI found on PATH" >&2
  exit 1
fi

npx --yes @vscode/vsce package --out "$EXT_DIR/goop.vsix" >/dev/null

# Remove stale versioned installs (cause duplicate grammar errors).
for old in "$HOME/.cursor/extensions"/goop.goop-* "$HOME/.vscode/extensions"/goop.goop-*; do
  [ -d "$old" ] && rm -rf "$old"
done

"$CLI" --install-extension "$EXT_DIR/goop.vsix" --force

echo "Installed Goop extension via $CLI."
echo "Reload the window (Ctrl+Shift+P → Developer: Reload Window)."
