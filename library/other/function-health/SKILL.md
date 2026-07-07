---
name: pp-function-health
description: "Every Function Health feature, plus a local SQLite store with biomarker trends across every round you've ever drawn Trigger phrases: `show me my function health labs`, `what's worst on my function panel`, `trend my apoB`, `make me a pdf for my doctor with my function history`, `what changed since my last function draw`, `use function-health`, `run function-health-pp-cli`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - function-health-pp-cli
---

# Function Health — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `function-health-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install function-health --cli-only
   ```
2. Verify: `function-health-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Function Health gives you a great single-round dashboard but no way to see what's drifting across years of draws — and no way to share a clean, branded lab history with your physician. function-health-pp-cli pulls every round into a local SQLite store with FTS5, exposes cross-round trend analysis nobody else offers (`goat`, `biomarker trend`, `biomarkers trending`, `biomarkers oscillating`), and renders a Function-branded PDF with your name and date of birth that an MD can read in 30 seconds.

## When to Use This CLI

Reach for function-health-pp-cli whenever an agent is asked to reason about lab trends over time, to prioritize which biomarker the user should worry about, or to produce an artifact a clinician will actually read. It is the right tool when the question requires joining data across multiple test rounds, when the user wants a branded PDF for a doctor, or when an agent needs a focused Markdown context block for one biomarker rather than the entire JSON export. It is the wrong tool for one-off single-result lookups on a draw that hasn't been synced yet — for those, run `sync` first or query the Function web app directly.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Doctor-ready outputs
- **`export pdf-for-doctor`** — Render a Function-branded multi-round lab report PDF with your name and date of birth, suitable for emailing to your personal physician.

  _When an agent needs to produce something the user will hand to a third party, this is the durable artifact — not a JSON blob._

  ```bash
  function-health-pp-cli export pdf-for-doctor --out ~/Downloads/function-history.pdf
  ```

### Local-SQL trend analysis
- **`biomarker trend`** — Every value of a biomarker across every round you've ever drawn, with the delta from Function's optimal range, an ASCII sparkline in the terminal, and structured JSON for agents.

  _When the agent is asked 'how has X changed', this is the one query that answers it without re-fetching anything._

  ```bash
  function-health-pp-cli biomarker trend ApoB --json
  ```
- **`goat`** — Ranks every biomarker by distance-from-optimal multiplied by its slope-away-from-optimal across the last 3 rounds; returns the single most worrying biomarker right now with reasoning fields.

  _When the agent is asked 'what should I worry about', this is the one-call answer._

  ```bash
  function-health-pp-cli goat --agent
  ```
- **`biomarkers trending`** — Lists every biomarker whose slope across the last N rounds points away from Function's optimal range, sorted by magnitude.

  _Lets the agent prioritize a long biomarker list without burning the user's context window on every reading._

  ```bash
  function-health-pp-cli biomarkers trending --direction worse --last 3 --json
  ```
