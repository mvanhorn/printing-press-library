---
name: pp-yeswehack
description: "Every YesWeHack researcher feature, plus an offline SQLite-backed cockpit for scope cartography, drift detection,... Trigger phrases: `hunt on yeswehack`, `qualify a yeswehack program`, `triage my yeswehack programs`, `draft a yeswehack report`, `is this yeswehack bug a duplicate`, `what changed in yeswehack scope`, `yeswehack hacktivity for fintech`, `use yeswehack`, `run yeswehack-pp-cli`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - yeswehack-pp-cli
---

# YesWeHack — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `yeswehack-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install yeswehack --cli-only
   ```
2. Verify: `yeswehack-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/yeswehack/cmd/yeswehack-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

`yeswehack-pp-cli` is the researcher-side cockpit for the YesWeHack bug bounty platform. It syncs every program you can see, every scope, every hacktivity disclosure into a local SQLite store so an agent can answer 'what should I work on', 'has this been reported', and 'what is in scope here' in milliseconds, offline. Submit and draft commands are guard-railed by design - the goal is better reports, not more reports.

## When to Use This CLI

Reach for this CLI when a security researcher (or their agent) is qualifying YesWeHack programs, drafting a report, or trying to calibrate severity from prior disclosures. Particularly strong for agent-driven triage workflows where the agent needs structured local state to answer 'has this been reported', 'what is in scope', and 'which program pays the most for this asset'. Skip it for program-manager workflows (use ywh2bugtracker) and for one-off curl calls to the public API.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`programs scope-drift`** — See what changed in any program's scope this week — assets added, removed, or modified, with first-seen dates.

  _When an agent triages where to spend the hunter's week, drift is the highest-signal source of fresh attack surface. Pick this over a generic program list when the user has already chosen programs and wants to know what changed._

  ```bash
  yeswehack-pp-cli programs scope-drift --since-days 7 --json
  ```
- **`scopes overlap`** — Surface assets (host or wildcard) that appear in two or more of your invited programs, ranked by best payout.

  _When the agent finds a candidate finding on an asset, this answers 'which program pays the most for this asset' before drafting the report._

  ```bash
  yeswehack-pp-cli scopes overlap --min-programs 2 --json
  ```
- **`triage weekend`** — Ranked plan for a short hunting session - newly added scope, reports needing your response, and trending CWEs in your specialty.

  _Picks the right starting move when the hunter (or their agent) has limited time and needs a confidence-weighted plan, not a feed._

  ```bash
  yeswehack-pp-cli triage weekend --hours 6 --json
  ```
- **`programs fit`** — Rank invited and public programs by how well your historical CWE specialties match each program's hacktivity payout pattern.

  _Answers 'which program am I most likely to land on this week' before time is spent on scope reading or report drafting._

  ```bash
  yeswehack-pp-cli programs fit --specialty xss,ssrf,idor --json
  ```
- **`events calendar`** — Chronological view of platform events, payout deadlines, and CTFs gating private invites - filtered to programs you are invited to.

  _Surfaces time-bound opportunities (renewal bumps, CTF gates) the hunter would otherwise miss until after the fact._

  ```bash
  yeswehack-pp-cli events calendar --mine --json
  ```

### Anti-spam guard-rails
- **`report dedupe`** — FTS5 search over the public hacktivity feed plus your own reports for title, asset, or CWE overlap — exits 2 if a high-confidence collision exists.

  _Aligns with the YesWeHack Platform Code of Conduct's anti-spam rule. Before an agent drafts a report, this answers 'has someone already filed this' deterministically._

  ```bash
  yeswehack-pp-cli report dedupe --title 'SQLi in /api/users/{id}' --asset api.example.com --cwe CWE-89 --json
  ```
- **`report cvss-check`** — Parse a CVSS 3.1 vector, recompute its base score, and flag impossible combinations against report steps text - rule-based, no LLM.

  _Catches CVSS misrepresentations before the report is filed - the kind of mistake that loses credibility with triagers._

  ```bash
  yeswehack-pp-cli report cvss-check 'CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H' --steps draft.md --json
  ```
- **`report draft`** — Create a markdown draft pre-filled with the program's reward grid, accepted severity levels, and an allowed asset picker from local scopes - no network call.

  _Gives an agent a deterministic shape for a report instead of letting it fabricate the structure. Quality multiplier per the Platform CoC._

  ```bash
  yeswehack-pp-cli report draft yes-we-hack --output ./my-draft.md
  ```
- **`report submit`** — Submit a drafted report after dry-run preview, in-scope validation, and automatic pre-submit dedupe. Requires --confirm.

  _Lets an agent close the loop on submission without violating the platform's anti-AI-slop policy. No batch flag, no template-flood._

  ```bash
  yeswehack-pp-cli report submit ./my-draft.md --confirm
  ```

### Agent-native plumbing
- **`hacktivity trends`** — Histogram of disclosed report categories and average bounty for one program over a time window.

  _Calibrates severity expectations and report-style for a target program before the agent starts hunting it._

  ```bash
  yeswehack-pp-cli hacktivity trends gojek --since-days 90 --json
  ```
