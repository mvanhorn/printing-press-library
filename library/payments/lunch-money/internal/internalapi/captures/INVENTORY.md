# Internal API — full endpoint inventory

Verified live via `lunch-money-pp-cli internal request <path>` against `api.lunchmoney.app`
(cookie auth, see `../README.md`).

## User / billing / system
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/me` | yes | User profile (id, email, name, primary_currency, tier, stripe status) |
| GET | `/me/canny_token` |  | Canny SSO token for help-center widget |
| PUT | `/me` |  | Handler exists but same-value profile probes returned 500; body shape still unknown |
| GET | `/v2/me` |  | Slim profile read incl. `budget_name`; `PUT /v2/me` 404s |
| GET | `/billing` | yes | Stripe subscription state, plan, balance, trial info |
| GET | `/system/status` | yes | Health probe (204) |
| GET | `/notifications` | yes | In-app notifications array |
| GET | `/gdpr` | yes | GDPR-related events array |
| GET | `/currencies` | yes | Full currency catalog (eth, usd, ...) |
| GET | `/api_tokens` | yes | List of issued API tokens |
| DELETE | `/api_tokens/{id}` | yes | Revoke a token |

## Auth
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| POST | `/auth/token/refresh` | yes | JWT refresh (auto on 401 by internal client) |

## Accounts (assets)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/assets` | yes | Full list incl. Plaid access_tokens (**sensitive**) |
| POST | `/assets` | yes | Create manual account |
| PUT | `/assets/{id}` | yes | Update manual account |
| GET | `/assets/{id}/status` | yes | Dependency check before delete |
| PUT | `/assets/{id}/delete` | yes | Soft-delete with keep_items flag |
| GET | `/assets/subtypes` | yes | Valid type/subtype enum |

## Categories
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/categories` |  | Nested form (`{nested: [...]}`) |
| GET | `/v2/categories` |  | Flat form, same shape as public API |
| POST | `/v2/categories` |  | Create — same as public API |
| GET | `/v2/categories/{id}` |  | Get one |
| PUT | `/v2/categories/{id}` |  | Update |
| DELETE | `/v2/categories/{id}` |  | Delete |

## Tags
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/v2/tags` |  | Same as public API |
| POST | `/v2/tags` |  | Create |
| PUT | `/v2/tags/{id}` |  | Update |
| DELETE | `/v2/tags/{id}` |  | Delete |

## Rules (internal-only — public API does not expose)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/rules?offset=N&limit=M` | yes | Wrapper `{rules, total_returned}` |
| POST | `/rules` | yes | Create — `{conditions, actions}` body |
| PUT | `/rules/{criteria_id}` | yes | Update — use `criteria_id` not `rule_id` |
| POST | `/rules/apply` | yes | `{criteria_ids, dry_run, include_transaction_ids}` |
| POST | `/rules/bulk_delete` | yes | `{criteria_ids: [...]}` |
| GET | `/rules/suggested?criteria_id={id}` |  | Suggested rule expansions |
| GET/POST | `/rules/suggested*` variants |  | 2026-05-14 probes returned 400/404; accept flow still unknown |

## Recurring items
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/recurring_items` | yes | All recurring items |
| GET | `/recurring_items?start_date=...&end_date=...` |  | Filtered |
| GET | `/recurring_items/{id}` |  | One item — verify shape |
| GET | `/v2/recurring_items?include_suggested=true` |  | Wrapper `{recurring_items:[...]}`; this account returned suggested items only |

## Budget / summary
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/summary?start_date=&end_date=&include_*=` | yes | The Budget page; rich include flags |
| PUT | `/v2/budgets` | yes | Set/update category period budget; same body as public v2 |
| DELETE | `/v2/budgets?category_id=&start_date=` | yes | Clear category period budget |

Useful include flags discovered:
`include_exclude_from_budgets`, `include_occurrences`, `include_recurring_items`,
`include_totals`, `include_rollover_pool`, `include_budget_properties`,
`include_past_budget_dates`.

