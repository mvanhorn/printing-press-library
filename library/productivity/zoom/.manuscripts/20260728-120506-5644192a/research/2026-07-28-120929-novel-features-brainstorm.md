# Zoom Reprint — Novel Features Brainstorm (2026-07-28)

Subagent output (customer model, candidates, survivors/kills, reprint verdicts). Reprint of run 20260519-094503 under Printing Press 4.29.0.

## Customer model

**Maya — the back-to-back meeting knowledge worker (product manager, macOS).**
Today: 6-8 Zoom calls a day; joining means invite hunting + browser interstitial. Past-call content split between `~/Documents/Zoom/` and cloud.
Weekly ritual: Monday reconstruction of "what did I commit to last week" from memory and 2x recording scrubbing.
Frustration: recovering one quote takes 20 minutes of video scrubbing; unknown whether recording is local or cloud.

**Devon — the macOS power user / hotkey operator.**
Today: Stream Deck + fragile AppleScript gists; bookmarks are a text file of hand-parsed URLs.
Weekly ritual: joins the same 4 recurring meetings dozens of times; hand-dissects every new invite URL.
Frustration: "save this Zoom link to join by name later" is manual URL surgery; no "join my next meeting" verb.

**Priya — the meeting-notes archivist (chief of staff).**
Today: relies on AI Companion summaries and the My Notes portal (no public API); hand-builds to-do lists from Notes exports.
Weekly ritual: exports Notes PDFs, transcribes action items into a personal backlog.
Frustration: notes corpus locked in a web UI — not greppable, not joinable, action items re-typed weekly.

**Sam — the recording-archive analyst (ops/IT).**
Today: `~/Documents/Zoom/` is the biggest directory on the laptop; audits with `du -sh` + Zoom web page side by side.
Weekly ritual: disk cleanup, guessing which local recordings also exist in cloud.
Frustration: no tool sees local disk and cloud recordings at once; every deletion risks losing the only copy.

## Candidates (pre-cut)

| # | Name | Command | Persona | Source | Verdict |
|---|------|---------|---------|--------|---------|
| C1 | Find a quote across transcripts + notes | `find "<q>" --source local\|cloud\|notes\|both --speaker <n>` | Maya | (d) prior-reframe (absorbs `notes search`) | Keep |
| C2 | Storage audit | `storage --by month --also-in-cloud` | Sam | (d) prior-keep | Keep |
| C3 | Recording drift detector | `recordings drift` | Sam | (d) prior | Soft kill — overlaps `storage --also-in-cloud`, occasional-use |
| C4 | Today's load + conflicts | `today --with-recordings` | Maya | (d) prior-keep | Keep |
| C5 | Bookmark from URL paste | `saved add-from-url <name> "<url>"` | Devon | (d) prior-keep | Keep |
| C6 | Schedule + bookmark | `schedule "<topic>" --when <iso> --save-as <n>` | Devon | (d) prior-keep | Keep |
| C7 | Speaker-time analytics | `recordings analyze <id>` | Sam, Maya | (d) prior-keep | Keep |
| C8 | Export recording bundle | `recordings export <id>` | Maya | (d) prior | Soft kill — monthly-use, covered by cloud download + find |
| C9 | Notes web opener | `notes web` | Priya | (d) prior | Kill — thin browser-open wrapper |
| C10 | AI Companion summary | `notes summary <uuid>` | Priya | (d) prior-keep | Keep (`// pp:client-call`, spec-gap endpoint) |
| C11 | AI Companion transcript | `notes transcript <uuid>` | Priya | (d) prior | Kill — thin twin of summary; verbatim served by cloud download --type transcript |
| C12 | Ingest Notes PDF/DOCX | `notes ingest <file>` | Priya | (d) prior-keep | Keep |
| C13 | Extract notes to-dos | `notes todos --since 7d` | Priya | (d) prior-keep | Keep |
| C14 | Notes download via Chrome auth | `notes download [--meeting <id>]` | Priya | (d) prior-keep (amend carry-forward) | Keep |
| C15 | Join next meeting | `next [--dry-run]` | Maya, Devon | (a) persona-driven | Keep — NEW |
| C16 | Chat links extractor | `recordings links` | Maya | (b) | Kill — covered by find |
| C17 | Recurring-meeting history | `recurring history <id>` | Priya | (c) | Kill — composable from today/find/sql |
| C18 | Weekly meeting-load stats | — | Maya | (c) | Kill — framework `analytics --type meetings --group-by topic` |

