---
title: "Data Layer Specification: Dub CLI"
type: feat
status: active
date: 2026-03-27
phase: "0.7"
api: "dub"
---

# Data Layer Specification: Dub CLI

## Entity Classification

| Entity | Type | Est. Volume | Update Freq | Temporal Field |
|--------|------|-------------|-------------|----------------|
| Links | Accumulating | 10k-100k/workspace | Mutable (createdAt, updatedAt) | createdAt (sortable), updatedAt |
| Analytics | Append-only (aggregated) | Millions aggregated | Pre-computed, refreshed per query | start/end params |
| Events | Append-only | High (individual clicks/leads/sales) | Never edited | timestamp |
| Customers | Accumulating | 100-10k | Mutable | createdAt |
| Partners | Accumulating | 10-1k | Mutable | createdAt |
| Domains | Reference | < 50 | Rarely changes | createdAt |
| Tags | Reference | < 100 | Rarely changes | N/A |
| Folders | Reference | < 50 | Rarely changes | createdAt |
| Commissions | Accumulating | 100-10k | Mutable | createdAt |
| Payouts | Accumulating | 10-100 | Mutable | createdAt |
| Bounties | Reference | < 50 | Rarely changes | N/A |
| QR Codes | Ephemeral | On-demand | N/A | N/A |
| Embed Tokens | Ephemeral | On-demand | N/A | N/A |
| Track | Ephemeral | Write-only | N/A | N/A |

## Data Gravity Scoring

| Entity | Volume (0-3) | QueryFreq (0-3) | JoinDemand (0-2) | SearchNeed (0-2) | TemporalValue (0-2) | Total | Tier |
|--------|-------------|-----------------|-----------------|-----------------|--------------------|----|------|
| **Links** | 2 | 3 | 2 | 2 | 2 | **11** | Primary |
| **Analytics** | 3 | 3 | 1 | 0 | 2 | **9** | Primary |
| **Events** | 3 | 2 | 1 | 1 | 2 | **9** | Primary |
| Customers | 1 | 2 | 2 | 1 | 1 | **7** | Support |
| Partners | 1 | 1 | 1 | 1 | 1 | **5** | Support |
| Commissions | 1 | 1 | 2 | 0 | 1 | **5** | Support |
| Tags | 0 | 1 | 2 | 1 | 0 | **4** | API-only |
| Domains | 0 | 1 | 2 | 0 | 0 | **3** | API-only |
| Folders | 0 | 1 | 1 | 1 | 0 | **3** | API-only |
| Payouts | 0 | 1 | 1 | 0 | 1 | **3** | API-only |

**Primary (>= 8):** Links (11), Analytics (9), Events (9)
**Support (5-7):** Customers (7), Partners (5), Commissions (5)

## SQLite Schema

