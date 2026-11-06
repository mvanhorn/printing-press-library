# Circle CLI

**Run a Circle community from the terminal: spaces as files, courses as files, seat counts, and an offline mirror.**

Every Circle Admin API V2 endpoint, plus commands built for operators who run a paid community and a course on Circle. Declare spaces in a file and apply only the diff, keep a course as markdown folders, count founding seats, onboard members without invite emails, and triage what needs you from a local SQLite mirror.

Learn more at [Circle](https://circle.so).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `circle-pp-cli` binary and the `pp-circle` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install circle
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install circle --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install circle --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install circle --agent claude-code
npx -y @mvanhorn/printing-press-library install circle --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/circle-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install circle --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-circle --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-circle --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install circle --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/circle-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CIRCLE_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "circle": {
      "command": "circle-pp-mcp",
      "env": {
        "CIRCLE_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`circle-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `CIRCLE_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `CIRCLE_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export CIRCLE_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
circle-pp-mcp --transport http
```

Set `CIRCLE_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Authentication

Circle admin tokens come from Circle admin > Developers > Tokens. Export the token as `CIRCLE_API_TOKEN`; the CLI sends it as `Authorization: Token <token>`. Admin API calls count against your plan's monthly request quota (5,000 to 250,000 a month by plan), so read from the local mirror after `sync` where you can.

## Quick Start

```bash
# Confirm the binary runs without calling Circle
circle-pp-cli doctor --dry-run

# Confirm CIRCLE_API_TOKEN reaches your community
circle-pp-cli doctor

# Mirror spaces, members, posts and payments locally
circle-pp-cli sync

# See what needs you, read from the mirror
circle-pp-cli inbox --since 7d

# Count paid founding seats
circle-pp-cli seats --cap 30

```

## Unique Features

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

## Usage

Run `circle-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CIRCLE_CONFIG_DIR`, `CIRCLE_DATA_DIR`, `CIRCLE_STATE_DIR`, or `CIRCLE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CIRCLE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CIRCLE_HOME=/srv/circle
circle-pp-cli doctor
```

Under `CIRCLE_HOME=/srv/circle`, the four dirs resolve to `/srv/circle/config`, `/srv/circle/data`, `/srv/circle/state`, and `/srv/circle/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:CIRCLE_HOME = "C:\srv\circle"
circle-pp-cli doctor
```

Under `$env:CIRCLE_HOME = "C:\srv\circle"`, the four dirs resolve to `C:\srv\circle\config`, `C:\srv\circle\data`, `C:\srv\circle\state`, and `C:\srv\circle\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `CIRCLE_DATA_DIR` overrides an explicit `--home` for that kind. Use `CIRCLE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CIRCLE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `circle-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `circle-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `circle-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `CIRCLE_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `CIRCLE_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### access-groups

Manage access groups

- **`circle-pp-cli access-groups create`** - Creates an active access group in the community with a name and an optional description. The name must be unique within the community.
- **`circle-pp-cli access-groups delete`** - Archives an access group; the record is kept and can be restored by unarchiving it.
Archiving is rejected while the group is still used by a paywall, invitation link, form,
onboarding or community setting, or is linked to newsletter posts. After archiving, access
granted through the group is revoked from its members asynchronously.
- **`circle-pp-cli access-groups list`** - Lists the community's access groups as a paginated list. Returns only active groups unless status is set to archived or all, and can be filtered by IDs or a case-insensitive partial name match.
- **`circle-pp-cli access-groups update`** - Updates the name and/or description of an access group. It does not change which spaces and space groups the group grants.

### advanced-search

Manage advanced search

- **`circle-pp-cli advanced-search`** - Searches the community by keyword across members, spaces, posts, comments, lessons and
events. The type parameter picks what to search (general searches members and spaces), and
filters narrow the results. Results are paginated and include highlighted matches.

### chat-preferences

Manage chat preferences

- **`circle-pp-cli chat-preferences`** - Updates the community's chat settings for messaging, group messaging, voice messages and member-to-member messaging. Only the settings provided are changed. Setting messaging_enabled to false also sets the other settings in the same request to false, except voice_messages_enabled.

### chat-room-message-imports

Manage chat room message imports

- **`circle-pp-cli chat-room-message-imports`** - Imports a historical message into a chat space on behalf of the specified user.
The user is automatically added as a space member and chat room participant if not already present.
The sent_at field is required and must be a past timestamp — this endpoint is intended exclusively
for importing historical messages. Notifications are never sent for imported messages.

### comments

Manage comments

- **`circle-pp-cli comments create`** - Creates a comment on a post, or a reply when parent_comment_id is given. The comment is
authored by the API token's user unless user_email or user_id names another
member. Its author is added to the post's space if not
already a member.
Notifications for the new comment are skipped unless skip_notifications is false.
- **`circle-pp-cli comments delete`** - Permanently deletes a comment by ID. Replies to the comment are deleted with it.
- **`circle-pp-cli comments get`** - Returns a single comment by ID, including its replies, author, post and space.
- **`circle-pp-cli comments list`** - Lists comments in the community, newest first, optionally filtered by post or space. Results are paginated.

### community

Manage community

- **`circle-pp-cli community list`** - Gets the community the API token belongs to, including its general, access, landing page, branding and digest settings and its community_setting feature toggles.
- **`circle-pp-cli community update`** - Updates the community the API token belongs to. The required community object holds general,
access, landing page, branding and digest settings; the optional community_setting object
holds feature toggles. Only the fields sent are changed.

### community-leads

Manage community leads

- **`circle-pp-cli community-leads create`** - Creates a non-member contact (also called a lead) in the community from an email address.

A non-member contact appears in the community's audience and can be reached by email marketing,
but has no login and no access to the community.

The email must not already belong to a member or to an existing non-member contact in this
community - both cases return a 422.
- **`circle-pp-cli community-leads delete`** - Deletes a non-member contact (also called a lead) from the community's audience.

This is a soft delete: the contact is removed from the audience and no longer appears in
listings, but is retained and is restored automatically if the same contact is re-added.
It only removes non-member contacts: pass the numeric id of a non-member contact in this
community. An id that belongs to a full community member (or to a contact in another
community) returns a 404 and deletes nothing.

### community-member-charges

Manage community member charges

- **`circle-pp-cli community-member-charges export`** - Creates a background task that exports community member charges matching the given filters to a CSV, emailed to the requesting admin
- **`circle-pp-cli community-member-charges list`** - Searches community member charges for a community

### community-member-spaces

Manage community member spaces

- **`circle-pp-cli community-member-spaces`** - Lists the spaces a community member belongs to, as a paginated list. Identify the member by community_member_id or user_email; when neither is given, returns the spaces of the member who owns the API token.

### community-member-subscriptions

Manage community member subscriptions

- **`circle-pp-cli community-member-subscriptions export`** - Creates a background task that exports community member subscriptions matching the given filters to a CSV, emailed to the requesting admin
- **`circle-pp-cli community-member-subscriptions list`** - Searches community member subscriptions for a community

### community-members

Manage community members

- **`circle-pp-cli community-members create`** - Creates a community member with the given email. Avatar requires using the direct_upload flow first (upload to presigned URL, then use signed_id and url). Removing a member from the community does not delete their Circle account, so creating the same email again re-adds the existing account.
- **`circle-pp-cli community-members delete`** - Removes a member from the community by deactivating (soft-deleting) their membership after canceling
their running paywall subscriptions. Their posts and comments are kept, their Circle account is not
deleted, and adding the same email again restores the membership. This is not a permanent
delete; permanently deleting a member and their content is a separate operation.
- **`circle-pp-cli community-members get`** - Gets a single community member by ID.
- **`circle-pp-cli community-members list`** - Lists the community's members as a paginated list. Returns active members by default; use status to include invited members who have not completed profile setup, and member_tag_ids to filter by member tags.
- **`circle-pp-cli community-members list-communitymembers`** - Finds a single community member by exact email address, including invited members who have not completed profile setup.
- **`circle-pp-cli community-members update`** - Updates a community member's profile, avatar, profile fields, moderation flag and preferences, and adds them to the given spaces, space groups and member tags. Existing space memberships and tags are not removed.

### community-reviews

Manage community reviews

- **`circle-pp-cli community-reviews get`** - Retrieves a single community review by id.
- **`circle-pp-cli community-reviews list`** - Lists the community's reviews, newest first by default. Supports filtering by rating or a free-text search of the review description, reviewer name, and tags, plus sorting by newest, highest, or lowest rating.
- **`circle-pp-cli community-reviews list-communityreviews`** - Returns the community's precomputed review totals: review count, average rating, and the rating distribution.

### community-segments

Manage community segments

- **`circle-pp-cli community-segments create`** - Creates a community segment
- **`circle-pp-cli community-segments delete`** - Permanently deletes a community segment. Deletion is blocked when the segment is referenced in another audience's filters or a bot's member filters.
- **`circle-pp-cli community-segments list`** - Lists the segments shown on the community's Segments page, newest first, optionally filtered by a case-insensitive partial match on title. Audiences built inside broadcasts or workflows are not included. Results are paginated.
- **`circle-pp-cli community-segments update`** - Updates an existing community segment

### contacts

Manage contacts


### course-lesson-progress

Manage course lesson progress

- **`circle-pp-cli course-lesson-progress`** - Marks a course lesson as completed or incomplete for a member of the lesson's course space, identified by email.

### course-lessons

Manage course lessons

- **`circle-pp-cli course-lessons create`** - Creates a lesson in a course section. section_id is required. Use rich_text_body instead of body_html for rich text content.
- **`circle-pp-cli course-lessons delete`** - Permanently deletes a course lesson, including members' completion records for it.
- **`circle-pp-cli course-lessons get`** - Gets a single lesson from one of the community's course spaces by ID.
- **`circle-pp-cli course-lessons list`** - Lists lessons across the community's course spaces as a paginated list, oldest first by default. Can be filtered by course space, section and status.
- **`circle-pp-cli course-lessons reorder`** - Reorders a course's sections and lessons, and moves lessons between sections. The new_order array must contain every course section in its final order, with every lesson listed under its destination section in its final order.
- **`circle-pp-cli course-lessons update`** - Updates a course lesson. Use rich_text_body instead of body_html for rich text content.

### course-sections

Manage course sections

- **`circle-pp-cli course-sections create`** - Creates a section in a course space. The section is added at the end of the course and starts with no lessons.
- **`circle-pp-cli course-sections delete`** - Permanently deletes a course section together with all of its lessons, and removes it from the course's section order.
- **`circle-pp-cli course-sections get`** - Gets a single section from one of the community's course spaces by ID.
- **`circle-pp-cli course-sections list`** - Lists sections across the community's course spaces as a paginated list, oldest first by default. Can be filtered by course space.
- **`circle-pp-cli course-sections update`** - Renames a course section.

### direct-uploads

Manage direct uploads

- **`circle-pp-cli direct-uploads`** - Creates a presigned URL for file upload. Upload the file via PUT to the returned direct_upload.url with the provided headers. The checksum is a base64-encoded MD5 hash of the file content.

### embeds

Manage embeds

- **`circle-pp-cli embeds create`** - Creates an embed from a URL and returns the embed data with an sgid
- **`circle-pp-cli embeds get`** - Retrieves an embed by its sgid

### event-attendees

Manage event attendees

- **`circle-pp-cli event-attendees create`** - Invites a community member, identified by email, to an event and adds them to the event's
space if needed. The member is RSVPed directly when the event auto-RSVPs invited attendees;
otherwise their status is invited and an invitation email is sent if the event is published.
- **`circle-pp-cli event-attendees delete`** - Removes a member's RSVP from an event by setting their RSVP status to no. The member is identified by email and the attendee record is kept.
- **`circle-pp-cli event-attendees list`** - Lists the confirmed attendees of an event (members who RSVPed yes), with each member's name, email and RSVP status. Results are paginated.

### events

Manage events

- **`circle-pp-cli events create`** - Creates an event in an event-type space. space_id must be provided both at the top level and inside the event object.
- **`circle-pp-cli events delete`** - Permanently deletes an event by ID, along with its attendees and comments. space_id must be the space the event belongs to.
- **`circle-pp-cli events get`** - Gets an event by ID, including its schedule, location, RSVP and notification settings, and its space.
- **`circle-pp-cli events list`** - Lists published events in the community, newest first by default. Filter by space_id, or by
a date range (send both filter_date[start_date] and filter_date[end_date]) that matches
events starting or ending within it. Results are paginated.
- **`circle-pp-cli events update`** - Updates an event by ID. The top-level space_id must be the space the event currently
belongs to. Only the fields sent are changed. For a recurring event, set event_type to
recurring and recurring_setting_attributes.edit_mode to update more than this occurrence.

### filter-kit-configuration

Manage filter kit configuration

- **`circle-pp-cli filter-kit-configuration`** - Returns the resolved runtime filter configuration members currently see, including defaults and saved customizations.

### filter-kit-controls

Manage filter kit controls

- **`circle-pp-cli filter-kit-controls create`** - Creates one saved filter customisation for the member directory or a member space. Use this to
hide a default filter, or use key "profile_field" with profile_field_id to show a profile field
as a filter. Omit space_id for the member directory.
- **`circle-pp-cli filter-kit-controls delete`** - Deletes one member filter kit control
- **`circle-pp-cli filter-kit-controls get`** - Retrieves a single member directory or member space filter customisation by ID.
- **`circle-pp-cli filter-kit-controls list`** - Lists a community's saved filter customisations, including whether each filter is enabled and the member directory, member space, or profile field it applies to. Results are ordered by sort key.
- **`circle-pp-cli filter-kit-controls update`** - Updates an existing member directory or member space filter customisation.

### flagged-contents

Manage flagged contents

- **`circle-pp-cli flagged-contents create`** - Reports a post, comment, connection or chat message for moderation as the API token's user,
with a reason category and explanation. Depending on the community's moderation settings,
the reported content may also be flagged for admin approval.
- **`circle-pp-cli flagged-contents list`** - Lists moderation reports for content in the community, most recently updated first, optionally filtered by review status (inbox for reports not yet reviewed). Results are paginated.

### forms

Manage forms

- **`circle-pp-cli forms delete`** - Permanently deletes a form along with its elements and submissions.
- **`circle-pp-cli forms get`** - Returns a form by ID with its settings, styles, submission count and element tree. Element IDs from this response are needed to update the form's elements and to submit answers.
- **`circle-pp-cli forms list`** - Lists the community's forms, newest first, optionally filtered by a case-insensitive partial match on name. Elements are not included; fetch a single form to get its elements. Results are paginated.
- **`circle-pp-cli forms update`** - Updates a form's settings and styles, and its elements when a non-empty elements list is sent. Unspecified settings remain unchanged, except that sending a non-empty elements list also publishes a draft form.

### gamification

Manage gamification

- **`circle-pp-cli gamification`** - Returns the community's gamification leaderboard: up to 10 active members ranked by points, for all time or the last 7 or 30 days. Requires gamification to be enabled for the community.

### invitation-links

Manage invitation links

- **`circle-pp-cli invitation-links create`** - Creates an active invitation link.
- **`circle-pp-cli invitation-links delete`** - Delete an invitation link by setting its status to deleted
- **`circle-pp-cli invitation-links list`** - Lists the community's invitation links, newest first, optionally filtered by a case-insensitive partial match on name or by status (active or revoked). Deleted links are not included. Results are paginated.
- **`circle-pp-cli invitation-links update`** - Updates an invitation link.

### live

Manage live

- **`circle-pp-cli live get`** - Returns the raw VTT transcripts for a given live room. Transcripts are sourced from the recording blob's media transcript via a polymorphic association.
- **`circle-pp-cli live list`** - Returns a paginated list of live rooms for the community.

### locations

Manage locations

- **`circle-pp-cli locations`** - Resolves a free-text location query into geocoded candidates via Mapbox. Read-only lookup used to resolve a place before setting a location profile field or filtering contacts by geo_location.

### member-tags

Manage member tags

- **`circle-pp-cli member-tags create`** - Creates a member tag in the community. Tags are labels admins apply to members to organize and segment them.
- **`circle-pp-cli member-tags delete`** - Permanently deletes a member tag and removes it from every member who has it.
- **`circle-pp-cli member-tags get`** - Gets a single member tag by ID, including its appearance and display settings.
- **`circle-pp-cli member-tags list`** - Lists the community's member tags as a paginated list, oldest first by default. Can be filtered by a case-insensitive partial name match and by is_public.
- **`circle-pp-cli member-tags update`** - Updates the name, appearance or visibility of a member tag. Only the fields provided are changed. It does not add or remove the tag on members.

### messages

Manage messages

- **`circle-pp-cli messages`** - Sends a chat message from the authenticated community member. Provide user_email for a direct message, user_emails for a group chat, or chat_room_uuid to post in an existing chat room. Set parent_message_id to reply in a thread.

### page-profile-fields

Manage page profile fields

- **`circle-pp-cli page-profile-fields`** - Get Page Profile Fields

### payment-method-setting

Manage payment method setting

- **`circle-pp-cli payment-method-setting`** - Returns the enabled/disabled state and Stripe capability status for every supported payment method (Apple Pay, Google Pay, card, Klarna, Afterpay/Clearpay, Affirm, Bancontact, iDEAL, Pix, SEPA Debit, Link) for the community's connected Stripe account.

### paywall-affiliate-payouts

Manage paywall affiliate payouts

- **`circle-pp-cli paywall-affiliate-payouts export-payouts`** - Creates a background task that exports processing affiliate payouts to a CSV, optionally scoped to specific community members
- **`circle-pp-cli paywall-affiliate-payouts mark-paid`** - Moves processing affiliate payouts to paid, optionally scoped to specific community members
- **`circle-pp-cli paywall-affiliate-payouts start`** - Moves due affiliate payouts to processing, optionally scoped to specific community members

### paywall-affiliates

Manage paywall affiliates

- **`circle-pp-cli paywall-affiliates create`** - Invites existing community members to the affiliate program
- **`circle-pp-cli paywall-affiliates list`** - Searches affiliates for a community
- **`circle-pp-cli paywall-affiliates update`** - Updates an affiliate's status

### paywall-coupons

Manage paywall coupons

- **`circle-pp-cli paywall-coupons <id>`** - Deletes a paywall coupon. Removes any Stripe-side coupon and invalidates offers that reference it.

### paywall-groups

Manage paywall groups

- **`circle-pp-cli paywall-groups create`** - Creates a new subscription group (paywall group) for the community. Paywalls with subscription prices can be added to a subscription group to allow members to upgrade/downgrade between them.
- **`circle-pp-cli paywall-groups update`** - Updates a subscription group's name. The currency can only be changed while the group has no paywalls attached to it.

### paywalls

Manage paywalls

- **`circle-pp-cli paywalls delete`** - Permanently deletes a paywall. Fails if the paywall has real transactions -- archive it instead.
- **`circle-pp-cli paywalls list`** - Search a community's paywalls and their prices. Only authorized community admins may search. Ordinary search remains available when both contextual parameters are omitted.
Before the host paywall is saved, search active targets in the host currency and inspect their subscription group to choose an add-on or upgrade. After saving, first resolve/read the current host with an ordinary paywall search filtered by paywall_id=<saved host ID>; verify the returned host ID, group, currency, and prices, then repeat the search with both contextual parameters. For example, use upsell_subtype=addon for an add-on, or upsell_subtype=upgrade for a same-group plan or eligible self-cycle. A draft host is eligible for a self-cycle only when its prices meet the existing cycle-upgrade rules.
Contextual filters, sorting, and pagination apply to the eligible candidate set. This is configuration discovery only; it does not change member subscriptions or the community billing plan and does not guarantee a checkout quote, trial, or purchase. For workflow trigger setup, use a returned PaywallPrice ID when the workflow field asks for a price ID, and a paywall ID when it asks for a paywall ID. Ordinary search includes prices of all statuses; prefer active prices for new triggers and retain inactive-price visibility for existing workflow references.

### posts

Manage posts

- **`circle-pp-cli posts create`** - Creates a basic post in a basic-type space. Use tiptap_body instead of body_html for rich text content.
- **`circle-pp-cli posts delete`** - Removes a basic post by ID from its space and unpins it if it was the pinned post. The post is soft deleted and related cleanup runs in the background.
- **`circle-pp-cli posts get`** - Returns a single basic post by ID, including its content, status, settings and engagement counts.
- **`circle-pp-cli posts list`** - Lists basic posts in the community, optionally filtered by space, space group, status or a
search on the post name. Only published posts are returned unless status is given (use all
for every status). Results are paginated.
- **`circle-pp-cli posts update`** - Updates a basic post by ID. Only the fields provided are changed; tiptap_body replaces the
content only for posts that already use the Tiptap editor, and topics replaces the post's
current topics.

### profile-fields

Manage profile fields

- **`circle-pp-cli profile-fields create`** - Creates a new custom profile field for the community.

Every part of the field is stated explicitly: `pages_attributes` lists the pages the field
appears on and whether it is visible on each, `choices_attributes` lists the options for a
select or checkbox field in the order given by `sort_key`, and `number_options_attributes`
configures a number field. Nothing is derived — what you send is what is stored.

To hide a field everywhere, send every page with `visible: false`.
- **`circle-pp-cli profile-fields delete`** - Permanently deletes an archived custom profile field.

The field must be archived first — deleting an active field returns a 404 telling you to
archive it. Archiving is reversible; this is not.

Deletion cascades: the field's pages, choices and number options go with it, along with
every member's stored value for the field. There is no undo and no confirmation step.
- **`circle-pp-cli profile-fields list`** - Lists the community's profile fields as a paginated list. Returns active fields by default, or archived fields when archived is 'true', and can be filtered by a case-insensitive partial label match.
- **`circle-pp-cli profile-fields update`** - Updates an existing custom profile field. Only the attributes sent are changed; anything
omitted is left as it is.

Nested records are addressed by `id`: send `pages_attributes` with a page id to change its
visibility, and `choices_attributes` with a choice id to rename or reorder that option.
Omit the id to add a new choice, or send `_destroy: true` to remove one — removing a choice
also removes every member's selection of it.

`field_type` cannot be changed once the field exists.

### settings

Manage settings

- **`circle-pp-cli settings list`** - Returns the current community's Connect settings — the admin settings that control member-to-member interactions. No parameters; always reads the authenticated community's live settings.

Three sections:
- connect: enabled, recommendations, exclude_admins_from_recommendations, exclude_moderators_from_recommendations
- member_directory: public, sort (alphabetical/oldest/latest), view (list/cards)
- messaging: enabled, member_to_member_messaging_enabled, group_messaging_enabled, voice_messages_enabled, require_connection_before_messaging

Use this to answer questions like "Is messaging enabled?", "Is the member directory public?", or "Are voice messages enabled?" — fetch live rather than answering from memory.
- **`circle-pp-cli settings update`** - Updates the current community's Connect settings — the admin settings that control member-to-member interactions. Every field is optional; only the fields you send are changed, the rest are left untouched (partial update).

Params are grouped into three nested objects:
- connect: enabled, recommendations (only effective when connect.enabled is true), exclude_admins_from_recommendations, exclude_moderators_from_recommendations
- member_directory: public, sort (alphabetical/oldest/latest), view (list/cards)
- messaging: enabled (master toggle), member_to_member_messaging_enabled, group_messaging_enabled, voice_messages_enabled, require_connection_before_messaging

Use this to change settings — e.g. "Enable Connect", "Make the member directory public", "Sort the member directory alphabetically", or "Turn off group messaging".

### space-group-member

Manage space group member

- **`circle-pp-cli space-group-member`** - Returns the space group membership of a user, identified by email, in a space group.

### space-group-members

Manage space group members

- **`circle-pp-cli space-group-members create`** - Adds a community member, identified by email, to a space group and to every space in the group. If they are already in the group, nothing changes.
- **`circle-pp-cli space-group-members delete`** - Removes a user, identified by email, from a space group and from every space in the group. They stay a member of the community.
- **`circle-pp-cli space-group-members list`** - List Space Group Members

### space-groups

Manage space groups

- **`circle-pp-cli space-groups create`** - Creates a space group, the sidebar section that spaces are organized under. The community member who owns the API token is added as its first member.
To add a space to the group, pass this group's ID as space_group_id when creating the space.
- **`circle-pp-cli space-groups delete`** - Deletes a space group by ID together with every space in it. The group and its spaces are soft-deleted right away,
and their content is soft-deleted in a background job.
- **`circle-pp-cli space-groups get`** - Returns a single space group by ID with its settings.
- **`circle-pp-cli space-groups list`** - Lists the space groups in the community, newest first, optionally filtered by name. Results are paginated.
- **`circle-pp-cli space-groups update`** - Updates the name, slug and settings of a space group by ID. A non-empty moderator_community_member_ids list replaces the group's current moderators; omitted or empty lists leave moderators unchanged. Only the fields sent are changed.

### space-member

Manage space member

- **`circle-pp-cli space-member`** - Returns the space membership of a user, identified by email, in a space.

### space-members

Manage space members

- **`circle-pp-cli space-members create`** - Adds a person to a space by email. The email must belong to an existing Circle user; if that user is not yet a member
of the community, they are also added to the community and emailed about it. If the space group adds members on space
join, the person is added to the space group too.
- **`circle-pp-cli space-members delete`** - Removes a community member, identified by email, from a space. They stay a member of the community.
- **`circle-pp-cli space-members list`** - Lists the members of a space, optionally filtered by status. Results are paginated.

### spaces

Manage spaces

- **`circle-pp-cli spaces create`** - Creates a space. space_type defaults to "basic" if not provided. Use course_setting for course-type spaces. Access levels are controlled via is_private and is_hidden_from_non_members.
- **`circle-pp-cli spaces delete`** - Deletes a space by ID. The space is soft-deleted right away and its content is soft-deleted in a background job.
- **`circle-pp-cli spaces get`** - Returns a single space by ID, including its settings, space group, topics and post IDs.
- **`circle-pp-cli spaces list`** - Lists the spaces in the community. Results are paginated.
- **`circle-pp-cli spaces update`** - Updates the settings of a space by ID. Only the fields sent are changed; space_type cannot be changed.
Sending topics replaces the space's topics, and is_draft can only be set to false, to publish a course space.

### tagged-members

Manage tagged members

- **`circle-pp-cli tagged-members create`** - Applies a member tag to the community member with the given email. If the member already has the tag, the existing assignment is kept.
- **`circle-pp-cli tagged-members delete`** - Removes a member tag from the community member with the given email. Only the tag assignment is deleted; the member tag itself is kept.
- **`circle-pp-cli tagged-members get`** - Gets a single tagged member record, the assignment of a member tag to a community member, by its ID.
- **`circle-pp-cli tagged-members list`** - Lists member tag assignments in the community, newest first, as a paginated list. Each record links a member tag to a community member. Can be filtered by member tag IDs.

### tax-setting

Manage tax setting

- **`circle-pp-cli tax-setting list`** - Returns the community's Stripe Tax settings, refreshed from a live Stripe read.
- **`circle-pp-cli tax-setting update`** - Updates the community's Stripe Tax platform status and/or default tax behavior.

### topics

Manage topics

- **`circle-pp-cli topics create`** - Creates a topic, a label members can add to posts, and optionally assigns it to spaces.
- **`circle-pp-cli topics delete`** - Permanently deletes a topic by ID. The topic is removed from its posts and spaces in background jobs.
- **`circle-pp-cli topics get`** - Returns a single topic by ID.
- **`circle-pp-cli topics list`** - Lists the topics in the community, optionally filtered by name and sorted. Results are paginated.
- **`circle-pp-cli topics update`** - Updates a topic by ID. space_ids replaces the spaces the topic is assigned to: spaces left out are unassigned,
and omitting space_ids removes the topic from all spaces.

### workflows

Manage workflows

- **`circle-pp-cli workflows get`** - Gets a single workflow by its UUID, including its trigger and all action steps in execution order. Each action carries a 1-indexed `list_position` for placement decisions; the raw `position` can have gaps.
- **`circle-pp-cli workflows list`** - Lists the community's automations/workflows, newest first, paginated. Each record is a lean
summary (id, name, status, type); it omits the trigger/actions graph and any URL - fetch a
single workflow to get the full graph. Filters are optional and combine with AND.

`status` semantics (note the omit-vs-all difference):
- omit -> all workflows, INCLUDING archived
- `all` -> all workflows EXCEPT archived
- `archived` -> only archived workflows
- `draft` / `active` / `inactive` -> only workflows with that status


### course


- **`circle-pp-cli course push`** - Upload a course kept as module folders and markdown lessons to a Circle course space, new lessons as drafts.

- **`circle-pp-cli course pull`** - Download a Circle course space into module folders with one file per lesson; lesson bodies stay as Circle's HTML and media is not downloaded.


### founding


- **`circle-pp-cli founding`** - Answer whether at least N people paid with GO or NO-GO and the count.


### inbox


- **`circle-pp-cli inbox`** - See new members, unreplied posts, flagged content, form submissions, failed charges and cancellations since a time, read from the local mirror.


### onboard


- **`circle-pp-cli onboard`** - Add people to the community, their spaces, space groups and access groups, and tag them, without sending invite emails by default.


### seats


- **`circle-pp-cli seats`** - Count paying members against a seat cap and list who paid.


### show-tell


- **`circle-pp-cli show-tell schedule`** - Create the next Show & Tell events on a cadence by duplicating a template event.

- **`circle-pp-cli show-tell recap`** - Pull a past session's RSVPed attendees and its live-room transcript into a markdown recap draft.


### spaces


- **`circle-pp-cli spaces apply`** - Make Circle's space groups and spaces match a YAML file, with a plan printed before any write.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`circle-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`circle-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`circle-pp-cli learnings list`** - Inspect taught rows
- **`circle-pp-cli learnings forget <query>`** - Undo a teach
- **`circle-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`circle-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`circle-pp-cli teach-pattern`** - Install a query/resource template up front
- **`circle-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CIRCLE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `circle-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
circle-pp-cli access-groups list

# JSON for scripting and agents
circle-pp-cli access-groups list --json
# Filter to specific fields by name
circle-pp-cli access-groups list --json --select <field>[,<field>...]

# Dry run — show the request without sending
circle-pp-cli access-groups list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
circle-pp-cli access-groups list --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
circle-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `circle-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/circle-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\circle-pp-cli\config.toml` on Windows; `--home`, `CIRCLE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `CIRCLE_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `CIRCLE_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CIRCLE_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `circle-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `circle-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CIRCLE_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 401 on every command** — Check `CIRCLE_API_TOKEN` is an Admin API V2 token (V1 tokens do not work), then run `circle-pp-cli doctor`.
- **HTTP 429 Too Many Requests** — Circle allows 2,000 requests per 5 minutes per IP. Wait a few minutes, then rerun with `--rate-limit 2`.
- **Monthly quota running low** — Run `circle-pp-cli sync` once, then use `search`, `analytics`, `seats` and `inbox` (or the MCP `sql` tool), which read the local mirror.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**circleso**](https://github.com/atomimus/circleso) — JavaScript (2 stars)
- [**circle-mcp**](https://github.com/iamnortey/circle-mcp) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
