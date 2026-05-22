# Reddit CLI — Absorb Manifest

Run ID: 20260522-081137

## Tools surveyed

| Tool | Repo | Stars | Type | Features absorbed |
|------|------|-------|------|-------------------|
| jordanburke/reddit-mcp-server | TS + PRAW | 100 | MCP (write-capable) | 9 read + 6 write tools |
| Arindam200/reddit-mcp | Python PRAW | 286 | MCP | 9 read tools |
| Hawstein/mcp-server-reddit | Python | 177 | MCP (no auth) | 7 listing tools |
| adhikasp/mcp-reddit | Python | 401 | MCP | ~6 tools (analysis-flavored) |
| PRAW (praw-dev) | Python lib | ~3k | SDK | ~140 endpoints, full surface |
| RedditWarp | Python lib | ~50 | SDK | type-safe alternative |
| vartanbeno/go-reddit | Go lib | 331 | SDK | service-oriented Go client |
| turnage/graw | Go lib | ~300 | SDK | event streams |
| snoowrap | JS lib | ~1k | SDK | dying; Reddit JS client |
| rtv (defunct) | Python CLI | ~4.5k | TUI (defunct 2019) | terminal browser |
| tuir (defunct) | Python CLI | ~700 | TUI (defunct) | rtv fork |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Status | Added Value |
|---|---------|-------------|-------------------|--------|-------------|
| 1 | Get frontpage hot/new/top/rising | Hawstein MCP `get_frontpage_posts` | `frontpage [--listing hot\|new\|top\|rising] [--limit N]` | live | --json, --select, SQLite cache, --since cursor |
| 2 | Subreddit listings (hot/new/top/rising/controversial) | Hawstein MCP `get_subreddit_<sort>_posts` | `sub <name> [--sort SORT] [--time T] [--limit N]` | live | offline cache + FTS5 |
| 3 | Subreddit about / stats | every MCP `get_subreddit_info` | `sub-about <name>` | live | full schema |
| 4 | Submission detail | jordanburke `get_reddit_post`, Arindam `get_submission_by_url/by_id` | `post get <id-or-url> [--comments] [--depth N]` | live | MoreComments auto-expand |
| 5 | Comments tree | every MCP `get_post_comments` | `post comments <id> [--depth N] [--threshold N] [--expand-all]` | live | depth+score thresholding, MoreComments resolution |
| 6 | Cross-Reddit + per-sub search | jordanburke `search_reddit`, Arindam `search_posts` | `search "<q>" [--sub X] [--sort] [--time] [--type link\|user\|sr]` | live | --json, cursor pagination |
| 7 | User about | every MCP `get_user_info` | `user about <username>` | live | --json |
| 8 | User submissions | jordanburke, Arindam `get_user_posts` | `user submitted <username> [--sort] [--time]` | live | cursor pagination |
| 9 | User comments | jordanburke, Arindam `get_user_comments` | `user comments <username> [--sort] [--time]` | live | --json |
| 10 | Trending subreddits | jordanburke, Arindam `get_trending_subreddits` | `subs trending` | live | --json |
| 11 | Submit post (link/self/crosspost) | jordanburke `create_post` | `submit <sub> --title T [--text\|--url\|--crosspost-of] [--flair F] [--nsfw] [--spoiler] [--oc] [--no-replies]` | live | --dry-run, idempotent |
| 12 | Reply to post/comment | jordanburke `reply_to_post` | `reply <parent-id> --text "..." [--distinguish]` | live | --dry-run, parent type auto-detected |
| 13 | Edit own post | jordanburke `edit_post` | `post edit <id> --text "..."` | live | --dry-run |
| 14 | Edit own comment | jordanburke `edit_comment` | `comment edit <id> --text "..."` | live | --dry-run |
| 15 | Delete own post | jordanburke `delete_post` | `post delete <id> [--confirm]` | live | --dry-run |
| 16 | Delete own comment | jordanburke `delete_comment` | `comment delete <id> [--confirm]` | live | --dry-run |
| 17 | Vote on thing | PRAW `.upvote()/.downvote()/.clear_vote()` | `vote <thing-id> --dir up\|down\|none` | live | --dry-run |
| 18 | Save / unsave | PRAW `.save()/.unsave()` | `save <id>` / `unsave <id>` | live | --dry-run |
| 19 | Hide / unhide | PRAW `.hide()/.unhide()` | `hide <id>` / `unhide <id>` | live | --dry-run |
| 20 | Own profile (`/api/v1/me`) | PRAW `reddit.user.me()` | `me` | live | --json |
| 21 | Own karma breakdown | PRAW `reddit.user.karma()` | `me karma` | live | per-sub breakdown |
| 22 | Own trophies | PRAW `redditor.trophies()` | `me trophies` | live | --json |
| 23 | Own subscriptions | PRAW `reddit.user.subreddits()` | `me subs` | live | paginated, --json |
| 24 | Own friends | PRAW `reddit.user.friends()` | `me friends` | live | --json |
| 25 | Own blocked | PRAW `reddit.user.blocked()` | `me blocked` | live | --json |
| 26 | Subscribe / unsubscribe | PRAW `subreddit.subscribe()` | `subscribe <sub>` / `unsubscribe <sub>` | live | batch via stdin, --dry-run |
| 27 | Multireddit CRUD | PRAW `reddit.multireddit.*` | `multi list\|get\|create\|update\|delete` | live | --dry-run, --json |
| 28 | Multireddit add/remove sub | PRAW `multi.add()/.remove()` | `multi add-sub <name> <sub>` / `multi remove-sub <name> <sub>` | live | --dry-run |
| 29 | Own saved | PRAW `redditor.saved()` | `me saved [--type link\|comment]` | live | paginated, sync to SQLite |
| 30 | Own upvoted | PRAW `redditor.upvoted()` | `me upvoted` | live | sync to SQLite |
| 31 | Own downvoted | PRAW `redditor.downvoted()` | `me downvoted` | live | sync to SQLite |
| 32 | Inbox / unread / mentions / messages / sent | PRAW `reddit.inbox.*` | `inbox [--filter all\|unread\|mentions\|messages\|sent]` | live | --json |
| 33 | Mark inbox read/unread | PRAW `message.mark_read()/.mark_unread()` | `inbox mark-read <ids...>` / `inbox mark-unread <ids...>` | live | batch |
| 34 | Mark all inbox read | PRAW `reddit.inbox.mark_all_read()` | `inbox mark-all-read` | live | --dry-run |
| 35 | Compose message | PRAW `redditor.message()` | `message send --to U --subject S --body B` | live | --dry-run |
| 36 | Modmail conversations | PRAW `subreddit.modmail.conversations()` | `modmail list <sub> [--filter new\|inprogress\|archived\|appeals]` | live | --json |
| 37 | Modmail reply | PRAW `convo.reply()` | `modmail reply <convo-id> --text B [--internal] [--private]` | live | --dry-run |
| 38 | Modqueue / reports / spam / edited | PRAW `subreddit.mod.modqueue/reports/spam/edited()` | `mod queue\|reports\|spam\|edited <sub>` | live | --since, --json |
| 39 | Approve / remove / distinguish | PRAW `submission.mod.*` | `mod approve <id>` / `mod remove <id> [--spam] [--reason ID]` / `mod distinguish <id> [--sticky]` | live | --dry-run |
| 40 | Lock / unlock / sticky / unsticky | PRAW `submission.mod.*` | `mod lock <id>` / `mod unlock <id>` / `mod sticky <id>` / `mod unsticky <id>` | live | --dry-run |
| 41 | Ban / unban | PRAW `subreddit.banned.add()/.remove()` | `mod ban <sub> <user> [--reason] [--days] [--note]` / `mod unban <sub> <user>` | live | --dry-run |
| 42 | Mute / unmute | PRAW `subreddit.muted.add()/.remove()` | `mod mute <sub> <user>` / `mod unmute <sub> <user>` | live | --dry-run |
| 43 | Mod log | PRAW `subreddit.mod.log()` | `mod log <sub> [--action] [--mod] [--since]` | live | --json |
| 44 | Removal reason templates | PRAW `subreddit.mod.removal_reasons` | `mod removal-reasons list\|create\|update\|delete <sub>` | live | --dry-run |
| 45 | Banned users list | PRAW `subreddit.banned()` | `mod banned <sub>` | live | --json |
| 46 | Subreddit rules | PRAW `subreddit.rules` | `sub rules <sub>` | live | --json |
| 47 | Subreddit traffic (mod) | PRAW `subreddit.traffic()` | `sub traffic <sub>` | live | --json |
| 48 | Flair templates | PRAW `subreddit.flair.link_templates / redditor_templates` | `flair templates <sub> [--type link\|redditor]` | live | --dry-run for CRUD |
| 49 | Set link flair | PRAW `submission.flair.select()` | `flair set <post-id> --template-id X [--text T]` | live | --dry-run |
| 50 | Set user flair | PRAW `subreddit.flair.set()` | `flair set-user <sub> <user> --text T [--css-class C]` | live | --dry-run |
| 51 | Wiki page get | PRAW `subreddit.wiki[name]` | `wiki get <sub> <page>` | live | --json (md + html) |
| 52 | Wiki page list | PRAW `subreddit.wiki.pages` | `wiki list <sub>` | live | --json |
| 53 | Wiki edit | PRAW `wikipage.edit()` | `wiki edit <sub> <page> --content C [--reason R]` | live | --dry-run |
| 54 | Wiki revisions | PRAW `wikipage.revisions()` | `wiki revisions <sub> <page>` | live | --json |
| 55 | Subreddit search | PRAW `reddit.subreddits.search()` | `subs search <q> [--sort relevance\|activity]` | live | --json |
| 56 | Popular / default / new subreddits | PRAW `reddit.subreddits.popular/default/new()` | `subs popular` / `subs default` / `subs new` | live | --json |
| 57 | Crosspost (single target) | PRAW `submission.crosspost()` | `crosspost <id> --to <sub> [--title T]` | live | --dry-run |
| 58 | Live stream new posts/comments | graw event streams | `stream sub <name> --type posts\|comments [--match REGEX]` | live | live streaming with backpressure |
| 59 | Friend / unfriend / block / unblock | PRAW `redditor.friend()/.unfriend()/.block()` | `friend add <user>` / `friend remove <user>` / `block <user>` / `unblock <user>` | live | --dry-run |
| 60 | Refresh OAuth token | PRAW auth manager | `auth refresh` | live | rotates bearer, persists |

