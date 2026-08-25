# Woolworths CLI build log

Manifest transcendence rows: 6 planned, 3 built. Phase 3 will not pass until all 6 ship.

## Generation
- Spec hand-authored from live-verified endpoints (no official spec exists).
- 16 typed endpoints / 7 resources. All generator gates PASS.

## Bugs found and fixed during Phase 2/3

1. **`stores list` -> HTTP 400.** Endpoint requires Postcode OR Latitude+Longitude;
   Max/Division alone are insufficient. Spec updated with all three params and a
   working `happy_args`.
2. **Cache auto-refresh was structurally invalid for this API.** `cache.enabled: true`
   made the pre-read refresh fire a *parameterless* list call. Every list-shaped
   endpoint here requires an argument (SearchTerm / categoryId / Postcode), so the
   refresh could never succeed. Dry-run showed two requests: a bare GET (400) then the
   correct one. Cache block removed with a comment recording why.
3. **`categories browse` -> HTTP 400 "Url is required." then "FormatObject is required."**
   Found by the Phase 3 build agent, confirmed independently. Both fields are mandatory
   and undocumented anywhere in prior research; their values only drive response SEO
   metadata, so generic defaults work for every node. Spec updated. This is a headline
   command (specials browsing) and would have failed the ship threshold.

## Phase 3 progress

### Built (3/6) - history group
`real-special`, `cycle`, `specials-diff`, plus `internal/pricehist/` (pure logic, unit-tested).
build/vet/test all clean. Behavioural acceptance evidence captured:
- real-special returns NO-HISTORY (not a guess) on 1 observation; GENUINE on a true low;
  WAS-PRICE-INFLATED on a $3.00x20 history advertised as "was $6.00".
- cycle returns `[]` + honest note on zero episodes; median gap 28d / confidence medium
  on 3 seeded episodes; rejects `--data-source live` with exit 2.
- specials-diff returns `[]` + note on a single snapshot (does NOT report all members as
  entrants); exact 2 entered / 1 left / 2 stayed on two seeded snapshots.
- Live: 343 Seasonal Price members captured per snapshot, diff correct.
- Negative: Woolworths returns 1754 loosely-related products for "quantum flux biscuit";
  the relevance gate drops all of them rather than issuing verdicts.

Deviations accepted:
- Fifth verdict `ORDINARY` added so the four graded labels stay meaningful rather than
  overloading RECYCLED as a dustbin.
- `specials-diff` executes on bare invocation (no required input) instead of printing help.
- Missing-mirror guard is conditional on read-only mode for real-special/specials-diff,
  since in live/refresh mode the command creates the mirror.
- `--refresh` snapshots page-capped at 10 pages/group; larger groups set
  `item_lists_truncated` so a diff is never silently taken against a partial snapshot.

### In progress (3/6) - unit-price group
`swap`, `multibuy`, `basket` + `internal/unitprice/`.

## Phase 4 shipcheck fixes

4. **`sync` is inapplicable to this API.** Only `settings` syncs (1702 records).
   Products/categories/stores/trolley have no `list`-named endpoint or need arguments.
   `categories` was renamed tree->list and given `response_path: Categories` (it now
   fetches 25 items) but still stores 0 because its items key on `NodeId`, which the
   generator's ID extraction does not recognise and no spec field overrides.
   RETRO CANDIDATE (generator gap). Narrative, troubleshoots and the three
   missing-mirror hints were rewritten to name the real mechanism: history is recorded
   by `real-special` and `specials-diff --refresh`, not by `sync`.

5. **Cold-profile POST hang (ship blocker, fixed).** On a profile whose cookie jar has
   never seen the host, POST endpoints do not error - the HTTP/2 stream is reset with
   INTERNAL_ERROR and the HTTP/1.1 fallback hangs to timeout. Measured: cold
   `products search` = 47s FAIL (rc=5); after one GET of /shop = 926ms.
   Fixed with `internal/client/woolworths_warm.go` (hand-authored, separate file) plus a
   one-line guarded call in `client.New()`. The warm is skipped when the jar already has
   cookies for the host, so warm profiles pay nothing (887ms/848ms measured).
   Cold results after fix: products search 13.1s rc=0, categories browse 1.1s,
   real-special 1.1s, swap 1.5s, multibuy 1.2s, specials-diff 0.14s.
   RETRO CANDIDATE: the generated client has no post-construction hook, so the call had
   to go into a templated file.

6. **`auth login --chrome` / `--browser` is unusable on Windows.** Both flags fail with
   "pycookiecheat does not support Windows" and the `--browser` branch advises running
   `--browser`, i.e. it points at itself. RETRO CANDIDATE (generator template bug).
   Consequence: savedlists/pastshops cannot be exercised on this host, and
   `auth_protocol` scores 2/10.

## Shipcheck status
6/7 legs PASS (verify, validate-narrative, dogfood, workflow-verify, apify-audit,
verify-skill). scorecard HOLD on `live_api_verification` only (unverified, not failed).
Score 83/100 Grade A. Sample Output Probe 6/6 (100%), up from 2/6.

## Phase 4.9 / 4.95 review findings

Two review subagents ran against the built binary.

### Code review: 17 findings, 1 HIGH
- **HIGH — arbitrary file read reachable from MCP.** `basket <path>` opens any caller-supplied
  path and echoes every non-comment line into its output, and it is registered as an MCP tool
  whose `path` argument passes straight through. An untrusted MCP caller could point it at the
  CLI's own `cookies.json` and read `w-rctx` / `wow-auth-token` off stdout. 0600 does not help:
  the server runs as the same user. Fix dispatched: extension allowlist, data-dir rejection,
  regular-files-only, 1 MiB cap, rune-safe truncation of echoed text.
- 8 medium (zero-price poisoning local_min; multibuy printing $0.00 unit price when
  normalisation failed; package-level warmOnce meaning only the first client per process warms;
  --refresh overriding --data-source local; truncation flag conflating two conditions;
  --no-record documented but unregistered; swap silently anchoring on the wrong product for an
  unmatched stockcode; specials-diff missing pp:no-error-path-probe).
- 7 low + 1 maintainability (two parallel decode families that have already drifted on the
  availability check).
- Explicitly checked clean: no string-concatenated SQL, NULL-safe scans throughout, drain-first
  honoured, no nested write transactions, every cancel deferred, no division by zero,
  unitprice correctly refuses cross-kind comparison and never returns 0 for unparseable.

### Docs review
- Headline overclaimed "Every Woolworths feature" while the SKILL's own anti-triggers exclude
  Rewards / barcode / Coles. Narrowed at source in research.json.
- **Two runtime strings told users to "run sync"** - the exact thing established as
  non-functional here (`pricehist.go:201`, `swap.go:787`). First fixed; second pending.
- `basket --dry-run` example demonstrated nothing. Now ships a real `groceries.txt` at the repo
  root so `basket ./groceries.txt --record=false` is a genuine demonstration.
- Windows auth dead end was undisclosed. Added to auth_narrative and anti_triggers.
- Cold-start latency (~13s first call) was under-disclosed. Now a leading troubleshoot entry.
- **`specials-diff` hardcodes five specials groups; the live tree has six non-empty top-level
  groups** - it silently ignores `specialsgroup.3704` (Bundles) and any node that becomes
  non-empty later. Verified independently against the live category tree. Fix pending.
- Template-level issues recorded as RETRO CANDIDATES (generated README/SKILL, not hand-editable):
  unfilled config-path placeholder, `config.toml` vs `config.json`, missing exit code 1,
  `--idempotent` bullet with no create command, `which` documented as exiting 2 when it always
  exits 0.
