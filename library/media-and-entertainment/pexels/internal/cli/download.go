// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source live

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/pexels"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/store"
)

type downloadedItem struct {
	ID           int64  `json:"id"`
	FilePath     string `json:"file_path"`
	URL          string `json:"url"`
	Photographer string `json:"photographer"`
}

type downloadResult struct {
	Downloaded      []downloadedItem `json:"downloaded"`
	DownloadedCount int              `json:"downloaded_count"`
	SkippedDupes    int              `json:"skipped_dupes"`
	Scanned         int              `json:"scanned"`
	PagesScanned    int              `json:"pages_scanned"`
	RateRemaining   int64            `json:"rate_remaining"`
	StoppedEarly    bool             `json:"stopped_early"`
	Note            string           `json:"note"`
}

// search envelope shapes
type photoSearchEnvelope struct {
	NextPage string `json:"next_page"`
	Photos   []struct {
		ID              int64             `json:"id"`
		URL             string            `json:"url"`
		Photographer    string            `json:"photographer"`
		PhotographerURL string            `json:"photographer_url"`
		AvgColor        string            `json:"avg_color"`
		Alt             string            `json:"alt"`
		Src             map[string]string `json:"src"`
	} `json:"photos"`
}

type videoSearchEnvelope struct {
	NextPage string `json:"next_page"`
	Videos   []struct {
		ID    int64  `json:"id"`
		URL   string `json:"url"`
		Image string `json:"image"`
		User  struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"user"`
		VideoFiles []struct {
			Quality  string `json:"quality"`
			FileType string `json:"file_type"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
			Link     string `json:"link"`
		} `json:"video_files"`
	} `json:"videos"`
}

