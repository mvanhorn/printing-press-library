# Printing Press Retro: semrush

## Session Stats
- API: semrush
- Spec source: hand-written internal YAML (HAR-derived auto-spec was unusable; see findings 2 & 3)
- Scorecard: 76/100 (per polish run)
- Verify pass rate: 100%
- Fix loops: ~2 (off-by-one patch + domain_overview type fix)
- Manual code edits: ~14 (9 off-by-one patches, 1 type default, 4 Phase-3 hand-built features: cookie auth, KMT-RPC, sheets, research)
- Features built from scratch: 5 (auth login --chrome, keyword-magic JSON-RPC, sheets push/info, auth google OAuth, research workflow)

## Findings

### F1. Off-by-one in positional arg parsing when `path` has no template params (Bug)
- **What happened:** Every generated command with `path: "/"` and a required positional param emitted `args[1]` and `len(args) < 2` instead of `args[0]` and `len(args) < 1`. Calling `semrush-pp-cli domain overview semrush.com` returned `Error: domain is required` until manually patched in 9 files.
- **Scorer correct?** N/A — not a scorer finding.
- **Root cause:** `internal/generator/` positional-arg template appears to reserve `args[0]` for a path template even when the path has no templates to consume. The index should be derived from `path.templateParams.length`, not a constant.
- **Cross-API check (Step A):** Affects any spec where one or more resources have `path: "/"` (root path) with required positional params — i.e., report-style APIs that pass everything via query string.
- **Step B (3 APIs with evidence):**
  1. **SEMrush** — confirmed; 9 commands affected (`domain overview`, `domain organic-keywords`, `domain organic-pages`, `domain organic-competitors`, `keyword overview`, `keyword related`, `keyword questions`, `keyword difficulty`, `backlinks overview`).
  2. **Moz Links API** — same shape; reports passed as `?Operations.GenerateQueriesForSubject.Items[0].Subject=...` with root `/v2/`. (Speculation; not confirmed against a generated CLI.)
  3. **Adobe Analytics 1.4 REST** — legacy report API with `?method=Report.Run` against a single endpoint. (Speculation.)
  - **Honest:** Only SEMrush is confirmed with hard evidence. APIs 2 and 3 are plausible but unverified.
- **Step C (counter-check):** Would the fix hurt APIs with proper path templates? No — `args[len(pathTemplates)]` is correct in both cases.
- **Step D (recurrence):** First retro on this system; no prior.
- **Step E (fallback cost):** Claude catches the symptom on first run (`Error: <param> is required` with a clear usage line), but the fix is mechanical and Claude has to remember to apply across all affected files. Reliable when caught; ~10-15% chance of missing one file on a 20+ command CLI.
- **Step F (tradeoff):** Step B is weak — only 1 confirmed API. Step C is clean. Step D N/A. **The case-against is real.**
- **Step G (case against):** "This is a niche spec pattern. Most modern APIs have proper path structure (`/v1/<resource>/<id>`); the user could have hand-edited their spec to use `path: "/{type}"` instead. The fact that I had to write the spec by hand because the auto-detected one was junk means there's no good reason to keep `path: "/"`." — This case has merit. Survives because: the bug is unambiguous, the fix is small, and the alternative (telling users to restructure their spec) is worse UX than fixing the template.
- **Inherent or fixable:** Fixable. Generator template change.
- **Durable fix:** In the generator's command-emit template, compute the positional-arg index as `args[len(pathTemplateParams)]` rather than hardcoded `args[1]` (or wherever the constant lives). Validate `len(args) >= len(pathTemplateParams) + 1`.
- **Test:** Positive — generate from a spec where `path: "/"` and a positional param exists; the binary returns the param value when called with one positional arg. Negative — generate from a spec where `path: "/v1/{id}"` and a separate positional param; the binary still correctly maps `args[0]` to `{id}` and `args[1]` to the named param.
- **Evidence:** `/Users/rafa/printing-press/manuscripts/semrush/20260512-201920/research/semrush-spec.yaml` (the input spec) and `/Users/rafa/printing-press/library/semrush/internal/cli/domain_overview.go` (now-patched output).
- **Related prior retros:** None (first retro on this system).

