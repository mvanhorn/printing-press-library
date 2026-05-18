# substack-creator-pp-cli — Vollständige Befehls-Dokumentation

Eine CLI für Substack, gebaut auf der reverse-engineerten internen API
(`*.substack.com/api/v1/...`). Alle Endpoints sind live gegen die echte API
verifiziert.

---

## Auth-Modell

Substack hat **keine API-Keys**. Die CLI nutzt dein Browser-Session-Cookie
`substack.sid` (das alte `connect.sid` ist seit 2024 abgelöst).

```bash
# Cookie aus einem eingeloggten Chrome importieren (braucht pycookiecheat o.ä.)
substack-creator-pp-cli auth login --chrome

# ODER manuell: Cookie aus DevTools kopieren und in die Config schreiben
#   ~/.config/substack-creator-pp-cli/config.toml:
#   access_token = "substack.sid=<value>"

substack-creator-pp-cli auth status     # Auth-Status prüfen
substack-creator-pp-cli doctor          # Health-Check: Auth + DB + Reachability
substack-creator-pp-cli auth logout     # Cookie löschen
```

**Cookie-Lebensdauer:** Rolling Session — 90 Tage ab dem *letzten* Request.
Jeder CLI-Aufruf verlängert die Session automatisch um 90 Tage. Solange du die
CLI (oder den Browser) regelmäßig nutzt, läuft das Cookie nie ab.

---

## Das `--subdomain` Flag (WICHTIG)

Substacks **Creator-Endpoints** leben auf der Publikations-Subdomain, nicht auf
`substack.com`. Wenn du eigene Posts, Drafts, Notes oder Sections einer
Publikation ansteuerst, musst du `--subdomain <name>` angeben:

```bash
substack-creator-pp-cli posts list --subdomain mypub
substack-creator-pp-cli drafts create --subdomain mypub --title "..."
```

Globale/öffentliche Endpoints (`categories`, `publications search`,
`profiles me`, `me subscriptions`, `feed`) brauchen kein `--subdomain`.

---

## Globale Flags (auf jedem Befehl)

| Flag | Wirkung |
|------|---------|
| `--json` | JSON-Ausgabe |
| `--csv` / `--plain` / `--quiet` | alternative Ausgabeformate |
| `--compact` | nur Schlüsselfelder (minimaler Token-Verbrauch) |
| `--select a,b,c.d` | nur genannte Felder (dotted paths für nested) |
| `--dry-run` | Request anzeigen, nicht senden |
| `--subdomain <name>` | Publikations-Subdomain für Creator-Endpoints |
| `--agent` | alle Agent-Defaults setzen (`--json --compact --no-input --no-color --yes`) |
| `--rate-limit <n>` | max. Requests/Sekunde |
| `--no-cache` | Response-Cache umgehen |
| `--deliver file:<path>` / `webhook:<url>` | Ausgabe an Sink routen |

---

## POSTS

Substack hat **keine separaten APIs** für Podcasts, Videos oder Chat-Threads —
alles sind `posts`, unterschieden durch das `type`-Feld
(`newsletter` | `podcast` | `video` | `thread`).

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `posts list --subdomain <p>` | Veröffentlichte Posts auflisten. `--type` filtert nach newsletter/podcast/video/thread | `GET /posts` |
| `posts get <slug> --subdomain <p>` | Einzelnen Post per Slug holen (inkl. body_html) | `GET /posts/{slug}` |
| `posts stats <id> --subdomain <p>` | Engagement-Stats (Likes/Comments/Restacks) | `GET /post/{id}/stats` |
| `posts react <id>` | Post mit Herz reagieren | `POST /reaction` |
| `posts restack <id>` | Post in deine Notes restacken | `POST /restack` |

**Transzendenz-Befehle** (lokal, brauchen `sync`):

| Befehl | Was es macht |
|--------|--------------|
| `posts best --by views\|likes\|comments\|restacks [--cross-pub]` | Top-Posts nach Engagement, optional über alle deine Publikationen |
| `posts twin <slug> --to <pub>` | Veröffentlichten Post als Draft in eine andere deiner Publikationen spiegeln |
| `posts pair <en> <de>` | EN↔DE Übersetzungs-Paar registrieren |
| `posts pairs [--missing]` | Registrierte Paare anzeigen / Posts ohne Twin finden |

