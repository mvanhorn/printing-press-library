# Printing Press Retro: cdc-pakistan

## Session Stats
- API: cdc-pakistan (Central Depository Company of Pakistan — Cloudflare-gated HTML + PDF, no JSON API)
- Spec source: internal YAML, `http_transport: standard`, `auth.type: cookie`
- Printing Press: **4.31.7, LOCALLY PATCHED** with press-fix-4539.diff (sha256 39a10b8d…)
- Scorecard **80/100 Grade A**; verify **100% (34/34, 0 critical)**; dogfood WARN
- Live dogfood: **107/107, verdict PASS, level full**, `coverage_hollow` ABSENT, 191 files fingerprinted
- 8 novel features + `identity resolve`; 4,879 hand-written lines; 15 tests
- Outcome: **PROMOTED** to `library/cdc-pakistan`
- Retro method: 10 candidates in, **1 filed**. Nine died at triage or on reproduction.

## Findings

### F1. dogfood's config-consistency check reports REGEX CAPTURES as field names (Scorer bug)
Component: `scorer`. Type: `bug`. Priority: **P2**.

On every cookie-auth CLI, dogfood emits an issue naming two "fields" that are not fields:

    config inconsistency: write fields [, token, ]
                    vs read fields [ || c.CookieCredential() != ]

`", token, "` and `" || c.CookieCredential() != "` are fragments captured by the check's own
regex, not identifiers. The finding is therefore permanently unactionable: there is no field
to reconcile.

**Reproduced independently on two CLIs**, so it is not a per-CLI quirk:

| CLI | auth type | write_fields | read_fields |
|---|---|---|---|
| cdc-pakistan | `cookie` | `[", token, "]` | `[" \|\| c.CookieCredential() != "]` |
| nccpl | `composed` | `[", token, "]` | `[" \|\| c.CookieCredential() != ", " \|\| strings.TrimSpace(cfg.CookieCredential()) != "]` |

Why it matters beyond cosmetics: it contributes a permanent WARN item to every cookie-auth
CLI's dogfood output, which trains operators to skim past dogfood findings. A check that can
never be satisfied is worse than no check.

Fix direction: the config-consistency extractor should capture identifier tokens, not the
surrounding expression text; and when it cannot resolve an identifier it should stay silent
rather than emit the raw capture. Verification: a cookie-auth CLI reports either real field
names or no config-consistency issue at all.

## Prioritized Improvements

### P1 — High priority
None. No finding leaves a printed CLI broken, unsafe, or emitting silently wrong data.

### P2 — Medium priority
- F1 config-consistency emits regex captures as field names (`comp:scorer`, `bug`)

### Comment on existing issues (recurrence evidence, NOT new tickets)
- **#4612** (doctor calls a healthy HTML-200 site "unreachable") — **RECURRED exactly as the
  MUFAP retro predicted.** CDC serves HTML at `/`, so `doctor` needed the same hand-written
  health-check workaround (`GetWithHeaders` + `client.HTMLResponseHeader`) that MUFAP wrote.
  Third CLI to need it. This is the "latent-on-reprint" case that retro flagged, now realised
  on a *new* print rather than a reprint.
- **#4614** (collapsed single-endpoint commands ship a stale two-word Example) — **RECURRED,
  and confirms that retro's own root-cause correction.** All three of this CLI's collapsed
  commands shipped a broken Example (`downloads list …`, `statistics get`, `assets fetch …`)
  because `promotedExampleLine` echoes the spec author's `example:` VERBATIM. Two of them were
  additionally wrong on flag names because `flag_name` renames were not reflected. The
  detect-and-refuse fix that issue proposes would have caught all three at generate time.

### Skip (recorded, deliberately not filed)
- **Generated `printJSONFiltered` never calls `SetEscapeHTML(false)`**, so all **10** library
  CLIs HTML-escape `&`, `<`, `>` in JSON. Real and fleet-wide — but the output is VALID JSON:
  both `python json.load` and `jq` decode `&` back to `&` correctly. It is Go's documented
  default producing conformant output, and harms only a human reading raw bytes. This was
  initially written up as the headline fleet-wide finding; testing the consumer path
  disproved that framing.
- **`command_mirror_capabilities` truncates at a clause boundary with no ellipsis** — left one
  MCP description dangling on "…its own share and". Single observation, single CLI, and
  `dogfood --overwrite-command-mirror` already fixes it. A sibling scan found no other case
  (and the regex that "found" one in mufap was matching `or` inside "sect**or**").
- **novel-feature depth resolver ambiguates duplicate leaf names** — reported `stats history`
  as "registered as learnings stats". Both a top-level `stats` and a real `learnings stats`
  exist. Adjacent to psx WU-3 but a different cause; belongs as evidence on WU-3, not a
  third ticket in the same walker family.
- **`pp:typed-exit-codes` honoured by the live matrix but not by scorecard's sample probe.**
  Cannot be filed responsibly: this run had no press source checkout (`IN_REPO=false`), so the
  scorer's logic could not be traced, and Phase 2f forbids proposing a scorer fix from a guess.

