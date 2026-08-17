# Twitch CLI Build Log

## What Was Built

A **generator-produced** CLI (`cli-printing-press generate`) from a community
OpenAPI document for the Twitch Helix API. No novel commands were hand-written
beyond the generator's standard scaffold; two small post-generation patches
adapt the generated client to Twitch's auth requirements (recorded under
`.printing-press-patches/`).

### Source spec

- Community spec from
  [DmitryScaletta/twitch-api-swagger](https://github.com/DmitryScaletta/twitch-api-swagger),
  auto-generated from the official Twitch API reference with manual fixes
  (95 paths).
- Vendored locally as `spec.json` with two corrections:
  1. **Auth flow.** The upstream spec declared only the OAuth2 `implicit`
     browser flow. It was rewritten to `clientCredentials` (app access token)
     with the real token endpoint `https://id.twitch.tv/oauth2/token`. This is
     Twitch's recommended grant for server-to-server access to public data and
     requires no user-consent browser round trip.
  2. **Text hygiene.** Unicode smart punctuation (curly quotes, en/em dashes)
     in descriptions was normalized to ASCII to avoid mojibake in generated
     help text.
  The server URL `https://api.twitch.tv/helix` was already correct.

### Generation

```
cli-printing-press generate \
  --spec twitch-spec.json \
  --name twitch \
  --spec-source community \
  --category media-and-entertainment
```

The spec advertises a single security scheme, so no `--auth-preference` was
needed; the parser emits the `client_credentials` mint-and-cache flow.

### Generated surface (absorb layer)

30 resource groups / 144 endpoints across the Helix API: `games`
(+ `games get-top`), `streams`, `channels`, `clips`, `videos`, `users`, `chat`,
`channel-points`, `polls`, `predictions`, `subscriptions`, `moderation`,
`bits`, `charity`, `goals`, `hypetrain`, `schedule`, `teams`, `whispers`,
`eventsub`, `twitch-helix-analytics`, `twitch-helix-search`. The `analytics`
and `search` Helix resources are prefixed `twitch-helix-*` because their bare
names collide with the framework's transcend commands.

- Auth: `auth login` (client_credentials mint) / `auth status` / `auth logout`
  / `auth set-token`.
- Agent plumbing: `agent-context`, `which`, `doctor`, `feedback`, `import`,
  `export`, `profile`, `version`.
- MCP server `twitch-pp-mcp` (144 endpoints, code orchestration pattern).

## Post-generation patches

1. **`twitch-client-id-header.json`** — Twitch Helix requires a `Client-Id`
   header on every request in addition to the bearer token. The generated
   client only set `Authorization`; the patch adds
   `req.Header.Set("Client-Id", cfg.ClientID)` in `client.do()` and mirrors it
   in the `--dry-run` preview. Without it every live call returns
   `401 Client-Id header required`.
2. **`twitch-default-sync-app-token-viable.json`** — trimmed
   `defaultSyncResources` to the resources an app access token can fetch with
   no required parameter (`games-top`, `streams`,
   `content-classification-labels`, `chat-emotes-global`,
   `eventsub-subscriptions`). The remaining resources stay reachable via
   `--resources`. Out of the box `sync` returns ~349 records with zero
   access-policy warnings.

## Quality gates

All 8 generation gates passed: `go mod tidy`, ensure safe `golang.org/x/net`,
`govulncheck ./...`, `go vet ./...`, `go build ./...`, build runnable binary,
`--help`, `version`, `doctor`. Shipcheck: 6/6 legs pass, scorecard 94/100
Grade A.
