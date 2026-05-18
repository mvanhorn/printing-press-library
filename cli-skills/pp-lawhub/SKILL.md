---
name: pp-lawhub
description: "Use LawHub PP CLI for LSAT practice-test analytics: sync authenticated LawHub score metadata, inspect attempts, rank weaknesses by question type/section, add user-authored review notes, and open official LawHub review links without storing LSAT question content. Trigger phrases: lawHub LSAT analytics, sync LSAT practice tests, LSAT weaknesses, wrong answer review, LawHub review link, lawhub-pp-cli."
author: "Nolan McCafferty"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - lawhub-pp-cli
    install:
      - kind: go
        bins: [lawhub-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/education/lawhub/cmd/lawhub-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/education/lawhub/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# LawHub — Printing Press CLI

## Prerequisites

Verify the CLI is installed before use:

```bash
lawhub-pp-cli version --agent
lawhub-pp-cli doctor --agent
```

If missing after publication:

```bash
go install github.com/mvanhorn/printing-press-library/library/education/lawhub/cmd/lawhub-pp-cli@latest
```

## Use When

Use for LSAT practice analytics from LawHub: sync history, sync score-report metadata, summarize attempts, rank weaknesses, list wrong questions, add user-authored notes, and open LawHub review links.

## Do Not Use For

Do not extract or store LSAT question stems, passages, answer-choice text, official explanations, or bulk official LSAT content. Do not ask the user to paste LSAC credentials into chat.

## Auth Workflow

LawHub/MSAL requires browser storage. Import auth from a browser launched with Chrome DevTools Protocol enabled.

1. User starts a browser and logs into LawHub:

```bash
# Chrome preferred, if installed:
google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/lawhub-debug-profile https://app.lawhub.org/library/fulltests

# Brave fallback:
brave-browser --remote-debugging-port=9222 --user-data-dir=/tmp/lawhub-debug-profile https://app.lawhub.org/library/fulltests
```

2. Import session:

```bash
lawhub-pp-cli auth login --cdp http://127.0.0.1:9222
```

3. Verify:

```bash
lawhub-pp-cli auth status --live --agent
```

Expected: `live.ok` is `true`.

Fallback:

```bash
lawhub-pp-cli auth import-file <storage-state.json>
```

If user-id discovery fails:

```bash
export LAWHUB_USER_ID=<LAW_HUB_USER_ID>
```

## Best Command Mapping

- Check status → `lawhub-pp-cli doctor --agent`
- Verify auth → `lawhub-pp-cli auth status --live --agent`
- Import auth from browser → `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222`
- Sync tests/history → `lawhub-pp-cli sync browser --agent && lawhub-pp-cli sync history --agent`
- Sync score metadata → `lawhub-pp-cli sync report-metadata --agent`
- Summarize progress → `lawhub-pp-cli summary --agent`
- Find weaknesses → `lawhub-pp-cli weakness report --agent`
- List wrong questions → `lawhub-pp-cli questions list --incorrect --agent`
- Add review note → `lawhub-pp-cli questions note <question-id> --note "..." --agent`
- Open official review → `lawhub-pp-cli review open <attempt-id> --section 1 --question 17`

## Recommended Workflow

```bash
lawhub-pp-cli doctor --agent
lawhub-pp-cli auth status --live --agent
lawhub-pp-cli sync browser --agent
lawhub-pp-cli sync history --agent
lawhub-pp-cli sync report-metadata --agent
lawhub-pp-cli summary --agent
lawhub-pp-cli weakness report --agent
```

## Commands

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

Analytics:

```bash
lawhub-pp-cli summary --agent
lawhub-pp-cli weakness report --agent
lawhub-pp-cli questions list --incorrect --agent
lawhub-pp-cli questions show <question-id> --agent
```

Notes/review:

```bash
lawhub-pp-cli questions note <question-id> --note "..." --agent
lawhub-pp-cli review open <attempt-id> --section 1 --question 17 --print-url --agent
```

Low-token output:

```bash
lawhub-pp-cli summary --select counts,recent_average,weakest_question_types --agent
lawhub-pp-cli questions list --incorrect --select id,question_type,chosen_answer,correct_answer,is_correct --agent
```

## Intentionally Absent

Do not rely on removed prototype-only commands. The public sync surface is limited to `sync browser`, `sync history`, and `sync report-metadata`; there is no public bulk sync, HAR capture, Obsidian export, or attempts-import command.

## Gates

```bash
make build VERSION=0.1.0-dev
make test
make vet
make smoke VERSION=0.1.0-dev
```
