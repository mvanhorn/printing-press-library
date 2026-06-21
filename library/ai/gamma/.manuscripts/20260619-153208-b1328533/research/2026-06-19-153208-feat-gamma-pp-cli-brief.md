# Gamma API Research Brief
Run: 20260619-153208-b1328533

## API Summary

**Base URL:** `https://public-api.gamma.app`  
**Auth:** `X-API-KEY` header (NOT Authorization Bearer). Env var: `GAMMA_API_KEY`. Key format: `sk-gamma-...`  
**Plans:** Pro, Ultra, Teams, Business required for API keys. Connectors work on all plans.  
**Credits:** 1-3/card (text), 2-125/image (model-dependent). Returned in `credits.deducted` / `credits.remaining`.

## Endpoints (7 total)

### Generate
| Method | Path | Key Params |
|--------|------|-----------|
| POST | `/v1.0/generations` | `inputText` (req), `format`, `textMode`, `numCards`, `themeId`, `folderIds`, `exportAs`, `additionalInstructions`, `textOptions`, `imageOptions`, `cardOptions`, `sharingOptions` |
| POST | `/v1.0/generations/from-template` | `prompt` (req), `gammaId` (req) + subset of generation params |
| GET | `/v1.0/generations/{id}` | Poll → `status`: pending/completed/failed; on complete: `gammaId`, `gammaUrl`, `exportUrl`, `credits` |

### Workspace
| Method | Path | Key Params |
|--------|------|-----------|
| GET | `/v1.0/themes` | `query`, `limit` (1-50), `after`, `type` (standard/custom) |
| GET | `/v1.0/folders` | `query`, `limit`, `after` |

### Management
| Method | Path | Notes |
|--------|------|-------|
| POST | `/v1.0/gammas/{gammaId}/archive` | Idempotent. Returns `{gammaId, archived: true}`. **gammaId must be API file ID (`g_...`), NOT URL slug.** |
| DELETE | `/v1.0/gammas/{gammaId}` | Requires workspace admin role. Auto-archives first. |

## Async Pattern
1. POST → get `generationId`
2. Poll GET every 5s → wait for `completed` or `failed`
3. Generations take 1-3 min typically (up to 5+ min for 40-card + AI images)
4. Export URLs expire in ~1 week. PNG export = `.zip` with one PNG per card.

## Rate Limits
Response headers: `x-ratelimit-remaining-burst`, `x-ratelimit-remaining`, `x-ratelimit-remaining-daily`  
429 → back off 30s + exponential backoff.

## Error Codes
| Code | Meaning |
|------|---------|
| 400 | Invalid params (check enum values, required fields, inputText 1-400000 chars) |
| 401 | Bad/missing X-API-KEY |
| 402 | Insufficient credits |
| 403 | No permission; on archive = gammaId is URL slug not file ID |
| 404 | Generation not found (wrong generationId) |
| 429 | Rate limited |
| 500/502 | Gamma server error |

## Key Enum Values

**format:** `presentation`, `document`, `social`, `webpage`  
**textMode:** `generate`, `condense`, `preserve`  
**cardSplit:** `inputTextBreaks`, `auto`  
**exportAs:** `pptx`, `pdf`, `png`  
**textOptions.amount:** `brief`, `medium`, `detailed`, `extensive`  
**imageOptions.source:** `webAllImages`, `webFreeToUse`, `webFreeToUseCommercially`, `aiGenerated`, `pictographic`, `giphy`, `pexels`, `placeholder`, `noImages`, `themeAccent`  
**cardOptions.dimensions:** `fluid`, `16x9`, `4x3`, `pageless`, `letter`, `a4`, `1x1`, `4x5`, `9x16`  
**sharingOptions.workspaceAccess:** `edit`, `comment`, `view`, `noAccess`, `fullAccess`  
**sharingOptions.externalAccess:** `edit`, `comment`, `view`, `noAccess`  
**imageOptions.model:** dall-e-3, imagen-3-flash, imagen-3-pro, imagen-4-pro, imagen-4-ultra, ideogram-v3, ideogram-v3-turbo, ideogram-v3-quality, flux-1-pro, flux-1-quick, flux-1-ultra, flux-kontext-pro, flux-kontext-max, flux-kontext-fast, leonardo-phoenix, recraft-v3, recraft-v3-svg, recraft-v4, recraft-v4-svg, recraft-v4-pro, luma-photon-1, luma-photon-flash-1, gpt-image-1-medium, gpt-image-1-high, gpt-image-1-mini-low, gpt-image-1-mini-medium, gpt-image-1-mini-high, gpt-image-2-mini, gpt-image-2, gpt-image-2-hd, gemini-2.5-flash-image, veo-3.1-fast, veo-3.1, luma-ray-2-flash, luma-ray-2, leonardo-motion-2-fast, leonardo-motion-2, gemini-3-pro-image, gemini-3-pro-image-hd, gemini-3.1-flash-image-mini, gemini-3.1-flash-image, gemini-3.1-flash-image-hd, flux-2-pro, flux-2-flex, flux-2-max, flux-2-klein

## headerFooter positions
topLeft, topCenter, topRight, bottomLeft, bottomCenter, bottomRight  
element types: `text`, `image`, `cardNumber`  
image source: `custom` (requires src URL), `themeLogo`  
image sizes: `sm`, `md`, `lg`, `xl`

## Reachability
Probed `GET /v1.0/themes` with invalid key → 401 (expected). API is live.

## Ecosystem
- No standalone CLI for gamma.app exists
- Official MCP server: `statechangelabs/gamma-app-mcp` (Node.js)
- Community MCP: `rarkhipov/gamma-mcp-server`
- Python SDK: `gamma_ai_python_sdk_ppt_generator` (GitHub, typed Pydantic)
- MCP-only tools NOT in REST API: `get_gammas` (list user's gammas), `read_gamma` (read content)

## What's NOT in REST API
- Cannot list existing gammas (MCP OAuth only)
- Cannot read/edit existing gamma content (MCP OAuth only)
- Cannot update a gamma after generation (create new instead)
