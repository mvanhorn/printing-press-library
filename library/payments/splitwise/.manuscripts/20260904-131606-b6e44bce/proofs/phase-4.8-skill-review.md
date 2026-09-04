# Phase 4.8 — SKILL.md Review (splitwise-pp-cli reprint, 2026-09-04)

Ground truth: rebuilt binary via `GOTOOLCHAIN=go1.26.6 go build -o ./splitwise-pp-cli ./cmd/splitwise-pp-cli`
(succeeded, no errors) and ran `--help` recursively (top-level + `auth`, `settle-up`, `fairness`,
`fairness nudge`, `split`, `which`, `balances`, `debts`, `net`, `ledger`, `audit`, `spend`, `report`,
`recurring`, `forecast`, `normalize`, `brief`, `activity`, `reconcile`, `doctor`, `auth setup/set-token/status`)
against `working/splitwise-pp-cli/`. Read commands probed with `--data-source local --db /tmp/skillreview.db`
against an empty store; no sync/--record/--send/live-DB calls made. Compared against the absorb manifest
(`research/2026-09-03-215539-feat-splitwise-pp-cli-absorb-manifest.md`) and `research.json`
(`novel_features` vs `novel_features_built`, `auth`, `narrative`).

## Findings

1. **Trigger phrases match capabilities** — PASS. Frontmatter trigger phrases ("what do I owe on
   splitwise", "who owes me money", "split this expense", "settle up the trip", "how much did we spend
   on food") map to `debts`/`balances`, `debts`, `split`, `settle-up`, `spend` — all verified real
   commands via `--help`.

2. **Verified-set alignment** — PASS. `novel_features` and `novel_features_built` in research.json are
   identical (18 entries, no drift). SKILL's "Unique Capabilities" section lists exactly the same 18
   command/name pairs (`balances` appears twice — net view + `--by-group` mode — matching the two
   "Balances"/"Balances by group" rows in `novel_features_built`). No extra, missing, or renamed
   commands.

3. **Novel-feature descriptions match `<cmd> --help`** — PASS. Spot-checked `balances`, `debts`, `net`,
   `ledger`, `settle-up`, `audit`, `split`, `fairness`/`fairness nudge`, `spend`, `report`, `recurring`,
   `forecast`, `normalize`, `brief`, `activity`, `reconcile`: each command's real `--help` long
   description (the "Use this command for X. Do NOT use it for Y; use 'Z'" redirect pattern) matches the
   SKILL bullet's description/why-it-matters framing and the cross-command redirects in the absorb
   manifest's "Transcendence" table.

4. **Stub/gated disclosure** — WARNING, SKILL.md:88, SKILL.md:109. The `settle-up` bullet ("optionally
   record the payments" / "previewed before anything is recorded") and the `fairness nudge` bullet
   ("previewed before it sends") do not state, at the point of use, that `--record` / `--send` perform
   **real writes to the user's live Splitwise account** (a payment expense other group members will see;
   a comment posted to a friend's expense thread). The only place this is spelled out is the unrelated
   Anti-triggers line ("do not use to pay anyone — it records settlements in Splitwise only", line 43),
   which an agent skimming only "Unique Capabilities" would not connect to these two bullets. Fix: add a
   one-clause disclosure inline, e.g. "`--record` creates real payment expenses visible to the group" and
   "`--send` posts a real comment visible to the friend."

5. **Auth narrative accuracy** — PASS. All four `auth` subcommands mentioned/implied (`logout`,
   `set-token`, `setup`, `status`) exist and match api-key/Bearer auth via `SPLITWISE_API_KEY` (verified
   `--help` on each). The OAuth 2.0 authorization-code claim is backed by real code (`AuthSource =
   "oauth2"`, `AccessToken`/`ClientID`/`ClientSecret` fields in `internal/config/config.go`) — SKILL does
   not overclaim a CLI-driven OAuth flow (none exists as a subcommand), it only states OAuth is
   "supported," which is accurate.

6. **Recipe output claims match shape/intent** — PASS. Ran `balances --agent`, `brief --agent --compact`,
   `fairness --agent`, `spend --agent`, and `which "who owes me money"` / `which "send message"` against
   an empty local store: outputs matched the documented `{"meta":{"source":...},"results":...}` envelope,
   the `net`/`stalest_debts`/`recent_changes` shape claimed for `brief`, and `which`'s exit-2 /
   `{"matches":[]}` contract for a no-match query.

7. **Marketing-copy smell** — PASS. Grepped SKILL.md for hype terms (best-in-class, revolutionary,
   seamless, effortless, powerful, robust, cutting-edge, world-class, unparalleled, blazing,
   state-of-the-art) — no hits. "No other Splitwise tool has" / "no competing tool" style claims are
   substantiated per-feature in the absorb manifest's Absorbed/Transcendence tables, not bare assertion.

## Summary

6 PASS, 1 WARNING (finding #4 — settle-up/fairness-nudge write disclosure). No errors.
