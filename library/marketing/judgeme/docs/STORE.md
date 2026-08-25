# Judge.me review store

`judgeme-pp-cli sync --resources reviews --full` writes a count-verified snapshot to the CLI's standard SQLite database. The command fetches Judge.me's live `/reviews/count`, partitions corpora over 10,000 rows by rating, deduplicates by review ID, and replaces the stored corpus only after the final unique-ID count matches the live count. If a rating partition itself exceeds 10,000 rows, the sync attempts date-window subdivision and fails closed if Judge.me ignores those filters.

The typed `reviews` table is the downstream integration contract:

| Column | Contract |
|---|---|
| `id` | Judge.me review ID and primary key |
| `data` | Original review JSON |
| `synced_at` | UTC snapshot timestamp |
| `body` | Original review body |
| `body_hash` | SHA-256 of lowercase, trimmed, whitespace-collapsed body text; empty bodies have `NULL` |
| `published` | Storefront-visible flag from Judge.me |
| `hidden` | Hidden moderation flag from Judge.me |
| `rating` | Numeric review rating |
| `product_external_id` | External product/SKU association |
| `product_handle`, `product_title` | Product display associations |
| `created_at`, `updated_at` | Judge.me review timestamps |

`body_hash` identifies syndicated or repeated bodies; it does not decide which product association is canonical. Consumers should use `reviews unique-bodies` when body-level evidence is the unit of analysis and `reviews syndication` before attributing language to a product.

Population definitions are explicit:

- `published`: `published = true`
- `hidden`: `hidden = true`
- `pending`: neither published nor hidden
- `all`: every row returned by the authenticated API

The raw `resources` table also receives the same verified review rows for compatibility with the standard `search`, `sql`, and downstream store tooling.