```sql
-- Links: Primary entity (gravity 11)
CREATE TABLE links (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    key TEXT NOT NULL,
    url TEXT NOT NULL,
    short_link TEXT NOT NULL,
    title TEXT,
    description TEXT,
    archived INTEGER NOT NULL DEFAULT 0,
    folder_id TEXT,
    external_id TEXT,
    clicks INTEGER NOT NULL DEFAULT 0,
    leads INTEGER NOT NULL DEFAULT 0,
    sales INTEGER NOT NULL DEFAULT 0,
    sale_amount REAL NOT NULL DEFAULT 0,
    last_clicked TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE INDEX idx_links_domain ON links(domain);
CREATE INDEX idx_links_folder_id ON links(folder_id);
CREATE INDEX idx_links_created_at ON links(created_at);
CREATE INDEX idx_links_updated_at ON links(updated_at);
CREATE INDEX idx_links_clicks ON links(clicks);

-- FTS5 on link text fields (url, key, title, description)
CREATE VIRTUAL TABLE links_fts USING fts5(
    url, key, title, description,
    content='links', content_rowid='rowid'
);

-- Triggers to keep FTS5 in sync
CREATE TRIGGER links_ai AFTER INSERT ON links BEGIN
    INSERT INTO links_fts(rowid, url, key, title, description)
    VALUES (new.rowid, new.url, new.key, new.title, new.description);
END;
CREATE TRIGGER links_ad AFTER DELETE ON links BEGIN
    INSERT INTO links_fts(links_fts, rowid, url, key, title, description)
    VALUES ('delete', old.rowid, old.url, old.key, old.title, old.description);
END;
CREATE TRIGGER links_au AFTER UPDATE ON links BEGIN
    INSERT INTO links_fts(links_fts, rowid, url, key, title, description)
    VALUES ('delete', old.rowid, old.url, old.key, old.title, old.description);
    INSERT INTO links_fts(rowid, url, key, title, description)
    VALUES (new.rowid, new.url, new.key, new.title, new.description);
END;

-- Analytics snapshots: Primary entity (gravity 9)
CREATE TABLE analytics_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_by TEXT NOT NULL,     -- 'count', 'timeseries', 'countries', etc.
    event_type TEXT NOT NULL,   -- 'clicks', 'leads', 'sales', 'composite'
    period_start TEXT NOT NULL, -- ISO date
    period_end TEXT NOT NULL,   -- ISO date
    synced_at TEXT NOT NULL,
    data JSON NOT NULL          -- full response array
);

CREATE INDEX idx_analytics_group_period ON analytics_snapshots(group_by, period_start, period_end);
CREATE INDEX idx_analytics_synced ON analytics_snapshots(synced_at);

-- Events: Primary entity (gravity 9)
CREATE TABLE events (
    id TEXT PRIMARY KEY,        -- click_id for clicks, eventId for leads/sales
    event_type TEXT NOT NULL,   -- 'clicks', 'leads', 'sales'
    link_id TEXT,
    domain TEXT,
    key TEXT,
    url TEXT,
    country TEXT,
    city TEXT,
    device TEXT,
    browser TEXT,
    os TEXT,
    referer TEXT,
    timestamp TEXT NOT NULL,
    sale_amount REAL,
    data JSON NOT NULL
);

CREATE INDEX idx_events_link_id ON events(link_id);
CREATE INDEX idx_events_event_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp);
CREATE INDEX idx_events_country ON events(country);

-- FTS5 on events (url, referer for search)
CREATE VIRTUAL TABLE events_fts USING fts5(
    url, referer,
    content='events', content_rowid='rowid'
);

-- Customers: Support entity (gravity 7)
CREATE TABLE customers (
    id TEXT PRIMARY KEY,
    name TEXT,
    email TEXT,
    country TEXT,
    external_id TEXT,
    sales INTEGER NOT NULL DEFAULT 0,
    sale_amount REAL NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE INDEX idx_customers_email ON customers(email);
CREATE INDEX idx_customers_country ON customers(country);
CREATE INDEX idx_customers_created_at ON customers(created_at);

-- Sync state tracking
CREATE TABLE sync_state (
    entity TEXT PRIMARY KEY,
    last_cursor TEXT,
    last_synced_at TEXT NOT NULL,
    total_synced INTEGER NOT NULL DEFAULT 0
);
```

## Sync Strategy

### Links Sync
- **Cursor:** `startingAfter` param (link ID-based cursor pagination)
- **Sort:** `sortBy=createdAt&sortOrder=desc` (API only supports createdAt, clicks, saleAmount, lastClicked sort)
- **Limitation:** No `updatedAt` sort. Cannot efficiently fetch only updated links.
- **Strategy:** Full sync on first run (paginate through all). Incremental sync: paginate from cursor, compare updatedAt locally to detect changes. Upsert with ON CONFLICT to handle updates.
- **Batch size:** 100 (API max pageSize)
- **Rate limit:** 60 req/min free, 600 req/min pro. At 100 links/page, can sync 6000 links/min on free, 60000/min on pro.

### Analytics Sync
- **Cursor:** `start` and `end` date params
- **Strategy:** Pull analytics for the period since last sync. Store snapshots per (groupBy, event_type, period).
- **Dimensions to sync:** count, timeseries, countries, devices, browsers, os, referers, top_links, top_urls
- **Rate limit:** Analytics API has separate per-second limits (2/sec on Pro). Sync one dimension at a time with 500ms delay.

