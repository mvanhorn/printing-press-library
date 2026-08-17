# QuranKu CLI Absorb Manifest

## Absorbed (match/beat every read the QuranKu app does)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List all surahs | tafsiriyah /surahs | quranku-pp-cli surah list | Offline after sync, --json/--select |
| 2 | Read surah + Tafsiriyah | tafsiriyah /surahs/{id} | quranku-pp-cli surah get | Arabic + Indonesian Tafsiriyah, offline |
| 3 | Chapter list (Quran.com) | quran.com /chapters | (generated endpoint) chapters list | i18n metadata, --json |
| 4 | Chapter background/info | quran.com /chapters/{id}/info | (generated endpoint) chapters info | Indonesian intro text |
| 5 | Juz structure | quran.com /juzs | (generated endpoint) juz list | verse_mapping per juz |
| 6 | Verses by chapter (Arabic) | quran.com /verses/by_chapter/{id} | (generated endpoint) verses by-chapter | Uthmani text, pagination |
| 7 | Live full-text search | quran.com /search | (generated endpoint) search | Works across whole Qur'an (Tafsiriyah server search is dead) |
| 8 | Available tafsirs | quran.com /resources/tafsirs | (generated endpoint) tafsirs list | discover other tafsirs |
| 9 | Uthmani script text | quran.com /quran/verses/uthmani | (generated endpoint) uthmani | clean Arabic-only |
| 10 | Chapter recitation audio | quran.com /chapter_recitations/{r}/{c} | (generated endpoint) recitation | audio_url per reciter |
| 11 | Per-surah audio manifest | quranapi /audio/{surah}.json | (generated endpoint) audio surah | reciter->url map |

## Transcendence (only possible with local store + combo)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Offline full-text search | find | hand-code | SQLite FTS over synced Arabic + Tafsiriyah; server /search is 404 | Use to search the Qur'an offline by word/meaning across Arabic and the Indonesian Tafsiriyah. |
| 2 | Cross-source verse compare | verse | hand-code | Joins Tafsiriyah translation + Quran.com Arabic/metadata in one lookup | Use to get one verse (e.g. 2:255) with Arabic + Tafsiriyah together. Do NOT use for whole-surah reads; use 'surah get'. |
| 3 | Daily verse | daily | hand-code | Date-seeded deterministic pick from local store, no network | Use for a stable verse-of-the-day with Tafsiriyah. |
| 4 | Khatam reading plan | plan | hand-code | Computes daily juz/surah ranges to finish in N days + tracks progress in SQLite | Use to build and follow a plan to finish the Qur'an in N days. Do NOT use for one-off reads. |
| 5 | Hifz (memorization) tracker | hifz | hand-code | Local per-surah/verse memorization state no API provides | Use to mark and review memorization progress. |
| 6 | Bookmarks + notes | bookmark | hand-code | Local verse bookmarks with notes, offline | Use to save verses with personal notes. |
| 7 | Random ayah | random | hand-code | Local random pick with Tafsiriyah, offline | Use for a random verse for reflection. |

## Stubs
- None. All rows are shipping scope.

## Notes
- Combo primary base_url = Tafsiriyah; Quran.com + quranapi via per-resource base_url overrides.
- Server-side Tafsiriyah search is dead -> offline `find` is a genuine value-add, not a mirror.
