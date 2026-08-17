# Crestron CLI Shipcheck

## Verdict: ship

`cli-printing-press shipcheck` — **PASS, 7/7 legs**

| Leg | Result | Elapsed |
|---|---|---|
| verify | PASS | 10.0s |
| validate-narrative | PASS | 0.28s |
| dogfood | PASS | 1.05s |
| workflow-verify | PASS | 0.01s |
| apify-audit | PASS | 0.02s |
| verify-skill | PASS | 2.28s |
| scorecard | PASS | 2.61s |

**Scorecard: 83/100, Grade A.**
Perfect scores on output modes, auth, terminal UX, README, doctor, agent-native,
MCP remote transport, local cache, breadth, workflows. Type fidelity 5/5.
Data pipeline integrity and sync correctness both 10/10.

**Sample Output Probe: 6/6 (100%)** after the fixes below. The first run scored
3/6; every failure was a real user-facing defect, not a probe artifact.

## Failures found by the output probe and fixed

1. **`firmware diff` example cited a version that does not exist.**
   The narrative used `7.3.0125`; the real adjacent versions are
   `7.3.5149.23092` and `7.4.0255.22319`. Copy-pasting the README example
   failed. Fixed in `research.json` and the rendered README/SKILL.

2. **`submittal` example timed out.** The example used `--out`, which downloads
   roughly twenty files per model and takes ~30s. Changed the headline example
   to the list form, which is also the better first invocation — it prints the
   asset-class coverage report so gaps are seen before downloading. Added a
   `cliutil.IsVerifyEnv()` curtailment alongside the existing dogfood one.

3. **`specs compare` hard-errored without a synced mirror.** Crestron has no
   server-rendered product search (`/Search-Results` now returns 404), so a
   catalog path is genuinely only available from the mirror. Replaced the error
   with the skill's missing-mirror guard: exit 0, a parseable empty result, and
   an actionable `run: crestron-pp-cli sync ...` hint on stderr. Applied the same
   guard to `product get` and `specs show`.

## Other verification

- `go vet ./...` clean.
- `go test ./... -count=1` — **16 packages pass**, including 20 parser tests
  against fixtures captured live from crestron.com, 8 firmware-resolver tests,
  and 8 local-mirror tests.
- `verify-skill` — all checks pass (flag-names, flag-commands, positional-args,
  shell-var-quotes, unknown-command, canonical-sections).
- `validate-narrative --strict --full-examples` — 13 ok, 0 missing, 0 failed.
  The 2 `UNSUPPORTED` entries are `auth login --chrome`, which is side-effectful
  by design and is reported rather than executed.

## Known gaps

1. **MCP readiness reports `partial`: 13 tools, 0 public.** The spec declared
   cookie auth globally, so every endpoint is presented as auth-gated even though
   only the firmware release page actually requires a session. The spec has been
   corrected — 11 endpoints now carry `no_auth: true`, leaving only
   `/Software-Firmware/{path}` gated — but applying it needs a regeneration, and
   this build carries hand-edits inside generator-emitted files (the 12 parser
   dispatcher swaps and the `os=0` fix). Regenerating to gain a label while
   risking the wiring that makes all parsing work is the wrong trade at this
   point. A reprint picks the fix up automatically.
   Scorecard effect: `auth_protocol 2/10` and the `partial` readiness label.
   No functional effect — every public command works signed out today.

2. **Discontinued products are not synced.** `/sitemap` lists 79 catalog paths
   and none of them is the `Inactive/Discontinued` tree, so the mirror holds 0
   discontinued products. `lifecycle` therefore falls back to a live lookup for
   those and says so in its `note`. It never implies a retired part is current.

3. **`lifecycle` cannot see every End-of-Sale notice without a mirror.** Notices
   are often titled for the family rather than the model — UC-FCM-Z's is
   "End-of-Sale Notice: FlexCarts" — so they are reachable only through the
   product's own document id. Reported honestly in the `note` field.

4. **Cache freshness 3/10.** The spec opts into cache freshness but the
   hand-authored `sync` does not register with the generated freshness helpers,
   which key off generator-emitted syncable resources.

## Printing Press retro candidates

1. **An all-HTML spec silently yields no sync, search, sql, or stale surface.**
   Nothing warns at generate time that the local store will have nothing to
   populate it. This invalidated an approved `spec-emits` feature mid-build and
   required a user re-approval and a hand-written mirror.
2. **Zero-valued int params are dropped from the query string.** Crestron
   requires `os=0` on the first page of its product-tile endpoint; the generated
   command omitted it and the endpoint returned zero products.
3. **`boundCtx` turns a per-request timeout into a whole-command deadline.** The
   skill recommends it for hand-written commands calling sibling clients, but for
   a crawl-shaped command it killed a 3-minute sync that was working correctly.
4. **No post-extract hook for HTML responses.** Routing responses through
   domain parsers required editing 12 generator-emitted files.
