# NCCPL Cloudflare replay — evidence ledger (do not re-test these)

## Goal
Make a non-browser Go CLI reach `https://www.nccpl.com.pk/api/*` reliably, so it can
back-fill daily settlement data. The user's real Chrome reaches the site fine.

## Environment facts (verified)
- Origin: `https://www.nccpl.com.pk`, Cloudflare in front, Laravel app behind.
- Browser that works: Chrome **149.0.7827.197** on macOS.
- HAR of a working session: every one of 176 NCCPL requests used **HTTP/3 (h3)**.
- Cookies in Chrome Profile 1, all valid:
  `cf_clearance` (533 bytes, host_key `.nccpl.com.pk`, expires 2027),
  `nccpl-session` (342 bytes, host_key `www.nccpl.com.pk`, ~2h TTL),
  `XSRF-TOKEN` (342 bytes, host_key `www.nccpl.com.pk`).
- CLI stack: Go, `github.com/enetx/surf v1.0.199` with `Impersonate().Chrome()`;
  `surfClient.Std()` -> `*http.Client` with a `net/http/cookiejar` attached.

## RULED OUT — do not repeat
1. **Cookies not reaching the wire.** Disproved. A jar probe on the exact request URL
   returns all three cookies, and requests go through `httpClient.Do(req)` which attaches
   jar cookies. Verified by a temporary Go test.
2. **stdlib `net/http` + cookies.** -> HTTP 403 **"Sorry, you have been blocked"**
   (hard WAF block, `cf-mitigated: None`). Distinct from the challenge below.
3. **Surf `Impersonate().Chrome()` over HTTP/2 + cookies.** -> HTTP 403
   **"Just a moment..."** (JS challenge), 6320-byte body.
4. **Surf with `builder.ForceHTTP3()` + cookies.** -> identical 403 "Just a moment...".
5. **Stale clearance.** Disproved. Re-tested with a `nccpl-session` **14 seconds old**
   and a `cf_clearance` Chrome was actively using successfully. Same 403.
6. **User-Agent mismatch alone.** Surf sends Chrome/145, the cookie was minted by
   Chrome/149. Pinned the request UA to the exact minting UA -> still 403 "Just a moment...".
7. **Path-specific protection.** Disproved. `/assets/css/main.css` (a static CSS file),
   `/api/fipi/latest-date` and `/market-information` ALL return 403 to stdlib and Surf.
   The challenge is site-wide, so static-asset and download surfaces are not an escape.
8. **`auth login --chrome` being broken.** Fixed separately and now works: it extracts all
   three cookies and persists them. (The generated `validateComposedAuth` used stdlib HTTP
   and misreported a valid session as "expired"; patched.)
9. **Automated Chromium (agent-browser default profile).** -> hard WAF block
   ("Sorry, you have been blocked"). Not usable.
10. **Chrome Safe Storage / Keychain decryption.** Works fine; `pycookiecheat` returns all
    three cookie values. Not a blocker.

11. **Sending ALL browser cookies, not just the three "required" ones.** The jar was rewritten
    with all six Chrome holds for the origin (`cf_clearance`, `nccpl-session`, `XSRF-TOKEN`,
    `acw_tc` (an Alibaba Cloud WAF token), `_ga`, `_ga_THNE9GFEFT`). -> identical 403
    "Just a moment...".
