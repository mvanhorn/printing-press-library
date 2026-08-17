# QuranKu (Tarjamah Tafsiriyah) CLI Brief

## API Identity
- Domain: Qur'an reading/reference. Combo CLI over the QuranKu web app's backends.
- Users: Indonesian Muslims reading/studying the Qur'an with Ustadz Muhammad Thalib's Tarjamah Tafsiriyah; agents/scripts needing Qur'an text+translation.
- Data profile: 114 surahs, ~6236 verses, 30 juz. Arabic text + Indonesian Tafsiriyah translation + rich Quran.com metadata + audio.

## Reachability Risk
- None. All content sources return HTTP 200 via plain HTTP, no bot protection, no auth.
- Discovery: recovered from the SPA JS bundle + direct HTTP probing (the replayable surface the printed CLI uses).

## Source Priority (combo)
- Primary: tafsiriyah (media-quran-api.lpoalr.easypanel.host/api) — no-spec, direct-HTTP. Auth: free.
- Secondary: quran-com-v4 (api.quran.com/api/v4) — documented public API. Auth: free.
- Tertiary: quranapi (quranapi.pages.dev/api) — static audio manifests. Auth: free.
- Economics: all free, no key. Supabase (user state) intentionally excluded.
- Inversion risk: Quran.com has the biggest surface; do NOT let that demote the Tafsiriyah primary. Tafsiriyah IS the site's identity.

## Confirmed Endpoints
### Tafsiriyah (primary)
- GET /surahs -> {status,count,data:[{id,name,nameArabic,nameTransliteration,numberOfVerses,revelationType}]}
- GET /surahs/{id} -> surah + verses[{verseNumber,textArabic,translations.terjemahTafsiriyah}]
- (/search is 404 on the deployed server -> offline search becomes a transcendence feature)
### Quran.com v4 (secondary)
- GET /chapters?language=id ; GET /chapters/{id}/info?language=id ; GET /juzs
- GET /verses/by_chapter/{id} ; GET /search?q= (LIVE full-text) ; GET /resources/tafsirs
- GET /quran/verses/uthmani?chapter_number= ; GET /chapter_recitations/{reciter}/{chapter}
### quranapi (tertiary)
- GET /audio/{surah}.json -> reciter->url map

## Data Layer
- Primary entities: surah, verse (with arabic + tafsiriyah), juz, reciter, tafsir.
- Sync cursor: none needed (static corpus); sync = pull all 114 surahs into SQLite once.
- FTS/search: full-text over verse arabic + tafsiriyah translation (server search is dead -> real value).

## Top Workflows
1. Read a surah with the Tafsiriyah translation (surah get 1).
2. Search the Qur'an by meaning/word (offline over Tafsiriyah + live via Quran.com).
3. Look up a specific verse (2:255) with translation + arabic.
4. Follow a reading/khatam plan to finish the Qur'an in N days.
5. Listen: get audio URLs for a surah/reciter.

## Product Thesis
- Name: QuranKu CLI (quranku-pp-cli)
- Why it should exist: the only CLI that carries the Indonesian Tarjamah Tafsiriyah (Ustadz M. Thalib) offline, cross-referenced with Quran.com's metadata, with agent-native JSON, offline FTS search, a khatam reading-plan tracker, and a hifz (memorization) tracker — none of which any existing Qur'an tool offers together.

## Build Priorities
1. Data layer: surahs + verses (arabic + tafsiriyah) in SQLite; sync-all.
2. Absorbed read surface across all 3 sources (surah/chapters/juz/verses/search/tafsirs/audio).
3. Transcendence: offline FTS search, cross-source verse compare, daily verse, khatam plan, hifz tracker, bookmarks, random ayah.