## Transactions
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/transactions?...` | yes | List with rich filters |
| PUT | `/transactions/{id}` | yes | Update single — any writable field |
| PUT | `/transactions/bulk_update` | yes | Universal bulk-edit |
| POST | `/transactions/group` |  | Group N transactions (verify shape) |
| DELETE | `/transactions/group/{id}` |  | Ungroup |
| POST | `/transactions/split/{id}` |  | Split into children |
| POST | `/transactions/new` |  | Create single (verify shape) |
| POST | `/transactions/import` |  | CSV import (multipart) |
| POST | `/transactions/{id}/attachments` |  | Upload attachment |
| GET | `/transactions/attachments/{file_id}` |  | Download attachment |
| PUT | `/transactions/bulk_insert/check` | yes | CSV-import dry-run check; body `{transactions, apply_rules, skip_duplicates, create_new_categories}` |
| PUT | `/transactions/bulk_insert` | yes | CSV-import commit; client helper only, use cautiously |
| GET | `/import_configs` | yes | Saved CSV column mappings |
| PUT | `/import_configs` | yes | Save CSV column mapping preset |

Useful query params on LIST: `start_date`, `end_date`, `is_unreviewed=true`,
`match=all`, `paginate=true`, `minimal=true`, `exclude_pending=true`,
`exclude_parents=true`, `date_range=Last+6+months`.

## Plaid / autocategorization
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/plaid/categories` | yes | Plaid taxonomy + LM mappings |
| POST | `/plaid/categories/populate` | yes | Refresh taxonomy (sets `count` fields) |

## Snapshots (boot/page-load data)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/snapshot/transactions_page` | yes | `{transactionMonths, lastImport}` |
| GET | `/snapshot/tags_page` | yes | `{allTags, tagColors}` |

## Balance history (net worth)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/balance_history?start_date=&end_date=` | yes | Per-month assets/liabilities/net_worth |

## Analytics
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/trends?start_date=&end_date=&include_recurring=&include_exclude_from_totals=&group_by=` | yes | Aggregations. `group_by`: `category`, `payee`, `tag`, `asset`, `type`. Returns `{data:{categories,expenses_count,expenses_total,...}}` |
| GET | `/stats?...` | yes | Same params; returns `by_price_desc` — top transactions by price |
| GET | `/calendar?start_date=&end_date=&include_recurring=` | yes | Daily grid keyed by ISO date, with `tx_expense` + `tx_income` |

## Plaid accounts (internal /v2)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/v2/plaid_accounts` | yes | `{plaid_accounts: [...]}` — richer than `/assets` (`plaid_item_id`, `display_name`, `last_import`) |
| GET | `/v2/plaid_accounts/{id}` | yes | One Plaid account |
| POST | `/v2/plaid_accounts/fetch?id={id}` | yes | Queue Plaid fetch; returns 202 |

## v2 versioned variants (work alongside their non-v2 counterparts)
| Method | Path | Notes |
|---|---|---|
| GET | `/v2/me` | Slim profile incl. `api_key_label` |
| GET | `/v2/transactions` | Wrapper shape `{transactions:[...]}` |
| GET | `/v2/categories` | Flat list |
| GET | `/v2/categories/{id}` | One category |
| PUT | `/v2/categories/{id}` | Update (incl. `archived:true`) |
| GET | `/v2/tags` | Wrapper `{tags:[]}` |
| GET | `/v2/recurring_items` | Wrapper `{recurring_items:[]}` |

## API tokens (continued)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| POST | `/api_tokens` | yes | Body: `{"label":"...","apiKeyReason":null,"apiKeyReasonText":""}` → returns the raw token as a JSON string |

## Referral / tag colors
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| GET | `/referral` | yes | `{token, count_active, users}` |
| GET | `/tags/colors` | yes | Per-tag color overrides (`{}` if none) |

## Transactions (continued)
| Method | Path | Wired | Notes |
|---|---|:-:|---|
| POST | `/transactions/group` |  | Exists (500 on bad body). Body shape TBD — capture next time |
| POST | `/transactions/split/{id}` |  | Exists per bundle, payload shape TBD |
| POST | `/transactions/new` |  | Single-transaction create |
| POST | `/transactions/import` |  | CSV import (multipart) |
| POST | `/transactions/{id}/attachments` |  | Upload attachment |
| GET | `/transactions/attachments/{file_id}` |  | Download attachment |

## Public-API mount (same host, different auth)
`api.lunchmoney.app/v1/*` exists at the same host and accepts `Authorization: Bearer <api_token>`. This is the **documented public API** served from the same hostname as the internal API. Useful when you've created an API token via `internal api-tokens create` and want bearer-token semantics instead of cookie auth.

## Frontend-only routes (no direct API)
These map to React Router paths, not API endpoints. Their data is derived client-side from the endpoints above:
- `/merchants` — derived from `/transactions`
- `/analyze` — derived from `/transactions`
- `/overview` — uses `/recurring_items` + `/summary`
- `/budget` — uses `/summary`
- `/settings`, `/profile`, `/billing` (page), `/community`, `/refer`, `/developers` — UI only; data from `/me`, `/billing` (endpoint), `/api_tokens`, `/referral`
