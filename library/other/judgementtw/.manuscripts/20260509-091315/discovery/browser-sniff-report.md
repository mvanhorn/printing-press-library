# Browser-Sniff Report — judgementTW

**Approach:** Direct curl probing of both ASP.NET WebForms sites with browser User-Agent.
ASP.NET sites are deterministic from `__VIEWSTATE` + `__EVENTVALIDATION` tokens that are
embedded in HTML; no JS execution needed. This is faster than browser-use and produces the
same evidence.

**Effective rate:** ~6 requests over the discovery session, well below any rate-limit
threshold. No 429s observed.

---

## Source 1: FJUD (judgment.judicial.gov.tw)

**Goal:** Discover the search → result list → judgment detail flow with replayable HTTP.

**Transport:** standard_http (probe confirmed). Stdlib HTTP with Chrome-style User-Agent
returns 200 OK for every probed path. ASP.NET session cookie (`ASP.NET_SessionId`) is
set on first GET; passing it back is sufficient for the subsequent POST and result-list
GET.

### Endpoints

| Method | Path | Purpose | Replay strategy |
|--------|------|---------|-----------------|
| GET | `/FJUD/Default_AD.aspx` | Search form (extracts `__VIEWSTATE`/`__EVENTVALIDATION` and court list) | HTML scrape of hidden fields and `<select id="jud_court">` options |
| POST | `/FJUD/Default_AD.aspx` | Submit search (returns iframe-wrapped result-list URL with `q` token) | Form POST with all hidden fields + search params; parse `qryresultlst.aspx?ty=JUDBOOK&q=<token>` URL out of the response body |
| GET | `/FJUD/qryresultlst.aspx?ty=JUDBOOK&q={token}` | Paginated judgment list | HTML scrape `<a href="data.aspx?ty=JD&id=<JID>&ot=in">` items; total count in `<span class="badge">` |
| GET | `/FJUD/data.aspx?ty=JD&id={JID}&ot=in` | Single judgment detail | HTML scrape `<div class="int-table" id="jud">` for metadata + `<div class="htmlcontent">` for body text |
| GET | `/FILES/{court}/{jid_url_encoded}.pdf` | Direct PDF download (when judgment has PDF attachment) | Plain HTTP GET, save bytes |

### Search Form Fields

| Field | Type | Purpose |
|-------|------|---------|
| `__VIEWSTATE`, `__VIEWSTATEGENERATOR`, `__EVENTVALIDATION` | hidden | ASP.NET WebForms tokens (extract from GET, replay on POST) |
| `judtype` | hidden = `JUDBOOK` | Search mode (judgment book) |
| `whosub` | hidden = `0` | Submission state |
| `jud_sys` | multi-checkbox | Case type: `C`=憲法, `V`=民事, `M`=刑事, `A`=行政, `P`=懲戒 |
| `jud_court` | multi-select | Court code (41 courts; see `courts.json` below) |
| `jud_year` | string (3 digits, ROC year, e.g. `110`) | 案件年度 |
| `jud_case` | string | 字別 (case character, e.g. `毒抗`, `台上`) |
| `jud_no`, `jud_no_end` | string | Case number range |
| `dy1`/`dm1`/`dd1` to `dy2`/`dm2`/`dd2` | string | Date range, ROC calendar |
| `jud_title` | string | 裁判案由 (case reason) substring |
| `jud_jmain` | string | 主文 (verdict main text) substring |
| `jud_kw` | string | Free-text keyword in body |
| `KbStart`, `KbEnd` | string | File size range (KB) |
| `ctl00$cp_content$btnQry` | submit = `送出查詢` | Submit button name |

### Court List (41 courts, sample)

Format: `code → name`. Full list embedded in `courts.json` companion artifact.

```
JCC  憲法法庭                 TPS  最高法院
TPC  司法院刑事補償法庭      TPA  最高行政法院
TPU  司法院－訴願決定        TPP  懲戒法院－懲戒法庭
TPH  臺灣高等法院             TPJ  懲戒法院－職務法庭
TCH  臺灣高等法院 臺中分院   IPC  智慧財產及商業法院
TPD  臺灣臺北地方法院         SLD  臺灣士林地方法院
PCD  臺灣新北地方法院         TYD  臺灣桃園地方法院
TCD  臺灣臺中地方法院         TND  臺灣臺南地方法院
KSD  臺灣高雄地方法院         (...41 total)
```

### Result List Pagination

Result counts can be very large (test query returned **59,316** for `最高法院 + 刑事 + 毒品危害防制條例`). Pagination is via `&page=N` query (standard ASP.NET WebForms pager pattern). Default page size is ~20 items.

### JID Schema

Confirmed from observed list items:
`<court4chars>,<year>,<case_character>,<no>,<date YYYYMMDD>,<check>`

Examples:
- `TPSM,115,台抗,703,20260430,1` (最高法院 刑事 115年度台抗字第703號)
- `TPSM,115,台上,933,20260430,1` (最高法院 刑事 115年度台上字第933號)
- `TPHM,110,毒抗,1212,20210831,1` (臺灣高等法院 刑事 110年度毒抗字第1212號)