### Events Sync
- **Cursor:** timestamp-based. Track last event timestamp in sync_state.
- **Strategy:** GET /events?sortBy=timestamp&sortOrder=desc&limit=1000. Paginate until we see events older than last sync timestamp. Dedup by click_id/eventId.
- **Batch size:** 1000 (API max limit)

### Customers Sync
- **Cursor:** page-based. Track last page.
- **Strategy:** Paginate through all customers. Upsert with ON CONFLICT.

## Domain-Specific Search Filters

| CLI Flag | SQL WHERE Clause | Entity |
|----------|-----------------|--------|
| `--domain <d>` | `WHERE domain = ?` | links, events |
| `--tag <t>` | `WHERE data->'tags' LIKE '%"name":"<t>"%'` | links |
| `--folder <f>` | `WHERE folder_id = ?` | links |
| `--archived` | `WHERE archived = 1` | links |
| `--country <c>` | `WHERE country = ?` | events, customers |
| `--device <d>` | `WHERE device = ?` | events |
| `--browser <b>` | `WHERE browser = ?` | events |
| `--event <e>` | `WHERE event_type = ?` | events |
| `--since <date>` | `WHERE created_at >= ?` or `WHERE timestamp >= ?` | links, events |
| `--before <date>` | `WHERE created_at <= ?` or `WHERE timestamp <= ?` | links, events |
| `--min-clicks <n>` | `WHERE clicks >= ?` | links |
| `--no-clicks` | `WHERE clicks = 0` | links (stale command) |

## Compound Queries (5)

### 1. Stale links with domain breakdown
```sql
SELECT domain, COUNT(*) as stale_count,
       GROUP_CONCAT(short_link, char(10)) as links
FROM links
WHERE clicks = 0
  AND created_at < datetime('now', '-30 days')
GROUP BY domain
ORDER BY stale_count DESC;
```

### 2. Click events by country for a specific link
```sql
SELECT e.country, COUNT(*) as clicks,
       l.short_link, l.url
FROM events e
JOIN links l ON e.link_id = l.id
WHERE l.key = ?
GROUP BY e.country
ORDER BY clicks DESC;
```

### 3. Top performing links in a folder
```sql
SELECT short_link, url, title, clicks, leads, sales, sale_amount
FROM links
WHERE folder_id = ?
ORDER BY clicks DESC
LIMIT 20;
```

### 4. Customer conversion funnel
```sql
SELECT
    COUNT(*) as total_customers,
    SUM(CASE WHEN sales > 0 THEN 1 ELSE 0 END) as converted,
    ROUND(100.0 * SUM(CASE WHEN sales > 0 THEN 1 ELSE 0 END) / COUNT(*), 1) as conversion_rate,
    SUM(sale_amount) as total_revenue
FROM customers
WHERE created_at >= ?;
```

### 5. Recent events with link context
```sql
SELECT e.event_type, e.timestamp, e.country, e.device,
       l.short_link, l.url, l.title
FROM events e
LEFT JOIN links l ON e.link_id = l.id
ORDER BY e.timestamp DESC
LIMIT 50;
```

## Tail Strategy

- **Method:** REST polling (no WebSocket/SSE available)
- **Implementation:** GET /events?sortBy=timestamp&sortOrder=desc&limit=10, poll every 5 seconds
- **Cursor:** Track last seen timestamp, filter client-side
- **Display:** Stream new events to terminal as they appear
- **Optimization:** Respect rate limits (60/min free). At 12 polls/min (every 5s), leaves budget for other commands.

## Commands to Build in Phase 4 Priority 0

1. `sync` - Full/incremental sync of links, events, analytics to local SQLite
2. `search` - FTS5 search across links (url, key, title, description)
3. `sql` - Raw read-only SQL queries against local database
4. `snapshot` - Pull analytics for date range, store locally
5. `stale` - Find links with 0 clicks (compound query #1)
6. `tail` - Stream recent events via REST polling
