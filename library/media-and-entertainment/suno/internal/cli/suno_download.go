// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `download` — fetch a clip's audio to disk. mp3: read clip (store first,
// else GET /api/feed/?ids=), download audio_url to <dir>/<title>.mp3 and
// embed the lyrics (metadata.prompt) into an ID3v2 tag. wav: trigger
// /api/gen/{id}/convert_wav/, poll /api/gen/{id}/wav_file/ until the URL is
// populated, then download the wav. Read-only with respect to Suno's server
// state (it writes local files only), so it is annotated mcp:read-only.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	id3v2 "github.com/bogem/id3v2/v2"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

func newSunoDownloadCmd(flags *rootFlags) *cobra.Command {
	var format string
	var outDir string

	cmd := &cobra.Command{
		Use:   "download <clip_id> [<clip_id>...]",
		Short: "Download clip audio (mp3 or wav) to disk; mp3 embeds lyrics into ID3",
		Long: `Download one or more clips' audio to local files.

mp3 (default): reads the clip from the local store (if synced) or from the API,
downloads its audio_url, and embeds the clip's lyrics (metadata.prompt) into an
ID3v2 tag.

wav: triggers lossless conversion (POST /api/gen/<id>/convert_wav/), polls until
the WAV URL is ready, then downloads it. WAV conversion requires a Pro/Premier
account.`,
		Example: `  suno-pp-cli download 550e8400-e29b-41d4-a716-446655440000
  suno-pp-cli download <id> --format wav --out ~/Music
  suno-pp-cli download <id1> <id2> --out ./tracks --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("at least one clip_id is required"))
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format != "mp3" && format != "wav" {
				return usageErr(fmt.Errorf("--format must be mp3 or wav (got %q)", format))
			}
			if outDir == "" {
				outDir = "."
			}

			if dryRunOK(flags) {
				for _, id := range args {
					fmt.Fprintf(cmd.OutOrStdout(), "would download clip %s as %s to %s\n", id, format, outDir)
				}
				return nil
			}
			// Verify mode: do not hit the network or write files.
			if cliutil.IsVerifyEnv() {
				for _, id := range args {
					fmt.Fprintf(cmd.OutOrStdout(), "would download clip %s as %s to %s\n", id, format, outDir)
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var results []string
			for _, id := range args {
				clip, cerr := loadClip(cmd.Context(), c, id)
				if cerr != nil {
					return classifyAPIError(cerr, flags)
				}
				var out string
				if format == "wav" {
					out, err = downloadClipWAV(cmd.Context(), c, id, clip, outDir)
				} else {
					out, err = downloadClipMP3(cmd.Context(), c, clip, outDir)
				}
				if err != nil {
					return classifyAPIError(err, flags)
				}
				results = append(results, out)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"downloaded": results}, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "downloaded: %s\n", r)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "mp3", "Audio format: mp3 or wav")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory (default: current directory)")
	return cmd
}

// loadClip resolves a clip's JSON by ID: from the local store if present,
// otherwise via GET /api/feed/?ids=.
func loadClip(ctx context.Context, c *client.Client, id string) (json.RawMessage, error) {
	if db, err := store.OpenWithContext(ctx, defaultDBPath("suno-pp-cli")); err == nil {
		defer db.Close()
		if data, gerr := db.Get("clips", id); gerr == nil && len(data) > 0 {
			return data, nil
		}
	}
	clips, err := fetchClips(ctx, c, []string{id})
	if err != nil {
		return nil, err
	}
	if len(clips) == 0 {
		return nil, notFoundErr(fmt.Errorf("clip %s not found", id))
	}
	return clips[0], nil
}

// clipDownloadFields extracts the fields the download path needs from clip JSON.
type clipDownloadFields struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	AudioURL string `json:"audio_url"`
	Metadata struct {
		Prompt string `json:"prompt"`
	} `json:"metadata"`
}

// downloadClipMP3 downloads the clip's audio_url to <dir>/<title>.mp3 and
// embeds the lyrics into an ID3v2 tag. Returns the written path.
func downloadClipMP3(ctx context.Context, c *client.Client, clip json.RawMessage, dir string) (string, error) {
	var cf clipDownloadFields
	if err := json.Unmarshal(clip, &cf); err != nil {
		return "", fmt.Errorf("parsing clip: %w", err)
	}
	if cf.AudioURL == "" {
		return "", fmt.Errorf("clip %s has no audio_url yet (still generating?)", cf.ID)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, sanitizeFilename(cf.Title, cf.ID)+".mp3")
	if err := downloadToFile(ctx, c.ConfiguredTimeout(), cf.AudioURL, dest); err != nil {
		return "", err
	}
	if cf.Metadata.Prompt != "" {
		embedLyrics(dest, cf.Title, cf.Metadata.Prompt)
	}
	return dest, nil
}

// wavFileResponse is the /api/gen/{id}/wav_file/ envelope.
type wavFileResponse struct {
	WavFileURL *string `json:"wav_file_url"`
}

// downloadClipWAV triggers conversion, polls for the wav URL, then downloads
// it to <dir>/<title>.wav. Polls up to ~60s (1 attempt under dogfood).
func downloadClipWAV(ctx context.Context, c *client.Client, id string, clip json.RawMessage, dir string) (string, error) {
	if _, _, err := c.Post(ctx, replacePathParam("/api/gen/{clip_id}/convert_wav/", "clip_id", id), map[string]any{}); err != nil {
		return "", err
	}
	deadline := time.Now().Add(60 * time.Second)
	single := cliutil.IsDogfoodEnv()
	var wavURL string
	for {
		data, err := c.GetNoCache(ctx, replacePathParam("/api/gen/{clip_id}/wav_file/", "clip_id", id), nil)
		if err != nil {
			return "", err
		}
		var resp wavFileResponse
		if json.Unmarshal(data, &resp) == nil && resp.WavFileURL != nil && *resp.WavFileURL != "" {
			wavURL = *resp.WavFileURL
			break
		}
		if single || time.Now().After(deadline) {
			return "", fmt.Errorf("WAV for clip %s not ready (conversion may require Pro/Premier or more time)", id)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	var cf clipDownloadFields
	_ = json.Unmarshal(clip, &cf)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, sanitizeFilename(cf.Title, id)+".wav")
	if err := downloadToFile(ctx, c.ConfiguredTimeout(), wavURL, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// downloadToFile streams url to dest using a plain HTTP client (the audio CDN
// is not the studio-api host and needs no Suno headers/auth).
func downloadToFile(ctx context.Context, timeout time.Duration, url, dest string) error {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	// #nosec G304 -- dest is filepath.Join(userDir, sanitizeFilename(...)); sanitizeFilename strips path separators so the filename cannot escape userDir.
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

// embedLyrics writes the lyrics (and title) into an ID3v2 tag on the mp3 at
// path. Best-effort: a tagging failure leaves the audio file intact and is
// surfaced as a stderr warning.
func embedLyrics(path, title, lyrics string) {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open %s to embed lyrics: %v\n", path, err)
		return
	}
	defer tag.Close()
	if title != "" {
		tag.SetTitle(title)
	}
	tag.AddUnsynchronisedLyricsFrame(id3v2.UnsynchronisedLyricsFrame{
		Encoding:          id3v2.EncodingUTF8,
		Language:          "eng",
		ContentDescriptor: "Lyrics",
		Lyrics:            lyrics,
	})
	if err := tag.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save lyrics tag to %s: %v\n", path, err)
	}
}

// sanitizeFilename produces a safe base filename from a clip title, falling
// back to the id when the title is empty or sanitizes to nothing.
func sanitizeFilename(title, id string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return id
	}
	repl := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', 0:
			return '_'
		}
		return r
	}
	cleaned := strings.Map(repl, title)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return id
	}
	return cleaned
}