### Dropped at triage
- **52 gosec findings in generated files** (G202 in `internal/platform/migration.go`, which is
  byte-identical across mufap/nccpl/psx; G112 in `cmd/*-pp-mcp`) — triage Q6 names lint/gosec
  hygiene as a weekday-triage close.
- **Generated dead code** (`maxAge` persistent flag in `root.go`, `successfulNoop` in
  `helpers.go`) — same Q6 hygiene class; no gate impact, dogfood WARN does not fail shipcheck.
- **dogfood dead-function false positive on `isDryRunResponseForClient`** — DID NOT
  GENERALISE. dogfood reports `dead=0` on nccpl and psx despite identical 3-call-site counts,
  so the FP fired only here and the mechanism is untraceable without press source. Single
  observation → drop.
- **`scorecard --live-check` SIGBUS / "database disk image is malformed (11)"** — the MUFAP
  retro already DROPPED a scorecard SIGBUS as experimentally falsified, and polish reproduced
  integrity-ok across 32 concurrent invocations in 4 rounds. Not reproducible standalone here
  either. Re-opening a closed-as-falsified finding on unreproducible evidence is exactly what
  that retro warned against.

## Anti-patterns

- **Trusting a credential's self-declared expiry instead of measuring it.** The cf_clearance
  cookie's `expires` attribute claims 365 days; its measured server-side life is a hard ~30
  minutes. Acting on the declared value produced a wrong design ("mint once, use for a year")
  and required two separate corrections.
- **Generalising a trap from a two-point sample.** "The year filter silently under-reports"
  was recorded as a verified trap after probing exactly two years of one category. The full
  300-pair sweep showed the files sat under a third year. The filter was fine.
- **Comparing two columns without confirming what they hold.** A generic row regex reported
  "154 invariant violations" by comparing a paid-up-CAPITAL column against a PERCENTAGE
  column. The real violation count is zero across 29,060 rows.
- **Substring matching on taxonomy labels.** "Securities-Unlisted" contains "listed", so it
  collapsed onto the listed key and silently lost a 59,217 count. "…Number of Securities under
  Share Registrar" contains "number of securities" and was keyed as a securities total.
- **Verification harnesses that fail open.** Four separate harnesses in this session reported
  failures against working code: zsh not word-splitting an unquoted `$p` (twice), a `sed`
  range that matched nothing, and a `grep -A6` that truncated a command list and "proved" a
  command did not exist. A harness that can produce a false FAIL is not a verification.
- **`str.replace` with no assertion.** A fix was reported as applied when the search string
  did not match and the call silently no-op'd. Patches must assert their anchor.

## What the Printing Press Got Right

- **The patched `coverage_hollow` gate did exactly its job.** It refused to certify
  `stats history` as covered, and it was RIGHT: the feature's `pp:happy-args` named a flag the
  command did not declare, so its happy path exited 2 and the write path never ran. The gate
  caught a genuinely broken invocation, not a technicality. Declaring the feature at an
  executable leaf plus fixing the args cleared it honestly.
- **The source fingerprint is load-bearing and works.** 190→191 files hashed; every post-
  acceptance edit correctly invalidated the marker and forced a re-earn. Polish independently
  built a verifier against it and confirmed 190/190 intact.
- **`pp:happy-args` carrying `--timeout=30m` works**, and the harness re-emits it as separate
  argv tokens exactly as the MUFAP retro documented — matching `--timeout[= ]30m` was the
  right gate.
- **The live matrix is cheap.** A full 107-test live dogfood completes in well under a minute
  and was re-run four times in one session.
- **`generate --force` preserved every hand-authored file** across two regenerations, exactly
  as the hand-edit durability rules promise. New packages (`internal/cdcpdf`,
  `internal/cdcparse`) and separate-file extensions (`store/cdc_migrations.go`,
  `cli/cdc_*.go`) all survived untouched.

## Filed (7 Sep 2026, owner-approved)
- **F1** config-consistency emits regex captures as field names →
  https://github.com/mvanhorn/cli-printing-press/issues/4626
- **#4612** doctor HTML-200 recurrence (third CLI to need a bespoke health check; the original
  write-up's "any NEW CLI with an HTML health path breaks on first print" prediction realised) →
  https://github.com/mvanhorn/cli-printing-press/issues/4612#issuecomment-5574273679
- **#4614** collapsed-path stale Example recurrence (all 3 collapsed commands affected; two
  ALSO wrong on flag names because `flag_name` renames were not reflected, so a path-only
  token check would still pass them) →
  https://github.com/mvanhorn/cli-printing-press/issues/4614#issuecomment-5574273676

Labels could not be applied (filer has pull-only permission on the repo, as in the MUFAP
retro); type/component/priority are stated in the issue body instead.

Dedup scan before filing surfaced #695 ("Auth template emission disagrees with config emission
for security-scheme-silent specs", CLOSED/COMPLETED). Read in full and rejected as a duplicate:
that is a GENERATOR auth-template-selection defect; F1 is a SCORER extraction defect. Different
component, different symptom.
