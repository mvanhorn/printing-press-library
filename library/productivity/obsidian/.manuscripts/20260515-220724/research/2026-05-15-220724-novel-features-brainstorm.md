# Obsidian CLI Novel Features Brainstorm — Subagent Output (audit trail)

## Customer model

**Persona 1: Damien-as-Loop-Architect (running UCE three-layer protocol)**

*Today (without this CLI):* Damien opens Obsidian, navigates the file tree, and types frontmatter by hand. He also wires Claude/Codex agents to read and write into `~/Documents/Dev/UCE`. When an agent writes a meeting note, the frontmatter drifts: `date: 2026-5-15` instead of ISO, `type: meeting-notes` instead of the enum value `meeting`, no `description`. He discovers the drift days later when the cm extraction pipeline produces low-confidence entities and he has to walk back through dozens of files diffing against `frontmatter_parser.py`. He keeps `frontmatter_parser.py` open in a second window as a reference. He cannot answer: "which notes in `People/` are missing required fields right now?" or "show me every fact about [[Jeff Smith]] across all notes."

*Weekly ritual:* Every Build Session Friday, he triages the week's meeting notes, journal entries, and fact additions. Touches roughly 30-60 notes/week. Hands the vault to cm via Tuck for downstream extraction.

*Frustration:* The vault drifts silently. He cannot trust agent-written notes, and he cannot trust the cm pipeline output because the upstream `.md` is the failure point. The protocol exists only as Python in a sibling repo, not as a guardrail at write time.

**Persona 2: Claude/Codex agent operating on the vault**

*Today (without this CLI):* The agent does `cat People/Jeff\ Smith.md`, reads the full note (3KB of YAML + body + facts) to answer "what's Jeff's company?" — burns context. To add a fact it reads the file, edits the YAML, writes it back, hopes it parses. To find a note it does `grep -r "buttermilk" Vault/` and waits seconds. To check if a date field is valid, it pattern-matches a regex. Has no idea whether the write will pass the UCE validator until cm runs hours later and silently drops the entity.

*Weekly ritual:* Dozens of vault interactions per session — fact captures, note creation from meeting transcripts, lookups during nurture refresh.

*Frustration:* Every read is full-fat. Every write is gamble-validated. The agent burns 5-10x the tokens it should and produces silently broken frontmatter.

**Persona 3: cm-extraction-lambda (downstream consumer via Tuck)**

*Today (without this CLI):* Receives `.md` files via Tuck filesystem sync with `SourceType::ObsidianImport`. Tries to extract entities, events, facts. Half the notes are missing `description`, a third have non-ISO dates, some have `type: meeting-notes` instead of `meeting`. Confidence scores degrade. No way to know which notes were written by trusted agents (protocol-enforced) vs hand-edited (suspect).

*Weekly ritual:* Continuous extraction triggered by Tuck file events.

*Frustration:* Extraction quality is bounded by upstream frontmatter quality, and upstream has no enforcement. Cannot distinguish a "clean" note from a drifted one without re-running the validator.

## Candidates (pre-cut)

(Full list per subagent output — 16 candidates labeled C1–C16. See subagent return for full table.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Protocol lint (with severity tiers + non-zero-exit mode) | `obsidian lint [--severity error\|warn\|info] [--rule <id>] [--exit-nonzero-on error]` | 10/10 | Walks every `.md` row in local SQLite, applies the rule set ported from UCE's `frontmatter_parser.py`, reports findings grouped by severity; exit code 2 when error-tier findings are present | UCE/src/vault/frontmatter_parser.py:45-191 |
| 2 | Auto-migrate common violations | `obsidian migrate [--rule date-iso\|type-enum\|missing-description] [--dry-run]` | 9/10 | Reads `validation_findings` from local store, applies mechanical fixers for the bounded set | Persona Damien's "vault drifts silently" |
| 3 | Three-layer dashboard | `obsidian layers stats` | 8/10 | Aggregates `notes.type` against the layer map (KG vs Events vs Patterns) | Brief User Vision; persona Damien's Friday triage |
| 4 | Fact graduation candidates | `obsidian facts graduation-candidates [--threshold 20]` | 8/10 | SQL on local facts table joining count vs threshold | Brief Top Workflow #4 |
| 5 | Decision-trace tracker | `obsidian facts decision-trace <trace-id>` | 8/10 | SQL on decision_trace_id across inline + TOML facts | Brief Build Priority #7 |
| 6 | Entity dossier (agent-shaped) | `obsidian entity dossier "[[Entity]]" [--layer description]` | 9/10 | Joins notes + frontmatter + facts + links + tags for one entity | Persona Agent's "every read is full-fat" |
| 7 | cm extraction-readiness audit | `obsidian readiness [--source-tag <tag>] [--since <date>]` | 8/10 | Filters validation_findings to cm-blocking rules | Brief Codebase Intelligence (cm SourceType::ObsidianImport) |
| 8 | Stale notes by type | `obsidian stale [--type meeting] [--older-than 90d]` | 7/10 | SQL on notes.mtime + type | Persona Damien's un-promoted journals |
| 9 | Provenance audit | `obsidian provenance <note-path>` | 7/10 | Walks source field on note + facts; prints chain | Brief data layer lists source + decision_trace_id |
| 10 | Daily note append (protocol-enforced) | `obsidian daily append "<text>" [--section "## Notes"]` | 7/10 | Resolves today's path from periodic-note settings, creates from template if missing, appends under section | Persona Agent's transcript-ingest |

### Killed candidates

| Candidate | Kill reason |
|-----------|-------------|
| C2 (separate `--exit-nonzero-on` command) | Merged into C1 as a mode flag |
| C9 (protocol-enforced fact add) | Already absorb #31 |
| C10 (`context list --layer description`) | Already absorb #33 / brief Top Workflow #5 |
| C12 (link centrality / PageRank) | Monthly-at-best use; C4 covers vault-heaviness better |
| C14 (`sync --rehash`) | One option flag on existing absorbed `sync`, not a feature |
| C15 (Python validator parity shell-out) | Fails External Service / Auth-gap check; better as repo CI tool |
