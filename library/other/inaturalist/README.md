# iNaturalist CLI

**Explore iNaturalist's full API, plus privacy-safe field briefings and identification progress that ordinary endpoint wrappers miss.**

Use nearby highlights and seasonal-shift for transparent, bounded biodiversity briefings. Create factual scavenger hunts from real taxa, and use identification commands to track whether observations gained community IDs without exposing locations.

## Install

The recommended path installs both the `inaturalist-pp-cli` binary and the `pp-inaturalist` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install inaturalist
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install inaturalist --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install inaturalist --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install inaturalist --agent claude-code
npx -y @mvanhorn/printing-press-library install inaturalist --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/inaturalist/cmd/inaturalist-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/inaturalist-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install inaturalist --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-inaturalist --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-inaturalist --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install inaturalist --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/inaturalist-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `INATURALIST_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/inaturalist/cmd/inaturalist-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "inaturalist": {
      "command": "inaturalist-pp-mcp",
      "env": {
        "INATURALIST_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Public read endpoints work without credentials. Authenticated iNaturalist responses can include private data, so the CLI must never surface or store private location fields in compound workflows; write commands require the official JWT/OAuth flow.

## Quick Start

```bash
# Check local configuration without credentials or network mutation.
inaturalist-pp-cli doctor --dry-run

# Get a privacy-safe briefing for an explicit area.
inaturalist-pp-cli nearby highlights --lat 37.7749 --lng -122.4194 --radius 5 --agent

# Create a factual scavenger-hunt checklist from local taxa.
inaturalist-pp-cli hunt create --place-id 97394 --iconic-taxa Aves,Plantae --agent

# Check current observation identification progress.
inaturalist-pp-cli observations id-status --user inaturalist --since 30d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Privacy-safe field briefings
- **`nearby highlights`** — Get a transparent, recent wildlife briefing for an area without exposing observation coordinates.

  _Use this when an agent needs an explanation-backed local wildlife overview rather than a raw observation list._

  ```bash
  inaturalist-pp-cli nearby highlights --lat 37.7749 --lng -122.4194 --radius 5 --agent
  ```
- **`hunt create`** — Create a factual, balanced nature scavenger-hunt checklist from taxa actually observed nearby.

  _Use this to turn local biodiversity evidence into a safe field activity with traceable taxa._

  ```bash
  inaturalist-pp-cli hunt create --place-id 97394 --iconic-taxa Aves,Plantae --agent
  ```
- **`nearby seasonal-shift`** — Compare two field windows and surface taxa that newly appeared, returned, or changed materially.

  _Use this to answer how local wildlife changed between explicit time windows._

  ```bash
  inaturalist-pp-cli nearby seasonal-shift --place-id 97394 --recent-days 30 --baseline-days 30 --agent
  ```

### Identification progress
- **`observations id-status`** — See which of an observer's recent observations are identified, need IDs, disagree, or have no taxon.

  _Use this for a current identification-progress answer instead of manually filtering raw observations._

  ```bash
  inaturalist-pp-cli observations id-status --user inaturalist --since 30d --agent
  ```
- **`observations id-changes`** — Report observations that became identified, changed, withdrew, or still need IDs since a previous privacy-safe sync.

  _Use this to find identification progress since the last check without fabricating history._

  ```bash
  inaturalist-pp-cli observations id-changes --user inaturalist --since 30d --agent
  ```

## Recipes

### Nearby wildlife briefing

```bash
inaturalist-pp-cli nearby highlights --lat 37.7749 --lng -122.4194 --radius 5 --agent --select results.taxon_name,results.reason,results.geoprivacy
```

Return just the privacy-safe taxa, ranking rationale, and privacy state for an explicit area.

### Build a bird and plant hunt

```bash
inaturalist-pp-cli hunt create --place-id 97394 --iconic-taxa Aves,Plantae --agent
```

Make a factual checklist from observed taxa, without observation locations.

### Check identification progress

```bash
inaturalist-pp-cli observations id-status --user inaturalist --since 30d --agent
```

Group recent observations by their current identification state.

### Compare field windows

```bash
inaturalist-pp-cli nearby seasonal-shift --place-id 97394 --recent-days 30 --baseline-days 30 --agent
```

See transparent changes between two bounded observation windows.

## Usage

