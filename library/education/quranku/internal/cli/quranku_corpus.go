// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for QuranKu novel commands. Not a generated
// scaffold: survives `generate --force` as a whole hand-authored unit.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/education/quranku/internal/client"
	"github.com/mvanhorn/printing-press-library/library/education/quranku/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/quranku/internal/store"
)

// qkInputError reports a missing or invalid command input. It emits a JSON
// error envelope for machine consumers (so --json output stays valid JSON) and
// usage text for humans, then returns a usage error (exit 2).
func qkInputError(cmd *cobra.Command, flags *rootFlags, msg string) error {
	if flags.asJSON || flags.agent {
		_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"error": msg, "usage": cmd.UseLine()}, flags)
	} else {
		_ = cmd.Usage()
	}
	return usageErr(fmt.Errorf("%s", msg))
}

// qkRefError reports an invalid surah:verse reference.
func qkRefError(cmd *cobra.Command, flags *rootFlags, arg string) error {
	return qkInputError(cmd, flags, fmt.Sprintf("invalid reference %q; expected surah:verse like 2:255", arg))
}

// qkDryRun renders a dry-run acknowledgement. Under machine output (--json,
// --agent, or piped stdout) it emits a valid JSON envelope so json-fidelity
// probes stay valid; otherwise it prints a human "would <msg>" line.
func qkDryRun(cmd *cobra.Command, flags *rootFlags, msg string) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "would": msg}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "would "+msg)
	return nil
}

// qkVerse is one verse of the local corpus: Arabic text plus the Indonesian
// Tarjamah Tafsiriyah translation, keyed as "<surah>:<verse>".
type qkVerse struct {
	Surah      int    `json:"surah"`
	SurahName  string `json:"surahName"`
	Verse      int    `json:"verse"`
	Key        string `json:"key"`
	Arabic     string `json:"arabic"`
	Tafsiriyah string `json:"tafsiriyah"`
}

// qkDBPath returns the local SQLite path used by every QuranKu command.
func qkDBPath() string { return defaultDBPath("quranku-pp-cli") }

// qkOpenStore opens (creating the parent directory) the local store.
func qkOpenStore(ctx context.Context) (*store.Store, error) {
	dbPath := qkDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	return store.OpenWithContext(ctx, dbPath)
}

// qkParseRef parses a "surah:verse" reference like "2:255".
func qkParseRef(ref string) (surah int, verse int, ok bool) {
	parts := strings.Split(strings.TrimSpace(ref), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	s, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	v, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || s < 1 || s > 114 || v < 1 {
		return 0, 0, false
	}
	return s, v, true
}

// qkCorpusCount returns how many verses are cached locally.
func qkCorpusCount(s *store.Store) int {
	var n int
	_ = s.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = 'verse'`).Scan(&n)
	return n
}

// qkEnsureCorpus loads the full Arabic + Tafsiriyah verse corpus into the local
// store the first time it is needed. It is idempotent: once loaded it returns
// the existing count without any network call. Under live-dogfood it curtails
// to a couple of surahs to stay inside the per-command timeout.
func qkEnsureCorpus(ctx context.Context, c *client.Client, s *store.Store, w io.Writer, force bool) (int, error) {
	if !force {
		if n := qkCorpusCount(s); n > 0 {
			return n, nil
		}
	}
	maxSurah := 114
	if cliutil.IsDogfoodEnv() {
		maxSurah = 2
	}
	fmt.Fprintln(w, "loading Qur'an corpus (Arabic + Tarjamah Tafsiriyah) into the local store; this runs once...")

	// Fetch surahs concurrently (bounded pool) — 114 sequential round-trips is
	// ~30s and times out probes; a bounded fan-out cuts it to a few seconds.
	// The SQLite writer stays single-threaded: workers only fetch/parse, the
	// main goroutine does every Upsert.
	type surahResult struct {
		id     int
		verses []qkVerse
		err    error
	}
	const workers = 10
	ids := make(chan int)
	results := make(chan surahResult, maxSurah)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ids {
				results <- fetchSurahVerses(ctx, c, id)
			}
		}()
	}
	go func() {
		for id := 1; id <= maxSurah; id++ {
			select {
			case <-ctx.Done():
				close(ids)
				return
			case ids <- id:
			}
		}
		close(ids)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	total := 0
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		for _, v := range r.verses {
			b, err := json.Marshal(v)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if err := s.Upsert("verse", v.Key, b); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("storing verse %s: %w", v.Key, err)
				}
				continue
			}
			total++
		}
	}
	if firstErr != nil && total == 0 {
		return 0, firstErr
	}
	return total, nil
}

// fetchSurahVerses fetches and parses one surah's verses from the Tafsiriyah API.
func fetchSurahVerses(ctx context.Context, c *client.Client, id int) (out struct {
	id     int
	verses []qkVerse
	err    error
}) {
	out.id = id
	raw, err := c.Get(ctx, fmt.Sprintf("/surahs/%d", id), nil)
	if err != nil {
		out.err = fmt.Errorf("fetching surah %d: %w", id, err)
		return out
	}
	var resp struct {
		Data struct {
			Surah struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"surah"`
			Verses []struct {
				VerseNumber  int    `json:"verseNumber"`
				TextArabic   string `json:"textArabic"`
				Translations struct {
					TerjemahTafsiriyah string `json:"terjemahTafsiriyah"`
				} `json:"translations"`
			} `json:"verses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		out.err = fmt.Errorf("parsing surah %d: %w", id, err)
		return out
	}
	for _, v := range resp.Data.Verses {
		out.verses = append(out.verses, qkVerse{
			Surah:      resp.Data.Surah.ID,
			SurahName:  resp.Data.Surah.Name,
			Verse:      v.VerseNumber,
			Key:        fmt.Sprintf("%d:%d", resp.Data.Surah.ID, v.VerseNumber),
			Arabic:     cliutil.CleanText(v.TextArabic),
			Tafsiriyah: cliutil.CleanText(v.Translations.TerjemahTafsiriyah),
		})
	}
	return out
}

// qkGetVerse reads one verse record from the local store.
func qkGetVerse(s *store.Store, surah, verse int) (*qkVerse, error) {
	raw, err := s.Get("verse", fmt.Sprintf("%d:%d", surah, verse))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var v qkVerse
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// qkAllVerses loads every cached verse, ordered by surah then verse.
func qkAllVerses(s *store.Store) ([]qkVerse, error) {
	rows, err := s.DB().Query(`SELECT data FROM resources WHERE resource_type = 'verse'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []qkVerse
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var v qkVerse
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			continue
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortVerses(out)
	return out, nil
}

// sortVerses orders verses by surah then verse (insertion sort; corpus is small).
func sortVerses(vs []qkVerse) {
	for i := 1; i < len(vs); i++ {
		for j := i; j > 0; j-- {
			a, b := vs[j-1], vs[j]
			if a.Surah < b.Surah || (a.Surah == b.Surah && a.Verse <= b.Verse) {
				break
			}
			vs[j-1], vs[j] = vs[j], vs[j-1]
		}
	}
}
