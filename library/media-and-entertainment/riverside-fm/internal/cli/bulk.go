// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBulkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk operations across a studio: export every take's transcript + asset metadata + manifest with a resume cursor.",
	}
	cmd.AddCommand(newBulkExportCmd(flags))
	return cmd
}

func newBulkExportCmd(flags *rootFlags) *cobra.Command {
	var studio string
	var since string
	var outDir string
	var resume bool

	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export every project / take / asset for a studio with resume.",
		Long:        "Walks projects-by-studio -> takes-by-project -> transcripts + take/assets endpoints. Writes each take's transcript + assets-manifest + per-participant HLS manifest URL to <out>/<studio>/<project>/<session>/. Resume state lives in <out>/.resume.json so interruption never restarts the walk.",
		Example:     `  riverside-fm-pp-cli bulk export --studio damien-stevenss-studio --since 2026-04-01 --out ./archive`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if studio == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if outDir == "" {
				outDir = "./riverside-export"
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}

			var sinceTime *time.Time
			if since != "" {
				t, perr := parseSince(since)
				if perr != nil {
					return usageErr(fmt.Errorf("--since %q: %w", since, perr))
				}
				sinceTime = &t
			}

			resumePath := filepath.Join(outDir, ".resume.json")
			done := map[string]bool{}
			if resume {
				if b, err := os.ReadFile(resumePath); err == nil {
					_ = json.Unmarshal(b, &done)
				}
			}

			summary := bulkSummary{}

			// 1. List projects in studio
			pp := "/api/v4/projects/studio/" + url.PathEscape(studio)
			projData, err := c.Get(pp, map[string]string{"offset": "0", "limit": "200", "sortBy": "createdAt", "orderBy": "desc"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			projects := extractProjects(projData)
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d projects in studio %s\n", len(projects), studio)

			for _, proj := range projects {
				if sinceTime != nil && proj.CreatedAt.Before(*sinceTime) {
					summary.SkippedOlderThanSince++
					continue
				}
				projDir := filepath.Join(outDir, sanitize(studio), sanitize(proj.Title)+"-"+proj.ID)
				if err := os.MkdirAll(projDir, 0o755); err != nil {
					return err
				}

				// 2. List takes per project
				tp := "/api/v4/projects/" + url.PathEscape(proj.ID) + "/takes"
				takesData, err := c.Get(tp, map[string]string{"offset": "0", "limit": "100"})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "WARN: list takes for %s: %v\n", proj.ID, err)
					summary.Errors++
					continue
				}
				takes := extractTakes(takesData)

				for _, take := range takes {
					if done[take.SessionID] {
						summary.AlreadyDone++
						continue
					}
					takeDir := filepath.Join(projDir, sanitize(take.Title)+"-"+take.SessionID)
					if err := os.MkdirAll(takeDir, 0o755); err != nil {
						return err
					}

					// 3a. Transcript
					tt := "/api/v4/transcriptions/editableWithVoiceActivity/" + url.PathEscape(take.SessionID)
					tdata, terr := c.Get(tt, nil)
					if terr == nil && hasTranscriptContent(tdata) {
						_ = os.WriteFile(filepath.Join(takeDir, "transcript.json"), tdata, 0o644)
						_ = os.WriteFile(filepath.Join(takeDir, "transcript.txt"), []byte(transcriptToPlainText(tdata)), 0o644)
						summary.TranscriptsWritten++
					}

					// 3b. Asset metadata
					ap := "/api/v4/take/" + url.PathEscape(take.SessionID) + "/assets"
					adata, aerr := c.Get(ap, nil)
					if aerr == nil {
						_ = os.WriteFile(filepath.Join(takeDir, "assets.json"), adata, 0o644)
						summary.AssetManifestsWritten++

						// 3c. VOD manifest per participant
						handles := extractParticipantHandles(adata)
						for _, h := range handles {
							vp := "/api/v4/vod/" + url.PathEscape(take.SessionID) + "/" + url.PathEscape(h)
							vdata, verr := c.Get(vp, nil)
							if verr == nil && len(vdata) > 0 {
								_ = os.WriteFile(filepath.Join(takeDir, h+".m3u8"), vdata, 0o644)
								summary.HlsManifestsWritten++
							}
						}
					}

					done[take.SessionID] = true
					summary.TakesProcessed++

					// Persist resume cursor every take
					if rb, err := json.Marshal(done); err == nil {
						_ = os.WriteFile(resumePath, rb, 0o644)
					}
				}
			}

			if flags.asJSON {
				sb, _ := json.MarshalIndent(summary, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(sb))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDone. takes=%d transcripts=%d assets=%d hls=%d errors=%d skipped_resume=%d skipped_since=%d\n",
					summary.TakesProcessed, summary.TranscriptsWritten, summary.AssetManifestsWritten, summary.HlsManifestsWritten,
					summary.Errors, summary.AlreadyDone, summary.SkippedOlderThanSince)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&studio, "studio", "", "Studio slug (required)")
	cmd.Flags().StringVar(&since, "since", "", "Only process projects created after this date (YYYY-MM-DD or duration like 30d / 7d)")
	cmd.Flags().StringVar(&outDir, "out", "./riverside-export", "Output directory root")
	cmd.Flags().BoolVar(&resume, "resume", true, "Skip takes already exported in this directory")
	return cmd
}