Run `inaturalist-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `INATURALIST_CONFIG_DIR`, `INATURALIST_DATA_DIR`, `INATURALIST_STATE_DIR`, or `INATURALIST_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `INATURALIST_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export INATURALIST_HOME=/srv/inaturalist
inaturalist-pp-cli doctor
```

Under `INATURALIST_HOME=/srv/inaturalist`, the four dirs resolve to `/srv/inaturalist/config`, `/srv/inaturalist/data`, `/srv/inaturalist/state`, and `/srv/inaturalist/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "inaturalist": {
      "command": "inaturalist-pp-mcp",
      "env": {
        "INATURALIST_HOME": "/srv/inaturalist"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `INATURALIST_DATA_DIR` overrides an explicit `--home` for that kind. Use `INATURALIST_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `INATURALIST_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `inaturalist-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### annotations

Create, delete, and vote

- **`inaturalist-pp-cli annotations create`** - Create an annotation
- **`inaturalist-pp-cli annotations delete`** - Delete an annotation

### colored-heatmap

Manage colored heatmap


### comments

Create, update, and delete

- **`inaturalist-pp-cli comments create`** - Create a comment
- **`inaturalist-pp-cli comments delete`** - Delete a comment
- **`inaturalist-pp-cli comments update`** - Update a comment

### controlled-terms

Search and fetch

- **`inaturalist-pp-cli controlled-terms list`** - List all attribute controlled terms
- **`inaturalist-pp-cli controlled-terms list-controlledterms`** - Returns attribute controlled terms relevant to a taxon

### flags

Create, update, and delete flags

- **`inaturalist-pp-cli flags create`** - Create a flag. To create a custom flag beyond the standard `spam` and
`inappropriate` flags, set `flag` to `other` and include a `flag_explanation`
- **`inaturalist-pp-cli flags delete`** - Delete a flag
- **`inaturalist-pp-cli flags update`** - Update a flag. Generally only used to resolve the flag.

### grid

Manage grid


### heatmap

Manage heatmap


### identifications

Create, update, and delete

