# ht-ml.app — Novel Features Brainstorm (audit trail)

Subagent: general-purpose, 3-pass (customer model → candidates → adversarial cut). First print (no prior research).

## Customer model

**Cody, the in-session coding agent** (Claude Code in a worktree, Codex via `codex exec`). Publishes prototypes, diagrams, code reviews, decks, illustrations.
- Today: runs the 3-step dance by hand; the `update_key` lands in the transcript and nowhere else; when the worktree is torn down it evaporates.
- Weekly ritual: spins up dozens of one-off sites a week; each created fresh because there is no list endpoint.
- Frustration: can publish but never update its own past work (key gone → every tweak = new orphaned site); routinely ships broken `<img>` because nothing reconciles HTML refs vs uploaded assets (403 asset-not-referenced).

**Hermes, the always-on home-server agent** (Mac mini; recurring status reports, triage boards, dashboards).
- Today: wants a stable bookmarkable URL but never persisted yesterday's `update_key`, so every scheduled run creates a NEW site and the URL churns.
- Weekly ritual: daily regenerate-and-republish; maintains a handful of long-lived "living document" sites.
- Frustration: the accountless API gives a living document no stable identity; can only POST a new one; last week's site is orphaned.

**Bobe, the operator the agents act for** (agency; sends client-facing URLs; two-Mac / here-now adjacency).
- Today: receives URLs in chat, pastes to clients; when a client says "change one number" the agent cannot (key gone); no dashboard/account/list.
- Weekly ritual: reviews everything published this week across all agents; finds "that status report from Tuesday"; confirms no client site has broken images; wants recovery assurance.
- Frustration: everything is public+crawlable yet he has zero inventory, cannot search history, cannot prove a site is his, knows one lost key freezes a client page forever.

## Survivors (transcendence set, all hand-code, score >= 7)

| # | Feature | Command | Persona | Score | Build | Why only we can do it | Long Description |
|---|---------|---------|---------|-------|-------|-----------------------|-----------------|
| 1 | Site registry inventory | `list [--orphaned] [--missing-assets] [--sort age\|versions]` | Bobe, Cody | 10 | hand-code | Local `sites` table is the only inventory possible — the API has no list endpoint | none |
| 2 | Key vault & recovery | `keys show <id> --reveal` / `keys export` / `keys import` | Bobe, Hermes | 10 | hand-code | Local store is the sole holder of the once-only, unrecoverable write secret | none |
| 3 | One-shot asset reconcile | `assets sync <id> [--root <dir>]` | Cody, Hermes | 10 | hand-code | Parses HTML refs, diffs API missing-list, uploads referenced-but-missing files in one pass (honors referenced-first 403 rule) | Use to upload every referenced-but-missing asset for ONE site; to find WHICH sites have broken assets, use `assets audit` |
| 4 | Version rollback | `rollback <id> [<version>]` | Hermes, Cody | 9 | hand-code | Local `versions` table + auto key-resolve; API has no version concept | none |
| 5 | Broken-asset audit | `assets audit [--missing-only]` | Bobe, Cody | 9 | hand-code | Cross-entity `sites`⋈`assets` join across all sites; no single GET can answer it | Use to find broken/missing assets across ALL sites (read-only); to upload missing files for a site, use `assets sync <site_id>` |
| 6 | Living-doc republish by alias | `republish --as <name> <file\|->` | Hermes | 8 | hand-code | Local alias map + upsert (PUT in place or create+bind) gives the accountless API stable identity | none |
| 7 | Pre-publish secret/PII scan | `scan <file\|-> [--rules secrets,pii]` | Bobe, Cody | 7 | hand-code | Mechanical regex guard before content becomes public+permanent; API content-scans for safety but not secret leakage | none |

## Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| Version diff (`diff`) | Comfort inspector, not weekly; folds into bare `rollback`. | rollback |
| Version history list (`history`) | Thin lister; bare `rollback <id>` already lists versions; `list` shows counts. | rollback |
| Clone a site (`clone`) | Occasional (fork own past site), not weekly; new-site creation already served. | republish |
| Bulk publish (`bulk publish`) | Occasional batch; source (e) explicitly light; publish/republish cover it. | republish |
| Open in browser (`open`) | Thin wrapper over stored URL + OS open; no SQLite leverage/join/agent output. | list |
| Export site to disk (`export`) | Redundant with keys vault for recovery; HTML already in SQLite. | keys |
| Stale/drift report (`stale`) | Speculative (live HTML changes only via our PUTs); overlaps framework sync/doctor. | assets audit |
