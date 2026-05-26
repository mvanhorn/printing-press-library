# GoHighLevel CLI — Absorb Manifest

## Source Tools Surveyed

| Tool | URL | Features Contributed |
|------|-----|----------------------|
| mastanley13/GoHighLevel-MCP (local fork) | https://github.com/mastanley13/GoHighLevel-MCP | ~250 tools across 19 categories (contacts, conversations, calendars, blogs, social, payments, custom-fields, etc.) |
| BusyBee3333/Go-High-Level-MCP-2026-Complete | https://github.com/BusyBee3333/Go-High-Level-MCP-2026-Complete | 520+ tools, includes Agent Studio API (March 2026) |
| robbyDAninja/7fa-ghl-mcp | https://github.com/robbyDAninja/7fa-ghl-mcp | 34-tool curated subset, ~8k tokens vs 350k full |
| tenfoldmarc/ghl-mcp | https://github.com/tenfoldmarc/ghl-mcp | 70+ tools, terminal-positioning |
| @gohighlevel/api-client | https://www.npmjs.com/package/@gohighlevel/api-client | Official Node SDK, OAuth refresh |
| highlevel-python (PyPI) | https://pypi.org/project/highlevel-python/ | OAuth-only, minimal coverage |
| KWCP custom Python scripts | local | Real-world dedup + bulk-tag + at-risk patterns |

No dedicated GoHighLevel CLI exists in the public library or GitHub. This CLI is the first.

## Absorbed (60 features — match or beat every shipping tool)

### Contacts (highest leverage)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | Create contact | All MCPs | `ghlcli contact create` | --stdin batch, --dry-run, idempotent |
| 2 | Get contact by id | All MCPs | `ghlcli contact get <id>` | --select fields, --json |
| 3 | Update contact | All MCPs | `ghlcli contact update <id>` | --no-overwrite-if-set flag |
| 4 | Delete contact | All MCPs | `ghlcli contact delete <id>` | --dry-run preview |
| 5 | Search contacts | All MCPs | `ghlcli contact search` | Auto searchAfter cursor (no 100-page cap), FTS5 offline |
| 6 | Find duplicate | mastanley13 MCP | `ghlcli contact find-duplicate` | Returns rich match metadata |
| 7 | Upsert by email/phone | mastanley13 MCP | `ghlcli contact upsert` | Stdin batch, retry-safe |
| 8 | Add tags | All MCPs | `ghlcli contact add-tag <id> --tag <name>` | Resolves tag name→id from cache |
| 9 | Remove tags | All MCPs | `ghlcli contact remove-tag` | Same |
| 10 | Bulk tag update | mastanley13 MCP | `ghlcli contact bulk-tag` | Chunks 100 at a time, --csv input |
| 11 | List contact notes | mastanley13 MCP | `ghlcli contact notes <id>` | --json |
| 12 | Create note | mastanley13 MCP | `ghlcli contact add-note` | --from-file |
| 13 | List contact tasks | mastanley13 MCP | `ghlcli contact tasks <id>` | --pending only |
| 14 | Create task | mastanley13 MCP | `ghlcli contact add-task` | --due-date relative ("+3d") |
| 15 | Enroll in workflow | mastanley13 MCP | `ghlcli contact enroll <contactId> <workflowId>` | Warning about "Form Submitted" non-fire |
| 16 | Remove from workflow | mastanley13 MCP | `ghlcli contact unenroll` | Same |

### Opportunities
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 17 | Search opportunities | All MCPs | `ghlcli opp search` | Filters by stage, status, contact |
| 18 | Get opportunity | All MCPs | `ghlcli opp get <id>` | --select |
| 19 | Create opportunity | All MCPs | `ghlcli opp create` | --stage-name (not id) resolution |
| 20 | Update opportunity | All MCPs | `ghlcli opp update` | Move to stage by name |
| 21 | Update status only | mastanley13 MCP | `ghlcli opp status <id> <status>` | Convenience wrapper |
| 22 | Upsert opportunity | mastanley13 MCP | `ghlcli opp upsert` | --by contact-id |
| 23 | Delete opportunity | mastanley13 MCP | `ghlcli opp delete <id>` | --dry-run |
| 24 | List pipelines | All MCPs | `ghlcli pipeline list` | Includes stage breakdown |

