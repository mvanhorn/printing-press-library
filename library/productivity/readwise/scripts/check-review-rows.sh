#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
task_tmp=$(mktemp -d "${TMPDIR:-/tmp}/readwise-rows.XXXXXX")
[ -n "$task_tmp" ] || exit 1
trap 'rm -rf "$task_tmp"' EXIT HUP INT TERM
(cd "$root" && go test ./internal/cli ./internal/store -json -count=1 -timeout=60s -run '^(TestReview|TestR2|TestR3|TestR4)') > "$task_tmp/results.jsonl"
python3 - "$task_tmp/results.jsonl" <<'PY'
import json,re,sys
rows=[v['Test'] for line in open(sys.argv[1]) if (v:=json.loads(line)).get('Action')=='run' and '/' in v.get('Test','')]
assert rows, 'no hazard rows ran'
for prefix in ('TestR2LatestOnlyReadsHeadAndPreservesResumeState/', 'TestR2LatestOnlyCommandPreservesAllState/'):
    assert any(row.startswith(prefix) for row in rows), 'latest-only rows did not run: '+prefix
for row in rows:
    parts=row.split('/')
    assert len(parts)==2 and re.fullmatch(r'[A-Za-z0-9_]+',parts[1]), 'unselectable hazard row: '+row
print('review hazard rows are individually selectable:',len(rows))
PY