- **`inaturalist-pp-cli identifications create`** - Create an identification
- **`inaturalist-pp-cli identifications delete`** - Delete an identification. See description of `PUT /identifications/{id}
for notes on withdrawing and restoring identifications.
- **`inaturalist-pp-cli identifications get`** - Given an ID, or an array of IDs in comma-delimited format, returns
corresponding identifications. A maximum of 30 results will be returned
- **`inaturalist-pp-cli identifications list`** - Given zero to many of following parameters, returns identifications
matching the search criteria
- **`inaturalist-pp-cli identifications list-categories`** - Given zero to many of following parameters, return counts of the
categories of identifications matching the search criteria
- **`inaturalist-pp-cli identifications list-identifiers`** - Given zero to many of following parameters, returns creators of
identifications matching the search criteria and the count of
matching identifications, ordered by count descending. A
maximum of 500 results will be returned
- **`inaturalist-pp-cli identifications list-observers`** - Given zero to many of following parameters, returns creators of
observations of identifications matching the search criteria and
the count of matching observations, ordered by count descending
- **`inaturalist-pp-cli identifications list-recenttaxa`** - Returns an array of objects each containing an identification and a
taxon. Returns IDs representing the earliest occurrence of taxa
associated with identifications in the filtered set of results
- **`inaturalist-pp-cli identifications list-similarspecies`** - Returns species attached to IDs of observations of this taxon, or
attached to observations identified as this species, ordered by combined
frequency descending. This will only return species in the same iconic
taxon, and will never return descendants of the chosen taxon
- **`inaturalist-pp-cli identifications list-speciescounts`** - Given zero to many of following parameters, returns `leaf taxa`
associated with identifications matching the search criteria and the
count of identifications they are associated with, ordered by count
descending. `Leaf taxa` are the leaves of the taxonomic tree containing
only the taxa associated with observations matching the search criteria.
- **`inaturalist-pp-cli identifications update`** - Update an identification. Note that to "withdraw" an observation you
send a `PUT` request to this endpoint and set the `current`
attribute to false. To "restore" it you do the same but set
`current` to `true`. Only one identification by a given user can be
`current` for a given observation, so if you "restore" one all the other
identifications by the authenticated user for the given observation will
be withdrawn.

### messages

Create, fetch, delete

- **`inaturalist-pp-cli messages create`** - Create and deliver a new message to another user
- **`inaturalist-pp-cli messages delete`** - This will all of the authenticated user's copies of the messages in tha
thread to which the specified message belongs.
- **`inaturalist-pp-cli messages get`** - Retrieves all messages in the thread the specified message belongs to
and marks them all as read.
- **`inaturalist-pp-cli messages list`** - Retrieve messages for the authenticated user. This does not mark them as read.
- **`inaturalist-pp-cli messages list-unread`** - Gets a count of messages the authenticated user has not read

### observation-field-values

Create, update, and delete

- **`inaturalist-pp-cli observation-field-values create`** - Create an observation field value
- **`inaturalist-pp-cli observation-field-values delete`** - Delete an observation field value
- **`inaturalist-pp-cli observation-field-values update`** - Update an observation field value

### observation-photos

Create and delete

- **`inaturalist-pp-cli observation-photos create`** - Create an observation photo
- **`inaturalist-pp-cli observation-photos delete`** - Delete an observation photo
- **`inaturalist-pp-cli observation-photos update`** - Update an observation photo

### observations

CRUD, search, faving, quality metrics, stats, and more

- **`inaturalist-pp-cli observations create`** - Create an observation
- **`inaturalist-pp-cli observations delete`** - Delete an observation
- **`inaturalist-pp-cli observations get`** - Given an ID, or an array of IDs in comma-delimited format, returns
corresponding observations. A maximum of 200 results will be returned
- **`inaturalist-pp-cli observations list`** - Given zero to many of following parameters, returns observations
matching the search criteria. The large size of the observations index
prevents us from supporting the `page` parameter when retrieving records
from large result sets. If you need to retrieve large numbers of
records, use the `per_page` and `id_above` or `id_below` parameters
instead.
- **`inaturalist-pp-cli observations list-deleted`** - Given a starting date, return an array of IDs of the authenticated
user's observations that have been deleted since that date. Requires
authentication
- **`inaturalist-pp-cli observations list-histogram`** - Given zero to many of following parameters, returns histogram data about
observations matching the search criteria
- **`inaturalist-pp-cli observations list-identifiers`** - Given zero to many of following parameters, returns identifiers of
observations matching the search criteria and the count of
observations they have identified, ordered by count descending. A
maximum of 500 results will be returned
- **`inaturalist-pp-cli observations list-observers`** - Given zero to many of following parameters, returns observers of
observations matching the search criteria and the count of
observations and distinct taxa of rank `species` they have observed. A
maximum of 500 results will be returned
- **`inaturalist-pp-cli observations list-popularfieldvalues`** - Given zero to many of following parameters, returns an array of
relevant controlled terms values and a monthly histogram
- **`inaturalist-pp-cli observations list-speciescounts`** - Given zero to many of following parameters, returns `leaf taxa`
associated with observations matching the search criteria and the count of
observations they are associated with, ordered by count descending.
`Leaf taxa` are the leaves of the taxonomic tree containing only the
taxa associated with observations matching the search criteria.
- **`inaturalist-pp-cli observations list-updates`** - Given zero to many of following parameters, returns an array of objects
representing new comments and identifications on observations the authenticated
user has subscribed to. Requires authentication
- **`inaturalist-pp-cli observations update`** - Update an observation

### photos

Manage photos

- **`inaturalist-pp-cli photos`** - Create a photo

### places

Search and fetch

- **`inaturalist-pp-cli places get`** - Given an ID, or an array of IDs in comma-delimited format, returns
corresponding places. A maximum of 500 results will be returned
- **`inaturalist-pp-cli places list`** - Given an string, returns places with names starting with the search
term.
- **`inaturalist-pp-cli places list-nearby`** - Given an bounding box, and an optional name query, return `standard`
iNaturalist curator approved and `community` non-curated places nearby

### points

Manage points


### posts

Fetch site and project posts

- **`inaturalist-pp-cli posts create`** - Create a post
- **`inaturalist-pp-cli posts delete`** - Delete a post
- **`inaturalist-pp-cli posts list`** - Return journal posts from the iNaturalist site
- **`inaturalist-pp-cli posts list-foruser`** - Return journal posts from the iNaturalist site. If the user is logged-in,
also return posts from projects the user is subscribed to
- **`inaturalist-pp-cli posts update`** - Update a post

### project-observations

Create, update, and delete

- **`inaturalist-pp-cli project-observations create`** - Add an observation to a project
- **`inaturalist-pp-cli project-observations delete`** - Delete a project observation
- **`inaturalist-pp-cli project-observations update`** - Update a project observation

### projects

Search and fetch projects and members

- **`inaturalist-pp-cli projects get`** - Given an ID, or an array of IDs in comma-delimited format, returns
corresponding projects. A maximum of 100 results will be returned
- **`inaturalist-pp-cli projects list`** - Given zero to many of following parameters, returns projects
matching the search criteria
- **`inaturalist-pp-cli projects list-autocomplete`** - Given an string, returns projects with titles starting with the search term

### site_search

Manage site search

- **`inaturalist-pp-cli site-search`** - Given zero to many of following parameters, returns object matching the search criteria

### subscriptions

Manage subscriptions

- **`inaturalist-pp-cli subscriptions create`** - Toggles current user's subscription to this observation. If the logged-in
user is not subscribed, POSTing here will subscribe them. If they are already
subscribed, this will remove the subscription
- **`inaturalist-pp-cli subscriptions create-project`** - Toggles current user's subscription to this project. If the logged-in
user is not subscribed, POSTing here will subscribe them. If they are already
subscribed, this will remove the subscription

### taxa

Search and fetch

- **`inaturalist-pp-cli taxa get`** - Given an ID, or an array of IDs in comma-delimited format, returns
corresponding taxa. A maximum of 30 results will be returned
- **`inaturalist-pp-cli taxa list`** - Given zero to many of following parameters, returns taxa matching the search criteria
- **`inaturalist-pp-cli taxa list-autocomplete`** - Given an string, returns taxa with names starting with the search term

### taxon-places

Manage taxon places


### taxon-ranges

Manage taxon ranges


### users

Fetch and update

- **`inaturalist-pp-cli users create`** - Resend an email confirmation
- **`inaturalist-pp-cli users get`** - Given an ID, returns corresponding user
- **`inaturalist-pp-cli users list`** - Given an string, returns users with names or logins starting with the search term
- **`inaturalist-pp-cli users list-me`** - Fetch the logged-in user
- **`inaturalist-pp-cli users update`** - Update the logged-in user's session
- **`inaturalist-pp-cli users update-id`** - Update a user

### votes

Manage votes

- **`inaturalist-pp-cli votes create`** - Vote on an annotation
- **`inaturalist-pp-cli votes create-vote`** - Vote on an observation. A vote with an empty `scope` is recorded as a
`fave` of the observation. A vote with scope `needs_id` is recorded as a
vote on the Quality Grade criterion "can the Community ID still be
confirmed or improved?", and can be an up or down vote
- **`inaturalist-pp-cli votes delete`** - Remove a vote from annotation
- **`inaturalist-pp-cli votes delete-unvote`** - Remove a vote from an observation


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`inaturalist-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`inaturalist-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`inaturalist-pp-cli learnings list`** - Inspect taught rows
- **`inaturalist-pp-cli learnings forget <query>`** - Undo a teach
- **`inaturalist-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`inaturalist-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`inaturalist-pp-cli teach-pattern`** - Install a query/resource template up front
- **`inaturalist-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `INATURALIST_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `inaturalist-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
inaturalist-pp-cli controlled-terms list

# JSON for scripting and agents
inaturalist-pp-cli controlled-terms list --json

# Filter to specific fields
inaturalist-pp-cli controlled-terms list --json --select id,name,status

# Dry run — show the request without sending
inaturalist-pp-cli controlled-terms list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
inaturalist-pp-cli controlled-terms list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
inaturalist-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `inaturalist-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/inaturalist-pp-cli/config.toml`; `--home`, `INATURALIST_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INATURALIST_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `inaturalist-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `inaturalist-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INATURALIST_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 Too Many Requests** — Use narrower date, place, or taxon filters and wait between requests; iNaturalist asks clients to stay around one request per second.
- **A compound command has no prior history** — Run a bounded privacy-safe sync first, then use observations id-changes after a later update.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**iNaturalist MCP**](https://github.com/cvsouth/inaturalist-mcp) — Python (4 stars)
- [**iNaturalist MCP**](https://github.com/ufo2243/inaturalist-mcp) — TypeScript (1 stars)
- [**iNaturalist MCP**](https://github.com/cssnr/inaturalist-mcp) — Python (1 stars)
- [**inaturalist-cli**](https://github.com/tamnd/inaturalist-cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
