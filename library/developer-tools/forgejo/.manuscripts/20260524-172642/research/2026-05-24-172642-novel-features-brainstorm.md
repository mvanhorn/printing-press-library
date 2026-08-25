## Customer model

### Persona 1: Devika — Codeberg OSS maintainer

**Today (without this CLI):** Devika maintains two OSS libraries on Codeberg. She uses the Forgejo web UI for issue triage, the `tea` CLI for quick PR checks, and wget + curl scripts to download release assets from CI. `tea` hasn't had a release in 18 months, breaks on Codeberg's newer API responses, and has no OAuth2 flow — she keeps her token in a `.env` file she pastes manually every few months when it expires.

**Weekly ritual:** Every Monday she scans her issue inbox by label (bug, help-wanted), closes stale issues, reviews open PRs with at least one approval, and cuts a patch release when the diff is clean. She does this across two repos on Codeberg.

**Frustration:** Switching between the web UI and the broken `tea` CLI to do something as simple as listing PRs awaiting review. She's typed `tea pr ls` only to see an auth error, re-copied her token, and landed back on the web UI. The merge-and-tag-and-release sequence requires four browser tabs.

### Persona 2: Barend — Self-hosted Forgejo admin at a 30-person fintech

**Today (without this CLI):** Barend runs a Forgejo instance for internal dev tooling. He registers Actions runners manually via the web UI, which means logging in, navigating to org settings, copying a token, SSH-ing to the runner host, and pasting. He also audits user accounts monthly by exporting data from the Forgejo admin panel as CSV — no CLI automation exists for this.

**Weekly ritual:** Monday morning: check runner health across three orgs. Friday afternoon: verify that the weekly release of the internal SDK got its assets uploaded correctly. He does both by clicking through the web UI.

**Frustration:** Runner registration is a five-step manual process. When a runner goes down, he has no fast way to see runner status across all three orgs without navigating to three separate web UI pages. There is no `watch` command, no scriptable runner state output.

### Persona 3: Priya — DevOps engineer at a startup on Forgejo Cloud / Codeberg

**Today (without this CLI):** Priya owns the release pipeline. She scripts releases with `curl` hitting the Forgejo API directly — wrapped in bash with token from `$FORGEJO_TOKEN`. The scripts work but are fragile (no pagination, no retry, no human-readable feedback). She'd use `gh` if she were on GitHub, but the team chose Forgejo for data sovereignty reasons.

**Weekly ritual:** On every sprint boundary (~every 2 weeks) she: creates a release tag, uploads 4-6 binary assets, posts the changelog link to Slack. Between releases she monitors CI runners and occasionally reroutes issues to the right milestone.

**Frustration:** Asset upload is the most painful step — her curl script silently fails on large files and has no progress output. She's lost two release uploads to network hiccups and only discovered the failure when the end user reported missing binaries.

### Persona 4: Marcus — Open source contributor using multiple Forgejo instances

**Today (without this CLI):** Marcus contributes to projects on Codeberg, on a university Forgejo instance, and on a company self-hosted instance. He uses three different browser profiles, three separate `tea` config files (renamed manually), and context-switches constantly. He frequently forgets which token is for which host.

**Weekly ritual:** Skim notifications from all three instances, pick up one or two issues to fix, open a PR, wait for CI, merge. All done through the web UI because no CLI handles multi-host gracefully.

**Frustration:** Forgejo notifications from three instances exist in three disconnected inboxes. He has no unified view — he has to open three browser tabs just to check whether he has anything urgent.

---

## Candidates (pre-cut)

### C1 — Multi-repo triage dashboard (SQLite cross-join)
**Command:** `fj issue dashboard [--label <l>] [--assignee @me] [--host <profile>] [--all-hosts]`
**Source:** (c) cross-entity local query + (a) Devika frustration

Syncs issues from all watched repos into SQLite, then renders a terminal table sorted by updated_at — cross-repo, optionally cross-host.

### C2 — Runner health sweep (cross-org, scripted output)
**Command:** `fj runner sweep [--org <o>] [--all-orgs] [--json] [--watch]`
**Source:** (a) Barend frustration + (b) Forgejo runner API

### C3 — Release upload with progress + retry
**Command:** `fj release upload <tag> <files...> [--repo <r>] [--retry <n>] [--progress]`
**Source:** (a) Priya frustration

### C4 — Unified notification inbox (cross-host SQLite)
**Command:** `fj notification inbox [--all-hosts] [--unread] [--since <duration>] [--json]`
**Source:** (c) cross-host join + (a) Marcus frustration

### C5 — PR review queue
**Command:** `fj pr queue [--repo <r>] [--all-repos] [--assignee @me] [--review-requested @me]`
**Source:** (a) Devika weekly ritual + (c) local query

### C6 — Federation actor inspect (killed)
**Command:** `fj activitypub actor <owner>[/repo]`
**Source:** (b) Forgejo-specific federation
**Kill:** User Pain = 1/3. Debug command not a weekly workflow.

### C7 — Admin user audit export (killed)
**Command:** `fj admin users export`
**Source:** (a) Barend monthly audit
**Kill:** Monthly cadence, score 4/10.

### C8 — Release changelog generation
**Command:** `fj release changelog <from-tag> <to-tag> [--format md|json] [--section bug,feature,break]`
**Source:** (a) Priya's release pipeline + (c) cross-entity join

