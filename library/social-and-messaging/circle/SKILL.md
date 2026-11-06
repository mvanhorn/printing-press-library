---
name: pp-circle
description: "Run a Circle community from the terminal: spaces as files, courses as files, seat counts, and an offline mirror. Trigger phrases: `did enough people pay for the circle course`, `how many founding seats are left on circle`, `push the course to circle`, `pull the circle course into files`, `apply this spaces file to circle`, `what needs me in the circle community`, `onboard these people to circle`, `schedule the next show and tell`, `draft the show and tell recap`, `use circle`, `run circle-pp-cli`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - circle-pp-cli
---

# Circle — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `circle-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install circle --cli-only
   ```
2. Verify: `circle-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.10 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/cmd/circle-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Circle Admin API V2 endpoint, plus commands built for operators who run a paid community and a course on Circle. Declare spaces in a file and apply only the diff, keep a course as markdown folders, count founding seats, onboard members without invite emails, and triage what needs you from a local SQLite mirror.

## When to Use This CLI

Use this CLI when an agent needs to set up or run a Circle community through the Admin API: create or update spaces from a file, push a course from markdown, onboard members, count paid seats, schedule recurring events, or triage new activity. It suits scripted and agent use where every write should be previewed first.

## Anti-triggers

Do not use this CLI for:
- Creating a paywall, price, or coupon: the Admin API has no create endpoint, use the Circle admin UI
- Reading lesson completion or progress: the API only writes progress
- Member-facing features in your own app: use Circle's Headless Member API
- Sending DMs or announcement emails without a person approving them

## Unique Capabilities

### Live sessions and triage
- **`inbox`** — See new members, unreplied posts, flagged content, form submissions, failed charges and cancellations since a time, read from the local mirror.

  _Run sync first: inbox reads only the local mirror, and an empty or missing mirror returns empty sections with exit 0, not an all-clear._

  ```bash
  circle-pp-cli inbox --since 24h --agent
  ```
- **`show-tell schedule`** — Create the next Show & Tell events on a cadence by duplicating a template event.

  _Prints the series plan by default; --yes creates the occurrences as quiet drafts and records them so reruns skip dates already scheduled._

  ```bash
  circle-pp-cli show-tell schedule --template 123 --every 2w --until 2027-01-17
  ```
- **`show-tell recap`** — Pull a past session's RSVPed attendees and its live-room transcript into a markdown recap draft.

  _A transcript needs a live-room recording near the event time; otherwise the recap says none was found (transcript_found false). --post --yes saves it as a draft post._

  ```bash
  circle-pp-cli show-tell recap 123 --agent
  ```

### Community structure as files
- **`spaces apply`** — Make Circle's space groups and spaces match a YAML file, with a plan printed before any write.

  _Use this to set up or change community structure repeatably; rerunning an applied file is a no-op._

  ```bash
  circle-pp-cli spaces apply ./spaces.yaml --agent
  ```
- **`course push`** — Upload a course kept as module folders and markdown lessons to a Circle course space, new lessons as drafts.

  _Prints the plan by default; add --yes to push. New lessons land as drafts, and lesson order is tracked in course.yaml because Circle's API does not expose it._

  ```bash
  circle-pp-cli course push ./course --space 1000969 --agent
  ```
- **`course pull`** — Download a Circle course space into module folders with one file per lesson; lesson bodies stay as Circle's HTML and media is not downloaded.

  _Use this to back up a course or hand it to an agent to edit as files; files are numbered in creation order because Circle's API does not expose lesson order._

  ```bash
  circle-pp-cli course pull ./course --space 1000969 --agent
  ```

### Seats and members
- **`seats`** — Count paying members against a seat cap and list who paid.

  _Use this to see how many founding seats are left; active, trialing and past-due subscriptions plus paid charges count as paid._

  ```bash
  circle-pp-cli seats --cap 30 --agent
  ```
- **`onboard`** — Add people to the community, their spaces, space groups and access groups, and tag them, without sending invite emails by default.

  _Prints the plan by default; --yes applies it. No invite email goes out unless you add --send together with --yes._

  ```bash
  circle-pp-cli onboard ada@example.com --tag founding
  ```
