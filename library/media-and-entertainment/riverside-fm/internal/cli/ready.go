// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// newReadyCmd checks every synced studio for fully-ready takes.
// A take is "ready" when: every track.status == "done" AND transcription has speakers/voice-activity.
// Live mode: walks studios -> projects -> takes via the API.
func newReadyCmd(flags *rootFlags) *cobra.Command {
	var studio string

	cmd := &cobra.Command{
		Use:         "ready",
		Short:       "List takes that are fully ready: backup done, transcription done, no participant uploading.",
		Long:        "Walks studios -> projects -> takes via the API and filters to takes whose every track is done AND whose transcription has content. Use --studio to scope to one studio (recommended for large workspaces).",
		Example:     "  riverside-fm-pp-cli ready --studio damien-stevenss-studio --json --select session_id,project,duration_min",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if studio == "" {
				return usageErr(fmt.Errorf("--studio is required (no global studio inventory endpoint exposed by the API)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projData, err := c.Get("/api/v4/projects/studio/"+url.PathEscape(studio),
				map[string]string{"offset": "0", "limit": "200", "sortBy": "createdAt", "orderBy": "desc"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			projects := extractProjects(projData)

			type readyRow struct {
				Studio       string  `json:"studio"`
				Project      string  `json:"project"`
				ProjectID    string  `json:"project_id"`
				TakeID       string  `json:"take_id"`
				SessionID    string  `json:"session_id"`
				Title        string  `json:"title"`
				DurationMin  float64 `json:"duration_min"`
				Participants int     `json:"participants"`
			}
			rows := []readyRow{}

			for _, proj := range projects {
				tdata, err := c.Get("/api/v4/projects/"+url.PathEscape(proj.ID)+"/takes",
					map[string]string{"offset": "0", "limit": "100"})
				if err != nil {
					continue
				}
				takes := extractTakes(tdata)
				for _, take := range takes {
					adata, aerr := c.Get("/api/v4/take/"+url.PathEscape(take.SessionID)+"/assets", nil)
					if aerr != nil {
						continue
					}
					if !allTracksDone(adata) {
						continue
					}
					ttData, tterr := c.Get("/api/v4/transcriptions/editableWithVoiceActivity/"+url.PathEscape(take.SessionID), nil)
					if tterr != nil || !hasTranscriptContent(ttData) {
						continue
					}
					rows = append(rows, readyRow{
						Studio:       studio,
						Project:      proj.Title,
						ProjectID:    proj.ID,
						TakeID:       take.ID,
						SessionID:    take.SessionID,
						Title:        take.Title,
						DurationMin:  takeDurationMin(adata),
						Participants: len(extractParticipantHandles(adata)),
					})
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(j))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No fully-ready takes in studio %s.\n", studio)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d ready takes in %s:\n", len(rows), studio)
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s | %s | %s | %.1f min | %d participants\n",
					r.SessionID, r.Title, r.Project, r.DurationMin, r.Participants)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&studio, "studio", "", "Studio slug to scan (required)")
	return cmd
}

func allTracksDone(data json.RawMessage) bool {
	var probe struct {
		Take struct {
			Tracks []struct {
				Status string `json:"status"`
			} `json:"tracks"`
		} `json:"take"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return false
	}
	if len(probe.Take.Tracks) == 0 {
		return false
	}
	for _, t := range probe.Take.Tracks {
		s := strings.ToLower(t.Status)
		if s != "done" {
			return false
		}
	}
	return true
}

func takeDurationMin(data json.RawMessage) float64 {
	var probe struct {
		Take struct {
			Tracks []struct {
				Duration float64 `json:"duration"`
			} `json:"tracks"`
		} `json:"take"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return 0
	}
	var max float64
	for _, t := range probe.Take.Tracks {
		if t.Duration > max {
			max = t.Duration
		}
	}
	return max / 60.0
}