```bash
substack-creator-pp-cli posts list --subdomain mypub --type podcast --json
substack-creator-pp-cli posts get my-post-slug --subdomain mypub --json
substack-creator-pp-cli posts best --by restacks --window 30d --cross-pub --json
```

---

## DRAFTS

Drafts sind die Schreib-Werkstatt. `drafts create` deckt **alle** Substack-
Felder ab und konvertiert Markdown automatisch in Substacks ProseMirror-JSON.

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `drafts create --subdomain <p>` | Neuen Draft anlegen (alle Felder, s.u.) | `POST /drafts` |
| `drafts list --subdomain <p>` | Drafts auflisten | `GET /drafts` |
| `drafts get <id> --subdomain <p>` | Draft per ID holen | `GET /drafts/{id}` |
| `drafts update <id> --subdomain <p>` | Draft ändern (nur geänderte Felder werden gesendet) | `PUT /drafts/{id}` |
| `drafts publish <id> --subdomain <p>` | Draft **sofort** veröffentlichen. `--send-email` (default true) | `POST /drafts/{id}/publish` |
| `drafts schedule <id> --subdomain <p> --at <datetime>` | Draft für **zukünftige** Veröffentlichung planen | `POST /drafts/{id}/scheduled_release` |
| `drafts preview <id> --subdomain <p>` | Autor-Vorschaulink generieren | `GET /drafts/{id}/preview` |
| `drafts delete <id> --subdomain <p>` | Draft löschen | `DELETE /drafts/{id}` |

### `drafts create` / `drafts update` — alle Felder

**Content:**
- `--title` / `--subtitle`
- `--body <markdown>` — Inline-Markdown, auto-konvertiert zu ProseMirror
- `--body-file <path>` — Body aus Datei (Markdown oder fertiges ProseMirror-JSON)
- `--body-json <json>` — rohes ProseMirror-JSON (volle Kontrolle)

**Post-Typ & Authoring:**
- `--type newsletter|podcast|video|thread` — Post-Typ
- `--byline <user-id>` (wiederholbar) — Autoren; default = eingeloggter User (auto-resolved)
- `--bylines '<json>'` — Bylines als JSON-Array
- `--section-id <id>` — Section zuordnen

**Audience & Paywall:**
- `--audience everyone|only_paid|founding`
- `--meter-type <type>`
- `--send-free-preview`
- `--exempt-paywall` / `--free-unlock`

**SEO & Social:**
- `--description` — Kurzbeschreibung (Archiv)
- `--cover-image <url>` / `--cover-square` / `--cover-explicit`
- `--seo-title` / `--seo-description` / `--social-title`

**Diskussion:**
- `--comment-sort best_first|most_recent|most_liked`
- `--comment-perms everyone|only_paid|none`
- `--show-guest-bios`

**Sichtbarkeit:**
- `--hide-from-feed` — aus dem Haupt-Feed verbergen
- `--hidden` — nur für Autor sichtbar

**Podcast / Video:**
- `--podcast-url` / `--podcast-duration` / `--podcast-episode-number` / `--podcast-season-number` / `--podcast-episode-type`
- `--free-podcast-url` / `--free-podcast-duration`
- `--video-url` / `--voiceover-url`

### Markdown → ProseMirror: unterstützte Body-Elemente

Alle gegen den echten Substack-Editor verifiziert:

| Markdown | Substack-Node |
|----------|---------------|
| `**fett**` | `strong` mark |
| `*kursiv*` | `em` mark |
| `` `code` `` | `code` mark |
| `[text](url)` | `link` mark |
| `# / ## / ### / ####` | `heading` (level 1–4) |
| `- item` / `* item` | `bullet_list` |
| `1. item` | `ordered_list` |
| `> zitat` | `blockquote` |
| ` ```lang ... ``` ` | `highlighted_code_block` (mit Sprache) |
| `$formel$` | `inline_latex` (inline math) |
| `$$formel$$` | `latex_block` (display math) |
| `---` | `horizontal_rule` |
| `[paywall]` | `paywall` |