func newNovelDownloadCmd(flags *rootFlags) *cobra.Command {
	var (
		flagType         string
		flagSize         string
		flagQuality      string
		flagOrientation  string
		flagLimit        int
		flagMaxPages     int
		flagOutput       string
		flagNameTemplate string
		flagSidecar      bool
		flagDB           string
		flagQuery        string
	)

	cmd := &cobra.Command{
		Use:         "download [query]",
		Short:       "Bulk-download Pexels media matching a query, deduping against a local ledger.",
		Example:     "mountain lake --type photo --limit 30 --max-pages 3 --size large --sidecar",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--query=nature;--limit=1;--max-pages=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search Pexels and download matching media into the output directory")
				return nil
			}
			// query comes from the positional args OR the --query flag (the flag
			// form lets agents and the verifier supply it without a positional).
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				query = strings.TrimSpace(flagQuery)
			}
			if query == "" {
				return usageErr(fmt.Errorf("query is required (positional <query> or --query)\nUsage: %s [query]", cmd.CommandPath()))
			}
			if flagType != "photo" && flagType != "video" {
				return usageErr(fmt.Errorf("--type must be photo or video, got %q", flagType))
			}

			maxPages := flagMaxPages
			limit := flagLimit
			if cliutil.IsDogfoodEnv() {
				if maxPages > 1 {
					maxPages = 1
				}
				if limit > 3 {
					limit = 3
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("pexels-pp-cli")
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()
			db := st.DB()
			if err := store.EnsurePexelsDownloads(db); err != nil {
				return fmt.Errorf("ensure ledger: %w", err)
			}

			cfg, _ := config.Load(flags.configPath)
			key := ""
			if cfg != nil {
				key = cfg.PexelsApiKey
			}
			client := pexels.New(key)

			noWrite := cliutil.IsVerifyEnv() || dryRunOK(flags)
			result := downloadResult{Downloaded: make([]downloadedItem, 0)}

			searchPath := "/search"
			if flagType == "video" {
				searchPath = "/videos/search"
			}

			perPage := 80
			nextPage := ""
		pageLoop:
			for page := 1; page <= maxPages; page++ {
				params := map[string]string{
					"query":    query,
					"per_page": strconv.Itoa(perPage),
					"page":     strconv.Itoa(page),
				}
				if flagOrientation != "" {
					params["orientation"] = flagOrientation
				}
				body, ri, err := client.Get(ctx, searchPath, params)
				if err != nil {
					var rle *cliutil.RateLimitError
					if errors.As(err, &rle) {
						result.StoppedEarly = true
						result.Note = "stopped: " + rle.Error()
						break
					}
					return classifyAPIError(err, flags)
				}
				result.PagesScanned = page
				if ri.Known {
					result.RateRemaining = ri.Remaining
				}

				processed, stop, perr := processPage(ctx, client, db, body, flagType, flagSize, flagQuality, flagNameTemplate, flagOutput, query, flagSidecar, noWrite, limit, &result, &nextPage)
				if perr != nil {
					return perr
				}
				_ = processed

				if ri.Known && ri.Remaining <= 2 {
					result.StoppedEarly = true
					if result.Note == "" {
						result.Note = "stopped before rate-limit exhaustion"
					}
					break
				}
				if stop || len(result.Downloaded) >= limit || nextPage == "" {
					break pageLoop
				}
			}

			result.DownloadedCount = len(result.Downloaded)
			if result.Note == "" {
				if noWrite {
					result.Note = "verify/dry-run mode: recorded planned downloads without writing files"
				} else {
					result.Note = fmt.Sprintf("downloaded %d, skipped %d duplicates", result.DownloadedCount, result.SkippedDupes)
				}
			}
			return emitDownload(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "photo", "media type: photo or video")
	cmd.Flags().StringVar(&flagSize, "size", "large", "photo size key (original|large2x|large|medium|small|tiny|portrait|landscape)")
	cmd.Flags().StringVar(&flagQuality, "quality", "hd", "video quality (hd|sd|uhd)")
	cmd.Flags().StringVar(&flagOrientation, "orientation", "", "filter by orientation (landscape|portrait|square)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "maximum number of files to download")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 3, "maximum search pages to scan")
	cmd.Flags().StringVar(&flagOutput, "output", "pexels-downloads", "output directory")
	cmd.Flags().StringVar(&flagNameTemplate, "name-template", "{id}", "filename template; tokens {id} {photographer} {type}")
	cmd.Flags().BoolVar(&flagSidecar, "sidecar", false, "write a <file>.meta.json attribution sidecar per download")
	cmd.Flags().StringVar(&flagDB, "db", "", "download ledger DB path (default: standard data dir)")
	cmd.Flags().StringVar(&flagQuery, "query", "", "search query (alternative to the positional argument)")
	return cmd
}

// processPage handles one search-result page. It updates result and nextPage.
// It returns stop=true when the per-result limit was reached.
func processPage(ctx context.Context, client *pexels.Client, db *sql.DB, body json.RawMessage, mediaType, size, quality, nameTpl, outDir, query string, sidecar, noWrite bool, limit int, result *downloadResult, nextPage *string) (processed int, stop bool, err error) {
	if mediaType == "video" {
		var env videoSearchEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return 0, false, fmt.Errorf("decode video search: %w", err)
		}
		*nextPage = env.NextPage
		for _, v := range env.Videos {
			// --limit bounds DOWNLOADS, not scans: dedup skips don't consume the
			// budget, so a populated ledger still yields up to `limit` new files
			// (scanning is bounded separately by --max-pages).
			if len(result.Downloaded) >= limit {
				return processed, true, nil
			}
			result.Scanned++
			exists, derr := store.PexelsDownloadExists(db, v.ID, "video")
			if derr != nil {
				return processed, false, derr
			}
			if exists {
				result.SkippedDupes++
				continue
			}
			files := make([]pexels.VideoFile, 0, len(v.VideoFiles))
			for _, f := range v.VideoFiles {
				files = append(files, pexels.VideoFile{Quality: f.Quality, FileType: f.FileType, Width: f.Width, Height: f.Height, Link: f.Link})
			}
			chosen, ok := pexels.PickVideoFileByQuality(files, quality)
			if !ok || chosen.Link == "" {
				continue
			}
			attribution := fmt.Sprintf("Video by %s on Pexels", v.User.Name)
			rec := store.PexelsDownload{
				MediaID: v.ID, MediaType: "video", Query: query,
				Photographer: v.User.Name, PhotographerURL: v.User.URL,
				PageURL: v.URL, SrcURL: chosen.Link, Alt: "",
				DownloadedAt: time.Now().UTC().Format(time.RFC3339),
			}
			fp, werr := finalizeDownload(ctx, client, db, rec, chosen.Link, nameTpl, outDir, "video", v.User.Name, sidecar, noWrite, attribution, map[string]string{"video": chosen.Link})
			if werr != nil {
				return processed, false, werr
			}
			result.Downloaded = append(result.Downloaded, downloadedItem{ID: v.ID, FilePath: fp, URL: chosen.Link, Photographer: v.User.Name})
			processed++
		}
		return processed, false, nil
	}

	var env photoSearchEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return 0, false, fmt.Errorf("decode photo search: %w", err)
	}
	*nextPage = env.NextPage
	for _, p := range env.Photos {
		// --limit bounds DOWNLOADS, not scans (see the video loop above).
		if len(result.Downloaded) >= limit {
			return processed, true, nil
		}
		result.Scanned++
		exists, derr := store.PexelsDownloadExists(db, p.ID, "photo")
		if derr != nil {
			return processed, false, derr
		}
		if exists {
			result.SkippedDupes++
			continue
		}
		url := p.Src[size]
		if url == "" {
			url = p.Src["large"]
		}
		if url == "" && p.Src["original"] != "" {
			url = p.Src["original"]
		}
		if url == "" {
			continue
		}
		attribution := fmt.Sprintf("Photo by %s on Pexels", p.Photographer)
		rec := store.PexelsDownload{
			MediaID: p.ID, MediaType: "photo", Query: query,
			Photographer: p.Photographer, PhotographerURL: p.PhotographerURL,
			PageURL: p.URL, SrcURL: url, AvgColor: p.AvgColor, Alt: p.Alt,
			DownloadedAt: time.Now().UTC().Format(time.RFC3339),
		}
		fp, werr := finalizeDownload(ctx, client, db, rec, url, nameTpl, outDir, "photo", p.Photographer, sidecar, noWrite, attribution, p.Src)
		if werr != nil {
			return processed, false, werr
		}
		result.Downloaded = append(result.Downloaded, downloadedItem{ID: p.ID, FilePath: fp, URL: url, Photographer: p.Photographer})
		processed++
	}
	return processed, false, nil
}

