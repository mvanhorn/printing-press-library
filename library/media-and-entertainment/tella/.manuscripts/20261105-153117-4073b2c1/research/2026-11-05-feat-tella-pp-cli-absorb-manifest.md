# Tella absorb manifest

### Absorbed (current API and retained workflows)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Get the account | Official OpenAPI | (generated endpoint) account | Typed input, JSON, dry-run and MCP |
| 2 | List a video's agent chats | Official OpenAPI | (generated endpoint) agent-chats list | Typed input, JSON, dry-run and MCP |
| 3 | Start an agent chat | Official OpenAPI | (behavior in tella-pp-cli agent-chats start) credit-gated workflow | Typed input, JSON, dry-run and MCP |
| 4 | Get workspace analytics | Official OpenAPI | (generated endpoint) tella-public-analytics | Typed input, JSON, dry-run and MCP |
| 5 | Report an Auto Capture take | Official OpenAPI | (generated endpoint) auto-captures reports auto-capture | Typed input, JSON, dry-run and MCP |
| 6 | List available backgrounds | Official OpenAPI | (generated endpoint) backgrounds | Typed input, JSON, dry-run and MCP |
| 7 | List library items | Official OpenAPI | (generated endpoint) library list-items | Typed input, JSON, dry-run and MCP |
| 8 | Add a source to the library | Official OpenAPI | (generated endpoint) library create-item | Typed input, JSON, dry-run and MCP |
| 9 | Generate media with AI | Official OpenAPI | (generated endpoint) library create-generation | Typed input, JSON, dry-run and MCP |
| 10 | Get a generation's status | Official OpenAPI | (generated endpoint) library get-generation | Typed input, JSON, dry-run and MCP |
| 11 | Import media from a URL | Official OpenAPI | (generated endpoint) library start-import | Typed input, JSON, dry-run and MCP |
| 12 | Get an import's status | Official OpenAPI | (generated endpoint) library get-import | Typed input, JSON, dry-run and MCP |
| 13 | Capture a web page or social post | Official OpenAPI | (generated endpoint) library capture-screenshot | Typed input, JSON, dry-run and MCP |
| 14 | Remove a library item | Official OpenAPI | (generated endpoint) library delete-item | Typed input, JSON, dry-run and MCP |
| 15 | Add a Tella Motion clip | Official OpenAPI | (generated endpoint) motion create-clip | Typed input, JSON, dry-run and MCP |
| 16 | List Tella Motion documents | Official OpenAPI | (generated endpoint) motion list-documents | Typed input, JSON, dry-run and MCP |
| 17 | Create a Tella Motion document | Official OpenAPI | (generated endpoint) motion create-document | Typed input, JSON, dry-run and MCP |
| 18 | Summarize a Tella Motion document | Official OpenAPI | (generated endpoint) motion get-document | Typed input, JSON, dry-run and MCP |
| 19 | Edit a Tella Motion document | Official OpenAPI | (generated endpoint) motion apply-commands | Typed input, JSON, dry-run and MCP |
| 20 | Inspect a composition | Official OpenAPI | (generated endpoint) motion get-composition | Typed input, JSON, dry-run and MCP |
| 21 | Download a Tella Motion document | Official OpenAPI | (generated endpoint) motion download-document | Typed input, JSON, dry-run and MCP |
| 22 | Duplicate a Tella Motion document | Official OpenAPI | (generated endpoint) motion duplicate-document | Typed input, JSON, dry-run and MCP |
| 23 | List a document's media | Official OpenAPI | (generated endpoint) motion list-media | Typed input, JSON, dry-run and MCP |
| 24 | Inspect a node's properties | Official OpenAPI | (generated endpoint) motion inspect-node | Typed input, JSON, dry-run and MCP |
| 25 | Get a node's stored fields | Official OpenAPI | (generated endpoint) motion get-node | Typed input, JSON, dry-run and MCP |
| 26 | Get a frame or preview of a Tella Motion document | Official OpenAPI | (generated endpoint) motion get-document-thumbnail | Typed input, JSON, dry-run and MCP |
| 27 | List Tella Motion effects | Official OpenAPI | (generated endpoint) motion list-effects | Typed input, JSON, dry-run and MCP |
| 28 | List Tella Motion fonts | Official OpenAPI | (generated endpoint) motion list-fonts | Typed input, JSON, dry-run and MCP |
| 29 | Get a Tella Motion JSON schema | Official OpenAPI | (generated endpoint) motion get-schema | Typed input, JSON, dry-run and MCP |
| 30 | List playlist groups | Official OpenAPI | (generated endpoint) playlist-groups list | Typed input, JSON, dry-run and MCP |
| 31 | Create a playlist group | Official OpenAPI | (generated endpoint) playlist-groups create | Typed input, JSON, dry-run and MCP |
| 32 | Delete a playlist group | Official OpenAPI | (generated endpoint) playlist-groups delete | Typed input, JSON, dry-run and MCP |
| 33 | Rename or reorder a playlist group | Official OpenAPI | (generated endpoint) playlist-groups update | Typed input, JSON, dry-run and MCP |
| 34 | List all playlists | Official OpenAPI | (generated endpoint) playlists list | Typed input, JSON, dry-run and MCP |
| 35 | Create a new playlist | Official OpenAPI | (generated endpoint) playlists create | Typed input, JSON, dry-run and MCP |
| 36 | Delete a playlist | Official OpenAPI | (generated endpoint) playlists delete | Typed input, JSON, dry-run and MCP |
| 37 | Get playlist details | Official OpenAPI | (generated endpoint) playlists get | Typed input, JSON, dry-run and MCP |
| 38 | Update a playlist | Official OpenAPI | (generated endpoint) playlists update | Typed input, JSON, dry-run and MCP |
| 39 | Add a video to a playlist | Official OpenAPI | (generated endpoint) playlists videos add-to-playlist | Typed input, JSON, dry-run and MCP |
| 40 | Remove a video from a playlist | Official OpenAPI | (generated endpoint) playlists videos remove-from-playlist | Typed input, JSON, dry-run and MCP |
| 41 | List connected social accounts | Official OpenAPI | (generated endpoint) social-accounts | Typed input, JSON, dry-run and MCP |
| 42 | Create a source upload (video, audio, image, or LUT) | Official OpenAPI | (generated endpoint) sources create | Typed input, JSON, dry-run and MCP |
| 43 | Get a source's processing status | Official OpenAPI | (generated endpoint) sources get | Typed input, JSON, dry-run and MCP |
| 44 | Get the beats of an audio or video source | Official OpenAPI | (generated endpoint) sources beats get-source | Typed input, JSON, dry-run and MCP |
| 45 | Get a streaming upload storyboard | Official OpenAPI | (generated endpoint) sources storyboard get-source | Typed input, JSON, dry-run and MCP |
| 46 | Get a thumbnail of a source on no clip | Official OpenAPI | (generated endpoint) sources thumbnail get-standalone-source | Typed input, JSON, dry-run and MCP |
| 47 | Get the transcript of a source on no clip | Official OpenAPI | (generated endpoint) sources transcript get-source | Typed input, JSON, dry-run and MCP |
| 48 | Get the audio waveform JSON for a source on no clip | Official OpenAPI | (generated endpoint) sources waveform get-standalone-source | Typed input, JSON, dry-run and MCP |
| 49 | List subtitle presets | Official OpenAPI | (generated endpoint) subtitle-presets list | Typed input, JSON, dry-run and MCP |
| 50 | Save a subtitle preset | Official OpenAPI | (generated endpoint) subtitle-presets save | Typed input, JSON, dry-run and MCP |
| 51 | List all tags | Official OpenAPI | (generated endpoint) tags list | Typed input, JSON, dry-run and MCP |
| 52 | Create a new tag | Official OpenAPI | (generated endpoint) tags create | Typed input, JSON, dry-run and MCP |
| 53 | Delete a tag | Official OpenAPI | (generated endpoint) tags delete | Typed input, JSON, dry-run and MCP |
| 54 | Update a tag | Official OpenAPI | (generated endpoint) tags update | Typed input, JSON, dry-run and MCP |
| 55 | List text presets | Official OpenAPI | (generated endpoint) text-presets list | Typed input, JSON, dry-run and MCP |
| 56 | Save a text preset | Official OpenAPI | (generated endpoint) text-presets save | Typed input, JSON, dry-run and MCP |
| 57 | Delete a text preset | Official OpenAPI | (generated endpoint) text-presets delete | Typed input, JSON, dry-run and MCP |
| 58 | List all videos | Official OpenAPI | (generated endpoint) videos list | Typed input, JSON, dry-run and MCP |
| 59 | Create a video | Official OpenAPI | (generated endpoint) videos create | Typed input, JSON, dry-run and MCP |
| 60 | Search videos | Official OpenAPI | (generated endpoint) videos search | Typed input, JSON, dry-run and MCP |
| 61 | Delete a video | Official OpenAPI | (generated endpoint) videos delete | Typed input, JSON, dry-run and MCP |
| 62 | Get video information | Official OpenAPI | (generated endpoint) videos get | Typed input, JSON, dry-run and MCP |
| 63 | Update a video | Official OpenAPI | (generated endpoint) videos update | Typed input, JSON, dry-run and MCP |
| 64 | Get video analytics | Official OpenAPI | (generated endpoint) videos analytics get-video | Typed input, JSON, dry-run and MCP |
| 65 | Remove background music | Official OpenAPI | (generated endpoint) videos background-music remove | Typed input, JSON, dry-run and MCP |
| 66 | Set background music | Official OpenAPI | (generated endpoint) videos background-music set | Typed input, JSON, dry-run and MCP |
| 67 | Get the beats of the background music | Official OpenAPI | (generated endpoint) videos background-music get-beats | Typed input, JSON, dry-run and MCP |
| 68 | Download video captions | Official OpenAPI | (generated endpoint) videos captions get-video | Typed input, JSON, dry-run and MCP |
| 69 | Get video chapters | Official OpenAPI | (generated endpoint) videos chapters get-video | Typed input, JSON, dry-run and MCP |
| 70 | Replace video chapters | Official OpenAPI | (generated endpoint) videos chapters set-video | Typed input, JSON, dry-run and MCP |
| 71 | List clips for a video | Official OpenAPI | (generated endpoint) videos clips list | Typed input, JSON, dry-run and MCP |
| 72 | Add a clip from an uploaded source | Official OpenAPI | (generated endpoint) videos clips add | Typed input, JSON, dry-run and MCP |
| 73 | Delete a clip | Official OpenAPI | (generated endpoint) videos clips delete | Typed input, JSON, dry-run and MCP |
| 74 | Get a clip | Official OpenAPI | (generated endpoint) videos clips get | Typed input, JSON, dry-run and MCP |
| 75 | Update a clip | Official OpenAPI | (generated endpoint) videos clips update | Typed input, JSON, dry-run and MCP |
| 76 | Get audio levels in a clip | Official OpenAPI | (generated endpoint) videos clips get-audio-levels | Typed input, JSON, dry-run and MCP |
| 77 | Auto-crop a clip's screen recording | Official OpenAPI | (generated endpoint) videos clips auto-crop | Typed input, JSON, dry-run and MCP |
| 78 | List blurs on a clip | Official OpenAPI | (generated endpoint) videos clips list-blurs | Typed input, JSON, dry-run and MCP |
| 79 | Add a blur to a clip | Official OpenAPI | (generated endpoint) videos clips add-blur | Typed input, JSON, dry-run and MCP |
| 80 | Remove a blur | Official OpenAPI | (generated endpoint) videos clips remove-blur | Typed input, JSON, dry-run and MCP |
| 81 | Update a blur | Official OpenAPI | (generated endpoint) videos clips update-blur | Typed input, JSON, dry-run and MCP |
| 82 | List faces in a clip's camera | Official OpenAPI | (generated endpoint) videos clips list-camera-faces | Typed input, JSON, dry-run and MCP |
| 83 | Cut one or more time ranges from a clip | Official OpenAPI | (generated endpoint) videos clips cut | Typed input, JSON, dry-run and MCP |
| 84 | Cut clip ranges by transcript word indices | Official OpenAPI | (generated endpoint) videos clips cut-by-transcript | Typed input, JSON, dry-run and MCP |
| 85 | Duplicate a clip | Official OpenAPI | (generated endpoint) videos clips duplicate | Typed input, JSON, dry-run and MCP |
| 86 | Get clip color filters | Official OpenAPI | (generated endpoint) videos clips get-filters | Typed input, JSON, dry-run and MCP |
| 87 | Set clip color filters | Official OpenAPI | (generated endpoint) videos clips set-filters | Typed input, JSON, dry-run and MCP |
| 88 | List highlights on a clip | Official OpenAPI | (generated endpoint) videos clips list-highlights | Typed input, JSON, dry-run and MCP |
| 89 | Add a highlight to a clip | Official OpenAPI | (generated endpoint) videos clips add-highlight | Typed input, JSON, dry-run and MCP |
| 90 | Remove a highlight | Official OpenAPI | (generated endpoint) videos clips remove-highlight | Typed input, JSON, dry-run and MCP |
| 91 | Update a highlight | Official OpenAPI | (generated endpoint) videos clips update-highlight | Typed input, JSON, dry-run and MCP |
| 92 | List key presses on a clip | Official OpenAPI | (generated endpoint) videos clips list-key-events | Typed input, JSON, dry-run and MCP |
| 93 | List layouts on a clip | Official OpenAPI | (generated endpoint) videos clips list-layouts | Typed input, JSON, dry-run and MCP |
| 94 | Add a layout to a clip | Official OpenAPI | (generated endpoint) videos clips add-layout | Typed input, JSON, dry-run and MCP |
| 95 | Remove a layout | Official OpenAPI | (generated endpoint) videos clips remove-layout | Typed input, JSON, dry-run and MCP |
| 96 | Update a layout | Official OpenAPI | (generated endpoint) videos clips update-layout | Typed input, JSON, dry-run and MCP |
| 97 | List mouse events on a clip | Official OpenAPI | (generated endpoint) videos clips list-mouse-events | Typed input, JSON, dry-run and MCP |
| 98 | List overlays on a clip | Official OpenAPI | (generated endpoint) videos clips list-overlays | Typed input, JSON, dry-run and MCP |
| 99 | Add an overlay to a clip | Official OpenAPI | (generated endpoint) videos clips add-overlay | Typed input, JSON, dry-run and MCP |
| 100 | Remove an overlay | Official OpenAPI | (generated endpoint) videos clips remove-overlay | Typed input, JSON, dry-run and MCP |
| 101 | Update an overlay | Official OpenAPI | (generated endpoint) videos clips update-overlay | Typed input, JSON, dry-run and MCP |
| 102 | Remove filler words from a clip | Official OpenAPI | (generated endpoint) videos clips remove-fillers | Typed input, JSON, dry-run and MCP |
| 103 | Remove silences from a clip | Official OpenAPI | (generated endpoint) videos clips remove-silences | Typed input, JSON, dry-run and MCP |
| 104 | Reorder a clip | Official OpenAPI | (generated endpoint) videos clips reorder | Typed input, JSON, dry-run and MCP |
| 105 | List shape overlays on a clip | Official OpenAPI | (generated endpoint) videos clips list-shape-overlays | Typed input, JSON, dry-run and MCP |
| 106 | Add a shape overlay to a clip | Official OpenAPI | (generated endpoint) videos clips add-shape-overlay | Typed input, JSON, dry-run and MCP |
| 107 | Remove a shape overlay | Official OpenAPI | (generated endpoint) videos clips remove-shape-overlay | Typed input, JSON, dry-run and MCP |
| 108 | Update a shape overlay | Official OpenAPI | (generated endpoint) videos clips update-shape-overlay | Typed input, JSON, dry-run and MCP |
| 109 | Get silent ranges in a clip | Official OpenAPI | (generated endpoint) videos clips get-silences | Typed input, JSON, dry-run and MCP |
| 110 | List sound effects on a clip | Official OpenAPI | (generated endpoint) videos clips list-sound-effects | Typed input, JSON, dry-run and MCP |
| 111 | Add a sound effect to a clip | Official OpenAPI | (generated endpoint) videos clips add-sound-effect | Typed input, JSON, dry-run and MCP |
| 112 | Remove a sound effect | Official OpenAPI | (generated endpoint) videos clips remove-sound-effect | Typed input, JSON, dry-run and MCP |
| 113 | Update a sound effect | Official OpenAPI | (generated endpoint) videos clips update-sound-effect | Typed input, JSON, dry-run and MCP |
| 114 | List sources for a clip | Official OpenAPI | (generated endpoint) videos clips list-sources | Typed input, JSON, dry-run and MCP |
| 115 | Get a source thumbnail or animated preview | Official OpenAPI | (generated endpoint) videos clips get-source-thumbnail | Typed input, JSON, dry-run and MCP |
| 116 | Get the audio waveform JSON for a source | Official OpenAPI | (generated endpoint) videos clips get-source-waveform | Typed input, JSON, dry-run and MCP |
| 117 | Split a clip in two | Official OpenAPI | (generated endpoint) videos clips split | Typed input, JSON, dry-run and MCP |
| 118 | Get a clip storyboard | Official OpenAPI | (generated endpoint) videos clips get-storyboard | Typed input, JSON, dry-run and MCP |
| 119 | List text overlays on a clip | Official OpenAPI | (generated endpoint) videos clips list-text-overlays | Typed input, JSON, dry-run and MCP |
| 120 | Add a text overlay to a clip | Official OpenAPI | (generated endpoint) videos clips add-text-overlay | Typed input, JSON, dry-run and MCP |
| 121 | Remove a text overlay | Official OpenAPI | (generated endpoint) videos clips remove-text-overlay | Typed input, JSON, dry-run and MCP |
| 122 | Update a text overlay | Official OpenAPI | (generated endpoint) videos clips update-text-overlay | Typed input, JSON, dry-run and MCP |
| 123 | Get a clip thumbnail or animated preview | Official OpenAPI | (generated endpoint) videos clips get-thumbnail | Typed input, JSON, dry-run and MCP |
| 124 | Get clip transcript | Official OpenAPI | (generated endpoint) videos clips get-cut-transcript | Typed input, JSON, dry-run and MCP |
| 125 | Correct transcript words | Official OpenAPI | (generated endpoint) videos clips update-transcript-words | Typed input, JSON, dry-run and MCP |
| 126 | List zooms on a clip | Official OpenAPI | (generated endpoint) videos clips list-zooms | Typed input, JSON, dry-run and MCP |
| 127 | Add a zoom to a clip | Official OpenAPI | (generated endpoint) videos clips add-zoom | Typed input, JSON, dry-run and MCP |
| 128 | Auto-generate zooms from mouse activity | Official OpenAPI | (generated endpoint) videos clips generate-auto-zooms | Typed input, JSON, dry-run and MCP |
| 129 | Remove a zoom | Official OpenAPI | (generated endpoint) videos clips remove-zoom | Typed input, JSON, dry-run and MCP |
| 130 | Update a zoom | Official OpenAPI | (generated endpoint) videos clips update-zoom | Typed input, JSON, dry-run and MCP |
| 131 | Add a collaborator to a video | Official OpenAPI | (generated endpoint) videos collaborators add-to-video | Typed input, JSON, dry-run and MCP |
| 132 | Duplicate a video | Official OpenAPI | (generated endpoint) videos duplicate video | Typed input, JSON, dry-run and MCP |
| 133 | Apply many video edits | Official OpenAPI | (generated endpoint) videos apply-edits | Typed input, JSON, dry-run and MCP |
| 134 | List video exports | Official OpenAPI | (generated endpoint) videos exports list-video | Typed input, JSON, dry-run and MCP |
| 135 | Start a video export | Official OpenAPI | (generated endpoint) videos exports video | Typed input, JSON, dry-run and MCP |
| 136 | Get video export status | Official OpenAPI | (generated endpoint) videos exports get-video | Typed input, JSON, dry-run and MCP |
| 137 | Get a live preview link | Official OpenAPI | (generated endpoint) videos live-preview get-link | Typed input, JSON, dry-run and MCP |
| 138 | Cancel a scheduled publish | Official OpenAPI | (generated endpoint) videos publish cancel-scheduled-publication | Typed input, JSON, dry-run and MCP |
| 139 | List social publications | Official OpenAPI | (generated endpoint) videos publish list-video-publications | Typed input, JSON, dry-run and MCP |
| 140 | Publish a video to social media | Official OpenAPI | (generated endpoint) videos publish video | Typed input, JSON, dry-run and MCP |
| 141 | Get a recording link | Official OpenAPI | (generated endpoint) videos recording-link get | Typed input, JSON, dry-run and MCP |
| 142 | Get a video storyboard | Official OpenAPI | (generated endpoint) videos storyboard get-video | Typed input, JSON, dry-run and MCP |
| 143 | Apply a subtitle preset | Official OpenAPI | (generated endpoint) videos subtitle-preset apply | Typed input, JSON, dry-run and MCP |
| 144 | List a video's tags | Official OpenAPI | (generated endpoint) videos tags list-video | Typed input, JSON, dry-run and MCP |
| 145 | Add a tag to a video | Official OpenAPI | (generated endpoint) videos tags add-video | Typed input, JSON, dry-run and MCP |
| 146 | Remove a tag from a video | Official OpenAPI | (generated endpoint) videos tags remove-video | Typed input, JSON, dry-run and MCP |
| 147 | Remove a video's custom thumbnail | Official OpenAPI | (generated endpoint) videos thumbnail remove-video | Typed input, JSON, dry-run and MCP |
| 148 | Get a video thumbnail or animated preview | Official OpenAPI | (generated endpoint) videos thumbnail get-video | Typed input, JSON, dry-run and MCP |
| 149 | Set a video's thumbnail | Official OpenAPI | (generated endpoint) videos thumbnail set-video | Typed input, JSON, dry-run and MCP |
| 150 | Get a video's timeline | Official OpenAPI | (generated endpoint) videos timeline get-video | Typed input, JSON, dry-run and MCP |
| 151 | List webhook endpoints | Official OpenAPI | (generated endpoint) webhooks list-endpoints | Typed input, JSON, dry-run and MCP |
| 152 | Create a webhook endpoint | Official OpenAPI | (generated endpoint) webhooks create-endpoint | Typed input, JSON, dry-run and MCP |
| 153 | Delete a webhook endpoint | Official OpenAPI | (generated endpoint) webhooks delete-endpoint | Typed input, JSON, dry-run and MCP |
| 154 | Update a webhook endpoint | Official OpenAPI | (generated endpoint) webhooks update-endpoint | Typed input, JSON, dry-run and MCP |
| 155 | Get endpoint signing secret | Official OpenAPI | (generated endpoint) webhooks get-endpoint-secret | Typed input, JSON, dry-run and MCP |
| 156 | List recent webhook messages | Official OpenAPI | (generated endpoint) webhooks list-messages | Typed input, JSON, dry-run and MCP |
| 157 | Get a specific webhook message | Official OpenAPI | (generated endpoint) webhooks get-message | Typed input, JSON, dry-run and MCP |
| 158 | Cross-video transcript search | Prior published CLI and patches | (behavior in tella-pp-cli transcripts search) preserve documented workflow | Compatibility and safe errors |
| 159 | Watch-milestone digest | Prior published CLI and patches | (behavior in tella-pp-cli videos viewed) preserve documented workflow | Compatibility and safe errors |
| 160 | Webhook tail + replay | Prior published CLI and patches | (behavior in tella-pp-cli webhooks tail) preserve documented workflow | Compatibility and safe errors |
| 161 | Bulk standard edit pass | Prior published CLI and patches | (behavior in tella-pp-cli clips edit-pass) preserve documented workflow | Compatibility and safe errors |
| 162 | Transcript diff (cut vs uncut) | Prior published CLI and patches | (behavior in tella-pp-cli clips transcript-diff) preserve documented workflow | Compatibility and safe errors |
| 163 | Upload-to-edit workflow | Prior published CLI and patches | (behavior in tella-pp-cli sources upload-file) preserve documented workflow | Compatibility and safe errors |
| 164 | Exports waitlist | Prior published CLI and patches | (behavior in tella-pp-cli exports wait) preserve documented workflow | Compatibility and safe errors |
| 165 | Caption-file export | Prior published CLI and patches | (behavior in tella-pp-cli clips captions) preserve documented workflow | Compatibility and safe errors |
| 166 | Workspace stats | Prior published CLI and patches | (behavior in tella-pp-cli workspace stats) preserve documented workflow | Compatibility and safe errors |
| 167 | Declarative video storyboard | Prior published CLI and patches | (behavior in tella-pp-cli videos storyboard) preserve documented workflow | Compatibility and safe errors |
| 168 | One-shot media insertion | Prior published CLI and patches | (behavior in tella-pp-cli videos clips insert-file) preserve documented workflow | Compatibility and safe errors |
| 169 | Transcript-driven repairs | Prior published CLI and patches | (behavior in tella-pp-cli videos clips cut-words) preserve documented workflow | Compatibility and safe errors |
| 170 | Pre-publish edit QA | Prior published CLI and patches | (behavior in tella-pp-cli videos audit) preserve documented workflow | Compatibility and safe errors |
| 171 | Playlist-scale edit planning | Prior published CLI and patches | (behavior in tella-pp-cli playlists edit-pass) preserve documented workflow | Compatibility and safe errors |
| 172 | Safe clip clean workflow | Prior published CLI and patches | (behavior in tella-pp-cli videos clips clean) preserve documented workflow | Compatibility and safe errors |
| 173 | Cut rollback workflow | Prior published CLI and patches | (behavior in tella-pp-cli videos clips undo-last-cuts) preserve documented workflow | Compatibility and safe errors |
| 174 | Live preview alias | User brief / official API | (behavior in tella-pp-cli videos preview) URL and expiry output | URL and expiry output |
| 175 | Credit-gated Agent fallback | User brief / official API | (behavior in tella-pp-cli agent-chats start) Explicit consent and balance checks | Explicit consent and balance checks |
| 176 | Write identity | User brief / official API | (behavior in tella-pp-cli videos update) Idempotency-Key for every shared-client write | Idempotency-Key for every shared-client write |
| 177 | Compact timeline | User brief / official API | (behavior in tella-pp-cli videos timeline) Readable playback outline | Readable playback outline |

