# なっぷ (Nap Camp) CLI

**Plan campervan campsite stays with source-backed pitch rules and dated acceptance evidence.**

Discover public campervan-entry campsites, inspect the specific pitch and compare a bounded shortlist. Planner commands distinguish source acceptance and starting prices from unknown vehicle clearance, vacancy and full totals.

## Install

After this CLI is merged into the public library, install the CLI and skill with:

```bash
npx -y @mvanhorn/printing-press-library install nap-camp --cli-only
nap-camp-pp-cli --version
npx skills add mvanhorn/printing-press-library --skill pp-nap-camp
```

The installer defaults binaries to `~/.local/bin` on macOS/Linux. Ensure that directory is on the invoking runtime's PATH. Before merge, or to build this source directly with Go 1.26.6 or newer:

```bash
go build -o nap-camp-pp-cli ./cmd/nap-camp-pp-cli
./nap-camp-pp-cli version
```

A direct Go install after merge is also supported:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/nap-camp/cmd/nap-camp-pp-cli@latest
```

The generated optional MCP server uses stdio. Build it from `./cmd/nap-camp-pp-mcp` when needed; installation does not enable a server. The snapshot tool is write-capable because its optional `--save-to` flag creates a new local observation file.

## Authentication

Public read-only source contracts need no API key or account. Booking is a canonical-site handoff.

## Quick Start

```bash
# Check installation without network requests.
nap-camp-pp-cli doctor --dry-run

# Find bounded campervan-entry candidates.
nap-camp-pp-cli campsite discover --region kanto --limit 3 --json

# Read the chosen pitch restrictions.
nap-camp-pp-cli pitch inspect 11007 20005062 --json

# Surface contradictions and unresolved requirements.
nap-camp-pp-cli planner fit 11007 20005062 --people 2 --power --agent