## Survivors and kills

### Survivors (11 — reprint keep-rule: 9 prior features re-scored ≥7/10)

| # | Feature | Command | Score | Buildability | Persona | Long Description |
|---|---------|---------|-------|--------------|---------|------------------|
| 1 | Find a quote everywhere | `find "<q>" --source both\|notes --speaker <n>` | 10/10 | hand-code | Maya | Use this command to search transcript and notes text. Do NOT use it to list recording files; use 'recordings local list' or `search "<term>" --type local_recordings --limit 10`. |
| 2 | Storage audit | `storage --by month --also-in-cloud` | 8/10 | hand-code | Sam | none |
| 3 | Today + conflicts | `today --with-recordings` | 8/10 | hand-code | Maya | Use this command for a one-screen today view with conflicts. Do NOT use it to enumerate future meetings; use 'meetings list --type upcoming'. |
| 4 | Bookmark from URL paste | `saved add-from-url <name> "<url>"` | 8/10 | hand-code | Devon | none |
| 5 | Schedule + bookmark | `schedule "<topic>" --when <iso> --save-as <n>` | 7/10 | hand-code | Devon | none |
| 6 | Speaker-time analytics | `recordings analyze <id>` | 8/10 | hand-code | Sam, Maya | Use this command for per-speaker talk-time math on one recording. Do NOT use it to search what was said; use 'find'. |
| 7 | Join next meeting | `next [--dry-run]` | 9/10 | hand-code | Maya, Devon | Use this command to join the next scheduled meeting. Do NOT use it for a known ID/URL ('join') or a bookmark ('saved join <name>'). |
| 8 | AI Companion summary | `notes summary <uuid>` | 7/10 | hand-code | Priya | Use this command for the AI Companion summary of one meeting. Do NOT use it to search across notes; use 'find --source notes'. |
| 9 | Ingest Notes export | `notes ingest <file>` | 9/10 | hand-code | Priya | none |
| 10 | Extract notes to-dos | `notes todos --since 7d` | 9/10 | hand-code | Priya | Use this command to extract action-item lines from ingested notes. Do NOT use it for free-text search; use 'find --source notes'. |
| 11 | Notes download (browser auth) | `notes download [--meeting <id>]` | 8/10 | hand-code | Priya | Use this command to pull Notes documents down from zoom.us. Do NOT use it to read already-downloaded files; use 'notes ingest' then 'find --source notes'. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `recordings drift` | Occasional-use; overlaps `storage --also-in-cloud` | `storage` |
| `recordings export` | Monthly-use; covered by absorbed `recordings cloud download` + `find` | `find` |
| `notes web` | Thin browser-open wrapper, no data leverage | `notes download` |
| `notes transcript` | Thin twin of `notes summary`; verbatim need served by cloud download | `notes summary` |
| `recordings links` | Occasional-use; `find` covers chat retrieval | `find` |
| `recurring history` | Composable from `today`/`find`/framework `sql` | `today` |
| Weekly meeting-load stats | Framework `analytics --type meetings --group-by topic` | (framework) |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| `find` | keep | Core retrieval; 10/10; gains `--source notes`. |
| `storage` | keep | Weekly cleanup; only disk+cloud join; 8/10. |
| `recordings drift` | drop | Occasional-use; overlaps `storage --also-in-cloud`. |
| `today` | keep | Daily three-source compose; 8/10. |
| `saved add-from-url` | keep | Every-invite command; URL grammar; 8/10. |
| `schedule` | keep | Cloud write + local bookmark round-trip; 7/10. |
| `recordings analyze` | keep | LLM-free talk-time math; 8/10. |
| `recordings export` | drop | Monthly-use; covered by download + find. |
| `notes web` | drop | Superseded by carried-forward `notes download`. |
| `notes summary` | keep | Documented spec-gap endpoint; weekly recap; 7/10. |
| `notes transcript` | drop | Thin twin of summary. |
| `notes ingest` | keep | Only on-disk My Notes path; 9/10. |
| `notes search` | reframe | Folded into `find --source notes`. |
| `notes todos` | keep | User's explicit weekly ritual; 9/10. |
| `notes download` (amend) | keep | Post-publish hand-amend = strongest demand signal; press-auth cookie path; 8/10. |
