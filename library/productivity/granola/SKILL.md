---
name: pp-granola
description: "Every Granola feature — plus offline SQLite cross-meeting search, attendee timelines, and a MEMO pipeline runner... Trigger phrases: `memo run for today's meetings`, `what's in granola but not yet memo'd`, `every meeting we had with trevin`, `did i run the discovery recipe`, `talk time in last week's meetings`, `calendar overlay missed meetings`, `find duplicates in meeting transcripts`, `extract granola meeting`, `use granola`, `run granola`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - granola-pp-cli
---

<!-- // PATCH(skill-doc-auth-rewrite): Auth Setup section rewritten for the
     encrypted-cache install flow (Keychain prompt on first sync, no API key,
     D6 read-only refresh). See library/productivity/granola/.printing-press-patches.json
     patches[6]. -->

# Granola — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `granola-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install granola --cli-only
   ```
2. Verify: `granola-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI reads Granola’s local cache directly and adds the queries Granola.ai’s web app and existing community CLIs cannot answer. Cache-first, then internal API, then public API — transparent fallthrough. memo run, memo queue, attendee timeline, recipes coverage, calendar overlay, and talktime are local-data joins no per-meeting tool produces. Works offline; agent-native JSON by default.

## When to Use This CLI

Reach for granola-pp-cli when you need to answer cross-meeting questions Granola.ai’s web app and the GUI cannot — attendee timelines, MEMO pipeline state, recipes coverage gaps, calendar overlay, talk-time aggregation. It is the right tool for an agent processing transcripts in a loop, a CSM doing pre-call prep, or a consultant running a weekly retro. Pair the --json default with --select dotted paths to keep agent context lean.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Platform Notes

`warm <id> <query>` drives the Granola desktop GUI via AppleScript and is **macOS-only**. It prints what it would do by default; pass `--launch` to actually activate the app. On non-macOS hosts the command exits 0 with a "not supported" message. All other commands are cross-platform.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### MEMO pipeline
- **`memo run`** — Run the preflight → extract pipeline on one meeting or every new meeting since a timestamp, emitting the MEMO three-file artifact and an ndjson run-state ledger.

  _Replaces the per-meeting shell loop that drives the MEMO pipeline — one call, one ndjson stream, agent-readable._

  ```bash
  granola-pp-cli memo run --since 24h --to ~/Documents/Dev/meeting-transcripts --json
  ```
- **`memo queue`** — List every meeting whose transcript is in the cache but whose MEMO triple is not yet on disk.

  _Answers the daily question “what’s still un-MEMO’d?” without the user opening Granola at all._

  ```bash
  granola-pp-cli memo queue --since 7d --json
  ```

### Attendee intelligence
- **`attendee timeline`** — Every meeting with a given attendee, ordered oldest→newest, with title, date, folder, and recipe-applied flag per row.

  _Pre-call prep in one command; surfaces the conversation arc with a single person across months of meetings._

  ```bash
  granola-pp-cli attendee timeline alice@example.com --since 60d --json --select id,title,started_at,folder,recipes
  ```
- **`attendee brief`** — Pulls the last N meetings with an attendee and stitches together their real cached notes plus real AI panel summaries — no synthesis.

  _Eliminates the click-each-meeting copy-paste that account leads do before every external call._

  ```bash
  granola-pp-cli attendee brief alice@example.com --last 3 --panel action-items --json
  ```

### Folders + recipes
- **`folder stream`** — ndjson stream of every meeting in a Granola folder (resolved via documentLists + listRules) with notes and a named panel inlined.

  _Replaces the weekly retro workflow of opening a folder and copy-pasting each meeting’s summary into a spreadsheet._

  ```bash
  granola-pp-cli folder stream client-foo --panel summary --json
  ```
- **`recipes coverage`** — Surface meetings that did NOT have a named panel template/recipe applied within a date range.

  _Friday retro question “did I run the Discovery recipe on every new-prospect call?” answered in one row per gap._

  ```bash
  granola-pp-cli recipes coverage discovery --since 14d --json
  ```

