#!/usr/bin/env bash
set -euo pipefail
export PATH=/home/hermes/.local/bin:$PATH
export GOCACHE=${GOCACHE:-/tmp/doordash-consumer-pp-cli-go-build-cache}
export GOTMPDIR=${GOTMPDIR:-/tmp}
mkdir -p "$GOCACHE"
cd /home/hermes/printing-press/library/doordash-consumer-pp-cli

go test -vet=off ./... >/tmp/doordash-consumer-go-test.log
go build -o /tmp/doordash-consumer-pp-cli-current ./cmd/doordash-consumer-pp-cli
go build -o /tmp/doordash-consumer-pp-mcp-current ./cmd/doordash-consumer-pp-mcp

/tmp/doordash-consumer-pp-cli-current doctor --json >/tmp/doordash-consumer-doctor.json
/tmp/doordash-consumer-pp-cli-current agent-context --pretty >/tmp/doordash-consumer-agent-context.txt
/tmp/doordash-consumer-pp-cli-current graphql --help >/tmp/doordash-consumer-graphql-help.txt

python3 - <<'PY2'
import pathlib, re, sys
text=pathlib.Path('/tmp/doordash-consumer-graphql-help.txt').read_text()
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

if /tmp/doordash-consumer-pp-cli-current cart place --confirm 'PLACE DOORDASH ORDER' --json >/tmp/doordash-consumer-order.log 2>&1; then
  echo "ERROR: order without all live gates unexpectedly succeeded" >&2
  exit 1
fi
if ! grep -q 'live DoorDash order placement is disabled' /tmp/doordash-consumer-order.log; then
  echo "ERROR: order gate message missing" >&2
  cat /tmp/doordash-consumer-order.log >&2
  exit 1
fi

echo "doordash-consumer-pp-cli smoke passed: build/test ok, 19 GraphQL commands exposed, order gate rejects live call"
