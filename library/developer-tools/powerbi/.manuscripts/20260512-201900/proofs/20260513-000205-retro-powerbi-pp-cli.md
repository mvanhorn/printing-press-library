# Printing Press Retro: Power BI (production-use addendum)

## Session Stats
- API: powerbi (Power BI REST API)
- Spec source: hand-crafted internal YAML
- Mode: manuscripts-only retro (fresh conversation, no generation history)
- Trigger: three production bugs reported 2026-05-12 against the powerbi CLI
- Prior retro: `20260512-223111-retro-powerbi-pp-cli.md` (same run, post-generation)
- Surviving findings: 2 (1 Do-P1, 1 Do-P2, 1 Skip)

## Findings

### 1. Generator emits `args[1]` for path params exposed as flags, not positionals (Bug)
- **What happened:** `reports get-pages-in-group <reportId> --group-id <gid>` fails with `reportId is required`. The command's `Use` declares one positional (`<reportId>`), and `--group-id` is a flag, but the generated RunE reads `args[1]` (not `args[0]`) for `reportId`. Reproduction:
  ```
  powerbi-pp-cli.exe reports get-pages-in-group 37ae3f5d-… --group-id 804c5edc-… --json
  Error: reportId is required
  ```
- **Scorer correct?** N/A — runtime bug, no scorer involved.
- **Root cause:** Generator's command-emission template assigns path-param values from `args[N]` where N is the path-param's position in the URL path, regardless of whether that param is exposed as a positional or as a flag. When the agent (or profiler) chooses flag exposure for one path param (e.g., `--group-id`) and positional for the next, the args index for the positional is wrong by exactly the number of flag-exposed predecessors. Confirmed in `internal/cli/reports_get_pages_in_group.go:35-39`, `reports_get_in_group.go:35-39`, `dashboards_get_tiles_in_group.go:35-38` — every `*_in_group` command has the same off-by-one.
- **Cross-API check:** Universal across REST APIs with mixed-exposure path params:
  - Power BI `/groups/{groupId}/reports/{reportId}` — direct evidence (broken).
  - GitHub `/repos/{owner}/{repo}/issues/{issue_number}` — `owner`+`repo` typically scoped via flags, `issue_number` positional.
  - Microsoft Graph `/users/{userId}/messages/{messageId}` — `userId` flag, `messageId` positional.
- **Frequency:** every-API that uses flag-exposure for any subset of path params (which is most REST APIs with scoping containers).
- **Fallback if the Printing Press doesn't fix it:** Every printed CLI ships with broken commands in this shape. Claude can fix per-command (`args[1]` → `args[0]`, `len(args) < 2` → `len(args) < 1`) but won't realize it's template-level until two unrelated CLIs both fail the same way. The fix is a 4-character per-command edit; the cost is the cumulative confusion of multiple CLIs shipping broken until someone notices the pattern.
- **Worth a Printing Press fix?** Yes — universal, structural, prevents an entire class of broken commands.
- **Inherent or fixable:** Fixable. The generator already knows which path params are flag-exposed vs positional (it emits the `StringVar` for one and the `Use:` placeholder for the other). The indexing logic needs to count only positional path-params toward the args index.
- **Durable fix:** In the generator's command-emission template:
  1. Compute a positional-only path-param list (excluding flag-exposed ones).
  2. Emit `len(args) < len(positionalParams)` for the validator (not `len(args) < len(allPathParams)`).
  3. For each path-param, emit `replacePathParam(path, "name", args[i])` for positionals or `replacePathParam(path, "name", flagVar)` for flags — never reference `args[i]` for a flag-exposed param.

  Example correct output for `get-pages-in-group`:
  ```go
  if len(args) < 1 {
      return usageErr(fmt.Errorf("reportId is required\nUsage: %s <reportId>", cmd.CommandPath()))
  }
  path = replacePathParam(path, "reportId", args[0])  // not args[1]
  path = replacePathParam(path, "groupId", flagGroupId)
  ```
- **Test:**
  - positive: a generated CLI with `Use: "get-X <id>" --group-id <gid>` accepts `cli get-X <id> --group-id <gid> --json` and resolves both params correctly.
  - negative: `cli get-X --group-id <gid>` (missing positional) returns `<id> is required` with exit 2.
  - regression: a single-positional command (no flag-exposed path params) keeps `args[0]` behavior unchanged.