### Transcript analytics
- **`talktime`** — Per-segment-source talk-time for one meeting — microphone (you) vs system (everyone else) in minutes.

  _Confidence column lets you grade transcript accuracy; mic vs system split is the input to “am I talking too much” retros._

  ```bash
  granola-pp-cli talktime 196037d9 --json
  ```
- **`talktime`** — Lifts the per-source talk-time aggregation across N meetings since a date — who-talked-most over time.

  _Time-defrag retro input that no per-meeting tool can produce._

  ```bash
  granola-pp-cli talktime --by participant --since 7d --json
  ```

### Cache-native data
- **`chat list`** — List and dump Granola’s AI chat threads anchored to a meeting (entities.chat_thread + entities.chat_message in the cache).

  _Recovers the AI Q&A history a user has accumulated against a meeting — useful when chasing what you asked about an account weeks ago._

  ```bash
  granola-pp-cli chat list 196037d9 --json
  ```
- **`calendar overlay`** — Left-anti-join meetingsMetadata calendar events with documents.google_calendar_event to find calendared-but-not-recorded meetings.

  _Sarah’s Friday retro and Damien’s “what did I miss” sweep both reduce to this row-level diff._

  ```bash
  granola-pp-cli calendar overlay --week 2026-05-11 --missed-only --json
  ```

### Pipeline hygiene
- **`duplicates scan`** — Hash (title, date-bucket, attendee-email-set) across the cache and a meeting-transcripts repo to surface duplicates at scale.

  _Repos accumulate near-duplicate files when meetings are re-extracted; this returns the dupe groups for cleanup._

  ```bash
  granola-pp-cli duplicates scan --root ~/Documents/Dev/meeting-transcripts --json
  ```
- **`tiptap extract`** — Render documents[id].notes (TipTap JSON: headings, bullet_list, list_item, bold marks, paragraph_break) to canonical markdown instead of falling back to notes_plain.

  _The MEMO summary file’s quality is bounded by extractor fidelity; granola.py loses sub-list hierarchy and bold runs._

  ```bash
  granola-pp-cli tiptap extract 196037d9 --as markdown
  ```

## Command Reference

This CLI exposes 35+ commands. The full tree is too long to inline; ask the CLI for the canonical list:

```bash
granola-pp-cli --help                              # top-level commands
granola-pp-cli <command> --help                    # subcommands + flags
granola-pp-cli agent-context --json                # machine-readable command tree for agents
```

Quick orientation by group:

| Group | Commands | Purpose |
|-------|----------|---------|
| **MEMO pipeline** | `memo run`, `memo queue`, `preflight`, `extract` | Composed three-stream pipeline; reads cache + writes MEMO triple |
| **Meetings** | `meetings list`, `meetings get`, `meetings fetch-batch`, `meetings delete`, `meetings restore`, `show` | List/inspect/mutate meetings (delete/restore mutate via internal API) |
| **Streams** | `notes-show`, `panel get`, `transcript get`, `tiptap extract` | The three streams — human notes, AI panels, transcript — addressable separately |
| **Export** | `export`, `export-all` | Combined three-stream markdown export, single or bulk |
| **Cross-meeting analytics** | `attendee timeline`, `attendee brief`, `folder stream`, `recipes coverage`, `talktime`, `calendar overlay`, `stats frequency`, `stats duration`, `stats attendees`, `stats calendar`, `collect`, `duplicates scan`, `chat list`, `chat get` | Queries no per-meeting tool can answer |
| **Folders / recipes / workspaces** | `folders` (public-API), `folder list`, `folder stream`, `recipes list`, `recipes describe`, `recipes coverage`, `workspaces list` | Granola organizational entities |
| **Public-API mirrors** | `notes list`, `notes get`, `folders` | Typed Bearer-key endpoints |
| **Sync / system** | `sync`, `sync-api`, `doctor`, `auth login`, `auth status`, `auth set-token`, `auth logout`, `which`, `agent-context`, `version`, `import` | Local store hydration, auth, capability discovery, batch import |
| **GUI bridge** | `warm` (macOS only) | Drives Granola desktop app via AppleScript |

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
granola-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily MEMO loop

```bash
granola-pp-cli memo run --since 24h --to ~/Documents/Dev/meeting-transcripts --json
```

