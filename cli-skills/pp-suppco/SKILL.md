---
name: pp-suppco
description: "Read SuppCo stack products, nutrient relationships, dated provider schedules, and normalized regimen snapshots. Read-only; no health interpretation."
author: "Felix Banuchi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - suppco-pp-cli
    install:
      - kind: go
        bins: [suppco-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/suppco/cmd/suppco-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/health/suppco/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# SuppCo PrintingPress CLI

Use this skill only for read-only SuppCo provider extraction. It returns provider facts and provenance; it does not calculate nutrient exposure, infer actual adherence, interpret labs or diet, or provide coaching.

## Prerequisites: Install the CLI

This skill drives the `suppco-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install suppco --cli-only
   ```
2. Verify: `suppco-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/suppco/cmd/suppco-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read minimum SuppCo stack and schedule facts, preserve overlapping nutrient relationships, and emit a deterministic snapshot for a later importer. Periodic bearer-token replacement is the intentionally simple authentication model.

## Authentication

Provide a current bearer token through `SUPPCO_ACCESS_TOKEN`. Tokens may expire; periodic replacement is expected. Do not attempt browser automation or token refresh.

To save a token without placing it in shell history:

```bash
op read 'op://<vault>/<item>/credential' | suppco-pp-cli auth set-token
```

Never pass the token as a positional argument, print it, or place it in an agent transcript.

## Command reference

```bash
suppco-pp-cli stack products
suppco-pp-cli stack nutrients
suppco-pp-cli schedule 2026-01-15
suppco-pp-cli regimen snapshot 2026-01-15
```

- `stack products` returns minimum product identity and label fields.
- `stack nutrients` returns product-bound ingredient rows with ancestry-derived parent/component relationships. Never sum parent and child rows as independent exposure.
- `schedule <date>` returns minimized provider activities, scheduled products, and reminder state for one ISO date.
- `regimen snapshot <date>` combines one stack read and one schedule read, preserves separate observation times, and emits the normalized import candidate.

All four commands emit JSON. The snapshot's `user_override` is absent and `effective_source` remains `provider_schedule`; downstream manual truth belongs to Trainer Core.

## Safety boundary

Do not use this CLI for writes, intake logging, schedule edits, product edits, health interpretation, or publication. The runtime refuses non-canonical origins, cross-origin redirects, disabled TLS verification, local data-source mode, profiles, and built-in output delivery. Redirect stdout only when the user has chosen to retain private output.

Do not activate this skill for requests involving raw account responses, cookies, bearer values, contact/profile fields, intake history, or account settings.

## MCP

The stdio MCP binary is `suppco-pp-mcp`. It exposes exactly:

- `stack_products`
- `stack_nutrients`
- `schedule_show`
- `regimen_snapshot`
- `context`

Pass `SUPPCO_ACCESS_TOKEN` in the MCP host environment. CLI and MCP share the same provider service and output contract.

## Direct use

1. Empty input or `help` -> run `suppco-pp-cli --help`.
2. A supported command -> run it exactly as provided.
3. If authentication fails -> explain that the token may have expired and request replacement; never request that the token be pasted into chat.
4. Return or summarize only the minimum fields needed for the user's task, preserving provenance and overlap warnings.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Private provider normalization
- **`regimen snapshot`** — Combines the current stack and one dated provider schedule into deterministic JSON while preserving nutrient relationships and separate observation times.

  _Use this when an agent needs a bounded import candidate rather than raw SuppCo account payloads._

  ```bash
  suppco-pp-cli regimen snapshot 2026-01-15
  ```

## Recipes

### List products

```bash
suppco-pp-cli stack products
```

Returns the minimum product projection and observation provenance.

### Inspect nutrient hierarchy

```bash
suppco-pp-cli stack nutrients
```

Preserves parent and child rows so downstream code can avoid naive totals.

### Build a snapshot

```bash
suppco-pp-cli regimen snapshot 2026-01-15
```

Combines two bounded reads without introducing manual regimen truth.

## Auth Setup

Provide a current bearer token through SUPPCO_ACCESS_TOKEN or pipe it to auth set-token. If it expires, replace it; automatic refresh and browser automation are not part of this package.

Run `suppco-pp-cli doctor` to verify setup.
