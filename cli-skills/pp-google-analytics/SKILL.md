---
name: pp-google-analytics
description: Google Analytics 4 Printing Press CLI for GA4 raw reports, funnels, acquisition/revenue insights, period comparisons, anomaly scans, and property-access health checks.
tags: [printing-press, ga4, google-analytics, analytics, marketing]
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/google-analytics/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# pp-google-analytics

## Prerequisites: Install the CLI

This skill drives the `google-analytics-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install google-analytics --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `google-analytics-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/cmd/google-analytics-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs live GA4 property data, acquisition/revenue summaries, ecommerce diagnostics, funnels, period comparisons, anomaly scans, or property-access diagnostics. Use `google-search-console-pp-cli` for Search Console.


Use `google-analytics-pp-cli` for GA4-only analytics work. Search Console is covered by `google-search-console-pp-cli`; do not use the legacy combined GSC/GA4 CLI for new work.

## Auth and property selection

- Service account key: `--credentials`, else `GOOGLE_APPLICATION_CREDENTIALS`.
- Scope: `https://www.googleapis.com/auth/analytics.readonly`.
- Property: pass `--property`, or set `GA4_PROPERTY_ID`.
- Fleet health checks: `health --properties $GA4_PROPERTY_IDS --agent` or set `GA4_PROPERTY_IDS`.

Durable gotcha: Google Cloud API access is not GA4 property access. The service account must be granted Viewer access inside the GA4 property. If `health` shows token/admin OK but a property check is 403/404, fix the GA4 property grant rather than rotating credentials.

## Best commands for agents

```bash
google-analytics-pp-cli agent-context --agent
google-analytics-pp-cli health --properties $GA4_PROPERTY_IDS --agent
google-analytics-pp-cli channels --property "$GA4_PROPERTY_ID" --start 28daysAgo --end yesterday --agent
google-analytics-pp-cli sources --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli top-pages --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli compare --property "$GA4_PROPERTY_ID" --metric sessions,totalRevenue --period wow --agent
google-analytics-pp-cli whats-changed --property "$GA4_PROPERTY_ID" --agent
google-analytics-pp-cli revenue --property "$GA4_PROPERTY_ID" --by channel --agent
google-analytics-pp-cli funnel --property "$GA4_PROPERTY_ID" --steps view_item,add_to_cart,begin_checkout,purchase --agent
```

## Raw wrappers

`report`, `pivot`, `batch`, `realtime`, `metadata`, `compatibility`, `properties`, `property`, and `streams` are available for low-level escape hatches. Prefer novel commands when answering business questions.
