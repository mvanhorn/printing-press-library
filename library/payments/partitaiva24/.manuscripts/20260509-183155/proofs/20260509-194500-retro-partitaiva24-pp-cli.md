# Printing Press Retro: partitaiva24

## Session Stats
- API: partitaiva24 (Italian e-invoicing platform on WordPress REST)
- Spec source: agent-authored internal YAML (built from live `GET /api/v1/` self-discovery + 26 authenticated GET probes)
- Scorecard: 85/100 (Grade A)
- Verify pass rate: 100% (42/42)
- Live dogfood: 295/304 pass (9 reclassified non-bugs)
- Fix loops: 1 (codex-bail recovery)
- Manual code edits: ~5 (auth_set.go, config.go nonce wiring, narrative drift, backup zip ordering, f24 ical JSON mode)
- Features built from scratch: 12 (transcendence) + 1 (auth set)

This run was unusually clean. WP-REST self-description gave us 77 endpoints + every response shape in two HTTP probes; the user provided live cookie+nonce so Phase 5 ran against the real tenant. The friction was concentrated in two places: codex delegation and narrative authoring. Most "issues" I noticed are per-CLI quirks or working-as-designed.

## Findings

### 1. Codex `exec` can exit 127 partway and silently produce incomplete output (skill instruction gap)
- **What happened:** Delegated 12 novel features to `codex exec --dangerously-bypass-approvals-and-sandbox -o /tmp/last-msg.txt …`. Codex explored the repo, wrote 13 of the planned files (most of the work, including helpers + 9 commands + root.go registrations), then exited 127 mid-stream. The `--output-last-message` file was never created. The bash background task reported "completed (exit code 0)" because nohup masked the non-zero. I noticed only because (a) the last-message file was missing, and (b) my first `ls` for novel files used the wrong glob and showed nothing — I almost concluded codex did nothing, when in fact it had done ~80%.
- **Scorer correct?** N/A (not a score-driven finding).
- **Root cause:** The codex-delegation reference (`skills/printing-press/references/codex-delegation.md`) tells the SKILL to use `-o <file>` but doesn't require checking the file exists / non-empty / contains a recognizable summary marker before treating the delegation as successful. nohup-masking compounds it: `exec 0` from nohup ≠ codex exit 0.
- **Cross-API check:** Affects every codex-delegated run regardless of API. Independent of API shape.
- **Frequency:** Every codex run (~once per `/printing-press <api> codex`).
- **Fallback if the Printing Press doesn't fix it:** Agent has to remember to verify the output file each time. I forgot once in this run; would forget again.
- **Worth a Printing Press fix?** Yes — small skill change, high-frequency surface.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Add to `references/codex-delegation.md`: after `codex exec` returns, treat the run as "incomplete" if `--output-last-message` produced no file or a zero-byte file, and re-list the working dir to compare against the planned filename set. Even simpler: have the prompt template require codex to write a `_codex-result.json` summary at the end with file list + build status, and the SKILL fails the delegation if that file is missing. This converts "did codex finish?" from a guess into a deterministic check.
- **Test:** Positive: prompt template emits `_codex-result.json`; SKILL parses it. Negative: simulate codex bailing partway by killing the process — SKILL detects missing summary and surfaces the partial state instead of silently continuing.
- **Evidence:** This run, ~7:35pm — `cat /tmp/p24-codex-output.txt` returned "No such file or directory". `ls "$DD"/sample-*.json` (with the wrong glob) reinforced the false-negative.
- **Related prior retros:** None (first retro in user's manuscripts).
- **Step G case-against:** Codex exit-127 may be a transient sandbox issue specific to this codex/macOS combination. The fix is one of those "obvious in hindsight" defenses that may not pay dividends on every machine.
- **Why Step G fails the case-against:** The cost is a one-line check + a result-summary marker. Even if codex 1.0.5 fixes the specific exit-127 path, *any* premature exit (network blip, OOM, user Ctrl-C on a different terminal) produces the same silent failure shape. Defense-in-depth pays.

### 3. Italian invoice sezionali confound naive integer-extraction parsers (skill instruction gap, narrow but recurring)
- **What happened:** The hand-authored `numbering audit` command (a transcendence feature) used a "strip the number, ignore the rest" parser for invoice numbers. Tested live against my real partitaiva24 invoices and produced a false `date_disorder` finding because the user has two sezionali — main series (1, 2, …, 16) and a separate `INF` series with one invoice (`01/INF`). The parser collapsed both into one "sequence of integer 1, integer 1, …" and saw the dates jumping back when it crossed sezionali. The correct shape is: parse `(series, number)` tuples, group by series, run gap/duplicate/date checks within each group.
- **Scorer correct?** N/A — no scorer flagged this; only a real user (me) caught it during use.
- **Root cause:** The novel-features brainstorm subagent doesn't (and probably can't) know the sezionale convention. The SKILL's narrative-authoring rules don't warn about it either. Any agent writing an Italian-tax CLI's numbering audit will reach for the obvious "extract integer, sort, compare" pattern.
- **Cross-API check:** Italian e-invoicing is the obvious case. But the broader pattern — "an ID field has a structural prefix/suffix that splits items into independent sequences" — recurs: PR numbers per repo (GitHub `org/repo#123` vs `org/other-repo#123`), Stripe charge IDs across modes (`ch_live_*` vs `ch_test_*`), invoice numbering in any country with multiple registers (Italy, Spain, France, Germany all permit sezionali under their VAT laws). Italy is just the loudest because the AdE guidance is explicit about it.
- **Frequency:** Specific to invoice-numbering audits, but those exist in every accounting/SaaS billing CLI. ~5-10% of catalog APIs have at least one numbering-audit-shaped novel feature.
- **Fallback if the Printing Press doesn't fix it:** The agent ships the naive parser, the user runs the command on real data, gets a false positive, debugs for 10 minutes, fixes the parser. Cost = one wasted user session per affected CLI.
- **Worth a Printing Press fix?** Borderline. A single SKILL note could prevent the recurrence: "When an audit feature parses identifiers that may contain structural prefixes/suffixes (sezionali, environments, repo scopes, regional codes), do NOT collapse to the integer alone — separate the alpha and numeric tokens, group by alpha, run checks per group."
- **Inherent or fixable:** Fixable in the SKILL.
- **Durable fix:** Add a single bullet to the SKILL's transcendence-feature guidance (Phase 3 build checklist or the helpers section): "**ID parsers in audit commands must not collapse structural prefixes/suffixes.** When the underlying domain has multiple parallel sequences (sezionali, environments, scopes), parse identifiers into `(group, number)` tuples and run sequence checks per group, not globally. Example: Italian invoice numbers like `01/INF` and `1/A` are distinct sezionali under DPR 633/72; the audit's `(group=INF, n=1)` and `(group=A, n=1)` belong in separate streams. Same pattern applies to PR numbers per repo, charges per mode, etc."
- **Test:** Positive: SKILL contains the bullet; an agent writing a similar audit feature on the next CLI follows it. Negative: a follow-up CLI's numbering audit doesn't false-positive on multi-stream IDs.
- **Evidence:** This conversation, ~8:45pm — I ran `numbering audit --year 2026` on real data, got the `01/INF → 1` date-disorder false positive. Drilled into the SQL, identified the parser flattening sezionali, rewrote `parseInvoiceNumber` to return `(series, n, ok)` and group checks per series. Output went from "1 false-positive disorder finding" to "PASS All sezionali are AdE-compliant."
- **Related prior retros:** None.
- **Step G case-against:** This is one specific feature on one specific CLI. Generalizing to "all parsers everywhere" is overreach. The post-hoc fix was 80 lines of code; doable in any reuse.
- **Why Step G fails the case-against:** The cost of the SKILL bullet is ~3 lines. The benefit is preventing the same shape of false positive from shipping in any future audit-style command. The novel-features brainstorm subagent already produces audit features routinely; this is a known pattern with a known fix and a low-cost prophylactic.

