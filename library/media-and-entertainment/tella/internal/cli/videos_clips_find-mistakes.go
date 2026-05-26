// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): hand-added composition with mixed-auth surface. Tella's
// Cut-panel "Find mistakes" button has no public-API endpoint (verified
// via 404 smoke test against api.tella.com on 2026-05-16). The detection
// step lives at `POST https://prod-stream.tella.tv/ai-mistakes/analyze-scene`
// (Server-Sent Events; rejects the public-API Bearer token with 401
// `not_authenticated`; requires a session cookie issued to a logged-in
// browser).
//
// To keep the apply step honest, we use the PUBLIC `POST /cut` endpoint
// for every detected mistake rather than the unofficial frontend PATCH
// the web UI uses. The unofficial PATCH replaces the clip's entire cuts
// list (destructive against prior remove-buffers / remove-fillers work
// in the same session); the public /cut endpoint merges. So this
// command:
//   1. Calls analyze-scene with cookie auth (unofficial, required)
//   2. For each detected mistake, POSTs the cut to /v1/.../cut with
//      Bearer auth (official, additive)
//
// Cataloged in .printing-press-patches.json#add-cut-panel-parity.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/config"
	"github.com/spf13/cobra"
)

func newVideosClipsFindMistakesCmd(flags *rootFlags) *cobra.Command {
	var unofficial bool
	var detectOnly bool
	cmd := &cobra.Command{
		Use:   "find-mistakes <id> <clipId>",
		Short: "Run Tella's AI mistake detector against a clip and apply the detected cuts. Detection is UNOFFICIAL — requires --unofficial + TELLA_SESSION_COOKIE.",
		Long: `find-mistakes calls the AI mistake-detection service the Tella web app uses on the Cut panel.

The DETECT step calls prod-stream.tella.tv/ai-mistakes/analyze-scene (Server-Sent
Events). That endpoint is NOT part of Tella's public API and rejects the
public-API Bearer token (401). The CLI authenticates by sending a session cookie
copied from a logged-in browser.

The APPLY step uses the PUBLIC /v1/videos/{id}/clips/{clipId}/cut endpoint to
post each detected cut (Bearer auth). This keeps the mutation surface
documented/supported and additive with other edit-pass flags.

REQUIRED:
  --unofficial             explicit opt-in (the AI service is undocumented and may break)
  TELLA_SESSION_COOKIE     env var with the raw browser Cookie header value

The cookie expires; refresh from DevTools → Application → Cookies → tella.tv.`,
		Example: `  TELLA_SESSION_COOKIE='__Secure-Tella.session=...' tella-pp-cli videos clips find-mistakes vid_abc cl_xyz --unofficial`,
		// No pp:endpoint annotation: mixed-auth composition.
		RunE: func(cmd *cobra.Command, args []string) error {
			if !unofficial {
				return usageErr(fmt.Errorf("find-mistakes calls Tella's unofficial AI service (prod-stream.tella.tv). " +
					"Pass --unofficial to opt in; the detection endpoint isn't part of the public API and may break"))
			}
			if len(args) < 2 {
				_ = cmd.Help()
				return usageErr(fmt.Errorf("usage: %s <id> <clipId>", cmd.CommandPath()))
			}
			videoID, clipID := args[0], args[1]

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			uc, err := newUnofficialClient(cfg.SessionCookie, flags.timeout)
			if err != nil {
				return configErr(err)
			}
			mistakes, unknownEvents, status, err := analyzeMistakes(uc, videoID, clipID)
			if err != nil {
				return apiErr(err)
			}

			type plannedCut struct {
				FromMs     int     `json:"fromMs"`
				ToMs       int     `json:"toMs"`
				Reasoning  string  `json:"reasoning,omitempty"`
				WordsToCut string  `json:"wordsToCut,omitempty"`
				Confidence float64 `json:"confidence,omitempty"`
			}
			planned := make([]plannedCut, 0, len(mistakes))
			for _, m := range mistakes {
				if m.Trim.Duration <= 0 {
					continue
				}
				planned = append(planned, plannedCut{
					FromMs:     int(m.Trim.StartTime + 0.5),
					ToMs:       int(m.Trim.StartTime + m.Trim.Duration + 0.5),
					Reasoning:  m.Reasoning,
					WordsToCut: m.WordsToCut,
					Confidence: m.Confidence,
				})
			}

			result := map[string]any{
				"video_id":          videoID,
				"clip_id":           clipID,
				"detected_mistakes": len(mistakes),
				"unknown_events":    unknownEvents,
				"planned":           planned,
				"analyze_status":    status,
			}

			if flags.dryRun || detectOnly {
				result["dry_run"] = flags.dryRun
				result["detect_only"] = detectOnly
				result["applied"] = false
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Official client for the apply step. Public /cut is Bearer-auth
			// and additive with whatever other cuts already live on the clip.
			oc, err := flags.newClient()
			if err != nil {
				return err
			}

			type appliedCut struct {
				FromMs int    `json:"fromMs"`
				ToMs   int    `json:"toMs"`
				Status int    `json:"status,omitempty"`
				Error  string `json:"error,omitempty"`
			}
			applied := make([]appliedCut, 0, len(planned))
			succeeded, failed := 0, 0
			for _, p := range planned {
				_, st, postErr := oc.Post(
					fmt.Sprintf("/v1/videos/%s/clips/%s/cut", videoID, clipID),
					map[string]any{"fromMs": p.FromMs, "toMs": p.ToMs},
				)
				ac := appliedCut{FromMs: p.FromMs, ToMs: p.ToMs}
				if postErr != nil {
					failed++
					ac.Error = postErr.Error()
				} else {
					succeeded++
					ac.Status = st
				}
				applied = append(applied, ac)
			}
			result["applied"] = true
			result["applied_ops"] = succeeded
			result["failed_ops"] = failed
			result["cuts"] = applied
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&unofficial, "unofficial", false, "Required: explicit opt-in to call Tella's undocumented AI service for detection")
	cmd.Flags().BoolVar(&detectOnly, "detect-only", false, "Print the detected mistakes without applying any cuts")
	return cmd
}

// analyzeMistakes POSTs to analyze-scene and parses the SSE stream.
// Returns the detected mistakes (in stream order), unknown event count, and HTTP status.
// Kept as a free function so it can be exercised by tests without going
// through the full cobra runner.
func analyzeMistakes(uc *unofficialClient, videoID, clipID string) (mistakes []detectedMistake, unknownEvents int, status int, err error) {
	baseURL := uc.aiBaseURL
	if baseURL == "" {
		baseURL = unofficialAIHost
	}
	url := fmt.Sprintf("%s/ai-mistakes/analyze-scene", baseURL)
	body := map[string]any{
		"storyID": videoID,
		"sceneID": clipID,
	}
	stream, status, err := uc.postSSE(url, body)
	if err != nil {
		return nil, 0, status, err
	}
	defer stream.Close()
	mistakes, unknownEvents, parseErr := parseMistakesSSE(stream)
	if parseErr != nil {
		return mistakes, unknownEvents, status, parseErr
	}
	return mistakes, unknownEvents, status, nil
}
