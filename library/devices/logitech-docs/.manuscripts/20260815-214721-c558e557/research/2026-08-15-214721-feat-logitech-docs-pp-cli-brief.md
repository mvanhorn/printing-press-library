# logitech-docs CLI Brief

## API Identity
- Domain: Logitech support documentation — user manuals, spec sheets, install/setup guides, downloads, FAQs, warranty.
- Host: `support.logi.com` is a **Zendesk Help Center** exposing the standard **Zendesk Help Center API v2** (`/api/v2/help_center/...`), JSON, no auth, behind Cloudflare.
- Users: AV integrators/technicians, IT admins, support staff, consumers. Read-heavy reference content: 33k+ articles across 9 categories / 62 sections.
- Data profile: reference docs; doc type is encoded in `label_names` (`webcontent=<type>`, `webproduct=<uuid>`).

## Reachability Risk
- None. `probe-reachability` on `/api/v2/help_center/en-us/articles.json` → `mode: standard_http` (confidence 0.95): stdlib 200 + surf-chrome 200, `application/json`. No clearance cookie, no browser transport.
- Probe-safe endpoint used: `GET /api/v2/help_center/en-us/articles.json`

## Top Workflows
1. Get the spec sheet for a product → search → filter `webcontent=productspecs` → read dimensions/compatibility.
2. Get the install/setup guide → search → `webcontent=productgettingstarted`.
3. Download the manual/firmware/software → `webcontent=productdocument` / `webcontent=productdownload` → extract `download01.logi.com` links.
4. List every doc for a product family → browse category → section → articles.
5. Full-text search inside the manuals (offline) → local FTS index of article bodies.

## Table Stakes
- Search by text (`/api/v2/help_center/articles/search.json`)
- Browse by category/section (product hierarchy)
- Read article body (HTML → plain text)
- Filter by doc type (label taxonomy)
- Download files (`download01.logi.com`)

## Data Layer
- Primary entities: Category, Section (product family), Article (title, body HTML, labels, section_id, timestamps), download links.
- Sync cursor: `updated_at` per article + Zendesk `next_page` / `page_count` pagination.
- FTS/search: local SQLite FTS5 over article title + body text (HTML stripped) → offline full-text search.

## Doc-type taxonomy (`label_names`)
| Friendly | Label |
|---|---|
| spec sheet | `webcontent=productspecs` |
| manual / document | `webcontent=productdocument` |
| install / setup | `webcontent=productgettingstarted` |
| faq | `webcontent=productfaq` |
| download | `webcontent=productdownload` |
| warranty | `webcontent=productwarranty` |
| video | `webcontent=productvideo` |

## Product Thesis
- Name: `logitech-docs`
- Why it should exist: support.logi.com is a JS-heavy Zendesk portal with a hidden-but-clean JSON API. A CLI turns 33k reference docs into a greppable, offline, agent-native surface — no browser, no Cloudflare friction per lookup, no ad-laden search. Nobody ships a local full-text index of Logitech manuals today.

## Build Priorities
1. Search + doc-type filter (`docs spec|manual|install|faq|download <query>`).
2. Offline full-text index (`sync` scoped by category/section/label → SQLite FTS5 over article bodies).
3. Download resolver (extract `download01.logi.com` links, fetch, `--dry-run`, cache).
4. PDF text extraction for manual attachments (full-text search inside PDF manuals).
