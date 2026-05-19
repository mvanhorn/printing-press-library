---
name: pp-magnific
description: "Every Magnific feature, plus a local archive of every prompt, task, and credit — so your generation history... Trigger phrases: `use magnific`, `run magnific-pp-cli`, `upscale this image with magnific`, `generate a magnific mystic image`, `compare image models on magnific`, `what did I spend on magnific this month`, `find the magnific prompt I used last week`."
author: "Nuno Duarte"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - magnific-pp-cli
---

# Magnific — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `magnific-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install magnific --cli-only
   ```
2. Verify: `magnific-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Magnific exposes 80+ AI generation and editing models across image, video, audio, and stock content. This CLI mirrors all of them with the same `x-freepik-api-key` you already have, then adds what the platform cannot: SQLite-backed prompt history, model bake-off (`compare`), unified task polling (`task wait`), a credit-cost ledger, and a local asset gallery. Built as a single Go binary with offline search, agent-native JSON output, and an MCP code-orchestration surface so agents see search+execute instead of 388 raw tools.

## When to Use This CLI

Reach for this CLI when an agent or pipeline needs to drive Magnific's full AI catalog non-interactively: generating images across multiple models for comparison, polling long-running video tasks without hand-rolled retry loops, querying historical prompt usage to recover a known-good seed, or budgeting credits against the curated per-model cost table. It is the right surface when a Mac-native single-binary install matters more than a pretty TUI, and when MCP code-orchestration matters more than 388 raw tool entries in the agent catalog.

## Unique Capabilities

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

## Command Reference

**ai** — Manage ai

