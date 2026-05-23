---
name: servosity-pp-cli
type: printed-cli
status: active
description: Every Servosity endpoint as a typed CLI command, plus a local fleet mirror, snapshot history, and cross-engine rollups the web UI doesn't have.
binary_name: servosity-pp-cli
binary_install_path: ./cmd/servosity-pp-cli
auth_env_var: SERVOSITY_API_TOKEN
auth_help_url: https://cp.servosity.com/user/
trigger_phrases:
  - "what needs my attention on servosity"
  - "fleet stale backups"
  - "show me company in servosity"
  - "triage servosity issues"
  - "drift since yesterday on servosity"
  - "use servosity-pp-cli"
  - "run servosity"
related_skills: []
printed_from: 2026-05-12 via Printing Press v4.4.0 (run 20260511-211606)
printed_by: dstevens
---

# Servosity CLI

The Tier-One terminal for Servosity's backup/DR fleet. Wraps the full REST surface (Resellers, Companies, three backup engines, Issues, Reports, Admin) as typed Cobra commands with `--json` / `--select` / `--csv` / `--dry-run` / typed exit codes, and adds a local SQLite mirror that makes the fleet queryable offline. Ships with 10 novel commands that aren't in the web UI: `attention` (fleet rollup), `drift` (day-over-day diff), `stale-backups` (offline query against `/reports/stale-backup-sets/`), `backup-facts` (cross-engine view), `find` (cross-table FTS5), `company show` (per-company snapshot), `restore-queue list --watch` (DRaaS oversight), `triage` (terminal-speed issue triage), `clear` (clear-company / clear-partner), and `stale-issues` (per-engineer stale-issue cleanup).

**Production safety:** every mutating command (`triage`, `clear`, `stale-issues`) defaults to PLAN mode. `--confirm` is required to actually call the live API; the global `--dry-run` flag is an extra safety net that overrides `--confirm`.

## When to use

- Morning fleet sweep: "what needs my attention right now?" across all admin rollups.
- Friday stale-backup hunt: who hasn't had a successful job in N days, sliced by reseller/engine.
- Customer-asks-"is my backup OK?": one command pulls every relevant fact for a company.
- Support-team issue triage: batch ignore / archive / reactivate / comment without 5+ UI clicks per issue.
- DRaaS-in-flight oversight: live-watch every restore queue across the fleet.
- Day-over-day fleet trend: `drift` against the snapshots the CLI itself captures.

## Install

Run the catalog installer (handles symlink + `go install` + Keychain + `doctor`):

```bash
bash ~/Documents/Dev/cc-skills/scripts/install.sh servosity-pp-cli
```

The installer will:
1. Symlink `~/.claude/skills/servosity-pp-cli` → `~/Documents/Dev/cc-skills/servosity-pp-cli/` so Claude Code discovers the SKILL.md.
2. Run `go install ./cmd/servosity-pp-cli` so `servosity-pp-cli` is on your `$PATH`.
3. Check whether `SERVOSITY_API_TOKEN` is set; if not, offer to store it in the OS keychain (macOS Keychain on Mac, libsecret on Linux, manual instructions on Windows).
4. Run `servosity-pp-cli doctor` to verify auth + reachability.

Get a token at [cp.servosity.com/user/](https://cp.servosity.com/user/) (button at the bottom of the page).

If you prefer to install by hand:

```bash
ln -sf ~/Documents/Dev/cc-skills/servosity-pp-cli ~/.claude/skills/servosity-pp-cli
cd ~/Documents/Dev/cc-skills/servosity-pp-cli && go install ./cmd/servosity-pp-cli
security add-generic-password -a "$USER" -s SERVOSITY_API_TOKEN -U -w   # macOS only; paste token at prompt
echo 'export SERVOSITY_API_TOKEN="$(security find-generic-password -a "$USER" -s SERVOSITY_API_TOKEN -w 2>/dev/null)"' >> ~/.zshrc
source ~/.zshrc
servosity-pp-cli doctor
```

## Invoke

```bash
servosity-pp-cli doctor                                           # health check
servosity-pp-cli attention --json                                 # morning fleet rollup
servosity-pp-cli stale-backups --refresh --days 7 --timeout 180s  # Friday stale hunt (first time pulls /reports/stale-backup-sets/ — slow on big fleets)
servosity-pp-cli company show 5914 --json                         # per-company snapshot across all 3 engines
servosity-pp-cli triage --company 5914 --json                     # see open issues (PLAN mode by default)
servosity-pp-cli clear "ACME Corp" --until "6am tomorrow"         # PLAN mode; add --confirm to mutate
servosity-pp-cli find "image manager" --in issues,backups --json  # FTS5 across the synced store
```

## Trigger phrases

- "what needs my attention on servosity"
- "fleet stale backups"
- "show me company in servosity"
- "triage servosity issues"
- "drift since yesterday on servosity"
- "use servosity-pp-cli" / "run servosity"

## Related

None yet. (When `hubspot-pp-cli`, `granola-pp-cli`, or other printed CLIs land in this catalog, link the related ones here — e.g., HubSpot for partner outreach context, Granola for meeting follow-ups touching the same companies.)