### F2. Browser-sniff name & auth auto-detection picks 3rd-party host noise (Bug)
- **What happened:** Running `printing-press browser-sniff --har <semrush-capture.har>` against a 64MB HAR from a logged-in SEMrush session produced:
  - `description: "Discovered API spec for streaming-bi-owox"` (a TikTok-related host)
  - `auth.header: "sentry_key"` (a Sentry error-reporting header)
  - `env_vars: [STREAMING_BI_OWOX_API_KEY]` (nonsensical)
  The auto-generated YAML had 62 endpoints across 33 resources, of which only ~5 were actual SEMrush endpoints; the rest were CookieHub, GTM, Sentry, TikTok pixels, Zoominfo, etc.
- **Scorer correct?** N/A.
- **Root cause:** The browser-sniff analyzer treats every distinct host in the HAR as a candidate API, picks the one with the most requests, and infers auth from observed headers. Modern SaaS sites load 30+ 3rd-party scripts; the noise often outranks first-party endpoints by request count and the noise's "API key" headers (Sentry's `sentry_key`, TikTok's various pixel keys) get falsely detected as auth.
- **Cross-API check (Step A):** Any HAR-mode generation against a modern SaaS site that loads Sentry, GTM, CookieHub, Intercom, TikTok, Snowflake, or similar 3rd-party scripts. Essentially every public SaaS in 2026.
- **Step B (3 APIs with evidence):**
  1. **SEMrush** — confirmed; auto-name = `streaming-bi-owox`, auto-auth header = `sentry_key`.
  2. **Any future HAR-mode capture against any modern SaaS** — Stripe Dashboard, Notion app, Linear app, HubSpot, Salesforce Lightning all load Sentry/GTM/equivalent. Not run to confirm but the heuristic would misfire identically.
  3. **Any logged-in vendor portal** — AWS console, GCP console, Azure portal all load 10+ tracking/error-reporting scripts.
  - **Honest:** Only SEMrush is confirmed. APIs 2-3 are well-grounded extrapolation but not verified.
- **Step C (counter-check):** Would tightening the heuristic hurt legitimate multi-host APIs? Possibly — APIs that genuinely span subdomains (e.g., `api.example.com` + `cdn.example.com` + `webhooks.example.com`) would lose endpoints if we filter too aggressively. **Guard required:** use the input HAR's primary navigation URL (from the first `text/html` entry with status 200) to identify the first-party domain, then accept endpoints on that domain or its subdomains.
- **Step D:** None.
- **Step E (fallback cost):** Claude catches this quickly (the auto-spec is obviously garbage on inspection) and falls back to either using a different spec or hand-writing one. But the time cost is real (~15-30 min of manual triage and spec writing per HAR-mode run).
- **Step F:** Step B has 1 confirmed, 2 well-grounded; Step C surfaces a guard requirement (first-party domain anchoring) which is reasonable. The case-for holds.
- **Step G (case against):** "Browser-sniff is intentionally reverse-engineering tooling; the user is expected to review and edit the output. A maintainer could close this as 'works as designed; users should review HAR-derived specs.'" — Has merit, but the auto-spec was *so* unusable (33 resources, ~5 real) that no one would have used it. The defaults aren't producing a 'starting point' so much as a write-off.
- **Inherent or fixable:** Fixable. Anchor on first-party domain inferred from the HAR's primary navigation URL.
- **Durable fix:** In `printing-press browser-sniff`:
  1. From the HAR, find the first `Content-Type: text/html; charset=*` response with status 200 — that's the user-loaded page. Its host is the first-party domain.
  2. Filter endpoint candidates to: the first-party domain + its subdomains. Drop everything else.
  3. Auth detection: only consider headers on first-party endpoints; ignore Sentry/3rd-party error-reporting auth tokens entirely.
  4. Name: derive from the first-party domain (`semrush.com` → `semrush`).
