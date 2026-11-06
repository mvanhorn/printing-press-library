# Phase 14 — semantic SKILL review


## Findings

1. **Description accuracy — error — SKILL.md:62.** `sources upload-file` promises to add a new clip, but it only creates/uploads a source and returns its ID; there is no video target flag. Describe that boundary and point to `videos clips insert-file` for upload-and-attach.

2. **Description accuracy — error — SKILL.md:76.** `videos storyboard` promises to apply a reviewed edit plan, but its shipped command only reads the storyboard. Say it exports editable state and name `videos apply-storyboard --file … --apply` as the separate application command.

3. **Description accuracy — error — SKILL.md:78.** `videos clips cut-words` promises optional replacement-media insertion, while its only task flags are `--term` and `--apply`. Describe term-based cuts and point to `videos clips replace-word-ranges` for replacement media.

4. **Description accuracy — error — SKILL.md:82.** `undo-last-cuts` claims it accepts an explicit cuts JSON body; it actually reads a saved snapshot via `--snapshot` and validates expected post-edit cuts. Describe saved-snapshot restoration and point to `restore-cuts` for an explicit cuts body.

5. **Description accuracy — error — SKILL.md:57.** `webhooks tail` claims replay behavior that belongs to the separate `webhooks replay` command. Keep the combined capability, but explicitly name both commands instead of implying tail performs the replay.

6. **Marketing-copy smell — warning — SKILL.md:672.** “These capabilities aren't available in any other tool for this API” is an unsupported absolute exclusivity claim. Replace it with a concrete heading sentence such as “Workflows provided by this CLI.”

These descriptions repeat in the command reference and lower Unique Capabilities section. Fix descriptions in both `novel_features` and `novel_features_built` of research.json, then sync all generated surfaces; do not patch only SKILL.md. Keep all21 feature identities and companion commands. The exclusivity sentence is hand-authored surrounding prose and can be edited locally.

## Seven-check disposition

| Check | Result | Evidence |
| --- | --- | --- |
| Trigger phrases match capabilities | PASS | Editing, preview and caption export have concrete shipped commands. |
| Verified-set alignment | PASS | Exactly21 unique canonical capability commands, equal to verified research set. |
| Descriptions match commands | FAIL | Findings1–5; all21 helps checked, targeted source confirms operation boundaries. |
| Stub/gated disclosure | PASS | Plan-by-default, explicit application, layouts unsupported, retired cookie mistakes, credit-gated Agent and cuts-only recovery are disclosed. |
| Auth narrative | PASS | `auth setup --launch`, stdin `auth set-token`, TELLA_API_KEY and doctor match shipped help. |
| Recipe output claims | PASS | Lesson plan/apply/ship sequence, retake review, cut recovery and media limitations match help/source. No positive live retake coverage claim is required here. |
| Marketing-copy smell | WARN | Finding6. |

## Ground truth consulted

- Current SKILL.md59702bytes and README.md66442bytes.
- Current research.json14458bytes and approved absorb manifest26995bytes.
- All21 verified novel command helps (offline only), auth setup/set-token helps and webhooks replay help.
- `sources_upload-file.go`, `videos_storyboard.go`, `novel_workflows.go` cut-words/apply-storyboard/replace-word-ranges handlers and `videos_clips_restore.go` snapshot restoration.