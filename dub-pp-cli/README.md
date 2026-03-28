# dub-pp-cli

Dub is the modern link attribution platform for short links, conversion tracking, and affiliate programs.

## Install

### Homebrew

```
brew install tmchow/tap/dub-pp-cli
```

### Go

```
go install github.com/tmchow/dub-pp-cli/cmd/dub-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/tmchow/dub-pp-cli/releases).

## Quick Start

```bash
# 1. Set your API credentials
export DUB_TOKEN="your-key-here"

# 2. Verify everything works
dub-pp-cli doctor

# 3. Start using it
dub-pp-cli analytics --help
```

## Usage

<!-- HELP_OUTPUT -->

## Commands

### analytics

Manage analytics

- **`dub-pp-cli analytics retrieve`** - Retrieve analytics for a link, a domain, or the authenticated workspace.

### bounties

Manage bounties


### commissions

Manage commissions

- **`dub-pp-cli commissions list`** - List all commissions
- **`dub-pp-cli commissions update`** - Update a commission

### customers

Manage customers

- **`dub-pp-cli customers delete`** - Delete a customer
- **`dub-pp-cli customers get`** - Retrieve a list of customers
- **`dub-pp-cli customers get-id`** - Retrieve a customer
- **`dub-pp-cli customers update`** - Update a customer

### domains

Manage domains

- **`dub-pp-cli domains check-status`** - Check the availability of one or more domains
- **`dub-pp-cli domains create`** - Create a domain
- **`dub-pp-cli domains delete`** - Delete a domain
- **`dub-pp-cli domains list`** - Retrieve a list of domains
- **`dub-pp-cli domains register`** - Register a domain
- **`dub-pp-cli domains update`** - Update a domain

### events

Manage events

- **`dub-pp-cli events list`** - Retrieve a list of events

### folders

Manage folders

- **`dub-pp-cli folders create`** - Create a folder
- **`dub-pp-cli folders delete`** - Delete a folder
- **`dub-pp-cli folders list`** - Retrieve a list of folders
- **`dub-pp-cli folders update`** - Update a folder

### links

Manage links

- **`dub-pp-cli links bulk-create`** - Bulk create links
- **`dub-pp-cli links bulk-delete`** - Bulk delete links
- **`dub-pp-cli links bulk-update`** - Bulk update links
- **`dub-pp-cli links create`** - Create a link
- **`dub-pp-cli links delete`** - Delete a link
- **`dub-pp-cli links get`** - Retrieve a list of links
- **`dub-pp-cli links get-count`** - Retrieve links count
- **`dub-pp-cli links get-info`** - Retrieve a link
- **`dub-pp-cli links update`** - Update a link
- **`dub-pp-cli links upsert`** - Upsert a link

### partners

Manage partners

- **`dub-pp-cli partners ban`** - Ban a partner
- **`dub-pp-cli partners create`** - Create or update a partner
- **`dub-pp-cli partners create-link`** - Create a link for a partner
- **`dub-pp-cli partners deactivate`** - Deactivate a partner
- **`dub-pp-cli partners list`** - List all partners
- **`dub-pp-cli partners retrieve-analytics`** - Retrieve analytics for a partner
- **`dub-pp-cli partners retrieve-links`** - Retrieve a partner's links.
- **`dub-pp-cli partners upsert-link`** - Upsert a link for a partner

### payouts

Manage payouts

- **`dub-pp-cli payouts list`** - List all payouts

### qr

Manage qr

- **`dub-pp-cli qr get-qrcode`** - Retrieve a QR code

### tags

Manage tags

- **`dub-pp-cli tags create`** - Create a tag
- **`dub-pp-cli tags delete`** - Delete a tag
- **`dub-pp-cli tags get`** - Retrieve a list of tags
- **`dub-pp-cli tags update`** - Update a tag

### tokens

Manage tokens

- **`dub-pp-cli tokens create-referrals-embed`** - Create a referrals embed token

### track

Manage track

- **`dub-pp-cli track lead`** - Track a lead
- **`dub-pp-cli track open`** - Track a deep link open event
- **`dub-pp-cli track sale`** - Track a sale


## Output Formats

```bash
# Human-readable table (default)
dub-pp-cli analytics list

# JSON for scripting and agents
dub-pp-cli analytics list --json

# Filter specific fields
dub-pp-cli analytics list --json --select id,name,status

# Plain tab-separated for piping
dub-pp-cli analytics list --plain

# Dry run (show request without sending)
dub-pp-cli analytics list --dry-run
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | dub-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
dub-pp-cli doctor
```

<!-- DOCTOR_OUTPUT -->

## Configuration

Config file: `~/.config/dub-pp-cli/config.toml`

Environment variables:
- `DUB_TOKEN`

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `dub-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DUB_TOKEN`

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
