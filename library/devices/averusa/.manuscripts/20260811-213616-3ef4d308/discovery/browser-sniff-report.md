# Browser-Sniff Discovery Report — averusa.com

## 1. User Goal Flow
- **Goal:** Get the user manual / spec sheet / white paper for a specific AVer model, plus enumerate the document catalog.
- **Steps completed:**
  1. Opened support portal `https://averusa.my.site.com/support/s/` (Salesforce Experience Cloud community).
  2. Clicked "Conference Cameras" category → Knowledge Articles list (titles, view counts, dates).
  3. Opened article "Where can I download the latest Firmware Version for the CAM570?" → revealed article URL pattern + "HERE" download link.
  4. Opened "CAM540 Manual" article → revealed `putFileLink` (JS-resolved PDF) → clicked it → captured the file endpoint.
- **Coverage:** support portal homepage, one category article list, 2 article pages + related/trending article fetches (203 unique article record ids captured via Aura responses).

## 2. Pages & Interactions
- `https://averusa.my.site.com/support/s/` — loaded; category list (Interactive Flat Panels, Pro AV PTZ, Conference Cameras, Charging, Document Cameras, EVC/SVC, Software, Wireless).
- `Conference Cameras` category click → article list page.
- CAM570 firmware article → `https://averusa.my.site.com/support/s/article/Where-can-I-download-the-latest-Firmware-Version-for-the-CAM570`.
- CAM540 manual article → `https://averusa.my.site.com/support/s/article/CAM540-Manual` → clicked "CAM540 Manual.pdf" (JS link).

## 3. Browser-Sniff Configuration
- **Backend:** agent-browser 0.33.2 with a dedicated profile (`~/.agent-browser-press`) attaching to an externally-launched Chrome instance (outside the agent sandbox). browser-use harness was blocked by macOS TCC/AppleScript privilege violations; agent-browser's own auto-launch failed ("CDP response channel closed") until the dedicated-profile Chrome was provided.
- **Pacing:** manual; ~467 requests captured across the flow.
- **HAR:** `$DISCOVERY_DIR/browser-sniff-capture.har` (96 log entries, 9.7 MB, bodies included).

## 4. Endpoints Discovered (replayable)
| Method | Path | Content-Type | Notes |
|---|---|---|---|
| GET | `https://averusa.my.site.com/support/servlet/fileField?entityId={kaId}&field=File__Body__s` | `application/pdf` | **Manual/spec PDF download. Verified over plain curl (no cookies): 200, real PDF.** |
| GET | `https://averusa.my.site.com/support/s/sitemap.xml` → `sitemap-topicarticle-1.xml` | XML | **737 Knowledge article URLs** (plain HTTP, any UA). |
| GET | `https://averusa.my.site.com/support/s/sitemap-topic-1.xml` | XML | **33 topic URLs** (plain HTTP). |
| GET | `https://averusa.my.site.com/support/s/article/{URL-Name}` | HTML | Article SSR **for crawler UAs** (Googlebot → full HTML with title, doc-name links, body; default UA → 17 KB JS shell). |
| POST | `https://averusa.my.site.com/support/s/sfsites/aura?r=N&<action>` | JSON | Salesforce Aura API (getArticleVersions, ArticleView.addArticleViewCount, getTopics, getRecord…). Replayable in principle (fwuid in shell HTML) but **not yet verified over plain HTTP**; used by browser only in this run. |
| GET | `https://www.averusa.com/products/{category}/{model}/` | HTML | Product pages (plain HTTP, verified earlier) with datasheet links. |
| GET | `https://www.averusa.com/business/downloads/datasheet-brochure/{model}-datasheet.pdf` | `application/pdf` | Datasheet (spec sheet) PDFs (plain HTTP, verified). |

## 5. Traffic Analysis
- `$DISCOVERY_DIR/traffic-analysis.json`: analyzer emitted only 2 endpoint clusters (GET/POST `/support/s/sfsites/aura`) and flagged `reachability.mode: browser_required` (Cloudflare header markers on the SPA shell). **Correction from direct verification:** the SPA shell is JS-only for default UAs, but the article/download/sitemap surfaces ARE plain-HTTP replayable via the crawler-UA + fileField paths above — the `browser_required` verdict applies to the *Aura UI*, not the data surfaces this CLI needs.
- **Auth signals:** none. All document/download surfaces are public.
- **Protection signals:** Salesforce community behind Cloudflare (headers), but no CAPTCHA blocked any data fetch; fileField + sitemaps + SSR all returned 200 via curl.

## 6. Coverage Analysis
- **Covered:** full article URL index (737), topic index (33), article SSR content, manual PDF download (verified end-to-end: CAM540 manual = 1.2 MB `%PDF-1.5`), product-catalog + datasheets on the main site.
- **Gap:** article → `entityId` mapping for the fileField download is only partially seeded (203 ids from the capture). Extending it to all 737 articles requires replaying the Aura `getArticleVersions` action (fwuid + token) or browser execution. Spec will mark Aura replay as a `known-gap` / optional enrichment; the CLI still works for any article whose entityId is known or seeded, and for all metadata.

## 7. Response Samples
- Manual PDF: `https://averusa.my.site.com/support/servlet/fileField?entityId=ka00y000000DGArAAO&field=File__Body__s` → 1,236,713 bytes, `application/pdf`, `%PDF-1.5`.
- Article SSR (Googlebot UA): `<title>CAM540 Manual</title>`, contains `putFileLink … href="javascript:void(0);">CAM540 Manual.pdf</a>` and related-article links.
- Sitemap topicarticle-1: 737 `<loc>` entries, e.g. `…/article/AVer-TR530-TR530-Plus-No-USB-Stream-Output`, `…/article/C36i-blinking-orange-light`.

## 8. Rate Limiting Events
- None observed (no 429s during capture).

## 9. Authentication Context
- **No authenticated session used.** All surfaces are public. Session state was not captured; `$SESSION_DIR` remains empty.

## 10. Bundle Extraction
- Not run. The Salesforce Aura bundle is not needed: the data surfaces were enumerated via sitemaps + SSR instead.
