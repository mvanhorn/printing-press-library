#!/usr/bin/env bash
# Print the Go module directory for a catalog root.
#
# Same rule as resolveModuleDir in main.go. A root that already has
# go.mod is printed unchanged. Otherwise use .goreleaser.yaml builds[].dir
# when every build names the same relative directory and that directory
# exists, and otherwise the single child directory that contains go.mod.
# Unresolved roots are printed unchanged. Disagreeing or unsafe build dirs
# exit non-zero.

set -euo pipefail

catalog="${1:?catalog root required}"

if [ -f "${catalog}/go.mod" ]; then
  printf '%s\n' "$catalog"
  exit 0
fi

goreleaser="${catalog}/.goreleaser.yaml"
if [ -f "$goreleaser" ]; then
  dirs=()
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    dirs+=("$line")
  done < <(awk '
    /^[[:space:]]*($|#)/ { next }
    /^builds:[[:space:]]*$/ { inb = 1; next }
    inb && /^[^[:space:]]/ { inb = 0 }
    inb && /^[[:space:]]+(-[[:space:]]+)?dir:[[:space:]]*/ {
      val = $0
      sub(/[[:space:]]+#.*$/, "", val)
      sub(/^[[:space:]]*(-[[:space:]]+)?dir:[[:space:]]*/, "", val)
      gsub(/^["'\'']|["'\'']$/, "", val)
      sub(/^\.\//, "", val)
      sub(/\/$/, "", val)
      if (val != "" && val != ".") print val
    }
  ' "$goreleaser")

  if [ "${#dirs[@]}" -gt 0 ]; then
    rel="${dirs[0]}"
    for dir in "${dirs[@]}"; do
      if [ "$dir" != "$rel" ]; then
        echo "${catalog}: .goreleaser.yaml builds disagree on dir: ${dirs[*]}" >&2
        exit 1
      fi
      if [[ "$dir" == *..* || "$dir" == /* || "$dir" == *' '* || "$dir" == *\\* ]]; then
        echo "${catalog}: refusing goreleaser build dir ${dir}" >&2
        exit 1
      fi
    done
    if [ -d "${catalog}/${rel}" ]; then
      printf '%s\n' "${catalog}/${rel}"
      exit 0
    fi
  fi
fi

shopt -s nullglob
nested=()
for dir in "${catalog}"/*/; do
  name="$(basename "$dir")"
  if [ -f "${catalog}/${name}/go.mod" ]; then
    nested+=("$name")
  fi
done
if [ "${#nested[@]}" -eq 1 ]; then
  printf '%s\n' "${catalog}/${nested[0]}"
  exit 0
fi

printf '%s\n' "$catalog"