- **Test:** Positive — feed a SEMrush HAR, expect `name: semrush`, auth header from a SEMrush endpoint, no `streaming-bi-owox` artifacts. Negative — feed an API-only HAR (no HTML page load), fall back to current behavior with a clear "no primary domain detected" log.
- **Evidence:** `/Users/rafa/printing-press/manuscripts/semrush/20260512-201920/research/semrush-browser-sniff-spec.yaml` lines 1-15 show the garbage; `traffic-analysis.json` shows endpoint_clusters dominated by `analytics.tiktok.com`, `cookiehub.net`, `sentry.semrush.net`.
- **Related prior retros:** None.

### F3. Reachability classifier false-positives on CookieHub / GTM / Sentry "CAPTCHA markers" (Bug)
- **What happened:** `traffic-analysis.json` from the SEMrush HAR reported `reachability.mode: browser_required` with `confidence: 0.9` and reason `"CAPTCHA challenge observed"`. The "evidence" was:
  - `/__static__/webpack/toolkit.ae465b5a63804c46.js` (a SEMrush JS bundle — not a CAPTCHA)
  - `/gtm.js` (Google Tag Manager — not a CAPTCHA)
  - `/c2/06c77e2e.js` (CookieHub consent banner — not a CAPTCHA)
  Meanwhile the underlying SEMrush API was fully reachable via standard HTTP with `?key=...` query auth (verified across `domain overview`, `keyword overview`, `pt campaigns`, etc.).
- **Scorer correct?** N/A — this is a classification bug in browser-sniff/probe, not a scorer.
- **Root cause:** The classifier appears to substring-match against JS bundle/path names looking for "captcha"-like strings. Common 3rd-party bundles (`toolkit.js`, `gtm.js`, CookieHub client JS) contain markers that trigger the heuristic without any actual challenge response.
- **Cross-API check (Step A):** Any HAR against a modern SaaS that loads GTM, CookieHub, OneTrust, Cookiebot, Sentry, or webpack-named bundles starting with common substrings.
- **Step B (3 APIs with evidence):**
  1. **SEMrush** — confirmed with hard evidence in `traffic-analysis.json`.
  2. **Any GTM-using site** — google-analytics.com requests serve `gtm.js`; any HAR including a tracked page will include it. Not confirmed against another concrete API but the heuristic is deterministic.
  3. **Any CookieHub/OneTrust customer** — same.
