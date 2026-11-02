# Publish review fixes

The first automated review identified three valid issues, now fixed:

- Saved normalized records populate California/federal typed tables and the generic datasets index. A fresh-store regression test saves identical IDs in both sources and verifies both filtered searches plus generic search.
- Catalog and resource clients use the shared configured TLS transport and timeout while preserving same-origin redirects and source-isolated credentials. SEC requests receive the same selected transport and timeout through command-scoped context settings, preserve SEC-only redirects, and honor a lower user rate ceiling. Local HTTPS tests verify custom CA and server-name trust, timeout above 30 seconds, invalid CA rejection, rate configuration and redirect isolation.
- The desktop MCP manifest exposes an optional SEC application/contact setting mapped to SEC_USER_AGENT; credentials are placeholders only.

Validation after the fixes: full unit suite and go vet passed, publish validate passed, SKILL and MCP manifest checks passed. Full live matrix reran against this final source: 171/171 executed checks passed, zero failed, 152 skipped/unverified retained separately.