### Custom Fields & Values
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 25 | List custom fields | All MCPs | `ghlcli field list` | Sorted by position, --json |
| 26 | Get custom field | All MCPs | `ghlcli field get <id-or-name>` | name→id resolution |
| 27 | Create custom field | All MCPs | `ghlcli field create` | Errors for dropdown w/ link to UI |
| 28 | Update custom field | All MCPs | `ghlcli field update` | — |
| 29 | Delete custom field | All MCPs | `ghlcli field delete` | --dry-run |
| 30 | List custom values | mastanley13 MCP | `ghlcli value list` | --json |
| 31 | Create custom value | mastanley13 MCP | `ghlcli value create` | — |
| 32 | Get/update/delete custom value | mastanley13 MCP | `ghlcli value <verb>` | name→id resolution |

### Tags
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 33 | List location tags | mastanley13 MCP | `ghlcli tag list` | --json |
| 34 | Create tag | mastanley13 MCP | `ghlcli tag create <name>` | Idempotent (returns existing if name clash) |
| 35 | Delete tag | mastanley13 MCP | `ghlcli tag delete <name-or-id>` | name→id resolution |

### Calendars & Appointments
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 36 | List calendars | All MCPs | `ghlcli calendar list` | — |
| 37 | Get calendar | All MCPs | `ghlcli calendar get <id>` | — |
| 38 | Create calendar | All MCPs | `ghlcli calendar create` | — |
| 39 | List calendar events | All MCPs | `ghlcli appt list` | --upcoming, --calendar-name |
| 40 | Get free slots | All MCPs | `ghlcli calendar free-slots <calendarId>` | --start "+1d" relative |
| 41 | Create appointment | All MCPs | `ghlcli appt create` | --calendar-name resolution |
| 42 | Get/update/delete appointment | All MCPs | `ghlcli appt <verb>` | — |

### Conversations & Messages
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 43 | Search conversations | All MCPs | `ghlcli convo search` | --contact, --assigned-to |
| 44 | Get conversation | All MCPs | `ghlcli convo get <id>` | — |
| 45 | List messages | All MCPs | `ghlcli convo messages <id>` | --json |
| 46 | Send SMS | mastanley13 MCP | `ghlcli msg send --type sms --to <contactId>` | --dry-run |
| 47 | Send email | mastanley13 MCP | `ghlcli msg send --type email` | --subject, --html-file, --dry-run |

### Locations & Users
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 48 | Search locations | mastanley13 MCP | `ghlcli location list` | Agency-level |
| 49 | Get location | mastanley13 MCP | `ghlcli location get [id]` | Defaults to active location |
| 50 | List users | mastanley13 MCP | `ghlcli user list` | --json |
| 51 | Get user | mastanley13 MCP | `ghlcli user get <id>` | — |

### Workflows & Surveys
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 52 | List workflows | All MCPs | `ghlcli workflow list` | Sorted by name |
| 53 | List surveys | mastanley13 MCP | `ghlcli survey list` | — |

### Multi-Location Config & Auth (CLI primitives)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 54 | Auth setup | none | `ghlcli auth setup` | Stores PIT in OS keychain, validates lowercase prefix |
| 55 | Auth status | none | `ghlcli auth status` | Shows current location, masked token |
| 56 | Location add | none | `ghlcli location add <slug>` | Multi-location config (KWCP, THINK, etc.) |
| 57 | Location switch | none | `ghlcli --location <slug> <cmd>` | Cross-location ops in one CLI |
| 58 | Local sync | none | `ghlcli sync` | Pulls contacts/opportunities/fields/tags/users into SQLite |
| 59 | SQL passthrough | framework | `ghlcli sql "<query>"` | Direct SQL against local cache |
| 60 | Doctor | framework | `ghlcli doctor` | Auth check, API reachability, version-header confirmation |

## Stubs (None)

All shipping-scope features above are fully implemented. No stubs.

## Transcendence (only possible with our approach)