```

## Evidence and Limits

- A campervan-entry facility category does not establish permission for every vehicle or pitch. Physical length, width and height remain unknown even when a pitch area is listed.
- Specific plan capacity, passenger-car/campervan memo, pets, AC power, season, group-wide rules, check-in cutoff and cancellation terms can affect suitability. Read the source text and contact the facility when unresolved.
- Calendar codes preserve the public legend: 0 準備中, 1 受付中, 2 残りわずか, 3 受付終了, 4 キャンセル発生通知対象. Acceptance candidates are not a promise of vacancy.
- Prices are starting-price evidence in JPY. Group, options, taxes and full dated totals remain unknown. A zero or missing price is unknown, including closed/preparation dates; it never means free.
- Dates use Asia/Tokyo (JST). For check-in D and checkout D+N, only N stay nights are evaluated. Windows can use following-month nights only when included in the source response.
- Discovery reads one explicit page, returns at most 50 facilities and at most three plan previews per facility. Inspections bound plan previews to 25. Calendar and windows are bounded. Text longer than 4,000 characters is marked in `text_truncated_fields`.
- Data is observed from Japanese public listings and carries UTC observation timestamps, canonical URLs and source coverage. It is not a complete offline database or a driving-clearance routing tool.

The published transport reads at most 4 MiB plus one overflow sentinel byte, then fails explicitly instead of returning a partial response. This applies to planning and raw source commands, success and error bodies, transparent decompression and explicit compressed-body inflation. Normalized list/text limits remain separate. The earlier local build had a post-read allocation limitation; the publication review resolved it in this CLI without changing the Printing Press.

## Commands

| Command | Use |
|---|---|
| `campsite discover` | One bounded campervan-entry result page with region/facility/date context |
| `campsite inspect` | Facility season, fees, rules and bounded plan previews |
| `pitch inspect` | Specific plan entry memo, capacity, power, pets and rules |
| `pitch calendar` | Requested-month acceptance and starting-price evidence |
| `catalog` | Source regions, prefectures and filter labels |
| `planner fit` | Conservative selected-pitch requirement checklist |
| `planner compare` | 1–5 selected campsite:plan pairs; no subjective ranking |
| `planner windows` | 1–14-night source acceptance candidates, excluding checkout |
| `planner snapshot` | Fresh versioned observation and optional new JSON file |
| `planner changes` | Domain differences between comparable saved observations |
| `source` | Advanced raw public contracts; inspect help and select fields |

Run the command's `--help` for current flag names and argument shapes. `source` retains raw source payloads and can be verbose; the planning commands above provide bounded projections.

## Agent Usage

Use `--agent` for structured, noninteractive output and `--select` to choose known fields. For example:

```bash
nap-camp-pp-cli planner snapshot 11007 20005062 --agent --select campsite.name,pitch.vehicle_entry,pitch.power
nap-camp-pp-cli pitch calendar 11007 20005062 --month 2026-10 --limit 7 --json
```

JSON lists stay `[]` when empty. A fit checklist can return `contradiction` or `needs_confirmation`; it never certifies overall clearance. A compare result includes per-pair errors and exits nonzero on incomplete source acquisition. For the MCP comparison tool, provide a whitespace-separated `pair` string, or the first `pair` plus the advertised `args` string for additional campsite:plan pairs. Arrays are not advertised and flag-like tokens are rejected. The MCP bridge keeps that operation failed and preserves complete bounded JSON stdout as `partial_output` beside the error. Opaque or oversized output is explicitly a bounded preview, with no fabricated structured result. `--data-source local` is rejected by fresh source commands; `planner changes` reads local observation files and rejects `--data-source live`.

## Health Check

```bash
nap-camp-pp-cli doctor --json
nap-camp-pp-cli agent-context --pretty
```

Doctor checks installation and source reachability. Agent context reports runtime configuration/data/cache locations. Health does not prove inventory or vehicle suitability.

## Cookbook

Save a full observation independently of display projection:

```bash
nap-camp-pp-cli planner snapshot 11007 20005062 --month 2026-10 --save-to before.json --json
nap-camp-pp-cli planner snapshot 11007 20005062 --month 2026-10 --save-to after.json --json
nap-camp-pp-cli planner changes before.json after.json --json
```

`--save-to` requires an existing parent directory and refuses existing destination files. An omitted month saves `calendar: []`. Changes ignores observation timestamp differences, requires identical campsite/pitch IDs and schema version 1, and reports dates covered by only one file separately from comparable changes.

```bash
nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --limit 5 --agent
nap-camp-pp-cli planner compare 11007:20005062 11007:20005063 --people 2 --power --agent
nap-camp-pp-cli campsite discover --region kanto --power --shower --limit 3 --json
```

## Configuration

The default config path is `~/.config/nap-camp-pp-cli/config.toml`. Public source commands need no credentials. `NAP_CAMP_BASE_URL` supports a controlled source fixture or explicit override; normal source origin is `https://www.nap-camp.com`. `--timeout` bounds individual HTTP requests; retry backoff can extend process runtime. Fresh planning commands always request new source evidence. Optional framework learning/state remains local.

Forgetting a teaching atomically reconciles its affected inferred rules against all retained family bindings. Only positive `boost` actions from eligible teaching sources can create or support inferred rules; `hide` and `alias_of` rows remain stored and cannot become positive support or example provenance. Rules supported by at least two distinct compatible retained entity/value bindings remain with valid example provenance. Stale, mismatching or multi-entity family rows do not supply support or veto other valid bindings; fewer than two compatible bindings removes the rule. Explicit taught rules and unrelated families remain. Explicit pattern teaching replaces its requested scope and examples; later inference preserves that manual payload. Prefix IDs are literal and must resolve uniquely before recall verifies a hit.

Known cached identities also gate direct alias promotion and generalized recall: an explicit conflicting resource name remains a mismatch. Generalized recall validates only the entity actually used to substitute the target ID; an unrelated entity in the same query cannot rescue a conflicting target. All ID-verified bindings of the same pattern reach cached-identity validation, so a rejected binding cannot suppress a later valid one. Final ranking chooses the best accepted hit per typed target, including exact patterns over direct partial matches; exact direct matches retain precedence. Emitted target IDs are removed from rejected diagnostics before their limit. Distinct resource type/ID tuples stay distinct even when either string contains a separator. The recall limit applies after identity validation, final confidence/source ranking and deduplication. It caps returned results rather than pattern-search or database work; matching local bindings are evaluated exhaustively to preserve rank and diagnostic coverage. A synthetic local SQLite measurement at one returned result used five ordinary CLI process runs per size: 100 matching patterns had a 14.635 ms median and 1,000 had a 25.276 ms median, including startup and local telemetry. Every run selected the later highest-confidence target. These observations are not a latency guarantee or a cap on validation work. Unexpected SQLite payload-read and cancellation failures return an explicit recall error; only a genuinely absent row permits the compatibility fallback. Missing resources retain a warning-bearing teaching fallback; an empty cached identity retains its existing direct partial or identifier-verified pattern classification.

