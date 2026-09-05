# Retro candidates — nccpl run 20260905-000548

## 1. `truncateJSONArray` emitted but never defined (BUILD BREAK)
- **Binary:** cli-printing-press v4.31.7
- **Trigger:** any GET endpoint whose spec declares a `limit` param.
- **Symptom:** generated endpoint command contains
  `data = truncateJSONArray(cmd.Context(), data, flagLimit)` under the comment
  `// Honor --limit when the API accepts but ignores ?limit=N.`, but no
  `func truncateJSONArray` is emitted anywhere in the tree. `go build ./...` fails with
  `undefined: truncateJSONArray`.
- **Evidence:** single call site at `internal/cli/market_latest.go:60`; zero definitions.
  `helpers.go` defines `truncate(s string, max int) string` and `isJSONArray(raw)` but not
  `truncateJSONArray`. `resource_paths.go` defines `resourceJSONArray(data)`.
- **Repro:** spec with a GET endpoint carrying `params: [- name: limit, type: int]`.
- **Scope:** hard build break for the whole module — not confined to the one command.
- **Workaround used this run:** define the helper in a hand-authored, regen-preserved file
  (`internal/cli/nccpl_truncate.go`) rather than dropping a legitimate upstream `limit` param.
  Carries a duplicate-definition risk on a future reprint if the generator starts emitting it.
- **Suspected fix:** emit `truncateJSONArray` into `helpers.go` alongside `isJSONArray`, or
  guard the call site on a helper-emitted flag.

## 2. CONFIRMED: composed-auth cookie values are not URL-decoded
- Laravel sets `XSRF-TOKEN` as a **URL-encoded** cookie value (`=` padding arrives as `%3D`).
  Laravel's `VerifyCsrfToken` decrypts `X-XSRF-TOKEN` and will fail on a still-encoded value.
  The reference implementation (`hmehmood56-debug/PSX-Trader`) explicitly calls
  `decodeURIComponent` on the cookie before sending it.
- **Confirmed by reading the generated code.** `composeAuthFromCookies` (internal/cli/auth.go:1232)
  does `strings.ReplaceAll(composed, "{"+name+"}", cookieMap[name])` with no decoding step, so the
  percent-encoded value goes straight into the header and every POST /data call would 419.
- Fix if confirmed: hand-authored `internal/client/nccpl_headers.go` (regen-preserved separate
  file) that URL-decodes before composing. Verify at Phase 5 dogfood against the live site.
- Only promote to a retro item if the generator's composed-auth path is expected to handle
  percent-encoded cookie values generically.

## 3. TWENTY generated tests fail on a clean generation (learn-loop / playbook / teach surface)
- **Binary:** cli-printing-press v4.31.7
- **Symptom:** `go test ./internal/cli/` fails **20 tests** on a freshly generated tree,
  all on the learn-loop / playbook / teach surface. Examples:
  `TestLearnNormalizers_SynonymFoldSymmetry`, `TestTeachPromotesOpenCandidate_OneArtifact`,
  `TestPlaybookInit_SeedsAllPlaybooks`, `TestPlaybookInit_ReseedReplacesNotesWithoutAmend`,
  `TestPlaybookInit_ReseedPreservesNotesWithAmend`, `TestPlaybookInit_AmendMarkerSpecificity`,
  `TestTeachPlaybook_HappyPath`, `TestTeachPlaybook_InlineJSON`, `TestTeachPlaybook_NotesOnly`,
  `TestPlaybookAmend_HappyPath_ExistingPlaybook`, `TestPlaybookAmend_EmptyFamily_CreatesNotesOnly`,
  `TestTeachCommand_PlaybookJSONInline` (`teach_test.go:1223: expected 1 playbook from inline
  JSON, got 0`).
- **CORRECTION to an earlier version of this note:** this was first filed as a single failing
  test. That was wrong — the earlier check truncated the output with `tail` and reported the
  tail as the whole. The real count is 20.
- **Reproduced in a PRISTINE tree.** Generated the same spec into a throwaway directory with
  zero hand-written code (only the `truncateJSONArray` stub needed to make the module compile)
  and the test fails identically. Not caused by any novel code in this run.
