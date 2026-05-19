# Magnific CLI

**Every Magnific feature, plus a local archive of every prompt, task, and credit — so your generation history finally becomes queryable.**

Magnific exposes 80+ AI generation and editing models across image, video, audio, and stock content. This CLI mirrors all of them with the same `x-freepik-api-key` you already have, then adds what the platform cannot: SQLite-backed prompt history, model bake-off (`compare`), unified task polling (`task wait`), a credit-cost ledger, and a local asset gallery. Built as a single Go binary with offline search, agent-native JSON output, and an MCP code-orchestration surface so agents see search+execute instead of 388 raw tools.

Printed by [@nunomcduarte](https://github.com/nunomcduarte) (Nuno Duarte).

## Install

The recommended path installs both the `magnific-pp-cli` binary and the `pp-magnific` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install magnific
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install magnific --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install magnific --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install magnific --agent claude-code
npx -y @mvanhorn/printing-press install magnific --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/magnific-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-magnific --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-magnific --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-magnific skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-magnific. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/magnific-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MAGNIFIC_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "magnific": {
      "command": "magnific-pp-mcp",
      "env": {
        "MAGNIFIC_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Magnific (formerly Freepik) authenticates with a single `x-freepik-api-key` header. The same key works on both `api.freepik.com` and `api.magnific.com`. Set `FREEPIK_API_KEY` (preferred) or the rebrand alias `MAGNIFIC_API_KEY` in your environment, or run `magnific-pp-cli auth set-token <key>` to write it to your config. Get a key at https://www.magnific.com/api.

## Quick Start

```bash
# Store your Magnific/Freepik key once; subsequent commands pick it up.
magnific-pp-cli auth set-token sk_live_...


# Verify auth + connectivity. Magnific has no /v1/me endpoint so a doctor probe is the canonical auth check.
magnific-pp-cli doctor --json


# Print the agent-native orientation bundle: top models you've used, recent prompts, recent assets, task counts, API reachability.
magnific-pp-cli context --json


# Run a real bake-off across three flagship image models with cost+latency in the manifest.
magnific-pp-cli compare "a cinematic Tokyo street at golden hour" --models mystic,flux-2-pro,seedream-v4-5 --aspect 16:9 --json


# Recover the exact prompt that produced last month's frame.
magnific-pp-cli history search "tokyo" --since 30d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local archive that compounds
- **`history search`** — Full-text search every prompt you have ever sent to Magnific, filtered by model, date, or cost — your generation archive becomes queryable instead of lost in a tab.

  _When the user says 'find the prompt I used for the ACME hero shot' the agent gets a structured answer instead of guessing._

  ```bash
  magnific-pp-cli history search "neon city skyline" --since 90d --json
  ```
- **`gallery list`** — Filter every output you have ever downloaded by tag, date, orientation, or model. Companion `gallery open <id>` reveals the file (opt-in side effect under --launch).

  _Outputs become an addressable archive instead of UUID-named blobs in ~/Downloads._

  ```bash
  magnific-pp-cli gallery list --tag client-acme --since 7d --orientation landscape --json
  ```
- **`prompt save`** — Save reusable prompt templates with {{placeholder}} syntax; replay them with --override key=value substitutions through the real Magnific endpoint.

  _Stable prompt surfaces for agents that re-run brand-approved shots across hundreds of variants._

  ```bash
  magnific-pp-cli prompt run hero-shot --override city=tokyo --override mood=neon --json
  ```
- **`stock library index`** — Walk your downloaded Freepik icons/videos/resources into a local FTS5 index. `stock library search` queries that index first before hitting the API.

  _Lets the agent answer 'do we already have this asset?' without an API round-trip._

  ```bash
  magnific-pp-cli stock library search "rocket icon" --json
  ```

### Cross-model leverage
- **`compare`** — Fire the same prompt at N image or video models in parallel, then return a manifest of cost, latency, and output URL per model so you can pick the right tool empirically.

  _Answers the recurring question 'is the new model worth the credit cost?' with real outputs and timing, not vendor marketing._

  ```bash
  magnific-pp-cli compare "a futuristic Tokyo street at golden hour" --models mystic,flux-2-pro,seedream-v4-5,z-image-turbo --aspect 16:9 --json
  ```
- **`models list`** — Curated catalog of every Magnific model (family, listed credit cost, capabilities) joined with your local task aggregation (your p50 latency, success rate, $ spent). Sibling `models stats <model>` deep-dives one.

  _Agents pick the cheapest fit-for-purpose model instead of cargo-culting Mystic for everything._

  ```bash
  magnific-pp-cli models list --capability text-to-image --sort cost --json
  ```

### Async lifecycle, unified
- **`task wait`** — Resolve any Magnific task_id back to its model endpoint and block (or stream) until terminal state — one command across all 80+ async model families.

  _Lets an agent submit work then block on completion without knowing which of 80+ model families produced the task._

  ```bash
  magnific-pp-cli task wait task-7f3a-... --timeout 5m --json
  ```
- **`tasks stale`** — Find tasks the local store thinks are still pending past a threshold; sibling `tasks reconcile` re-polls each via the real GET and updates terminal state.

  _Keeps the local ledger honest so cost queries and history search are not lying about pending work._

  ```bash
  magnific-pp-cli tasks stale --since 24h --json
  ```

### Credits and accountability
- **`cost ledger`** — Local SQL aggregation over every task you have run: spend by model, by day, by tag, with a sibling `cost forecast` that estimates a planned batch against the curated per-model credit cost table.

  _Turns 'where did my credits go this month?' from a spreadsheet exercise into one SQL-backed command._

  ```bash
  magnific-pp-cli cost ledger --since 30d --group-by model --json
  ```

### Agent-native plumbing
- **`context`** — Single command emitting JSON with: top 10 most-used models, last 10 prompts, recent output directories, and the live API reachability status. Read-only and MCP-safe.

  _An agent reads this once at session start and stops asking obvious questions about which model to pick or what's in flight._

  ```bash
  magnific-pp-cli context --json
  ```

## Usage

Run `magnific-pp-cli --help` for the full command reference and flag list.

## Commands

### ai

Manage ai

- **`magnific-pp-cli ai apply-style-transfer-to-image`** - Style Transfer - Transform image style
- **`magnific-pp-cli ai create`** - This endpoint allows you to generate or edit images using the Gemini 2.5 Flash model. You can provide a prompt for image generation, or include reference images for image editing and style transfer. The model supports both text-to-image generation and image-to-image editing with up to 3 reference images.
- **`magnific-pp-cli ai create-audio-isolation-task`** - Isolate and extract specific sounds from audio or video files using SAM Audio AI technology.
Describe the sound you want to isolate, and the API separates it from background noise.

**Use cases:**
- Extract speech from noisy recordings
- Isolate musical instruments from a mix
- Separate specific sound effects from video audio
- Remove background noise while preserving target sounds

**Input options:**
- Provide either an `audio` URL/base64 or a `video` URL/base64 (mutually exclusive)
- Supported audio formats: WAV, MP3, FLAC, OGG, M4A
- Supported video formats: MP4, MOV, WEBM, AVI
- For video input, use bounding box coordinates (x1, y1, x2, y2) to focus on a specific region

**Output:** WAV audio file containing the isolated sound
- **`magnific-pp-cli ai create-beta`** - (Beta, synchronous) Reimagine Flux is a new AI model that allows you to generate images from text prompts.
- **`magnific-pp-cli ai create-change-camera-task`** - Transform an image by changing the camera angle using AI. Adjust horizontal rotation (0-360 degrees), vertical tilt (-30 to 90 degrees), and zoom level (0-10) to generate a new image as if the camera had been repositioned around the subject.

This is an asynchronous endpoint. After submitting a request, use the task ID to poll for results or provide a `webhook_url` to receive a notification when processing completes.

**Camera controls:**
- **Horizontal angle** (`horizontal_angle`): Rotate the viewpoint 0-360 degrees around the subject. `0` = front view, `90` = right side, `180` = back view, `270` = left side.
- **Vertical angle** (`vertical_angle`): Tilt the camera from -30 (looking up) to 90 (bird's eye view). `0` = eye level.
- **Zoom** (`zoom`): Adjust from `0` (wide shot, full scene) to `10` (close-up).

**Use cases:** Product photography with multiple angle views, architectural visualization, creative image manipulation, and generating consistent multi-angle views of objects and scenes.
- **`magnific-pp-cli ai create-ideogram-image-edit`** - Edit an image using Ideogram AI's inpainting capabilities. Provide an image and a mask to specify the areas to edit, along with a prompt describing the desired changes.

**Key features:**
- Inpainting: Edit specific areas of an image using a mask
- Multiple rendering speeds: TURBO, DEFAULT, or QUALITY
- MagicPrompt: Automatically enhance your prompt for better results
- Style customization: Use style codes, style types, and reference images
- Character reference: Use reference images to maintain character consistency

**Supported formats:** JPEG, WebP, PNG (max 10MB each)
- **`magnific-pp-cli ai create-image-edit-seedream-v4-5`** - Edit images using ByteDance's Seedream 4.5 model with text guidance.

**Key Features:**
- Preserves subject details, lighting, and color tone
- Supports up to 5 reference images
- Enhanced editing consistency
- Up to 4MP output resolution

**Best for:**
- Image-to-image editing
- Style transfer with consistency
- Multi-image reference editing
- **`magnific-pp-cli ai create-image-edit-seedream-v5-lite`** - Edit images using ByteDance's Seedream V5 Lite model with text guidance.

**Key Features:**
- Preserves subject details, lighting, and color tone
- Supports up to 5 reference images
- Enhanced editing consistency
- Up to 4MP output resolution

**Best for:**
- Image-to-image editing
- Style transfer with consistency
- Multi-image reference editing
- **`magnific-pp-cli ai create-image-flux-2-klein`** - Generate images with sub-second speed using FLUX.2 [klein], the fastest model in the FLUX.2 family by Black Forest Labs.

**Key Features:**
- Sub-second generation time
- Up to 4 reference images for style/subject transfer
- 10 preset aspect ratios with 1k or 2k resolution options
- Adjustable safety tolerance (0-5)
- Multiple output formats (PNG/JPEG)

**Use Cases:**
- Real-time applications requiring fast generation
- Style transfer with reference images
- Rapid prototyping and iteration
- High-volume image generation
- **`magnific-pp-cli ai create-image-flux-2-pro`** - Create professional-grade images using FLUX.2 [pro], the next generation of Black Forest Labs' image models.

**Key Features:**
- Professional quality without complex tuning
- Text-to-image generation
- Image-to-image editing (up to 4 input images)
- Customizable dimensions (256-1440px)
- Optional prompt enhancement
- Reproducible results with seed

**Use Cases:**
- Marketing materials and advertisements
- Product photography variations
- Concept art and illustrations
- Image editing and enhancement
- **`magnific-pp-cli ai create-image-flux-2-turbo`** - Create high-quality images quickly using FLUX.2 [turbo], the speed-optimized version of Flux 2.

**Key Features:**
- Fast generation (optimized for speed)
- Lower cost than Pro version
- Adjustable guidance scale for prompt adherence
- Custom image dimensions (512-2048px)
- Safety checker for content filtering
- Multiple output formats (PNG/JPEG)

**Use Cases:**
- Rapid prototyping and iteration
- Content exploration
- High-volume generation
- Testing prompts and concepts
- **`magnific-pp-cli ai create-image-from-text-classic`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-flux`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-flux-kontext-pro`** - Generate images using FLUX Kontext Pro, an advanced text-to-image model with optional image input support.

This model excels at understanding context and generating high-quality images from text descriptions.
Optionally, you can provide an input image to guide the generation process.
- **`magnific-pp-cli ai create-image-from-text-flux-pro-v1-1`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-imagen3`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-imagen4-fast`** - Convert descriptive text input into images using AI with Imagen 4 Fast model. Optimized for speed and cost-effectiveness.
- **`magnific-pp-cli ai create-image-from-text-imagen4-ultra`** - Convert descriptive text input into images using AI with Imagen 4 Ultra model. Highest quality for professional needs.
- **`magnific-pp-cli ai create-image-from-text-nano-banana-pro`** - Generate high-quality images from text descriptions using Google's Nano Banana Pro model (Gemini 3).

**Key Features:**
- Advanced image generation with complex compositions
- Support for reference images for guided generation
- Multiple aspect ratios and resolutions
- High-quality output up to 4K resolution

**Best for:**
- High-quality image generation
- Complex scene compositions
- Reference-guided generation
- Professional visual content
- **`magnific-pp-cli ai create-image-from-text-nano-banana-pro-flash`** - Generate images from text descriptions using Google's Nano Banana Pro Flash model (Gemini 3.1 Flash), a faster variant of Nano Banana Pro optimized for quick image generation.

**Key Features:**
- Fast image generation with Gemini 3.1 Flash
- Google Search grounding for real-world accuracy
- Support for reference images for guided generation
- Multiple aspect ratios and resolutions up to 4K

**Best for:**
- Rapid image generation with shorter wait times
- Grounded image generation using Google Search
- Reference-guided generation
- Iterative creative workflows where speed matters
- **`magnific-pp-cli ai create-image-from-text-runway`** - Generate high-quality images from text descriptions using RunWay's Gen4 Image model.

**Key Features:**
- Photorealistic and artistic image generation
- Multiple aspect ratios supported
- Reference image support with @tag syntax
- High-resolution output

**Best for:**
- Photorealistic images
- Artistic and creative visuals
- Marketing and promotional content
- **`magnific-pp-cli ai create-image-from-text-seedream`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-seedream-v4`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-seedream-v4-5`** - Generate high-quality images from text descriptions using ByteDance's Seedream 4.5 model.

**Key Features:**
- Superior typography and text rendering
- Excellent poster composition and branded visuals
- Up to 4MP resolution support (2048x2048)
- Enhanced editing consistency

**Best for:**
- Marketing materials with text
- Professional posters and banners
- Branded visual content
- High-resolution image generation
- **`magnific-pp-cli ai create-image-from-text-seedream-v4-edit`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-from-text-seedream-v5-lite`** - Generate high-quality images from text descriptions using ByteDance's Seedream V5 Lite model.

**Key Features:**
- Improved detail and composition over previous versions
- Up to 4MP resolution support
- Various aspect ratios available
- Optional seed for reproducibility

**Best for:**
- High-quality image generation
- Detailed scenes and compositions
- Cinematic and artistic imagery
- **`magnific-pp-cli ai create-image-from-text-z-image`** - Generate high-quality images from text descriptions using the Z-Image turbo model.

**Key Features:**
- Superior speed with turbo architecture
- High-quality image generation
- Flexible image size configuration
- Supports LoRA and ControlNet variants

**Best for:**
- Fast prototyping and iteration
- High-volume image generation
- Cost-effective production workloads
- **`magnific-pp-cli ai create-image-mystic`** - Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-image-to-prompt-task`** - Generate descriptive prompts from input images using AI analysis
- **`magnific-pp-cli ai create-imageexpand`** - This endpoint allows you to expand an image using the AI Flux Pro model. The image will be expanded based on the provided parameters.
- **`magnific-pp-cli ai create-imageexpand-2`** - This endpoint allows you to expand an image using the Ideogram AI model. The image will be expanded based on the provided pixel values for each side.
If no prompt is provided, the model will auto-generate one based on the image content.
- **`magnific-pp-cli ai create-imageexpand-3`** - This endpoint allows you to expand an image using the Seedream V4.5 AI model. The image will be expanded based on the provided pixel values for each side.
If no prompt is provided, the model will auto-generate one based on the image content.
- **`magnific-pp-cli ai create-imagerelight`** - Relight an image using AI. This endpoint accepts a variety of parameters to customize the generated images.
- **`magnific-pp-cli ai create-imagetovideo`** - Create a video from an image using the Kling v2 model
- **`magnific-pp-cli ai create-imageupscaler`** - This asynchronous endpoint enables image upscaling using advanced AI algorithms. Upon submission, it returns a unique `task_id` which can be used to track the progress of the upscaling process. For real-time production use, include the optional `webhook_url` parameter to receive an automated notification once the task has been completed. This allows for seamless integration and efficient task management without the need for continuous polling.
- **`magnific-pp-cli ai create-improve-prompt-task`** - Enhance user prompts for AI image or video generation using advanced AI models.
- **Image prompts**: Improve a prompt for image generation
- **Video prompts**: Improve a prompt for video generation
- **`magnific-pp-cli ai create-latent-sync-lip-sync`** - Generate lip-synced video by synchronizing a video with an audio file using AI Latent-Sync technology
- **`magnific-pp-cli ai create-loras`** - Create you own custom character using LoRAs training

For now you can check the status of the training calling `v1/ai/loras`. We are working on it
- **`magnific-pp-cli ai create-loras-2`** - Create you own custom style using LoRAs training

For now you can check the status of the training calling `v1/ai/loras`. We are working on it
- **`magnific-pp-cli ai create-music-generation-task`** - Generate original music tracks from text descriptions using AI.

Create high-quality music compositions based on your text prompts. Specify genre, mood, instruments, and tempo to get exactly the sound you need. Perfect for video production, game development, podcasts, and multimedia projects.

**Tips for effective prompts:**
- Include genre: "jazz", "electronic", "classical", "rock"
- Describe mood: "upbeat", "melancholic", "energetic", "peaceful"
- Mention instruments: "piano", "guitar", "drums", "synthesizer"
- Add tempo hints: "slow", "fast-paced", "moderate groove"
- **`magnific-pp-cli ai create-pixverse-v5-5-video`** - Generate a video from an image using the PixVerse-V5.5 model.

Compared to PixVerse-V5, version 5.5 adds:
- 10-second duration (in addition to 5s and 8s; 10s capped at 720p)
- Native audio generation via `generate_audio_switch`
- Multi-clip output with dynamic camera changes via `generate_multi_clip_switch`
- Prompt reasoning enhancement via `thinking_type`
- **`magnific-pp-cli ai create-pixverse-v5-transition-video`** - Generate a video transition between two images using the PixVerse-V5 model.
- **`magnific-pp-cli ai create-pixverse-v5-video`** - Generate a video using the PixVerse-V5 model. Resolution is specified in the request body.
- **`magnific-pp-cli ai create-pixverse-v6-video`** - Generate a video from an image using the PixVerse-V6 model.

Compared to PixVerse-V5.5, version 6 adds:
- Flexible duration: any integer from `1` to `15` seconds (previously fixed to `5`, `8` or `10`).
- Optional `last_frame_image`: when provided, PixVerse-V6 generates a transition between `image_url` (first frame) and `last_frame_image` (last frame) in the same endpoint — no separate transition endpoint required.

Existing PixVerse-V5.5 features are kept: native audio (`generate_audio_switch`), multi-clip output (`generate_multi_clip_switch`) and prompt reasoning enhancement (`thinking_type`).
- **`magnific-pp-cli ai create-skinenhancer`** - Enhance skin in images using AI with the Creative mode. This mode provides more artistic and stylized enhancements.
- **`magnific-pp-cli ai create-skinenhancer-2`** - Enhance skin in images using AI with the Faithful mode. This mode preserves the original appearance while improving skin quality.
- **`magnific-pp-cli ai create-skinenhancer-3`** - Enhance skin in images using AI with the Flexible mode. This mode allows you to choose the optimization target for the enhancement.
- **`magnific-pp-cli ai create-sound-effects-task`** - Create realistic sound effects from text descriptions using AI.

Generate high-quality audio sound effects based on your text prompts. Perfect for video production, game development, podcasts, and multimedia projects.
- **`magnific-pp-cli ai create-texttoimage`** - This endpoint allows you to generate an image using the HyperFlux model, the fastest Flux model available
- **`magnific-pp-cli ai create-veed-fabric-1-0-fast-lip-sync`** - Generate a realistic talking video by combining a static portrait image with an audio file using Veed Fabric 1.0 Fast.
This is the faster variant of Veed Fabric 1.0, optimized for reduced generation time while maintaining quality lip synchronization.
Ideal for workflows requiring quick turnaround on talking head video generation.
- **`magnific-pp-cli ai create-veed-fabric-1-0-lip-sync`** - Generate a realistic talking video by combining a static portrait image with an audio file using Veed Fabric 1.0.
The model produces a lip-synced video (MP4) where the person in the image speaks naturally in sync with the provided audio.
Ideal for creating talking head videos from a single photo and voice recording.
- **`magnific-pp-cli ai create-video-from-image-kling-elements-pro`** - Generate a video from an image using the Kling Elements Pro model.
- **`magnific-pp-cli ai create-video-from-image-kling-elements-std`** - Generate a video from an image using the Kling Elements Std model.
- **`magnific-pp-cli ai create-video-from-image-kling-v2-1-master`** - Generate a video from an image using the Kling 2.1 Master model.
- **`magnific-pp-cli ai create-video-kling-1-6-pro`** - Generate a video from an image using the Kling 1.6 Pro model.
- **`magnific-pp-cli ai create-video-kling-1-6-std`** - Generate a video from an image using the Kling 1.6 Std model.
- **`magnific-pp-cli ai create-video-kling-2-1-pro`** - Generate a video from an image using the Kling 2.1 Pro model.
- **`magnific-pp-cli ai create-video-kling-2-1-std`** - Generate a video from an image using the Kling 2.1 Std model.
- **`magnific-pp-cli ai create-video-kling-2-5-pro`** - Generate a video from an image using the Kling 2.5 Pro model.
- **`magnific-pp-cli ai create-video-ltx-2-fast-i2v`** - Generate a video from an image using the LTX Video 2.0 Fast model.

**Features:**
- Fast video generation with resolutions up to 4K (2160p)
- Extended duration options: 6-20 seconds in 2-second increments
- Uses the provided image as the first frame
- Optional synchronized audio generation
- **Note:** Durations longer than 10 seconds require 25 FPS and 1080p resolution
- **`magnific-pp-cli ai create-video-ltx-2-fast-t2v`** - Generate a video from text prompt using the LTX Video 2.0 Fast model.

**Features:**
- Fast video generation with resolutions up to 4K (2160p)
- Extended duration options: 6-20 seconds in 2-second increments
- Optional synchronized audio generation
- **Note:** Durations longer than 10 seconds require 25 FPS and 1080p resolution
- **`magnific-pp-cli ai create-video-ltx-2-pro-i2v`** - Generate a video from an image using the LTX Video 2.0 Pro model.

**Features:**
- High-quality video generation with resolutions up to 4K (2160p)
- Duration options: 6, 8, or 10 seconds
- Uses the provided image as the first frame
- Optional synchronized audio generation
- Reproducible results with seed parameter
- **`magnific-pp-cli ai create-video-ltx-2-pro-t2v`** - Generate a video from text prompt using the LTX Video 2.0 Pro model.

**Features:**
- High-quality video generation with resolutions up to 4K (2160p)
- Duration options: 6, 8, or 10 seconds
- Optional synchronized audio generation
- Reproducible results with seed parameter
- **`magnific-pp-cli ai create-video-minimax-hailuo-02-1080p`** - Generate a video from text or image using the MiniMax Hailuo-02 1080p model.
- **`magnific-pp-cli ai create-video-minimax-hailuo-02-768p`** - Generate a video from text or image using the MiniMax Hailuo-02 768p model.
- **`magnific-pp-cli ai create-video-minimax-hailuo-23-1080p`** - Generate a video from text or image using the MiniMax Hailuo 2.3 1080p model.
- **`magnific-pp-cli ai create-video-minimax-hailuo-23-1080p-fast`** - Generate a video from text or image using the MiniMax Hailuo 2.3 1080p model with fast prompt optimization.
- **`magnific-pp-cli ai create-video-minimax-hailuo-23-768p`** - Generate a video from text or image using the MiniMax Hailuo 2.3 768p model.
- **`magnific-pp-cli ai create-video-minimax-hailuo-23-768p-fast`** - Generate a video from text or image using the MiniMax Hailuo 2.3 768p model with fast prompt optimization.
- **`magnific-pp-cli ai create-video-minimax-live-i2v`** - Generate a video from an image using MiniMax Video-01-Live model (Live Illustrations).

**Features:**
- Supports camera movements in square brackets: [Truck left], [Pan right], [Push in], [Pull out], [Zoom in], [Tracking shot], [Static shot]
- Optional prompt optimization for better results
- Works best with illustrations and artwork
- **`magnific-pp-cli ai create-video-omni-human-1-5`** - Generate animated video of a human figure driven by audio using OmniHuman 1.5 model (ByteDance). Supports natural head movements, facial expressions, and body motion synchronized to audio input.
- **`magnific-pp-cli ai create-video-runway-45-i2v`** - Generate high-quality videos from images using RunWay Gen 4.5 model.

**Features:**
- Transform static images into dynamic videos
- Precise motion control via text prompts
- Multiple aspect ratios for different use cases
- Duration options: 5, 8, or 10 seconds
- Reproducible results with seed parameter

**Supported aspect ratios:**
- `1280:720`: Landscape (16:9) - ideal for YouTube, presentations
- `720:1280`: Portrait (9:16) - ideal for TikTok, Instagram Reels
- `1104:832`: Landscape (4:3) - classic format
- `960:960`: Square (1:1) - ideal for Instagram posts
- `832:1104`: Portrait (3:4) - ideal for Pinterest

**Use cases:** Product animations, social media content, marketing videos, and bringing photos to life.
- **`magnific-pp-cli ai create-video-runway-45-t2v`** - Generate high-quality videos from text descriptions using RunWay Gen 4.5 model.

**Features:**
- State-of-the-art text-to-video generation
- Multiple aspect ratios for different use cases
- Duration options: 5, 8, or 10 seconds
- High visual fidelity and motion quality

**Supported aspect ratios:**
- `1280:720`: Landscape (16:9) - ideal for YouTube, presentations
- `720:1280`: Portrait (9:16) - ideal for TikTok, Instagram Reels
- `1104:832`: Landscape (4:3) - classic format
- `960:960`: Square (1:1) - ideal for Instagram posts
- `832:1104`: Portrait (3:4) - ideal for Pinterest

**Use cases:** Social media content, marketing videos, creative projects, and visual storytelling.
- **`magnific-pp-cli ai create-video-runway-act-two`** - Generate a character performance video using RunWay Act Two model. Transfer facial expressions and body movements from a reference video to a character image or video.
- **`magnific-pp-cli ai create-video-runway-gen4-turbo`** - Generate a video from an image using RunWay Gen4 Turbo model.
- **`magnific-pp-cli ai create-video-seedance-1-5-pro-1080p`** - Generate 1080p video with synchronized audio using Seedance 1.5 Pro. Supports text-to-video and image-to-video with lip-sync, dialogue, foley, and music generation.
- **`magnific-pp-cli ai create-video-seedance-1-5-pro-480p`** - Generate 480p video with synchronized audio using Seedance 1.5 Pro. Supports text-to-video and image-to-video with lip-sync, dialogue, foley, and music generation.
- **`magnific-pp-cli ai create-video-seedance-1-5-pro-720p`** - Generate 720p video with synchronized audio using Seedance 1.5 Pro. Supports text-to-video and image-to-video with lip-sync, dialogue, foley, and music generation.
- **`magnific-pp-cli ai create-video-seedance-lite-1080p`** - Generate a video from image using the Seedance Lite 1080p model.
- **`magnific-pp-cli ai create-video-seedance-lite-480p`** - Generate a video from image using the Seedance Lite 480p model.
- **`magnific-pp-cli ai create-video-seedance-lite-720p`** - Generate a video from image using the Seedance Lite 720p model.
- **`magnific-pp-cli ai create-video-seedance-pro-1080p`** - Generate a video from image using the Seedance Pro 1080p model.
- **`magnific-pp-cli ai create-video-seedance-pro-480p`** - Generate a video from image using the Seedance Pro 480p model.
- **`magnific-pp-cli ai create-video-seedance-pro-720p`** - Generate a video from image using the Seedance Pro 720p model.
- **`magnific-pp-cli ai create-video-veo-31-i2v`** - Generate a video from an image using Google Veo 3.1 model. Supports multiple resolutions (720p, 1080p, 4K) and optional audio generation.
- **`magnific-pp-cli ai create-video-veo-31-i2v-fast`** - Generate a video from an image using Google Veo 3.1 Fast model. Faster generation at a lower cost.
- **`magnific-pp-cli ai create-video-veo-31-ref2v`** - Generate a video with character or object consistency using reference images. Maintains visual identity across scenes for storytelling and multi-scene projects. Supports 720p, 1080p, and 4K resolutions with native audio generation including dialogue and sound effects. Fixed 8-second duration at 24 FPS.
- **`magnific-pp-cli ai create-video-veo-31-t2v`** - Generate a video from text prompt using Google Veo 3.1 model. Supports multiple resolutions (720p, 1080p, 4K) and optional audio generation.
- **`magnific-pp-cli ai create-video-veo-31-t2v-fast`** - Generate a video from text prompt using Google Veo 3.1 Fast model. Faster generation at a lower cost.
- **`magnific-pp-cli ai create-video-vfx`** - Apply professional visual effects and filters to your videos using the VFX API. Transform any video with cinematic effects like film grain, motion blur, VHS retro style, and anamorphic lens distortion.

**Available filters:**
- `1` Film grain - Adds cinematic grain texture for a classic film look
- `2` Motion blur - Creates motion blur effect (configurable kernel size and decay)
- `3` Fish eye - Applies fish eye lens distortion
- `4` VHS - Retro VHS tape effect with scan lines and color distortion
- `5` Shake - Camera shake effect for dynamic footage
- `6` VGA - Low resolution VGA effect for retro aesthetics
- `7` Bloom - Glowing bloom effect (adjustable contrast)
- `8` Anamorphic lens - Cinematic anamorphic lens effect with horizontal flares

**Use cases:** Social media content, music videos, film production, advertising, retro-style videos, and creative projects.
- **`magnific-pp-cli ai create-video-wan-25-i2v-1080p`** - Generate a 1080p video from image using the WAN 2.5 model.
- **`magnific-pp-cli ai create-video-wan-25-i2v-480p`** - Generate a 480p video from image using the WAN 2.5 model.
- **`magnific-pp-cli ai create-video-wan-25-i2v-720p`** - Generate a 720p video from image using the WAN 2.5 model.
- **`magnific-pp-cli ai create-video-wan-25-t2v-1080p`** - Generate a 1080p video from text prompt using the WAN 2.5 model.
- **`magnific-pp-cli ai create-video-wan-25-t2v-480p`** - Generate a 480p video from text prompt using the WAN 2.5 model.
- **`magnific-pp-cli ai create-video-wan-25-t2v-720p`** - Generate a 720p video from text prompt using the WAN 2.5 model.
- **`magnific-pp-cli ai create-video-wan-27-edit`** - Edit an existing video using WAN 2.7. Transform video content with text instructions, apply visual references, or transfer artistic styles.

**Three editing modes:**
- **Instruction-based editing**: Provide `prompt` with editing instructions (e.g., "change the sky to sunset")
- **Reference-image editing**: Provide `prompt` and `image_urls` to apply visual references to the video
- **Style transfer**: Provide `prompt` and `image_urls` with style reference images to transfer artistic styles

**Key features:**
- 720P and 1080P resolution support
- Up to 3 reference images for editing or style transfer
- Set `duration` to `0` to preserve the full input video duration
- Configurable audio handling: auto-generate or preserve original
- **`magnific-pp-cli ai create-video-wan-27-i2v`** - Generate a video from an image or extend an existing video using WAN 2.7.

**Three generation modes:**
- **First frame**: Provide `start_image_url` alone to animate from a starting image
- **First + last frame**: Provide both `start_image_url` and `end_image_url` for controlled start-to-end animation
- **Video continuation**: Provide `video_url` to extend an existing video, optionally with `end_image_url` as the target ending frame

**Key features:**
- 720P and 1080P resolution support
- Optional audio-guided generation
- Duration range: 2-15 seconds
- **`magnific-pp-cli ai create-video-wan-27-r2v`** - Generate a video featuring characters from reference images or videos using WAN 2.7. Maintains visual identity of referenced characters across the generated video.

**How to use references:**
- Provide character images via `image_urls` and/or character videos via `video_urls`
- Combined total of image and video references must not exceed 5
- Reference characters in the prompt as "Image 1", "Image 2", "Video 1", etc.
- Optionally include `reference_voice` per character for voice-guided generation

**Key features:**
- 720P and 1080P resolution support
- 5 aspect ratio options (or use `start_image_url` to set aspect ratio from an image)
- Duration range: 2-10 seconds
- **`magnific-pp-cli ai create-video-wan-27-t2v`** - Generate a video from a text prompt using WAN 2.7. Supports configurable aspect ratios (16:9, 9:16, 1:1, 4:3, 3:4), resolutions (720P, 1080P), optional audio input, and duration from 2 to 15 seconds.

**Key features:**
- 720P and 1080P resolution support
- 5 aspect ratio options
- Optional audio-guided generation
- Automatic prompt expansion for richer output
- Duration range: 2-15 seconds
- **`magnific-pp-cli ai create-video-wan-v22-480p`** - Generate a video from image using the WAN 2.2 480p model.
- **`magnific-pp-cli ai create-video-wan-v22-580p`** - Generate a video from image using the WAN 2.2 580p model.
- **`magnific-pp-cli ai create-video-wan-v22-720p`** - Generate a video from image using the WAN 2.2 720p model.
- **`magnific-pp-cli ai create-video-wan-v26-i2v-1080p`** - Generate a 1080p video from image using the WAN 2.6 model.
- **`magnific-pp-cli ai create-video-wan-v26-i2v-720p`** - Generate a 720p video from image using the WAN 2.6 model.
- **`magnific-pp-cli ai create-video-wan-v26-t2v-1080p`** - Generate a 1080p video from text prompt using the WAN 2.6 model.
- **`magnific-pp-cli ai create-video-wan-v26-t2v-720p`** - Generate a 720p video from text prompt using the WAN 2.6 model.
- **`magnific-pp-cli ai create-voiceover-task`** - Generate natural-sounding speech from text using ElevenLabs AI voices.

Create professional voiceovers for videos, podcasts, presentations, and more.
Supports multiple languages, voice customization, and high-quality audio output.
- **`magnific-pp-cli ai detect-image`** - Accepts an image file as input and analyzes it to determine the probability that the image was generated by artificial intelligence, providing a confidence score.
- **`magnific-pp-cli ai download-text-to-icon-render-format`** - Download the generated AI icon in the specified format png or svg.
- **`magnific-pp-cli ai generate-text-to-icon`** - Create stunning icons in different styles and formats (png, svg) from text prompts using our advanced AI models.
- **`magnific-pp-cli ai generate-text-to-icon-preview`** - Create stunning previews icons in different styles and formats (png, svg) from text prompts using our advanced AI models.
- **`magnific-pp-cli ai get`** - Gemini 2.5 Flash - Get task status
- **`magnific-pp-cli ai get-all-audio-isolation-tasks`** - Get the status of all audio isolation tasks
- **`magnific-pp-cli ai get-all-change-camera-tasks`** - Retrieve the status of all Change Camera tasks for the authenticated user. Returns a list of tasks with their current status, creation time, and result URLs for completed tasks.
- **`magnific-pp-cli ai get-all-flux-2-klein-tasks`** - Retrieve the status of all FLUX.2 [klein] text-to-image generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-flux-2-pro-tasks`** - Retrieve the status of all Flux 2 Pro text-to-image generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-flux-2-turbo-tasks`** - Retrieve the status of all Flux 2 Turbo text-to-image generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-flux-kontext-pro-tasks`** - Retrieve the status of all Flux Kontext Pro text-to-image generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-ideogram-image-edit-tasks`** - Get the status of all Ideogram Image Edit tasks
- **`magnific-pp-cli ai get-all-image-to-prompt-tasks`** - Get the status of all image-to-prompt tasks
- **`magnific-pp-cli ai get-all-imagen3-tasks`** - Get the status of all Imagen3 tasks
- **`magnific-pp-cli ai get-all-imagen4-fast-tasks`** - Get the status of all Imagen4 Fast tasks
- **`magnific-pp-cli ai get-all-imagen4-ultra-tasks`** - Get the status of all Imagen4 Ultra tasks
- **`magnific-pp-cli ai get-all-improve-prompt-tasks`** - Get the status of all improve-prompt tasks
- **`magnific-pp-cli ai get-all-kling-elements-std-tasks`** - Get the list of the kling-elements-std tasks
- **`magnific-pp-cli ai get-all-kling-pro-tasks`** - Get the list of the kling-pro tasks
- **`magnific-pp-cli ai get-all-latent-sync-tasks`** - Get the status of all Latent-Sync lip-sync tasks
- **`magnific-pp-cli ai get-all-ltx-2-fast-i2v-tasks`** - LTX Video 2.0 Fast I2V - List tasks
- **`magnific-pp-cli ai get-all-ltx-2-fast-t2v-tasks`** - LTX Video 2.0 Fast T2V - List tasks
- **`magnific-pp-cli ai get-all-ltx-2-pro-i2v-tasks`** - LTX Video 2.0 Pro I2V - List tasks
- **`magnific-pp-cli ai get-all-ltx-2-pro-t2v-tasks`** - LTX Video 2.0 Pro T2V - List tasks
- **`magnific-pp-cli ai get-all-minimax-live-i2v-tasks`** - MiniMax Video 01 Live - List tasks
- **`magnific-pp-cli ai get-all-music-generation-tasks`** - Get the status of all music-generation tasks
- **`magnific-pp-cli ai get-all-nano-banana-pro-flash-tasks`** - Get the status of all Nano Banana Pro Flash image generation tasks
- **`magnific-pp-cli ai get-all-nano-banana-pro-tasks`** - Get the status of all Nano Banana Pro image generation tasks
- **`magnific-pp-cli ai get-all-omni-human-1-5-tasks`** - OmniHuman 1.5 - List tasks
- **`magnific-pp-cli ai get-all-runway-45-i2v-tasks`** - Retrieve the status of all RunWay Gen 4.5 image-to-video generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-runway-45-t2v-tasks`** - Retrieve the status of all RunWay Gen 4.5 text-to-video generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-runway-act-two-tasks`** - RunWay Act Two - List tasks
- **`magnific-pp-cli ai get-all-runway-gen4-turbo-tasks`** - RunWay Gen4 Turbo - List tasks
- **`magnific-pp-cli ai get-all-runway-text-to-image-tasks`** - Get the status of all RunWay text-to-image generation tasks
- **`magnific-pp-cli ai get-all-seedance-1-5-pro-1080p-tasks`** - Seedance 1.5 Pro 1080p - List tasks
- **`magnific-pp-cli ai get-all-seedance-1-5-pro-480p-tasks`** - Seedance 1.5 Pro 480p - List tasks
- **`magnific-pp-cli ai get-all-seedance-1-5-pro-720p-tasks`** - Seedance 1.5 Pro 720p - List tasks
- **`magnific-pp-cli ai get-all-seedream-tasks`** - Get the status of all Seedream tasks
- **`magnific-pp-cli ai get-all-seedream-v4-5-edit-tasks`** - Get the status of all Seedream 4.5 image editing tasks
- **`magnific-pp-cli ai get-all-seedream-v4-5-tasks`** - Get the status of all Seedream 4.5 image generation tasks
- **`magnific-pp-cli ai get-all-seedream-v4-edit-tasks`** - Get the status of all Seedream v4 edit tasks
- **`magnific-pp-cli ai get-all-seedream-v4-tasks`** - Get the status of all Seedream v4 tasks
- **`magnific-pp-cli ai get-all-seedream-v5-lite-edit-tasks`** - Get the status of all Seedream V5 Lite image editing tasks
- **`magnific-pp-cli ai get-all-seedream-v5-lite-tasks`** - Get the status of all Seedream V5 Lite image generation tasks
- **`magnific-pp-cli ai get-all-sound-effects-tasks`** - Get the status of all sound-effects tasks
- **`magnific-pp-cli ai get-all-style-transfer-tasks`** - Get the status of all Style Transfer tasks
- **`magnific-pp-cli ai get-all-veed-fabric-1-0-fast-tasks`** - Retrieve all Veed Fabric 1.0 Fast lip-sync tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-veed-fabric-1-0-tasks`** - Retrieve all Veed Fabric 1.0 lip-sync tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-veo-31-i2v-fast-tasks`** - Get all Veo 3.1 I2V Fast tasks
- **`magnific-pp-cli ai get-all-veo-31-i2v-tasks`** - Get all Veo 3.1 I2V tasks
- **`magnific-pp-cli ai get-all-veo-31-ref2v-tasks`** - Retrieve a list of all reference-to-video generation tasks for Veo 3.1.
- **`magnific-pp-cli ai get-all-veo-31-t2v-fast-tasks`** - Get all Veo 3.1 T2V Fast tasks
- **`magnific-pp-cli ai get-all-veo-31-t2v-tasks`** - Get all Veo 3.1 T2V tasks
- **`magnific-pp-cli ai get-all-vfx-tasks`** - Retrieve the status of all VFX video effect tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-voiceover-tasks`** - Get the status of all voiceover tasks
- **`magnific-pp-cli ai get-all-wan-27-i2v-tasks`** - Retrieve the list of all WAN 2.7 image-to-video tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-wan-27-r2v-tasks`** - Retrieve the list of all WAN 2.7 reference-to-video tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-wan-27-t2v-tasks`** - Retrieve the list of all WAN 2.7 text-to-video tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-wan-27-video-edit-tasks`** - Retrieve the list of all WAN 2.7 video edit tasks for the authenticated user.
- **`magnific-pp-cli ai get-all-z-image-tasks`** - Get the status of all Z-Image image generation tasks
- **`magnific-pp-cli ai get-app`** - Returns the full definition of a workflow app, including its dynamic inputs and cost.

**Inputs** define what values the caller must provide when running the app:
- `creation` type: accepts an image/media URL, base64-encoded data, or an existing creation identifier
- `text` type: accepts a plain text string (prompt, label, etc.)
- `number` type: accepts a numeric value
- `select` type: accepts a value from predefined options

Use each input's `id` field (UUID format) as the identifier when calling the run endpoint.
The `tool_metadata.total_cost` field indicates the credit cost per execution.

<Note>
  **Enterprise exclusive.** This endpoint is only available to Enterprise customers. [Contact sales](https://www.freepik.com/api#contact) for access.
</Note>
- **`magnific-pp-cli ai get-app-run`** - Retrieve the current status and results of a workflow app execution run.

Use this endpoint to **poll** the status of a run after calling the
`POST /v1/ai/apps/{app-id}/run` endpoint. The `run_id` path parameter
corresponds to the `workflow_run_identifier` returned in the run response.

**Polling strategy:**
- Wait 2-3 seconds after starting the run before the first poll
- Poll every 3-5 seconds
- When `status` is `completed`, the `result` field contains download URLs
- When `status` is `failed`, the `error_message` field describes the failure

**Asset URLs** in the `result` field are temporary and valid for **12 hours**.

**Statuses:**
| Status | Description |
|--------|-------------|
| `pending` | Run is queued but has not started yet |
| `running` | Workflow is executing |
| `completed` | Finished successfully, `result` is available |
| `completed_with_errors` | Execution finished but some nodes failed |
| `failed` | Execution failed, see `error_message` |
| `cancelled` | Execution was cancelled |

<Note>
  **Enterprise exclusive.** This endpoint is only available to Enterprise customers. [Contact sales](https://www.freepik.com/api#contact) for access.
</Note>
- **`magnific-pp-cli ai get-audio-isolation-task-status`** - Get the status of one audio isolation task
- **`magnific-pp-cli ai get-available-loras-list`** - Get available loras list
- **`magnific-pp-cli ai get-change-camera-task`** - Retrieve the status and result of a specific Change Camera task by its task ID. When the task status is `completed`, the response includes the URL of the generated image with the new camera angle.
- **`magnific-pp-cli ai get-flux-2-klein-task`** - Retrieve the status and results of a specific FLUX.2 [klein] generation task.
- **`magnific-pp-cli ai get-flux-2-pro-task`** - Retrieve the status and results of a specific Flux 2 Pro generation task.
- **`magnific-pp-cli ai get-flux-2-turbo-task`** - Retrieve the status and results of a specific Flux 2 Turbo generation task.
- **`magnific-pp-cli ai get-flux-kontext-pro-task`** - Retrieve the status and results of a specific Flux Kontext Pro text-to-image generation task.
- **`magnific-pp-cli ai get-ideogram-image-edit-task-status`** - Get the status of a specific Ideogram Image Edit task
- **`magnific-pp-cli ai get-image-to-prompt-task-status`** - Get the status of one image-to-prompt task
- **`magnific-pp-cli ai get-image-to-video-kling-v26-task`** - Kling 2.6 Pro - Get task status
- **`magnific-pp-cli ai get-image-to-video-kling-v26-tasks`** - Kling 2.6 Pro - List tasks
- **`magnific-pp-cli ai get-image-upscaler-precision-task`** - Returns the current status and output URL of a specific precision upscaler task. The output URL is included only if the task has completed successfully.
- **`magnific-pp-cli ai get-image-upscaler-precision-tasks`** - Returns a list of all precision upscaler tasks. Each task includes its ID, current status, and output URL if completed.
- **`magnific-pp-cli ai get-image-upscaler-precision-v2-task`** - Returns the current status and output URL of a specific precision upscaler V2 task. The output URL is included only if the task has completed successfully.
- **`magnific-pp-cli ai get-image-upscaler-precision-v2-tasks`** - Returns a list of all precision upscaler V2 tasks. Each task includes its ID, current status, and output URL if completed.
- **`magnific-pp-cli ai get-imageexpand`** - Get the status of one image expand task
- **`magnific-pp-cli ai get-imageexpand-2`** - Get the status of one image expand ideogram task
- **`magnific-pp-cli ai get-imageexpand-3`** - Get the status of one image expand seedream v4.5 task
- **`magnific-pp-cli ai get-imagen3-task-status`** - Get the status of the Imagen3 task
- **`magnific-pp-cli ai get-imagen4-fast-task-status`** - Get the status of the Imagen4 Fast task
- **`magnific-pp-cli ai get-imagen4-ultra-task-status`** - Get the status of the Imagen4 Ultra task
- **`magnific-pp-cli ai get-imagerelight`** - Get the status of the relight task
- **`magnific-pp-cli ai get-imagetovideo`** - Get the status of the kling-elements task
- **`magnific-pp-cli ai get-imagetovideo-10`** - Get the status of the MiniMax Hailuo 2.3 768p Fast task
- **`magnific-pp-cli ai get-imagetovideo-11`** - Get the status of the MiniMax Hailuo 2.3 768p task
- **`magnific-pp-cli ai get-imagetovideo-12`** - PixVerse V5.5 - Get task status
- **`magnific-pp-cli ai get-imagetovideo-13`** - PixVerse V5 Transition - Get task status
- **`magnific-pp-cli ai get-imagetovideo-14`** - PixVerse V5 - Get task status
- **`magnific-pp-cli ai get-imagetovideo-15`** - PixVerse V6 - Get task status
- **`magnific-pp-cli ai get-imagetovideo-16`** - Get the status of the Seedance Lite 1080p task
- **`magnific-pp-cli ai get-imagetovideo-17`** - Get the status of the Seedance Lite 480p task
- **`magnific-pp-cli ai get-imagetovideo-18`** - Get the status of the Seedance Lite 720p task
- **`magnific-pp-cli ai get-imagetovideo-19`** - Get the status of the Seedance Pro 1080p task
- **`magnific-pp-cli ai get-imagetovideo-2`** - Get the status of the kling-v2-1-master task
- **`magnific-pp-cli ai get-imagetovideo-20`** - Get the status of the Seedance Pro 480p task
- **`magnific-pp-cli ai get-imagetovideo-21`** - Get the status of the Seedance Pro 720p task
- **`magnific-pp-cli ai get-imagetovideo-22`** - Get the status of a WAN 2.5 Image-to-Video 1080p task
- **`magnific-pp-cli ai get-imagetovideo-23`** - Get the status of a WAN 2.5 Image-to-Video 480p task
- **`magnific-pp-cli ai get-imagetovideo-24`** - Get the status of a WAN 2.5 Image-to-Video 720p task
- **`magnific-pp-cli ai get-imagetovideo-25`** - Get the status of the WAN 2.2 480p task
- **`magnific-pp-cli ai get-imagetovideo-26`** - Get the status of the WAN 2.2 580p task
- **`magnific-pp-cli ai get-imagetovideo-27`** - Get the status of the WAN 2.2 720p task
- **`magnific-pp-cli ai get-imagetovideo-28`** - Get the status of a WAN 2.6 Image-to-Video 1080p task
- **`magnific-pp-cli ai get-imagetovideo-29`** - Get the status of a WAN 2.6 Image-to-Video 720p task
- **`magnific-pp-cli ai get-imagetovideo-3`** - Get the status of the kling-v2-5-pro task
- **`magnific-pp-cli ai get-imagetovideo-4`** - Get the status of the kling-v2 task
- **`magnific-pp-cli ai get-imagetovideo-5`** - Get the status of the kling task
- **`magnific-pp-cli ai get-imagetovideo-6`** - Get the status of the MiniMax Hailuo-02 1080p task
- **`magnific-pp-cli ai get-imagetovideo-7`** - Get the status of the MiniMax Hailuo-02 768p task
- **`magnific-pp-cli ai get-imagetovideo-8`** - Get the status of the MiniMax Hailuo 2.3 1080p Fast task
- **`magnific-pp-cli ai get-imagetovideo-9`** - Get the status of the MiniMax Hailuo 2.3 1080p task
- **`magnific-pp-cli ai get-imageupscaler`** - Get the status of the upscaling task
- **`magnific-pp-cli ai get-improve-prompt-task-status`** - Get the status of one improve-prompt task
- **`magnific-pp-cli ai get-kling-2-1-task-status`** - Get the status of the kling-v2-1 task
- **`magnific-pp-cli ai get-kling-o1-task`** - Kling O1 - Get task status
- **`magnific-pp-cli ai get-latent-sync-task-status`** - Get the status of one Latent-Sync lip-sync task
- **`magnific-pp-cli ai get-ltx-2-fast-i2v-task`** - LTX Video 2.0 Fast I2V - Get task status
- **`magnific-pp-cli ai get-ltx-2-fast-t2v-task`** - LTX Video 2.0 Fast T2V - Get task status
- **`magnific-pp-cli ai get-ltx-2-pro-i2v-task`** - LTX Video 2.0 Pro I2V - Get task status
- **`magnific-pp-cli ai get-ltx-2-pro-t2v-task`** - LTX Video 2.0 Pro T2V - Get task status
- **`magnific-pp-cli ai get-minimax-live-i2v-task`** - MiniMax Video 01 Live - Get task status
- **`magnific-pp-cli ai get-music-generation-task-status`** - Get the status of one music-generation task
- **`magnific-pp-cli ai get-mystic-task-status`** - Get the status of the Mystic task
- **`magnific-pp-cli ai get-nano-banana-pro-flash-task-status`** - Get the status of a specific Nano Banana Pro Flash image generation task
- **`magnific-pp-cli ai get-nano-banana-pro-task-status`** - Get the status of a specific Nano Banana Pro image generation task
- **`magnific-pp-cli ai get-omni-human-1-5-task`** - OmniHuman 1.5 - Get task status
- **`magnific-pp-cli ai get-reference-to-video-kling-v3-omni-task`** - Retrieve the status and result of a specific Kling 3 Omni reference-to-video task (Pro or Standard) by its task ID.
- **`magnific-pp-cli ai get-reference-to-video-kling-v3-omni-tasks`** - Retrieve the list of all Kling 3 Omni reference-to-video tasks (both Pro and Standard) for the authenticated user.
- **`magnific-pp-cli ai get-runway-45-i2v-task`** - Retrieve the status and result of a specific RunWay Gen 4.5 image-to-video task by its task ID.
- **`magnific-pp-cli ai get-runway-45-t2v-task`** - Retrieve the status and result of a specific RunWay Gen 4.5 text-to-video task by its task ID.
- **`magnific-pp-cli ai get-runway-act-two-task`** - RunWay Act Two - Get task status
- **`magnific-pp-cli ai get-runway-gen4-turbo-task`** - RunWay Gen4 Turbo - Get task status
- **`magnific-pp-cli ai get-runway-text-to-image-task-status`** - Get the status and result of a specific RunWay text-to-image task
- **`magnific-pp-cli ai get-seedance-1-5-pro-1080p-task`** - Seedance 1.5 Pro 1080p - Get task status
- **`magnific-pp-cli ai get-seedance-1-5-pro-480p-task`** - Seedance 1.5 Pro 480p - Get task status
- **`magnific-pp-cli ai get-seedance-1-5-pro-720p-task`** - Seedance 1.5 Pro 720p - Get task status
- **`magnific-pp-cli ai get-seedream-task-status`** - Get the status of the Seedream task
- **`magnific-pp-cli ai get-seedream-v4-5-edit-task-status`** - Get the status of a specific Seedream 4.5 image editing task
- **`magnific-pp-cli ai get-seedream-v4-5-task-status`** - Get the status of a specific Seedream 4.5 image generation task
- **`magnific-pp-cli ai get-seedream-v4-edit-task-status`** - Get the status of the Seedream v4 edit task
- **`magnific-pp-cli ai get-seedream-v4-task-status`** - Get the status of the Seedream v4 task
- **`magnific-pp-cli ai get-seedream-v5-lite-edit-task-status`** - Get the status of a specific Seedream V5 Lite image editing task
- **`magnific-pp-cli ai get-seedream-v5-lite-task-status`** - Get the status of a specific Seedream V5 Lite image generation task
- **`magnific-pp-cli ai get-skinenhancer`** - Skin Enhancer - Get task status
- **`magnific-pp-cli ai get-sound-effects-task-status`** - Get the status of one sound-effects task
- **`magnific-pp-cli ai get-style-transfer-task-status`** - Get the status of the Style Transfer task
- **`magnific-pp-cli ai get-texttoimage`** - Get the status of the flux-dev task
- **`magnific-pp-cli ai get-texttoimage-2`** - Get the status of the flux-pro 1.1 task
- **`magnific-pp-cli ai get-texttoimage-3`** - HyperFlux - Get task status
- **`magnific-pp-cli ai get-texttovideo`** - Get the status of a WAN 2.5 Text-to-Video 1080p task
- **`magnific-pp-cli ai get-texttovideo-2`** - Get the status of a WAN 2.5 Text-to-Video 480p task
- **`magnific-pp-cli ai get-texttovideo-3`** - Get the status of a WAN 2.5 Text-to-Video 720p task
- **`magnific-pp-cli ai get-texttovideo-4`** - Get the status of a WAN 2.6 Text-to-Video 1080p task
- **`magnific-pp-cli ai get-texttovideo-5`** - Get the status of a WAN 2.6 Text-to-Video 720p task
- **`magnific-pp-cli ai get-veed-fabric-1-0-fast-task-status`** - Retrieve a specific Veed Fabric 1.0 Fast lip-sync task by its ID, including generation status and result URL when completed.
- **`magnific-pp-cli ai get-veed-fabric-1-0-task-status`** - Retrieve a specific Veed Fabric 1.0 lip-sync task by its ID, including generation status and result URL when completed.
- **`magnific-pp-cli ai get-veo-31-i2v-fast-task`** - Get Veo 3.1 I2V Fast task by ID
- **`magnific-pp-cli ai get-veo-31-i2v-task`** - Get Veo 3.1 I2V task by ID
- **`magnific-pp-cli ai get-veo-31-ref2v-task`** - Retrieve the status and results of a specific reference-to-video generation task.
- **`magnific-pp-cli ai get-veo-31-t2v-fast-task`** - Get Veo 3.1 T2V Fast task by ID
- **`magnific-pp-cli ai get-veo-31-t2v-task`** - Get Veo 3.1 T2V task by ID
- **`magnific-pp-cli ai get-vfx-task`** - Retrieve the status and results of a specific VFX video effect task by its task ID.
- **`magnific-pp-cli ai get-video-kling-advanced-custom-elements-task`** - **Deprecated.** This endpoint is deprecated. You can pass reference images directly in the `elements` array
of the Kling V3 video generation endpoints (`/ai/video/kling-v3-pro`, `/ai/video/kling-v3-std`)
without needing to pre-create elements.

Retrieve the status and result of a specific Advanced Custom Elements creation task by its task ID.
- **`magnific-pp-cli ai get-video-kling-advanced-custom-elements-tasks`** - **Deprecated.** This endpoint is deprecated. You can pass reference images directly in the `elements` array
of the Kling V3 video generation endpoints (`/ai/video/kling-v3-pro`, `/ai/video/kling-v3-std`)
without needing to pre-create elements.

Retrieve the list of all Advanced Custom Elements creation tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-kling-v3-motion-control-pro-task`** - Retrieve the status and result of a specific Kling 3 Pro Motion Control video generation task by its task ID.
- **`magnific-pp-cli ai get-video-kling-v3-motion-control-pro-tasks`** - Retrieve the list of all Kling 3 Pro Motion Control video generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-kling-v3-motion-control-std-task`** - Retrieve the status and result of a specific Kling 3 Standard Motion Control video generation task by its task ID.
- **`magnific-pp-cli ai get-video-kling-v3-motion-control-std-tasks`** - Retrieve the list of all Kling 3 Standard Motion Control video generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-kling-v3-omni-task`** - Retrieve the status and result of a specific Kling 3 Omni video generation task by its task ID.
- **`magnific-pp-cli ai get-video-kling-v3-omni-tasks`** - Retrieve the list of all Kling 3 Omni video generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-kling-v3-task`** - Retrieve the status and result of a specific Kling 3 video generation task by its task ID.
- **`magnific-pp-cli ai get-video-kling-v3-tasks`** - Retrieve the list of all Kling 3 video generation tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-kling4k-i2v-task`** - Retrieve the status and result of a specific Kling 4K Image-to-Video task by its task ID.
- **`magnific-pp-cli ai get-video-kling4k-i2v-tasks`** - Retrieve the list of all Kling 4K Image-to-Video tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-kling4k-t2v-task`** - Retrieve the status and result of a specific Kling 4K Text-to-Video task by its task ID.
- **`magnific-pp-cli ai get-video-kling4k-t2v-tasks`** - Retrieve the list of all Kling 4K Text-to-Video tasks for the authenticated user.
- **`magnific-pp-cli ai get-video-upscaler-precision-task`** - Returns the current status and output URL of a specific video upscaler precision task. The output URL is included only if the task has completed successfully.
- **`magnific-pp-cli ai get-video-upscaler-precision-tasks`** - Returns a list of all video upscaler precision tasks. Each task includes its ID, current status, and output URL if completed.
- **`magnific-pp-cli ai get-video-upscaler-task`** - Returns the current status and output URL of a specific video upscaler task. The output URL is included only if the task has completed successfully.
- **`magnific-pp-cli ai get-video-upscaler-tasks`** - Returns a list of all video upscaler tasks. Each task includes its ID, current status, and output URL if completed.
- **`magnific-pp-cli ai get-voiceover-task-status`** - Get the status of one voiceover task
- **`magnific-pp-cli ai get-wan-27-i2v-task`** - Retrieve the status and result of a specific WAN 2.7 image-to-video task by its ID.
- **`magnific-pp-cli ai get-wan-27-r2v-task`** - Retrieve the status and result of a specific WAN 2.7 reference-to-video task by its ID.
- **`magnific-pp-cli ai get-wan-27-t2v-task`** - Retrieve the status and result of a specific WAN 2.7 text-to-video task by its ID.
- **`magnific-pp-cli ai get-wan-27-video-edit-task`** - Retrieve the status and result of a specific WAN 2.7 video edit task by its ID.
- **`magnific-pp-cli ai get-z-image-task-status`** - Get the status of a specific Z-Image image generation task
- **`magnific-pp-cli ai list`** - Gemini 2.5 Flash - List tasks
- **`magnific-pp-cli ai list-apps`** - Returns all workflow apps **published as tools** that the authenticated user can execute.
Each item includes the app identifier, name, and tool metadata.

Workflow apps are visual AI pipelines created in [Freepik Spaces](https://www.freepik.com/spaces).
Each app chains together AI tools (upscale, generate, edit, etc.) and can be executed via API.

**Use this endpoint to discover** runnable apps, then call the detail endpoint to get input definitions before running them.

<Note>
  **Enterprise exclusive.** This endpoint is only available to Enterprise customers. [Contact sales](https://www.freepik.com/api#contact) for access.
</Note>
- **`magnific-pp-cli ai list-imageexpand`** - Get the status of all image expand tasks
- **`magnific-pp-cli ai list-imageexpand-2`** - Get the status of all image expand ideogram tasks
- **`magnific-pp-cli ai list-imageexpand-3`** - Get the status of all image expand seedream v4.5 tasks
- **`magnific-pp-cli ai list-imagerelight`** - Get the status of all relight tasks
- **`magnific-pp-cli ai list-imagetovideo`** - Get the list of the kling-elements-pro tasks
- **`magnific-pp-cli ai list-imagetovideo-10`** - Get the list of MiniMax Hailuo-02 768p tasks
- **`magnific-pp-cli ai list-imagetovideo-11`** - Get the list of MiniMax Hailuo 2.3 1080p tasks
- **`magnific-pp-cli ai list-imagetovideo-12`** - Get the list of MiniMax Hailuo 2.3 1080p Fast tasks
- **`magnific-pp-cli ai list-imagetovideo-13`** - Get the list of MiniMax Hailuo 2.3 768p tasks
- **`magnific-pp-cli ai list-imagetovideo-14`** - Get the list of MiniMax Hailuo 2.3 768p Fast tasks
- **`magnific-pp-cli ai list-imagetovideo-15`** - List PixVerse-V5 tasks filtered by resolution provided in the request body.
- **`magnific-pp-cli ai list-imagetovideo-16`** - PixVerse V5.5 - List tasks
- **`magnific-pp-cli ai list-imagetovideo-17`** - PixVerse V5 Transition - List tasks
- **`magnific-pp-cli ai list-imagetovideo-18`** - PixVerse V6 - List tasks
- **`magnific-pp-cli ai list-imagetovideo-19`** - Get the list of Seedance Lite 1080p tasks
- **`magnific-pp-cli ai list-imagetovideo-2`** - Get the list of the kling-o1 tasks
- **`magnific-pp-cli ai list-imagetovideo-20`** - Get the list of Seedance Lite 480p tasks
- **`magnific-pp-cli ai list-imagetovideo-21`** - Get the list of Seedance Lite 720p tasks
- **`magnific-pp-cli ai list-imagetovideo-22`** - Get the list of Seedance Pro 1080p tasks
- **`magnific-pp-cli ai list-imagetovideo-23`** - Get the list of Seedance Pro 480p tasks
- **`magnific-pp-cli ai list-imagetovideo-24`** - Get the list of Seedance Pro 720p tasks
- **`magnific-pp-cli ai list-imagetovideo-25`** - Get the list of WAN 2.5 Image-to-Video 1080p tasks
- **`magnific-pp-cli ai list-imagetovideo-26`** - Get the list of WAN 2.5 Image-to-Video 480p tasks
- **`magnific-pp-cli ai list-imagetovideo-27`** - Get the list of WAN 2.5 Image-to-Video 720p tasks
- **`magnific-pp-cli ai list-imagetovideo-28`** - Get the list of WAN 2.2 480p tasks
- **`magnific-pp-cli ai list-imagetovideo-29`** - Get the list of WAN 2.2 580p tasks
- **`magnific-pp-cli ai list-imagetovideo-3`** - Get the list of the kling-pro tasks
- **`magnific-pp-cli ai list-imagetovideo-30`** - Get the list of WAN 2.2 720p tasks
- **`magnific-pp-cli ai list-imagetovideo-31`** - Get the list of WAN 2.6 Image-to-Video 1080p tasks
- **`magnific-pp-cli ai list-imagetovideo-32`** - Get the list of WAN 2.6 Image-to-Video 720p tasks
- **`magnific-pp-cli ai list-imagetovideo-4`** - Get the list of the kling-v2 tasks
- **`magnific-pp-cli ai list-imagetovideo-5`** - Get the list of the kling-v2-1-std tasks
- **`magnific-pp-cli ai list-imagetovideo-6`** - Get the list of the kling-v2-1-pro tasks
- **`magnific-pp-cli ai list-imagetovideo-7`** - Get the list of the kling-v2-1-std tasks
- **`magnific-pp-cli ai list-imagetovideo-8`** - Get the list of the kling-v2-5-pro tasks
- **`magnific-pp-cli ai list-imagetovideo-9`** - Get the list of MiniMax Hailuo-02 1080p tasks
- **`magnific-pp-cli ai list-imageupscaler`** - Get the status of all upscaling tasks
- **`magnific-pp-cli ai list-my-apps`** - Returns all workflow apps **owned** by the authenticated user, including drafts and unpublished workflows.

Unlike `GET /v1/ai/apps` which returns only apps published as tools, this endpoint returns
every app the user has created — regardless of whether it has been published as a tool.

Each item includes the app identifier and name.

<Note>
  **Enterprise exclusive.** This endpoint is only available to Enterprise customers. [Contact sales](https://www.freepik.com/api#contact) for access.
</Note>
- **`magnific-pp-cli ai list-mystic`** - Get the status of all Mystic tasks
- **`magnific-pp-cli ai list-skinenhancer`** - Skin Enhancer - List tasks
- **`magnific-pp-cli ai list-texttoimage`** - Get the status of the flux-dev task
- **`magnific-pp-cli ai list-texttoimage-2`** - Get the status of the flux-pro 1.1 task
- **`magnific-pp-cli ai list-texttoimage-3`** - HyperFlux - List tasks
- **`magnific-pp-cli ai list-texttovideo`** - Get the list of WAN 2.5 Text-to-Video 1080p tasks
- **`magnific-pp-cli ai list-texttovideo-2`** - Get the list of WAN 2.5 Text-to-Video 480p tasks
- **`magnific-pp-cli ai list-texttovideo-3`** - Get the list of WAN 2.5 Text-to-Video 720p tasks
- **`magnific-pp-cli ai list-texttovideo-4`** - Get the list of WAN 2.6 Text-to-Video 1080p tasks
- **`magnific-pp-cli ai list-texttovideo-5`** - Get the list of WAN 2.6 Text-to-Video 720p tasks
- **`magnific-pp-cli ai post-image-to-video-kling-o1-pro`** - Kling O1 Pro - Create video from image
- **`magnific-pp-cli ai post-image-to-video-kling-o1-pro-video-reference`** - Kling O1 Pro - Create video with reference
- **`magnific-pp-cli ai post-image-to-video-kling-o1-std`** - Kling O1 Standard - Create video from image
- **`magnific-pp-cli ai post-image-to-video-kling-o1-std-video-reference`** - Kling O1 Standard - Create video with reference
- **`magnific-pp-cli ai post-image-to-video-kling-v26-pro`** - Kling 2.6 Pro - Create video from text or image
- **`magnific-pp-cli ai post-image-upscaler-precision`** - Upscales an image while adding new visual elements or details.
This endpoint may modify the original image content based on the prompt and inferred context.
- **`magnific-pp-cli ai post-image-upscaler-precision-v2`** - Upscales an image while adding new visual elements or details (V2).
This endpoint may modify the original image content based on the prompt and inferred context.
- **`magnific-pp-cli ai post-reference-to-video-kling-v3-omni-pro`** - Generate AI video using Kling 3 Omni Pro with a reference video for motion and style guidance.

**Video-to-video mode:** This endpoint requires a `video_url` parameter. Reference the video in your prompt using `@Video1`.

**Features:**
- Use a reference video (3-10s) to guide motion and style
- Combine with an image for start frame control
- High-quality pro output

**Use case:** Create videos that follow motion patterns from a reference video while applying your creative prompt.

**Duration:** 3-15 seconds
**Quality:** Pro mode offers highest quality output.

**Tip:** For text-to-video or image-to-video without a reference video, use the `/ai/video/kling-v3-omni-pro` endpoint instead.
- **`magnific-pp-cli ai post-reference-to-video-kling-v3-omni-std`** - Generate AI video using Kling 3 Omni Standard with a reference video for motion and style guidance.

**Video-to-video mode:** This endpoint requires a `video_url` parameter. Reference the video in your prompt using `@Video1`.

**Features:**
- Use a reference video (3-10s) to guide motion and style
- Combine with an image for start frame control
- Faster generation at slightly lower quality

**Use case:** Create videos that follow motion patterns from a reference video while applying your creative prompt.

**Duration:** 3-15 seconds
**Quality:** Standard mode offers faster generation at slightly lower quality.

**Tip:** For text-to-video or image-to-video without a reference video, use the `/ai/video/kling-v3-omni-std` endpoint instead.
- **`magnific-pp-cli ai post-video-kling-advanced-custom-elements`** - **Deprecated.** This endpoint is deprecated. You can pass reference images directly in the `elements` array
of the Kling V3 video generation endpoints (`/ai/video/kling-v3-pro`, `/ai/video/kling-v3-std`)
without needing to pre-create elements.

Create reusable custom elements (characters, animals, items, costumes, scenes, effects) from reference images or videos.
Elements can be referenced in Kling video generation tasks for consistent identity across videos.

**Features:**
- Create elements from multi-angle reference images or a reference video
- Supports character, animal, item, costume, scene, and effect element types
- Optionally bind a voice to the element for video generation with speech
- Elements can be reused across multiple video generation tasks

**Reference types:**
- `image_refer`: Create element from 1 frontal image + up to 3 additional reference images from different angles
- `video_refer`: Create element from a single reference video (3-8 seconds, 1080P, mp4/mov)
- **`magnific-pp-cli ai post-video-kling-v26-motion-control-pro`** - Transfer motion from a reference video to a character image using Kling 2.6 Pro. The model preserves the character's appearance while applying motion patterns from the reference video.
- **`magnific-pp-cli ai post-video-kling-v26-motion-control-std`** - Transfer motion from a reference video to a character image using Kling 2.6 Standard. The model preserves the character's appearance while applying motion patterns from the reference video.
- **`magnific-pp-cli ai post-video-kling-v3-motion-control-pro`** - Transfer motion from a reference video to a character image using Kling 3 Pro. The model preserves the character's appearance while applying motion patterns from the reference video.
- **`magnific-pp-cli ai post-video-kling-v3-motion-control-std`** - Transfer motion from a reference video to a character image using Kling 3 Standard. The model preserves the character's appearance while applying motion patterns from the reference video.
- **`magnific-pp-cli ai post-video-kling-v3-omni-pro`** - Generate AI video using Kling 3 Omni Pro with advanced multi-modal capabilities.

**Features:**
- **Text-to-video**: Generate videos from text prompts
- **Image-to-video**: Use start and/or end frame images to guide generation
- **Multi-shot**: Create videos with up to 6 shots (max 15s total)
- **Element control**: Include reference images for consistent character/style

**Duration:** 3-15 seconds
**Quality:** Pro mode offers highest quality output.

**Note:** For video-to-video generation using a reference video, use the `/ai/reference-to-video/kling-v3-omni-pro` endpoint instead.
- **`magnific-pp-cli ai post-video-kling-v3-omni-std`** - Generate AI video using Kling 3 Omni Standard with advanced multi-modal capabilities.

**Features:**
- **Text-to-video**: Generate videos from text prompts
- **Image-to-video**: Use start and/or end frame images to guide generation
- **Multi-shot**: Create videos with up to 6 shots (max 15s total)
- **Element control**: Include reference images for consistent character/style

**Duration:** 3-15 seconds
**Quality:** Standard mode offers faster generation at slightly lower quality.

**Note:** For video-to-video generation using a reference video, use the `/ai/reference-to-video/kling-v3-omni-std` endpoint instead.
- **`magnific-pp-cli ai post-video-kling-v3-pro`** - Generate AI video using Kling 3 Pro with text-to-video or image-to-video capabilities.

**Features:**
- **Text-to-video**: Generate videos from text prompts
- **Image-to-video**: Use start and/or end frame images to guide generation
- **Multi-shot**: Create videos with up to 6 shots (max 15s total)
- **Element control**: Include reference images for consistent character/style

**Duration:** 3-15 seconds
**Quality:** Pro mode offers highest quality output with longer processing time.
- **`magnific-pp-cli ai post-video-kling-v3-std`** - Generate AI video using Kling 3 Standard with text-to-video or image-to-video capabilities.

**Features:**
- **Text-to-video**: Generate videos from text prompts
- **Image-to-video**: Use start and/or end frame images to guide generation
- **Multi-shot**: Create videos with up to 6 shots (max 15s total)
- **Element control**: Include reference images for consistent character/style

**Duration:** 3-15 seconds
**Quality:** Standard mode offers faster generation at slightly lower quality compared to Pro.
- **`magnific-pp-cli ai post-video-kling4k-i2v`** - Generate AI video in 4K resolution from an image using Kling 4K Image-to-Video.

**Features:**
- **Image-to-video**: Use a reference image to guide 4K video generation
- **End frame control**: Optionally specify an end frame image
- **Motion brush**: Use static and dynamic masks for precise motion control

**Duration:** 3-15 seconds
**Resolution:** 4K output quality.
- **`magnific-pp-cli ai post-video-kling4k-t2v`** - Generate AI video in 4K resolution from a text prompt using Kling 4K Text-to-Video.

**Features:**
- **Text-to-video**: Generate 4K videos from text prompts
- **Camera control**: Optionally specify camera movement type and configuration

**Duration:** 3-15 seconds
**Resolution:** 4K output quality.
- **`magnific-pp-cli ai post-video-upscaler`** - Upscales a video while enhancing visual quality and resolution.
Supports various output resolutions (720p, 1k, 2k, 4k) with optional creativity, sharpening, and grain controls.
- **`magnific-pp-cli ai post-video-upscaler-precision`** - Upscales a video with precision-grade quality enhancement.
Designed for frame-accurate upscaling with fine-grained control over sharpening, grain, and output strength.
Supports output resolutions of 720p, 1k, 2k, or 4k with optional FPS boost.
- **`magnific-pp-cli ai post-video-upscaler-turbo`** - Upscales a video using turbo processing with premium quality enhancement applied automatically.
Turbo mode reduces processing time while maintaining high visual quality.
Supports various output resolutions (720p, 1k, 2k, 4k) with optional creativity, sharpening, and grain controls.
- **`magnific-pp-cli ai remove-image-background`** - This endpoint removes the background from an image provided via a URL. The URLs in the response are temporary and valid for **5 minutes** only.

**Supported formats:** JPG, PNG

**File size limit:** up to 20 MB

**Output resolutions:** Preview (up to 0.25 megapixels), Full resolution (up to 25 megapixels)
- **`magnific-pp-cli ai run-app`** - Triggers the execution of a workflow app. Provide values for all required inputs
defined in the app's definition (see the GET endpoint).

The `inputs` object maps each input's `id` (UUID) to its value:

| Input Type | Accepted Values |
|------------|----------------|
| `creation` | Image URL (`https://...`), base64-encoded image string, or existing creation identifier |
| `text` | Plain text string |
| `number` | Numeric value |
| `select` | Value from allowed options |

**This endpoint is asynchronous.** It returns immediately with `status: "running"` and a
`workflow_run_identifier`. Provide a `webhook` URL to receive notifications when the execution
completes. The webhook receives two events: `initialized` (run started) and `finished`/`failed` (run completed).

The webhook payload for a completed run includes a `result` object containing `images`, `videos`,
and `audios` arrays with download URLs for the generated assets.

<Note>
  **Enterprise exclusive.** This endpoint is only available to Enterprise customers. [Contact sales](https://www.freepik.com/api#contact) for access.
</Note>

### icons

Manage icons

- **`magnific-pp-cli icons get-detail-by-id`** - Get detailed information about a specific icon identified by its unique ID.
- **`magnific-pp-cli icons search`** - Get a list of icons based on the provided parameters and ordering criteria.

### music

Manage music

- **`magnific-pp-cli music get-detail`** - Retrieve full details for a music item including artist biography, genre and mood metadata, popularity score, and download statistics.
- **`magnific-pp-cli music search`** - Search the Freepik Music catalog. Filter by genre, mood, artist, premium status, and creation date range. Returns paginated results sorted by popularity by default.

### resources

Manage resources

- **`magnific-pp-cli resources get-detail-by-id`** - Retrieve the detailed information of a specific resource by its ID. This endpoint supports multiple resource types including PSD, vector, photo, and AI-generated content.
- **`magnific-pp-cli resources search`** - Retrieve a list of resources based on various filter criteria such as orientation, content type, license, and more.

### sound-effects

Manage sound effects

- **`magnific-pp-cli sound-effects get-detail`** - Retrieve full details for a sound effect including category hierarchy, tags, duration, popularity, and download statistics.
- **`magnific-pp-cli sound-effects search`** - Search the Freepik Sound Effects catalog. Filter by category, duration range, premium status, and creation date. Returns paginated results sorted by popularity by default.

### videos

Manage videos

- **`magnific-pp-cli videos get`** - Get detailed video information by ID
- **`magnific-pp-cli videos list`** - Search and filter videos by specified order


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
magnific-pp-cli ai list

# JSON for scripting and agents
magnific-pp-cli ai list --json

# Filter to specific fields
magnific-pp-cli ai list --json --select id,name,status

# Dry run — show the request without sending
magnific-pp-cli ai list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
magnific-pp-cli ai list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
magnific-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/magnific-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FREEPIK_API_KEY` | per_call | Yes (or `MAGNIFIC_API_KEY`) | Preferred env var name (matches the official Freepik docs + MCP server). |
| `MAGNIFIC_API_KEY` | per_call | Yes (or `FREEPIK_API_KEY`) | Rebrand-aware alias. Same key works against both `api.freepik.com` and `api.magnific.com`. `MAGNIFIC_API_KEY` wins when both are set. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `magnific-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MAGNIFIC_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized with "No API key provided"** — Set `FREEPIK_API_KEY` in your environment or run `magnific-pp-cli auth set-token <key>`. Same key works for both magnific.com and freepik.com hostnames.
- **429 throttled during a bake-off run** — Magnific enforces 10 req/s average, 50 req/s peak. Pass fewer slugs to `--models` on `compare` or stagger your `task wait` loops.
- **task stays in IN_PROGRESS for hours** — Run `magnific-pp-cli tasks stale --since 30m` to find leaked tasks, then `magnific-pp-cli tasks reconcile` to re-poll terminal state from the real GET endpoints.
- **output URL returns 404 days later** — Magnific signed URLs expire. Download completed outputs to disk while the task is fresh (e.g. `curl -L "$URL" -o out.png`), then register the local copy in the gallery so `gallery list` can find it later.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**freepik-mcp (official)**](https://github.com/freepik-company/freepik-mcp) — Python (67 stars)
- [**freepik-cli**](https://github.com/Balneario-de-Cofrentes/freepik-cli) — TypeScript (3 stars)
- [**freepik-mcp (mcerqua)**](https://github.com/mcerqua/freepik-mcp) — TypeScript (2 stars)
- [**freepik-mcp-server (grafikogr)**](https://github.com/grafikogr/freepik-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
