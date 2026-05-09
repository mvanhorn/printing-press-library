# Product Hunt CLI Brief

## API Identity
- Domain: Product discovery platform — founders launch products, hunters upvote, the community surfaces what's worth using
- Users: Founders/makers (launch day tracking, competitive awareness), hunters (discovering new tools), investors (watching launch momentum), developers (monitoring specific topics for competitive intel), agents (daily briefing workflows)
- Data profile: Posts (launches), Votes, Comments, Collections, Topics, Users/Makers, Goals, MakerGroups
- API: GraphQL v2 at `https://api.producthunt.com/v2/api/graphql` — all calls POST with Bearer auth

## Reachability Risk
- **Low** with valid PRODUCT_HUNT_TOKEN (GET free developer token from producthunt.com → Settings → API Dashboard → Create Application → Developer Token)
- **Medium** without token: Issue #322 reports Cloudflare bot protection on the API; issues #324, #326 report 403 errors
- Issue #328: token endpoint temporarily returning 404 (sporadic, not consistent)
- Rate limiting enforced but undocumented; community reports 60 req/min effective ceiling
- Verdict: API is functional and actively used; friction is auth-setup, not structural breakage

## Top Workflows
1. **Today's feed**: What launched today? Ranked by votes. Filter by topic. Morning ritual for PH power users.
2. **Product deep-dive**: Get all details on a specific product — tagline, makers, vote count, comments thread
3. **Topic monitoring**: Watch a specific topic (e.g., "developer-tools") for new launches weekly
4. **Maker research**: Who has the most momentum? Find makers launching multiple successful products recently
5. **Launch prep research**: Before launching, audit competitors on the same topic with similar vote patterns

## Table Stakes (what every competitor has)
- List/browse posts (today, featured, newest)
- Filter posts by topic
- Get post details (name, tagline, votes, comments)
- Get user profile
- Browse collections
- Search topics

## Data Layer
- Primary entities: Post, User, Topic, Collection, Comment, Goal, MakerGroup
- Sync cursor: posts by `featuredAt`/`createdAt` desc, paginated via cursor-based connections
- FTS/search: post name + tagline + description; topic name; user name + username
- High-value joins: posts ↔ makers (co-maker networks), posts ↔ topics (category velocity), posts ↔ votes over time (launch momentum)

## Codebase Intelligence
- Source: producthunt/producthunt-api (official) + jaipandya/producthunt-mcp-server (reference impl)
- Auth: `Authorization: Bearer {token}` — developer_token never expires, linked to PH account
- Data model: `Post` has makers (User[]), topics (Topic[]), media (Media[]), reviewsRating (float)
- Rate limiting: undocumented; observed ~60 req/min for standard tokens
- Architecture: Pure GraphQL — all operations POST to `/v2/api/graphql` with JSON body `{query, variables}`

## Product Thesis
- Name: `product-hunt-pp-cli`
- Why it should exist: Every existing Product Hunt CLI is either JavaScript-menu-driven, unmaintained (3-8 years old), missing offline search, or has no structured output. A Go CLI with SQLite sync lets founders track launch momentum, agents run daily discovery briefings, and developers monitor topic velocity — all without hitting rate limits on every query.

## Build Priorities
1. **Posts**: list-today, get (by slug/ID), filter by topic, sort by votes/newest/featured
2. **Sync + offline**: Sync posts/topics/users to SQLite, FTS search offline
3. **Transcendence**: launch radar (vote velocity), hot-streak makers, topic heatmap, maker network
4. **Users + Topics + Collections**: browse and get commands
5. **Comments**: thread view on a post
