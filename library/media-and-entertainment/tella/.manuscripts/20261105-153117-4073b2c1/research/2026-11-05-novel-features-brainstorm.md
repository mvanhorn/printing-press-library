## Customer model

### Course creator, recording course lessons

**Today:** Course creator records lessons in Tella and opens the editor to run automatic edits, inspect mistakes, and watch a preview. She wants the agent to do as much editing as the API allows, while keeping the review decision with her.

**Weekly ritual:** Record a lesson, run the house editing recipe, watch the live preview on her phone, then approve a separate delivery step.

**Frustration:** Review links and automatic editing currently require fragmented commands or browser work, and a completed edit is hard to distinguish from an export that is still processing.

### Course production assistant, preparing lesson delivery

**Today:** The assistant inspects the recording's transcript and timeline, applies captions and chapters, checks framing, and collects the export and caption file. Those tasks cross video, clip, source, and export endpoints.

**Weekly ritual:** Prepare newly recorded lessons consistently, flag repeated sentences for review, and deliver an MP4 plus captions only after approval.

**Frustration:** A successful API call is insufficient proof of a usable delivery folder, and retake removal needs an inspectable plan rather than an opaque AI claim.

### Automation agent, operating the recorded lesson pipeline

**Today:** The agent composes endpoint calls, manages saved snapshots and local transcript searches, and must avoid confusing cached state with live state or retrying an ambiguous write twice.

**Weekly ritual:** Inspect recordings, plan edits, apply reviewed plans, verify the resulting timeline, and report a preview or recovery information.

**Frustration:** Cookie-only mistake detection and stale endpoint shapes make unattended execution unreliable; unsafe retries or automatic restores can damage concurrent edits.

## Candidates (pre-cut)

The supplied brief has concrete Users and Top Workflows, so the pre-flight passes. Prior research discovery returned `none`; the explicitly supplied prior manifest and patch ledger are reconciliation evidence, not an invented research.json. The current official spec was parsed, not read as an unbounded document. Existing typed endpoints are absorbed; all 16 prior feature contracts remain required. Sources below mean (a) persona, (b) service pattern, (c) local cross-entity data, (d) prior manifest reconciliation, and (e) user vision. No DeepWiki evidence was supplied, so source (f) is unused.

| # | Candidate | Command | Description / persona | Source | Inline verdict | Long Description |
|---|-----------|---------|-----------------------|--------|----------------|------------------|
| 1 | Automatic edit pass with recovery | videos auto | Apply selected automatics across clips, retain cut snapshots, report progress and finish with preview; Course creator and agent | a,b,d,e | Keep: real multi-clip composition; dry-run first. No auto-layout endpoint exists, so requested layouts must report unsupported rather than fabricate success. | Use this command for selected automatic edits across a video. For a complete lesson recipe, use 'lessons pass'. |
| 2 | House-style lesson preparation | lessons pass | Compose recipe, automatics, retake plan, captions, chapter draft, thumbnail and preview; Course creator and assistant | a,b,d,e | Keep: explicit user requirement. Mechanical chapter drafts use transcript text/timestamps; no external LLM. Retakes remain a plan unless explicitly reviewed for application. | Use this command to prepare a lesson and create a preview for review. For delivery after review, use 'lessons ship'. |
| 3 | Approved lesson delivery folder | lessons ship | Export, poll, download MP4 and captions, and report local paths; assistant | a,b,d,e | Keep: compose existing export and captions APIs, bounded waiting and artifact checks, no publish call. | Use this command to export a reviewed lesson and download its delivery files. For editing and preview, use 'lessons pass'. |
| 4 | Mechanical retake plan | clips find-retakes | Compare repeated/restarted sentence text and show candidate cut ranges with timestamps; assistant and agent | a,b,d,e | Keep after reframe: deterministic repeated prefixes and exact normalized matches only; never claim semantic mistake detection. Apply only a reviewed plan with cut snapshots. | none |
| 5 | Agent review artifacts | videos storyboard-image / videos timeline | Save API storyboard images for video, clip, or source and format a compact timeline; agent and assistant | a,b,d,e | Keep: agent-readable files plus timeline-oriented output, with content/status checks instead of invented images. | none |
| 6 | Live preview alias | videos preview | Print live review URL and its documented 24-hour expiry; Course creator | b,e | Required absorbed convenience command, not transcendence: one endpoint. Keep in shipping scope. | none |
| 7 | Credit-aware Tella Agent fallback | agent-chats start / agent-chats list | Check account credits, require explicit --yes for start, list chats without a write; agent | a,b,e | Required absorbed workflow, not counted as novel transcendence. Start gates before the paid write; one live test maximum and credits before/after. | none |
| 8 | Safe write identity | client-wide Idempotency-Key | Give all remote writes a stable request identity across retries; agent | a,d,e | Required foundation behavior, not a standalone transcendence command. Reuse the same key per logical write and never share it across distinct writes. | none |
| 9 | Semantic mistake remover | clips semantic-mistakes | Infer whether speech is wrong and remove it automatically; assistant | a,b | Kill: semantic NLP needs an unprovided model and correctness cannot be established mechanically. Preserve the official Agent fallback separately. | none |
| 10 | Lesson release autopilot | lessons publish-all | Edit, export and publish lessons without an approval boundary; Course creator | a | Kill: conflicts with explicit no-publish scope and would erase the review decision. | none |
| 11 | Persistent lesson production dashboard | lessons dashboard | Run a background UI watching all recordings and exports; assistant | a,c | Kill: application scope and background infrastructure; existing local search/stats and one-shot workflows suffice. | none |
| 12 | Cookie mistake bridge | clips browser-mistakes | Reuse the unofficial browser-cookie mistake analyzer; agent | d | Kill new implementation: explicit retirement requirement and separate cookie auth. Leave the old find-mistakes name as a clear pointer, not a functioning cookie transport. | none |