### C9 — Multi-host repo search (killed)
**Command:** `fj repo search <query> [--all-hosts]`
**Source:** (a) Marcus multi-instance
**Kill:** Thin parallel HTTP loop, no SQLite join, score 4/10.

### C10 — Issue dependency graph (killed)
**Command:** `fj issue deps <issue-number>`
**Source:** (c) local graph query
**Kill:** Depends on freeform body parsing, fragile, speculative.

### C11 — Runner registration wizard (killed)
Already absorbed in manifest #44.

### C12 — Webhook event log (killed)
**Kill:** Narrow sub-audience, score 4/10.

### C13 — Stale issue sweeper
**Command:** `fj issue sweep --stale-after <duration> [--label stale] [--dry-run] [--comment <text>] [--close]`
**Source:** (a) Devika triage + agent pattern

### C14 — CI check status summary
**Command:** `fj ci status [--branch <b>] [--all-repos] [--watch] [--json]`
**Source:** (a) Priya monitors CI + (b) Forgejo Actions

### C15 — Package version diff (killed)
**Kill:** Thin date-filter wrapper, no transcendence.

### C16 — SSH key / deploy key audit (killed)
**Kill:** Monthly cadence, narrow audience, score 3/10.

---

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|-------------|--------------|----------|
| C1 | Multi-repo triage dashboard | `fj issue dashboard [--all-hosts] [--assignee @me] [--label <l>]` | 9/10 | hand-code | Reads from SQLite (populated by `fj sync`); joins repos × issues × labels; renders sorted terminal table cross-repo, optionally cross-host. Single API call cannot produce this view. | Devika's Monday ritual; brief §Local store value; no competitor does cross-repo issue aggregation |
| C4 | Unified notification inbox (cross-host) | `fj notification inbox [--all-hosts] [--unread] [--since <duration>] [--json]` | 8/10 | hand-code | Syncs from all configured host profiles into SQLite keyed by `(host, notification_id)`; renders unified inbox with host column; `fj notification count --all-hosts` returns summed integer for prompt badge. | Marcus's core frustration; absorb manifest covers single-host only; multi-host aggregation unaddressed |
| C2 | Runner health sweep (cross-org) | `fj runner sweep [--all-orgs] [--watch] [--json]` | 8/10 | hand-code | Iterates org/repo/user runner scopes in parallel, joins results, flags offline runners with last-seen timestamp; `--watch` polls every N seconds. | Barend's Friday sweep; no competitor CLI has runner commands at all |
| C8 | Release changelog generation | `fj release changelog <from-tag> <to-tag> [--format md\|json] [--section bug,feature,break]` | 8/10 | hand-code | Resolves tag timestamps; queries closed issues and merged PRs in that window; groups by label into sections. Three-entity join: tags × issues × PRs. | Priya posts changelog every sprint; no competitor does this |
| C3 | Release upload with progress + retry | `fj release upload <tag> <files...> [--retry <n>] [--progress]` | 7/10 | hand-code | Wraps generated upload endpoint with chunked transfer, per-file progress bar, retry-with-backoff; verifies asset size post-upload. | Priya lost two release uploads to silent curl failures |
| C13 | Stale issue sweeper | `fj issue sweep --stale-after <duration> [--label stale] [--dry-run] [--comment <text>] [--close]` | 7/10 | hand-code | Pre-filters via SQLite; fetches fresh state for candidates; presents confirmation list; applies label, posts comment, closes. Agent-shaped read→confirm→write loop. | Devika's most tedious Monday task; stale bot is a GitHub-ecosystem pattern |
| C5 | PR review queue | `fj pr queue [--all-repos] [--review-requested @me] [--json]` | 7/10 | hand-code | Queries synced PR table for PRs where auth user has pending review request; joins with check-run status and approval counts; renders age-sorted table. | Devika's Monday PR scan; absorb manifest `pr list` is per-repo and unfiltered |
| C14 | CI check status summary | `fj ci status [--branch <b>] [--all-repos] [--watch] [--json]` | 6/10 | hand-code | For each watched repo: fetches latest commit on branch, then check runs; joins branch → commit → check_run; renders pass/fail/pending per workflow; `--watch` tails at 10s. | Priya checks CI before cutting releases; no competitor surfaces check-run status cross-repo |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|--------------------------|
| C6 — Federation actor inspect | User Pain = 1/3. Debug command not a weekly workflow. Narrow audience on federation-enabled instances. | None |
| C7 — Admin user audit export | Monthly cadence. Score 4/10. | C2 (runner sweep serves admin-monitoring persona) |
| C9 — Multi-host repo search | Thin parallel HTTP loop, no SQLite join, no real transcendence. Score 4/10. | C4 (cross-host aggregation in notification inbox) |
| C10 — Issue dependency graph | Depends on freeform body-text conventions, not spec-native. Fragile parser. Score 3/10. | C1 (issue dashboard) |
| C11 — Runner registration wizard | Already absorbed in manifest #44. Implementation detail. | C2 (runner sweep) |
| C12 — Webhook event log | Narrow sub-audience. Score 4/10. | C14 (CI status) |
| C15 — Package version diff | Thin date-filter wrapper. No cross-entity join. Score 3/10. | C8 (changelog generation) |
| C16 — SSH key / deploy key audit | Monthly cadence. Score 3/10. | C2 (admin persona) |
