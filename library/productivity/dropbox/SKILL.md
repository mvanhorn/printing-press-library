---
name: pp-dropbox
description: "Find duplicates, sync conflicts, and stale shared links across a whole Dropbox, then clean up with plans you preview first. Trigger phrases: `clean up my dropbox`, `find duplicate files in dropbox`, `what's taking up space in dropbox`, `organize my camera uploads`, `revoke dead or public dropbox shared links`, `use dropbox`, `run dropbox-pp-cli`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "overview | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dropbox-pp-cli
    install:
      - kind: go
        bins: [dropbox-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/dropbox/cmd/dropbox-pp-cli
---

# Dropbox Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dropbox-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install dropbox --cli-only
   ```
2. Verify: `dropbox-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/dropbox/cmd/dropbox-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

dropbox-pp-cli copies the metadata of every file in your Dropbox into a local SQLite index, so finding where the space went or which files are byte-identical takes seconds and downloads nothing. The cleanup commands write plan files that you check and preview before anything changes; apply runs them in Dropbox batch jobs and records each change in a journal that undo can reverse. File deletes are Dropbox soft deletes that undo can restore, revoking a shared link is permanent, and code folders like node_modules and .git are skipped unless you opt in.

## When to Use This CLI

Use this CLI when an agent needs to understand or reorganize a large Dropbox: where the space goes, duplicate and conflict cleanup, rule-based filing, and shared-link review. It fits bulk changes that should be checked, previewed, and reversible.

## Anti-triggers

Do not use this CLI for:
- Editing file contents or collaborating on documents
- Syncing a local folder continuously (use the Dropbox desktop app)
- Permanently deleting files or pruning revisions
- Dropbox Business team administration (members, groups, audit logs)
- Dropbox Sign e-signatures (use the dropbox-sign CLI)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Find the mess

- **`conflicts`**: Find Dropbox conflicted copies and Selective Sync Conflict folders, compare each one to its original file by file, and plan removal only for copies that add nothing.

  _Use after device syncs leave conflicted copies or Selective Sync Conflict folders. Only identical copies and copies whose files all exist in the original go into the plan._

  ```bash
  dropbox-pp-cli conflicts --plan conflicts.json --agent
  ```
- **`links audit`**: List every shared link you own with its visibility and expiry, flag links to files that no longer exist, and plan bulk revokes.

  _Use for a privacy pass over old shared links. Nothing changes until apply runs with --yes, and revoked links cannot be restored._

  ```bash
  dropbox-pp-cli links audit --agent
  ```
- **`overview`**: See quota, space by top-level folder, file type, and year, real duplicate totals, conflict copies, and how much space code folders like node_modules take.

  _Run it first in any cleanup session, after index, to decide where to look. Duplicate and conflict totals skip code folders; pass --include-dev-dirs to the finder commands to include them._

  ```bash
  dropbox-pp-cli overview --agent
  ```
- **`mess`**: List empty folders, single-file folders, loose files at the root, 'Copy of' and '(1)' names, very deep paths, and near-duplicate folder names like Taxes and Taxes 2.

  _Use it to turn a vague 'clean this up' into a concrete list. Its plan only removes empty folders._

  ```bash
  dropbox-pp-cli mess --agent
  ```

### Change safely

- **`organize`**: Turn a glob and a destination template like /Photos/{year}/{month} into a move plan, with missing folders and name collisions worked out before anything moves.

  _Use when someone describes a filing rule. It writes a plan for plan check and apply instead of moving files directly. Dates use local time unless you pass --tz._

  ```bash
  dropbox-pp-cli organize --match '*.jpg' --under '/Camera Uploads' --to '/Photos/{year}/{month}' --plan photos.json
  ```
- **`apply`**: Preview a plan, then run it with --yes in Dropbox batch jobs of up to 1000 items that retry on lock contention and record every change in a local journal.

  _Run every cleanup plan through apply. Without --yes it only previews and lists any extra flags the plan needs (--allow-dev-dirs, --allow-cross-share, --allow-nonempty-delete, --allow-unshare). It refuses plans whose files changed since planning._

  ```bash
  dropbox-pp-cli apply photos.json
  ```
- **`plan check`**: Check a plan against the index before it runs: files that changed since planning, name collisions, case-only renames, code folders, and moves that would change who can see a file.

  _Run it on every plan, especially plans an agent wrote by hand. Exit code 2 means the plan has errors._

  ```bash
  dropbox-pp-cli plan check photos.json --agent
  ```
- **`undo`**: Reverse an applied batch: move files back and restore soft-deleted files from the revision recorded when they were deleted.

  _Use when an applied plan looks wrong. Restores work within Dropbox's restore window, at least 30 days on personal plans. Folders created by apply stay in place, and revoked links cannot be restored._

  ```bash
  dropbox-pp-cli undo 20261006-153012-abcd --yes
  ```
- **`journal`**: List applied batches with their results and how many days remain to restore deleted files.

  _Check it after apply to confirm results and to get the batch ID for undo._

  ```bash
  dropbox-pp-cli journal --agent
  ```

## Command Reference

**file-requests**: Inspect and manage file requests.

- `dropbox-pp-cli file-requests count`: Return the total number of open and closed file requests.
- `dropbox-pp-cli file-requests delete-all-closed`: Delete all closed file requests owned by the current user.
- `dropbox-pp-cli file-requests get`: Return a specified file request.
- `dropbox-pp-cli file-requests list`: Return file requests owned by the current user.
- `dropbox-pp-cli file-requests list-continue`: Continue paginating file requests using a cursor.

**files**: Inspect and manage Dropbox files and folders.

- `dropbox-pp-cli files copy`: Copy a file or folder to another location in the user's Dropbox.
- `dropbox-pp-cli files copy-batch`: Copy multiple files or folders to different locations in the user's Dropbox.
- `dropbox-pp-cli files copy-batch-check`: Return the status and per-entry results of an asynchronous copy batch.
- `dropbox-pp-cli files create-folder`: Create a folder at a given path.
- `dropbox-pp-cli files create-folder-batch`: Create multiple folders at once.
- `dropbox-pp-cli files create-folder-batch-check`: Return the status of an asynchronous create-folder batch.
- `dropbox-pp-cli files delete`: Delete a file or folder at a given path.
- `dropbox-pp-cli files delete-batch`: Delete multiple files or folders at once.
- `dropbox-pp-cli files delete-batch-check`: Return the status and per-entry results of an asynchronous delete batch.
- `dropbox-pp-cli files get-latest-cursor`: Get a cursor for the current folder state without returning entries.
- `dropbox-pp-cli files get-metadata`: Return metadata for a file or folder.
- `dropbox-pp-cli files get-temporary-link`: Get a temporary file content link that expires in four hours.
- `dropbox-pp-cli files get-thumbnail-batch`: Get thumbnails for up to 25 images.
- `dropbox-pp-cli files list-folder`: Start returning the contents of a folder.
- `dropbox-pp-cli files list-folder-continue`: Continue listing a folder or retrieve changes using its cursor.
- `dropbox-pp-cli files list-revisions`: Return revisions for a file path or file ID.
- `dropbox-pp-cli files move`: Move a file or folder to another location in the user's Dropbox.
- `dropbox-pp-cli files move-batch`: Move multiple files or folders to different locations in the user's Dropbox.
- `dropbox-pp-cli files move-batch-check`: Return the status and per-entry results of an asynchronous move batch.
- `dropbox-pp-cli files restore`: Restore a specific revision of a file to the given path.
- `dropbox-pp-cli files search`: Search for files and folders.
- `dropbox-pp-cli files search-continue`: Fetch the next page of search results.
- `dropbox-pp-cli files tags-add`: Add a tag to an item.
- `dropbox-pp-cli files tags-get`: Get tags assigned to items.
- `dropbox-pp-cli files tags-remove`: Remove a tag from an item.

**sharing**: Inspect and manage shared links and shared folders.

- `dropbox-pp-cli sharing create-shared-link`: Create a shared link with custom settings.
- `dropbox-pp-cli sharing get-shared-link-metadata`: Get metadata for a shared link.
- `dropbox-pp-cli sharing list-folder-members`: Return shared folder membership by folder ID.
- `dropbox-pp-cli sharing list-folders`: Return shared folders accessible to the current user.
- `dropbox-pp-cli sharing list-folders-continue`: Continue paginating shared folders using a cursor.
- `dropbox-pp-cli sharing list-received-files`: Return files shared with the current user.
- `dropbox-pp-cli sharing list-shared-links`: List shared links owned by the current user.
- `dropbox-pp-cli sharing modify-shared-link-settings`: Modify the settings of an existing shared link.
- `dropbox-pp-cli sharing revoke-shared-link`: Revoke a shared link.

**users**: Read account and space information.

- `dropbox-pp-cli users get-current-account`: Get information about the current user's account.
- `dropbox-pp-cli users get-space-usage`: Get space usage for the current user's account.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dropbox-pp-cli which "find duplicate files" --json
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match: fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Hand-written Extensions

These hand-written commands ship in the CLI:

- `dropbox-pp-cli index`: Build or refresh the local metadata index.
- `dropbox-pp-cli overview`: Show space by folder, file type, and year.
- `dropbox-pp-cli tree [path]`: Show indexed folder sizes as a tree.
- `dropbox-pp-cli dupes`: Find byte-identical files and optionally write a cleanup plan.
- `dropbox-pp-cli conflicts`: Find conflict copies and optionally write a cleanup plan.
- `dropbox-pp-cli mess`: Find structural clutter and optionally write an empty-folder plan.
- `dropbox-pp-cli organize`: Build a rule-based move plan.
- `dropbox-pp-cli search <query>`: Search indexed names and paths offline.
- `dropbox-pp-cli plan check <file>`: Validate a plan against the index.
- `dropbox-pp-cli apply <file>`: Preview a plan; add `--yes` to execute it and journal results.
- `dropbox-pp-cli undo <batch-id>`: Preview an undo; add `--yes` to execute it.
- `dropbox-pp-cli journal [batch-id]`: List batches or inspect one batch.
- `dropbox-pp-cli links audit`: Audit shared links and optionally write a revoke plan.
- `dropbox-pp-cli files download <path> --output <file|->`: Download to a local file or stdout.
- `dropbox-pp-cli files upload <local-file> <dropbox-path>`: Upload a file up to 150 MiB. Upload sessions are unavailable.

## Safety model

- Run `index` before cleanup. In testing, a few million entries took close to an hour; interrupted runs resume, later runs fetch changes, and the local index can reach a few GB on very large accounts.
- Cleanup finders can write plans, and `plan check` validates them. `apply` and `undo` preview without `--yes`; executing either requires `--yes`.
- Finders skip development folders such as `node_modules`, `.git`, and virtual environments unless `--include-dev-dirs` is set. Applying changes there requires `--allow-dev-dirs`.
- In apply plans, cross-share moves require `--allow-cross-share`; deleting nonempty or shared folders requires `--allow-nonempty-delete` or `--allow-unshare`, respectively.
- `apply` and `undo` refuse to execute under the Printing Press test harness.
- File deletes are soft deletes. `undo` can restore them within Dropbox's restore window, at least 30 days on personal plans.
- Link revokes and `file-requests delete-all-closed` are permanent.

## Recipes


### Find where the space goes

```bash
dropbox-pp-cli overview --agent --select top_folders,by_type,dev_dirs
```

Folder, file type, and code-folder breakdowns from the local index.

### Clean up sync conflict copies

```bash
dropbox-pp-cli conflicts --plan conflicts.json
```

Plans removal only for copies identical to their original or fully contained in it; everything else is listed for review.

### File Camera Uploads by month

```bash
dropbox-pp-cli organize --match '*.jpg' --under '/Camera Uploads' --to '/Photos/{year}/{month}' --plan photos.json
```

Builds a move plan from each file's client_modified date for plan check and apply.

### Review shared links

```bash
dropbox-pp-cli links audit --plan revoke.json --agent
```

Lists every shared link with visibility and expiry and plans revokes for links whose files are gone.

### Undo the last change

```bash
dropbox-pp-cli journal --agent --select batches.id,batches.created_at
```

Find the batch ID, then pass it to undo.

## Auth Setup

Run `dropbox-pp-cli auth setup` for the registration URL and steps (add `--launch` to open the URL). Then authenticate:

```bash
dropbox-pp-cli auth login
```

Tokens are stored locally and refreshed automatically.

Without `--client-id`, `DROPBOX_CLIENT_ID`, or a saved client ID, `auth login` prompts for the app key. `--no-input` and `--agent` disable that prompt.

Run `dropbox-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags apply to generated endpoint and hand-written commands:

- `--json`: one JSON document on stdout
- `--compact`: keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain`: tabular rows (collection envelopes unwrap to the row array)
- `--quiet`: one identity value per row, no envelope

- **Pipeable**: JSON on stdout, errors on stderr
- **Filterable**: `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dropbox-pp-cli overview --agent --select top_folders,by_type
  ```
- **Previewable**: `--dry-run` shows the HTTP request for endpoint commands; run `apply` or `undo` without `--yes` to preview its operations
- **Prompt control**: `--no-input` and `--agent` disable prompts. `auth login` can prompt for a missing client ID in interactive mode
- **Explicit confirmation**: `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries**: use `--idempotent` only when an already-existing create should count as success

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `DROPBOX_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `DROPBOX_CONFIG_DIR`, `DROPBOX_DATA_DIR`, `DROPBOX_STATE_DIR`, `DROPBOX_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `DROPBOX_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains `config.json` and profiles. `data` contains `credentials.toml`, `data.db`, and `feedback.jsonl`. `state` contains local invocation records and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.json` secrets are read for compatibility and leave `config.json` on the first auth write.
- Run `dropbox-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "dropbox": {
        "command": "dropbox-pp-mcp",
        "env": {
          "DROPBOX_HOME": "/srv/dropbox"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `DROPBOX_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `DROPBOX_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data. The shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
dropbox-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "dropbox-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `dropbox-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `dropbox-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities: direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): a row taught under a folder nickname matched the live query's canonical folder through `entity_lookups`. Verify the `resource_id` before fetching it.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a folder nickname resolved to multiple canonical paths (for example, "Archive" could name two folders). Resolve the path before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall: argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
dropbox-pp-cli teach --query "$QUERY" --resource-type "$RESOURCE_TYPE" --resource "$RESOURCE_ID"
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Set `RESOURCE_TYPE` and `RESOURCE_ID` from the resource you verified during discovery. Teach the **most specific** resource: if you walked through parent records to find the answer, teach the leaf ID. When `entity_lookups` contains folder aliases, recall can resolve another alias to the same canonical folder.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
dropbox-pp-cli teach \
  --query "$QUERY" \
  --resource-type "$RESOURCE_TYPE" \
  --resource "$RESOURCE_ID" \
  --playbook-file ./dropbox-playbook.json \
  --playbook-notes-file ./dropbox-playbook-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
dropbox-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ./dropbox-playbook.json \
  --notes-file ./dropbox-playbook-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know: a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback: fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
dropbox-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift, such as a Dropbox response field or category label changing.

What does NOT belong in notes:

- The file-specific answer to the user's question. That's the response, not a learning.
- Per-file or per-folder data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`dropbox-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `DROPBOX_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
dropbox-pp-cli feedback "dupes should explain why a file was chosen as the keeper"
dropbox-pp-cli feedback --stdin < notes.txt
dropbox-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `DROPBOX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DROPBOX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
dropbox-pp-cli profile save briefing --json
dropbox-pp-cli --profile briefing file-requests list
dropbox-pp-cli profile list --json
dropbox-pp-cli profile show briefing
dropbox-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | `apply` or `undo` had an operation fail or end unknown |
| 2 | Usage error or `plan check` found plan errors |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `dropbox-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/dropbox/cmd/dropbox-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dropbox-pp-mcp -- dropbox-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dropbox-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dropbox-pp-cli overview --agent --select top_folders,by_type
   ```
4. If ambiguous, inspect the relevant command's help, such as `dropbox-pp-cli dupes --help`.
