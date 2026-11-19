#!/bin/sh
# Check module discovery independently of a caller's Go workspace.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
task_tmp=$(mktemp -d "${TMPDIR:-/tmp}/readwise-module.XXXXXX")
[ -n "$task_tmp" ] || exit 1
trap 'rm -rf "$task_tmp"' EXIT HUP INT TERM
cp "$root/Makefile" "$task_tmp/Makefile"
printf '{"version":"2026.11.19"}\n' > "$task_tmp/manifest.json"
printf 'module example.test/readwise\n\ngo 1.23\n' > "$task_tmp/go.mod"
printf '\nprint-module:\n\t@echo $(MODULE)\n' >> "$task_tmp/Makefile"
actual=$(cd "$task_tmp" && GOWORK="$task_tmp/missing.work" make --no-print-directory print-module)
[ "$actual" = example.test/readwise ] || { echo "module discovery failed: $actual" >&2; exit 1; }