type bulkSummary struct {
	TakesProcessed        int `json:"takes_processed"`
	TranscriptsWritten    int `json:"transcripts_written"`
	AssetManifestsWritten int `json:"asset_manifests_written"`
	HlsManifestsWritten   int `json:"hls_manifests_written"`
	Errors                int `json:"errors"`
	AlreadyDone           int `json:"already_done"`
	SkippedOlderThanSince int `json:"skipped_older_than_since"`
}

type projectSummary struct {
	ID        string
	Title     string
	CreatedAt time.Time
}

type takeSummary struct {
	ID        string
	SessionID string
	Title     string
}

func extractProjects(data json.RawMessage) []projectSummary {
	// /api/v4/projects/studio/{slug} returns either {projects:[...]} or an array directly.
	var wrapped struct {
		Projects []struct {
			ID        string `json:"_id"`
			Title     string `json:"title"`
			CreatedAt string `json:"createdAt"`
		} `json:"projects"`
	}
	if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Projects) > 0 {
		out := make([]projectSummary, 0, len(wrapped.Projects))
		for _, p := range wrapped.Projects {
			t, _ := time.Parse(time.RFC3339, p.CreatedAt)
			out = append(out, projectSummary{ID: p.ID, Title: p.Title, CreatedAt: t})
		}
		return out
	}
	var arr []struct {
		ID        string `json:"_id"`
		Title     string `json:"title"`
		CreatedAt string `json:"createdAt"`
	}
	if json.Unmarshal(data, &arr) == nil {
		out := make([]projectSummary, 0, len(arr))
		for _, p := range arr {
			t, _ := time.Parse(time.RFC3339, p.CreatedAt)
			out = append(out, projectSummary{ID: p.ID, Title: p.Title, CreatedAt: t})
		}
		return out
	}
	return nil
}

func extractTakes(data json.RawMessage) []takeSummary {
	var wrapped struct {
		Takes []struct {
			ID         string `json:"_id"`
			Title      string `json:"title"`
			Recordings []struct {
				SessionID string `json:"sessionId"`
			} `json:"recordings"`
		} `json:"takes"`
	}
	if json.Unmarshal(data, &wrapped) != nil {
		return nil
	}
	out := make([]takeSummary, 0, len(wrapped.Takes))
	for _, t := range wrapped.Takes {
		sid := ""
		if len(t.Recordings) > 0 {
			sid = t.Recordings[0].SessionID
		}
		if sid == "" {
			continue
		}
		out = append(out, takeSummary{ID: t.ID, SessionID: sid, Title: t.Title})
	}
	return out
}

func sanitize(s string) string {
	if s == "" {
		return "untitled"
	}
	bad := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, b := range bad {
		s = strings.ReplaceAll(s, b, "-")
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return strings.TrimSpace(s)
}

func parseSince(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	// duration form: e.g., 30d, 7d, 24h
	if strings.HasSuffix(s, "d") {
		var days int
		_, err := fmt.Sscanf(s, "%dd", &days)
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("invalid date or duration")
}
