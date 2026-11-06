# Supplemental live workflows

These targeted tests supplement the read-only matrix. Only registered owned pp-test copies were mutated. Earlier generic Press verification may have read social-account listings through its unscoped sync fallback; no per-request evidence exists to certify absence. Generic live verification was stopped.

## CLI
- `videos duplicate video`: Two owned pp-test video copies created; original timeline unchanged.
- `videos auto`: Explicit apply of fillers/silences and zoom/crop; owned-copy re-read; exact cut undo. Layouts refused before write.
- `lessons pass`: Recipe applied on real recording copy, including chapters/thumbnail/style/music and source intro; final preview HTTP 200.
- `lessons ship`: Real export completed; MP4 ffprobe H264/AAC and SRT header/cues checked.
- `clips find-retakes`: Live transcript produced valid zero-candidate plan; positive candidate application tested only synthetically.
- `videos clips find-retakes`: Alias returned valid live plan; positive candidate application not claimed.
- `videos preview`: Owned-copy preview URL loaded: HTTP 200.
- `agent-chats start`: One explicit --yes paid chat completed; missing explicit consent refused; account balance checked.
- `videos storyboard-image`: Video, clip and source WebP files validated and visually opened.
- `videos clips clean`: Owned clip time-range and transcript-word cuts applied; re-read and exact cut restoration.
- `videos clips undo-last-cuts`: Snapshot undo restored exact previous cut array.
- `videos clips restore-cuts`: Explicit snapshot restore restored exact previous cut array.
- `videos apply-storyboard`: Owned-copy description changed and re-read exactly.
- `videos format`: Owned-copy YouTube format re-read as 1920x1080.
- `videos clips cut-words`: Live planning returned eight matching ranges; no positive mutation claimed for this command.
- `exports wait`: Real owned-video export reached completed status.
- `videos live-preview get-link`: Canonical POST produced a preview loaded with HTTP 200.
- `videos recording-link get`: Canonical POST succeeded on second owned copy; MCP-confirmed recording link loaded with HTTP 200.
- `videos update`: Owned-copy description update re-read exactly.
- `videos delete`: Both registered owned copies deleted; GET 404 and complete final listing confirmed absence.
- `sync`: Scoped videos --max-pages 1 --no-prune completed in 98.53s; also read child agent-chat lists; bounded cache result, not a complete aggregation.

## MCP
- `videos_auto`: Owned-copy workflow applied and re-read.
- `lessons_pass`: Owned-copy recipe applied; preview loaded successfully.
- `clips_find_retakes`: Live plan validated; zero candidate fixture; no positive retake apply claimed.
- `videos_preview`: Owned-copy preview created and loaded.
- `videos_storyboard_image`: Actual WebP downloaded and validated.
- `lessons_ship`: Actual MP4 and VTT downloaded; ffprobe and WEBVTT header checked.
- `agent_chats_start`: Negative consent guard tested: --agent-style mode without explicit yes refused. No paid MCP chat created.
- `tella_execute`: Agent POST remained blocked; link POSTs previewed without confirmation and succeeded only with explicit confirmation; read-only mode excluded execute.

## Limits and evidence

Cut undo restores cuts only. Other editor state is saved as a manual reference. No full-editor undo is claimed. Retake-positive application uses synthetic tests; the live fixture had zero candidates. The sync cap is disclosed and is not used for completeness claims.

Sanitized run proofs: live-lesson-summary.json, final-binary-lesson-pass.json, live-delivery-summary.json, live-intro-summary.json, undo-last-cuts-lifecycle.md, restore-cuts-lifecycle.md, transcript-cut-lifecycle.md, transcript-search-live-summary.json, mcp-link-policy-live-summary.json, original-source-unchanged.json and cleanup-summary.json.

Cleanup: 2 created / 2 deleted; final complete listing 45 rows, no next cursor; both owned IDs absent, deleted GET returned 404. Original source timeline exactly unchanged. One chat spent 41.38815 credits (9000 → 8958.61185).

Publication remains blocked on the pinned Press host false positive and requested binary refresh. No PR, merge, installation or skill sync is claimed.

## Refreshed final verification

The refreshed build added a third owned copy, completed lessons pass/ship again and bounded JSON tail viaCLI+MCP. Final read-only MCP count30 includes the corrected silence-map tool; actual silences/transcript/sources returned and both link POSTs remained refused. Final complete matrix120pass/0fail/241skips. All3ownedcopiesdeleted, GET404each and a complete45-row final listing prove absence; original timeline exactly unchanged. One paidchat total,41.38815credits. Latest cleanup-summary.json is authoritative.