- **`category trend`** — For one of the ~13 categories, returns a per-round aggregate score (percent of biomarkers inside Function's optimal range) over time.

  _Rolls up 100+ biomarkers into a single trajectory per body system, so the agent can summarize organ-level changes._

  ```bash
  function-health-pp-cli category trend cardiovascular --json
  ```
- **`biomarkers oscillating`** — Biomarkers that crossed the optimal-range boundary at least twice in the last N rounds — flags instability separate from trend.

  _Distinguishes 'unstable measurement noise' from 'consistently bad' — important when the agent is reasoning about whether a single high reading matters._

  ```bash
  function-health-pp-cli biomarkers oscillating --rounds 4
  ```
- **`recommendations stale`** — Recommendations (supplements / foods) whose target biomarker is STILL outside Function's optimal range, joined by Quest code. The /recommendations endpoint has no issued-date, so staleness is by outcome not age; --min-rounds tightens to persistence and --group (supplements, foods_to_eat, foods_to_avoid) focuses a large set.

  _Closes the loop between guidance and outcome — the agent can ask 'did the last fix work?' instead of 'what is recommended now?'._

  ```bash
  function-health-pp-cli recommendations stale --json
  ```

### Agent-native plumbing
- **`bundle`** — Composes a single Markdown file with a biomarker's full history, every clinician note mentioning it (FTS5), Function's optimal range, and relevant recommendations — ready to paste into Claude or ChatGPT.

  _When the agent is being asked to draft a question for the user's clinician, this is the prefab context block._

  ```bash
  function-health-pp-cli bundle ApoB --window 3rounds
  ```

## Command Reference

**biological_age** — Biological-age and BMI calculations Function derives from your panels.

- `function-health-pp-cli biological-age bio-age` — Your biological age vs. chronological age. 404 if not yet calculated.
- `function-health-pp-cli biological-age bmi` — BMI with the weight and height inputs Function recorded.

**biomarkers** — Function Health's full biomarker catalog with names, units, reference ranges, and Function's "optimal" ranges.

- `function-health-pp-cli biomarkers get` — Get one biomarker's full result history across every round you've drawn, with values, units, status, and ranges.
- `function-health-pp-cli biomarkers list` — List every biomarker the platform knows about (across all categories).

**categories** — The ~13 medical categories biomarkers are organized into (Heart, Hormones, Thyroid, Metabolic, Liver, Kidney, etc.).

- `function-health-pp-cli categories` — List every biomarker category with the biomarkers nested under it.

**notes** — Clinician notes attached to your results.

- `function-health-pp-cli notes` — List all clinician notes (annotations on biomarkers / rounds).

**notifications** — Change notifications (new results landing, biomarker direction changes).

- `function-health-pp-cli notifications` — Read all unread/read notifications about result changes.

**recommendations** — Personalized health recommendations tied to your results.

- `function-health-pp-cli recommendations` — Per-category health guidance. May 404 if Function hasn't computed any yet.

**requisitions** — Lab requisitions — pending (in-progress draws) and completed (rounds with results).

- `function-health-pp-cli requisitions completed` — Completed requisitions (your past test rounds, keyed by requisitionId).
- `function-health-pp-cli requisitions pending` — Currently in-progress requisitions (lab orders awaiting completion).

**results** — Lab results — both the structured biomarker-level data and the raw requisition documents.

- `function-health-pp-cli results list` — Raw requisition / PDF result data (less useful for queries; see results-report for structured values).
- `function-health-pp-cli results report` — The structured lab-results report — every biomarker, every round, with value, unit, status, Quest reference range

**schedules** — Upcoming scheduled lab-draw appointments.

- `function-health-pp-cli schedules` — Upcoming scheduled lab visits you've booked.

**user** — Your Function Health member profile (id, name, contact info, membership status).

- `function-health-pp-cli user` — Get the authenticated member profile. Used by doctor for reachability + auth check.

**visits** — Individual lab-collection events within a test round.

- `function-health-pp-cli visits` — List every visit (draw event) — useful when a round has multiple draws.

**wearables** — Wearable-device integrations (Apple Health, Garmin, Whoop, Oura, etc.).

- `function-health-pp-cli wearables` — List the wearable apps Function currently supports for integration.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
function-health-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning check-in

```bash
function-health-pp-cli sync check && function-health-pp-cli goat
```

Cheap incremental sync, then the single most-worrying biomarker right now — both calls are local after the first sync.

### Doctor-ready PDF

```bash
function-health-pp-cli export pdf-for-doctor --out ~/Downloads/heart.pdf --section Heart --out-of-range
```

Branded multi-round report with member name and DOB header, per-category sections, per-biomarker history. --out-of-range keeps only biomarkers outside Function's optimal range in their latest draw; --section <name> keeps only categories whose name contains the text (e.g. Heart, Liver, Nutrients); the two combine. Omit both for the full report.

### Narrow JSON for agents

```bash
function-health-pp-cli biomarker trend ApoB --agent --select \\brounds.draw_date,rounds.value,rounds.status\\b
```

Strip the response down to the three fields an agent actually reasons about; `--select` accepts dotted paths into nested response objects.

### LLM context bundle

```bash
function-health-pp-cli bundle hs-CRP --window 3rounds | pbcopy
```

Compose a Markdown context block — history + clinician notes + recommendations — and paste into Claude or ChatGPT for a focused conversation.

### Stale-rec audit

```bash
function-health-pp-cli recommendations stale --group supplements --json
```

Recommendations (supplements / foods) whose target biomarker is still outside Function's optimal range, joined by Quest code. The endpoint carries no issued-date, so staleness is by outcome, not age; --min-rounds N tightens to persistence and --group focuses one set.

## Auth Setup

Function Health blocks Firebase REST email/password sign-in at the project level, so `function-health-pp-cli auth login` with `--email/--password` does not work and `auth login --chrome` is unreliable. The working path is `auth set-token`: log in to https://my.functionhealth.com in Chrome, open DevTools (Cmd+Option+I) -> Network, copy the `authorization: Bearer <token>` value from any `member-app-mid.functionhealth.com/api/v1/...` request, then run `function-health-pp-cli auth set-token <token>`. The token is stored in `~/.config/function-health-pp-cli/config.toml` (mode 0600) and lasts ~1 hour; re-run `auth set-token` on `HTTP 401`. Set `FUNCTION_HEALTH_TOKEN` to override for CI or one-shot use.

Run `function-health-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  function-health-pp-cli biomarkers list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
function-health-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
function-health-pp-cli feedback --stdin < notes.txt
function-health-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/function-health-pp-cli/feedback.jsonl`. They are never POSTed unless `FUNCTION_HEALTH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FUNCTION_HEALTH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
function-health-pp-cli profile save briefing --json
function-health-pp-cli --profile briefing biomarkers list
function-health-pp-cli profile list --json
function-health-pp-cli profile show briefing
function-health-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `function-health-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add function-health-pp-mcp -- function-health-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which function-health-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   function-health-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `function-health-pp-cli <command> --help`.
