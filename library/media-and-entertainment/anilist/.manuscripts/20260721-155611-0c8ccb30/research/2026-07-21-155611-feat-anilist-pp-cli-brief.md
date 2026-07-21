# AniList Printing Press Brief

## API and evidence

- **API:** AniList GraphQL API, `POST https://graphql.anilist.co`.
- **Primary source:** official [AniList docs](https://docs.anilist.co/guide/graphql/) and the public schema mirrored by api-evangelist/anilist.
- **Authentication:** public catalog queries need no token. User-specific list reads and mutations require an OAuth token. Official docs say tokens are long-lived for one year and refresh tokens are unavailable. The canonical environment variable is `ANILIST_TOKEN`.
- **Reachability target:** a public `Media` GraphQL query must return HTTP 200 before generation.

## Users

1. **The weeknight anime watcher:** follows several currently airing shows and wants to know which followed series has an episode available tonight, then record a completed episode without opening a web form.
2. **The intentional backlog curator:** has a large Planning list, limited evening time, and wants a short, watchable pick filtered by episode count, status, genre, and personal priority rather than a generic popularity list.
3. **The shared-fandom planner:** compares seasonal schedules and recommendations with a friend or family member, wants succinct agent-ready cards, and needs links and titles that are safe to share.
4. **The media-data explorer:** searches anime, manga, characters, staff, studios, tags, reviews, recommendations, and user activity and wants structured results that pipe cleanly into automation.

## Top Workflows

1. **Tonight schedule:** ask “what anime I follow airs tonight?”, resolve the authenticated viewer’s CURRENT anime list, join it with upcoming `AiringSchedule` records, and return only due episodes with local-time timestamps and media links.
2. **Safe episode check-in:** resolve an anime by title or ID, inspect the viewer’s existing list entry, show the before/after progress, and require explicit `--apply` to write `SaveMediaListEntry`; dry-run is the default.
3. **Short backlog recommendation:** inspect the viewer’s PLANNING list, exclude completed/dropped media and entries with no known episode count, rank candidates by the user’s priority/score plus a short-runtime preference, and report why each pick qualified.
4. **Discover and decide:** search catalog media with filters, inspect title/season/episodes/tags/next-airing data, and retrieve related recommendations before adding an entry.
5. **Maintain the personal library:** list/update/remove MediaList entries; inspect profile/stats/activity; optionally favorite media, people, or studios with confirmation.

## User Vision

The user explicitly wants a personal-use agent CLI rather than another business-data wrapper. Its headline requests are: “What anime I follow airs tonight?”, “Mark that episode watched,” and “Recommend something short from my backlog.” The CLI must make those requests reliable and safe for an autonomous agent.

## Incumbent assessment

- **yuna0x0/anilist-mcp:** broad MCP-only wrapper with search, details, lists, user/activity/thread, favorites, genres/tags and mutations. It exposes many raw operations, but no standalone CLI, no local persistence, no safe dry-run/apply check-in flow, no tonight join, and no personal short-backlog ranking.
- **tamnd/anilist-cli:** presents a high-quality generic Go output contract but its README states that it is a fresh scaffold with one example `page` resource; it is not an AniList tracker implementation.
- **MALSync/MALSync and tracking applications:** prove demand for automated episode progress tracking, but are browser/media-server integrations rather than agent-native AniList CLI workflows.

## Scope policy

All approved rows are shipping rows. No rows may be silently converted to stubs. Account-changing operations must default to dry-run or explicit confirmation and must never depend on a browser at runtime. The printed CLI will use the API directly and store locally synced data only where it produces an inspectable personal workflow.