12. **Cookie header delivery under Surf.** Suspected Surf's impersonation middleware was
    rebuilding headers and dropping the jar's Cookie header. Tested by setting
    `req.Header.Set("Cookie", ...)` explicitly from the persisted jar. Response was byte-
    identical with and without the explicit header. **Cookies do reach the server under Surf;
    they are rejected.** (The 6320 vs 6298 byte difference between runs is just the challenge
    page's embedded ray token varying, not signal.)

13. **Genuine BoringSSL Chrome impersonation (curl_cffi / libcurl-IMPERSONATE).** Profiles
    chrome145, chrome146, chrome150, over BOTH h2 and real h3 (ngtcp2), with the byte-exact
    20-header set lifted from the HAR and live cookies. All -> 403 challenge.
    **DECISIVE: the no-cookie control returned the SAME 6085 bytes as the with-cookie runs.**
14. **ECH (Encrypted Client Hello).** Not a factor: the `HTTPS` RR for www.nccpl.com.pk is
    `1 . alpn=h3,h2 ipv4hint=... ipv6hint=...` with **no `ech=` parameter**.
15. **IPv6 egress split.** No global IPv6 on the machine, so Chrome cannot have minted the
    clearance over v6. Egress is a single PTCL (AS17557) v4 address; CF PoP is KHI.
16. **Newer surf / other Go TLS libraries.** surf v1.0.205 adds `JA().Chrome150()` but needs
    go>=1.27 (local is 1.26.7). bogdanfinn/tls-client has Chrome_144/146/150/152, not 149.
    CycleTLS tops out far older. utls tops out at HelloChrome_133. **No library ships a
    Chrome 149 profile.** Moot anyway given item 13.

17. **EXACT TLS FINGERPRINT MATCH — the hypothesis is dead.** The user pasted real Chrome 149's
    handshake from tls.peet.ws. It is **byte-identical to curl_cffi `chrome145`/`chrome146`**:
      ja4            t13d1516h2_8daaf6152771_d8a2da3f94cd   (identical)
      ja4_r          ...cca8,cca9_0005,000a,...             (identical)
      peetprint_hash 1d4ffe9b0e34acac0bd883fa7f79d7b5       (identical)
      akamai_fp      1:65536;2:0;4:6291456;6:262144|...     (identical)
    (curl_cffi `chrome150` DIFFERS and is the wrong profile to have been testing.)
    Then ran the full 2x2 against the live API with those matched profiles, exact Chrome 149
    headers, exact UA and live cookies:
      chrome146 h3 -> 403, 5893 B     chrome146 h2 -> 403, 5893 B
      chrome145 h3 -> 403, 5893 B     chrome145 h2 -> 403, 5893 B
    **A perfect fingerprint match, over both protocols, with valid cookies, still gets the
    challenge.** TLS/QUIC fingerprint is conclusively NOT the blocker.

## CONCLUSION: cf_clearance replay is IMPOSSIBLE for this origin
The matrix is fully explored -- fingerprint matched exactly, both h2 and h3, cookies
present/absent/all-six, exact UA, exact header set and order. Every cell returns the same
challenge. The clearance cookie does not by itself grant access to this site; access requires
something only a live browser session supplies (a per-session JS/Turnstile execution, or
connection-level state that cannot be transferred). Stop pursuing HTTP replay.

## THE REFRAME (superseded by item 17, kept for the audit trail)
Two independent TLS stacks (surf/quic-go and curl_cffi/BoringSSL) produce a response that is
**byte-identical with and without the cf_clearance cookie**. The cookie is not being rejected
on a fingerprint near-miss -- it is being **ignored entirely**. Any hypothesis that assumes
"get the fingerprint close enough and the cookie will be honoured" is therefore probably wrong.
The live question is now: what makes real Chrome's request eligible for the clearance at all?

## Key asymmetry worth reasoning about
- No cookies + Surf -> 403 `cf-mitigated: challenge`
- Cookies + Surf -> 403 "Just a moment..." (challenge page)
- Cookies + stdlib -> 403 "Sorry, you have been blocked" (hard block)
So the cookie DOES change server behaviour for stdlib (challenge -> hard block), which means
the cookie is being read and judged. For Surf the outcome is a challenge either way.

## Untested hypotheses (rank and attack these)
A. **TLS/JA3-JA4 fingerprint mismatch — NOW THE LEADING HYPOTHESIS.** Items 11 and 12 removed
   cookie-content and cookie-delivery as explanations; item 6 removed the User-Agent. What is
   left is the transport fingerprint. Surf impersonates Chrome 145; the cookie was minted
   by Chrome 149. Cloudflare binds clearance to the fingerprint. Candidate tools:
   `curl-impersonate` (real BoringSSL Chrome builds), `tls-client`, `cycletls`, or a newer
   surf release with a Chrome 149 profile.
B. **HTTP/2 or HTTP/3 fingerprint** beyond TLS: SETTINGS frame values, WINDOW_UPDATE,
   pseudo-header order, header ORDER and casing. The HAR records the exact header order the
   real browser used. Surf may differ.
C. **Missing client-hint headers**: `sec-ch-ua`, `sec-ch-ua-mobile`, `sec-ch-ua-platform`,
   `sec-fetch-site/mode/dest`, `priority`, `accept-encoding` value and order.
   The HAR has the exact set.
D. **IP binding.** cf_clearance may be bound to the IP that solved it; confirm the CLI and
   Chrome egress from the same IP.
E. **Alternate replayable surface for the same DATA** (not the same host): third-party
   republishers of NCCPL FIPI/LIPI (Portfolio360, FinHisaab, Youngs Capital, StockIntel,
   BullsView), the PSX data portal, SBP, or NCCPL's own PDF/xlsx bulletins on another origin.
   NOTE: switching data source is a product decision for the user, not a free substitution;
   propose it, do not assume it.
F. **A local proxy that borrows the real browser's TLS stack** — e.g. drive the user's own
   Chrome once to mint state, or route requests through a helper that reuses Chrome's network
   stack, while keeping the printed CLI a plain HTTP client.

## Constraints on any proposed solution
- The printed CLI must replay over ordinary HTTP (direct, Surf, or stored reusable auth).
  A resident browser sidecar as the normal command transport is NOT acceptable.
- macOS. Go. Must work unattended for a scheduled backfill.
- No paid third-party unblocking services.

---

# 2026-09-05 (afternoon) — ITEM 3 IS OVERTURNED. HEADLESS WORKS.

## The correction
The "CENTRAL FACT" above said headless is hard-blocked and cannot be used. **That is wrong,
and it cost this project a lot of effort.** The `HeadlessChrome/<v>` User-Agent token was the
*entire* tell. Pin the normal Chrome token and headless self-solves the challenge and reads
every endpoint.

## What actually distinguishes this from the eliminated hypotheses
This is NOT cf_clearance replay (items 1-17 stand — replay is still impossible). This is a
real Chrome doing a real challenge solve, with no window. The only change is the UA it
announces.

**The distinction that matters, and why an earlier attempt missed it:**
- CDP `Network.setUserAgentOverride` + `Emulation.setUserAgentOverride` sent before the first
  navigation **FAILS**. Measured: the challenge page loads, the challenge-platform
  `/cdn-cgi/challenge-platform/h/g/orchestrate/chl_page/v1` script fetches 200 (~80 KB), the
  `POST /cdn-cgi/challenge-platform/h/g/fo/...` returns 200 (~34 KB), and the page then sits
  on "Just a moment..." forever — polled once a second from t=1530ms to t=45740ms, never
  clearing. A CDP override does not reach the browser-level UA the challenge platform reads.
- The **`--user-agent=` command-line flag at launch** WORKS. It applies from process start,
  to every request including the first navigation and the challenge platform's own fetches.

## MEASURED, REPRODUCED, VERIFIED BY THE ORCHESTRATOR (not just a subagent claim)
Built from the patched tree and run against the live origin, fresh throwaway profile each run,
no window at any point, clean teardown (no surviving Chrome process):

  16:58:28  capture --resources var-margins --latest-only --launch --headless
            -> "challenge cleared; capturing..."
            -> var-margins 2026-09-04: **1091 rows, http_status 200**, rc=0.  24 seconds.

  16:59:12  capture --resources mts,slb --latest-only --launch --headless
            -> mts 2026-09-04: **68 rows, 200**;  slb 2026-08-27: **3 rows, 200**.  18 seconds.

  Rows verified present in the throwaway SQLite store afterwards:
    mts|2026-09-04|68   slb|2026-08-27|3   var-margins|2026-09-04|1091

## Also established this session
- **`--headless=old`, `--headless=new` and bare `--headless` ALL send `HeadlessChrome/149.0.0.0`.**
  Choosing an older headless engine does not hide the token. Only `--user-agent=` does.
- **A headed window CANNOT be hidden by `--window-position`.** macOS clamps it back on-screen:
  `--window-position=-3000,-3000` produced CDP bounds `left:0, top:33`, still visible. (It did
  self-solve and fetch 200/1091 rows — it just is not invisible.)
- **CDP `Browser.setWindowBounds {windowState:"minimized"}` DOES hide a headed window** and
  fetching continues to work: `visibility=hidden`, then `GET -> 200`, `POST -> 200`. This was
  the best headed answer before the headless result superseded it.
- **Copying Chrome.app to set `LSUIElement` fails**: editing Info.plist breaks the bundle's
  code signature ("invalid Info.plist (plist or signature have been modified)"). Dead end.
- **Handing a solved clearance to a headless Chrome makes things WORSE, not better.** Both
  cookie injection via `Network.setCookie` (6 cookies injected, 6 present) and reuse of the
  solved `--user-data-dir` produced the **hard WAF block** ("Sorry, you have been blocked"),
  not merely a challenge. Do not do this. The headless instance must solve for itself, which
  it does once the UA is pinned.
- **A solved profile persists.** Reused headed at +10 min: real page in 1669ms,
  `rechallenged=false`, `cf_clearance` value unchanged. Repeated spaced probes at +13/+18 min
  returned 200 with 1091 rows. A re-challenge did appear at one later probe and self-solved.
  `cf_clearance` expiry is ~1 year but the VALUE rotates on each re-solve.
  NOTE: this is a measured LOWER BOUND on profile lifetime, not the true expiry.
- **launchd works on a LOCKED Mac** — this was the operationally decisive question.
  A LaunchAgent in the **gui/501** domain, with the screen locked, ran the probe end to end:
  challenge solved in ~15s, `GET /api/fipi/latest-date` 200, `POST /api/var-margins/data`
  **200 / 1091 rows**, exited rc=0 in 21 seconds (log: leadB/launchd/gui.out).
  The **user/501** domain (no Aqua session) produced no output — treat gui/501 as required.
  With the headless fix this matters less, but it is the fallback if headless ever regresses.

## Product shape (respects the constraint: no resident browser)
`capture` gains `--headless`. It remains an explicit, occasional acquisition step that starts
Chrome, fetches, and tears it down. Nothing resident. Pair with `--profile <dir>` to keep a
solved clearance between scheduled runs and re-solve rarely.

## What is still NOT possible
HTTP replay of `cf_clearance` from any non-browser client. Items 1-17 above are unchanged.
The transport must still be a real Chrome; it simply no longer needs to be visible.
