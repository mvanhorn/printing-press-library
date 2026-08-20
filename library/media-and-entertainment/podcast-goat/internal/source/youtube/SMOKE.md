# YouTube Adapter Live Smoke Log

Run: 2026-05-17 (plan 2026-05-17-007, U2)

Smoke against `https://www.youtube.com/watch?v=lXUZvyajciY` (Dwarkesh's Karpathy
episode on YouTube; pairs with the Substack version of the same episode for
cross-source compare).

## Scenario A — yt-dlp on PATH

Setup: `/opt/homebrew/bin/yt-dlp` version 2026.03.17 (brew install).

```
$ time podcast-goat-pp-cli episode get https://www.youtube.com/watch?v=lXUZvyajciY
# Andrej Karpathy — "We're summoning ghosts, not building animals"

**Show:** dwarkesh-patel
**Host:** Dwarkesh Patel
**URL:** https://www.youtube.com/watch?v=lXUZvyajciY
**Provider:** youtube

[~4.8s wallclock]
```

Result: PASS (after parser fix).

## Scenario B — sidecar lazy-download

Setup: sidecar removed from `~/.config/podcast-goat/bin/`; PATH stripped of
`/opt/homebrew/bin` (`PATH=/usr/bin:/bin:/usr/sbin:/sbin`).

```
$ rm -f ~/.config/podcast-goat/bin/yt-dlp
$ env -i HOME="$HOME" PATH="/usr/bin:/bin" time podcast-goat-pp-cli episode get https://www.youtube.com/watch?v=lXUZvyajciY
podcast-goat: downloading yt-dlp (one-time, ~35MB) from https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos ...
podcast-goat: yt-dlp installed at /Users/.../bin/yt-dlp (35.0 MB)
# Andrej Karpathy — ...
[23.6s wallclock — ~6s download + normal fetch time]
```

Verified:
- `~/.config/podcast-goat/bin/yt-dlp` written, 35M, mode 755
- Progress message printed to stderr (verified via stderr capture)
- Subsequent fetch with same stripped PATH ran instantly (sidecar reused)
- PATH-installed yt-dlp wins over sidecar on subsequent unstripped runs

Result: PASS.

## Bug found and fixed in-unit

YouTube auto-subs emit rolling-window cues — each cue's text contains the
previous cue's text plus a few new words. Without collapsing, the canonical
markdown output repeats the same phrase 3-5 times per second. Fix landed at
`internal/source/youtube/youtube.go::collapseRollingWindow()`, covered by
six unit tests in `youtube_test.go`. The collapse merges chained prefix
extensions into one segment per coherent text chain.

Before: 5000+ duplicated rolling segments per 2hr episode.
After: substantially fewer; each segment is a complete or near-complete
sentence at the LAST observed timestamp.

## Known limitations (not bugs)

1. **No speaker diarization in yt-dlp's free path.** YouTube auto-subs don't
   carry per-speaker labels. The adapter attributes all speech to the channel
   uploader (e.g., "Dwarkesh Patel" for both host and guest). To get real
   diarization for YouTube episodes, use the `--paid --provider spoken` path
   (spoken.md does named diarization at $0.10/transcript) or wait for v0.2's
   whisperapi path (ElevenLabs Scribe ships diarization).

2. **Fragmented sentences.** Even with rolling-window collapse, some
   sentences span multiple cues that don't share a clean prefix. The
   collapsed output preserves all content but with more segment boundaries
   than a hand-edited transcript would have.

## Downstream commands verified

- `cache list` — YouTube row appears with show + title populated
- `speakers list` — Dwarkesh Patel aggregates across both `dwarkesh-podcast`
  (Substack) and `dwarkesh-patel` (YouTube) shows: 3 episodes, 5600 segments
- `episode quote "AGI"` — finds quotes from the YouTube cache
- `source compare <yt-url>` — fans out across all 11 adapters; only youtube
  matches the URL pattern, others skip cleanly
