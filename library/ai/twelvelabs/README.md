# Twelvelabs CLI

**Turn long videos into editor-ready cut plans.**

This CLI wraps the Twelve Labs API and adds workflow commands for video upload, indexing waits, structured briefs, embeddings, and local clip cutting. Agents can go from a long source video to JSON or Markdown editing guidance without manually watching the whole file.

## Install

The recommended path installs both the `twelvelabs-pp-cli` binary and the `pp-twelvelabs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install twelvelabs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install twelvelabs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install twelvelabs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install twelvelabs --agent claude-code
npx -y @mvanhorn/printing-press-library install twelvelabs --agent claude-code --agent codex
```

### Without Node

If `npx` is not available, install the CLI directly with Go:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/twelvelabs/cmd/twelvelabs-pp-cli@latest
```

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/twelvelabs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-twelvelabs --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-twelvelabs --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install twelvelabs --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/twelvelabs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TWELVELABS_X_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "twelvelabs": {
      "command": "twelvelabs-pp-mcp",
      "env": {
        "TWELVELABS_X_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export TWELVELABS_X_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/twelvelabs-video-understanding-pp-cli/config.toml`.

### 3. Verify Setup

```bash
twelvelabs-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
twelvelabs-pp-cli assets list
```

### 5. Create an Editor Brief

Upload or register a video, wait for indexing, then produce an editor-ready plan:

```bash
twelvelabs-pp-cli upload-video --index-id IDX --file ./long-video.mp4 --wait --json
twelvelabs-pp-cli video-brief --video-id VIDEO_ID --format json --out edit-plan.json
twelvelabs-pp-cli video-brief --video-id VIDEO_ID --format markdown --out edit-plan.md
```

Use `clips` to cut local files from `recommended_cuts` when you have the source video:

