# Marginal Revolution print brief

Marginal Revolution exposes a public RSS feed at `https://marginalrevolution.com/feed`. The printed package uses a minimal OpenAPI description for the feed endpoint, then layers RSS-native commands for the workflows agents need most often: recent posts, feed search, reading a post from the current feed, outbound link extraction, and author/category summaries.

The package is intentionally unauthenticated and read-only. The custom RSS layer parses feed XML into structured JSON/text output while keeping the generated v4.2.1 CLI, MCP, store, doctor, import/export, and skill scaffolding intact.