- **Evidence:**
  ```
  // reports_get_pages_in_group.go:35-39 (broken):
  if len(args) < 2 {
      return usageErr(fmt.Errorf("reportId is required\nUsage: %s <%s>", cmd.CommandPath(), "reportId"))
  }
  path = replacePathParam(path, "reportId", args[1])
  path = replacePathParam(path, "groupId", fmt.Sprintf("%v", flagGroupId))
  ```
  `Use: "get-pages-in-group <reportId>"` declares ONE positional, so `args[1]` never exists.
- **Related prior retros:**
  - #965 (`skill: verify-friendly RunE template misleads agents on multi-positional commands`) — `extends`. Different defect (multi-positional `len(args) < N` returns help on partial args instead of usage error) in the same RunE template area. The defect found here is off-by-one indexing, not validator dispatch. File new and reference #965.
  - #1192 (`replacePathParam emits zero path-param encoding`) — `extends`. Same template, different defect (encoding). Reference.

### 2. No `--file` alternative for query/expression-shaped positionals (Bug / Default gap)
- **What happened:** `dax save <name> <query>` fails on PowerShell with `usage: dax save <name> <query>` when the query contains brackets or single-quoted table names like `'Client Facilities'[Client Code] = "LFT"`. The user attributes this to "cobra treating bracketed tokens as something it shouldn't"; the actual root cause is that **PowerShell's word splitter and bracket-handling consume the query before cobra ever sees it**. The Printing Press cannot fix PowerShell. It *can* fix the symptom: emit a `--file <path>` flag alternative on commands whose positional is query/expression-shaped, so the user is never forced to shell-quote a complex query.
- **Scorer correct?** N/A.
- **Root cause:** Generator and SKILL templates for hand-built query commands emit a single positional path for the query body. `dax run` already has `--query` / `--file` flags (with the saved-query name as the positional fallback). `dax save` does not — it requires the literal query as the second positional. This inconsistency *within the same CLI* surfaces the template gap: the agent knew the pattern for `run` but didn't carry it to `save`.
- **Cross-API check:** Three concrete APIs where this recurs:
  - **Power BI DAX** — `'Table'[Column]` syntax, confirmed broken.
  - **Azure Monitor KQL** — `Heartbeat | where Computer == 'web01'`, brackets and quotes universally.
  - **BigQuery / Snowflake SQL** — backticks, quoted identifiers, brackets, `$variables`.
