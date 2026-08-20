#!/bin/bash
# Restore hand-patches after a full /printing-press openrouter regen.
# Run from anywhere; resolves library path relative to this script.
set -e
DIR="$(cd "$(dirname "$0")" && pwd)"
LIB="$(dirname "$DIR")"
echo "Restoring hand-patches into $LIB"
for src in $(find "$DIR" -name '*.go' -not -path "$DIR"); do
  rel="${src#$DIR/}"
  dst="$LIB/$rel"
  cp "$src" "$dst"
  echo "  ✓ $rel"
done
echo ""
echo "Now rebuild + verify:"
echo "  cd $LIB && go build -o ./openrouter-pp-cli ./cmd/openrouter-pp-cli && go build -o ./openrouter-pp-mcp ./cmd/openrouter-pp-mcp"
echo "  ./openrouter-pp-cli doctor"
echo "  launchctl kickstart -k gui/\$(id -u)/com.local.openrouter-mcp"
