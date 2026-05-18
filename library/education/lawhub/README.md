# LawHub PP CLI

**Local-first LSAT practice analytics from LawHub — without storing official LSAT content.**

`lawhub-pp-cli` syncs authenticated LawHub practice-test metadata into local SQLite, then provides score history, weakness reports, question lists, user-authored review notes, and safe links back into LawHub for official content review.

This is an unofficial tool. It is not affiliated with or endorsed by LSAC or LawHub.

## Data Boundary

Allowed/synced:

- attempt IDs and test/module IDs
- test names, dates, modes, scores
- raw/scored totals
- section IDs/types/counts
- chosen answer letters
- correctness
- question type/subtype
- difficulty
- per-question timing
- flag state
- links back to LawHub
- user-authored notes

Not stored:

- LSAT question stems
- passages
- answer-choice text
- official explanations
- bulk official LSAT content

Use `review open` to view official content in LawHub.

## Install

Once published in the Printing Press library:

```bash
go install github.com/mvanhorn/printing-press-library/library/education/lawhub/cmd/lawhub-pp-cli@latest
```

Verify:

```bash
lawhub-pp-cli version --agent
lawhub-pp-cli doctor --agent
```

Local development:

```bash
make build VERSION=0.1.0-dev
./bin/lawhub-pp-cli-go version --agent
```

## Authentication

Do **not** paste LSAC/LawHub credentials into chat.

LawHub/MSAL needs browser storage, not just cookies. The supported login flow imports storage from an already-running debuggable browser.

1. Start a browser with Chrome DevTools Protocol enabled. Chrome is preferred; Brave/Chromium work too.

```bash
# Chrome, if installed:
google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/lawhub-debug-profile https://app.lawhub.org/library/fulltests

# Or Brave:
brave-browser --remote-debugging-port=9222 --user-data-dir=/tmp/lawhub-debug-profile https://app.lawhub.org/library/fulltests
```

2. Log into LawHub in that browser and confirm the library page loads.

3. In another terminal, import the active browser session:

```bash
lawhub-pp-cli auth login --cdp http://127.0.0.1:9222
```

4. Verify:

```bash
lawhub-pp-cli auth status --live --agent
```

Expected live auth shape:

```json
{"live":{"checked":true,"ok":true,"probe":"library-page","status":200}}
```

Fallback import, if another tool produced Playwright/browser-use storage state:

```bash
lawhub-pp-cli auth import-file /path/to/storage-state.json
```

Other auth helpers:

```bash
lawhub-pp-cli auth status --agent
lawhub-pp-cli auth path --agent
lawhub-pp-cli auth logout --agent
```

If LawHub user-id discovery fails, set:

```bash
export LAWHUB_USER_ID=<LAW_HUB_USER_ID>
```

or pass:

```bash
--user-id <LAW_HUB_USER_ID>
```

## Quick Start

```bash
lawhub-pp-cli doctor --agent
lawhub-pp-cli auth status --live --agent
lawhub-pp-cli sync browser --agent
lawhub-pp-cli sync history --agent
lawhub-pp-cli sync report-metadata --agent
lawhub-pp-cli summary --agent
lawhub-pp-cli weakness report --agent
```

Use explicit sync steps. There is intentionally no public `sync all`.

## Commands

Health/version:

```bash
lawhub-pp-cli version --agent
lawhub-pp-cli doctor --agent
lawhub-pp-cli doctor --live --agent
```

Auth:

```bash
lawhub-pp-cli auth login --cdp http://127.0.0.1:9222
lawhub-pp-cli auth import-file <storage-state.json>
lawhub-pp-cli auth status --live --agent
lawhub-pp-cli auth path --agent
lawhub-pp-cli auth logout --agent
```

Sync:

```bash
lawhub-pp-cli sync browser --agent
lawhub-pp-cli sync history --agent
lawhub-pp-cli sync history --module LSAC140 --agent
lawhub-pp-cli sync report-metadata --agent
lawhub-pp-cli sync report-metadata --attempt <testInstanceId> --agent
```

Attempts/tests:

```bash
lawhub-pp-cli tests list --agent
lawhub-pp-cli attempts list --agent
lawhub-pp-cli attempts show <attempt-id> --agent
```

Questions and notes:

```bash
lawhub-pp-cli questions list --incorrect --agent
lawhub-pp-cli questions list --type "Matching Flaws" --agent
lawhub-pp-cli questions list --difficulty 4 --agent
lawhub-pp-cli questions list --min-time 120 --agent
lawhub-pp-cli questions show <question-id> --agent
lawhub-pp-cli questions note <question-id> --note "Misread conditional conclusion" --agent
lawhub-pp-cli questions note <question-id> --why-picked "..." --why-correct "..." --next-time "..." --agent
```

Analytics:

```bash
lawhub-pp-cli summary --agent
lawhub-pp-cli weakness report --agent
```

Review in LawHub:

```bash
lawhub-pp-cli review open <attempt-id> --section 1 --question 17
lawhub-pp-cli review open <attempt-id> --section 1 --question 17 --print-url --agent
```

Low-token output:

```bash
lawhub-pp-cli summary --select counts,recent_average,weakest_question_types --agent
lawhub-pp-cli questions list --incorrect --select id,question_type,chosen_answer,correct_answer,is_correct --agent
```

## Current Public Command Surface

```txt
auth
attempts
doctor
login       # alias for auth login
questions
review
summary
sync
tests
version
weakness
```

`sync` exposes:

```txt
browser
history
report-metadata
```

## Intentionally Absent

These prototype commands are intentionally not public unless reimplemented Go-native:

```bash
lawhub-pp-cli sync all
lawhub-pp-cli sync questions
lawhub-pp-cli capture-har
lawhub-pp-cli export obsidian
lawhub-pp-cli attempts import
```

## Local Files

```txt
DB:          ~/.local/share/lawhub-pp-cli/lawhub.sqlite
Config dir:  ~/.config/lawhub-pp-cli
Session dir: ~/.openclaw/secure/lawhub
Session:     ~/.openclaw/secure/lawhub/storage-state.json
Account:     ~/.openclaw/secure/lawhub/account.json
```

Override with `--data-dir`, `--config-dir`, and `--secure-dir`.

## Troubleshooting

### CDP connection refused

Start the browser with remote debugging first:

```bash
brave-browser --remote-debugging-port=9222 --user-data-dir=/tmp/lawhub-debug-profile https://app.lawhub.org/library/fulltests
curl http://127.0.0.1:9222/json/version
```

The `curl` command should return JSON containing `webSocketDebuggerUrl`.

### `auth status --live` returns `ok:false`

Re-import from the debuggable browser after confirming LawHub is logged in:

```bash
lawhub-pp-cli auth login --cdp http://127.0.0.1:9222
lawhub-pp-cli auth status --live --agent
```

### User id not found

Set:

```bash
export LAWHUB_USER_ID=<LAW_HUB_USER_ID>
```

### Need official LSAT content

Do not extract it. Open LawHub:

```bash
lawhub-pp-cli review open <attempt-id> --section <n> --question <q>
```

## Development

```bash
make build VERSION=0.1.0-dev
make test
make vet
make smoke VERSION=0.1.0-dev
```

## Packaging

Designed as a Printing Press package under:

```txt
library/education/lawhub
```

with `.printing-press.json`, `SKILL.md`, `.manuscripts/`, and Go source under `cmd/` and `internal/`.