`<court4chars>` is the court code (4 characters by spec, can be 3+1 like `TPS`+`M`).
`<case_character>` is non-ASCII (Chinese), URL-encoded.

### Detail Page Schema

`<div class="int-table" id="jud">` contains 4 metadata rows:
- 裁判字號 (case ID display form — e.g. "最高法院 115 年度台抗字第 703 號刑事裁定")
- 裁判日期 (民國 YYY 年 MM 月 DD 日)
- 裁判案由 (case reason)
- (case body header)

`<div class="htmlcontent">` (within the same outer container) contains the full judgment text including 主文, 理由, etc.

PDF link, when present: `<a href="/FILES/<court>/<jid_url_encoded>.pdf">`

---

## Source 2: FJUDKM (fjudkm.judicial.gov.tw)

**Goal:** Discover hierarchical browse + full-text search.

**Transport:** standard_http (probe confirmed).

### Endpoints

| Method | Path | Purpose | Replay strategy |
|--------|------|---------|-----------------|
| GET | `/Default.aspx` | Topic tree home (462 topic categories) | HTML scrape `<a href="index_title.aspx?id={N}">` items |
| GET | `/index_title.aspx?id={topic_id}` | Topic detail with case list | HTML scrape `<a href="index_doc.aspx?par=<base64>">` items |
| GET | `/index_doc.aspx?par={base64_token}` | Single case commentary detail | HTML scrape body |
| GET | `/searcher.aspx` | Full-text search form (extracts hidden fields) | HTML scrape `__VIEWSTATE`/`__EVENTVALIDATION` |
| POST | `/searcher.aspx` | Submit full-text search | Form POST, parse result list |

### Browse Topic Sample (462 total)

```
474  法律行為(民法第71條~第98條)
643  法律行為(民法第99條至第118條)
433  無因管理         434  不當得利        435  侵權行為
582  債之效力        107  買賣瑕疵擔保    577  違約金
287  租賃            288  借貸            162  承攬─給付工程款
644  委任(含消費金融) 500  合夥          487  保證、人事保證
514  共有            713  用益物權        191  一般抵押權
190  最高限額抵押權  492  質權           491  動產擔保交易法
715  典權            490  留置權          159  裁判離婚事由
215  家事事件最佳利益 430  婚姻關係存續中及消滅後之給付
559  喪失繼承權      567  遺產之範圍      565  遺產分割
572  拋棄繼承        573  無人承認繼承
(... 432 more topics across 民事 / 刑事 / 行政 / 智財 / 家事 / 商事)
```

### Search Form Fields

| Field | Purpose |
|-------|---------|
| `__VIEWSTATE`, `__VIEWSTATEGENERATOR`, `__EVENTVALIDATION` | ASP.NET tokens |
| `lc1`, `lc1a`, `lc1b`, `lc1c` | Layer-1 classification dropdowns (左側分類) |
| `lc2`, `lc2a`, `lc2b`, `lc2c`, `lc3`, `lc3a`, `lc3b`, `lc3c` | Deeper classification |
| `txtcase` | Case-character substring |
| `txtno` | Case number |
| `ddlcourt` | Court dropdown |
| `chkbxSearch$0/1/2` | Checkboxes for search scope |
| `chkAll` | Select-all toggle |
| `hfSW` | Hidden state field |
| `btnSearch` | Submit (value=`查詢`) |

### Doc Detail (`index_doc.aspx?par=<base64>`)

The `par` token is opaque (base64-encoded server-side identifier).
Detail page contains the curated case commentary text.

---

## Reachability + Anti-Bot Assessment

- ✅ Both sites: 200 OK with browser User-Agent on first request
- ✅ ASP.NET session cookies set on GET, replayed on POST: works
- ✅ No Cloudflare/CAPTCHA/bot-detection HTML observed
- ⚠️ F5 BIG-IP load balancer cookies (`TS012a02ea`, `TSee6e730d027`) are present but not auth-gating — they're transparent to clients
- ⚠️ Service window is 24/7 for the website (unlike the official open-data API which is 0-6 AM only)
- ⚠️ Aggressive scraping risk: the Lawsnote precedent applies in spirit. Generated CLI must:
  - Default to per-request use, not bulk crawl
  - Provide a TOS warning on first run
  - Rate-limit to ~1 req/sec by default (configurable up to 5)
  - Document non-commercial intent in README

## Replayability — Verdict

**Both sources pass the replayability bar.** Every captured surface replays as plain HTTP
through stdlib. No clearance cookies, no live page-context execution, no resident browser
sidecar required. The printed CLI ships standard_http transport for both sources.

## Discovered Capabilities (feeds absorb manifest)

**FJUD:**
- Search by court (multi-select) + case type (multi-select) + year + case-character + number range + date range + reason + verdict text + body keyword
- Result list with pagination + total count
- Single judgment fetch (text + metadata)
- PDF attachment download
- 41 courts, 5 case types

**FJUDKM:**
- 462-topic hierarchical browse
- Per-topic case-commentary list
- Single case-commentary detail
- Full-text search with multi-field query