Für Bilder, Embeds (Twitter/YouTube/Vimeo/Spotify), Buttons und Pullquotes
nutze den Substack-Editor oder `--body-json` mit rohem ProseMirror.

### `drafts schedule` — Felder

- `--at <datetime>` — RFC3339 (`2026-06-01T09:00:00Z`), `"2026-06-01 09:00"` oder `2026-06-01` (default 09:00 lokal)
- `--post-audience everyone|only_paid|only_founding`
- `--email-audience only_free|everyone|none`
- `--no-email` — web-only Release ohne E-Mail

```bash
# Sofort veröffentlichen mit E-Mail
substack-creator-pp-cli drafts publish 12345 --subdomain mypub

# Für die Zukunft planen
substack-creator-pp-cli drafts schedule 12345 --subdomain mypub --at 2026-06-01T09:00:00Z

# Podcast-Draft anlegen
substack-creator-pp-cli drafts create --subdomain mypub --type podcast \
  --title "Episode 5" --podcast-url https://cdn.../ep5.mp3 --podcast-duration 2400

# Vollständiger Artikel mit allen Body-Features
substack-creator-pp-cli drafts create --subdomain mypub --json \
  --title "Sharpe Ratio erklärt" --audience only_paid \
  --body-file ./artikel.md --seo-title "..." --comment-perms only_paid
```

---

## NOTES (Substacks Microblog)

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `notes list --subdomain <p>` | Deine letzten Notes | `GET /notes` |
| `notes publish <text>` | Neue Note veröffentlichen | `POST /comment/feed` |
| `notes reply <note-id> <text>` | Auf eine Note antworten | `POST /comment/{id}/reply` |
| `notes react <note-id>` | Note mit Herz reagieren | `POST /reaction/comment` |
| `notes restack <note-id>` | Note restacken | `POST /comment/{id}/restack` |

---

## COMMENTS

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `comments list <post-id>` | Kommentare zu einem Post | `GET /post/{id}/comments` |
| `comments add <post-id> <text>` | Kommentar zu einem Post hinzufügen | `POST /comment` |
| `comments react <comment-id>` | Kommentar mit Herz reagieren | `POST /reaction/comment` |

---

## SUBSCRIBERS

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `subscribers list <pub-id>` | Subscriber einer Publikation auflisten. `--tier free\|paid\|founding\|all` | `GET /publication/{id}/subscribers` |
| `subscribers count <pub-id>` | Subscriber-Zahlen (free + paid) | `GET /publication/{id}/subscriber_count` |
| `subscribers add <email>` | Subscriber per E-Mail hinzufügen | `POST /subscriber/add` |
| `subscribers export-free` | Free-Subscriber als CSV exportieren | `GET /free_subscribers/export` |
| `subscribers export-paid` | Paid-Subscriber als CSV exportieren | `GET /paid_subscribers/export` |

**Transzendenz-Befehle** (lokal, brauchen `sync`):

| Befehl | Was es macht |
|--------|--------------|
| `subscribers churn [--since 7d]` | Diff zweier Snapshots: wer kam/ging, free→paid Upgrades, paid→free Downgrades. Erst `subscribers churn --snapshot` für eine Baseline. |
| `subscribers cross-sell` | Emails, die auf einer Publikation zahlen, aber auf den anderen free/abwesend sind |

---

## PUBLICATIONS / PROFILES / CATEGORIES (öffentlich, kein --subdomain)

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `publications search --query <q>` | Publikationen global durchsuchen | `GET /publication/search` |
| `publications recommendations <pub-id>` | Von einer Publikation empfohlene Publikationen | `GET /publication/{id}/recommendations` |
| `profiles me` | Dein eigenes Profil + Publikationen | `GET /subscriptions` |
| `profiles get <handle>` | Profil eines anderen Users (kann je nach Handle 404 geben — Substack nutzt HTML-Redirect) | `GET /profile/{handle}` |
| `categories list` | Alle globalen Kategorien | `GET /categories` |
| `categories newsletters <category-id>` | Newsletter einer Kategorie | `GET /category/{id}/newsletters` |

