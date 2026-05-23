// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newWaitCmd polls Riverside's status endpoints for a session until selected facets are ready.
// Exits 0 on ready, 2 on timeout.
func newWaitCmd(flags *rootFlags) *cobra.Command {
	var include string
	var timeout time.Duration
	var interval time.Duration
	var projectID string

	cmd := &cobra.Command{
		Use:   "wait <session-id>",
		Short: "Block until selected facets of a take are ready (transcript, assets, ai).",
		Long: `Polls these endpoints with adaptive backoff:
  transcript -> GET /api/v4/transcriptions/editableWithVoiceActivity/{sessionId}
  assets     -> GET /api/v4/take/{sessionId}/assets  (all tracks status=done)
  ai         -> GET /api/v4/projects/{projectId}/ai-generation-status (--project required)

--include selects a comma-separated subset (default: transcript,assets). Use --include ai when you also pass --project.
Exits 0 when all selected facets are ready; 2 if --timeout elapses first.`,
		Example:     "  riverside-fm-pp-cli wait bf487406-af40-4bb4-b7f9-a6b49047b55d --include transcript,assets --timeout 30m",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sid := strings.TrimSpace(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			facets := map[string]bool{}
			for _, f := range strings.Split(include, ",") {
				facets[strings.TrimSpace(f)] = true
			}
			if facets["ai"] && projectID == "" {
				return usageErr(fmt.Errorf("--include ai requires --project"))
			}

			deadline := time.Now().Add(timeout)
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			for {
				if time.Now().After(deadline) {
					if flags.asJSON {
						fmt.Fprintf(cmd.OutOrStdout(), `{"ready":false,"timeout":true,"session_id":%q}`+"\n", sid)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "TIMEOUT: session %s not ready after %s\n", sid, timeout)
					}
					return &cliError{code: 2, err: fmt.Errorf("timeout")}
				}

				status := pollFacets(c, sid, projectID, facets)

				if status.allReady() {
					if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
						j, _ := json.MarshalIndent(status, "", "  ")
						fmt.Fprintln(cmd.OutOrStdout(), string(j))
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "READY: session %s — %s\n", sid, status.summary())
					}
					return nil
				}

				if !flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "waiting... %s\n", status.summary())
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
				}
			}
		},
	}

	cmd.Flags().StringVar(&include, "include", "transcript,assets", "Comma-separated facets to wait on: transcript, assets, ai")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Max time to wait (e.g. 30m, 1h)")
	cmd.Flags().DurationVar(&interval, "interval", 15*time.Second, "Poll interval (with adaptive backoff floor)")
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required when --include ai)")
	return cmd
}

type facetStatus struct {
	SessionID  string `json:"session_id"`
	Transcript struct {
		Wanted bool `json:"wanted"`
		Ready  bool `json:"ready"`
	} `json:"transcript"`
	Assets struct {
		Wanted bool `json:"wanted"`
		Ready  bool `json:"ready"`
	} `json:"assets"`
	AI struct {
		Wanted bool `json:"wanted"`
		Ready  bool `json:"ready"`
	} `json:"ai"`
}

func (f facetStatus) allReady() bool {
	if f.Transcript.Wanted && !f.Transcript.Ready {
		return false
	}
	if f.Assets.Wanted && !f.Assets.Ready {
		return false
	}
	if f.AI.Wanted && !f.AI.Ready {
		return false
	}
	return true
}

func (f facetStatus) summary() string {
	var parts []string
	if f.Transcript.Wanted {
		parts = append(parts, "transcript="+rdy(f.Transcript.Ready))
	}
	if f.Assets.Wanted {
		parts = append(parts, "assets="+rdy(f.Assets.Ready))
	}
	if f.AI.Wanted {
		parts = append(parts, "ai="+rdy(f.AI.Ready))
	}
	return strings.Join(parts, " ")
}

func rdy(b bool) string {
	if b {
		return "ready"
	}
	return "pending"
}

type clientLike interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func pollFacets(c clientLike, sid, projectID string, want map[string]bool) facetStatus {
	st := facetStatus{SessionID: sid}
	st.Transcript.Wanted = want["transcript"]
	st.Assets.Wanted = want["assets"]
	st.AI.Wanted = want["ai"]

	if st.Transcript.Wanted {
		data, err := c.Get("/api/v4/transcriptions/editableWithVoiceActivity/"+url.PathEscape(sid), nil)
		if err == nil && hasTranscriptContent(data) {
			st.Transcript.Ready = true
		}
	}
	if st.Assets.Wanted {
		data, err := c.Get("/api/v4/take/"+url.PathEscape(sid)+"/assets", nil)
		if err == nil && allTracksDone(data) {
			st.Assets.Ready = true
		}
	}
	if st.AI.Wanted {
		data, err := c.Get("/api/v4/projects/"+url.PathEscape(projectID)+"/ai-generation-status", nil)
		if err == nil && aiAllDone(data) {
			st.AI.Ready = true
		}
	}
	return st
}

func aiAllDone(data json.RawMessage) bool {
	var doc map[string]json.RawMessage
	if json.Unmarshal(data, &doc) != nil {
		return false
	}
	// AI status payload is a map keyed by some take/recording id; each value has fields like
	// {magicClips:{status:"done"}, magicSegments:{status:"done"}, ...}. "done" or "initialized".
	type fac struct {
		Status string `json:"status"`
	}
	type entry map[string]fac
	for _, raw := range doc {
		var e entry
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		for _, f := range e {
			s := strings.ToLower(f.Status)
			if s == "inprogress" || s == "in_progress" || s == "processing" || s == "queued" || s == "" {
				return false
			}
		}
	}
	return true
}