- **`founding`** — Answer whether at least N people paid with GO or NO-GO and the count.

  _Use this for the launch decision. Branch on the verdict field: exit 2 is also used for usage errors. Trialing and past-due subscriptions count as paid._

  ```bash
  circle-pp-cli founding --go-no-go 10 --agent
  ```

## Command Reference

**access-groups** — Manage access groups

- `circle-pp-cli access-groups create` — Creates an active access group in the community with a name and an optional description.
- `circle-pp-cli access-groups delete` — Archives an access group; the record is kept and can be restored by unarchiving it.
- `circle-pp-cli access-groups list` — Lists the community's access groups as a paginated list.
- `circle-pp-cli access-groups update` — Updates the name and/or description of an access group.

**advanced-search** — Manage advanced search

- `circle-pp-cli advanced-search` — Searches the community by keyword across members, spaces, posts, comments, lessons and events.

**chat-preferences** — Manage chat preferences

- `circle-pp-cli chat-preferences` — Updates the community's chat settings for messaging, group messaging, voice messages and member-to-member messaging.

**chat-room-message-imports** — Manage chat room message imports

- `circle-pp-cli chat-room-message-imports` — Imports a historical message into a chat space on behalf of the specified user.

**comments** — Manage comments

- `circle-pp-cli comments create` — Creates a comment on a post, or a reply when parent_comment_id is given.
- `circle-pp-cli comments delete` — Permanently deletes a comment by ID. Replies to the comment are deleted with it.
- `circle-pp-cli comments get` — Returns a single comment by ID, including its replies, author, post and space.
- `circle-pp-cli comments list` — Lists comments in the community, newest first, optionally filtered by post or space. Results are paginated.

**community** — Manage community

- `circle-pp-cli community list` — Gets the community the API token belongs to, including its general, access, landing page
- `circle-pp-cli community update` — Updates the community the API token belongs to.

**community-leads** — Manage community leads

- `circle-pp-cli community-leads create` — Creates a non-member contact (also called a lead) in the community from an email address.
- `circle-pp-cli community-leads delete` — Deletes a non-member contact (also called a lead) from the community's audience.

**community-member-charges** — Manage community member charges

- `circle-pp-cli community-member-charges export` — Creates a background task that exports community member charges matching the given filters to a CSV
- `circle-pp-cli community-member-charges list` — Searches community member charges for a community

**community-member-spaces** — Manage community member spaces

- `circle-pp-cli community-member-spaces` — Lists the spaces a community member belongs to, as a paginated list.

**community-member-subscriptions** — Manage community member subscriptions

- `circle-pp-cli community-member-subscriptions export` — Creates a background task that exports community member subscriptions matching the given filters to a CSV
- `circle-pp-cli community-member-subscriptions list` — Searches community member subscriptions for a community

**community-members** — Manage community members

- `circle-pp-cli community-members create` — Creates a community member with the given email.
- `circle-pp-cli community-members delete` — Removes a member from the community by deactivating (soft-deleting)
- `circle-pp-cli community-members get` — Gets a single community member by ID.
- `circle-pp-cli community-members list` — Lists the community's members as a paginated list.
- `circle-pp-cli community-members list-communitymembers` — Finds a single community member by exact email address, including invited members who have not completed profile setup.
- `circle-pp-cli community-members update` — Updates a community member's profile, avatar, profile fields, moderation flag and preferences

**community-reviews** — Manage community reviews

- `circle-pp-cli community-reviews get` — Retrieves a single community review by id.
- `circle-pp-cli community-reviews list` — Lists the community's reviews, newest first by default.
- `circle-pp-cli community-reviews list-communityreviews` — Returns the community's precomputed review totals: review count, average rating, and the rating distribution.

**community-segments** — Manage community segments

- `circle-pp-cli community-segments create` — Creates a community segment
- `circle-pp-cli community-segments delete` — Permanently deletes a community segment.
- `circle-pp-cli community-segments list` — Lists the segments shown on the community's Segments page, newest first
- `circle-pp-cli community-segments update` — Updates an existing community segment

