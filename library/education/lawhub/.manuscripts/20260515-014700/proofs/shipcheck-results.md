# Shipcheck

Current local gates:

```bash
make build VERSION=0.1.0-dev
make test
make vet
make smoke VERSION=0.1.0-dev
```

Last verified from final Printing Press package location (`library/education/lawhub`) on 2026-05-15:

```bash
GO=/home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go make build VERSION=0.1.0-dev
GO=/home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go make test
GO=/home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go make vet
GO=/home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go make smoke VERSION=0.1.0-dev
```

Results:

- build passed
- `go test ./...` passed
- `go vet ./...` passed
- smoke passed:
  - `--help`
  - `version --agent`
  - `doctor --agent`

Live validation on 2026-05-15:

```bash
/home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go run ./cmd/lawhub-pp-cli auth status --live --agent
LAWHUB_USER_ID=NOLANMCCAFFERTY /home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go run ./cmd/lawhub-pp-cli sync history --agent
LAWHUB_USER_ID=NOLANMCCAFFERTY /home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go run ./cmd/lawhub-pp-cli sync report-metadata --agent
LAWHUB_USER_ID=NOLANMCCAFFERTY /home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go run ./cmd/lawhub-pp-cli summary --agent
LAWHUB_USER_ID=NOLANMCCAFFERTY /home/nolan/.openclaw/workspace/printingpress/toolchains/go/bin/go run ./cmd/lawhub-pp-cli weakness report --agent
```

Results:

- live auth probe passed (`library-page`, HTTP 200)
- `sync history` synced 4 attempts
- `sync report-metadata` updated 416 question rows
- `summary` returned latest/best score 171 and counts: 4 attempts, 16 sections, 416 questions, 67 tests
- `weakness report` returned section and question-type rankings

Known live caveat: LawHub sessions expire. Re-import auth from a debuggable browser with `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222`, then run `lawhub-pp-cli auth status --live --agent` before live sync validation. If user-id auto-discovery is unavailable, set `LAWHUB_USER_ID` or pass `--user-id`.
