# Substack CLI Absorb Manifest

Total: **56 absorbed features** + **8 transcendence features** = **64 features**.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List own posts | postcli (`posts list`), jakub-k-slys SDK | `posts list [--publication]` + SQLite cache | Cross-publication scoping, offline |
| 2 | Get post by slug | postcli (`posts get --slug`) | `posts get <slug>` | Markdown export, paywall flag |
| 3 | React to post | postcli (`posts react`) | `posts react <id>` | `--dry-run`, batch from stdin |
| 4 | Restack post | postcli (`posts restack`) | `posts restack <id>` | Idempotent |
| 5 | List notes | postcli (`notes list`) | `notes list [--limit]` + cursor sync | Offline cache, FTS |
| 6 | Publish note | postcli (`notes publish`) | `notes publish <text>` | Markdown input |
| 7 | Reply to note | postcli (`notes reply`) | `notes reply <id> <text>` | `--dry-run` |
| 8 | React to note | postcli (`notes react`) | `notes react <id>` | Batch |
| 9 | Restack note | postcli (`notes restack`) | `notes restack <id>` | Idempotent |
| 10 | List comments on post | postcli (`comments list`) | `comments list <post-id>` + cache | Offline, FTS |
| 11 | Add comment | postcli (`comments add`) | `comments add <post-id> <text>` | `--dry-run` |
| 12 | React to comment | postcli (`comments react`) | `comments react <id>` | Batch |
| 13 | Get feed | postcli (`feed list`) | `feed list [--tab for-you\|following\|categories]` | Cache + dedup |
| 14 | Own profile | postcli (`profile me`) | `profile me` | + multi-pub summary |
| 15 | Get other profile | postcli (`profile get`) | `profile get <handle>` | Cache |
| 16 | Create draft (rich text) | ty13r (`create_formatted_post`) | `drafts create --title --body --md\|--html` | Markdown + paywall markers |
| 17 | Update draft | ty13r (`update_post`) | `drafts update <id>` | `--stdin` body |
| 18 | Publish draft | ty13r (`publish_post`) | `drafts publish <id>` | `--dry-run` |
| 19 | List drafts | ty13r (`list_drafts`) | `drafts list [--publication]` | Cross-pub view |
| 20 | List published | ty13r (`list_published`) | `posts list --status published` | Filterable |
| 21 | Get post content | ty13r (`get_post_content`) | `posts get <slug> --content` | MD/HTML/text |
| 22 | Duplicate post (single-pub) | ty13r (`duplicate_post`) | `drafts duplicate <id>` | Same-pub clone |
| 23 | Upload image | ty13r (`upload_image`) | `images upload <file>` | Returns CDN URL |
| 24 | Author preview link | ty13r (`preview_draft`) | `drafts preview <id>` | Open in browser |
| 25 | Get sections | ty13r (`get_sections`) | `sections list [--publication]` | Per-pub |
| 26 | Subscriber count | ty13r (`get_subscriber_count`) | `subscribers count [--publication]` | Cross-pub aggregate |
| 27 | Delete draft | ty13r (`delete_draft`) | `drafts delete <id>` | `--dry-run` |
| 28 | Subscribers list/export | alvarolorentedev | `subscribers list [--publication] --csv\|--json` | + paid/free split |
| 29 | Add subscriber | postcli automations preset | `subscribers add <email>` | `--dry-run` |
| 30 | Free subs export | sbstck-dl style | `subscribers export --tier free [--publication]` | CSV |
| 31 | Paid subs export | sbstck-dl style | `subscribers export --tier paid [--publication]` | CSV |
| 32 | Download post (MD) | sbstck-dl (`download`) | `posts download <slug> --format md` | Image download |
| 33 | Download post (HTML/text) | sbstck-dl | `posts download <slug> --format html\|text` | |
| 34 | Download images with post | sbstck-dl (`--download-images`) | `posts download <slug> --images` | |
| 35 | Download file attachments | sbstck-dl (`--download-files`) | `posts download <slug> --files` | |
| 36 | Date-filtered list | sbstck-dl (`--after`/`--before`) | `posts list --after YYYY-MM-DD --before YYYY-MM-DD` | |
| 37 | Bulk archive publication | sbstck-dl | `archive <publication-or-subdomain>` | All posts → MD |
| 38 | Search posts by query | NHagar (`newsletter search`) | `search "term" [--publication]` | FTS over local SQLite |
| 39 | List categories | NHagar (`categories`) | `categories list` | |
| 40 | Newsletters in category | NHagar (`category newsletters`) | `categories newsletters <id>` | |
| 41 | Search publications globally | core endpoint | `publications search "query"` | Public, no auth |
| 42 | Auth via Chrome cookie | postcli (`auth login`) | `auth login --chrome` | Reads `connect.sid`+`substack.sid` from Chrome SQLite |
| 43 | Auth via manual cookie paste | postcli (`auth setup`) | `auth set-cookie` | |
| 44 | Auth verification | postcli (`auth test`) | `doctor` + `auth status` | |
| 45 | Get user subscriptions | NHagar (`user subscriptions`) | `me subscriptions` | |
| 46 | Author follow list | postcli implicit | `me follows` | |
| 47 | Get podcast audio | NHagar (`get_podcasts`) | `posts download <slug> --media` | M4A audio |
| 48 | Get recommendations | NHagar (`get_recommendations`) | `me recommendations` | |
| 49 | Sections per publication | jakub-k-slys SDK | `sections list --publication <sub>` | |
| 50 | Like-back automation | postcli (`auto create --preset 1`) | `auto run like-back` | One-shot, no daemon |
| 51 | Auto-reply automation | postcli automation | `auto run reply-to-likes --text "..."` | One-shot |
| 52 | Auto-restack automation | postcli automation | `auto run restack-recent` | One-shot |
| 53 | Follow-back automation | postcli automation | `auto run follow-back` | One-shot |
| 54 | Engagement stats | various | `posts stats <id>` | + history from cache |
| 55 | Output JSON + `--select` | every tool | global `--json --select dotted.path` | dotted field filter |
| 56 | Sync resource | (our own) | `sync [--full]` | Refresh all SQLite tables |

