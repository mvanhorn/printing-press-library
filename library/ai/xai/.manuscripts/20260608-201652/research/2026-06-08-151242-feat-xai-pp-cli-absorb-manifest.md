# xAI Absorb Manifest

## Approved Shipping Scope

- Generate endpoint mirror commands from the official xAI OpenAPI surface.
- Preserve OpenAI-compatible workflows in examples where generated naturally by the spec.
- Include agent-readable JSON output and MCP exposure from the Printing Press defaults.
- Use live read-only smoke coverage with `XAI_API_KEY` for models/caller/API-key metadata where possible.

## Non-goals

- Do not create synthetic model catalogs or hardcoded response builders.
- Do not hand-roll OpenAI SDK behavior; endpoint commands must call xAI's real REST API.
- Do not run destructive file, skill, response, image, or video delete/create tests unless explicitly allowed.
