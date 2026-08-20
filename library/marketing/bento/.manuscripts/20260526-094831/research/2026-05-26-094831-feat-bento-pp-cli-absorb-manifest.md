# Bento CLI Absorb Manifest

**Generation target:** Match every feature of `@bentonow/bento-cli` (22 leaves) and `@bentonow/bento-mcp` (14 tools), plus the Ruby/PHP SDK features both miss (transactional send, spam check, batch commands, segments), plus 12 novel transcendence commands only possible with our local-store + compositional-shell + agent-native architecture.

**Sources scanned:** Bento docs (bentonow.com/docs/*), OpenAPI spec (github.com/bentonow/api/blob/main/bento-api.yaml), bento-cli, bento-mcp, bento-node-sdk, bento-ruby-sdk, bento-python-sdk, bento-php-sdk, bento-laravel-sdk, bento-actionmailer, n8n-nodes-bento, lobehub MCP catalog, Bento Shopify/Zapier integrations. ~13 SDK/CLI/MCP/community projects audited.

## Absorbed (match or beat everything that exists)

### Subscribers
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Get subscriber by email/uuid | bento-cli `subscribers search`, bento-mcp `bento_get_subscriber`, node/ruby/python SDKs | `bento subscribers get <email-or-uuid>` | --json --select cents, local-cache hit if synced, typed exit 4 on not-found |
| 2 | Batch import 1-1000 subscribers | bento-cli `subscribers import`, bento-mcp `bento_batch_import_subscribers` | `bento subscribers import --csv` and `--stdin` (JSON-lines) | --dry-run shows the batch payload, --validate runs hygiene first, async-import polling baked in |
| 3 | Subscribe (triggers automations) | bento-cli `subscribers subscribe`, node SDK `addSubscriber` | `bento subscribers subscribe <email>` | --tag/--field flags, idempotent, --dry-run |
| 4 | Unsubscribe | bento-cli `subscribers unsubscribe` | `bento subscribers unsubscribe <email>` | --dry-run, --batch |
| 5 | Add tag to subscriber | bento-cli `subscribers tag` | `bento subscribers tag <email> <tag>` | --batch via stdin, --remove flag |
| 6 | Search subscribers (server-side, paginated) | docs `/fetch/search` | `bento subscribers search --page N` | Local FTS5 search via `bento subscribers find` is the fast path; server-side search kept for unsynced lookups |

### Tags
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 7 | List tags | bento-cli `tags list`, bento-mcp `bento_list_tags` | `bento tags list` | --json --select, local cache |
| 8 | Create tag | bento-cli `tags create`, bento-mcp `bento_create_tag` | `bento tags create <name>` | --dry-run |
| 9 | Delete tag | docs-claimed (CLI README says "stub - UI only") | `bento tags delete <name>` | Actually call DELETE /fetch/tags (CLI never did); --dry-run shows blast radius first |

### Fields
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 10 | List custom fields | bento-cli `fields list`, bento-mcp `bento_list_fields` | `bento fields list` | local cache |
| 11 | Create custom field | bento-cli `fields create`, bento-mcp `bento_create_field` | `bento fields create <key>` | --description, --dry-run |

### Events
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 12 | Track event | bento-cli `events track`, bento-mcp `bento_track_event` | `bento events track --email <e> --type <t> --fields <json>` | --batch via stdin (uses `/batch/events`), --dry-run shows payload before send |
| 13 | Track purchase | node SDK `trackPurchase` (NOT in CLI/MCP) | `bento events purchase --email <e> --order-id <id> --amount-cents <n> --currency USD` | auto-cents conversion, dedupe key, item-list expansion |
| 14 | Batch import events | node SDK `importEvents` | covered by `bento events track --batch` | -- |

### Broadcasts
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 15 | List broadcasts (paginated) | bento-cli `broadcasts list`, bento-mcp `bento_list_broadcasts` | `bento broadcasts list` | local cache, --page-all |
| 16 | Create broadcast draft | bento-cli `broadcasts create`, bento-mcp `bento_create_broadcast` | `bento broadcasts create --name X --subject Y --content @file.html --from-name A --from-email B --inclusive-tags t1,t2` | --dry-run, file-arg for html, batch-size-per-hour validation, MAX_BROADCAST_CONTENT_BYTES enforced |

### Sequences
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 17 | List sequences | bento-cli `sequences list`, bento-mcp `bento_list_sequences` | `bento sequences list` | local cache |
| 18 | Append email to sequence | bento-cli `sequences create-email`, bento-mcp `bento_create_sequence_email` | `bento sequences add-email <seq-id> --subject X --html @file --delay-days N` | --dry-run, html-validates `{{ visitor.unsubscribe_url }}` is present |

### Workflows (read-only over REST)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 19 | List workflows + email templates + stats | bento-mcp `bento_list_workflows` | `bento workflows list` | local cache, --include-templates |

### Email Templates
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 20 | Get template | bento-mcp `bento_get_email_template` | `bento templates get <id>` | --html-only for piping |
| 21 | Update template (subject/html PATCH) | bento-mcp `bento_update_email_template` | `bento templates update <id> --subject X --html @file` | --dry-run shows diff, validates `{{ visitor.unsubscribe_url }}` |

### Stats
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 22 | Site-wide stats | bento-cli `stats site`, bento-mcp `bento_get_site_stats` | `bento stats site` | --json --select, --json --csv for reporting |
| 23 | Segment stats | docs `/stats/segment?segment_id=` (no SDK) | `bento stats segment <id>` | first CLI to expose this |
| 24 | Report stats | docs `/stats/report?report_id=` (no SDK) | `bento stats report <id>` | first CLI to expose this |

### Segments (no SDK wraps these)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 25 | List segments | docs `/fetch/segments` | `bento segments list` | first CLI |
| 26 | Get segment (incl. visitor_event_query JSON) | docs `/fetch/segments/:id` | `bento segments get <id>` | first CLI; --query-only to dump the JSON criteria |

### Commands API (batch tag/field/sub ops without firing automations)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 27 | execute_commands (subscribe, unsubscribe, change_email, add_tag, remove_tag, add_field, remove_field) | python SDK `execute_commands`, /fetch/commands | `bento commands run --type add_tag --email EMAIL_PLACEHOLDER --query VIP` and `--batch @file.jsonl` | first CLI; --dry-run, typed exit 4 for invalid command type |

### Transactional Email
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 28 | Send transactional (up to 60/min, 60/batch) | ruby SDK `send_transactional`, php SDK, /batch/emails | `bento emails send --to <e> --from <author> --subject X --html @file --transactional` | --batch @file.jsonl, dry-run, batch-size enforced at 60, validates `from` is authorized Author |

### Forms (SDK-verified, undocumented)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 29 | List form responses | node + php SDKs `/fetch/responses` | `bento forms responses <form-id>` | first CLI, --since-page N |

### Experimental (list hygiene, validation)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 30 | Email validation | ruby `Bento::Spam.valid?`, /experimental/validation | `bento experimental validate <email>` | --batch via stdin |
| 31 | Stricter ruleset (jesses_ruleset) | ruby SDK, /experimental/jesses_ruleset | `bento experimental jesses <email> --block-free-providers --wiggleroom 0.2` | first CLI |
| 32 | Domain/IP blacklist check | node SDK, /experimental/blacklist.json | `bento experimental blacklist --domain example.com` | --batch via stdin |
| 33 | Content moderation | node SDK, /experimental/content_moderation | `bento experimental moderate --text "..."` | first CLI |
| 34 | Gender guess | python SDK, /experimental/gender | `bento experimental gender <name>` | first CLI |
| 35 | IP geolocation | python SDK, /experimental/geolocation?ip= | `bento experimental geo <ip>` | first CLI |

### Data Deletion (GDPR/CCPA)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 36 | Queue subscriber anonymization | docs `/data_deletion_requests` (no SDK) | `bento delete request <email> --confirm` | --dry-run shows blast-radius first, hard cap 1000/day enforced |

### Auth / Config
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 37 | auth set / status / logout | bento-cli `auth set/status/clear` | `bento auth set --pk X --sk Y --site-uuid Z`, `bento auth status` | reads BENTO_* env vars, stores in ~/.config/bento, doctor command verifies |
| 38 | profile (who am I) | bento-cli `profile` | `bento doctor` | shows site UUID, validates auth via /stats/site probe, checks User-Agent set |
| 39 | dashboard (open URL) | bento-cli `dashboard` | `bento dashboard --launch` | print URL by default, --launch opens browser, verifies PRINTING_PRESS_VERIFY guard |

**Absorbed count: 39 features across 13 resource groups.** Every Bento CLI/MCP feature matched. SDK-only features (transactional send, batch commands, spam checks) brought in. Docs-only endpoints (segments, data_deletion) wrapped for the first time.

## Transcendence (only possible with our approach)

| # | Feature | Command | Description | Group | Score |
|---|---------|---------|-------------|-------|-------|
| T1 | Snapshot Diff | `bento sync diff --since <ts>` | Show exactly which subscribers, tags, fields, or templates changed between two local snapshots since Bento has no `since` filter. | Sync Integrity | 10 |
| T2 | Hygiene Pipeline | `bento hygiene scrub --in list.csv` | Run validation + jesses_ruleset + blacklist + content_moderation in one pass, emit clean.csv + rejected.csv with reasons. | List Hygiene | 10 |
| T3 | Purchase Replay | `bento events purchase-replay --from vendure.json --dry-run` | Dry-run a Vendure order export against `$purchase` mapping and show the exact batch payload before sending. | Vendure Bridge | 10 |
| T4 | Trigger Path Lint | `bento events lint --file events.json` | Warn when payload uses `/fetch/commands subscribe` but intent (welcome flow, tag-triggered automation) requires `/batch/events`. | Safety Rails | 9 |
| T5 | Churn Risk Score | `bento subscribers churn-risk` | Rank subscribers by days-since-last-open/click against cohort baseline before the next broadcast. | Retention | 9 |
| T6 | Win-Back Cohort | `bento subscribers winback --lapsed 180d --last-purchased 90d` | Build a CSV of customers who bought once, lapsed 180+ days, and never opened the last 3 broadcasts. | Retention | 9 |
| T7 | Review Trigger Coexist | `bento events review-window --shipped-csv orders.csv --delay 10d` | Emit dated Bento event stream for orders shipped 10 days ago, skipping any order Stamped already triggered. | Vendure Bridge | 9 |
| T8 | Pre-Delete Audit | `bento subscribers pre-delete --emails-from file` | Show tags, events, revenue, active automations a subscriber set carries before filing GDPR deletion. | Safety Rails | 9 |
| T9 | Broadcast What-If | `bento broadcasts whatif --segment <id>` | Estimate audience size, predicted opens, and hygiene-risk count for a draft broadcast before queuing. | Campaign Planning | 8 |
| T10 | Tag Drift Report | `bento tags drift --window 30d` | Surface tags whose subscriber counts swung more than N% week-over-week, catching misfiring workflows. | Sync Integrity | 8 |
| T11 | Field Schema Lint | `bento fields lint --against vendure-schema.json` | Diff local custom fields against a Vendure customer-schema file; flag missing/renamed/type-mismatched. | Vendure Bridge | 8 |
| T12 | Cohort FTS | `bento subscribers find "<query>" --tagged X --purchased-after Y` | FTS5 search over subscriber notes/fields plus structured filters; exportable as --csv. | Discovery | 8 |

**Transcendence count: 12 features across 5 themes (Sync Integrity, List Hygiene, Vendure Bridge, Safety Rails, Retention, Campaign Planning, Discovery).** All scored ≥8/10. No stubs. All implementable with the SQLite store + experimental endpoint chains + local diff engines.

## Stubs
**None.** All 39 absorbed + 12 transcendence features ship as full implementations.

## Adversarial cut (features considered + rejected)
- "AI subject-line scorer" — vague, not Bento-specific. Rejected.
- "Workflow editor" — Bento workflows are read-only over REST. Rejected.
- "Deliverability dashboard" — duplicates absorbed `stats site`. Rejected.
- "Auto-resubscribe lapsed users" — would silently re-trigger automations via the two-path footgun. Unsafe by design. Rejected.

## Source Priority
Single-source CLI. Bento OpenAPI spec at `github.com/bentonow/api/blob/main/bento-api.yaml` is primary (covers 20 ops across 17 paths). SDK source crawl fills the ~10 ops the spec misses (sequences, workflows, templates, forms, data_deletion, commands batch op, jesses_ruleset, gender, content_moderation, segments list). Auth is free for read; live testing requires Bento creds (Aaron has them, not in env today).

