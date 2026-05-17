# Google Reviews CLI — Generation Plan

## API Identity

**Name:** `google-reviews`  
**Base URL:** `https://www.google.com`  
**Auth:** None required for public reviews. Cookies optional (improves results).  
**No API key required.** Uses Google Maps internal endpoints reverse-engineered from browser traffic.

## CLI Name: `google-reviews`

## Commands

### 1. `google-reviews reviews <place-url-or-cid>`

Fetch reviews for a business by Google Maps URL or CID.

**Positional arg:** Google Maps URL or CID (format: `HEX_LO:HEX_HI`)

**Flags:**
- `--count int` — Reviews per request (default: 20, max: 20)
- `--sort string` — Sort order: `relevant` (default), `newest`, `highest`, `lowest`
- `--lang string` — Language code (default: `en`)
- `--country string` — Country code (default: `us`)
- `--all` — Fetch all available reviews (auto-paginate)
- `--output string` — Output file path (writes JSON; stdout if omitted)
- `--json` — Output as JSON (default human table)

**URL parsing:** Extract the CID from a Google Maps URL like:
`https://www.google.com/maps/place/.../@lat,lng,.../data=!3m5!1s0xHEX_LO:0xHEX_HI!...`
The CID is the `0xHEX_LO:0xHEX_HI` part after `!1s`.

**Feature ID derivation:**
Given CID `0xHEX_LO:0xHEX_HI`:
- `lo = uint64(HEX_LO)` — Parse hex string as unsigned 64-bit integer  
- `hi = uint64(HEX_HI)` — Parse hex string as unsigned 64-bit integer

**pb parameter construction:**
```
sort_code = 1  // relevant
sort_code = 2  // newest
sort_code = 3  // highest
sort_code = 4  // lowest

pb = fmt.Sprintf("!1m2!1y%d!2y%d!2m2!1i%d!2i%d!3e%d", lo, hi, count, offset, sort_code)
```

**Endpoint:**
`GET /maps/preview/review/listentitiesreviews?authuser=0&hl={lang}&gl={country}&pb={pb}`

**Response parsing:**
The response starts with `)]}'\n` — strip this prefix before JSON parsing.

```
data[2] = array of reviews
data[5] = [5-star-count, 4-star-count, 3-star-count, 2-star-count, 1-star-count]

Each review (data[2][i]) fields:
[0]  = [profile_url, name, photo_url, review_count_token, ...] (author info)
[1]  = relative date string ("3 months ago")
[3]  = review text (string, may be null for photos-only reviews)
[4]  = rating (integer 1-5)
[6]  = reviewer Google user ID (string)
[10] = review ID (base64 string)
[12] = [null, [[badge_level, null, null], total_reviews, total_photos, photo_url]]
[18] = review permalink URL
[27] = timestamp (Unix milliseconds)
[32] = review language code
[61] = pagination cursor for next page (format: "BASE64:OFFSET")
```

**Pagination:**  
- First call: offset=0
- From last review: `[61]` contains pagination token `"BASE64:N"` where N=items fetched
- Next page: increment offset by count (e.g., offset=20 for page 2)
- Stop when response returns 0 reviews

**Human output format (table):**
```
Rating  Author          Date           Review
5 ★     Kenneth Hiner   3 months ago   Stopped by Shake Shack...
4 ★     Jane Smith      1 month ago    Great burgers...
```

**JSON output format (one object per line):**
```json
{"review_id":"Ci9...", "rating":5, "author":"Kenneth Hiner", "date":"3 months ago", "timestamp_ms":1769986041693, "text":"Stopped by...", "language":"en", "reviewer_id":"107495387645373734757", "review_url":"https://..."}
```

### 2. `google-reviews summary <place-url-or-cid>`

Get rating summary and distribution for a business.

**Endpoint:** Same as `reviews` but extracts `data[5]` from first response.

**Output:**
```
Overall: 4.5 stars (7,713 reviews)
5 ★  64%  ████████████████████ 4,958
4 ★  24%  ████████             1,885
3 ★   7%  ███                    573
2 ★   2%  █                      175
1 ★   2%  █                      122
```

### 3. `google-reviews search <query>`

Search for businesses by name and get their CIDs.

**Note:** This command uses the Google Maps web search page and parses embedded JSON. It does NOT require an API key.

**Endpoint:** `GET https://www.google.com/maps/search/?api=1&query={query}`  
**Parse strategy:** Extract place data from the HTML response's embedded JSON.

**Output:**
```
Name                                  CID                                    Address
Shake Shack Madison Square Park  0x89c258bc949d58cf:0x84ac8a2dc2535dc2  Madison Ave & 23rd St, New York, NY
Shake Shack Times Square         0x89c259...:0x84ac...                 Eighth Ave, New York, NY
```

### 4. `google-reviews place <place-url-or-cid>`

Get business details (name, address, phone, hours, overall rating).

**Endpoint:** `GET /maps/preview/place?authuser=0&hl={lang}&gl={country}&q={name}&pb=...`

**pb parameter for place:**
```
pb = fmt.Sprintf("!1m17!1s0x%x:0x%x!2s%s", lo, hi, url_encode(name))
```

**Response parsing (data[6]):**
- `data[6][0]` = session ID
- `data[6][2]` = [place_name, address, ...]

## Data Store (SQLite)

Store fetched reviews locally for offline search and historical tracking.

**Schema:**
```sql
CREATE TABLE places (
    cid_lo TEXT NOT NULL,
    cid_hi TEXT NOT NULL,
    name TEXT,
    address TEXT,
    overall_rating REAL,
    review_count INTEGER,
    fetched_at DATETIME,
    PRIMARY KEY (cid_lo, cid_hi)
);

CREATE TABLE reviews (
    review_id TEXT PRIMARY KEY,
    cid_lo TEXT NOT NULL,
    cid_hi TEXT NOT NULL,
    rating INTEGER,
    text TEXT,
    author_name TEXT,
    reviewer_id TEXT,
    timestamp_ms INTEGER,
    language TEXT,
    review_url TEXT,
    fetched_at DATETIME
);

CREATE VIRTUAL TABLE reviews_fts USING fts5(
    review_id UNINDEXED,
    text,
    author_name,
    content='reviews',
    content_rowid='rowid'
);
```

## Store Commands

### `google-reviews store sync <place-url-or-cid>`

Fetch and upsert all reviews for a place into the local SQLite store.

### `google-reviews store search <query>`

Full-text search across stored reviews.

### `google-reviews store export <place-url-or-cid>`

Export stored reviews to CSV or JSON.

## HTTP Client Notes

- No auth header required
- User-Agent: standard browser UA
- Rate limit: ~10 req/s safe; implement 100ms delay between pages
- `gl` (country) and `hl` (language) affect results — US English recommended for completeness
- The `)]}'\n` anti-XSSI prefix MUST be stripped from all responses

## Competitive Position

Only Go CLI using internal JSON endpoints — no API key, no Playwright, no paid service.
- **vs gosom/google-maps-scraper**: No Playwright overhead, token-efficient output
- **vs Places API**: No 5-review limit, no API key, no cost