### Transcendence (approved hand-coded workflows)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Automatic video edit pass | videos auto | hand-code | Apply selected automatic edits across clips with recovery snapshots and finish with a live preview. | none |
| 2 | House-style lesson preparation | lessons pass | hand-code | Prepare a lesson from a validated recipe and finish with a preview for review. | none |
| 3 | Reviewed lesson delivery | lessons ship | hand-code | Export, wait, and download a lesson MP4 and SRT or VTT captions. | none |
| 4 | Conservative retake plan | clips find-retakes | hand-code | Propose repeated sentence cuts with timestamps and apply reviewed cuts under snapshots. | none |
| 5 | Review storyboard files | videos storyboard-image | hand-code | Save video, clip, or source storyboard images for visual inspection. | none |

## Reconciliation and gate decision
All 16 prior features and all patch contracts remain required. The seven user-requested groups are approved; no stubs are approved. Auto layouts fail clearly because no public one-shot endpoint exists. Cookie mistakes are retired with official retake/Agent pointers.

The separate 2026-11-05-novel-features-brainstorm.md retains the customer model, candidates, adversarial cut and prior verdict audit. BROWSER-ONLY.md is required. Recommended gate choices follow the explicit user instruction.

Shipping paths were rendered from the current runtime tree without changing approved scope. Agent chat starts use the guarded workflow rather than a bypassable generated POST handler.
