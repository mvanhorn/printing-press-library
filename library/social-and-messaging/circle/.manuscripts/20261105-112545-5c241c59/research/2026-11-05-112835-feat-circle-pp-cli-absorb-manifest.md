<!-- slop-gate: off -->
# circle-pp-cli absorb manifest

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List/get spaces | iamnortey/circle-mcp list_spaces/get_space | (generated endpoint) spaces list | --json/--select, offline via store |
| 2 | List/get posts | circle-mcp list_posts/get_post | (generated endpoint) posts list | store + FTS |
| 3 | List/search members | circle-mcp list_members | (generated endpoint) community-members list | offline, SQL |
| 4 | Comments list/get | circle-mcp list_comments/get_comment | (generated endpoint) comments list | offline |
| 5 | Topics, community, space groups | circle-mcp list_topics/get_community/list_space_groups | (generated endpoint) topics list | |
| 6 | Cross-entity search | circle-mcp search, Admin advanced_search | circle-pp-cli search | offline FTS, no quota |
| 7 | Create/update post | circle-mcp create_post/update_post | (generated endpoint) posts create | --dry-run |
| 8 | Unanswered-post detection | circle-mcp detect_unanswered_posts | (behavior in circle-pp-cli inbox) unreplied posts section | joined with other triage signals |
| 9 | Community health | circle-mcp community_health | (behavior in circle-pp-cli inbox) counts section | |
| 10 | Every Admin V2 endpoint (175 ops) | Official Circle MCP / circleso npm / circle-so-python-sdk | (generated endpoint) <resource> <endpoint> | agent-native CLI + MCP, typed exit codes |
| 11 | Local mirror + SQL | none | (behavior in circle-pp-cli analytics) local queries; raw SELECT through the MCP sql tool | quota-free repeat reads |
| 12 | Sync | none | circle-pp-cli sync | incremental local mirror |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Declarative structure apply | spaces apply | hand-code | 10 | Plan-then-apply diff across space groups and spaces; Circle UI and MCP write one object at a time with no preview | Use this command to make Circle's space structure match a YAML file. Do NOT use it to read structure; use 'spaces list' instead. |
| 2 | Course push | course push | hand-code | 9 | Sections, lessons, uploads and reorder in one idempotent pass from local markdown, with an id map so renames do not duplicate | Use this command to upload a local course folder to a Circle course space. Do NOT use this command to download a Circle course to disk; use 'course pull' instead. |
| 3 | Course pull | course pull | hand-code | 8 | Rebuilds the course tree into folders with frontmatter ids so push round-trips | Use this command to download a Circle course space into a local folder. Do NOT use this command to upload local lesson changes; use 'course push' instead. |
| 4 | Seat count | seats | hand-code | 9 | Deduplicates payers across subscriptions and charges against a cap from the local store | Use this command for current paid-seat counts and remaining capacity. Do NOT use this command for a yes/no launch decision against a target; use 'founding' instead. |
| 5 | Founding go/no-go | founding | hand-code | 8 | Scriptable GO/NO-GO verdict with a typed exit code from the same payer computation | Use this command for a GO/NO-GO verdict on whether at least N people have paid. Do NOT use this command to list payers or remaining capacity; use 'seats' instead. |
| 6 | Batch onboarding | onboard | hand-code | 9 | Member + spaces + space groups + access groups + tags per person, idempotent, no email by default | none |
| 7 | Recurring session scheduler | show-tell schedule | hand-code | 8 | Circle has no recurring series in the API; duplicate + reschedule per cadence date with dedupe | Use this command to create future Show & Tell events from a template on a cadence. Do NOT use this command to gather what happened at a past session; use 'show-tell recap' instead. |
| 8 | Session recap draft | show-tell recap | hand-code | 8 | Joins event, attendees, and the matching live room transcript, which Circle does not link | Use this command to assemble attendees and transcript for a past Show & Tell into a draft. Do NOT use this command to create upcoming sessions; use 'show-tell schedule' instead. |
| 9 | Morning triage | inbox | hand-code | 10 | Six resources in one local query instead of six quota-metered calls | Use this command for what needs attention since a point in time across members, posts, moderation, forms, and payments. Do NOT use this command for paid-seat counts or capacity; use 'seats' instead. |

Deferred (survived the cut, not on the owner-approved list): spaces pull (7/10).
Stubs: none.