Prior-feature candidates P1–P16 are also scored in Reprint verdicts below. None is silently dropped. Their preserved implementation does not inflate the five new transcendence survivors.

## Survivors and kills

### Survivors

Scores list Domain Fit / User Pain / Build Feasibility / Research Backing; evidence is the concrete user brief plus parsed current spec and, where cited, the recorded prior patch contracts. Scores are evidence assessments, not live validation claims. All five require hand-written composition or output handling after generation. Thin wrappers and safety foundations stay in the absorbed shipping scope.

For survivor 1, the weekly recording ritual supplies direct repeated use; it composes every clip rather than renaming one endpoint, and replaces candidate 12's cookie dependence with official automatic operations. For survivor 2, the same weekly ritual needs one consistent recipe; its value is service-specific multi-step composition, replacing candidate 10's unapproved autopilot. For survivor 3, weekly delivery requires real downloaded artifacts, not merely an export-start response; it replaces candidate 10's publish coupling. For survivor 4, weekly review needs a transparent, conservative repeat plan; transcript structure is the service-specific leverage and candidate 9's semantic inference is deliberately excluded. For survivor 5, weekly agent inspection needs actual viewable assets and meaningful timeline output; it replaces candidate 11's background dashboard. All non-none Long Description redirects point only at surviving command paths.

The survivors do not add unrequested infrastructure or an LLM. `lessons pass` may compose existing helpers; recipe fields must be validated before writes, and unsupported options must fail clearly. Auto layouts are absent from the spec: do not emulate a made-up endpoint or silently skip an explicitly requested flag. Preview and credit-gated Agent commands remain mandatory absorbed work even though their thin wrappers do not belong in the novelty score.

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Semantic mistake remover | Requires unprovided semantic inference and risks deleting valid speech; deterministic retake candidates are inspectable. | clips find-retakes |
| Lesson release autopilot | Publishing and bypassing review are outside the explicit user authorization. | lessons pass / lessons ship |
| Persistent lesson production dashboard | A new background application is unnecessary scope for one-shot lesson workflows. | videos storyboard-image / videos timeline |
| Cookie mistake bridge | New unofficial cookie transport conflicts with retirement of find-mistakes and adds a separate auth requirement. | clips find-retakes |

Candidates 6–8 are retained as absorbed requirements, not killed: preview, agent chats and client-wide idempotency all ship. The compatibility name `find-mistakes` must explain the retirement and point to `find-retakes` and `agent-chats start`; its cookie implementation is not retained.

## Reprint verdicts

