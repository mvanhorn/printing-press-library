#!/usr/bin/env bash
set -euo pipefail

branch="${1:-main}"
max_attempts="${PUSH_RETRY_MAX_ATTEMPTS:-5}"
delay_seconds="${PUSH_RETRY_DELAY_SECONDS:-2}"

retry_or_fail() {
  local reason="$1"

  if ((attempt == max_attempts)); then
    echo "::error::generated-artifact push failed after $max_attempts attempts"
    exit 1
  fi

  echo "$reason; retrying ($attempt/$max_attempts)."
  sleep "$delay_seconds"
}

for ((attempt = 1; attempt <= max_attempts; attempt++)); do
  if ! git fetch origin "$branch"; then
    retry_or_fail "Fetch failed"
    continue
  fi

  git rebase "origin/$branch"

  if git push origin "HEAD:$branch"; then
    exit 0
  fi

  retry_or_fail "Push raced with another main-branch writer"
done
