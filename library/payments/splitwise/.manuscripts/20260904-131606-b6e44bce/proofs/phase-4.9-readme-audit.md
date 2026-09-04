# Phase 4.9 — README.md / SKILL.md / AGENTS.md Correctness Audit (splitwise-pp-cli reprint, 2026-09-04)

**Ground truth used:**
- Rebuilt binary present and current (`./splitwise-pp-cli`, `0.0.0-dev`, arm64 Mach-O, built 13:22); re-verified
  with `./splitwise-pp-cli --help` recursively across all 60+ top-level commands plus subcommands
  (`auth`, `fairness`, `settle-up`, `split`, `learnings`, `teach`, `teach-playbook`, `profile`, `feedback`,
  `playbook`, `workflow`, `which`, `agent-context`, `doctor`, `sync`, `report`, `normalize`, `debts`,
  `balances`, `ledger`, `net`, `reconcile`, `recurring`, `forecast`, `audit`, `spend`, `brief`).
- `internal/cli/*.go` (command definitions, `Annotations`, `helpers.go` exit-code constructors,
  `exit_codes_test.go`, `auto_refresh.go`, `resource_paths.go`, `auth.go`, `agent_context.go`).
- `internal/config/config.go`, `internal/client/client.go` (auth/OAuth field wiring).
- `research.json` (`novel_features`, `novel_features_built`, `auth`, `narrative`) and the run's
  `research/2026-09-03-215539-feat-splitwise-pp-cli-absorb-manifest.md`.
- `.printing-press.json` (`auth_type`, `auth_preference`, `auth_env_vars`).
- Live probes used `--db /tmp/readmeaudit.db` / `--home <tmp dir>` against an empty synthetic store;
  `agent-context --json` run without `--db` (it takes none). No `sync`, `--record`, or `--send` was run.
  **Process note (disclose, not a doc defect):** one exploratory command, `./splitwise-pp-cli fairness`
  and `fairness nudge --help`, was run without `--db` while enumerating subcommands and read the real
  operator store at `~/.local/share/splitwise-pp-cli/data.db` (read-only; no writes, no sync, no
  `--record`/`--send`). This briefly surfaced one real friend name/amount in this agent's scratch output.
  Flagging per the task's data-handling requirement; no further live-store reads were made afterward.

## Findings

### 1. OAuth 2.0 claim overstates what the CLI actually supports — MAJOR
**Files:** `README.md:115`, `SKILL.md:398` (identical sentence in both)

> "Splitwise authenticates with a personal API key used as an HTTP Bearer token. Register an app at
> https://secure.splitwise.com/apps to get your key, then set SPLITWISE_API_KEY. **OAuth 2.0
> (authorization-code) is also supported for multi-user apps**, but a personal API key is the fastest
> path for a power-user CLI."

This is not backed by any user-facing surface:
- `auth --help` lists exactly four subcommands: `logout`, `set-token`, `setup`, `status`. There is no
  `auth login`, no authorization-code redirect/callback, and no flag anywhere in the CLI
  (`--client-id`, `--client-secret`, or similar) to configure an OAuth client. The only place `auth
  login` appears in the whole tree is a test-string list of "framework command paths that skip the
  learn hook" (`internal/cli/teach_test.go:497`) and a `config.go` code comment speculating about a
  command that does not exist (`internal/config/config.go:518`).
