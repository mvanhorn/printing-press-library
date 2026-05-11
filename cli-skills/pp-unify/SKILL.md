---
name: pp-unify
description: "The read layer Unify's API doesn't ship. Local search, SQL, and coverage reports over a CRM that has no list-records... Trigger phrases: `look up a company in Unify`, `vet these domains in Unify`, `what changed in our Unify schema`, `Unify coverage vs Salesforce`, `audit Unify scoring fields`, `search Unify records`, `use unify`, `run unify`."
author: "Nate Larkin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - unify-pp-cli
---

# Unify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `unify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install unify --cli-only
   ```
2. Verify: `unify-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Unify's Data API gives you upsert, find-unique, and write — but no way to list, search, or query records. This CLI ships the missing read layer: local SQLite mirror with FTS5 search, read-only SQL across Unify and Salesforce-mirrored objects, schema snapshot/diff, coverage and score-divergence audits, batch CSV vetting, and dry-run-by-default upserts.

## When to Use This CLI

Use this CLI any time you need to read, search, or audit Unify Data API records — the official API and SDKs can write and find-unique but cannot list, search, or join. Reach for it for account-coverage reviews against Salesforce, scoring audits, schema-change tracking, batch domain vetting before outbound sequencing, and dry-run-by-default CSV upserts. Not the right tool for live event streaming or the Intent API (browser pixel) — only the Data API surface.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local read layer over a write-only API
- **`search`** — Full-text search across every synced object's records in one command. Returns typed hits across company, person, opportunity, and every mirrored Salesforce object.

  _When you need to answer 'is this domain in our workspace anywhere?' across Unify and Salesforce-mirrored objects, this is the one command. No N find-unique calls._

  ```bash
  unify-pp-cli search 'gladly' --agent
  ```
- **`sql`** — Run read-only SQL queries with joins across per-object record tables. Cross Unify standard objects with Salesforce-mirrored ones in a single query.

  _Answers operational questions the API can't: industry/segment slices, employee thresholds, joined opportunity/owner views. The query mode an agent reaches for instead of writing scratch Python._

  ```bash
  unify-pp-cli sql "SELECT json_extract(attrs,'$.name') as name, json_extract(attrs,'$.employee_count') as employees FROM record_company WHERE json_extract(attrs,'$.industry')='Retail' AND CAST(json_extract(attrs,'$.employee_count') AS INTEGER) >= 200" --agent
  ```
- **`trace`** — Walk reference attributes (opportunities, people, owner) from a starting record without N+1 API calls.

  _One-shot 'show me everything connected to this company' for spot-checks and pre-call prep._

  ```bash
  unify-pp-cli trace company 73fcb798-9ccd-4138-8f6a-9a801123783c --agent
  ```
- **`watch`** — Persist match-keys for records you care about; sync refreshes them via parallel find-unique on every run.

  _Without a watchlist, sync has nothing to enumerate. With it, your daily refresh hits the records you actually use._

  ```bash
  unify-pp-cli watch add company --match domain=gladly.com
  ```

### Operational audits
- **`coverage`** — Set-difference report between two record tables on a shared key with optional bucketing by attribute and matched-but-stale rows.

  _Tells you which Salesforce accounts are missing from Unify (and vice versa), bucketed by industry/owner — the central artifact of an account-coverage review._

  ```bash
  unify-pp-cli coverage --left salesforce_account --right company --key domain --by industry --agent
  ```
- **`audit-scores`** — Flag records where two numeric attributes diverge beyond a threshold. Powers scoring sanity checks across Unify-internal and Salesforce-mirrored score fields.

  _Catches the 'false-scoring auto-deduct-50pts' class of bugs before they ship into a campaign._

  ```bash
  unify-pp-cli audit-scores --object company --field unify_score --field salesforce_lead_score --threshold 50 --agent
  ```
- **`schema diff`** — Snapshot objects, attributes, and attribute options; diff two snapshots to find adds, removes, and type changes.

  _When the SF-aligned admin adds new fields, you can prove what changed between any two points in time and act on the delta._

  ```bash
  unify-pp-cli schema diff --since 1d --agent
  ```

### AE / outbound workflows
- **`vet`** — Read a CSV column of match values, run find-unique in parallel for each, and enrich each row with exists/has_opportunity/owner/last_activity_at.

  _Pre-launch vet for an outbound sequence drops from N alt-tabs to one command. Bulk pre-flight without bothering an engineer._

  ```bash
  unify-pp-cli vet --csv /tmp/prospects.csv --object company --match-col domain --agent
  ```