**contacts** — Manage contacts


**course-lesson-progress** — Manage course lesson progress

- `circle-pp-cli course-lesson-progress` — Marks a course lesson as completed or incomplete for a member of the lesson's course space, identified by email.

**course-lessons** — Manage course lessons

- `circle-pp-cli course-lessons create` — Creates a lesson in a course section. section_id is required.
- `circle-pp-cli course-lessons delete` — Permanently deletes a course lesson, including members' completion records for it.
- `circle-pp-cli course-lessons get` — Gets a single lesson from one of the community's course spaces by ID.
- `circle-pp-cli course-lessons list` — Lists lessons across the community's course spaces as a paginated list, oldest first by default.
- `circle-pp-cli course-lessons reorder` — Reorders a course's sections and lessons, and moves lessons between sections.
- `circle-pp-cli course-lessons update` — Updates a course lesson. Use rich_text_body instead of body_html for rich text content.

**course-sections** — Manage course sections

- `circle-pp-cli course-sections create` — Creates a section in a course space. The section is added at the end of the course and starts with no lessons.
- `circle-pp-cli course-sections delete` — Permanently deletes a course section together with all of its lessons, and removes it from the course's section order.
- `circle-pp-cli course-sections get` — Gets a single section from one of the community's course spaces by ID.
- `circle-pp-cli course-sections list` — Lists sections across the community's course spaces as a paginated list, oldest first by default.
- `circle-pp-cli course-sections update` — Renames a course section.

**direct-uploads** — Manage direct uploads

- `circle-pp-cli direct-uploads` — Creates a presigned URL for file upload. Upload the file via PUT to the returned direct_upload.

**embeds** — Manage embeds

- `circle-pp-cli embeds create` — Creates an embed from a URL and returns the embed data with an sgid
- `circle-pp-cli embeds get` — Retrieves an embed by its sgid

**event-attendees** — Manage event attendees

- `circle-pp-cli event-attendees create` — Invites a community member, identified by email, to an event and adds them to the event's space if needed.
- `circle-pp-cli event-attendees delete` — Removes a member's RSVP from an event by setting their RSVP status to no.
- `circle-pp-cli event-attendees list` — Lists the confirmed attendees of an event (members who RSVPed yes), with each member's name, email and RSVP status.

**events** — Manage events

- `circle-pp-cli events create` — Creates an event in an event-type space. space_id must be provided both at the top level and inside the event object.
- `circle-pp-cli events delete` — Permanently deletes an event by ID, along with its attendees and comments.
- `circle-pp-cli events get` — Gets an event by ID, including its schedule, location, RSVP and notification settings, and its space.
- `circle-pp-cli events list` — Lists published events in the community, newest first by default.
- `circle-pp-cli events update` — Updates an event by ID. The top-level space_id must be the space the event currently belongs to.

**filter-kit-configuration** — Manage filter kit configuration

- `circle-pp-cli filter-kit-configuration` — Returns the resolved runtime filter configuration members currently see, including defaults and saved customizations.

**filter-kit-controls** — Manage filter kit controls

- `circle-pp-cli filter-kit-controls create` — Creates one saved filter customisation for the member directory or a member space.
- `circle-pp-cli filter-kit-controls delete` — Deletes one member filter kit control
- `circle-pp-cli filter-kit-controls get` — Retrieves a single member directory or member space filter customisation by ID.
- `circle-pp-cli filter-kit-controls list` — Lists a community's saved filter customisations, including whether each filter is enabled and the member directory
- `circle-pp-cli filter-kit-controls update` — Updates an existing member directory or member space filter customisation.

**flagged-contents** — Manage flagged contents

- `circle-pp-cli flagged-contents create` — Reports a post, comment, connection or chat message for moderation as the API token's user
- `circle-pp-cli flagged-contents list` — Lists moderation reports for content in the community, most recently updated first

**forms** — Manage forms

