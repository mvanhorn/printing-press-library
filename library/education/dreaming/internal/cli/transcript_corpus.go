// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// transcript export / import: share the transcript corpus as a portable,
// gzipped-JSON file (or fetch one from a URL). Only the impersonal `videos`
// catalog and `transcript_cues` are included — never user, playlist, or
// external-time data. Lets a complete corpus be built once and reused, instead
// of every user re-fetching all transcripts from Dreaming.
//
// Merge rule on import is "more complete wins": a video's transcript is taken
// from the incoming corpus only when the local store has no cues for it or has
// fewer cues than the incoming version. This prevents a partial corpus from
// clobbering a more complete local one while still letting users top each
// other up toward a complete shared corpus.

package cli

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"

	"github.com/spf13/cobra"
)

const corpusFormat = "dreaming-corpus"
const corpusVersion = 1

// corpusCue is the compact on-disk cue shape (short keys to keep files small).
type corpusCue struct {
	I int    `json:"i"`
	S int64  `json:"s"`
	E int64  `json:"e"`
	T string `json:"t"`
}

// corpusEntry is one video's transcript plus a content hash for the merge rule.
type corpusEntry struct {
	VideoID string      `json:"video_id"`
	Hash    string      `json:"hash"`
	Cues    []corpusCue `json:"cues"`
}

// corpusFile is the portable export envelope.
type corpusFile struct {
	Format      string            `json:"format"`
	Version     int               `json:"version"`
	ExportedAt  string            `json:"exported_at"`
	VideoCount  int               `json:"video_count"`
	CueCount    int               `json:"cue_count"`
	Videos      []json.RawMessage `json:"videos"`
	Transcripts []corpusEntry     `json:"transcripts"`
}