// finalizeDownload writes the media file (unless noWrite), an optional sidecar,
// and records the ledger row. It returns the planned/actual file path.
func finalizeDownload(ctx context.Context, client *pexels.Client, db *sql.DB, rec store.PexelsDownload, mediaURL, nameTpl, outDir, mediaType, photographer string, sidecar, noWrite bool, attribution string, urls map[string]string) (string, error) {
	ext := extFromURL(mediaURL)
	name := renderNameTemplate(nameTpl, rec.MediaID, photographer, mediaType) + ext
	// Collapse to a single path component so neither an API-derived photographer
	// name nor the operator-supplied --name-template can traverse out of outDir
	// (e.g. a "../" sequence or an absolute path). filepath.Base drops any
	// directory portion; the fallback guards a name that cleans to "." / "..".
	name = filepath.Base(filepath.Clean("/" + name))
	if name == "." || name == ".." || name == "/" || strings.TrimSpace(name) == "" {
		name = strconv.FormatInt(rec.MediaID, 10) + ext
	}
	fp := filepath.Join(outDir, name)
	rec.FilePath = fp

	if !noWrite {
		if err := os.MkdirAll(outDir, 0o750); err != nil {
			return "", fmt.Errorf("create output dir: %w", err)
		}
		if err := downloadFile(ctx, client, mediaURL, fp); err != nil {
			return "", err
		}
		if sidecar {
			if err := writeSidecar(fp, rec, attribution, urls); err != nil {
				return "", err
			}
		}
		// Record in the ledger only when we actually wrote the file. Under
		// verify/dry-run (noWrite) the ledger must not be mutated.
		if err := store.InsertPexelsDownload(db, rec); err != nil {
			return "", fmt.Errorf("record download: %w", err)
		}
	}
	return fp, nil
}

func downloadFile(ctx context.Context, client *pexels.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", client.UserAgent())
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("fetch media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fetch media %s: HTTP %d", url, resp.StatusCode)
	}
	// #nosec G304 -- dest is the user-chosen download target (--out dir joined
	// with a sanitized media filename); writing there is the command's purpose.
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func writeSidecar(filePath string, rec store.PexelsDownload, attribution string, urls map[string]string) error {
	meta := map[string]any{
		"id":               rec.MediaID,
		"type":             rec.MediaType,
		"urls":             urls,
		"photographer":     rec.Photographer,
		"photographer_url": rec.PhotographerURL,
		"page_url":         rec.PageURL,
		"avg_color":        rec.AvgColor,
		"alt":              rec.Alt,
		"attribution":      attribution,
		"attribution_html": attributionHTML(rec, attribution),
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath+".meta.json", data, 0o600)
}

func attributionHTML(rec store.PexelsDownload, attribution string) string {
	kind := "Photo"
	if rec.MediaType == "video" {
		kind = "Video"
	}
	pageURL := html.EscapeString(rec.PageURL)
	photographer := html.EscapeString(rec.Photographer)
	name := photographer
	if rec.PhotographerURL != "" {
		name = fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(rec.PhotographerURL), photographer)
	}
	return fmt.Sprintf(`<a href="%s">%s by %s on Pexels</a>`, pageURL, html.EscapeString(kind), name)
}

func renderNameTemplate(tpl string, id int64, photographer, mediaType string) string {
	safe := func(s string) string {
		s = strings.ReplaceAll(s, "/", "-")
		s = strings.ReplaceAll(s, string(filepath.Separator), "-")
		return strings.TrimSpace(s)
	}
	out := tpl
	out = strings.ReplaceAll(out, "{id}", strconv.FormatInt(id, 10))
	out = strings.ReplaceAll(out, "{photographer}", safe(photographer))
	out = strings.ReplaceAll(out, "{type}", mediaType)
	if strings.TrimSpace(out) == "" {
		out = strconv.FormatInt(id, 10)
	}
	return out
}

func extFromURL(u string) string {
	clean := u
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	ext := path.Ext(clean)
	if ext == "" {
		return ".bin"
	}
	return ext
}

func emitDownload(cmd *cobra.Command, flags *rootFlags, result downloadResult) error {
	stdout := cmd.OutOrStdout()
	if flags.asJSON || flags.agent || !isTerminal(stdout) {
		return printJSONFiltered(stdout, result, flags)
	}
	fmt.Fprintf(stdout, "%s\n  downloaded=%d skipped=%d scanned=%d pages=%d\n",
		result.Note, result.DownloadedCount, result.SkippedDupes, result.Scanned, result.PagesScanned)
	return nil
}
