# OpenArt Browser-Sniff Report

**Run ID:** 20260513-152641
**Backend:** chrome-MCP (drove the user's authenticated Chrome session)
**Auth state:** logged in as Matt Van Horn, "Matt Van Horn's workspace", Infinite plan, ~43,206 credits
**Discovery cost:** 150 credits (one Grok Imagine 720p/5s generation submitted to capture the contract)

## Reachability

- Marketing/SPA pages (`/`, `/suite/...`) return `200` over plain stdlib HTTP. No Cloudflare challenge.
- `/api/...` and `/suite/api/...` endpoints behind Cloudflare; require valid Clerk session cookies.
- Auth provider: **Clerk** — visible cookies include `__client_uat`, `__client_uat_<hash>`. Real session cookie (`__session`) is HttpOnly and not visible to `document.cookie`.
- All XHR is JSON over fetch.
- No GraphQL. No proxy-envelope pattern. Plain REST.
- Runtime mode: `browser_clearance_http` — printed CLI ships Chrome cookie import (`auth login --chrome`) + Surf/standard HTTP for replay.

## Confirmed Endpoints

| Method | Path | Notes |
|---|---|---|
| POST | `/suite/api/user/my-info` | User identity + credit balance (`free_credit_balance` field), subscription, settings |
| POST | `/suite/api/user/current-workspace` | Currently active workspace; body `{userId: ""}` works |
| GET | `/suite/api/user/last-active` | Heartbeat |
| GET | `/suite/api/user/settings` | User settings |
| GET | `/suite/api/workspace/list` | All workspaces the user is in: `[{id, team_id, team_name, team_description, team_avatar, ...}]` |
| POST | `/suite/api/workspace/update` | Update workspace |
| GET | `/suite/api/team/members` | Team members in active workspace |
| GET | `/suite/api/projects` | `{data: [{id, name, isDefault, createdAt, updatedAt}]}` |
| GET | `/suite/api/projects/default` | `{projectId}` |
| POST | `/suite/api/projects/transfer` | Move resources between projects |
| GET | `/suite/api/projects/{projectId}/folders?limit=50` | `{data: [], hasMore: false}` |
| GET | `/suite/api/uploaded-assets?limit=20&projectId={id}` | User's uploaded reference images |
| POST | `/suite/api/upload/sign` | Get a signed upload URL for a new reference image |
| POST | `/suite/api/upload/persist` | Persist an uploaded image as an asset |
| GET | `/suite/api/resources?cursor&folderIdNull&limit&projectId` | Paginated media library: every generation + upload, with `input`/`params`/`status`/`url`/`thumbnailUrl`/`metadata` |
| GET | `/suite/api/resources/{resourceId}` | Single resource (the polling endpoint for in-flight generations) |
| GET | `/suite/api/history/{historyId}` | History entry with `capability_id`, `request_form_id`, `tool` |
| **POST** | **`/suite/api/forms/creations/<capability-id>`** | **THE generation submit endpoint.** `<capability-id>` is URL-encoded `<model-slug>:<form-type>` (e.g. `grok-imagine:text2video` → `grok-imagine%3Atext2video`). |
| GET | `/suite/api/credits/logs` | Credit ledger: `{success, entries: [{id, sequenceId, type, amount, creditField, balanceBefore, balanceAfter, reference: {businessType, businessId}, businessDetails: [{subBusinessType, unitCredits, quantity}]}]}` |
| GET | `/suite/api/templates` | Saved generation templates `{success, data: [{id, tool, name, originInput: {prompt, ...}}]}` |
| GET | `/suite/api/album-collections` | Curated albums |
| GET | `/suite/api/realtime` | Realtime subscription (likely WebSocket; not exercised) |
| GET | `/suite/api/server-events` | SSE channel (not exercised) |
| POST | `/suite/api/enhance-prompt` | Auto-polish (LLM rewrite) of a user prompt |
| POST | `/suite/api/image-to-prompt` | Reverse: image → suggested prompt |
| POST | `/suite/api/character/batch-delete` | Bulk delete characters |
| POST | `/suite/api/community/submit` | Publish a generation to community feed |
| GET | `/suite/api/community/submit/status` | Community submission status |
| POST | `/suite/api/copyright-checks/batch` | Pre-publish copyright check |
| GET | `/suite/api/coupon/claimable` | Available coupons |
| POST | `/suite/api/feedback/submit` | User feedback |
| GET | `/suite/api/forms/templates` | Form templates |
| GET | `/suite/api/stripe/subscription` | Subscription state |
| GET | `/suite/api/system/banner` | System banner messages |
| POST | `/suite/api/topaz/estimate` | Pre-submit cost estimate (named after OpenArt's pricing engine, not Topaz Labs) |
| POST | `/suite/api/world/pano` | World/3D panorama generation |
| POST | `/suite/api/world/photo` | World photo generation |
| POST | `/suite/api/world/prompt-image` | World prompt-to-image |
| GET | `/suite/api/auth/sso/authorize` | Clerk SSO authorize endpoint |

## Confirmed Generation Contract (end-to-end)

### Submit

```
POST /suite/api/forms/creations/<capability-id-url-encoded>
Content-Type: application/json
Cookie: <clerk session>
```

Body (text-to-video):
```json
{
  "prompt": "test cli probe",
  "videoCount": 1,
  "duration": 5,
  "aspectRatio": "16:9",
  "resolution": "720p",
  "autoEnhancePrompt": false,
  "enableUnlimited": true,
  "model": "grok-imagine",
  "projectId": "<projectId>",
  "folderId": null
}
```

Response:
```json
{
  "historyId": "UUnscfw4ZJuBWdjHRiPz",
  "resourceIds": ["3dVHEhDjyq82gLwBudaG"]
}
```

### Poll

```
GET /suite/api/resources/<resourceId>
```

Response (in-flight): `status` not yet `"completed"`, `url` empty.

Response (completed):
```json
{
  "data": {
    "id": "...",
    "sourceType": "generation",
    "userId": "...",
    "url": "https://cdn.openart.ai/openart-ai/production/2026-05/create-video/<userId>/<id>.mp4",
    "thumbnailUrl": "https://cdn.openart.ai/openart/thumbnail/production/2026-05/create-video/<userId>/<id>.webp",
    "resourceType": "video",
    "metadata": {
      "media_type": "video",
      "formats": ["mov", "mp4", "m4a", "3gp", "3g2", "mj2"],
      "width": 1280,
      "height": 720,
      "duration": 5.041667,
      "fps": 24,
      "video_codec": "h264",
      "video_bitrate_kbps": 1232,
      "has_audio": true,
      "audio_codec": "aac",
      "audio_sample_rate": 44100,
      "audio_channels": 2
    },
    "input": { "prompt": "...", "videoCount": 1, "duration": 5, ... },
    "params": { "prompt": "...", "duration": 5, "aspect_ratio": "16:9", "resolution": "720p" },
    "status": "completed",
    "isStarred": false,
    "isDownloaded": false,
    "createdAt": 1778712069861,
    "generation": {
      "historyId": "...",
      "capabilityId": "grok-imagine:text2video",
      "requestFormId": "grok-imagine:text2video",
      "tool": "text-to-video"
    }
  }
}
```

### Download

The `url` field of a completed resource is a public CDN URL (`https://cdn.openart.ai/openart-ai/production/...`). No auth required to GET. Stream with normal HTTP and write to local file.

## Capability ID Conventions

`capability_id = "<model-slug>:<form-type>"`. URL-encoded as `%3A` between segments.

| Form Type | Tool | Route Convention |
|---|---|---|
| `text2video` | text-to-video | `/suite/create-video/<model-slug>` |
| `image2video` (inferred) | image-to-video / animate | `/suite/animate-video/<model-slug>` |
| Other forms exist for image gen, lip-sync, motion-sync, etc. (not exercised; same submit shape applies). |

Confirmed model slugs:
- `byte-plus-seedance-2` → Seedance 2.0 (user's headline model; 800 credits per 720p/10s video)
- `kling2-6` → Kling 2.6 (100 credits per 1080p/5s frame-to-video)
- `grok-imagine` → Grok Imagine (150 credits per 720p/5s text-to-video, ~15s wall-clock)

Other models seen in the picker (slugs not directly captured but inferable from URL convention):
Seedance 1.5 Pro, Kling 3.0 / 3.0 Omni, Veo 3.1, Wan 2.6 / 2.7, HappyHorse, plus image and lip-sync models.

## Cookie / Auth Notes

- `__session` (HttpOnly) carries the Clerk JWT.
- `__client_uat` and `__client_uat_<hash>` are visible session anchors.
- For the printed CLI: `auth login --chrome` must read all OpenArt cookies (including HttpOnly) from the user's Chrome cookie DB. Then a stored cookie jar is used for replay through Surf or standard HTTP.

## Replayability Verdict

**PASS.** Every observed generation surface uses plain HTTP with cookie auth. Polling is straightforward. No need for live page-context execution. The printed CLI ships as a normal CLI with Chrome cookie import.