- **Step C (counter-check):** Would tightening this hurt legitimate CAPTCHA detection? The legitimate signal is a *response body* containing "Just a moment", `cf_chl_opt`, etc. — not a JS bundle URL. Tightening to response-body matching only would actually improve accuracy.
- **Step D:** None.
- **Step E (fallback cost):** Cascades into bad transport choices. If a CLI ships with `browser_clearance_http` transport because the classifier said `browser_required`, it carries Chrome cookie import machinery for an API that doesn't need it. The user catches this when commands work fine with plain HTTP, but only after the downstream cost.
- **Step F:** Step B has 1 hard + 2 deterministic. Step C is clean (the fix narrows, doesn't broaden). Survives.
- **Step G (case against):** "The classifier is meant to be conservative — false positives toward `browser_required` are safer than false negatives. Better to over-recommend Surf transport than to ship a CLI that gets 403'd at runtime." — Has merit, but a `confidence: 0.9` false positive is too strong a signal. The classifier should either (a) not match on URL substring at all, or (b) report `confidence < 0.3` when evidence is JS-bundle-only.
- **Inherent or fixable:** Fixable.
- **Durable fix:** In the reachability classifier, only match CAPTCHA/challenge markers against:
  - Response body content (`Just a moment`, `cf_chl_opt`, `access denied`, etc.)
  - HTTP status codes 403/429 from first-party paths (not third-party CDNs)
  - Specific header patterns (`cf-mitigated: challenge`, `x-vercel-mitigated: challenge`)
  Drop URL-substring matching against JS bundle names entirely.
- **Test:** Positive — feed a HAR with a real Cloudflare challenge response (status 403, `<title>Just a moment...</title>` in body); expect `browser_clearance_http` mode. Negative — feed a SEMrush-shaped HAR (status 200 for every entry, no challenge body, but `gtm.js` and CookieHub present); expect `standard_http` mode with high confidence.
- **Evidence:** `/Users/rafa/printing-press/manuscripts/semrush/20260512-201920/discovery/traffic-analysis.json`.
- **Related prior retros:** None.

### F4. `profile save` cannot capture subcommand-specific flags (Default gap)
- **What happened:** Running `semrush-pp-cli profile save kram-template --columns "..." --tab "..." --header=false` returned `Error: unknown flag: --columns`. The profile system only recognizes root-level (persistent) flags. To preset subcommand-specific flags (which is the most useful case — `sheets push --columns`, `pt report --date-begin`, etc.), the user must use shell aliases instead.
- **Scorer correct?** N/A.
- **Root cause:** The `profile save` command's flag parser registers only persistent flags from the root command, not flags declared on individual subcommands.
- **Cross-API check (Step A):** Affects every CLI generated by the Printing Press. Every generated CLI has subcommand-specific flags (resource filters, output options, etc.) that users naturally want to preset.
- **Step B (3 APIs with evidence):**
  1. **SEMrush** — confirmed; `--columns`, `--tab`, `--header` on `sheets push` are not profile-able.
  2. **Stripe (catalog)** — would have the same issue: `--limit`, `--starting-after`, `--expand` on subcommands like `customers list`.
  3. **GitHub (catalog)** — same: `--state`, `--labels`, `--per-page` on `issues list`.
  - This is the strongest Step B set in the retro — every CLI is affected.
- **Step C (counter-check):** Would adding subcommand-flag capture break anything? Risk: name collisions between a flag declared on multiple subcommands. The fix needs to capture the flag *qualified by its subcommand path* (`sheets.push.columns`), not as a bare name.
- **Step D:** None.
- **Step E (fallback cost):** Workaround is a shell alias (1 line in `~/.zshrc`). Reliable, but per-CLI per-user — every user of every generated CLI hits the same wall and writes the same alias. The "raises the floor" cost is multiplicative.
- **Step F:** Strong on all three checks. Survives.
- **Step G (case against):** "Profiles are documented as root-flag-only; subcommand presets aren't a stated feature. Users have shell aliases. This is a feature request, not a bug." — Weak. The CLI is presented as the agent-friendly preset system; failing to support the most common use case (subcommand flag presets) is a real gap.
- **Inherent or fixable:** Fixable. Generator + binary change.
- **Durable fix:** In the generated `profile` command:
  1. Walk the full command tree (not just root) when parsing flags during `profile save`.
  2. Store captured flags qualified by command path: `{"sheets.push.columns": "...", "pt.report.date-begin": "..."}`.
  3. When `--profile <name>` is applied to a subcommand invocation, look up the matching qualified key first, then fall back to root-level for unqualified flags.
- **Test:** Positive — `semrush-pp-cli profile save kram --columns x,y,z`, then `semrush-pp-cli sheets push <id> --profile kram` resolves `--columns x,y,z`. Negative — profile saved on `sheets push --columns` does not apply when running `pt report --columns` (different subcommand, no resolution; explicit `--columns` on the invocation wins anyway).
- **Evidence:** Live shell error reproduced in this session: `Error: unknown flag: --columns` from `profile save`.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
*(none — see Skip table for what got considered)*

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Off-by-one positional args when path has no template params | generator | subclass: report-style APIs with `path: "/"` | High (Claude catches on first run; mechanical fix) | small | None needed; fix is `args[len(pathTemplates)]` |
| F2 | browser-sniff auto-detects name/auth from 3rd-party host noise | spec-parser (browser-sniff command) | every HAR-mode capture against modern SaaS | Low (auto-spec was completely unusable; Claude fell back to hand-writing) | medium | Anchor on first-party domain from HAR's primary HTML response |
| F4 | `profile save` cannot capture subcommand-specific flags | generator | every CLI | Medium (shell aliases work but compound across users) | medium | Capture flags qualified by command path |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | Reachability classifier false-positives on JS bundle URLs | spec-parser (browser-sniff command) | every HAR with GTM/CookieHub/etc. | Medium (cascades into bad transport choice) | small | Match challenge markers against response bodies/status/headers only, not URL substrings |

### Skip
*(no findings landed in Skip — all Phase 3 candidates either survived to Do or were dropped at Phase 2.5 triage)*

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| `kooky` library missed Chrome cookie path on macOS | Newer Chrome layout has `Default/Network/Cookies`, older has `Default/Cookies`; kooky's auto-finder only tried the new layout | API-quirk (third-party library issue, not Printing Press) |
| `kooky` returned wrong Chrome profile's cookies | Default profile had only tracking cookies; the SEMrush session was in Profile 1 — required heuristic across all profiles | printed-CLI (CLI-specific Chrome integration, not generator concern) |
| SEMrush UI uses JSON-RPC proxy-envelope to `/kmtgw/v2/webapi` | Custom transport, hand-built in Phase 3 | printed-CLI (per-API custom transport; SKILL recipe could cover proxy-envelope pattern but not generator) |
| CSV-default response wrapped raw in `results: "..."` JSON envelope | SEMrush public API returns semicolon-CSV; we wrote a SEMrush-specific converter | unproven-one-off (CSV-default APIs are mostly legacy enterprise; couldn't name 3 catalog examples with evidence) |
| Generator `Short:` field truncated mid-word (saw "in Go…" mid-sentence) | Cosmetic in `root.go` — the full description exists in `cli_description` | printed-CLI (cosmetic, per-CLI hand-tweak is normal) |
| spec auto-generated `description: "Discovered API spec for streaming-bi-owox"` propagated to root.go even after `--name semrush` override | Description field wasn't re-derived when name was overridden | iteration-noise (single edit, low recurrence) |
| Default `type` for `domain overview` was wrong (`domain_overview` rejected; should be `domain_ranks`) | Hand-written spec error, not a generator bug | printed-CLI (the user authored the spec; SEMrush's API has confusing report names) |
| Google OAuth code didn't initially handle `127.0.0.1` vs `localhost` redirect URI mismatch | I wrote the OAuth code by hand in Phase 3; this is my bug, not the generator's | printed-CLI (hand-written code in a Phase 3 transcendence feature) |

## Work Units

### WU-1: Fix positional-arg off-by-one when path has no template params (from F1)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generated commands with `path: "/"` and required positional params correctly map `args[0]` to the first param, not `args[1]`.
- **Target:** Generator command-emit templates in `internal/generator/` — find the positional-arg lookup that emits `args[N]` and the matching `len(args) < N+1` guard.
- **Acceptance criteria:**
  - **Positive:** generating from a spec with `resources.domain.endpoints.overview` having `path: "/"` and a required positional param `domain` produces code that maps `args[0]` to `params["domain"]` and checks `len(args) < 1`.
  - **Negative:** generating from a spec with `path: "/v1/{id}"` and a separate positional param `name` still maps `{id}` to `args[0]` and `name` to `args[1]`.
- **Scope boundary:** Does NOT touch path-template *parsing* (assumed correct). Only the index arithmetic in the command-emit template.
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Anchor browser-sniff on first-party domain to filter 3rd-party noise (from F2)
- **Priority:** P2
- **Component:** spec-parser
- **Goal:** `printing-press browser-sniff` against a modern-SaaS HAR produces a clean spec rooted on the captured site's first-party domain, not 3rd-party tracking hosts.
- **Target:** `internal/spec/` browser-sniff command — endpoint clustering, name derivation, auth detection.
- **Acceptance criteria:**
  - **Positive:** feeding a SEMrush HAR (or equivalent multi-tracker SaaS HAR) produces `name: semrush` (or the first-party domain root), no `streaming-bi-owox`-style artifacts, auth detected from a SEMrush endpoint (not Sentry).
  - **Negative:** feeding an API-only HAR (no HTML navigation entry) falls back to current behavior with a clear stderr note "no primary domain detected; using request-count heuristic."
- **Scope boundary:** Does NOT change reachability classification (covered by WU-4). Does NOT change endpoint normalization beyond the first-party filter.
- **Dependencies:** None.
- **Complexity:** medium

### WU-3: `profile save` captures subcommand-specific flags (from F4)
- **Priority:** P2
- **Component:** generator
- **Goal:** Every generated CLI's `profile save` captures flags declared on subcommands, not just persistent flags on root.
- **Target:** Generator's `profile` command template + the profile resolution logic in generated CLIs.
- **Acceptance criteria:**
  - **Positive:** `semrush-pp-cli profile save kram --columns x,y,z` succeeds; `semrush-pp-cli sheets push <id> --profile kram` resolves `--columns x,y,z`.
  - **Negative:** A profile saved with `sheets push --columns x` is NOT applied when running `pt report --columns y` — different subcommand path means no resolution.
- **Scope boundary:** Does NOT change root-level flag capture (still works as today). Does NOT support cross-subcommand wildcards.
- **Dependencies:** None.
- **Complexity:** medium

### WU-4: Reachability classifier matches challenge markers in bodies/status/headers, not URL substrings (from F3)
- **Priority:** P3
- **Component:** spec-parser
- **Goal:** `traffic-analysis.json` for a HAR with GTM/CookieHub/Sentry but no actual challenge does NOT report `browser_required` with high confidence.
- **Target:** `internal/spec/` reachability classifier.
- **Acceptance criteria:**
  - **Positive:** SEMrush-shaped HAR (status 200 everywhere, no challenge body) classifies as `standard_http` with confidence > 0.7.
  - **Negative:** A HAR with a real Cloudflare challenge response (`status: 403, body: "<title>Just a moment...</title>"`) still classifies as `browser_clearance_http` with confidence > 0.7.
- **Scope boundary:** Does NOT touch other reachability modes (`browser_http`, `unknown`). Only changes evidence-gathering for challenge detection.
- **Dependencies:** None — but the same SaaS-noise HAR that triggers WU-2 also triggers this; landing both together avoids regressing one.
- **Complexity:** small

## Anti-patterns
- **Substring-match heuristics on URLs.** "CAPTCHA marker" detected from JS bundle filenames is the canonical example. Substring matching against URLs is noisy by nature — match against bodies/headers/status codes where the signal actually lives.
- **Request-count-based primary-domain inference for HARs.** Modern SaaS load 30+ 3rd-party scripts; the loudest host is rarely the API. Anchor on the user's loaded HTML page instead.
- **Auth detection that doesn't distinguish first-party from 3rd-party.** Sentry's `sentry_key` looks like an API key to a naive analyzer.

## What the Printing Press Got Right
- **Generated CLI scaffolding compiled cleanly on first try** (after one hand-written spec). `go mod tidy`, `go vet`, `go build`, `govulncheck` all PASS. The scaffold included config, store, sync, agent-context, MCP bundle — the heavy lifting just worked.
- **The `--agent`, `--json`, `--select`, `--compact`, `--dry-run` global flag surface** is excellent — agent-native by default, no per-command thought required.
- **The `lock acquire` / `lock promote` / manifest workflow** kept state clean across the multi-step build → patch → install → polish → reinstall cycle. No state-collision issues despite many iterations.
- **The `printing-press-polish` skill correctly refused to ship-validate** a CLI whose advertised features (PKD, magic-recipe, etc.) didn't exist in code. False-advertising guards earned their keep.
- **CSV → JSON auto-parsing in a single edit point** (`client.go::do` after `sanitizeJSONResponse`) was clean to inject because the generator emitted that hook point. Good extension surface.
- **MCP bundle auto-generation** worked without any manual intervention; the read-only annotations on novel commands flowed through.
