# Medium CLI Brief

> Historical note: this is the original v1 research brief. The shipped CLI is **keyless** — it
> reads Medium's own public surfaces directly (RSS feeds, the article page, and Medium's internal
> GraphQL endpoint), with no API key and no third-party service; member-locked bodies use the
> user's own Medium session cookie. The endpoint-and-key plan below is the superseded original,
> kept as the generation record.

## API Identity
- Domain: Medium (medium.com) reading/research, via Medium's own public surfaces — RSS feeds, the article page (embedded JSON), and Medium's internal GraphQL endpoint — with no API key.
- Users: developers, agents, and researchers who want programmatic read access to Medium authors, publications, tags, and article content (the official write API closed to new integrations Jan 2025).
- Data profile: high-gravity entities are users (authors), articles, publications, tags, lists. Articles carry rich text (content/markdown/html), engagement (claps, voters, responses), and metadata (reading_time, word_count, topics, tags, is_locked).

## Reachability Risk
- Low. The public surfaces (RSS, the article page, the internal GraphQL endpoint) return 200 anonymously — no key, no per-call quota. If Medium changes the internal GraphQL, search/author-archive degrade gracefully while feed/read keep working.

## Top Workflows
1. Archive a specific author's full body of work and search it offline.
2. Monitor a topic/tag (trending, latest, top writers) in one digest.
3. Pull the full content (markdown/html) + images of a specific article, including member-only stories the user can read.
4. Find authoritative writers/publications on a subject.
5. Build and query a personal local corpus of synced Medium content.

## Table Stakes (absorbed)
- Full read surface: users (profile, articles, followers, following, interests, lists, books, publications), articles (info, content, markdown, html, assets, fans, responses, related, recommended), publications (info, articles, newsletter), tags (info, related, root, archived, recommended-feed, recommended-users, latestposts, top-writers, topfeeds), search (articles, users, publications, tags, lists), lists.
- Matches the Python/JS wrappers' entire method surface; beats them with offline SQLite, --json/--compact/--select, typed exit codes, and an MCP server.

## Data Layer
- Primary entities: users, articles, publications, tags, lists.
- Sync cursor: per-author / per-tag / per-publication article id sets; articles keyed by id.
- FTS/search: SQLite FTS over archived article title/subtitle/content for the `corpus` command.

## Competitors
- weeping-angel/medium-api (Python), medium-api-js (JS): same read surface, no local store, no agent-native output, no MCP.
- Dishant27/medium-mcp-server (TS): the only agent-native competitor; ~8 tools, no local corpus, no compound queries. We beat it on surface breadth + offline + compound commands.
- Abandoned publish CLIs (lambtron, djadmin, md2mid): dead since the 2025 write-API closure; out of scope.

## Product Thesis
- Name: Medium (medium-pp-cli)
- Why it should exist: there is no modern, agent-native, read-focused, open-source Medium CLI. Medium is a corpus of practitioner essays; the win is mirroring it locally for compound queries the raw API and the 10-item RSS feed cannot answer.

## User Vision (scope guardrail)
- Build the GENERIC read CLI only. The user's personal downstream workflows (Gmail newsletter triage, add-to-Reader, create-Outliner-resource) are explicitly OUT of scope and must NOT be absorbed as features. Keep the published CLI generic and keyless (member content via the user's own Medium session, never a third-party key).

## Build Priorities
1. Data layer + sync for users/articles/publications/tags/lists (Priority 0).
2. All 41 absorbed read endpoints (Priority 1 — already generated).
3. Transcendence commands: author-archive, corpus, tag-pulse, who-writes, author-compare, digest (Priority 2 — hand-built).