Canonical 11 survivors from the novel-features subagent brainstorm (see `2026-05-19-151335-novel-features-brainstorm.md` for the full Customer model + Candidate pre-cut + Killed-candidate audit trail).

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|------------------------|
| 1 | Stale opportunity report | `ghlcli opp stale --pipeline <name> --stage <name> --days N [--include-history]` | hand-code | 9 | Local SQLite `stage_transitions` table built from sync diffs (GHL exposes no stage-history endpoint); joins opportunities × stages × pipelines and filters by `entered_stage_at < now() - N days`. |
| 2 | Pipeline funnel snapshot | `ghlcli opp funnel --pipeline <name> [--tsv\|--json]` | hand-code | 9 | Aggregates opportunities grouped by stage_id joined to pipelines + stages in local SQLite; emits Looker-friendly TSV. No GHL funnel endpoint exists. |
| 3 | Custom field name resolver | `ghlcli field id <name>`; universal `--custom-field "Name=Value"` interceptor | hand-code | 10 | Local SQLite custom_fields table; flag interceptor translates `"Agent Affiliation=KWCP"` → `ilvReXHcDuxetPOZ4wCK=KWCP` before every API call. Did-you-mean via Levenshtein. |
| 4 | Bulk tag from stdin | `ghlcli contact bulk-tag --tag <t> [--remove] [--dry-run]` | hand-code | 9 | Reads emails/IDs from stdin, looks up contact IDs from local cache, calls `POST /contacts/{id}/tags` in batches of 100 with exponential backoff. Chunking + dedup is the leverage. |
| 5 | SQL-on-cache | `ghlcli sql "<query>"` | hand-code | 10 | Opens local SQLite cache; exposes contacts, opportunities, pipelines, stages, tags, custom_fields, conversations, messages, appointments as tables. Read-only. The cross-entity differentiator. |
| 6 | Dedup with richness scoring | `ghlcli contact dedup --by email,phone --dry-run [--apply]` | hand-code | 8 | Groups by lowercased email + E.164 phone, scores by filled-field count + `dateUpdated` recency, emits merge plan JSON; `--apply` calls `POST /contacts/upsert`. |
| 7 | Engagement decay alert | `ghlcli contact decay --stage <name> --idle-days N` | hand-code | 8 | Joins opportunities × contacts × conversations locally; flags rows where `max(message.dateAdded)` is older than N days. |
| 8 | Hot follow-up scorecard | `ghlcli recruit hot --threshold N` | hand-code | 7 | Composite scoring from user memory `kwcp_hot_followup_scoring_v2.md`: production + engagement + recruit tags via SQL over local cache. |
| 9 | Multi-location config + flag | `ghlcli config use <name>`; `--location <name>` global | hand-code | 8 | Named profiles in `~/.config/ghlcli/config.toml`; every command resolves locationId from profile so cross-tenant calls can't leak. |
| 10 | Doctor | `ghlcli doctor` | hand-code | 7 | Validates GHL_PIT_TOKEN prefix (auto-lowercases), pings `/locations/{id}`, reports cache freshness per table, warns on stale workflow membership. |
| 11 | Conversation thread reconstruction | `ghlcli convo thread --contact <email\|id>` | hand-code | 7 | Resolves contact ID, queries local messages table, sorts by `dateAdded` across channels, emits unified timeline with channel/direction/from/body/timestamp. |

## Hand-code commitment

- Absorbed features: 60 (driven by spec; most generate as endpoint commands)
- Transcendence features: 11 (all hand-code Go in `internal/cli/<feature>.go`)
- Hand-code count: **11 commands**, each ~80-200 LoC plus `root.go` wiring

## Risks & Notes

- **GHL connection drops under load** — Phase 3 client must use exponential backoff. Generator-emitted client has retry hooks; verify.
- **Phone search 500 error** — block `?phone=` at the CLI layer with a friendly error.
- **Dropdown/multi-select fields can't be created via API** — `field create --type dropdown` errors early with UI link.
- **PIT prefix case sensitivity** — `config.Load()` must lowercase the prefix; warn on uppercase input.
- **Version header per resource** — Conversations uses 2021-04-15; everything else uses 2021-07-28. Encoded in the spec, handled by the client at request time.

## Sources Considered, Not Adopted

- Full Memberships/Courses API surface — small audience for KWCP and adds 30+ endpoints.
- OAuth marketplace flow — out of scope; PIT covers all CLI use cases.
- Funnels/Forms create endpoints — read-only listings only via the surveys endpoint where supported.
- Phone System API — narrow use case, not needed for v1.
- Snapshots, SaaS, Triggers, Companies, Businesses — listed in Stoplight but not in the local MCP; defer to v2.
