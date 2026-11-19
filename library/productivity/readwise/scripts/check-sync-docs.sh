#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
python3 - "$root" <<'PY'
from pathlib import Path
import sys
for name in ('README.md','SKILL.md'):
    text=(Path(sys.argv[1])/name).read_text()
    for contract in ('never advances the shared checkpoint', 'Capped syncs resume where they stopped', 'restarts at page 1 when the page size changes or the saved page disappears', '`--full` discards any capped continuation'):
        assert contract in text, f'{name}: missing sync contract: {contract}'
PY
