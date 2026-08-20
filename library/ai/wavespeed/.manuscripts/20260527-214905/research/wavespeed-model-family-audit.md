# WaveSpeed model-family audit

This printed CLI was checked against the current WaveSpeed model catalog and
the official WaveSpeed CLI/API documentation before publication.

Primary families covered:

- Nano Banana: `google/nano-banana/text-to-image`, `google/nano-banana/edit`
- Nano Banana 2: `google/nano-banana-2/text-to-image`, `google/nano-banana-2/edit`
- Nano Banana Pro: `google/nano-banana-pro/text-to-image`, `google/nano-banana-pro/edit`
- Seedream: `bytedance/seedream-v4.5`, `bytedance/seedream-v5.0-lite`
- Seedance 2.0: text-to-video, image-to-video, turbo, edit, and extend variants
- Kling: v3.0, video-o3, motion-control, text-to-video, and image-to-video variants
- Veo: `google/veo3.1` text-to-video, image-to-video, reference-to-video, and extend variants

The catalog shows that these families share a dynamic run shape rather than one
fixed request body. Common fields include `prompt`, `image`, `images`, `video`,
`last_image`, `end_image`, `reference_images`, `reference_videos`, `duration`,
`resolution`, `aspect_ratio`, `size`, `quality`, `output_format`, `seed`,
`negative_prompt`, `multi_prompt`, and `element_list`.

Publication decision:

- Keep model-specific settings schema-driven through `-i key=value`.
- Add ergonomic media shorthands for the fields that recur across image and
  video families.
- Support `@local-file` upload coercion inside `run` and `price`, so local
  images/videos can be used directly in image-to-video, edit, reference, and
  motion-control workflows.
- Support JSON file references for structured fields such as `element_list` and
  `multi_prompt`.
- Preserve the generic `schema` and `run <model> --help` surfaces so fast-moving
  WaveSpeed model schemas remain discoverable without hardcoded wrappers.

Reference sources:

- https://wavespeed.ai/docs/wavespeed-cli
- https://wavespeed.ai/docs/sync-mode
- https://wavespeed.ai/nano-banana-2-api
- https://wavespeed.ai/seedance-2-api
- https://wavespeed.ai/kling-3-api
- https://wavespeed.ai/veo-3-1-api
