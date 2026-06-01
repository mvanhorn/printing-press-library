---
name: pp-seranking
description: "Printing Press CLI for Seranking. Agent-native OpenAPI subset for SE Ranking Data API and Project API based on official docs at..."
author: "Pimmetjeoss"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - seranking-pp-cli
---

# Seranking — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `seranking-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install seranking --cli-only
   ```
2. Verify: `seranking-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/seranking/cmd/seranking-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Agent-native OpenAPI subset for SE Ranking Data API and Project API based on official docs at https://seranking.com/api.html. Includes high-value read endpoints plus guarded write/recheck audit operations.

## Command Reference

**account** — Manage account

- `seranking-pp-cli account` — Get subscription details

**ai-search** — Manage ai search

- `seranking-pp-cli ai-search discover-brand-by-url` — Discover brand by URL
- `seranking-pp-cli ai-search get-leaderboard` — Get AI search leaderboard
- `seranking-pp-cli ai-search get-overview-time-series` — Get AI search overview time series
- `seranking-pp-cli ai-search get-prompts-by-brand` — Get prompts by brand
- `seranking-pp-cli ai-search get-prompts-by-target` — Get prompts by target

**backlinks** — Manage backlinks

- `seranking-pp-cli backlinks fetch-raw-batch` — Fetch raw backlinks batch
- `seranking-pp-cli backlinks get-anchor-texts` — Get backlink anchor texts
- `seranking-pp-cli backlinks get-count` — Get backlink count
- `seranking-pp-cli backlinks get-cumulative-history` — Get cumulative backlinks history
- `seranking-pp-cli backlinks get-domain-authority` — Get domain authority
- `seranking-pp-cli backlinks get-domain-authority-distribution` — Get domain authority distribution
- `seranking-pp-cli backlinks get-domain-authority-history` — Get domain authority history
- `seranking-pp-cli backlinks get-history-count` — Get backlink history count
- `seranking-pp-cli backlinks get-metrics` — Get backlink metrics
- `seranking-pp-cli backlinks get-page-authority` — Get page authority
- `seranking-pp-cli backlinks get-page-authority-history` — Get page authority history
- `seranking-pp-cli backlinks get-page-or-domain-authority` — Get page or domain authority
- `seranking-pp-cli backlinks get-referring-domain-count` — Get referring domain count
- `seranking-pp-cli backlinks get-referring-domain-history-count` — Get referring domain history count
- `seranking-pp-cli backlinks get-referring-ip-count` — Get referring IP count
- `seranking-pp-cli backlinks get-referring-subnet-count` — Get referring subnet count
- `seranking-pp-cli backlinks get-summary` — Get backlink summary
- `seranking-pp-cli backlinks list` — List backlinks
- `seranking-pp-cli backlinks list-new-or-lost` — List new or lost backlinks
- `seranking-pp-cli backlinks list-new-or-lost-referring-domains` — List new or lost referring domains
- `seranking-pp-cli backlinks list-pages-with` — List pages with backlinks
- `seranking-pp-cli backlinks list-referring-domains` — List referring domains
- `seranking-pp-cli backlinks list-referring-ips` — List referring IPs

**domain** — Manage domain

- `seranking-pp-cli domain analysis-analyze-keyword-overlap-and-gaps` — Analyze keyword overlap and gaps
- `seranking-pp-cli domain analysis-get-competitors` — Get domain competitors
- `seranking-pp-cli domain analysis-get-keyword-rankings` — Get domain keyword rankings
- `seranking-pp-cli domain analysis-get-overview-by-region` — Get domain overview by region
- `seranking-pp-cli domain analysis-get-pages` — Get domain pages
- `seranking-pp-cli domain analysis-get-paid-ads-by-keyword-or` — Get paid ads by keyword or domain
- `seranking-pp-cli domain analysis-get-subdomains` — Get domain subdomains
- `seranking-pp-cli domain analysis-get-traffic-and-keyword-history` — Get traffic and keyword history
- `seranking-pp-cli domain analysis-get-worldwide-overview` — Get worldwide domain overview
- `seranking-pp-cli domain analysis-get-worldwide-url-overview` — Get worldwide URL overview

**keywords** — Manage keywords

- `seranking-pp-cli keywords research-export-metrics` — Export keyword metrics
- `seranking-pp-cli keywords research-get-longtail` — Get longtail keywords
- `seranking-pp-cli keywords research-get-question` — Get question keywords
- `seranking-pp-cli keywords research-get-related` — Get related keywords
- `seranking-pp-cli keywords research-get-similar` — Get similar keywords