The prior manifest supplies 16 feature contracts even though run-level prior research.json was absent. Keep their command paths and outputs unless an explicit reframe below replaces an unofficial mechanism. Scores are Domain Fit / User Pain / Build Feasibility / Research Backing. Lower weekly relevance to the new lesson ritual is not authorization to delete an existing published feature.

| # | Prior feature / command | Verdict | Score | Source | Justification |
|---|-------------------------|---------|-------|--------|---------------|
| P1 | Cross-video transcript search — transcripts search | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Local FTS search supports finding prior lesson wording; preserve sync error propagation and live transcript route. |
| P2 | Watch-milestone digest — videos viewed | prior-keep | 7/10 (2/1/2/2) | prior (kept) | Retain webhook-based digest compatibility, clearly distinguishing view counts from percentages and inbox events from API totals. |
| P3 | Webhook tail + replay — webhooks tail | prior-keep | 8/10 (2/2/2/2) | prior (kept) | Event inspection remains useful to automation; preserve replay HMAC behavior and do not send live replay messages without an explicit authorized target. |
| P4 | Bulk standard edit pass — clips edit-pass | prior-reframe | 9/10 (3/2/2/2) | prior (reframed from unofficial mistake stage in clips edit-pass) | Keep the command and public edit stages; retire the cookie mistake stage and direct semantic requests to the explicit credit-gated Agent route. |
| P5 | Transcript diff — clips transcript-diff | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Comparing retained versus includeCut=true words is a useful mechanical audit; preserve timestamps and visible fetch errors. |
| P6 | Upload-to-edit — sources upload-file + videos clips add | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Recipe media requires real uploaded source bytes; preserve upload errors and support current spec media kinds instead of video-only assumptions. |
| P7 | Exports waitlist — exports wait | prior-keep | 10/10 (3/3/2/2) | prior (kept) | This is the delivery foundation; preserve export envelope parsing, uncached polling, encoded IDs, failed/timeout exit behavior and compatibility flags. |
| P8 | Caption-file export — clips captions | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Preserve per-clip SRT/VTT output even when lessons ship also downloads official video captions. |
| P9 | Workspace stats — workspace stats | prior-keep | 7/10 (2/1/2/2) | prior (kept) | Retain local aggregate compatibility and accurate provenance; never fabricate rows for empty list envelopes. |
| P10 | Declarative storyboard — videos storyboard + videos apply-storyboard | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Reviewed JSON plans remain a useful broader editing interface alongside lesson recipes; preserve errors rather than null transcripts. |
| P11 | One-shot media insertion — videos clips insert-file / add-broll | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Existing source upload and clip/layout composition supports recipe media without a new service. |
| P12 | Transcript-driven repairs — videos clips cut-words / replace-word-ranges | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Keep explicit word-index edits alongside conservative retake proposals; preserve official transcript routing and snapshot safety. |
| P13 | Pre-publish edit QA — videos audit | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Mechanical long-silence and timeline findings help weekly review without claiming subjective quality or publishing. |
| P14 | Playlist-scale edit planning — playlists edit-pass | prior-keep | 9/10 (3/2/2/2) | prior (kept) | Preserve batch planning and explicit apply boundaries for course modules rather than replacing it with a single-video pass. |
| P15 | Safe clip clean — videos clips clean | prior-reframe | 10/10 (3/3/2/2) | prior (reframed from cookie mistakes in videos clips clean) | Keep public cleanup and snapshots, replace unofficial mistakes with a clear pointer, and retain partial-failure recovery contracts. |
| P16 | Cut rollback — videos clips undo-last-cuts / restore-cuts | prior-keep | 10/10 (3/3/2/2) | prior (kept) | Preserve pre/last-known-post cuts, divergence refusal and exclusive-edit confirmation; never claim cut rollback reverses every zoom, crop, recipe or uploaded source. |

No prior feature contract is dropped. The two reframes preserve command names and remove only the explicitly retired unofficial mistake mechanism. The patch contracts also remain: all recorded API drifts and output/usage rules must survive generation; cursor walks must terminate before aggregation; empty envelopes store zero rows; filter types accept comma lists and JSON arrays; ambiguous writes retain their conservative policy unless stable idempotency is proven for that operation; creator and release provenance remain unchanged.