Use `--no-learn` or `NAP_CAMP_NO_LEARN=true` for deterministic invocations. `agent-context --pretty` is the authority for actual state locations. Read-only mirrored MCP tools enforce a child context that suppresses automatic journals, flag derivation and cache writes. The recall, learnings list/candidates/stats and playbook list helpers are marked as local writes because they intentionally measure, prune or initialize learning state; they retain their current read/write behavior and can read committed active-WAL changes. Ordinary CLI invocations retain the disclosed optional local learning behavior.

## Troubleshooting

- Invalid IDs, dates, limits or requirement units: inspect the leaf command's help; usage errors exit 2.
- HTTP 429 or a source restriction: stop and retry later as directed by the source. Direct classified throttling exits 7. Normal mode may retry short-interval 429s until `--timeout`; a deadline then exits 5 with Nap Camp request context and no availability data. No CAPTCHA, login, protection or provider restriction is bypassed.
- Changed public response shape: commands fail explicitly with a source-contract error. Recheck the canonical site before relying on older observations.
- Existing snapshot destination: choose a new path; existing observations are preserved.
- Empty discovery or windows: source evidence can be empty. Missing dates remain explicit unknowns and do not create acceptance candidates.

## Source and Verification

Native ordinary Chrome navigation/DOM/accessibility established the public workflows, then observed first-party assets supplied exact request contracts for HTTP replay. No CDP, cookie import, account, booking or resident browser is used. Deterministic tests, full Printing Press shipcheck, independent reviews, a fresh publication live matrix and measured performance results are documented in the public manuscripts. Raw responses and machine-specific build evidence remain private. No SDK/MCP wrapper was credited because none contributed a verified service-specific feature in scoped research.

## Recipes

### Compare specific pitches

```bash
nap-camp-pp-cli planner compare 11007:20005062 11007:20005063 --people 2 --agent
```

Each pair is checked independently; facility categories are not pitch permission.

### Consecutive candidate nights

```bash
nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --limit 5 --agent
```

Only nights with source acceptance statuses 1/2 form candidates; vacancy is unknown.

### Narrow pitch facts

```bash
nap-camp-pp-cli planner snapshot 11007 20005062 --agent --select campsite.name,pitch.vehicle_entry,pitch.power
```

Return only the selected source facts; those fields do not establish vehicle dimensions or a full dated total.

### Read a dated source calendar

```bash
nap-camp-pp-cli pitch calendar 11007 20005062 --month 2026-10 --limit 7 --json
```

Filter the two-month upstream response to the requested month; zero nonacceptance prices stay unknown.

## Unique Features


### Pitch evidence
- **`planner fit`** — Check one specific pitch for vehicle, group, pet and power requirements without certifying unreported dimensions.

  _Choose this for explicit contradictions and unknowns on a chosen pitch._

  ```bash
  nap-camp-pp-cli planner fit 11007 20005062 --people 2 --length-m 6 --agent
  ```
- **`planner compare`** — Compare a bounded shortlist of specific campsite/pitch pairs with the same requirements.

  _Choose this when comparing selected pitches across an itinerary._

  ```bash
  nap-camp-pp-cli planner compare 11007:20005062 11007:20005063 --people 2 --agent
  ```

### Dates and observations
- **`planner windows`** — Find candidate consecutive nights from dated source acceptance evidence.

  _Choose this for multi-night planning with explicit unknown inventory._

  ```bash
  nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --agent
  ```
- **`planner snapshot`** — Capture a compact dated observation spanning campsite, pitch and optional calendar.

  _Choose this to save a reviewable itinerary observation._

  ```bash
  nap-camp-pp-cli planner snapshot 11007 20005062 --month 2026-10 --agent
  ```
- **`planner changes`** — Compare two saved observations of the same campsite and pitch.

  _Choose this to detect source changes without repeating HTTP requests._

  ```bash
  nap-camp-pp-cli planner changes before.json after.json --agent
  ```

Created by [@zjsng](https://github.com/zjsng).