// cueContentHash hashes the ordered cue text so two corpora can be compared for
// equality without diffing every field.
func cueContentHash(cues []corpusCue) string {
	h := sha256.New()
	for _, c := range cues {
		io.WriteString(h, c.T)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func newTranscriptExportCmd(flags *rootFlags) *cobra.Command {
	var dbOverride string
	cmd := &cobra.Command{
		Use:   "export <file>",
		Short: "Export the transcript corpus (videos + cues only, no personal data) to a shareable gzipped-JSON file",
		Long: "Write the impersonal transcript corpus — the videos catalog and every\n" +
			"cached transcript cue — to a portable gzipped-JSON file. Your user, playlist,\n" +
			"and external-time data are never included. Share the file (or host it at a\n" +
			"URL) so others can 'transcript import' it instead of re-fetching every\n" +
			"transcript from Dreaming. Writing to '-' streams to stdout.",
		Example: strings.Trim(`
  dreaming-pp-cli transcript export corpus.json.gz
  dreaming-pp-cli transcript export - > corpus.json.gz
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStoreAt(cmd.Context(), dbOverride)
			if err != nil {
				return err
			}
			defer db.Close()

			videos, err := db.AllVideoData()
			if err != nil {
				return apiErr(err)
			}
			transcripts, err := db.AllTranscripts()
			if err != nil {
				return apiErr(err)
			}

			cf := corpusFile{
				Format:     corpusFormat,
				Version:    corpusVersion,
				ExportedAt: time.Now().UTC().Format(time.RFC3339),
				Videos:     videos,
			}
			for vid, cues := range transcripts {
				cc := make([]corpusCue, 0, len(cues))
				for _, c := range cues {
					cc = append(cc, corpusCue{I: c.Index, S: c.StartMS, E: c.EndMS, T: c.Text})
				}
				cf.CueCount += len(cc)
				cf.Transcripts = append(cf.Transcripts, corpusEntry{VideoID: vid, Hash: cueContentHash(cc), Cues: cc})
			}
			cf.VideoCount = len(videos)

			path := args[0]
			var w io.Writer
			var closeFn func() error
			if path == "-" {
				w = cmd.OutOrStdout()
				closeFn = func() error { return nil }
			} else {
				f, ferr := os.Create(path)
				if ferr != nil {
					return apiErr(fmt.Errorf("creating %s: %w", path, ferr))
				}
				w = f
				closeFn = f.Close
			}
			gz := gzip.NewWriter(w)
			enc := json.NewEncoder(gz)
			if err := enc.Encode(cf); err != nil {
				gz.Close()
				closeFn()
				return apiErr(fmt.Errorf("encoding corpus: %w", err))
			}
			if err := gz.Close(); err != nil {
				closeFn()
				return apiErr(err)
			}
			if err := closeFn(); err != nil {
				return apiErr(err)
			}

			if path != "-" {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet && !flags.plain && !flags.csv) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"file": path, "videos": cf.VideoCount, "transcripts": len(cf.Transcripts), "cues": cf.CueCount,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Exported %d videos and %d transcripts (%d cues) to %s\n",
					cf.VideoCount, len(cf.Transcripts), cf.CueCount, path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbOverride, "db", "", "Database path (default: ~/.local/share/dreaming-pp-cli/data.db)")
	return cmd
}

func newTranscriptImportCmd(flags *rootFlags) *cobra.Command {
	var dbOverride string
	cmd := &cobra.Command{
		Use:   "import <file-or-url>",
		Short: "Merge a shared transcript corpus (local file or https URL) into the local store",
		Long: "Read a corpus produced by 'transcript export' — from a local file or an\n" +
			"https:// URL — and merge it into the local store. Videos are upserted; a\n" +
			"video's transcript is taken from the corpus only when the local store is\n" +
			"missing it or has fewer cues (more-complete-wins), so partial corpora never\n" +
			"clobber a fuller local one. Only catalog + transcript data is read.",
		Example: strings.Trim(`
  dreaming-pp-cli transcript import corpus.json.gz
  dreaming-pp-cli transcript import https://example.com/dreaming-corpus.json.gz
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			src := args[0]
			rc, err := openCorpusSource(cmd.Context(), src)
			if err != nil {
				return err
			}
			defer rc.Close()

			gz, err := gzip.NewReader(rc)
			if err != nil {
				return usageErr(fmt.Errorf("reading %s: not a gzip corpus file: %w", src, err))
			}
			defer gz.Close()
			var cf corpusFile
			if err := json.NewDecoder(gz).Decode(&cf); err != nil {
				return usageErr(fmt.Errorf("decoding corpus: %w", err))
			}
			if cf.Format != corpusFormat {
				return usageErr(fmt.Errorf("not a %s file (got format %q)", corpusFormat, cf.Format))
			}
			if cf.Version > corpusVersion {
				return usageErr(fmt.Errorf("corpus version %d is newer than this CLI supports (%d); upgrade the CLI", cf.Version, corpusVersion))
			}

			db, err := openDreamingStoreAt(cmd.Context(), dbOverride)
			if err != nil {
				return err
			}
			defer db.Close()

			res := mergeCorpus(db, &cf)
			if res.err != nil {
				return res.err
			}
			vids, cues, err := db.TranscriptStats()
			if err != nil {
				return err
			}
			out := map[string]any{
				"videos_upserted":     res.videosUpserted,
				"videos_failed":       res.videosFailed,
				"transcripts_added":   res.added,
				"transcripts_updated": res.updated,
				"transcripts_skipped": res.skipped,
				"corpus_videos":       vids,
				"corpus_cues":         cues,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet && !flags.plain && !flags.csv) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Imported: %d videos (%d failed), %d new transcripts, %d updated, %d skipped (already as complete). Corpus now: %d videos, %d cues.\n",
				res.videosUpserted, res.videosFailed, res.added, res.updated, res.skipped, vids, cues)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbOverride, "db", "", "Database path (default: ~/.local/share/dreaming-pp-cli/data.db)")
	return cmd
}

// mergeResult tallies a corpus merge.
type mergeResult struct {
	videosUpserted int
	videosFailed   int
	added          int
	updated        int
	skipped        int
	err            error
}

// mergeCorpus upserts the corpus's videos and merges its transcripts into db
// using the "more complete wins" rule. Shared by `transcript import` and
// `transcript sync --from-url`.
func mergeCorpus(db *store.Store, cf *corpusFile) mergeResult {
	var r mergeResult
	for _, v := range cf.Videos {
		if err := db.UpsertVideos(v); err != nil {
			r.videosFailed++
			continue
		}
		r.videosUpserted++
	}
	for _, e := range cf.Transcripts {
		existing := db.TranscriptCueCount(e.VideoID)
		switch {
		case existing == 0:
			r.added++
		case len(e.Cues) > existing:
			r.updated++
		default:
			r.skipped++
			continue
		}
		stored := make([]store.TranscriptCue, 0, len(e.Cues))
		for _, c := range e.Cues {
			stored = append(stored, store.TranscriptCue{VideoID: e.VideoID, Index: c.I, StartMS: c.S, EndMS: c.E, Text: c.T})
		}
		if err := db.ReplaceTranscript(e.VideoID, stored); err != nil {
			r.err = apiErr(fmt.Errorf("importing transcript %s: %w", e.VideoID, err))
			return r
		}
	}
	return r
}

// importCorpusInto fetches/opens a corpus from src, validates it, and merges it
// into db. Returns the number of transcripts added+updated. Used by
// `transcript sync --from-url`.
func importCorpusInto(ctx context.Context, db *store.Store, src string) (int, error) {
	rc, err := openCorpusSource(ctx, src)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	gz, err := gzip.NewReader(rc)
	if err != nil {
		return 0, usageErr(fmt.Errorf("reading %s: not a gzip corpus file: %w", src, err))
	}
	defer gz.Close()
	var cf corpusFile
	if err := json.NewDecoder(gz).Decode(&cf); err != nil {
		return 0, usageErr(fmt.Errorf("decoding corpus: %w", err))
	}
	if cf.Format != corpusFormat {
		return 0, usageErr(fmt.Errorf("not a %s file (got format %q)", corpusFormat, cf.Format))
	}
	if cf.Version > corpusVersion {
		return 0, usageErr(fmt.Errorf("corpus version %d is newer than this CLI supports (%d); upgrade the CLI", cf.Version, corpusVersion))
	}
	res := mergeCorpus(db, &cf)
	if res.err != nil {
		return 0, res.err
	}
	return res.added + res.updated, nil
}

// openCorpusSource returns a reader for a local path or an https:// URL.
// Plain http is rejected: the fetched corpus is merged persistently into the
// local store, so an unencrypted channel would let an observer inject cues.
func openCorpusSource(ctx context.Context, src string) (io.ReadCloser, error) {
	if strings.HasPrefix(src, "http://") {
		return nil, usageErr(fmt.Errorf("refusing to fetch corpus over unencrypted http: %s — use https, or download the file and pass its path", src))
	}
	if strings.HasPrefix(src, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return nil, usageErr(err)
		}
		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			return nil, apiErr(fmt.Errorf("fetching corpus: %w", err))
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, apiErr(fmt.Errorf("fetching corpus: HTTP %d", resp.StatusCode))
		}
		return resp.Body, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, usageErr(fmt.Errorf("opening %s: %w", src, err))
	}
	return f, nil
}