Note: TUI from postcli is intentionally NOT absorbed (CLI + MCP only).

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Portfolio dashboard | `portfolio` | 9/10 | Aggregates subscriber count, posts, drafts, scheduled per publication from SQLite cache into one ASCII table | Brief Top Workflow #1 (3-pub unified terminal), no competitor, persona 1 weekly need |
| 2 | Cross-publication twin | `posts twin <slug> --to <pub>` | 8/10 | Reads source post, re-uploads images via `/api/v1/image`, POSTs `/api/v1/drafts` against target subdomain with preserved paywall markers + section mapping | Brief Top Workflow #3 (cross-pub reuse, EN↔DE), no tool does cross-pub; ty13r duplicate is single-pub |
| 3 | Subscriber churn diff | `subscribers churn [--publication] [--since 7d]` | 9/10 | Diffs two SQLite snapshots of `/api/v1/subscriber/list`: classifies rows as new/unsubscribed/upgraded/downgraded | Brief Top Workflow #2, Substack ships no diff view, persona 2 Sunday ritual |
| 4 | Cross-publication subscriber join | `subscribers cross-sell` | 8/10 | SQL join across `subscribers.publication_id`: paid on A but free/absent on B/C, sorted by paid LTV | Persona 2 explicit need, brief "killer move", no competitor |
| 5 | Best posts cross-pub | `posts best [--by views\|likes\|comments\|restacks] [--window 30d] [--cross-pub]` | 7/10 | Ranks cached post stats by chosen metric, optionally grouping across all 3 publications | Brief Top Workflow #3, persona 1 weekly, no cross-pub competitor |
| 6 | Bilingual pair tracker | `posts pair <en> <de>` / `posts pairs [--missing]` | 7/10 | Local `post_pairs` table; `--missing` lists posts without a recorded twin | Brief User Vision (EN+DE MacroView↔MakroSicht), persona 1 weekly, unique to bilingual owners |
| 7 | Full-text grep across years | `grep "query" [--scope] [--publication] [--since]` | 8/10 | FTS5 virtual table over posts.body_markdown + notes.body + comments.body with bm25 ranking + snippet | Brief Top Workflow #5, persona 3 Friday ritual, Substack search is shallow |
| 8 | Scheduled posts board | `schedule board` | 6/10 | Reads `scheduled_at` column from cached posts/drafts across all pubs, renders next-30-day ASCII calendar | post_management endpoints, persona 1 weekly planning, no cross-pub competitor |

## Sources Cited
- postcli/substack (16 MCP tools, CLI, TUI, automations)
- ty13r/substack-mcp-plus (12 tools, rich text)
- jakub-k-slys/substack-api v4.0.0 (TS SDK, entity-based)
- NHagar/substack_api (Python SDK + CLI, categories, podcasts)
- alexferrari88/sbstck-dl (Go, download/archive, image+file support, date filtering)
- alvarolorentedev/substack-cli (subscriber CSV/JSON)
- anshulkhare7/substack-cli (drafts + publish + subscribers)
- marcomoauro/substack-mcp, michalnaka/mcp-substack, dkyazzentwatwa/substack_mcp (MCP variants)
- iam.slys.dev reverse-engineering writeup (endpoint catalog)