- **Frequency:** subclass:query-positional APIs (DAX, KQL, SQL, GraphQL, JSONPath, JMESPath). Not every API; high-signal for analytics and database-shaped APIs.
- **Fallback if not fixed:** Agents either hand-build `--file` flags per CLI (inconsistently — `dax run` has one, `dax save` doesn't) or users hit shell-quoting issues and have to discover workarounds. Skill instruction alone won't close it: the prior retro Skip'd "PowerShell quote-stripping hint on `--query`" and the same symptom resurfaced 24 hours later in production use.
- **Worth a Printing Press fix?** Yes — borderline P2. The cost-benefit is positive because the trigger is name-pattern detectable (positional named `query|expression|dax|kql|sql|filter|script|graphql`).
- **Inherent or fixable:** Fixable as a template default. Detect query-shaped positional names → emit `--file <path>` and `--from-stdin` alternatives + at-most-one validation.
- **Durable fix:**
  1. In the generator (or SKILL for hand-built commands), when a command has a positional arg whose name matches `/query|expression|dax|kql|sql|filter|script|graphql/i`, emit:
     ```go
     cmd.Flags().StringVar(&flagFile, "file", "", "Read the <name> from this file instead of the positional arg")
     // RunE: enforce mutual exclusivity, prefer --file when set
     ```
  2. Update the SKILL's hand-built-commands guidance with a one-sentence rule: "Any command accepting a query/expression positional must also accept `--file <path>` as an alternative — shell-quoting complex expressions is unreliable across Windows/POSIX."
- **Test:**
  - positive: `dax save monthly-rev --file q.dax` saves the query body from disk.
  - positive: `dax save monthly-rev 'EVALUATE TOPN(10, Sales)'` still works on POSIX shells.
  - negative: `dax save monthly-rev 'q' --file q.dax` returns a usage error ("specify positional OR --file, not both").
- **Evidence:** User's reported failure on `'Client Facilities'[Client Code] = "LFT"` reproduces deterministically on PowerShell. Internal inconsistency: `dax run` (lines 91-200 of dax.go) has both `--query` and `--file`; `dax save` (lines 291-332) has only positional. Agent applied the pattern to `run` but missed `save`.
- **Related prior retros:**
  - Prior powerbi retro Skip row: "PowerShell quote-stripping hint on `--query`" — `extends`. That Skip rejected a *docs hint*; this finding proposes a *template change* (different fix shape). The 24-hour recurrence in production strengthens the case that a docs hint alone is insufficient.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Generator emits `args[1]` for path params exposed as flags | generator | every-API with mixed-exposure path params | poor — silent miscompile, cryptic runtime error | small | only adjust args index for positional path params; never for flag-exposed ones |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | No `--file` alternative for query/expression positionals | generator + skill | subclass:query-positional APIs | medium — agent hand-builds partial workaround | small | only emit `--file` when positional name matches query/expression pattern |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| Bonus | `verify-skill` UnicodeEncodeError on Windows cp1252 | Step D: already filed 6+ times (#819, #832, #876, #976, #1109, **#1265 by yesterday's powerbi retro on this same run**). Re-raising at the same priority is the failure mode Phase 2.5 #5 exists to prevent. The cost-benefit math has been "no" five times. If the user wants this fixed, the path forward is escalating an existing issue (likely #1265 — most recent, same evidence), not filing a seventh duplicate. |

### Dropped at triage
*(none — only three candidates, all reached Phase 3)*

## Work Units

### WU-1: Generator path-param indexing accounts for flag exposure (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** Generated commands with mixed positional+flag path params resolve all params correctly regardless of arg order.
- **Target:** Generator templates in `internal/generator/` that emit per-command RunE bodies for endpoints with `{}` placeholders in the path.
- **Acceptance criteria:**
  - positive test: a generated CLI command with `Use: "get-X <id>"` + `--group-id` flag resolves `cli get-X 123 --group-id 456 --json` correctly.
  - negative test: `cli get-X --group-id 456` (missing positional) returns `<id> is required` with exit 2.
  - regression test: single-positional commands with no flag-exposed path params keep `args[0]` indexing.
  - cross-API smoke: regenerate at least two CLIs in `~/printing-press/library/` that use the `_in_group`-style template and confirm no `args[N]` reference exceeds the positional count.
- **Scope boundary:** Do NOT change how the agent decides which path params are flag-exposed vs positional (that's the profiler / spec author's decision). Only fix the indexing inside the emitted RunE.
- **Dependencies:** None.
- **Complexity:** small.

### WU-2: Query/expression positional commands also emit `--file` alternative (from F2)
- **Priority:** P2
- **Component:** generator (primary) + skill (companion guidance)
- **Goal:** Commands whose positional arg is a query/expression body never force users to shell-quote complex bodies.
- **Target:** Generator command-emission template (positional arg detection) and `skills/printing-press/SKILL.md` (hand-built command guidance section).
- **Acceptance criteria:**
  - positive test: a generated command with positional `<query>` (or `<dax>`, `<kql>`, `<sql>`, `<expression>`, etc.) accepts `--file <path>` and reads the body from disk.
  - positive test: the same command still accepts the positional form on POSIX shells.
  - negative test: passing both positional AND `--file` returns a usage error.
  - regression test: positional names that don't match the query/expression pattern (e.g., `<name>`, `<id>`) do not gain `--file`.
- **Scope boundary:** Do NOT propagate `--file` to every positional indiscriminately. Match on name pattern only. Do NOT change how `dax run` already handles `--file` — only fix the gap on sibling commands like `dax save`.
- **Dependencies:** None.
- **Complexity:** small.

## Anti-patterns
- **Repeated duplicate filing.** The `verify-skill` Windows UTF-8 issue has now been filed seven times across retros. Each retro independently surfaces it, classifies it as systemic, files it. The Phase 2.5 recurrence-cost check exists for exactly this; it caught it here. Suggestion: the issue-template dedup scan in `references/issue-template.md` Step 2.5 should weight more heavily on title-substring matches like "verify-skill" + "Windows" + "UTF-8" / "cp1252" / "charmap" — currently it's missing them.

## What the Printing Press Got Right
- The DAX `run` command **does** include `--query` and `--file` alternatives — the pattern is in the SKILL playbook for hand-built query commands. The gap is that the same playbook wasn't applied to the sibling `dax save`. The skill knew the right shape; the agent forgot to carry it across.
- The `_in_group` template generates idiomatic per-command files (one Go file per endpoint), making the off-by-one easy to spot in code review. A monolithic generator would hide it.
- Prior retro correctly classified `verify-skill` UTF-8 crash as P1 and filed it. The system worked — the failure mode is the system's *aggregate* memory across retros, not any single retro's judgment.
