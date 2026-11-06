# Phase 14 — re-review after T10 fixes

Result: **one residual description error; not PASS**. Reviewed the modified working tree based on df4c73612e, not a clean committed head. All six initial findings are cleared. No API, credential, network, install or git-write actions ran; only offline help and scoped file reads.

## Original findings cleared

| Original finding | Current evidence | Result |
| --- | --- | --- |
| Upload vs attach | SKILL.md:64 now returns the uploaded source ID and points to insert-file for attachment. | CLEARED |
| Storyboard read vs apply | SKILL.md:76 names videos apply-storyboard --file PLAN.json --apply separately. | CLEARED |
| Term cuts vs replacement | SKILL.md:78 names replace-word-ranges for replacement media. | CLEARED |
| Snapshot vs explicit cuts body | SKILL.md:82 distinguishes --snapshot from restore-cuts and retains recovery checks. | CLEARED |
| Webhook tail vs replay | SKILL.md:57 explicitly names webhooks replay as a separate command. | CLEARED |
| Unsupported exclusivity claim | SKILL.md:672 now says “Workflows provided by this CLI.” | CLEARED |

Corrections also appear in research.json planned/built entries and repeated rendered descriptions. Both research lists are equal; the canonical Unique Capabilities section still has exactly21 unique names matching novel_features_built.

## Residual finding

**Description accuracy — error — SKILL.md:77 and253.** `videos clips insert-file` still claims clip **or B-roll** attachment, but its handler only calls `attachExistingSource`; B-roll uses `videos clips add-broll`. Describe clip attachment and explicitly name add-broll for B-roll layouts, updating both research sets and syncing rendered surfaces.

This is an additional missed instance of the same companion-command conflation, not a failure of T10's six requested corrections. Actual helps confirm insert-file takes videoId/path and clip metadata; add-broll additionally takes clipId and layout timing. Source novel_workflows.go:22–64 attaches a clip; :66 onward creates the B-roll layout.

## Seven-check disposition

| Check | Result |
| --- | --- |
| Trigger phrases match concrete capabilities | PASS |
| Canonical capability set equals verified research set | PASS —21 names; planned/built lists equal |
| Feature descriptions match commands | FAIL —one residual above |
| Stub/gated and safety-limit disclosure | PASS |
| Auth narrative accuracy | PASS |
| Recipe output claims and concrete Direct Use examples | PASS |
| Marketing-copy wording | PASS |

All21 verified novel `--help` calls exited0 on the rebuilt CLI. Also checked helps for account, clips captions, auth setup/set-token, webhooks replay, apply-storyboard, replace-word-ranges, restore-cuts and add-broll. Direct Use now uses `tella-pp-cli account --agent` (line666) and `tella-pp-cli clips captions --help` (line668), which are concrete valid paths. No placeholder executable examples remain there; argument-hint metadata remains descriptive syntax.