- **Impact:** `go test ./...` is a generation quality gate, so every CLI printed from this
  binary fails its own test suite out of the box on the learn-loop playbook path.
- **Scope:** appears spec-independent (the teach/playbook surface is framework code, not
  emitted from the spec), so it likely affects every print at this version.

## 4. Chrome cookie discovery uses substring LIKE, so a host-scoped `cookie_domain` misses parent-domain cookies
- **Binary:** cli-printing-press v4.31.7. Severity: low, but it silently breaks `auth login --chrome`.
- **Cause:** `discoverChromeProfiles` / `inspectCookiesForDomain` build the lookup as
  `domainPattern := "%" + strings.TrimPrefix(domain, ".") + "%"` and run
  `SELECT COUNT(*) FROM cookies WHERE host_key LIKE '<pattern>'`.
  With `cookie_domain: "www.example.com"` the pattern is `%www.example.com%`, which cannot match
  a cookie stored under `.example.com`.
- **Why it matters:** Cloudflare's `cf_clearance` is a *domain* cookie (`.example.com`) while the
  app's own session cookies are usually *host* cookies (`www.example.com`). A spec that names the
  full host therefore finds the session cookies, misses the clearance cookie, and reports
  `missing required cookies: cf_clearance` even though the cookie is present and valid.
- **Observed here:** `.nccpl.com.pk` held `cf_clearance` (valid until 2027) while
  `www.nccpl.com.pk` held `XSRF-TOKEN` and `nccpl-session`. With
  `cookie_domain: "www.nccpl.com.pk"` discovery reported the clearance cookie missing.
- **Inconsistency:** the same file already has an RFC-6265-correct matcher,
  `cookieDomainMatches` (auth.go:1187), which handles both directions
  (`candidate == target`, `candidate` suffix of `.target`, `target` suffix of `.candidate`).
  Only the SQL discovery path uses the loose substring form.
- **Suspected fix:** derive the registrable domain for the LIKE pattern, or replace the SQL
  filter with a broad fetch plus `cookieDomainMatches` in Go.
- **Workaround used this run:** set `cookie_domain` to the registrable domain `nccpl.com.pk`,
  which matches both host and dot-domain rows and still satisfies `cookieDomainMatches`.

## 5. WITHDRAWN - dogfood was right; this was an operator error, not a machine bug
- **Originally filed as:** "dogfood reports fully-implemented novel commands as TODO stubs,
  contradicting its own novel_features_check". That was wrong and the filing is withdrawn.
- **What was actually happening:** the generator emitted TODO scaffolds
  (`internal/cli/verify.go`, `coverage.go`, `panel.go`, `universe.go`, `leverage.go`,
  `risk_changes.go`, `contract_check.go`, each annotated `pp:novel-scaffold: true` and each
  returning `TODO: implement novel feature "<name>"`), and the implementations were written
  into PARALLEL files (`nccpl_verify.go` etc.) instead of replacing the scaffold bodies.
  `addNovelCommandIfAbsent` prefers a real command over a scaffold of the same name, so the
  CLI behaved correctly at runtime and every behavioural test passed -- which is exactly why
  the report looked contradictory. The scaffolds were still sitting in the tree as dead code,
  and dogfood was correctly reporting them.
- **The tell that exposed it:** after `flows`, `ingest` and `capture` were added
  post-generation, dogfood reported "7/10 novel features are TODO stubs" -- flagging exactly
  the seven that had been scaffolded at generate time and none of the three that had not.
  A blanket detector bug could not produce that split.
- **Resolution:** deleted the 7 scaffold files, their 7 scaffold `_test.go` files, and their
  `addNovelCommandIfAbsent(rootCmd, newNovel*Cmd(flags))` registrations in `root.go`.
  Dogfood issues went 4 -> 3 and the stub finding disappeared, with novel_features_check
  still 10/10.
- **Lesson for the skill, if anything:** the two documented patterns (edit the emitted
  scaffold in place vs. author a separate novel file plus a `registerNovelCommand` hook) can
  be followed simultaneously and leave orphaned scaffolds behind. A note that choosing the
  separate-file route means deleting the scaffold would have prevented this.