---

## ME (eigene Account-Daten)

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `me subscriptions` | Was du abonniert hast | `GET /subscriptions/page_v2` |
| `me follows` | Profile, denen du folgst | `GET /me/follows` |
| `me recommendations` | Persönliche Empfehlungen | `GET /me/recommendations` |

---

## SECTIONS / IMAGES / FEED / DASHBOARD

| Befehl | Was es macht | API |
|--------|--------------|-----|
| `sections list <pub> --subdomain <p>` | Sections (Kategorien) einer Publikation | `GET /publication/sections` |
| `images upload <file>` | Bild auf Substacks CDN hochladen | `POST /image` |
| `feed list [--tab for-you\|following\|categories]` | Dein Reader-Feed | `GET /reader/feed` |
| `dashboard stats <pub-id> --subdomain <p>` | Aggregierte Dashboard-Statistiken | `GET /publication/{id}/stats` |

---

## TRANSZENDENZ-BEFEHLE (Multi-Publikations-Tools)

Diese Befehle gibt es in keinem anderen Substack-Tool. Sie arbeiten auf der
lokalen SQLite-Datenbank — erst `sync` ausführen.

| Befehl | Was es macht |
|--------|--------------|
| `portfolio` | Eine ASCII-Tabelle: alle deine Publikationen mit Subscriber-, Paid-, Posts-, Drafts-Zahlen |
| `grep "<query>" [--scope posts\|notes\|comments\|all]` | FTS5-Volltextsuche über deinen gesamten gecachten Inhalt |
| `schedule board [--days 30]` | ASCII-Kalender aller geplanten Posts über alle deine Publikationen |

---

## FRAMEWORK-BEFEHLE

| Befehl | Was es macht |
|--------|--------------|
| `sync [--resources a,b]` | API-Daten in lokale SQLite-DB synchronisieren (für Offline-Suche + Transzendenz-Befehle) |
| `search "<query>"` | Generische FTS über synchronisierte Daten oder Live-API |
| `doctor` | Health-Check: Auth, DB, Reachability |
| `export <table> --format jsonl` | Daten für Backup/Migration exportieren |
| `import <file>` | Daten aus JSONL via API-Calls importieren |
| `auth login\|status\|logout` | Authentifizierung verwalten |
| `agent-context` | Strukturiertes JSON, das diese CLI für Agenten beschreibt |
| `which <capability>` | Befehl finden, der eine Fähigkeit implementiert |
| `version` | Version anzeigen |

---

## TYPISCHE WORKFLOWS

```bash
# 1. Einrichten
substack-creator-pp-cli auth login --chrome
substack-creator-pp-cli doctor

# 2. Posts einer Publikation ansehen
substack-creator-pp-cli posts list --subdomain mypub --json
substack-creator-pp-cli posts list --subdomain mypub --type podcast --json

# 3. Draft schreiben und planen
substack-creator-pp-cli drafts create --subdomain mypub --json \
  --title "Mein Artikel" --body-file ./artikel.md --audience only_paid
substack-creator-pp-cli drafts schedule <draft-id> --subdomain mypub --at 2026-06-01T09:00:00Z

# 4. Multi-Publikations-Analyse (erst syncen)
substack-creator-pp-cli sync --resources publications,subscribers
substack-creator-pp-cli portfolio --json
substack-creator-pp-cli subscribers cross-sell --json

# 5. Archiv durchsuchen
substack-creator-pp-cli sync
substack-creator-pp-cli grep "yield curve" --scope all --json
```

---

## NICHT UNTERSTÜTZT

- **Live Video** — Substacks Live-Streaming-API erfordert WebRTC/Session-Handshakes,
  die nicht über replaybare HTTP-Calls abbildbar sind. Bewusst weggelassen.
- **Inline-Bilder, typisierte Embeds (Twitter/YouTube/etc.), Buttons, Pullquotes** im
  Body — Schema ist komplexer; nutze den Substack-Editor oder `--body-json`.
- `profiles get <handle>` — kann 404 geben, da Substack für Handle-Lookups
  HTML-Redirects statt eines sauberen API-Endpoints nutzt.
