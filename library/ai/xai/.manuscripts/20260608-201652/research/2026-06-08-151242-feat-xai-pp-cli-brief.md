# xAI CLI Research Brief

## Source Resolution

- User target: xAI.
- User-provided docs URL: `https://docs.x.ai/api`.
- Raw fetch evidence: `https://docs.x.ai/api` and `https://docs.x.ai/api/` return 404 from the docs site.
- Official API reference route: `https://docs.x.ai/api-reference` returns 200 and redirects to `https://docs.x.ai/developers/rest-api-reference/inference`.
- Official OpenAPI source: `https://docs.x.ai/openapi.json` returns OpenAPI 3.1 JSON.
- Equivalent OpenAPI source: `https://docs.x.ai/api/openapi.json` also returns OpenAPI 3.1 JSON.

## Contract Notes

- The official spec describes xAI as an OpenAI-compatible REST API.
- The official docs state the route base is `https://api.x.ai` and require `Authorization: Bearer <your xAI API key>`.
- The OpenAPI document omits a `servers` block, so the local normalized spec adds `servers: [{url: "https://api.x.ai"}]` from the official API reference.
- The OpenAPI document references `#/components/schemas/PublicUrlOptions` but does not define it, so the local normalized spec adds a permissive placeholder schema.
- The spec has 36 path keys and the generator expands them to 41 operations.

## Surface Summary

Primary endpoint families include models, language models, embeddings, chat completions, responses, files, images, videos, skills, documents search, tokenization, caller identity, and API key metadata.

## Auth

Use `XAI_API_KEY` as the runtime credential environment variable for live smoke and dogfood. The key value is retrieved from 1Password during commands and is not written to artifacts.
