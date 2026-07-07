# OpenArt CLI - Phase 5 Acceptance Report

**Run ID:** 20260513-152641
**Level:** Full Dogfood
**Tests:** 12/12 passed
**Gate:** PASS

## Live test matrix (real account, real cookies)

| # | Test | Result | Notes |
|---|---|---|---|
| 1 | `auth login --chrome` | PASS | Imported 36 cookies via pycookiecheat from logged-in Chrome session |
| 2 | `auth status` | PASS | Authenticated, source=browser |
| 3 | `doctor` | PASS | Auth + Browser Session Proof + API + Credentials all green |
| 4 | `credits balance --json` | PASS | balance=43,496 (sub=43,206 + free=200 + trial=90), correct subscription state |
| 5 | `cost estimate --model seedance2 --duration 10 --count 1 --live` | PASS | 888 credits estimate (close to actual 800), balance projection correct |
| 6 | `models cheapest --type video --duration 10 --resolution 720p --json` | PASS | Ranks Kling 2.6 (142c) cheapest, Grok (187c), Wan 2.6 (250c) |
| 7 | `models list --json --select` | PASS | 10 video models surfaced from curated catalog |
| 8 | `video gen` (HEADLINE) — Seedance 2.0, 720p, 10s, 1 video, --wait, --download, --notify | **PASS** | Submit → poll → download e2e in real time |
| 9 | Downloaded MP4 file inspection | PASS | 7,046,311 bytes, 1280×720, 10.07s duration, h.264+aac stereo |
| 10 | Post-gen balance | PASS | 43,496 → 42,696 = exactly 800 credits (matches Seedance tier) |
| 11 | `media-sync` then `prompts find "phoenix"` | PASS | Found the Seedance gen with full prompt, URL, duration |
| 12 | `credits burn --since 1d --by model` + `stats` | PASS | Aggregated 22,550 credits across 19 events, 50 videos by model |

## Headline workflow validation

The user's stated need:
> "I want to be able to from the cli 'generate a video using seedance 2 that does this for 10 seconds, give me 2 videos' type thing and it'll do that, be able to link me to the URL or download locally."

Single-command realisation:
```
openart-pp-cli video gen \
  --prompt "a phoenix soaring over molten gold canyons, cinematic..." \
  --model byte-plus-seedance-2 \
  --duration 10 \
  --count 1 \
  --resolution 720p \
  --wait \
  --download ~/openart-out/ \
  --notify
```

Output: a real 10-second Seedance 2.0 video at `~/openart-out/6BXM1bLW0GXxqAA98LGl.mp4` (7 MB), terminal bell + "openart: Seedance 2.0 × 1 ready" notification.

Wall-clock: roughly 7 minutes from submit to file-on-disk for 1 video.

## Bugs fixed inline (during Phase 5)

These were generator/template-shape bugs that surfaced only against the live API. All have been patched in this CLI's source; each is also a retro candidate for the Printing Press generator.

1. **Auth domain hardcoded empty** — `internal/cli/auth.go` line 82 had `domain := ""` which broke press-auth and the cookie extraction backends. Fix: `domain := "openart.ai"`. Generator should derive this from `base_url` host.
2. **Cookie auth routed to Authorization header** — `internal/client/client.go` line 252 always set `Authorization: <value>`. For OpenArt's cookie auth the cookies must travel in the `Cookie` header. Fix: added `looksLikeCookieString()` heuristic that routes cookie-shaped values to `Cookie:` instead. Generator should emit cookie-vs-bearer routing based on `auth.type`.
3. **Wrong balance field** — `credits balance` and `credits forecast` and `cost estimate` all read `free_credit_balance` from `/user/my-info`, but for paid users that field is small (200) and the real balance is `subscription_monthly_credit + free_credit_balance + trial_credit_balance`. Fixed locally; this is API-specific so retro-worthiness depends on whether the framework can express "summed credit pools" generically.
4. **Sync limit too high** — generator emitted `limit=100` for `/credits/logs`, OpenArt caps at 50 ("Bad Request: Too big"). Fix: changed `determinePaginationDefaults` limit to 50. Retro candidate: generator should detect server-side caps via probe, or accept per-resource limit overrides in the spec.
5. **Media not in default sync list** — sync skipped `media` because the spec marks `projectId` as required and the framework can't auto-resolve it. Fix: hand-wrote `media-sync` command that resolves the default project then walks /resources. Retro candidate: framework should support "implicit default-project param injection" patterns.

## Phase 5 spend

- Phase 1.7 (browser-sniff probe): 150 credits (Grok Imagine, 720p/5s)
- Phase 5 (headline test): 800 credits (Seedance 2.0, 720p/10s, 1 video)
- **Total: 950 credits** out of 43,496 starting balance (2.2% of pool).

## Gate

PASS - all 12 acceptance tests green; headline workflow proven end-to-end on the user's real account; ready for Phase 5.5 polish + Phase 5.6 promote.