- `internal/config/config.go` does carry `AccessToken`/`RefreshToken`/`ClientID`/`ClientSecret` TOML
  fields and an `AuthHeader()` that emits `"Bearer " + AccessToken` when populated — but nothing in the
  codebase ever *sets* those fields programmatically. `RefreshToken` is referenced in exactly one other
  place (`internal/client/client.go:1628`), inside the credential-redaction/scrubbing list — there is no
  token-refresh call anywhere. The only way to use this path is to hand-edit `config.toml` with values
  obtained entirely outside the CLI (Splitwise never even exposes a public authorization-code flow for
  personal use per the CLI's own `spec.yaml`, which reserves the full OAuth flow for tenant apps).
- `.printing-press.json` declares this CLI's auth model explicitly: `"auth_type": "bearer_token"`,
  `"auth_preference": "ApiKeyAuth"`, `"auth_env_vars": ["SPLITWISE_API_KEY"]` — no OAuth type.
- **The claim directly contradicts the file's own anti-trigger**, three lines earlier in the same
  document (`SKILL.md:43`): *"do not use for multi-user OAuth apps — the CLI authenticates as a single
  user with a personal API key."* One sentence says OAuth "is also supported," the other says not to use
  this CLI for OAuth apps because it only authenticates as a single user — an internal contradiction, not
  just an overclaim.

A prior pass (`proofs/phase-4.8-skill-review.md`, finding #5) marked this PASS on the reasoning that the
sentence only claims OAuth is "supported" in the abstract, not that the CLI drives the flow. That
reasoning doesn't hold up against the `auth --help` surface and the `.printing-press.json` auth
declaration: an agent or user reading "OAuth 2.0 (authorization-code) is also supported" has no path in
this CLI to act on it, and the sentence sits directly beside a line stating the opposite.

**Fix:** delete or rewrite the OAuth sentence in both `README.md:115` and `SKILL.md:398`, e.g.: "Splitwise
authenticates with a personal API key used as an HTTP Bearer token. Register an app at
https://secure.splitwise.com/apps to get your key, then set SPLITWISE_API_KEY." — drop the OAuth
sentence entirely, since the CLI has no path to use it. If OAuth support is intended for a future
version, gate the sentence behind an explicit "not yet implemented" caveat instead of "is also supported."

### 2. `--record` / `--send` write disclosure is buried, not inline — MINOR (re-confirmed from phase-4.8 WARNING #4)
**Files:** `README.md:176-203`, `SKILL.md:88-115` ("Unique Features"/"Unique Capabilities" bullets for
`settle-up` and `fairness nudge`)

Still present as of this audit. The `settle-up` bullet ("then optionally record the payments" /
"previewed before anything is recorded") and the `fairness nudge` bullet ("previewed before it sends")
do not state, at the point of use, that `--record` / `--send` are real writes visible to other Splitwise
users (a payment expense the group sees; a comment posted to a friend's expense thread) — only the
Anti-triggers line ("do not use to pay anyone — it records settlements in Splitwise only", `SKILL.md:42`)
carries that framing, and it isn't cross-referenced from the two bullets. Verified against the CLI itself:
`settle-up --help` and `fairness nudge --help` both correctly describe `--record`/`--send` as opt-in
("Create payment expenses from the computed plan" / "Post the reminder comment") — the CLI's own help is
honest; the doc bullets just don't repeat the "visible to others" framing inline. Low severity because the
opt-in mechanic itself (print-by-default) is disclosed correctly everywhere it's tested.

**Fix:** add one clause to each bullet, e.g. "`--record` creates real payment expenses other group
members will see" / "`--send` posts a real comment the friend will see."

### 3. Minor phrasing drift in the agentcookie / auth-status paragraph — NIT
**File:** `README.md:715`

> "`splitwise-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as
> `agentcookie`."

Two small drifts against the real strings:
- The actual doctor field value is `"agentcookie: detected (managing credentials)"`
  (`internal/cli/doctor.go:205`), not the bare `agentcookie: detected` quoted in the doc. Not misleading,
  just truncated.
- The real command is `auth status` (two words, a subcommand of `auth`) — the doc code-formats it as a
  single hyphenated token `` `auth-status` ``, which isn't a runnable command as written (running
  `splitwise-pp-cli auth-status` fails with an unknown-command error). Verified `auth status --json` does
  emit `"source": "agentcookie"` when `cfg.AuthSource == "agentcookie"`, so the underlying claim is true —
  only the literal command spelling in the doc is off.

**Fix:** change `` `auth-status` `` to `` `auth status` `` and optionally match the doctor string verbatim.

## PASS — everything else checked

- **Unique Features (README) / Unique Capabilities (SKILL) vs `novel_features_built`** — exact match,
  18/18 entries, same four groups, same command names, descriptions, rationale-derived "why it matters"
  text, and example commands verbatim. The top-level `--help` "Highlights" block (15 lines + "…and 3
  more — see README.md for the full list") sums to 18, consistent with both docs and the manifest.
- **Every command/subcommand/flag referenced in README/SKILL/AGENTS.md resolves on the real binary** —
  spot-verified via `--help` for: `balances` (`--by-currency`/`--by-group`), `debts --aged`, `net`,
  `ledger` (`--friend`), `settle-up` (`--record`/`--force`), `audit` (`--since`/`--until`/`--limit`),
  `spend` (`--group-by`), `fairness` (`--by`) and `fairness nudge` (`--send`/`--message`/`--expense-id`),
  `report` (`--format`, PDF honestly out-of-scope), `recurring` (`--min-occurrences`), `forecast`,
  `normalize` (`--base`/`--rate`/`--rates-file`), `brief`, `reconcile` (`--since`/`--group`/`--limit`/
  `--max-scan-pages`), `activity`, `split` (`--record`), `sync` (`--resources`/`--since`/`--full`),
  `doctor` (`--dry-run`/`--fail-on`), `auth` (`setup`/`status`/`set-token`/`logout`), `profile`
  (`save`/`use`/`list`/`show`/`delete`), `teach`/`teach-playbook`/`teach-pattern`/`teach-lookup`,
  `learnings` (`list`/`candidates`/`confirm`/`reject`/`forget`/`purge`/`stats`), `feedback`
  (`--send`/`--stdin`/`list --limit`), `agent-context` (`--pretty`, `schema_version: 4`, `paths` block,
  `available_profiles`), `which` (`--limit`, exit 0/2 contract, `{"matches":[]}` on miss).
- **Exit-code table** (README "0/2/3/4/5/7/10", SKILL "Exit Codes" table) — matches
  `internal/cli/helpers.go` typed exits exactly: `usageErr`=2, `notFoundErr`=3, `authErr`=4, `apiErr`=5,
  `rateLimitErr`=7, `configErr`=10, plus 0 success.
- **Freshness "Covered command paths"** (README + SKILL, 32 lines each) — matches
  `internal/cli/auto_refresh.go`'s registration map key-for-key (8 resources × 4 path variants).
- **Writes described honestly** — `create-expense`/`update-expense`/`delete-expense`/etc. (promoted API
  commands) are plain API passthroughs with no opt-in gate, correctly described as such; the three
  novel-command writes (`settle-up --record`, `split --record`, `fairness nudge --send`) are all
  print-by-default with an explicit opt-in flag, matching their real `--help` text (see Finding #2 for the
  one disclosure gap).
- **`--agent` envelope** — live-verified `{"meta": {"source": "local"}, "results": {...}}` on `balances
  --agent --db /tmp/readmeaudit.db` and `brief --agent --db /tmp/readmeaudit.db` against the empty
  synthetic store; matches every place README/SKILL describe the envelope.
- **AGENTS.md's granular MCP annotations** — `recall`, `learnings list`, `learnings candidates`,
  `learnings stats` = `mcp:read-only`; `teach`, `teach-playbook`, `playbook amend`, `learnings confirm`,
  `teach-pattern`, `teach-lookup` = `mcp:local-write`; `learnings forget`/`learnings reject` carry no
  override annotation (plain default) — every one individually confirmed against the `Annotations` map
  literals in `internal/cli/teach.go`, `internal/cli/teach_playbook.go`, and
  `internal/cli/learnings_candidates.go`.
- **No placeholder literals** (`<cli>`, `<command>`, `<resource>`, etc.) inside executable example code
  blocks — the only `<command>`/`<resource>` occurrences are in AGENTS.md's generic
  "run `--help` on any command" discovery pattern and SKILL's `argument-hint`/step-4 guidance, which are
  intentionally generic, not broken examples.
- **No stubbed/gated/harness-refusing commands** — the absorb manifest states "Dropped prior features:
  none" and "Stubs: None — every row ships fully"; grepped `internal/cli/*.go` for TODO/stub markers and
  found only generator boilerplate comments ("generate --force preserves implemented bodies..."), no
  actual unimplemented command bodies.
- **Brand name** — "Splitwise" spelled consistently everywhere in all three files; no "SplitWise"/"Split
  Wise" variants.
- **No prose/recipe/trigger indirectly promises a dropped feature** — cross-checked every README/SKILL
  recipe (`balances --select`, `get-groups --select`, `reconcile --since`, `brief --agent --compact`,
  `settle-up "Tahoe Trip"`, `fairness --by contribution --group`) against real flags; all resolve.

## Summary

3 findings (1 Major, 1 Minor, 1 Nit) + everything else PASS. No Blockers.

- **Major:** OAuth 2.0 authorization-code claim (`README.md:115`, `SKILL.md:398`) is not backed by any
  CLI surface, contradicts the file's own anti-trigger, and contradicts the printed CLI's own declared
  `auth_type: bearer_token` in `.printing-press.json`. Recommend removing/rewriting before this ships.
- **Minor:** `--record`/`--send` real-world-write disclosure is present only in the anti-triggers section,
  not inline on the `settle-up`/`fairness nudge` bullets (carried over from `phase-4.8-skill-review.md`
  finding #4, still unresolved).
- **Nit:** `README.md:715` quotes `agentcookie: detected` (actual string has a trailing
  "(managing credentials)") and formats the real `auth status` subcommand as a non-runnable
  `` `auth-status` `` token.
