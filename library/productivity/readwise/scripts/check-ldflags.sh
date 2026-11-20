#!/bin/sh
# Keep subprocess build verification out of the Go unit suite.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
task_tmp=$(mktemp -d "${TMPDIR:-/tmp}/readwise-ldflags.XXXXXX")
[ -n "$task_tmp" ] || exit 1
trap 'rm -rf "$task_tmp"' EXIT HUP INT TERM
for module in readwise-pp-cli example.test/catalog/readwise; do
 cp "$root/Makefile" "$task_tmp/Makefile"
 mkdir -p "$task_tmp/internal/cli" "$task_tmp/cmd/readwise-pp-cli"
 printf 'module %s\n\ngo 1.23\n' "$module" > "$task_tmp/go.mod"
 printf '{"version":"2026.11.19"}\n' > "$task_tmp/manifest.json"
 printf 'package cli\nvar version="unstamped"\nfunc Version()string{return version}\n' > "$task_tmp/internal/cli/version.go"
 printf 'package main\nimport("fmt";"%s/internal/cli")\nfunc main(){fmt.Print(cli.Version())}\n' "$module" > "$task_tmp/cmd/readwise-pp-cli/main.go"
 (cd "$task_tmp" && GOWORK=off GOPROXY=off GOTOOLCHAIN=local make --no-print-directory build)
 actual=$("$task_tmp/bin/readwise-pp-cli")
 [ "$actual" = 2026.11.19 ] || { echo "version injection failed for $module: $actual" >&2; exit 1; }
done
"$root/scripts/check-module.sh"
