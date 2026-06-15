#!/usr/bin/env bash
# Daily Printing Press sync (hermes cron "Daily Printing Press library + CLI sync").
#
# PRIVATE (cathrynlavery) is the source of truth; PUBLIC (mvanhorn) is upstream that we
# only fetch to DETECT new/changed CLIs. All building + skill install happens from the
# private checkout. This wrapper delegates to pp-sync and prints pp-sync's report so the
# no_agent cron delivers it to Telegram daily.
#
# Previous behaviour (installed binaries from the public @mvanhorn npm package and
# `go install …mvanhorn…@latest`) is retired — see backup alongside this file.
set -euo pipefail

export PATH="$HOME/.local/bin:$HOME/go/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"
REPO="${PP_LIBRARY_REPO:-$HOME/Developer/PrintingPress/printing-press-library}"

# Self-update the deployed pp-sync from the version-controlled private repo copy
# (one cycle behind, converges automatically once the tooling PR is merged).
SRC="$REPO/tools/pp-sync/pp-sync"
if [[ -f "$SRC" && "$SRC" -nt "$HOME/.local/bin/pp-sync" ]]; then
  install -m 0755 "$SRC" "$HOME/.local/bin/pp-sync" || true
fi

command -v pp-sync >/dev/null 2>&1 || { echo "pp-sync not found on PATH"; exit 127; }

# pp-sync update: ff-pull private main, build the installed CLI set from private,
# sync skills, smoke test, diff against public upstream, write + print the report.
exec pp-sync update 2>&1