- **`import-csv`** — Predict what an upsert from CSV will do (create / update / no-op counts per row) by combining the local mirror with find-unique fallbacks.

  _Stops you from running a 2k-row upsert blind. Reports per-row outcome with a writable plan you can pipe back into upsert._

  ```bash
  unify-pp-cli import-csv --object company --file /tmp/accounts.csv --match-on domain --plan
  ```

## Command Reference

**data** — Manage data

- `unify-pp-cli data create-object` — Create object
- `unify-pp-cli data create-object-attribute` — Create object attribute
- `unify-pp-cli data create-object-attribute-option` — Create object attribute option
- `unify-pp-cli data create-object-record` — Create object record
- `unify-pp-cli data delete-object` — Delete object
- `unify-pp-cli data delete-object-attribute` — Delete object attribute
- `unify-pp-cli data delete-object-attribute-option` — Delete object attribute option
- `unify-pp-cli data delete-object-record` — Delete object record
- `unify-pp-cli data find-unique-object-record` — Find unique object record
- `unify-pp-cli data get-object` — Get object
- `unify-pp-cli data get-object-attribute` — Get object attribute
- `unify-pp-cli data get-object-attribute-option` — Get object attribute option
- `unify-pp-cli data get-object-record` — Get object record
- `unify-pp-cli data list-object-attribute-options` — List object attribute options
- `unify-pp-cli data list-object-attributes` — List object attributes
- `unify-pp-cli data list-objects` — List objects
- `unify-pp-cli data update-object` — Update object
- `unify-pp-cli data update-object-attribute` — Update object attribute
- `unify-pp-cli data update-object-attribute-option` — Update object attribute option
- `unify-pp-cli data update-object-record` — Update object record
- `unify-pp-cli data upsert-object-record` — Upsert object record


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
unify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pre-launch vet a sequence target list

```bash
unify-pp-cli vet --csv /tmp/prospects.csv --object company --match-col domain --agent
```

For each domain, parallel find-unique + local enrichment; outputs one row per input with exists/has_opportunity/owner.

### Find every retail company with >=200 employees and a recent opportunity

```bash
unify-pp-cli sql "SELECT json_extract(c.attrs,'\$.name') as name, json_extract(c.attrs,'\$.domain') as domain FROM record_company c JOIN record_opportunity o ON json_extract(o.attrs,'\$.company_id') = c.id WHERE json_extract(c.attrs,'\$.industry')='Retail' AND o.created_at > date('now','-30 days')" --agent --select rows
```

The query Nate writes scratch Python for today — one SQL, narrowed JSON output for the agent.

### Account coverage against Salesforce, bucketed by industry

```bash
unify-pp-cli coverage --left salesforce_account --right company --key domain --by industry --agent
```

Set-diff per industry bucket: which SF accounts are missing in Unify, which Unify companies have no SF counterpart, which are stale.

### What schema fields did Emily add this week?

```bash
unify-pp-cli schema diff --since 1d --agent
```

Snapshot today, then diff against last Friday's snapshot. Output the per-object adds/removes/type-changes only — the noisy fields you don't care about are filtered.

### Dry-run a 2k-row CSV upsert before writing

```bash
unify-pp-cli import-csv --object company --file /tmp/accounts.csv --match-on domain --plan
```

Predicts per-row creates vs updates vs no-ops by combining the local mirror with find-unique fallbacks. Run it twice; only the second --execute writes.

## Auth Setup

Set UNIFY_API_KEY (generated at Settings → Developers in the Unify dashboard) to a Data API key. The CLI uses the same X-Api-Key header as the official Python and TypeScript SDKs.

Run `unify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  unify-pp-cli data create-object --api-name example-resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
unify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
unify-pp-cli feedback --stdin < notes.txt
unify-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.unify-pp-cli/feedback.jsonl`. They are never POSTed unless `UNIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UNIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
unify-pp-cli profile save briefing --json
unify-pp-cli --profile briefing data create-object --api-name example-resource
unify-pp-cli profile list --json
unify-pp-cli profile show briefing
unify-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `unify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add unify-pp-mcp -- unify-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which unify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   unify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `unify-pp-cli <command> --help`.
