# Phase-5 dogfood skip breakdown — run 20260819-035139-99573931

**The honest headline: the matrix planned 297 probes, ran 173, skipped 124.** The earlier
"173/173" framing counted only the probes the runner chose to execute — this file accounts for
every skipped one. Machine-readable copy with every probe listed: `skip-breakdown.json` (same dir).

None of the skips need OAuth except possibly captions (group F). None of them touched the 10 new
novel features — every new feature's happy path executed for real (that was the hollow-coverage
gate this run just cleared).

## What a "probe" is

For each of the 66 matrix commands the runner plans up to 5 probes: `help`, `happy_path`,
`json_fidelity`, `error_path` (bogus input → typed usage error), `error_path_real` (real
invocation expected to fail cleanly). Skips split: **81 error-path probes** (structural or
policy) and **42 happy/json probes** on 21 commands (list below). All 66 `help` probes ran.

## Groups, largest first

### A. 43× `error_path: no positional argument` — structural, nothing to unlock
Commands that take no positional argument (velocity, monitor, sync, doctor, list-style
commands…). The bogus-positional probe is impossible by definition. Their happy paths passed.
**Needs to execute: nothing — not a coverage gap.**

### B. 31× mutating-command error paths (17× `error_path_real: mutating command dry-run only` + 14× `error_path: would call live API without --dry-run`)
Error-path probes on commands that genuinely write (watch add/remove, workspace
create/use/remove, keys add/use/remove, teach, learnings confirm/forget/reject, playbook amend,
profile save/delete, workflow archive, sync). The press's runner policy: never fire real
failing invocations of writers. Happy paths of the read leaves of these families DID run.
**Needs to execute: a press-side policy switch that does not exist in 4.31.0 (retro candidate).
Not an auth issue.**

### C. 16× video-flagship fixtures (12× `no list companion for "videoid"` + 2× `"playlisturlorid"` + 2× id-derivation failures) — THE ACTIONABLE GROUP
`happy_path` + `json_fidelity` skipped on: youtube videos-transcript, videos-related,
videos-comments, videos-embed, videos-enrich, videos-links, playlist-enrich (+ learnings
confirm/reject id-derivation). The runner builds fixtures by asking a companion list command
for an id; it cannot auto-derive a `videoId|url` positional. These commands ARE covered
elsewhere (behavioral gauntlet 17/17 and this run's Phase 5 novel-feature sampling per the
run proofs), but not by this matrix.
**Needs to execute: `pp:happy-args` with a real video id (e.g. `videoId=dQw4w9WgXcQ;--max=3`)
on those 7 preserved novel files — an in-tree edit, ~16 more probes would run. Cost: edit +
rebuild + re-mint marker + re-promote (~10 min). Decision is the operator's.**

### D. 23× free-text positionals (`non-id positional "name"/"query"/"text"`)
happy/json/error_path_real skipped on framework commands: recall, search, which, feedback,
profile save/show/use/delete, learnings forget. The runner refuses to invent free text.
These are press-shipped framework commands — the same gap exists in every printed CLI.
**Needs to execute: generator-side happy-args on framework templates (upstream retro item).**

### E. 3× `export: command path has fewer segments than placeholders`
The runner's model of `export` mismatches its own template — press bug (the export command
itself works; its `help` probe passed).
**Needs to execute: press fix upstream.**

### F. 2× `youtube captions-list: blocked-fixture: required API parameter`
captions.list requires a videoId the runner couldn't synthesize — AND listing captions of
third-party videos commonly returns 403 without the owner's OAuth. the operator ruled OAuth out of
scope for this CLI.
**Needs to execute: fixture id (like group C) — but may still 403 by API design; acceptable
permanent skip under the no-OAuth ruling.**

### G. 2× `teach-playbook: file fixture required`
Needs a real playbook JSON file on disk; the runner has no file-fixture mechanism.
**Needs to execute: press-side file-fixture support (upstream retro item).**

## Bottom line for the publish decision

- ✅ Every new novel feature and every generated endpoint command's primary happy path executed
  live (the 173 passes; endpoint surface additionally verified 50/50 live in Phase 4, receipt
  in the run proofs).
- ⚠️ The one group worth buying before publish is **C** (7 video flagship commands' matrix
  fixtures) — a small in-tree edit, needs marker re-mint + re-promote.
- 🪦 Groups A/B/D/E/G are press-runner limitations identical for every printed CLI; they go on
  the retro list, not on this CLI.
