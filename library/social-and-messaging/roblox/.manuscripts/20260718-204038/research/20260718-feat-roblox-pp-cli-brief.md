# Roblox CLI Brief

## API Identity
- Domain: Roblox public web APIs for users, avatars, groups, games, catalog, inventory, badges, friends, presence, and thumbnails.
- Data profile: 107 legacy web API operations across 61 resource families and multiple `*.roblox.com` hosts. The current CLI is strongest for unauthenticated public lookup and discovery. Some included endpoints require a Roblox session cookie and state-changing calls also require CSRF handling.
- Current official direction: Roblox Creator Hub now documents both legacy web endpoints and Open Cloud APIs. Open Cloud uses `apis.roblox.com`, API keys or OAuth 2.0, granular scopes, and an official OpenAPI document. This CLI remains intentionally scoped to the existing public/legacy web surface rather than silently changing into an Open Cloud creator-administration CLI.

## Users
- Roblox ecosystem researchers who repeatedly inspect public users, avatars, groups, badges, inventories, games, and catalog records while comparing related IDs.
- Trust-and-safety or community analysts who investigate public account/group relationships and need reproducible JSON rather than manually traversing profile pages.
- Experience and UGC developers who need lightweight public metadata and thumbnails in shell scripts, CI jobs, or agent workflows without standing up an SDK project.
- Data-minded Roblox players and community operators who periodically snapshot public entities and want local search, grouping, and change detection.

## Reachability Risk
- Low for tested public reads: `users get 1`, `avatar-users avatar get 1`, and `groups-2 get 1` returned valid live JSON on 2026-07-18.
- High for authenticated and mutating legacy endpoints: current research shows Cookie auth on many legacy operations, while the generated spec marks every endpoint unauthenticated. Write operations must not be represented as safely usable without session and CSRF support.
- Open Cloud is a separate authenticated surface and should not be conflated with this no-key public lookup CLI.

## Top Workflows
1. Public identity investigation: resolve a user, then inspect avatar, badges, groups, friends, inventory visibility, and thumbnails using the same numeric ID.
2. Group due diligence: fetch a group, its roles, membership-related public data, and linked public owner information in a repeatable JSON workflow.
3. Game and catalog research: look up games/universes, catalog items or bundles, owners/favorites, and thumbnail assets, then narrow large responses with `--select`.
4. Periodic public-data snapshotting: sync selected public resources into SQLite, search them locally, group them with analytics, and compare later snapshots.
5. Agent-safe automation: use `--agent`, deterministic exit codes, dry-run, and compact/select output to keep scripted research reproducible.

## Table Stakes
- Direct typed commands for user, group, avatar, badge, inventory, game, catalog, friend, presence, and thumbnail lookups.
- JSON and compact/select output suitable for scripts and agents.
- Pagination, caching, retry/rate-limit handling, dry-run, and structured errors.
- Explicit authentication boundaries so public commands and Cookie/CSRF-dependent commands are not confused.

## Data Layer
- Primary entities: users, groups, games/universes, assets/bundles, badges, outfits, inventories, friendships/follows, presence, and thumbnails.
- Sync cursor: endpoint-specific cursor pagination where available; timestamped local snapshots otherwise.
- FTS/search: names, display names, descriptions, usernames, group/game/catalog titles, and raw JSON for schema-tolerant discovery.

## Product Thesis
- Name: Roblox CLI
- Why it should exist: one agent-friendly shell surface for Roblox's fragmented public web APIs, with cross-resource investigation and local history that the website and one-off HTTP calls do not provide.
- Boundary: this is not a replacement for Roblox Studio, the official Open Cloud creator APIs, or browser-session account mutation.

## Build Priorities
1. Preserve and revalidate the working public lookup commands across Roblox hosts.
2. Correct the misleading all-endpoints-no-auth model and clearly gate Cookie/CSRF-dependent operations.
3. Retain local sync/search/analytics capabilities and add Roblox-specific cross-resource investigation commands only when they are testable.
4. Run live dogfood on safe public reads; do not exercise destructive or account-mutating endpoints.