```bash
twelvelabs-pp-cli clips --input ./long-video.mp4 --plan edit-plan.json --out ./clips
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Editor workflows
- **`upload-video`** — Upload a local video file or register a public URL, then poll Twelve Labs until indexing reaches a terminal status.

  _Use this before analysis when you need the CLI to wait until the video is usable._

  ```bash
  twelvelabs-pp-cli upload-video --index-id IDX --file ./long-video.mp4 --wait --json --dry-run
  ```
- **`video-brief`** — Generate a deterministic JSON or Markdown editing plan with title, topics, hashtags, chapters, highlights, and recommended cuts.

  _Use this when you want to avoid manually watching a long video just to find the strongest moments._

  ```bash
  twelvelabs-pp-cli video-brief --video-id VID --format json --json --dry-run
  ```
- **`embed`** — Create a video embedding task from a local file or public URL and optionally wait for ready results.

  _Use this when you need embeddings for deeper search, matching, or semantic workflows._

  ```bash
  twelvelabs-pp-cli embed --video-file ./long-video.mp4 --model marengo3.0 --wait --json --dry-run
  ```
- **`clips`** — Cut local clip files with ffmpeg from a video-brief JSON plan without inventing timestamps.

  _Use this after video-brief when you have the source video locally and want clip files immediately._

  ```bash
  twelvelabs-pp-cli clips --dry-run --json
  ```

## Usage

Run `twelvelabs-pp-cli --help` for the full command reference and flag list.

## Commands

### analyze

Manage analyze

- **`twelvelabs-pp-cli analyze`** - This endpoint analyzes your videos and creates fully customizable text based on your prompts, including but not limited to tables of content, action items, memos, and detailed analyses.

<Note title="Notes">
- This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
- This endpoint supports streaming responses. For details on integrating this feature into your application, refer to the [Analyze videos](/v1.3/docs/guides/analyze-videos) page.
</Note>

### editor workflows

Higher-level commands for turning long videos into editing plans:

- **`twelvelabs-pp-cli upload-video`** - Upload a local file or register a public URL with `POST /tasks`, then optionally wait until indexing reaches a terminal status.
- **`twelvelabs-pp-cli video-brief`** - Generate a deterministic JSON or Markdown plan with title, topics, hashtags, chapters, highlights, and recommended cuts. Uses `/analyze` when deprecated summary/gist endpoints are unavailable.
- **`twelvelabs-pp-cli embed`** - Create a video embedding task from a local file or URL and optionally wait for ready results.
- **`twelvelabs-pp-cli clips`** - Use local `ffmpeg` to cut files from a `video-brief` JSON plan without inventing timestamps.

### assets

Manage assets

- **`twelvelabs-pp-cli assets create`** - This method creates an asset by uploading a file to the platform. Assets are media files that you can use in downstream workflows, including indexing, analyzing video content, and creating entities.

**Supported content**: Video, audio, and images.

**Upload methods**:
- **Local file**: Set the `method` parameter to `direct` and use the `file` parameter to specify the file.
- **Publicly accessible URL**: Set the `method` parameter to `url` and use the `url` parameter to specify the URL of your file.

**File size**: 200MB maximum for local file uploads, 4GB maximum for URL uploads.

**Additional requirements** depend on your workflow:
- **Search**: [Marengo requirements](/v1.3/docs/concepts/models/marengo#video-file-requirements)
- **Video analysis**: [Pegasus requirements](/v1.3/docs/concepts/models/pegasus#input-requirements)
- **Entity search**: [Marengo image requirements](/v1.3/docs/concepts/models/marengo#image-file-requirements)
- **Create embeddings**: [Marengo requirements](/v1.3/docs/concepts/models/marengo#input-requirements)

<Note title="Note">
This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>
- **`twelvelabs-pp-cli assets create-multipart-upload`** - This method creates a multipart upload session.

**Supported content**: Video and audio

**File size**: 4GB maximum.

**Additional requirements** depend on your workflow:
- **Search**: [Marengo requirements](/v1.3/docs/concepts/models/marengo#video-file-requirements)
- **Video analysis**: [Pegasus requirements](/v1.3/docs/concepts/models/pegasus#input-requirements)
- **Create embeddings**: [Marengo requirements](/v1.3/docs/concepts/models/marengo#input-requirements)
- **`twelvelabs-pp-cli assets delete`** - This method deletes the specified asset. This action cannot be undone.
- **`twelvelabs-pp-cli assets get-upload-status`** - This method provides information about an upload session, including its current status, chunk-level progress, and completion state.

Use this method to:
- Verify upload completion (`status` = `completed`)
- Identify any failed chunks that require a retry
- Monitor the upload progress by comparing `uploaded_size` with `total_size`
- Determine if the session has expired
- Retrieve the status information for each chunk

You must call this method after reporting chunk completion to confirm the upload has transitioned to the `completed` status before using the asset.
- **`twelvelabs-pp-cli assets list`** - This method returns a list of assets in your account.

The platform returns your assets sorted by creation date, with the newest at the top of the list.
- **`twelvelabs-pp-cli assets list-incomplete-uploads`** - This method returns a list of all incomplete multipart upload sessions in your account.
- **`twelvelabs-pp-cli assets report-chunk-batch`** - This method reports successfully uploaded chunks to the platform. The platform finalizes the upload after you report all chunks.


For optimal performance, report chunks in batches and in any order.
- **`twelvelabs-pp-cli assets request-additional-presigned-urls`** - This method generates new presigned URLs for specific chunks that require uploading. Use this endpoint in the following situations:
- Your initial URLs have expired (URLs expire after one hour).
- The initial set of presigned URLs does not include URLs for all chunks.
- You need to retry failed chunk uploads with new URLs.
To specify which chunks need URLs, use the `start` and `count` parameters. For example, to generate URLs for chunks 21 to 30, use `start=21` and `count=10`.
The response will provide new URLs, each with a fresh expiration time of one hour.
- **`twelvelabs-pp-cli assets retrieve`** - This method retrieves details about the specified asset.

### embed

Manage embed

- **`twelvelabs-pp-cli embed create-text-image-audio-embedding`** - <Note title="Note">
  This endpoint will be deprecated in a future version. Migrate to the [Embed API v2](/v1.3/api-reference/create-embeddings-v2) for continued support and access to new features.
</Note>

This method creates embeddings for text, image, and audio content.

Ensure your media files meet the following requirements:
- [Audio files](/v1.3/docs/concepts/models/marengo#audio-requirements).
- [Image files](/v1.3/docs/concepts/models/marengo#image-requirements).

Parameters for embeddings:
- **Common parameters**:
  - `model_name`: The video understanding model you want to use. Example: "marengo3.0".
- **Text embeddings**:
  - `text`: Text for which to create an embedding.
- **Image embeddings**:
  Provide one of the following:
  - `image_url`: Publicly accessible URL of your image file.
  - `image_file`:  Local image file.
- **Audio embeddings**:
  Provide one of the following:
  - `audio_url`: Publicly accessible URL of your audio file.
  - `audio_file`: Local audio file.

<Note title="Notes">
- The Marengo video understanding model generates embeddings for all modalities in the same latent space. This shared space enables any-to-any searches across different types of content.
- You can create multiple types of embeddings in a single API call.
- Audio embeddings combine generic sound and human speech in a single embedding. For videos with transcriptions, you can retrieve transcriptions and then [create text embeddings](/v1.3/api-reference/create-embeddings-v1/text-image-audio-embeddings/create-text-image-audio-embeddings) from these
- This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>
- **`twelvelabs-pp-cli embed create-video-embedding-task`** - <Note title="Note">
  This endpoint will be deprecated in a future version. Migrate to the [Embed API v2](/v1.3/api-reference/create-embeddings-v2) for continued support and access to new features.
</Note>

This method creates a new video embedding task that uploads a video to the platform and creates one or multiple video embeddings.

<Note title="Note">
This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>

Upload options:
- **Local file**: Use the `video_file` parameter
- **Publicly accessible URL**: Use the `video_url` parameter.

Specify at least one option. If both are provided, `video_url` takes precedence.

Your video files must meet the [requirements](/v1.3/docs/concepts/models/marengo#video-file-requirements).
This endpoint allows you to upload files up to 2 GB in size.  To upload larger files, use the [Multipart Upload API](/v1.3/api-reference/upload-content/multipart-uploads)

<Note title="Notes">
- The Marengo video understanding model generates embeddings for all modalities in the same latent space. This shared space enables any-to-any searches across different types of content.
- Video embeddings are stored for seven days.
</Note>
- **`twelvelabs-pp-cli embed list-video-embedding-tasks`** - <Note title="Note">
  This method will be deprecated in a future version. Migrate to the [Embed API v2](/v1.3/api-reference/create-embeddings-v2) for continued support and access to new features.
</Note>
This method returns a list of the video embedding tasks in your account. The platform returns your video embedding tasks sorted by creation date, with the newest at the top of the list.

<Note title="Notes">
- Video embeddings are stored for seven days
- When you invoke this method without specifying the `started_at` and `ended_at` parameters, the platform returns all the video embedding tasks created within the last seven days.
</Note>
- **`twelvelabs-pp-cli embed retrieve-video-embedding`** - This method retrieves embeddings for a specific video embedding task. Ensure the task status is `ready` before invoking this method. Refer to the [Retrieve the status of a video embedding tasks](/v1.3/api-reference/create-embeddings-v1/video-embeddings/retrieve-video-embedding-task-status) page for instructions on checking the task status.
- **`twelvelabs-pp-cli embed retrieve-video-embedding-task`** - <Note title="Note">
  This endpoint will be deprecated in a future version. Migrate to the [Embed API v2](/v1.3/api-reference/create-embeddings-v2) for continued support and access to new features.
</Note>
This method retrieves the status of a video embedding task. Check the task status of a video embedding task to determine when you can retrieve the embedding.

A task can have one of the following statuses:
- `processing`: The platform is creating the embeddings.
- `ready`:  Processing is complete. Retrieve the embeddings by invoking the [`GET`](/v1.3/api-reference/create-embeddings-v1/video-embeddings/retrieve-video-embeddings) method of the `/embed/tasks/{task_id} endpoint`.
- `failed`: The task could not be completed, and the embeddings haven't been created.

### embed-v2

Manage embed v2

- **`twelvelabs-pp-cli embed-v2 create-async-embedding-task`** - This endpoint creates embeddings for audio and video content asynchronously.

<Note title="Note">
  This method only supports Marengo version 3.0 or newer.
</Note>

**When to use this endpoint**:
- Process audio or video files longer than 10 minutes
- Process files up to 4 hours in duration

<Accordion title="Input requirements">
  **Video**:
  - Minimum duration: 4 seconds
  - Maximum duration: 4 hours
  - Maximum file size: 4 GB
  - Formats: [FFmpeg supported formats](https://ffmpeg.org/ffmpeg-formats.html)
  - Resolution: 360x360 to 5184x2160 pixels
  - Aspect ratio: Between 1:1 and 1:2.4, or between 2.4:1 and 1:1

  **Audio**:
  - Minimum duration: 4 seconds
  - Maximum duration: 4 hours
  - Maximum file size: 2 GB
  - Formats: WAV (uncompressed), MP3 (lossy), FLAC (lossless)
</Accordion>

  Creating embeddings asynchronously requires three steps:

  1. Create a task using this endpoint. The platform returns a task ID.
  2. Poll for the status of the task using the [`GET`](/v1.3/api-reference/create-embeddings-v2/retrieve-embeddings) method of the `/embed-v2/tasks/{task_id}` endpoint. Wait until the status is `ready`.
  3. Retrieve the embeddings from the response when the status is `ready` using the [`GET`](/v1.3/api-reference/create-embeddings-v2/retrieve-embeddings) method of the `/embed-v2/tasks/{task_id}` endpoint.

  <Note title="Note">
  This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
  </Note>
- **`twelvelabs-pp-cli embed-v2 create-embeddings`** - This endpoint synchronously creates embeddings for multimodal content and returns the results immediately in the response.

<Note title="Note">
  This method only supports Marengo version 3.0 or newer.
</Note>

**When to use this endpoint**:
- Create embeddings for text, images, audio, or video content
- Get immediate results without waiting for background processing
- Process audio or video content up to 10 minutes in duration

**Do not use this endpoint for**:
- Audio or video content longer than 10 minutes. Use the [`POST`](/v1.3/api-reference/create-embeddings-v2/create-async-embedding-task) method of the `/embed-v2/tasks` endpoint instead.

<Accordion title="Input requirements">
  **Text**:
  - Maximum length: 500 tokens

  **Images**:
  - Formats: JPEG, PNG
  - Minimum size: 128x128 pixels
  - Maximum file size: 5 MB

  **Audio and video**:
  - Maximum duration: 10 minutes
  - Maximum file size for base64 encoded strings: 36 MB
  - Audio formats: WAV (uncompressed), MP3 (lossy), FLAC (lossless)
  - Video formats: [FFmpeg supported formats](https://ffmpeg.org/ffmpeg-formats.html)
  - Video resolution: 360x360 to 5184x2160 pixels
  - Aspect ratio: Between 1:1 and 1:2.4, or between 2.4:1 and 1:1
</Accordion>

<Note title="Note">
This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>
- **`twelvelabs-pp-cli embed-v2 list-async-embedding-tasks`** - This method returns a list of the async embedding tasks in your account. The platform returns your async embedding tasks sorted by creation date, with the newest at the top of the list.
- **`twelvelabs-pp-cli embed-v2 retrieve-embeddings`** - This method retrieves the status and the results of an async embedding task.

**Task statuses**:
- `processing`: The platform is creating the embeddings.
- `ready`: Processing is complete. Embeddings are available in the response.
- `failed`: The task failed. Embeddings were not created.

Invoke this method repeatedly until the `status` field is `ready`. When `status` is `ready`, use the embeddings from the response.

### entity-collections

Manage entity collections

- **`twelvelabs-pp-cli entity-collections create`** - This method creates an entity collection.
- **`twelvelabs-pp-cli entity-collections delete`** - This method deletes the specified entity collection. This action cannot be undone.
- **`twelvelabs-pp-cli entity-collections list`** - This method returns a list of the entity collections in your account.
- **`twelvelabs-pp-cli entity-collections retrieve`** - This method retrieves details about the specified entity collection.
- **`twelvelabs-pp-cli entity-collections update`** - This method updates the specified entity collection.

### gist

Manage gist

- **`twelvelabs-pp-cli gist`** - <Note title="Deprecation notice">
  This endpoint will be sunset and removed on February 15, 2026. Instead, use the [`POST`](/v1.3/api-reference/analyze-videos/analyze) method of the `/analyze` endpoint, passing the [`response_format`](/v1.3/api-reference/analyze-videos/analyze#request.body.response_format) parameter to specify the format of the response as structured JSON. For migration instructions, see the [Release notes](/v1.3/docs/get-started/release-notes#predefined-formats-for-video-analysis-will-be-sunset-and-removed) page.
</Note>

This method analyzes videos and generates titles, topics, and hashtags.

### indexes

Manage indexes

- **`twelvelabs-pp-cli indexes create-index`** - This method creates an index.
- **`twelvelabs-pp-cli indexes delete-index`** - This method deletes the specified index and all the videos within it. This action cannot be undone.
- **`twelvelabs-pp-cli indexes list`** - This method returns a list of the indexes in your account. The platform returns indexes sorted by creation date, with the oldest indexes at the top of the list.
- **`twelvelabs-pp-cli indexes retrieve-index`** - This method retrieves details about the specified index.
- **`twelvelabs-pp-cli indexes update-index`** - This method updates the name of the specified index.

### summarize

Manage summarize

- **`twelvelabs-pp-cli summarize`** - <Note title="Deprecation notice">
  This endpoint will be sunset and removed. Use the [`POST`](/v1.3/api-reference/analyze-videos/analyze) method of the `/analyze` endpoint. Pass the [`response_format`](/v1.3/api-reference/analyze-videos/analyze#request.body.response_format) parameter to specify the format of the response as structured JSON. For migration instructions, see the [Release notes](/v1.3/docs/get-started/release-notes#predefined-formats-for-video-analysis-will-be-sunset-and-removed) page.
</Note>

This endpoint analyzes videos and generates summaries, chapters, or highlights. Optionally, you can provide a prompt to customize the output.

<Note title="Note">
This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>

### tasks

Manage tasks

- **`twelvelabs-pp-cli tasks create-video-indexing`** - This method creates a video indexing task that uploads and indexes a video in a single operation.

<Warning title="Legacy endpoint">
This endpoint bundles two operations (upload and indexing) together. In the next major API release, this endpoint will be removed in favor of a separated workflow:
1. Upload your video using the [`POST /assets`](/v1.3/api-reference/upload-content/direct-uploads/create) endpoint
2. Index the uploaded video using the [`POST /indexes/{index-id}/indexed-assets`](/v1.3/api-reference/index-content/create) endpoint

This separation provides better control, reusability of assets, and improved error handling. New implementations should use the new workflow.
</Warning>


Upload options:
- **Local file**: Use the `video_file` parameter.
- **Publicly accessible URL**: Use the `video_url` parameter.

Your video files must meet requirements based on your workflow:
- **Search**: [Marengo requirements](/v1.3/docs/concepts/models/marengo#video-file-requirements).
- **Video analysis**: [Pegasus requirements](/v1.3/docs/concepts/models/pegasus#video-file-requirements).
- If you want to both search and analyze your videos, the most restrictive requirements apply.
- This method allows you to upload files up to 2 GB in size. To upload larger files, use the [Multipart Upload API](/v1.3/api-reference/upload-content/multipart-uploads)

<Note title="Note">
This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>
- **`twelvelabs-pp-cli tasks delete-video-indexing`** - This action cannot be undone.
Note the following about deleting a video indexing task:
- You can only delete video indexing tasks for which the status is `ready` or `failed`.
- If the status of your video indexing task is `ready`, you must first delete the video vector associated with your video indexing task by calling the [`DELETE`](/v1.3/api-reference/videos/delete) method of the `/indexes/videos` endpoint.
- **`twelvelabs-pp-cli tasks list-video-indexing`** - This method returns a list of the video indexing tasks in your account. The platform returns your video indexing tasks sorted by creation date, with the newest at the top of the list.
- **`twelvelabs-pp-cli tasks retrieve-video-indexing`** - This method retrieves a video indexing task.

### video_search

Manage video search

- **`twelvelabs-pp-cli video-search any-to`** - Use this endpoint to search for relevant matches in an index using text, media, or a combination of both as your query.

**Text queries**:
- Use the `query_text` parameter to specify your query.

**Media queries**:
- Set the `query_media_type` parameter to the corresponding media type (example: `image`).
- Specify either one of the following parameters:
  - `query_media_url`: Publicly accessible URL of your media file.
  - `query_media_file`: Local media file.
  If both `query_media_url` and `query_media_file` are specified in the same request, `query_media_url` takes precedence.

**Composed text and media queries** (Marengo 3.0 only):
- Use the `query_text` parameter for your text query.
- Set `query_media_type` to `image`.
- Specify the image using either the `query_media_url` or the `query_media_file` parameter.

  Example: Provide an image of a car and include  "red color"  in your query to find red instances of that car model.

**Entity search** (Marengo 3.0 only and in beta):

- To find a specific person in your videos, enclose the unique identifier of the entity you want to find in the `query_text` parameter.

<Note title="Notes">
- When using images in your search queries (either as media queries or in composed searches), ensure your image files meet the [requirements](/v1.3/docs/concepts/models/marengo#image-file-requirements).
- This endpoint is rate-limited. For details, see the [Rate limits](/v1.3/docs/get-started/rate-limits) page.
</Note>
- **`twelvelabs-pp-cli video-search any-to-video-retrieve-specific-page`** - Use this endpoint to retrieve a specific page of search results.

<Note title="Note">
When you use pagination, you will not be charged for retrieving subsequent pages of results.
</Note>


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
twelvelabs-pp-cli assets list

# JSON for scripting and agents
twelvelabs-pp-cli assets list --json

# Filter to specific fields
twelvelabs-pp-cli assets list --json --select id,name,status

# Dry run — show the request without sending
twelvelabs-pp-cli assets list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
twelvelabs-pp-cli assets list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
twelvelabs-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/twelvelabs-video-understanding-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TWELVELABS_X_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `twelvelabs-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `twelvelabs-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TWELVELABS_X_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
