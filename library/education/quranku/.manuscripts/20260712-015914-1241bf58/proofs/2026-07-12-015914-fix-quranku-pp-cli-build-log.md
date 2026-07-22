Manifest transcendence rows: 7 planned, 0 built. Phase 3 will not pass until all 7 ship.

## Build log
- Generation PASSED all quality gates (go mod tidy, govulncheck, go vet, go build, doctor).
- 11 absorbed commands verified live: surah list/get, chapters list/info, juz, verses, websearch, tafsirs, uthmani, recitation, audio.

## Phase 3 complete
Manifest transcendence rows: 7 planned, 7 built.
- find    — offline FTS over Arabic+Tafsiriyah corpus (114 surahs, ~6236 verses). VERIFIED: "kasih sayang" -> 14 results.
- verse   — cross-source single-verse lookup. VERIFIED: 2:255 (Ayat Kursi).
- daily   — date-seeded deterministic verse. VERIFIED.
- random  — random verse. VERIFIED.
- plan    — khatam plan (start/today/advance/status), 6236 total verses. VERIFIED end-to-end.
- hifz    — memorization tracker (mark/unmark/list). VERIFIED.
- bookmark— local bookmarks (add/list/rm). VERIFIED.
Foundation: internal/cli/quranku_corpus.go (shared corpus loader + helpers) with unit tests.