- `circle-pp-cli forms delete` — Permanently deletes a form along with its elements and submissions.
- `circle-pp-cli forms get` — Returns a form by ID with its settings, styles, submission count and element tree.
- `circle-pp-cli forms list` — Lists the community's forms, newest first, optionally filtered by a case-insensitive partial match on name.
- `circle-pp-cli forms update` — Updates a form's settings and styles, and its elements when a non-empty elements list is sent.

**gamification** — Manage gamification

- `circle-pp-cli gamification` — Returns the community's gamification leaderboard: up to 10 active members ranked by points

**invitation-links** — Manage invitation links

- `circle-pp-cli invitation-links create` — Creates an active invitation link.
- `circle-pp-cli invitation-links delete` — Delete an invitation link by setting its status to deleted
- `circle-pp-cli invitation-links list` — Lists the community's invitation links, newest first
- `circle-pp-cli invitation-links update` — Updates an invitation link.

**live** — Manage live

- `circle-pp-cli live get` — Returns the raw VTT transcripts for a given live room.
- `circle-pp-cli live list` — Returns a paginated list of live rooms for the community.

**locations** — Manage locations

- `circle-pp-cli locations` — Resolves a free-text location query into geocoded candidates via Mapbox.

**member-tags** — Manage member tags

- `circle-pp-cli member-tags create` — Creates a member tag in the community. Tags are labels admins apply to members to organize and segment them.
- `circle-pp-cli member-tags delete` — Permanently deletes a member tag and removes it from every member who has it.
- `circle-pp-cli member-tags get` — Gets a single member tag by ID, including its appearance and display settings.
- `circle-pp-cli member-tags list` — Lists the community's member tags as a paginated list, oldest first by default.
- `circle-pp-cli member-tags update` — Updates the name, appearance or visibility of a member tag. Only the fields provided are changed.

**messages** — Manage messages

- `circle-pp-cli messages` — Sends a chat message from the authenticated community member.

**page-profile-fields** — Manage page profile fields

- `circle-pp-cli page-profile-fields` — Get Page Profile Fields

**payment-method-setting** — Manage payment method setting