- `magnific-pp-cli ai apply-style-transfer-to-image` — Style Transfer - Transform image style
- `magnific-pp-cli ai create` — This endpoint allows you to generate or edit images using the Gemini 2.5 Flash model. You can provide a prompt for...
- `magnific-pp-cli ai create-audio-isolation-task` — Isolate and extract specific sounds from audio or video files using SAM Audio AI technology. Describe the sound you...
- `magnific-pp-cli ai create-beta` — (Beta, synchronous) Reimagine Flux is a new AI model that allows you to generate images from text prompts.
- `magnific-pp-cli ai create-change-camera-task` — Transform an image by changing the camera angle using AI. Adjust horizontal rotation (0-360 degrees), vertical tilt...
- `magnific-pp-cli ai create-ideogram-image-edit` — Edit an image using Ideogram AI's inpainting capabilities. Provide an image and a mask to specify the areas to edit,...
- `magnific-pp-cli ai create-image-edit-seedream-v4-5` — Edit images using ByteDance's Seedream 4.5 model with text guidance. **Key Features:** - Preserves subject details,...
- `magnific-pp-cli ai create-image-edit-seedream-v5-lite` — Edit images using ByteDance's Seedream V5 Lite model with text guidance. **Key Features:** - Preserves subject...
- `magnific-pp-cli ai create-image-flux-2-klein` — Generate images with sub-second speed using FLUX.2 [klein], the fastest model in the FLUX.2 family by Black Forest...
- `magnific-pp-cli ai create-image-flux-2-pro` — Create professional-grade images using FLUX.2 [pro], the next generation of Black Forest Labs' image models. **Key...
- `magnific-pp-cli ai create-image-flux-2-turbo` — Create high-quality images quickly using FLUX.2 [turbo], the speed-optimized version of Flux 2. **Key Features:** -...
- `magnific-pp-cli ai create-image-from-text-classic` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-flux` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-flux-kontext-pro` — Generate images using FLUX Kontext Pro, an advanced text-to-image model with optional image input support. This...
- `magnific-pp-cli ai create-image-from-text-flux-pro-v1-1` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-imagen3` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-imagen4-fast` — Convert descriptive text input into images using AI with Imagen 4 Fast model. Optimized for speed and...
- `magnific-pp-cli ai create-image-from-text-imagen4-ultra` — Convert descriptive text input into images using AI with Imagen 4 Ultra model. Highest quality for professional needs.
- `magnific-pp-cli ai create-image-from-text-nano-banana-pro` — Generate high-quality images from text descriptions using Google's Nano Banana Pro model (Gemini 3). **Key...
- `magnific-pp-cli ai create-image-from-text-nano-banana-pro-flash` — Generate images from text descriptions using Google's Nano Banana Pro Flash model (Gemini 3.1 Flash), a faster...
- `magnific-pp-cli ai create-image-from-text-runway` — Generate high-quality images from text descriptions using RunWay's Gen4 Image model. **Key Features:** -...
- `magnific-pp-cli ai create-image-from-text-seedream` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-seedream-v4` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-seedream-v4-5` — Generate high-quality images from text descriptions using ByteDance's Seedream 4.5 model. **Key Features:** -...
- `magnific-pp-cli ai create-image-from-text-seedream-v4-edit` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-from-text-seedream-v5-lite` — Generate high-quality images from text descriptions using ByteDance's Seedream V5 Lite model. **Key Features:** -...
- `magnific-pp-cli ai create-image-from-text-z-image` — Generate high-quality images from text descriptions using the Z-Image turbo model. **Key Features:** - Superior...
- `magnific-pp-cli ai create-image-mystic` — Convert descriptive text input into images using AI. This endpoint accepts a variety of parameters to customize the...
- `magnific-pp-cli ai create-image-to-prompt-task` — Generate descriptive prompts from input images using AI analysis
- `magnific-pp-cli ai create-imageexpand` — This endpoint allows you to expand an image using the AI Flux Pro model. The image will be expanded based on the...
- `magnific-pp-cli ai create-imageexpand-2` — This endpoint allows you to expand an image using the Ideogram AI model. The image will be expanded based on the...
- `magnific-pp-cli ai create-imageexpand-3` — This endpoint allows you to expand an image using the Seedream V4.5 AI model. The image will be expanded based on...
- `magnific-pp-cli ai create-imagerelight` — Relight an image using AI. This endpoint accepts a variety of parameters to customize the generated images.
- `magnific-pp-cli ai create-imagetovideo` — Create a video from an image using the Kling v2 model
- `magnific-pp-cli ai create-imageupscaler` — This asynchronous endpoint enables image upscaling using advanced AI algorithms. Upon submission, it returns a...
- `magnific-pp-cli ai create-improve-prompt-task` — Enhance user prompts for AI image or video generation using advanced AI models. - **Image prompts**: Improve a...
- `magnific-pp-cli ai create-latent-sync-lip-sync` — Generate lip-synced video by synchronizing a video with an audio file using AI Latent-Sync technology
- `magnific-pp-cli ai create-loras` — Create you own custom character using LoRAs training For now you can check the status of the training calling...
- `magnific-pp-cli ai create-loras-2` — Create you own custom style using LoRAs training For now you can check the status of the training calling...
- `magnific-pp-cli ai create-music-generation-task` — Generate original music tracks from text descriptions using AI. Create high-quality music compositions based on your...
- `magnific-pp-cli ai create-pixverse-v5-5-video` — Generate a video from an image using the PixVerse-V5.5 model. Compared to PixVerse-V5, version 5.5 adds: - 10-second...
- `magnific-pp-cli ai create-pixverse-v5-transition-video` — Generate a video transition between two images using the PixVerse-V5 model.
- `magnific-pp-cli ai create-pixverse-v5-video` — Generate a video using the PixVerse-V5 model. Resolution is specified in the request body.
- `magnific-pp-cli ai create-pixverse-v6-video` — Generate a video from an image using the PixVerse-V6 model. Compared to PixVerse-V5.5, version 6 adds: - Flexible...
- `magnific-pp-cli ai create-skinenhancer` — Enhance skin in images using AI with the Creative mode. This mode provides more artistic and stylized enhancements.
- `magnific-pp-cli ai create-skinenhancer-2` — Enhance skin in images using AI with the Faithful mode. This mode preserves the original appearance while improving...
- `magnific-pp-cli ai create-skinenhancer-3` — Enhance skin in images using AI with the Flexible mode. This mode allows you to choose the optimization target for...
- `magnific-pp-cli ai create-sound-effects-task` — Create realistic sound effects from text descriptions using AI. Generate high-quality audio sound effects based on...
- `magnific-pp-cli ai create-texttoimage` — This endpoint allows you to generate an image using the HyperFlux model, the fastest Flux model available
- `magnific-pp-cli ai create-veed-fabric-1-0-fast-lip-sync` — Generate a realistic talking video by combining a static portrait image with an audio file using Veed Fabric 1.0...
- `magnific-pp-cli ai create-veed-fabric-1-0-lip-sync` — Generate a realistic talking video by combining a static portrait image with an audio file using Veed Fabric 1.0....
- `magnific-pp-cli ai create-video-from-image-kling-elements-pro` — Generate a video from an image using the Kling Elements Pro model.
- `magnific-pp-cli ai create-video-from-image-kling-elements-std` — Generate a video from an image using the Kling Elements Std model.
- `magnific-pp-cli ai create-video-from-image-kling-v2-1-master` — Generate a video from an image using the Kling 2.1 Master model.
- `magnific-pp-cli ai create-video-kling-1-6-pro` — Generate a video from an image using the Kling 1.6 Pro model.
- `magnific-pp-cli ai create-video-kling-1-6-std` — Generate a video from an image using the Kling 1.6 Std model.
- `magnific-pp-cli ai create-video-kling-2-1-pro` — Generate a video from an image using the Kling 2.1 Pro model.
- `magnific-pp-cli ai create-video-kling-2-1-std` — Generate a video from an image using the Kling 2.1 Std model.
- `magnific-pp-cli ai create-video-kling-2-5-pro` — Generate a video from an image using the Kling 2.5 Pro model.
- `magnific-pp-cli ai create-video-ltx-2-fast-i2v` — Generate a video from an image using the LTX Video 2.0 Fast model. **Features:** - Fast video generation with...
- `magnific-pp-cli ai create-video-ltx-2-fast-t2v` — Generate a video from text prompt using the LTX Video 2.0 Fast model. **Features:** - Fast video generation with...
- `magnific-pp-cli ai create-video-ltx-2-pro-i2v` — Generate a video from an image using the LTX Video 2.0 Pro model. **Features:** - High-quality video generation with...
- `magnific-pp-cli ai create-video-ltx-2-pro-t2v` — Generate a video from text prompt using the LTX Video 2.0 Pro model. **Features:** - High-quality video generation...
- `magnific-pp-cli ai create-video-minimax-hailuo-02-1080p` — Generate a video from text or image using the MiniMax Hailuo-02 1080p model.
- `magnific-pp-cli ai create-video-minimax-hailuo-02-768p` — Generate a video from text or image using the MiniMax Hailuo-02 768p model.
- `magnific-pp-cli ai create-video-minimax-hailuo-23-1080p` — Generate a video from text or image using the MiniMax Hailuo 2.3 1080p model.
- `magnific-pp-cli ai create-video-minimax-hailuo-23-1080p-fast` — Generate a video from text or image using the MiniMax Hailuo 2.3 1080p model with fast prompt optimization.
- `magnific-pp-cli ai create-video-minimax-hailuo-23-768p` — Generate a video from text or image using the MiniMax Hailuo 2.3 768p model.
- `magnific-pp-cli ai create-video-minimax-hailuo-23-768p-fast` — Generate a video from text or image using the MiniMax Hailuo 2.3 768p model with fast prompt optimization.
- `magnific-pp-cli ai create-video-minimax-live-i2v` — Generate a video from an image using MiniMax Video-01-Live model (Live Illustrations). **Features:** - Supports...
- `magnific-pp-cli ai create-video-omni-human-1-5` — Generate animated video of a human figure driven by audio using OmniHuman 1.5 model (ByteDance). Supports natural...
- `magnific-pp-cli ai create-video-runway-45-i2v` — Generate high-quality videos from images using RunWay Gen 4.5 model. **Features:** - Transform static images into...
- `magnific-pp-cli ai create-video-runway-45-t2v` — Generate high-quality videos from text descriptions using RunWay Gen 4.5 model. **Features:** - State-of-the-art...
- `magnific-pp-cli ai create-video-runway-act-two` — Generate a character performance video using RunWay Act Two model. Transfer facial expressions and body movements...
- `magnific-pp-cli ai create-video-runway-gen4-turbo` — Generate a video from an image using RunWay Gen4 Turbo model.
- `magnific-pp-cli ai create-video-seedance-1-5-pro-1080p` — Generate 1080p video with synchronized audio using Seedance 1.5 Pro. Supports text-to-video and image-to-video with...
- `magnific-pp-cli ai create-video-seedance-1-5-pro-480p` — Generate 480p video with synchronized audio using Seedance 1.5 Pro. Supports text-to-video and image-to-video with...
- `magnific-pp-cli ai create-video-seedance-1-5-pro-720p` — Generate 720p video with synchronized audio using Seedance 1.5 Pro. Supports text-to-video and image-to-video with...
- `magnific-pp-cli ai create-video-seedance-lite-1080p` — Generate a video from image using the Seedance Lite 1080p model.
- `magnific-pp-cli ai create-video-seedance-lite-480p` — Generate a video from image using the Seedance Lite 480p model.
- `magnific-pp-cli ai create-video-seedance-lite-720p` — Generate a video from image using the Seedance Lite 720p model.
- `magnific-pp-cli ai create-video-seedance-pro-1080p` — Generate a video from image using the Seedance Pro 1080p model.
- `magnific-pp-cli ai create-video-seedance-pro-480p` — Generate a video from image using the Seedance Pro 480p model.
- `magnific-pp-cli ai create-video-seedance-pro-720p` — Generate a video from image using the Seedance Pro 720p model.
- `magnific-pp-cli ai create-video-veo-31-i2v` — Generate a video from an image using Google Veo 3.1 model. Supports multiple resolutions (720p, 1080p, 4K) and...
- `magnific-pp-cli ai create-video-veo-31-i2v-fast` — Generate a video from an image using Google Veo 3.1 Fast model. Faster generation at a lower cost.
- `magnific-pp-cli ai create-video-veo-31-ref2v` — Generate a video with character or object consistency using reference images. Maintains visual identity across...
- `magnific-pp-cli ai create-video-veo-31-t2v` — Generate a video from text prompt using Google Veo 3.1 model. Supports multiple resolutions (720p, 1080p, 4K) and...
- `magnific-pp-cli ai create-video-veo-31-t2v-fast` — Generate a video from text prompt using Google Veo 3.1 Fast model. Faster generation at a lower cost.
- `magnific-pp-cli ai create-video-vfx` — Apply professional visual effects and filters to your videos using the VFX API. Transform any video with cinematic...
- `magnific-pp-cli ai create-video-wan-25-i2v-1080p` — Generate a 1080p video from image using the WAN 2.5 model.
- `magnific-pp-cli ai create-video-wan-25-i2v-480p` — Generate a 480p video from image using the WAN 2.5 model.
- `magnific-pp-cli ai create-video-wan-25-i2v-720p` — Generate a 720p video from image using the WAN 2.5 model.
- `magnific-pp-cli ai create-video-wan-25-t2v-1080p` — Generate a 1080p video from text prompt using the WAN 2.5 model.
- `magnific-pp-cli ai create-video-wan-25-t2v-480p` — Generate a 480p video from text prompt using the WAN 2.5 model.
- `magnific-pp-cli ai create-video-wan-25-t2v-720p` — Generate a 720p video from text prompt using the WAN 2.5 model.
- `magnific-pp-cli ai create-video-wan-27-edit` — Edit an existing video using WAN 2.7. Transform video content with text instructions, apply visual references, or...
- `magnific-pp-cli ai create-video-wan-27-i2v` — Generate a video from an image or extend an existing video using WAN 2.7. **Three generation modes:** - **First...
- `magnific-pp-cli ai create-video-wan-27-r2v` — Generate a video featuring characters from reference images or videos using WAN 2.7. Maintains visual identity of...
- `magnific-pp-cli ai create-video-wan-27-t2v` — Generate a video from a text prompt using WAN 2.7. Supports configurable aspect ratios (16:9, 9:16, 1:1, 4:3, 3:4),...
- `magnific-pp-cli ai create-video-wan-v22-480p` — Generate a video from image using the WAN 2.2 480p model.
- `magnific-pp-cli ai create-video-wan-v22-580p` — Generate a video from image using the WAN 2.2 580p model.
- `magnific-pp-cli ai create-video-wan-v22-720p` — Generate a video from image using the WAN 2.2 720p model.
- `magnific-pp-cli ai create-video-wan-v26-i2v-1080p` — Generate a 1080p video from image using the WAN 2.6 model.
- `magnific-pp-cli ai create-video-wan-v26-i2v-720p` — Generate a 720p video from image using the WAN 2.6 model.
- `magnific-pp-cli ai create-video-wan-v26-t2v-1080p` — Generate a 1080p video from text prompt using the WAN 2.6 model.
- `magnific-pp-cli ai create-video-wan-v26-t2v-720p` — Generate a 720p video from text prompt using the WAN 2.6 model.
- `magnific-pp-cli ai create-voiceover-task` — Generate natural-sounding speech from text using ElevenLabs AI voices. Create professional voiceovers for videos,...
- `magnific-pp-cli ai detect-image` — Accepts an image file as input and analyzes it to determine the probability that the image was generated by...
- `magnific-pp-cli ai download-text-to-icon-render-format` — Download the generated AI icon in the specified format png or svg.
- `magnific-pp-cli ai generate-text-to-icon` — Create stunning icons in different styles and formats (png, svg) from text prompts using our advanced AI models.
- `magnific-pp-cli ai generate-text-to-icon-preview` — Create stunning previews icons in different styles and formats (png, svg) from text prompts using our advanced AI...
- `magnific-pp-cli ai get` — Gemini 2.5 Flash - Get task status
- `magnific-pp-cli ai get-all-audio-isolation-tasks` — Get the status of all audio isolation tasks
- `magnific-pp-cli ai get-all-change-camera-tasks` — Retrieve the status of all Change Camera tasks for the authenticated user. Returns a list of tasks with their...
- `magnific-pp-cli ai get-all-flux-2-klein-tasks` — Retrieve the status of all FLUX.2 [klein] text-to-image generation tasks for the authenticated user.
- `magnific-pp-cli ai get-all-flux-2-pro-tasks` — Retrieve the status of all Flux 2 Pro text-to-image generation tasks for the authenticated user.
- `magnific-pp-cli ai get-all-flux-2-turbo-tasks` — Retrieve the status of all Flux 2 Turbo text-to-image generation tasks for the authenticated user.
- `magnific-pp-cli ai get-all-flux-kontext-pro-tasks` — Retrieve the status of all Flux Kontext Pro text-to-image generation tasks for the authenticated user.
- `magnific-pp-cli ai get-all-ideogram-image-edit-tasks` — Get the status of all Ideogram Image Edit tasks
- `magnific-pp-cli ai get-all-image-to-prompt-tasks` — Get the status of all image-to-prompt tasks
- `magnific-pp-cli ai get-all-imagen3-tasks` — Get the status of all Imagen3 tasks
- `magnific-pp-cli ai get-all-imagen4-fast-tasks` — Get the status of all Imagen4 Fast tasks
- `magnific-pp-cli ai get-all-imagen4-ultra-tasks` — Get the status of all Imagen4 Ultra tasks
- `magnific-pp-cli ai get-all-improve-prompt-tasks` — Get the status of all improve-prompt tasks
- `magnific-pp-cli ai get-all-kling-elements-std-tasks` — Get the list of the kling-elements-std tasks
- `magnific-pp-cli ai get-all-kling-pro-tasks` — Get the list of the kling-pro tasks
- `magnific-pp-cli ai get-all-latent-sync-tasks` — Get the status of all Latent-Sync lip-sync tasks
- `magnific-pp-cli ai get-all-ltx-2-fast-i2v-tasks` — LTX Video 2.0 Fast I2V - List tasks
- `magnific-pp-cli ai get-all-ltx-2-fast-t2v-tasks` — LTX Video 2.0 Fast T2V - List tasks
- `magnific-pp-cli ai get-all-ltx-2-pro-i2v-tasks` — LTX Video 2.0 Pro I2V - List tasks
- `magnific-pp-cli ai get-all-ltx-2-pro-t2v-tasks` — LTX Video 2.0 Pro T2V - List tasks
- `magnific-pp-cli ai get-all-minimax-live-i2v-tasks` — MiniMax Video 01 Live - List tasks
- `magnific-pp-cli ai get-all-music-generation-tasks` — Get the status of all music-generation tasks
- `magnific-pp-cli ai get-all-nano-banana-pro-flash-tasks` — Get the status of all Nano Banana Pro Flash image generation tasks
- `magnific-pp-cli ai get-all-nano-banana-pro-tasks` — Get the status of all Nano Banana Pro image generation tasks
- `magnific-pp-cli ai get-all-omni-human-1-5-tasks` — OmniHuman 1.5 - List tasks
- `magnific-pp-cli ai get-all-runway-45-i2v-tasks` — Retrieve the status of all RunWay Gen 4.5 image-to-video generation tasks for the authenticated user.
- `magnific-pp-cli ai get-all-runway-45-t2v-tasks` — Retrieve the status of all RunWay Gen 4.5 text-to-video generation tasks for the authenticated user.
- `magnific-pp-cli ai get-all-runway-act-two-tasks` — RunWay Act Two - List tasks
- `magnific-pp-cli ai get-all-runway-gen4-turbo-tasks` — RunWay Gen4 Turbo - List tasks
- `magnific-pp-cli ai get-all-runway-text-to-image-tasks` — Get the status of all RunWay text-to-image generation tasks
- `magnific-pp-cli ai get-all-seedance-1-5-pro-1080p-tasks` — Seedance 1.5 Pro 1080p - List tasks
- `magnific-pp-cli ai get-all-seedance-1-5-pro-480p-tasks` — Seedance 1.5 Pro 480p - List tasks
- `magnific-pp-cli ai get-all-seedance-1-5-pro-720p-tasks` — Seedance 1.5 Pro 720p - List tasks
- `magnific-pp-cli ai get-all-seedream-tasks` — Get the status of all Seedream tasks
- `magnific-pp-cli ai get-all-seedream-v4-5-edit-tasks` — Get the status of all Seedream 4.5 image editing tasks
- `magnific-pp-cli ai get-all-seedream-v4-5-tasks` — Get the status of all Seedream 4.5 image generation tasks
- `magnific-pp-cli ai get-all-seedream-v4-edit-tasks` — Get the status of all Seedream v4 edit tasks
- `magnific-pp-cli ai get-all-seedream-v4-tasks` — Get the status of all Seedream v4 tasks
- `magnific-pp-cli ai get-all-seedream-v5-lite-edit-tasks` — Get the status of all Seedream V5 Lite image editing tasks
- `magnific-pp-cli ai get-all-seedream-v5-lite-tasks` — Get the status of all Seedream V5 Lite image generation tasks
- `magnific-pp-cli ai get-all-sound-effects-tasks` — Get the status of all sound-effects tasks
- `magnific-pp-cli ai get-all-style-transfer-tasks` — Get the status of all Style Transfer tasks
- `magnific-pp-cli ai get-all-veed-fabric-1-0-fast-tasks` — Retrieve all Veed Fabric 1.0 Fast lip-sync tasks for the authenticated user.
- `magnific-pp-cli ai get-all-veed-fabric-1-0-tasks` — Retrieve all Veed Fabric 1.0 lip-sync tasks for the authenticated user.
- `magnific-pp-cli ai get-all-veo-31-i2v-fast-tasks` — Get all Veo 3.1 I2V Fast tasks
- `magnific-pp-cli ai get-all-veo-31-i2v-tasks` — Get all Veo 3.1 I2V tasks
- `magnific-pp-cli ai get-all-veo-31-ref2v-tasks` — Retrieve a list of all reference-to-video generation tasks for Veo 3.1.
- `magnific-pp-cli ai get-all-veo-31-t2v-fast-tasks` — Get all Veo 3.1 T2V Fast tasks
- `magnific-pp-cli ai get-all-veo-31-t2v-tasks` — Get all Veo 3.1 T2V tasks
- `magnific-pp-cli ai get-all-vfx-tasks` — Retrieve the status of all VFX video effect tasks for the authenticated user.
- `magnific-pp-cli ai get-all-voiceover-tasks` — Get the status of all voiceover tasks
- `magnific-pp-cli ai get-all-wan-27-i2v-tasks` — Retrieve the list of all WAN 2.7 image-to-video tasks for the authenticated user.
- `magnific-pp-cli ai get-all-wan-27-r2v-tasks` — Retrieve the list of all WAN 2.7 reference-to-video tasks for the authenticated user.
- `magnific-pp-cli ai get-all-wan-27-t2v-tasks` — Retrieve the list of all WAN 2.7 text-to-video tasks for the authenticated user.
- `magnific-pp-cli ai get-all-wan-27-video-edit-tasks` — Retrieve the list of all WAN 2.7 video edit tasks for the authenticated user.
- `magnific-pp-cli ai get-all-z-image-tasks` — Get the status of all Z-Image image generation tasks
- `magnific-pp-cli ai get-app` — Returns the full definition of a workflow app, including its dynamic inputs and cost. **Inputs** define what values...
- `magnific-pp-cli ai get-app-run` — Retrieve the current status and results of a workflow app execution run. Use this endpoint to **poll** the status of...
- `magnific-pp-cli ai get-audio-isolation-task-status` — Get the status of one audio isolation task
- `magnific-pp-cli ai get-available-loras-list` — Get available loras list
- `magnific-pp-cli ai get-change-camera-task` — Retrieve the status and result of a specific Change Camera task by its task ID. When the task status is `completed`,...
- `magnific-pp-cli ai get-flux-2-klein-task` — Retrieve the status and results of a specific FLUX.2 [klein] generation task.
- `magnific-pp-cli ai get-flux-2-pro-task` — Retrieve the status and results of a specific Flux 2 Pro generation task.
- `magnific-pp-cli ai get-flux-2-turbo-task` — Retrieve the status and results of a specific Flux 2 Turbo generation task.
- `magnific-pp-cli ai get-flux-kontext-pro-task` — Retrieve the status and results of a specific Flux Kontext Pro text-to-image generation task.
- `magnific-pp-cli ai get-ideogram-image-edit-task-status` — Get the status of a specific Ideogram Image Edit task
- `magnific-pp-cli ai get-image-to-prompt-task-status` — Get the status of one image-to-prompt task
- `magnific-pp-cli ai get-image-to-video-kling-v26-task` — Kling 2.6 Pro - Get task status
- `magnific-pp-cli ai get-image-to-video-kling-v26-tasks` — Kling 2.6 Pro - List tasks
- `magnific-pp-cli ai get-image-upscaler-precision-task` — Returns the current status and output URL of a specific precision upscaler task. The output URL is included only if...
- `magnific-pp-cli ai get-image-upscaler-precision-tasks` — Returns a list of all precision upscaler tasks. Each task includes its ID, current status, and output URL if completed.
- `magnific-pp-cli ai get-image-upscaler-precision-v2-task` — Returns the current status and output URL of a specific precision upscaler V2 task. The output URL is included only...
- `magnific-pp-cli ai get-image-upscaler-precision-v2-tasks` — Returns a list of all precision upscaler V2 tasks. Each task includes its ID, current status, and output URL if...
- `magnific-pp-cli ai get-imageexpand` — Get the status of one image expand task
- `magnific-pp-cli ai get-imageexpand-2` — Get the status of one image expand ideogram task
- `magnific-pp-cli ai get-imageexpand-3` — Get the status of one image expand seedream v4.5 task
- `magnific-pp-cli ai get-imagen3-task-status` — Get the status of the Imagen3 task
- `magnific-pp-cli ai get-imagen4-fast-task-status` — Get the status of the Imagen4 Fast task
- `magnific-pp-cli ai get-imagen4-ultra-task-status` — Get the status of the Imagen4 Ultra task
- `magnific-pp-cli ai get-imagerelight` — Get the status of the relight task
- `magnific-pp-cli ai get-imagetovideo` — Get the status of the kling-elements task
- `magnific-pp-cli ai get-imagetovideo-10` — Get the status of the MiniMax Hailuo 2.3 768p Fast task
- `magnific-pp-cli ai get-imagetovideo-11` — Get the status of the MiniMax Hailuo 2.3 768p task
- `magnific-pp-cli ai get-imagetovideo-12` — PixVerse V5.5 - Get task status
- `magnific-pp-cli ai get-imagetovideo-13` — PixVerse V5 Transition - Get task status
- `magnific-pp-cli ai get-imagetovideo-14` — PixVerse V5 - Get task status
- `magnific-pp-cli ai get-imagetovideo-15` — PixVerse V6 - Get task status
- `magnific-pp-cli ai get-imagetovideo-16` — Get the status of the Seedance Lite 1080p task
- `magnific-pp-cli ai get-imagetovideo-17` — Get the status of the Seedance Lite 480p task
- `magnific-pp-cli ai get-imagetovideo-18` — Get the status of the Seedance Lite 720p task
- `magnific-pp-cli ai get-imagetovideo-19` — Get the status of the Seedance Pro 1080p task
- `magnific-pp-cli ai get-imagetovideo-2` — Get the status of the kling-v2-1-master task
- `magnific-pp-cli ai get-imagetovideo-20` — Get the status of the Seedance Pro 480p task
- `magnific-pp-cli ai get-imagetovideo-21` — Get the status of the Seedance Pro 720p task
- `magnific-pp-cli ai get-imagetovideo-22` — Get the status of a WAN 2.5 Image-to-Video 1080p task
- `magnific-pp-cli ai get-imagetovideo-23` — Get the status of a WAN 2.5 Image-to-Video 480p task
- `magnific-pp-cli ai get-imagetovideo-24` — Get the status of a WAN 2.5 Image-to-Video 720p task
- `magnific-pp-cli ai get-imagetovideo-25` — Get the status of the WAN 2.2 480p task
- `magnific-pp-cli ai get-imagetovideo-26` — Get the status of the WAN 2.2 580p task
- `magnific-pp-cli ai get-imagetovideo-27` — Get the status of the WAN 2.2 720p task
- `magnific-pp-cli ai get-imagetovideo-28` — Get the status of a WAN 2.6 Image-to-Video 1080p task
- `magnific-pp-cli ai get-imagetovideo-29` — Get the status of a WAN 2.6 Image-to-Video 720p task
- `magnific-pp-cli ai get-imagetovideo-3` — Get the status of the kling-v2-5-pro task
- `magnific-pp-cli ai get-imagetovideo-4` — Get the status of the kling-v2 task
- `magnific-pp-cli ai get-imagetovideo-5` — Get the status of the kling task
- `magnific-pp-cli ai get-imagetovideo-6` — Get the status of the MiniMax Hailuo-02 1080p task
- `magnific-pp-cli ai get-imagetovideo-7` — Get the status of the MiniMax Hailuo-02 768p task
- `magnific-pp-cli ai get-imagetovideo-8` — Get the status of the MiniMax Hailuo 2.3 1080p Fast task
- `magnific-pp-cli ai get-imagetovideo-9` — Get the status of the MiniMax Hailuo 2.3 1080p task
- `magnific-pp-cli ai get-imageupscaler` — Get the status of the upscaling task
- `magnific-pp-cli ai get-improve-prompt-task-status` — Get the status of one improve-prompt task
- `magnific-pp-cli ai get-kling-2-1-task-status` — Get the status of the kling-v2-1 task
- `magnific-pp-cli ai get-kling-o1-task` — Kling O1 - Get task status
- `magnific-pp-cli ai get-latent-sync-task-status` — Get the status of one Latent-Sync lip-sync task
- `magnific-pp-cli ai get-ltx-2-fast-i2v-task` — LTX Video 2.0 Fast I2V - Get task status
- `magnific-pp-cli ai get-ltx-2-fast-t2v-task` — LTX Video 2.0 Fast T2V - Get task status
- `magnific-pp-cli ai get-ltx-2-pro-i2v-task` — LTX Video 2.0 Pro I2V - Get task status
- `magnific-pp-cli ai get-ltx-2-pro-t2v-task` — LTX Video 2.0 Pro T2V - Get task status
- `magnific-pp-cli ai get-minimax-live-i2v-task` — MiniMax Video 01 Live - Get task status
- `magnific-pp-cli ai get-music-generation-task-status` — Get the status of one music-generation task
- `magnific-pp-cli ai get-mystic-task-status` — Get the status of the Mystic task
- `magnific-pp-cli ai get-nano-banana-pro-flash-task-status` — Get the status of a specific Nano Banana Pro Flash image generation task
- `magnific-pp-cli ai get-nano-banana-pro-task-status` — Get the status of a specific Nano Banana Pro image generation task
- `magnific-pp-cli ai get-omni-human-1-5-task` — OmniHuman 1.5 - Get task status
- `magnific-pp-cli ai get-reference-to-video-kling-v3-omni-task` — Retrieve the status and result of a specific Kling 3 Omni reference-to-video task (Pro or Standard) by its task ID.
- `magnific-pp-cli ai get-reference-to-video-kling-v3-omni-tasks` — Retrieve the list of all Kling 3 Omni reference-to-video tasks (both Pro and Standard) for the authenticated user.
- `magnific-pp-cli ai get-runway-45-i2v-task` — Retrieve the status and result of a specific RunWay Gen 4.5 image-to-video task by its task ID.
- `magnific-pp-cli ai get-runway-45-t2v-task` — Retrieve the status and result of a specific RunWay Gen 4.5 text-to-video task by its task ID.
- `magnific-pp-cli ai get-runway-act-two-task` — RunWay Act Two - Get task status
- `magnific-pp-cli ai get-runway-gen4-turbo-task` — RunWay Gen4 Turbo - Get task status
- `magnific-pp-cli ai get-runway-text-to-image-task-status` — Get the status and result of a specific RunWay text-to-image task
- `magnific-pp-cli ai get-seedance-1-5-pro-1080p-task` — Seedance 1.5 Pro 1080p - Get task status
- `magnific-pp-cli ai get-seedance-1-5-pro-480p-task` — Seedance 1.5 Pro 480p - Get task status
- `magnific-pp-cli ai get-seedance-1-5-pro-720p-task` — Seedance 1.5 Pro 720p - Get task status
- `magnific-pp-cli ai get-seedream-task-status` — Get the status of the Seedream task
- `magnific-pp-cli ai get-seedream-v4-5-edit-task-status` — Get the status of a specific Seedream 4.5 image editing task
- `magnific-pp-cli ai get-seedream-v4-5-task-status` — Get the status of a specific Seedream 4.5 image generation task
- `magnific-pp-cli ai get-seedream-v4-edit-task-status` — Get the status of the Seedream v4 edit task
- `magnific-pp-cli ai get-seedream-v4-task-status` — Get the status of the Seedream v4 task
- `magnific-pp-cli ai get-seedream-v5-lite-edit-task-status` — Get the status of a specific Seedream V5 Lite image editing task
- `magnific-pp-cli ai get-seedream-v5-lite-task-status` — Get the status of a specific Seedream V5 Lite image generation task
- `magnific-pp-cli ai get-skinenhancer` — Skin Enhancer - Get task status
- `magnific-pp-cli ai get-sound-effects-task-status` — Get the status of one sound-effects task
- `magnific-pp-cli ai get-style-transfer-task-status` — Get the status of the Style Transfer task
- `magnific-pp-cli ai get-texttoimage` — Get the status of the flux-dev task
- `magnific-pp-cli ai get-texttoimage-2` — Get the status of the flux-pro 1.1 task
- `magnific-pp-cli ai get-texttoimage-3` — HyperFlux - Get task status
- `magnific-pp-cli ai get-texttovideo` — Get the status of a WAN 2.5 Text-to-Video 1080p task
- `magnific-pp-cli ai get-texttovideo-2` — Get the status of a WAN 2.5 Text-to-Video 480p task
- `magnific-pp-cli ai get-texttovideo-3` — Get the status of a WAN 2.5 Text-to-Video 720p task
- `magnific-pp-cli ai get-texttovideo-4` — Get the status of a WAN 2.6 Text-to-Video 1080p task
- `magnific-pp-cli ai get-texttovideo-5` — Get the status of a WAN 2.6 Text-to-Video 720p task
- `magnific-pp-cli ai get-veed-fabric-1-0-fast-task-status` — Retrieve a specific Veed Fabric 1.0 Fast lip-sync task by its ID, including generation status and result URL when...
- `magnific-pp-cli ai get-veed-fabric-1-0-task-status` — Retrieve a specific Veed Fabric 1.0 lip-sync task by its ID, including generation status and result URL when completed.
- `magnific-pp-cli ai get-veo-31-i2v-fast-task` — Get Veo 3.1 I2V Fast task by ID
- `magnific-pp-cli ai get-veo-31-i2v-task` — Get Veo 3.1 I2V task by ID
- `magnific-pp-cli ai get-veo-31-ref2v-task` — Retrieve the status and results of a specific reference-to-video generation task.
- `magnific-pp-cli ai get-veo-31-t2v-fast-task` — Get Veo 3.1 T2V Fast task by ID
- `magnific-pp-cli ai get-veo-31-t2v-task` — Get Veo 3.1 T2V task by ID
- `magnific-pp-cli ai get-vfx-task` — Retrieve the status and results of a specific VFX video effect task by its task ID.
- `magnific-pp-cli ai get-video-kling-advanced-custom-elements-task` — **Deprecated.** This endpoint is deprecated. You can pass reference images directly in the `elements` array of the...
- `magnific-pp-cli ai get-video-kling-advanced-custom-elements-tasks` — **Deprecated.** This endpoint is deprecated. You can pass reference images directly in the `elements` array of the...
- `magnific-pp-cli ai get-video-kling-v3-motion-control-pro-task` — Retrieve the status and result of a specific Kling 3 Pro Motion Control video generation task by its task ID.
- `magnific-pp-cli ai get-video-kling-v3-motion-control-pro-tasks` — Retrieve the list of all Kling 3 Pro Motion Control video generation tasks for the authenticated user.
- `magnific-pp-cli ai get-video-kling-v3-motion-control-std-task` — Retrieve the status and result of a specific Kling 3 Standard Motion Control video generation task by its task ID.
- `magnific-pp-cli ai get-video-kling-v3-motion-control-std-tasks` — Retrieve the list of all Kling 3 Standard Motion Control video generation tasks for the authenticated user.
- `magnific-pp-cli ai get-video-kling-v3-omni-task` — Retrieve the status and result of a specific Kling 3 Omni video generation task by its task ID.
- `magnific-pp-cli ai get-video-kling-v3-omni-tasks` — Retrieve the list of all Kling 3 Omni video generation tasks for the authenticated user.
- `magnific-pp-cli ai get-video-kling-v3-task` — Retrieve the status and result of a specific Kling 3 video generation task by its task ID.
- `magnific-pp-cli ai get-video-kling-v3-tasks` — Retrieve the list of all Kling 3 video generation tasks for the authenticated user.
- `magnific-pp-cli ai get-video-kling4k-i2v-task` — Retrieve the status and result of a specific Kling 4K Image-to-Video task by its task ID.
- `magnific-pp-cli ai get-video-kling4k-i2v-tasks` — Retrieve the list of all Kling 4K Image-to-Video tasks for the authenticated user.
- `magnific-pp-cli ai get-video-kling4k-t2v-task` — Retrieve the status and result of a specific Kling 4K Text-to-Video task by its task ID.
- `magnific-pp-cli ai get-video-kling4k-t2v-tasks` — Retrieve the list of all Kling 4K Text-to-Video tasks for the authenticated user.
- `magnific-pp-cli ai get-video-upscaler-precision-task` — Returns the current status and output URL of a specific video upscaler precision task. The output URL is included...
- `magnific-pp-cli ai get-video-upscaler-precision-tasks` — Returns a list of all video upscaler precision tasks. Each task includes its ID, current status, and output URL if...
- `magnific-pp-cli ai get-video-upscaler-task` — Returns the current status and output URL of a specific video upscaler task. The output URL is included only if the...
- `magnific-pp-cli ai get-video-upscaler-tasks` — Returns a list of all video upscaler tasks. Each task includes its ID, current status, and output URL if completed.
- `magnific-pp-cli ai get-voiceover-task-status` — Get the status of one voiceover task
- `magnific-pp-cli ai get-wan-27-i2v-task` — Retrieve the status and result of a specific WAN 2.7 image-to-video task by its ID.
- `magnific-pp-cli ai get-wan-27-r2v-task` — Retrieve the status and result of a specific WAN 2.7 reference-to-video task by its ID.
- `magnific-pp-cli ai get-wan-27-t2v-task` — Retrieve the status and result of a specific WAN 2.7 text-to-video task by its ID.
- `magnific-pp-cli ai get-wan-27-video-edit-task` — Retrieve the status and result of a specific WAN 2.7 video edit task by its ID.
- `magnific-pp-cli ai get-z-image-task-status` — Get the status of a specific Z-Image image generation task
- `magnific-pp-cli ai list` — Gemini 2.5 Flash - List tasks
- `magnific-pp-cli ai list-apps` — Returns all workflow apps **published as tools** that the authenticated user can execute. Each item includes the app...
- `magnific-pp-cli ai list-imageexpand` — Get the status of all image expand tasks
- `magnific-pp-cli ai list-imageexpand-2` — Get the status of all image expand ideogram tasks
- `magnific-pp-cli ai list-imageexpand-3` — Get the status of all image expand seedream v4.5 tasks
- `magnific-pp-cli ai list-imagerelight` — Get the status of all relight tasks
- `magnific-pp-cli ai list-imagetovideo` — Get the list of the kling-elements-pro tasks
- `magnific-pp-cli ai list-imagetovideo-10` — Get the list of MiniMax Hailuo-02 768p tasks
- `magnific-pp-cli ai list-imagetovideo-11` — Get the list of MiniMax Hailuo 2.3 1080p tasks
- `magnific-pp-cli ai list-imagetovideo-12` — Get the list of MiniMax Hailuo 2.3 1080p Fast tasks
- `magnific-pp-cli ai list-imagetovideo-13` — Get the list of MiniMax Hailuo 2.3 768p tasks
- `magnific-pp-cli ai list-imagetovideo-14` — Get the list of MiniMax Hailuo 2.3 768p Fast tasks
- `magnific-pp-cli ai list-imagetovideo-15` — List PixVerse-V5 tasks filtered by resolution provided in the request body.
- `magnific-pp-cli ai list-imagetovideo-16` — PixVerse V5.5 - List tasks
- `magnific-pp-cli ai list-imagetovideo-17` — PixVerse V5 Transition - List tasks
- `magnific-pp-cli ai list-imagetovideo-18` — PixVerse V6 - List tasks
- `magnific-pp-cli ai list-imagetovideo-19` — Get the list of Seedance Lite 1080p tasks
- `magnific-pp-cli ai list-imagetovideo-2` — Get the list of the kling-o1 tasks
- `magnific-pp-cli ai list-imagetovideo-20` — Get the list of Seedance Lite 480p tasks
- `magnific-pp-cli ai list-imagetovideo-21` — Get the list of Seedance Lite 720p tasks
- `magnific-pp-cli ai list-imagetovideo-22` — Get the list of Seedance Pro 1080p tasks
- `magnific-pp-cli ai list-imagetovideo-23` — Get the list of Seedance Pro 480p tasks
- `magnific-pp-cli ai list-imagetovideo-24` — Get the list of Seedance Pro 720p tasks
- `magnific-pp-cli ai list-imagetovideo-25` — Get the list of WAN 2.5 Image-to-Video 1080p tasks
- `magnific-pp-cli ai list-imagetovideo-26` — Get the list of WAN 2.5 Image-to-Video 480p tasks
- `magnific-pp-cli ai list-imagetovideo-27` — Get the list of WAN 2.5 Image-to-Video 720p tasks
- `magnific-pp-cli ai list-imagetovideo-28` — Get the list of WAN 2.2 480p tasks
- `magnific-pp-cli ai list-imagetovideo-29` — Get the list of WAN 2.2 580p tasks
- `magnific-pp-cli ai list-imagetovideo-3` — Get the list of the kling-pro tasks
- `magnific-pp-cli ai list-imagetovideo-30` — Get the list of WAN 2.2 720p tasks
- `magnific-pp-cli ai list-imagetovideo-31` — Get the list of WAN 2.6 Image-to-Video 1080p tasks
- `magnific-pp-cli ai list-imagetovideo-32` — Get the list of WAN 2.6 Image-to-Video 720p tasks
- `magnific-pp-cli ai list-imagetovideo-4` — Get the list of the kling-v2 tasks
- `magnific-pp-cli ai list-imagetovideo-5` — Get the list of the kling-v2-1-std tasks
- `magnific-pp-cli ai list-imagetovideo-6` — Get the list of the kling-v2-1-pro tasks
- `magnific-pp-cli ai list-imagetovideo-7` — Get the list of the kling-v2-1-std tasks
- `magnific-pp-cli ai list-imagetovideo-8` — Get the list of the kling-v2-5-pro tasks
- `magnific-pp-cli ai list-imagetovideo-9` — Get the list of MiniMax Hailuo-02 1080p tasks
- `magnific-pp-cli ai list-imageupscaler` — Get the status of all upscaling tasks
- `magnific-pp-cli ai list-my-apps` — Returns all workflow apps **owned** by the authenticated user, including drafts and unpublished workflows. Unlike...
- `magnific-pp-cli ai list-mystic` — Get the status of all Mystic tasks
- `magnific-pp-cli ai list-skinenhancer` — Skin Enhancer - List tasks
- `magnific-pp-cli ai list-texttoimage` — Get the status of the flux-dev task
- `magnific-pp-cli ai list-texttoimage-2` — Get the status of the flux-pro 1.1 task
- `magnific-pp-cli ai list-texttoimage-3` — HyperFlux - List tasks
- `magnific-pp-cli ai list-texttovideo` — Get the list of WAN 2.5 Text-to-Video 1080p tasks
- `magnific-pp-cli ai list-texttovideo-2` — Get the list of WAN 2.5 Text-to-Video 480p tasks
- `magnific-pp-cli ai list-texttovideo-3` — Get the list of WAN 2.5 Text-to-Video 720p tasks
- `magnific-pp-cli ai list-texttovideo-4` — Get the list of WAN 2.6 Text-to-Video 1080p tasks
- `magnific-pp-cli ai list-texttovideo-5` — Get the list of WAN 2.6 Text-to-Video 720p tasks
- `magnific-pp-cli ai post-image-to-video-kling-o1-pro` — Kling O1 Pro - Create video from image
- `magnific-pp-cli ai post-image-to-video-kling-o1-pro-video-reference` — Kling O1 Pro - Create video with reference
- `magnific-pp-cli ai post-image-to-video-kling-o1-std` — Kling O1 Standard - Create video from image
- `magnific-pp-cli ai post-image-to-video-kling-o1-std-video-reference` — Kling O1 Standard - Create video with reference
- `magnific-pp-cli ai post-image-to-video-kling-v26-pro` — Kling 2.6 Pro - Create video from text or image
- `magnific-pp-cli ai post-image-upscaler-precision` — Upscales an image while adding new visual elements or details. This endpoint may modify the original image content...
- `magnific-pp-cli ai post-image-upscaler-precision-v2` — Upscales an image while adding new visual elements or details (V2). This endpoint may modify the original image...
- `magnific-pp-cli ai post-reference-to-video-kling-v3-omni-pro` — Generate AI video using Kling 3 Omni Pro with a reference video for motion and style guidance. **Video-to-video...
- `magnific-pp-cli ai post-reference-to-video-kling-v3-omni-std` — Generate AI video using Kling 3 Omni Standard with a reference video for motion and style guidance. **Video-to-video...
- `magnific-pp-cli ai post-video-kling-advanced-custom-elements` — **Deprecated.** This endpoint is deprecated. You can pass reference images directly in the `elements` array of the...
- `magnific-pp-cli ai post-video-kling-v26-motion-control-pro` — Transfer motion from a reference video to a character image using Kling 2.6 Pro. The model preserves the character's...
- `magnific-pp-cli ai post-video-kling-v26-motion-control-std` — Transfer motion from a reference video to a character image using Kling 2.6 Standard. The model preserves the...
- `magnific-pp-cli ai post-video-kling-v3-motion-control-pro` — Transfer motion from a reference video to a character image using Kling 3 Pro. The model preserves the character's...
- `magnific-pp-cli ai post-video-kling-v3-motion-control-std` — Transfer motion from a reference video to a character image using Kling 3 Standard. The model preserves the...
- `magnific-pp-cli ai post-video-kling-v3-omni-pro` — Generate AI video using Kling 3 Omni Pro with advanced multi-modal capabilities. **Features:** - **Text-to-video**:...
- `magnific-pp-cli ai post-video-kling-v3-omni-std` — Generate AI video using Kling 3 Omni Standard with advanced multi-modal capabilities. **Features:** -...
- `magnific-pp-cli ai post-video-kling-v3-pro` — Generate AI video using Kling 3 Pro with text-to-video or image-to-video capabilities. **Features:** -...
- `magnific-pp-cli ai post-video-kling-v3-std` — Generate AI video using Kling 3 Standard with text-to-video or image-to-video capabilities. **Features:** -...
- `magnific-pp-cli ai post-video-kling4k-i2v` — Generate AI video in 4K resolution from an image using Kling 4K Image-to-Video. **Features:** - **Image-to-video**:...
- `magnific-pp-cli ai post-video-kling4k-t2v` — Generate AI video in 4K resolution from a text prompt using Kling 4K Text-to-Video. **Features:** -...
- `magnific-pp-cli ai post-video-upscaler` — Upscales a video while enhancing visual quality and resolution. Supports various output resolutions (720p, 1k, 2k,...
- `magnific-pp-cli ai post-video-upscaler-precision` — Upscales a video with precision-grade quality enhancement. Designed for frame-accurate upscaling with fine-grained...
- `magnific-pp-cli ai post-video-upscaler-turbo` — Upscales a video using turbo processing with premium quality enhancement applied automatically. Turbo mode reduces...
- `magnific-pp-cli ai remove-image-background` — This endpoint removes the background from an image provided via a URL. The URLs in the response are temporary and...
- `magnific-pp-cli ai run-app` — Triggers the execution of a workflow app. Provide values for all required inputs defined in the app's definition...

**icons** — Manage icons

- `magnific-pp-cli icons get-detail-by-id` — Get detailed information about a specific icon identified by its unique ID.
- `magnific-pp-cli icons search` — Get a list of icons based on the provided parameters and ordering criteria.

**music** — Manage music

- `magnific-pp-cli music get-detail` — Retrieve full details for a music item including artist biography, genre and mood metadata, popularity score, and...
- `magnific-pp-cli music search` — Search the Freepik Music catalog. Filter by genre, mood, artist, premium status, and creation date range. Returns...

**resources** — Manage resources

- `magnific-pp-cli resources get-detail-by-id` — Retrieve the detailed information of a specific resource by its ID. This endpoint supports multiple resource types...
- `magnific-pp-cli resources search` — Retrieve a list of resources based on various filter criteria such as orientation, content type, license, and more.

**sound-effects** — Manage sound effects

- `magnific-pp-cli sound-effects get-detail` — Retrieve full details for a sound effect including category hierarchy, tags, duration, popularity, and download...
- `magnific-pp-cli sound-effects search` — Search the Freepik Sound Effects catalog. Filter by category, duration range, premium status, and creation date....

**videos** — Manage videos

- `magnific-pp-cli videos get` — Get detailed video information by ID
- `magnific-pp-cli videos list` — Search and filter videos by specified order


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
magnific-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pick the right image model for a brand campaign

```bash
magnific-pp-cli compare "product hero shot, neon-lit, isometric" --models mystic,flux-2-pro,seedream-v4-5,z-image-turbo --aspect 1:1 --json --select results[].model,results[].credit_cost,results[].dispatch_latency_ms,results[].task_id
```

Fans the same prompt to 4 image models, polls all tasks, and returns only the fields an agent needs to score the winners.

### Resume work after a polling crash

```bash
magnific-pp-cli tasks stale --since 24h --json && magnific-pp-cli tasks reconcile --json
```

Surfaces tasks the local store thinks are still pending, then re-polls each model endpoint to update terminal state — the standard reconcile fix from internal/store.

### Search your own generation history

```bash
magnific-pp-cli history search "tokyo neon" --since 90d --json --select prompt,model,output_url,credit_cost
```

FTS5 query against the local prompts table. Returns just the dotted fields agents need; verbose payloads stay out of context.

### Forecast a video batch before submitting

```bash
magnific-pp-cli cost forecast --model kling-v2-6-pro --count 20 --json
```

Multiplies the curated per-model credit cost by intended runs so you can compare against your dashboard balance before submitting. Magnific's spec exposes no live-credit endpoint; check the web dashboard for actual balance.

### Replay an approved prompt with one variable swap

```bash
magnific-pp-cli prompt run hero-shot --override city=osaka --json
```

Substitutes {{city}} in the saved template and dispatches the real model endpoint, recording the run in the local tasks table for later analytics.

## Auth Setup

Magnific (formerly Freepik) authenticates with a single `x-freepik-api-key` header. The same key works on both `api.freepik.com` and `api.magnific.com`. Set `FREEPIK_API_KEY` (preferred) or the rebrand alias `MAGNIFIC_API_KEY` in your environment, or run `magnific-pp-cli auth set-token <key>` to write it to your config. Get a key at https://www.magnific.com/api.

Run `magnific-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  magnific-pp-cli ai list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
magnific-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
magnific-pp-cli feedback --stdin < notes.txt
magnific-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.magnific-pp-cli/feedback.jsonl`. They are never POSTed unless `MAGNIFIC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MAGNIFIC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
magnific-pp-cli profile save briefing --json
magnific-pp-cli --profile briefing ai list
magnific-pp-cli profile list --json
magnific-pp-cli profile show briefing
magnific-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `magnific-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add magnific-pp-mcp -- magnific-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which magnific-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   magnific-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `magnific-pp-cli <command> --help`.