Process every new meeting since yesterday into the MEMO triple format and yield only the new artifacts.

### Pre-call attendee brief

```bash
granola-pp-cli attendee brief alice@example.com --last 3 --panel action-items --json --select meetings.title,meetings.started_at,panels.action_items
```

Pull the last three meetings with Trevin and only the title, date, and action-items panel content per meeting.

### Friday retro — missing recipes

```bash
granola-pp-cli recipes coverage discovery --since 14d --json
```

Surface every new-prospect call in the last fortnight that did not have the Discovery panel applied. Omit the slug to list coverage gaps across every panel template.

### Repo-wide duplicate scrub

```bash
granola-pp-cli duplicates scan --root ~/Documents/Dev/meeting-transcripts --json
```

Find duplicate-meeting clusters across the MEMO output repo for cleanup.

### Calendar-overlay missed-meeting sweep

```bash
granola-pp-cli calendar overlay --week 2026-05-11 --missed-only --json
```

Calendared meetings with no Granola recording — weekly accountability check.

## Auth Setup

1. **Install Granola desktop and sign in.** The CLI reads the local cache and tokens the desktop manages. No CLI-side credentials to configure.

2. **Run any command.** On macOS, the first invocation that needs the cache or tokens (typically `granola-pp-cli sync`) triggers a Keychain prompt for `Granola Safe Storage`. Click "Always Allow" so subsequent runs are silent. The CLI uses Granola's own Keychain-stored encryption key to decrypt `cache-v6.json.enc` and `supabase.json.enc`.

3. **CLI is read-only against the refresh token.** Granola desktop owns rotation; the CLI never calls `RefreshAccessToken` against the encrypted token store because rotating it there would sign Granola desktop out next time the desktop tries to refresh. If a request fails with "token expired", open Granola desktop briefly to refresh, then re-run the CLI command.

4. **Power users / CI: `GRANOLA_WORKOS_TOKEN`.** Setting this env var bypasses the Keychain prompt entirely and accepts the refresh-rotation trade-off (rotating the token via the CLI will sign the desktop out). Use only when CLI-side refresh is required, typically for headless agents.

Optional public REST API path: set `GRANOLA_API_KEY` for the typed `notes` and `folders` top-level commands at `public-api.granola.ai`. Most workflows do not need this.

Run `granola-pp-cli doctor` to verify setup. The "Encrypted store" line distinguishes four states: not installed, pre-encryption Granola, present-but-not-yet-synced, ok, or last-sync-failed-with-class.

### Troubleshooting

| `doctor` says... | What to do |
|---|---|
| `INFO no Granola install detected` | Install Granola desktop from granola.ai and sign in. |
| `INFO not in use (Granola pre-encryption)` | Granola desktop pre-encryption versions wrote plaintext files; the CLI still reads them. Upgrade Granola desktop to pick up the encrypted store. |
| `INFO present; run sync to authorize Keychain access` | Run `granola-pp-cli sync`. Click "Always Allow" on the macOS prompt. |
| `OK ok` | Last successful sync recorded. Token source and document-fetch count visible in `--json` output. |
| `ERROR last sync failed to decrypt (key_unavailable)` | Sign back into Granola desktop, re-run sync, accept the Keychain prompt. |
| `ERROR last sync failed to decrypt (decrypt_failed)` | Encryption scheme may have drifted with a Granola update. File an issue with the doctor output. |

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  granola-pp-cli folders --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — `sync` and the `meetings list --query <text>` FTS path use the local SQLite store
- **Non-interactive** — never prompts, every input is a flag
- **Mostly read-only** — `meetings delete`, `meetings restore`, `import`, and `warm --launch` are the only commands that mutate state; every other command inspects, exports, syncs, or analyzes

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
granola-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
granola-pp-cli feedback --stdin < notes.txt
granola-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.granola-pp-cli/feedback.jsonl`. They are never POSTed unless `GRANOLA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GRANOLA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
granola-pp-cli profile save briefing --json
granola-pp-cli --profile briefing folders
granola-pp-cli profile list --json
granola-pp-cli profile show briefing
granola-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `granola-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add granola-pp-mcp -- granola-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which granola-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   granola-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `granola-pp-cli <command> --help`.
