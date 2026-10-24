# Phase 4.85 output review: app-store-pp-cli

Run 20261023-151515-ab5f6220. Reviewed 2026-10-24 by the printing-press-output-review sub-skill (forked; it reviewed in place without dispatching a second agent).

**Status: WARN.** 5 warnings, none blocking (Wave B policy).

## How the sample was taken

`scorecard --live-check` passed 5/5 novel-feature samples, but every sample ran on an empty local store and showed only "nothing captured yet" notes. To judge real output, the reviewer built a separate local store in the session scratchpad (`--home`) from about 10 sequential, governed Apple requests: three keyword searches, US and GB ratings, one listing, and the US top-free chart. The owner's real store was not touched.

## What held up

- **keyword-share relevance.** "music streaming" returned music services (TIDAL, Spotify #2, Pandora, Qobuz, YouTube Music, SoundCloud). "podcast player" returned podcast apps (Apple Podcasts, Pocket Casts, Overcast, Castbox, Spotify #7).
- **traction.** It accounts for every requested storefront (pk is listed as having no data), and its best rank matches the chart capture.
- **impact.** It found the release on 2026-10-23, set the before and after windows correctly, and marked thin windows as insufficient data.
- **breakouts and changes.** Both list every requested storefront or app, with a reason when there is no data.

## Findings

| # | Check | Finding | Disposition |
|---|---|---|---|
| 1 | format-bugs | keyword-share leaves competitor titles empty: titles come only from app snapshots, and keyword-rank, search-store and top do not write any. | **Open (v-next).** It needs result titles stored with each keyword capture (a schema change); the app ids are correct. |
| 2 | ranking-sort | keyword-share's held, prev and change columns did not reconcile. | **Fixed** in commit cb54e05: a new "compared" column (held among terms with an earlier capture) makes compared minus prev equal change; when nothing is comparable, those columns show dashes instead of 0. |
| 3 | semantic-query-match | The empty-state capture hints in keyword-share and impact hard-coded a term and dropped `--country`. | **Fixed:** the hints name the requested term, app and storefront. |
| 4 | aggregation-sources | changes said requested apps had "at most one capture" when they had never been captured. | **Fixed:** the note says they were never captured and names `app <id>`. |
| 5 | format-bugs | JSON output HTML-escapes `&`, `<` and `>`. It is valid JSON but visible to readers. | **Retro (press template).** The cause is the generator's `printJSONFilteredKeep`, which calls `json.Marshal` without `SetEscapeHTML(false)`, so every printed CLI has it. |

## Retro note

The live check passes local-history commands on an empty store, so it cannot catch any of the issues above. Retro candidate: seed the sample store, or exempt history commands from the pass count.
