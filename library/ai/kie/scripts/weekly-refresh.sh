#!/usr/bin/env bash
# Weekly refresh of the Market model catalog (docs/MODELS.md) and the OpenAPI
# spec (research/kie-final-openapi.yaml) from docs.kie.ai.
#
# This does NOT regenerate the CLI's Go code -- new Market models fold into
# the existing unified create/query commands automatically, but a brand new
# dedicated endpoint family needs a human to add it to research/build_spec.py
# and re-run the full `cli-printing-press generate` pipeline (see README.md).
#
# Intended to run from cron (see README.md "Keeping this up to date").
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

python3 research/build_spec.py

if git diff --quiet -- docs/MODELS.md research/kie-final-openapi.yaml; then
  echo "$(date -Iseconds) weekly-refresh: no changes"
  exit 0
fi

git add docs/MODELS.md research/kie-final-openapi.yaml
git commit -m "chore: weekly model catalog refresh from docs.kie.ai"
git push

echo "$(date -Iseconds) weekly-refresh: pushed catalog update"
