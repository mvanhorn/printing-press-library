#!/bin/bash
# Regenerate pinterest-pp-cli from the overlay and re-apply hand customizations.
set -euo pipefail
R=~/printing-press/.runstate/headaideveloper-ca938232/runs/20261021-092559-b7e82418
W=$R/working/pinterest-pp-cli
# The run's own git history must exist; never fall through to an enclosing repo.
[ -d "$W/.git" ] || { echo "no .git in $W; restore it from the latest preserve snapshot first" >&2; exit 1; }
CUSTOM=$(git -C "$W" rev-parse --short HEAD)
cp -a "$W/.git" "$R/.git-backup-$$"
/Users/headaideveloper/go/bin/cli-printing-press generate --spec "$R/research/pinterest-overlay.yaml" --name pinterest --output "$W" --research-dir "$R" --category marketing --force --lenient --yes > "$R/generate-latest.log" 2>&1 || true
mkdir -p "$R/preserve-archive"; for d in "$W".preserve-*; do [ -e "$d" ] && mv "$d" "$R/preserve-archive/"; done
[ -d "$W/.git" ] || mv "$R/.git-backup-$$" "$W/.git"
rm -rf "$R/.git-backup-$$"
cd "$W"
git checkout HEAD -- go.mod go.sum .printing-press-patches 2>/dev/null || true
python3 "$R/research/reapply_customizations.py" "$W" 93901a3 "$CUSTOM" | grep -v '^kept' || true
go build -o pinterest-pp-cli ./cmd/pinterest-pp-cli
python3 "$R/research/fix_docs.py" "$W"
go vet ./...