## 6. `validateComposedAuth` uses stdlib HTTP, so `auth login --chrome` can never succeed on a browser-clearance CLI
- **Binary:** cli-printing-press v4.31.7. Severity: HIGH — blocks the entire cookie/composed auth path.
- **Code:** `internal/cli/auth.go:1258 validateComposedAuth` builds `client := &http.Client{Timeout: 5s}`
  (line 1279) and maps `403` to `fmt.Errorf("HTTP %d")`, which the caller renders as
  "Found cookies but the session has expired."
- **Why that is always wrong for this CLI shape:** the spec declares
  `http_transport: browser-chrome` *precisely because* stdlib HTTP cannot pass the origin's
  bot protection. Worse, a `cf_clearance` cookie is bound to the TLS fingerprint that earned
  it, so presenting it from stdlib is treated as token abuse and answered with a hard
  "Sorry, you have been blocked" page rather than a challenge.
- **Measured during this run:** with the identical cookie jar, stdlib returns
  403 + "Sorry, you have been blocked"; an unauthenticated `probe-reachability` of the same
  origin returns 403 + `cf-mitigated: challenge`. So the hard block is caused by the
  fingerprint/cookie mismatch, not by an expired session. The cookies were valid throughout
  (`cf_clearance` valid to 2027, session cookie ~30 min of life remaining).
- **User-visible effect:** `auth login --chrome` extracted all three cookies correctly and then
  refused to save them, reporting "session expired" for a perfectly good session. There is no
  flag to skip validation, so the CLI is unusable without patching it.
- **Suspected fix:** validate through the same transport the CLI ships (Surf when
  `http_transport` is browser-*), or skip the probe entirely when the credential contains a
  clearance cookie, or treat a 403 as inconclusive rather than as expiry.
- **Workaround used this run:** `internal/cli/nccpl_auth_validate.go` +
  a guard in `validateComposedAuth` that returns nil when the cookie header contains
  `cf_clearance=`. After that, login succeeds and persists all three cookies.

## 7. Reachability finding (not a machine bug): Surf cannot replay NCCPL's Cloudflare clearance
- After the login fix, all three cookies are persisted and **verified on the wire**: a jar probe
  for `https://www.nccpl.com.pk/api/fipi/latest-date` returns `cf_clearance` (533 bytes),
  `nccpl-session` (342) and `XSRF-TOKEN` (342).
- Every live request still returns `403` + `Just a moment...` (a Cloudflare challenge, not the
  hard block). Cookies reach the wire and are rejected.
- Conclusion: this origin binds `cf_clearance` more tightly than Surf's `Impersonate().Chrome()`
  reproduces. `browser_clearance_http` replay does not work here.
- `press-auth` does not address this: it solves cookie *capture* (RAM-only session cookies),
  which already worked. The rejection is at replay time.

## 8. FINDING (not a bug): a fully replayable FIPI/LIPI surface exists at scstrade
- `POST https://www.scstrade.com/FIPILIPI.aspx/loadfipisector` with
  `{"date1":"MM/DD/YYYY","date2":"MM/DD/YYYY"}` returns clean JSON over plain HTTPS.
  No Cloudflare, no auth, no cookies, no browser.
- Fields: `FLSectorName`, `FLTypeNew` (investor class), `FLBuyVolume`, `FLBuyValue`,
  `FLSellVolume`, `FLSellValue`, `FLNetValueUSD` (signed, mn$).
- **Both NCCPL invariants hold exactly** on 2026-09-04: all 11 sector rows net to zero
  across investor classes (0 failing at tolerance 0.05), and FIPI net = -LIPI net with
  residual -0.0000. Strong evidence it is NCCPL's data republished, not re-derived.
- Archive starts ~**2016-08-01**; `~/psx-research/data/research.db daily_bars` starts
  **2016-08-22**, so the flow history fully spans the existing price panel.
- **Trap:** the sibling `loadmain` PageMethod returns values that sum to exactly 100.000 --
  they are PERCENTAGE SHARES, not net flows, despite the field being named `NetValue`.
  Using it as a flow series would silently corrupt a regression.
- **Does NOT cover** MTS/MFS/MSF/SLB open positions, VAR margins, free float, or
  settlement. Those remain NCCPL-only and still blocked by the clearance wall.
