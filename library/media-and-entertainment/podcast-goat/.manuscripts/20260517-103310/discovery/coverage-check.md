# Six-Show Coverage Check

Each target show must be reachable end-to-end. Status definitions:
- **covered** — at least one source returns a usable transcript today with the auth the user
  is documented to have, plus at least one fallback if the primary fails.
- **partial** — primary path is gated on user runtime action (logging in via `auth login --chrome`)
  or yt-dlp install; fallback exists.
- **unsupported** — no replayable surface exists for this show.

| # | Show | Primary | Fallback chain | Status | Notes |
|---|------|---------|----------------|--------|-------|
| 1 | Dwarkesh | dwarkesh free (#5) | spoken paid (#8), youtube (#7) | covered | Public Substack with inline `<strong>Speaker</strong>` and `(HH:MM:SS)` H2s, 200 OK direct HTTP. No auth needed. |
| 2 | Acquired | member.acquired cookie (#2) | youtube (#7), spoken paid (#8) | partial | Member transcript HTML behind Memberstack. User must run `auth login --chrome --service acquired` first. Webflow rich-text shape, H3 chapter markers. YouTube fallback covers free episodes. |
| 3 | Huberman Lab | member.huberman cookie (#1) | youtube (#7), spoken paid (#8) | partial | Supercast-gated transcripts. User must run `auth login --chrome --service huberman` first. Huberman's free RSS lacks `<podcast:transcript>`, so source #6 doesn't help here. YouTube has every episode with auto-subs. |
| 4 | Founders | member.founders cookie (#3) | spoken paid (#8), taddy paid (#9) | partial | Member-only transcripts on founderspodcast.com. User must run `auth login --chrome --service founders` first. No free YouTube auto-subs path (audio-only show on YouTube). Spoken.md publicly covers some recent episodes. |
| 5 | Peter Attia — The Drive | member.peterattia cookie (#4) | spoken paid (#8) | partial | Premium subscriber-only shownotes + transcript on peterattiamd.com. User must run `auth login --chrome --service peterattia` first. Spoken.md covers public episodes. |
| 6 | Zhang Xiaojun | youtube #7 (zh-Hans + en auto-translate) | spoken paid (#8) — limited coverage for CN podcasts | covered | YouTube channel `@xiaojunpodcast`. yt-dlp `--write-auto-subs --sub-langs zh-Hans,en --skip-download` covers it; en track is auto-translated. Spoken.md primarily indexes English shows; fallback exists but is partial. |

## Summary

- **covered**: Dwarkesh, Xiaojun (2/6)
- **partial** (covered after one-time `auth login --chrome`): Acquired, Huberman, Founders, Peter Attia (4/6)
- **unsupported**: none (0/6)

All six target shows are reachable through the documented chain. Four require a one-time
`auth login --chrome --service <name>` invocation; this is the headline workflow Lan
specifically requested ("she'll pay for legit subscriptions and wants the CLI to use her
logged-in cookies — that's why cookie-first dispatch is non-negotiable").

## Implications for Phase 5 dogfood

Live dogfood expectations:
- Source #5 (dwarkesh) — fully testable in Phase 5 without setup.
- Source #6 (rss) — synthetic test against a feed known to advertise podcast:transcript
  (e.g., one from podcastindex.org's curated 2.0 list).
- Source #7 (youtube) — fully testable; yt-dlp present.
- Source #8 (spoken) — fully testable with `pt_demo`.
- Sources #1-#4 — testable only if the user has performed `auth login --chrome --service <name>`
  for that publisher; deferred to user-driven smoke after publish, not gating Phase 5.
- Source #9 (taddy) — requires user-provided TADDY_API_KEY; deferred.
- Source #10 (whisperapi) — requires user-provided provider key; deferred.