- `circle-pp-cli payment-method-setting` — Returns the enabled/disabled state and Stripe capability status for every supported payment method (Apple Pay

**paywall-affiliate-payouts** — Manage paywall affiliate payouts

- `circle-pp-cli paywall-affiliate-payouts export-payouts` — Creates a background task that exports processing affiliate payouts to a CSV
- `circle-pp-cli paywall-affiliate-payouts mark-paid` — Moves processing affiliate payouts to paid, optionally scoped to specific community members
- `circle-pp-cli paywall-affiliate-payouts start` — Moves due affiliate payouts to processing, optionally scoped to specific community members

**paywall-affiliates** — Manage paywall affiliates

- `circle-pp-cli paywall-affiliates create` — Invites existing community members to the affiliate program
- `circle-pp-cli paywall-affiliates list` — Searches affiliates for a community
- `circle-pp-cli paywall-affiliates update` — Updates an affiliate's status

**paywall-coupons** — Manage paywall coupons

- `circle-pp-cli paywall-coupons <id>` — Deletes a paywall coupon. Removes any Stripe-side coupon and invalidates offers that reference it.

**paywall-groups** — Manage paywall groups

- `circle-pp-cli paywall-groups create` — Creates a new subscription group (paywall group) for the community.
- `circle-pp-cli paywall-groups update` — Updates a subscription group's name. The currency can only be changed while the group has no paywalls attached to it.

**paywalls** — Manage paywalls

- `circle-pp-cli paywalls delete` — Permanently deletes a paywall. Fails if the paywall has real transactions -- archive it instead.
- `circle-pp-cli paywalls list` — Search a community's paywalls and their prices. Only authorized community admins may search.

**posts** — Manage posts

- `circle-pp-cli posts create` — Creates a basic post in a basic-type space. Use tiptap_body instead of body_html for rich text content.
- `circle-pp-cli posts delete` — Removes a basic post by ID from its space and unpins it if it was the pinned post.
- `circle-pp-cli posts get` — Returns a single basic post by ID, including its content, status, settings and engagement counts.
- `circle-pp-cli posts list` — Lists basic posts in the community, optionally filtered by space, space group, status or a search on the post name.
- `circle-pp-cli posts update` — Updates a basic post by ID.

**profile-fields** — Manage profile fields

- `circle-pp-cli profile-fields create` — Creates a new custom profile field for the community.
- `circle-pp-cli profile-fields delete` — Permanently deletes an archived custom profile field.
- `circle-pp-cli profile-fields list` — Lists the community's profile fields as a paginated list.
- `circle-pp-cli profile-fields update` — Updates an existing custom profile field. Only the attributes sent are changed; anything omitted is left as it is.

**settings** — Manage settings

- `circle-pp-cli settings list` — Returns the current community's Connect settings — the admin settings that control member-to-member interactions.
- `circle-pp-cli settings update` — Updates the current community's Connect settings — the admin settings that control member-to-member interactions.

**space-group-member** — Manage space group member

- `circle-pp-cli space-group-member` — Returns the space group membership of a user, identified by email, in a space group.

**space-group-members** — Manage space group members

- `circle-pp-cli space-group-members create` — Adds a community member, identified by email, to a space group and to every space in the group.
- `circle-pp-cli space-group-members delete` — Removes a user, identified by email, from a space group and from every space in the group.
- `circle-pp-cli space-group-members list` — List Space Group Members

**space-groups** — Manage space groups

- `circle-pp-cli space-groups create` — Creates a space group, the sidebar section that spaces are organized under.
- `circle-pp-cli space-groups delete` — Deletes a space group by ID together with every space in it.
- `circle-pp-cli space-groups get` — Returns a single space group by ID with its settings.
- `circle-pp-cli space-groups list` — Lists the space groups in the community, newest first, optionally filtered by name. Results are paginated.
- `circle-pp-cli space-groups update` — Updates the name, slug and settings of a space group by ID.

**space-member** — Manage space member

- `circle-pp-cli space-member` — Returns the space membership of a user, identified by email, in a space.

**space-members** — Manage space members

- `circle-pp-cli space-members create` — Adds a person to a space by email.
- `circle-pp-cli space-members delete` — Removes a community member, identified by email, from a space. They stay a member of the community.
- `circle-pp-cli space-members list` — Lists the members of a space, optionally filtered by status. Results are paginated.

**spaces** — Manage spaces

- `circle-pp-cli spaces create` — Creates a space. space_type defaults to 'basic' if not provided. Use course_setting for course-type spaces.
- `circle-pp-cli spaces delete` — Deletes a space by ID. The space is soft-deleted right away and its content is soft-deleted in a background job.
- `circle-pp-cli spaces get` — Returns a single space by ID, including its settings, space group, topics and post IDs.
- `circle-pp-cli spaces list` — Lists the spaces in the community. Results are paginated.
- `circle-pp-cli spaces update` — Updates the settings of a space by ID. Only the fields sent are changed; space_type cannot be changed.

**tagged-members** — Manage tagged members

- `circle-pp-cli tagged-members create` — Applies a member tag to the community member with the given email.
- `circle-pp-cli tagged-members delete` — Removes a member tag from the community member with the given email.
- `circle-pp-cli tagged-members get` — Gets a single tagged member record, the assignment of a member tag to a community member, by its ID.
- `circle-pp-cli tagged-members list` — Lists member tag assignments in the community, newest first, as a paginated list.

**tax-setting** — Manage tax setting

- `circle-pp-cli tax-setting list` — Returns the community's Stripe Tax settings, refreshed from a live Stripe read.
- `circle-pp-cli tax-setting update` — Updates the community's Stripe Tax platform status and/or default tax behavior.

**topics** — Manage topics

- `circle-pp-cli topics create` — Creates a topic, a label members can add to posts, and optionally assigns it to spaces.
- `circle-pp-cli topics delete` — Permanently deletes a topic by ID. The topic is removed from its posts and spaces in background jobs.
- `circle-pp-cli topics get` — Returns a single topic by ID.
- `circle-pp-cli topics list` — Lists the topics in the community, optionally filtered by name and sorted. Results are paginated.
- `circle-pp-cli topics update` — Updates a topic by ID.

**workflows** — Manage workflows

- `circle-pp-cli workflows get` — Gets a single workflow by its UUID, including its trigger and all action steps in execution order.
- `circle-pp-cli workflows list` — Lists the community's automations/workflows, newest first, paginated.


**course**


- `circle-pp-cli course push` — Upload a course kept as module folders and markdown lessons to a Circle course space, new lessons as drafts.

- `circle-pp-cli course pull` — Download a Circle course space into module folders with one file per lesson; lesson bodies stay as Circle's HTML and media is not downloaded.


**founding**


- `circle-pp-cli founding` — Answer whether at least N people paid with GO or NO-GO and the count.


**inbox**


- `circle-pp-cli inbox` — See new members, unreplied posts, flagged content, form submissions, failed charges and cancellations since a time, read from the local mirror.


**onboard**


- `circle-pp-cli onboard` — Add people to the community, their spaces, space groups and access groups, and tag them, without sending invite emails by default.


**seats**


- `circle-pp-cli seats` — Count paying members against a seat cap and list who paid.


**show-tell**


- `circle-pp-cli show-tell schedule` — Create the next Show & Tell events on a cadence by duplicating a template event.

- `circle-pp-cli show-tell recap` — Pull a past session's RSVPed attendees and its live-room transcript into a markdown recap draft.


**spaces**


- `circle-pp-cli spaces apply` — Make Circle's space groups and spaces match a YAML file, with a plan printed before any write.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
circle-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Preview a structure change

```bash
circle-pp-cli spaces apply ./spaces.yaml
```

Prints the create and update plan; add --apply to make the changes.

### Push course edits as drafts

```bash
circle-pp-cli course push ./course --space 1000969
```

Prints which sections and lessons would be created or updated; re-run with --yes to push them as drafts.

### Launch decision

```bash
circle-pp-cli founding --go-no-go 10 --agent
```

Exit 0 means at least 10 people paid; exit 2 means not yet.

### Narrow a member listing

```bash
circle-pp-cli community-members list --agent --select records.email,records.name,records.created_at
```

Keeps only the fields an agent needs from a large member payload.

### Declare spaces in a file

```bash
circle-pp-cli spaces apply ./spaces.yaml --apply
```

spaces.yaml lists space_groups and spaces (name, slug, group, type, privacy flags); without --apply it only prints the plan.

## Auth Setup

Circle admin tokens come from Circle admin > Developers > Tokens. Export the token as `CIRCLE_API_TOKEN`; the CLI sends it as `Authorization: Token <token>`. Admin API calls count against your plan's monthly request quota (5,000 to 250,000 a month by plan), so read from the local mirror after `sync` where you can.

Run `circle-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  circle-pp-cli access-groups list --agent
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `CIRCLE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CIRCLE_CONFIG_DIR`, `CIRCLE_DATA_DIR`, `CIRCLE_STATE_DIR`, `CIRCLE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CIRCLE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `circle-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "circle": {
        "command": "circle-pp-mcp",
        "env": {
          "CIRCLE_HOME": "/srv/circle"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CIRCLE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CIRCLE_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `CIRCLE_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `CIRCLE_DATA_SOURCE` sets the `--data-source` default.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
circle-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "circle-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `circle-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `circle-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `circle-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
circle-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
circle-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
circle-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
circle-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`circle-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `CIRCLE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
circle-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
circle-pp-cli feedback --stdin < notes.txt
circle-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CIRCLE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CIRCLE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
circle-pp-cli profile save briefing --json
circle-pp-cli --profile briefing access-groups list
circle-pp-cli profile list --json
circle-pp-cli profile show briefing
circle-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `circle-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add circle-pp-mcp -- circle-pp-mcp
```

Verify: `claude mcp list`

For a remote host, `circle-pp-mcp --transport http` requires `CIRCLE_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `CIRCLE_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `CIRCLE_MCP_READ_ONLY=1` drops mirrored write commands (local-write commands that only touch the CLI's own learn store stay) and makes the raw execute tool refuse state-changing calls.

## Direct Use

1. Check if installed: `which circle-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   circle-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `circle-pp-cli <command> --help`.