**project-management** — Manage project management

- `seranking-pp-cli project-management ai-result-tracker-get-llm-statistics` — Get LLM statistics
- `seranking-pp-cli project-management ai-result-tracker-get-llm-status` — Get LLM status
- `seranking-pp-cli project-management ai-result-tracker-get-prompt-answer` — Get prompt answer
- `seranking-pp-cli project-management ai-result-tracker-get-prompt-rankings` — Get prompt rankings
- `seranking-pp-cli project-management ai-result-tracker-get-site-brand` — Get site brand
- `seranking-pp-cli project-management ai-result-tracker-list-or-get-llm-engines` — List or get LLM engines
- `seranking-pp-cli project-management ai-result-tracker-list-prompts` — List prompts
- `seranking-pp-cli project-management competitors-get-all-competitors-metrics` — Get all competitors metrics
- `seranking-pp-cli project-management competitors-get-competitor-keyword-positions` — Get competitor keyword positions
- `seranking-pp-cli project-management competitors-get-top-10-results` — Get top 10 results
- `seranking-pp-cli project-management competitors-get-top-100-results` — Get top 100 results
- `seranking-pp-cli project-management competitors-list-competitors` — List competitors
- `seranking-pp-cli project-management project-account-get-user-profile` — Get user profile
- `seranking-pp-cli project-management project-account-list-or-manage-sub-accounts` — List or manage sub accounts
- `seranking-pp-cli project-management project-account-list-owned-projects` — List owned projects
- `seranking-pp-cli project-management project-account-list-shared-projects` — List shared projects
- `seranking-pp-cli project-management projects-get-ads-chart-statistics` — Get ads chart statistics
- `seranking-pp-cli project-management projects-get-check-dates` — Get check dates
- `seranking-pp-cli project-management projects-get-historical-dates` — Get historical dates
- `seranking-pp-cli project-management projects-get-keyword-positions` — Get keyword positions
- `seranking-pp-cli project-management projects-get-project-position-history` — Get project position history
- `seranking-pp-cli project-management projects-get-project-summary-statistics` — Get project summary statistics
- `seranking-pp-cli project-management projects-list-or-manage-projects` — List or manage projects
- `seranking-pp-cli project-management projects-list-project-search-engines` — List project search engines
- `seranking-pp-cli project-management projects-list-website-keywords` — List website keywords
- `seranking-pp-cli project-management projects-run-position-check` — Run position check
- `seranking-pp-cli project-management reference-list-google-languages` — List Google languages
- `seranking-pp-cli project-management reference-list-google-regions` — List Google regions
- `seranking-pp-cli project-management reference-list-search-engines` — List search engines

**site-audit** — Manage site audit

- `seranking-pp-cli site-audit create-advanced-audit` — Create advanced audit
- `seranking-pp-cli site-audit create-standard-audit` — Create standard audit
- `seranking-pp-cli site-audit get-audit-history` — Get audit history
- `seranking-pp-cli site-audit get-audit-links` — Get audit links
- `seranking-pp-cli site-audit get-audit-pages-by-issue` — Get audit pages by issue
- `seranking-pp-cli site-audit get-audit-report` — Get audit report
- `seranking-pp-cli site-audit get-audit-status` — Get audit status
- `seranking-pp-cli site-audit get-crawled-pages` — Get crawled pages
- `seranking-pp-cli site-audit get-issues-for-url` — Get issues for URL
- `seranking-pp-cli site-audit list-audits` — List audits
- `seranking-pp-cli site-audit recheck-advanced-audit` — Recheck advanced audit
- `seranking-pp-cli site-audit recheck-standard-audit` — Recheck standard audit


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
seranking-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `seranking-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export SE_RANKING_API_KEY="<your-key>"
```

The generated name `SE_RANKING_API_KEY_QUERY` is also supported for compatibility, but prefer `SE_RANKING_API_KEY` in new automation.

Or persist it in `~/.config/se-ranking-pp-cli/config.toml`.

Run `seranking-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  seranking-pp-cli backlinks list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
seranking-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
seranking-pp-cli feedback --stdin < notes.txt
seranking-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.seranking-pp-cli/feedback.jsonl`. They are never POSTed unless `SERANKING_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SERANKING_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
seranking-pp-cli profile save briefing --json
seranking-pp-cli --profile briefing backlinks list
seranking-pp-cli profile list --json
seranking-pp-cli profile show briefing
seranking-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `seranking-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add seranking-pp-mcp -- seranking-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which seranking-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   seranking-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `seranking-pp-cli <command> --help`.