- **`hacktivity learn`** — Filtered slice of disclosed reports for a program and CWE - top N by bounty, with severity and writeup links, in pipe-friendly JSON.

  _Lets the agent calibrate from prior art before the hunter writes a single line - turning hacktivity into a learning surface, not just a feed._

  ```bash
  yeswehack-pp-cli hacktivity learn --program gojek --cwe CWE-89 --since-days 90 --json | claude 'summarize what worked'
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**business_units** — Customer organizations that run programs

- `yeswehack-pp-cli business_units` — List business units visible to the user

**events** — Platform events (CTFs, dojos, live sessions)

- `yeswehack-pp-cli events` — List YesWeHack events

**hacktivity** — Public disclosed reports feed (the platform's learning surface)

- `yeswehack-pp-cli hacktivity by_hunter` — List a hunter's disclosed reports
- `yeswehack-pp-cli hacktivity list` — List recently disclosed reports across all public programs

**hunters** — Researcher profiles (other hunters on the platform)

- `yeswehack-pp-cli hunters get` — Get a hunter's public profile (points, rank, impact, achievements)
- `yeswehack-pp-cli hunters list_achievements` — List a hunter's earned achievement badges

**programs** — Bug bounty programs (public and private the user is invited to)

- `yeswehack-pp-cli programs get` — Get a program's full detail (rules, reward grid, scope counts, BU, etc.)
- `yeswehack-pp-cli programs list` — List bug bounty programs the user can see
- `yeswehack-pp-cli programs list_scopes` — List the in-scope and out-of-scope assets for a program

**ranking** — Global researcher leaderboard

- `yeswehack-pp-cli ranking` — Top hunters by points

**taxonomies** — Reference data used by the platform (vulnerability parts, countries, profile URL types)

- `yeswehack-pp-cli taxonomies list_countries` — Country reference list (codes, names)
- `yeswehack-pp-cli taxonomies list_profile_url_types` — Allowed profile URL types (twitter, github, linkedin, etc.)
- `yeswehack-pp-cli taxonomies list_vulnerable_parts` — List vulnerability parts (CWE-like taxonomy used when filing reports)

**user** — Authenticated user account, reports, invitations, email aliases

- `yeswehack-pp-cli user get_self` — Get the authenticated user
- `yeswehack-pp-cli user list_email_aliases` — List the authenticated user's email aliases (per-program forwarding addresses)
- `yeswehack-pp-cli user list_invitations` — List the authenticated user's program invitations
- `yeswehack-pp-cli user list_reports` — List reports the authenticated user has submitted


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
yeswehack-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Triage a hunting session

```bash
yeswehack-pp-cli triage weekend --hours 6 --json --select programs,reports,cwes
```

Single ranked plan for a time-boxed session. --select narrows the JSON to the three high-gravity fields so agents don't burn context on full payloads.

### Detect new scope before competitors do

```bash
yeswehack-pp-cli programs scope-drift --since-days 7 --json
```

Compares this week's scope snapshot to last week's. Catches the asset that quietly got added between Sunday syncs.

### Pre-submit dedupe before drafting

```bash
yeswehack-pp-cli report dedupe --title 'SQLi /api/users/{id}' --asset api.example.com --cwe CWE-89 --json
```

Run before you spend an hour drafting. Exit 2 = high-confidence collision; the agent should stop and look at the matching disclosure.

### Calibrate severity from disclosed reports

```bash
yeswehack-pp-cli hacktivity learn --program gojek --cwe CWE-89 --since-days 90 --json | claude 'summarize the highest-bounty tactics'
```

Pipes a deterministic data slice into an LLM for synthesis - the CLI stays auditable; the model only sees the curated slice.

### Pick the right program for an asset finding

```bash
yeswehack-pp-cli scopes find 'api-v3.*example\\.com' --json --select asset,program_slug,bounty_reward_max
```

Regex lookup across every synced scope. Picks the program with the highest payout when the asset is in multiple scopes.

## Auth Setup

Authentication is JWT-based and tied to your logged-in browser session. Run `yeswehack-pp-cli auth login --chrome` and the CLI reads the access_token from your Chrome profile's localStorage - no copy-paste from DevTools. The JWT refreshes automatically against the OAuth2 token endpoint when it expires. YesWeHack's Personal Access Tokens are gated to manager-tier accounts; the CLI does not support them for the researcher surface.

Run `yeswehack-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  yeswehack-pp-cli business_units --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
yeswehack-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
yeswehack-pp-cli feedback --stdin < notes.txt
yeswehack-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.yeswehack-pp-cli/feedback.jsonl`. They are never POSTed unless `YESWEHACK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YESWEHACK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
yeswehack-pp-cli profile save briefing --json
yeswehack-pp-cli --profile briefing business_units
yeswehack-pp-cli profile list --json
yeswehack-pp-cli profile show briefing
yeswehack-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `yeswehack-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add yeswehack-pp-mcp -- yeswehack-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which yeswehack-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   yeswehack-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `yeswehack-pp-cli <command> --help`.
