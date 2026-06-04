#!/usr/bin/env bash
set -euo pipefail
export PATH=/home/hermes/.local/bin:$PATH
export GOCACHE=${GOCACHE:-/tmp/doordash-pp-cli-go-build-cache}
export GOTMPDIR=${GOTMPDIR:-/tmp}
mkdir -p "$GOCACHE"
cd /home/hermes/printing-press/library/doordash-pp-cli

go test -vet=off ./... >/tmp/doordash-go-test.log
go build -o /tmp/doordash-pp-cli-current ./cmd/doordash-pp-cli
go build -o /tmp/doordash-pp-mcp-current ./cmd/doordash-pp-mcp

/tmp/doordash-pp-cli-current doctor --json >/tmp/doordash-doctor.json
/tmp/doordash-pp-cli-current agent-context --pretty >/tmp/doordash-agent-context.txt
/tmp/doordash-pp-cli-current graphql --help >/tmp/doordash-graphql-help.txt

python3 - <<'PY2'
import pathlib, re, sys
text=pathlib.Path('/tmp/doordash-graphql-help.txt').read_text()
cmds=[]
in_avail=False
for line in text.splitlines():
    if line.strip() == 'Available Commands:':
        in_avail=True
        continue
    if in_avail:
        if not line.strip():
            break
        m=re.match(r'\s{2}([a-zA-Z0-9_-]+)\s+', line)
        if m:
            cmds.append(m.group(1))
if len(cmds) != 19:
    print(f'ERROR: expected 19 graphql commands, got {len(cmds)}: {cmds}', file=sys.stderr)
    sys.exit(1)
PY2

if /tmp/doordash-pp-cli-current cart place --confirm 'PLACE DOORDASH ORDER' --json >/tmp/doordash-order.log 2>&1; then
  echo "ERROR: order without all live gates unexpectedly succeeded" >&2
  exit 1
fi
if ! grep -q 'live DoorDash order placement is disabled' /tmp/doordash-order.log; then
  echo "ERROR: order gate message missing" >&2
  cat /tmp/doordash-order.log >&2
  exit 1
fi

echo "doordash-pp-cli smoke passed: build/test ok, 19 GraphQL commands exposed, order gate rejects live call"