Status legend: `live` = full implementation. `(stub)` = placeholder. No stubs in this run.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|--------------|---------|
| T1 | Stale modqueue ranker | `mod queue <sub> --sort age [--older-than 24h]` | 9/10 | Local timestamp math over modqueue payload; sorts by `created_utc` ascending; `--older-than` filters | Mara |
| T2 | Reporter reputation ledger | `mod reporters <sub> [--window 30d] [--min-reports 3]` | 9/10 | Joins synced `mod_actions` with reports payload locally; per-reporter (filed, removed%, approved%, no-action%) | Mara |
| T3 | Ghost mod-action / split detector | `mod ghost-actions <sub> [--since 7d]` | 7/10 | Temporal join on synced modlog for `(target_id, action, mod)` chains; flags approve→remove reversals | Mara |
| T4 | Cross-sub user dossier | `user dossier <username> [--in sub1,...]` | 9/10 | Aggregates `/user/<u>/about` + `submitted` + `comments` + karma; groups per-sub locally | Mara, Riko |
| T5 | Personal FTS5 search | `me search "<q>" [--scope saved,submitted,upvoted,comments]` | 10/10 | SQLite FTS5 over synced own-history submissions(title, selftext) + comments(body) | Devi, Riko |
| T6 | Sub-scoped local FTS5 search | `search-local "<q>" --sub <name> [--type submissions,comments]` | 9/10 | Same FTS5 engine, scoped to `sync sub <name>` corpus | Riko, Syauqi |
| T7 | Multi-sub brand watch with context | `watch "<term>" --in sub1,sub2,... [--since 24h] [--enrich-karma]` | 8/10 | Fan-out `/search` across N subs, dedupe by `t3_id`, enrich with parent comment + OP karma | Syauqi |
| T8 | Best-time-to-post analyzer | `me posting-stats [--sub X] [--by hour\|dow]` | 7/10 | Local aggregation of synced own-submissions by (sub × dow × hour-utc) → median + p75 | Devi |
| T9 | Plan-driven crosspost batch | `crosspost-batch <id> --plan plan.yaml [--dry-run]` | 8/10 | YAML lists per-sub overrides (title, flair-id, send-replies, nsfw); idempotent via plan-row hash | Devi |
| T10 | Modqueue remove-batch with templates | `mod remove-batch <sub> --plan plan.csv [--dry-run]` | 8/10 | CSV row → remove + removal_reason + optional ban + modmail in one call per item | Mara |
| T11 | Comment-velocity radar | `post velocity <id> [--baseline-sample 50]` | 6/10 | Comments/minute over first 60 min; compared to sample of recent sub posts for percentile | Devi, Syauqi |
| T12 | Inbox digest by sub + score | `inbox digest [--window 24h]` | 7/10 | Group inbox by source-sub, enrich with current thread score via `/api/info?id=t3_*` | Mara |