### 2. Narrative recipes/examples aren't sanity-checked against the actual command surface during authoring (skill instruction gap)
- **What happened:** Two of my own narrative drift bugs slipped past the gate until `validate-narrative --strict --full-examples` caught them in shipcheck:
  1. `narrative.novel_features[*].example` for `vies bulk` was `partitaiva24-pp-cli vies bulk --country-type eu --json` — but `--country-type` doesn't exist on the command (the implementation auto-filters EU customers from the synced store; no flag needed).
  2. `narrative.recipes[*].command` for the "Forfettario quarter check-in" was `… sync && … turnover --year 2026 --json && … tax-due --quarter 2026-Q2 --json`. validate-narrative runs the entire string as a single binary invocation — `&&` becomes positional args — so the command fails with `unknown flag: --year` (sync doesn't have it).
- **Scorer correct?** Yes — `validate-narrative --strict --full-examples` correctly flagged both. The scorer is the post-hoc safety net working as designed.
- **Root cause:** The SKILL's narrative-authoring guidance (Phase 1.5d in the main printing-press SKILL) tells the agent to write `command` and `example` strings but doesn't enumerate the two failure modes that recur:
  1. Inventing a flag that "feels right" without verifying it exists in the implementation. Particularly common when the absorb-LLM writes the manifest and a different LLM (or model state) writes research.json.
  2. Using shell chaining (`&&`, `|`, `;`) inside `narrative.recipes[*].command`, which validate-narrative runs as a single argv vector — chaining doesn't survive the parse.
- **Cross-API check:** Both failure modes recur. Drift on imagined flags is more frequent on combo / novel-heavy CLIs (more opportunity to mis-name). Shell chaining is universal — anyone writing a "do step 1, then step 2" recipe reaches for it.
- **Frequency:** Most CLIs with non-trivial novel features will hit at least one. I hit two on a CLI I designed end-to-end myself.
- **Fallback if the Printing Press doesn't fix it:** validate-narrative catches them in shipcheck. So the bug doesn't ship — it just costs a fix-loop. A SKILL note moves the catch upstream into the authoring step.
- **Worth a Printing Press fix?** Yes — single-paragraph SKILL addition, high recurrence rate.
- **Inherent or fixable:** Fixable in the SKILL.
- **Durable fix:** Add a single note to the main printing-press SKILL's narrative authoring section (Step 1.5d "Narrative rules", probably after rule 8 about recipes): "**Recipe and example commands run through `validate-narrative` as a single binary invocation under `PRINTING_PRESS_VERIFY=1`.** Two consequences: (a) shell chaining (`&&`, `|`, `;`) does NOT execute — the entire string is parsed as one argv vector, so `cli sync && cli turnover` becomes `cli sync && cli turnover` (positional args). Split multi-step recipes into separate recipes. (b) Every `--flag` you write must already exist on that exact command path. Verify by running `<binary> <command> --help` on the just-built binary before committing the recipe to research.json."
- **Test:** Positive: the SKILL's narrative-rules section explicitly mentions both pitfalls. Negative: an agent following the updated SKILL on a future CLI doesn't ship a `&&`-chained recipe or invent a flag.
- **Evidence:** `validate-narrative` output during shipcheck step 1, ~7:30pm. Two FAILED entries against research.json's `narrative.recipes` and `narrative.novel_features[].example`.
- **Related prior retros:** None.
- **Step G case-against:** validate-narrative already catches both. Adding to the SKILL increases skill-doc weight without preventing a shipped bug. The fix-loop is cheap.
- **Why Step G fails the case-against:** The fix-loop ate a turn to identify, two `sed`/Edit invocations to repair, plus a regen of research.json through python (the file had unicode-escaped `&&`). Each future CLI pays the same cost. A 4-line SKILL note transfers that cost to one PR review.

### 4. WP-REST mutating endpoints can leave phantom records on partial-failure 4xx (skill instruction gap)
- **What happened:** During interactive use after the run shipped, I ran `invoices create --stdin` with a body that triggered HTTP 409 (`foreign key constraint fails (e_invoice_meta, …)`). The CLI surfaced the 409 verbatim and exited non-zero. But: the next `invoices list` call showed a phantom draft invoice with the same number — partitaiva24 had inserted the row in `e_invoices` then failed on the cascade insert into `e_invoice_meta`, returned 409 to the client, and left the partial record in the database. From the agent/user perspective, "the create failed" was a false belief; the record was real.
- **Scorer correct?** N/A.
- **Root cause:** Not a CLI bug per se — partitaiva24's WordPress backend isn't atomic across the parent + meta inserts. But the CLI's own error message ("create failed: HTTP 409 …") implicitly tells the user "this didn't happen," which is wrong. The correct message would acknowledge that 4xx on a WP-REST POST may have left server state behind, and direct the agent to verify with a follow-up GET before retrying.
- **Cross-API check:** Recurs on any WordPress REST API where the plugin uses WP_Database transactions inconsistently (very common: WP-REST plugins routinely don't wrap parent+children INSERTs in a transaction). Not specific to partitaiva24 — the pattern is structural across WordPress as a backend choice.
- **Frequency:** Specific to WP-REST-backed CLIs. Within that class: any mutating endpoint that creates parent + cascade rows. ~1-3 catalog APIs likely WP-REST-backed.
- **Fallback if the Printing Press doesn't fix it:** The agent retries on 409, ends up with N phantom records, eventually figures it out from `invoices list`. This happened to me in this very session.
- **Worth a Printing Press fix?** Borderline — the SKILL note is a one-liner, but the case is narrow. Stronger argument: the *generic* lesson "4xx after POST does not always mean server state is unchanged — verify before retrying" applies broadly across REST APIs that lack proper transaction boundaries (Salesforce, Shopify under load, certain Stripe edge cases). One bullet covers all of them.
- **Inherent or fixable:** Fixable in the SKILL.
- **Durable fix:** Add to the main printing-press SKILL's "Anti-reimplementation" / mutator guidance: "**A 4xx response on a POST/PUT/DELETE does not guarantee the server has no partial state.** WP-REST plugins, plugins under load, and partitioned-write APIs commonly leave parent records when the cascade fails. After any 4xx on a mutator, do NOT retry blindly — issue a GET (or list with appropriate filters) to verify whether the resource exists. If it exists in an unexpected state, prefer DELETE-then-recreate over retry." Keep it generic; cite WP-REST as the canonical example.
- **Test:** Positive: SKILL contains the bullet. Negative: an agent following it on a future run sees 409, runs `<resource> list` to verify, deletes the phantom, then retries with a fixed body — instead of retrying blindly and creating duplicates.
- **Evidence:** This conversation, ~9:45pm. POST → 409 FK constraint on `e_invoice_meta`. `invoices list` showed both the new (#18 Berlino) AND a phantom (#17 Progetto Greenwich) created by the earlier 500-tagged attempt. Both deleted via CLI to confirm zero residue.
- **Related prior retros:** None.
- **Step G case-against:** This is one specific WordPress-plugin behavior that may not generalize. Adding generic mutator guidance for one observed case is overreach.
- **Why Step G fails the case-against:** The phantom-record pattern is a known anti-pattern in REST APIs broadly; WP-REST is just where I caught it in this session. The bullet's scope is "verify before retrying after 4xx on mutators" — that's a universally good agent behavior whose only cost is one SKILL line. The cost of getting it wrong (silent duplicate creation in a fiscally-binding API like an Italian invoicing system) is asymmetric to the upside.

## Prioritized Improvements

### P3 — Low priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Codex output-file verification before treating delegation as successful | skill | every codex run | low (agent forgets) | small | none — pure defense |
| F2 | SKILL note: validate-narrative parses recipes as single argv; verify flags against built binary | skill | most non-trivial CLIs | medium (validate-narrative catches it post-hoc) | small | none |
| F3 | SKILL note: ID parsers in audit features must not collapse structural prefixes (sezionali, environments, scopes) | skill | ~5-10% of CLIs (any with audit-shaped novel features over multi-stream IDs) | low (agent ships naive parser, user catches it) | small | none |
| F4 | SKILL note: 4xx on mutators may leave phantom server state — verify before retrying | skill | every WP-REST CLI + any non-transactional mutator | low (agent retries blindly, creates duplicates) | small | applies to mutating commands only |

### Skip

| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| S1 | Spec extension for "companion auth headers" (cookie + X-WP-Nonce as separate headers) | Step B: only 1 named API class with evidence (WP-REST). Existing `Config.Headers` map already absorbs the runtime mechanism; the agent post-patch was 6 lines. |
| S2 | dogfood "expected non-zero exit for invalid argument" matrix produces false failures on permissive REST APIs | Step G: case-against stronger. The acceptance-report reclassification flow already handles this; tightening the matrix needs per-API annotation that adds friction worse than the false positives. |
| S3 | Phase 5 promote gate is too strict ("tests_passed == matrix_size" for full) — forced downgrade to "quick" with reclassification notes | Step G: the gate's rigidity is a feature. Allowing reclassification opens a slippery slope. The "fix-bugs-in-session" ethic depends on this gate being unforgiving. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Reserved-name `profile` resource collision | Generation rejected `profile` as a resource name; clear actionable error message. | working-as-designed; explicit rejection > silent rename |
| Bash sandbox blocks curl in for-loops | Curl-in-loop fails inside Bash tool; switched to Python urllib | Bash tool sandbox behavior, not Printing Press |
| Empty-Printer warning fires every run | `WARNING: spec.Printer is empty` | iteration-noise; user fixes once via `git config github.user` |
| `mcp:hidden` annotation isn't enumerated alongside `mcp:read-only` in main SKILL | Skill mentions read-only but not hidden | unproven-one-off; one example only, agent can read the runtime walker contract directly |
| MCP enrichment "code orchestration + hidden endpoint tools" warning fires for >50 tool surfaces | Generator emits a clear pre-generation hint | working-as-designed; the hint is exactly what got me to add the `mcp:` block on regen |
| Codex bailing on internal sandbox via `dangerously-bypass-approvals-and-sandbox` flag | Exit 127 in codex's internal harness | partially upstream codex behavior; F1 captures the systemic mitigation |

## Work Units

### WU-1: Add codex output-file + summary-marker verification (from F1)
- **Priority:** P3
- **Component:** skill
- **Goal:** Make codex delegation failures (premature exit, sandbox abort, network drop) deterministically detectable instead of silently producing partial output.
- **Target:** `skills/printing-press/references/codex-delegation.md` (the prompt template + the post-run check).
- **Acceptance criteria:**
  - positive test: a successful codex run writes a `_codex-result.json` (or equivalent named marker) at the end of its prompt; the SKILL reads it and proceeds.
  - negative test: a codex run that exits before writing the marker is detected by the SKILL — the user/agent sees an explicit "codex output marker missing — partial work may be present in `<dir>`; review before continuing" rather than silent success.
- **Scope boundary:** Doesn't change codex itself. Doesn't add retry logic. Just adds the deterministic-completion check.
- **Dependencies:** None.
- **Complexity:** small.

### WU-2: SKILL note on narrative recipe pitfalls (from F2)
- **Priority:** P3
- **Component:** skill
- **Goal:** Prevent two recurring drift modes in `research.json` narratives — invented flags and shell-chained recipes — by adding a clear note in the narrative-authoring rules.
- **Target:** Main printing-press SKILL.md, the "Narrative rules" subsection of Step 1.5d (numbered rule list).
- **Acceptance criteria:**
  - positive test: the SKILL.md narrative rules contain two short sub-bullets — one warning that recipes parse as single argv (no `&&`/`|`/`;`), one warning to verify flags via `<binary> <command> --help` before committing to research.json.
  - negative test: an agent following the updated SKILL doesn't author a recipe with shell chaining, and validate-narrative shipcheck shows zero `--strict --full-examples` failures on the next CLI's first try.
- **Scope boundary:** Doesn't change validate-narrative itself. Doesn't change research.json schema. Just adds an authoring-time rule.
- **Dependencies:** None.
- **Complexity:** small.

## Anti-patterns
- **Trusting nohup'd background-task exit code as a proxy for the wrapped command's exit code.** A `codex exec` that exited 127 was reported as task-completed exit 0 because nohup masked it. Use `--output-last-message` AND verify the file exists; or use `codex exec --json` and parse the events stream for completion.
- **Writing narrative recipes that look like shell pipelines.** validate-narrative runs the whole string as a single binary invocation — `&&` becomes positional args. One recipe per shell-step.
- **Authoring `narrative.novel_features[*].example` from imagination.** Always run `<binary> <command> --help` against the just-built CLI before committing the example. The cost of building the binary first is microseconds; the cost of a shipped wrong-flag example reaches every user and every agent that copy-pastes the recipe.

## What the Printing Press Got Right
- **`Config.Headers` map (v4.2.0) is exactly the right mechanism for non-Authorization auxiliary headers.** Adding X-WP-Nonce was a 12-line patch in `internal/config/config.go` because the runtime infrastructure already existed. Without `Config.Headers`, this would have meant editing the client template — a generator-level change for a printed-CLI need.
- **WP-REST self-description hit a perfect groove.** `printing-press generate --spec <internal yaml>` consumed a hand-authored 770-line spec built from live route discovery and produced a CLI that passed all 8 quality gates on first run. The internal YAML format absorbed 77 endpoints + 18 typed responses without any parser flexes — exactly what it should do.
- **MCP enrichment warning at >50 tools is the right shape.** The generator printed a clear pre-generation hint suggesting `mcp.transport: [stdio, http]` + `orchestration: code` + `endpoint_tools: hidden`. Following the hint took 4 lines in the spec and dropped the surface from 89 raw tools to a clean 2-tool orchestration pair without losing any functionality. That's the difference between "generator nags about scale" and "generator helps fix scale."
- **Doctor's `verify_path` + the cookie auth template gave honest credential validation.** With cookie+nonce both set, doctor reported `Credentials: valid` against `/user/profile` — a real authenticated round-trip. With the cookie missing or nonce stale, it correctly reported invalid (HTTP 401). The chain "spec auth.verify_path → generated doctor probe → live API" worked end-to-end without a single hand-edit.
- **`printing-press shipcheck` umbrella with `validate-narrative --strict --full-examples` caught both narrative drift bugs before promote.** That's the safety net working — Finding F2 proposes moving the catch upstream to authoring, but the post-hoc gate did its job and prevented bad recipes from shipping.
- **`Config.Headers` + `auth set --nonce` together let the CLI handle WordPress's ~24h nonce rotation without re-entering the cookie.** Refresh path is `partitaiva24-pp-cli auth set --nonce <new>` — single value, single command. The agent didn't have to invent this UX; it followed naturally from the existing `auth set --token` pattern.
