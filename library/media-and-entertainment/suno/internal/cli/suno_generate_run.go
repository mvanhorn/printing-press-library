// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// Shared generation-flow plumbing for the user-facing generate/describe/
// extend/cover/remaster commands: the captcha gate, the POST to
// /api/generate/v2-web/, store-upsert of returned clips, the status fetch
// (GET /api/feed/?ids= in pairs of 2 — Suno bug with 4+), the optional
// poll-until-complete wait loop, and the post-complete mp3 download.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

const sunoGeneratePath = "/api/generate/v2-web/"

// captchaRequiredError returns the actionable usage error (exit 2) shown when
// Suno's adaptive hCaptcha challenge actually fires for a generation. The CLI
// attempts generation optimistically (no token); this only surfaces when the
// API rejects the request as captcha-gated. Generation never launches a
// browser/solver.
func captchaRequiredError() error {
	return usageErr(fmt.Errorf(
		"Suno required an hCaptcha token for this generation.\n" +
			"      Suno gates generation adaptively — many requests succeed with no token,\n" +
			"      but this one was challenged (typically after sustained use). Retry with:\n" +
			"        --token <hcaptcha-token>   (e.g. solved via 2Captcha)\n" +
			"      This CLI will not launch a browser or solver on your behalf."))
}

// isCaptchaRequired reports whether a generate error is Suno's adaptive
// hCaptcha challenge (HTTP 422 token_validation_failed / "we couldn't verify
// your request"). Because the client keeps the Clerk JWT fresh before every
// call, a token_validation_failed on the generate endpoint means the request
// needs an hCaptcha token, not a stale session JWT.
func isCaptchaRequired(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "token_validation_failed") ||
		strings.Contains(msg, "verify your request")
}

// resolveModel maps a CLI model value to its wire key using the supplied
// table. Returns a usage error listing valid values on an unknown key.
func resolveModel(value string, table map[string]string, order []string) (string, error) {
	if value == "" {
		value = defaultGenerateModel
	}
	if mv, ok := table[value]; ok {
		return mv, nil
	}
	return "", usageErr(fmt.Errorf("unknown --model %q; valid values: %s", value, strings.Join(order, ", ")))
}

// sunoGenerateResponse is the POST /api/generate/v2-web/ response envelope.
type sunoGenerateResponse struct {
	Clips  []json.RawMessage `json:"clips"`
	Status string            `json:"status"`
}

// submitGeneration POSTs the full body, upserts any returned clips into the
// local store (best-effort), and returns the parsed response. The caller is
// responsible for the captcha gate and dry-run short-circuit before calling.
func submitGeneration(ctx context.Context, c *client.Client, configPath string, body sunoGenerateBody) (*sunoGenerateResponse, error) {
	// Budget enforcement (restored from the 2026-05-15 build): if a local
	// daily/monthly credit cap is configured, refuse to submit a generation
	// that would breach it. A missing store or unset cap is a no-op. The
	// caller short-circuits dry-run before reaching submitGeneration, so this
	// never fires on a dry run.
	if bs, berr := openExistingStore(ctx); berr == nil && bs != nil {
		capCredits, period, exceeded, cerr := budgetCapExceeded(ctx, bs)
		_ = bs.Close()
		if cerr != nil {
			return nil, fmt.Errorf("checking budget cap: %w", cerr)
		}
		if exceeded {
			return nil, usageErr(fmt.Errorf("%s budget cap of %d credits would be exceeded; raise it with 'suno-pp-cli budget set %s <N>' or remove it with 'suno-pp-cli budget clear'", period, capCredits, period))
		}
	}
	data, _, err := c.Post(ctx, sunoGeneratePath, body)
	if err != nil {
		return nil, err
	}
	var resp sunoGenerateResponse
	if uerr := json.Unmarshal(data, &resp); uerr != nil {
		return nil, fmt.Errorf("parsing generate response: %w", uerr)
	}
	upsertClips(ctx, resp.Clips)
	return &resp, nil
}

// upsertClips writes returned clips into the local store as resource_type
// 'clips'. Best-effort: store-open or per-clip failures are ignored so a
// successful generation is never reported as a failure due to local IO.
func upsertClips(ctx context.Context, clips []json.RawMessage) {
	if len(clips) == 0 {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("suno-pp-cli"))
	if err != nil {
		return
	}
	defer db.Close()
	for _, clip := range clips {
		_ = db.UpsertClips(clip)
	}
}

// clipStatus is the slice of clip fields the status/wait/download paths read.
type clipStatus struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Status   string          `json:"status"`
	AudioURL string          `json:"audio_url"`
	Metadata json.RawMessage `json:"metadata"`
}

// fetchClips fetches clips by ID via GET /api/feed/?ids=, batching IDs in
// pairs of 2 (Suno returns malformed results for 4+ IDs in one call). Returns
// the parsed clip slice in request order (best-effort; missing IDs are
// skipped by the API).
func fetchClips(ctx context.Context, c *client.Client, ids []string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	for i := 0; i < len(ids); i += 2 {
		end := i + 2
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]
		data, err := c.GetNoCache(ctx, "/api/feed/", map[string]string{"ids": strings.Join(batch, ",")})
		if err != nil {
			return all, err
		}
		var clips []json.RawMessage
		if json.Unmarshal(data, &clips) != nil {
			// Some responses wrap clips in an object — try {clips:[...]}.
			var env struct {
				Clips []json.RawMessage `json:"clips"`
			}
			if json.Unmarshal(data, &env) == nil {
				clips = env.Clips
			}
		}
		all = append(all, clips...)
	}
	return all, nil
}

// clipIsTerminal reports whether a clip's status is a finished state
// (complete/streaming-complete or error). Suno reports "complete" for a
// finished clip and "error" for a failed one; "streaming" / "submitted" /
// "queued" are in-progress.
func clipIsTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "complete", "error":
		return true
	}
	return false
}

// waitForClips polls fetchClips until every requested clip reaches a terminal
// status or the context deadline/cap is hit. Under dogfood it polls once.
// Returns the final clip slice.
func waitForClips(ctx context.Context, c *client.Client, ids []string, events *cobra.Command) ([]json.RawMessage, error) {
	deadline := time.Now().Add(10 * time.Minute)
	single := cliutil.IsDogfoodEnv()
	for {
		clips, err := fetchClips(ctx, c, ids)
		if err != nil {
			return clips, err
		}
		done := true
		for _, raw := range clips {
			var cs clipStatus
			if json.Unmarshal(raw, &cs) == nil && !clipIsTerminal(cs.Status) {
				done = false
				break
			}
		}
		// Re-upsert the refreshed clips so the local store reflects the
		// completed state.
		upsertClips(ctx, clips)
		if done || single || time.Now().After(deadline) {
			return clips, nil
		}
		if humanFriendly && events != nil {
			fmt.Fprintln(events.ErrOrStderr(), "waiting for clips to finish...")
		}
		select {
		case <-ctx.Done():
			return clips, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// deviceIDFromFlags resolves the configured Device-Id for the active config.
func deviceIDFromFlags(flags *rootFlags) string {
	return config.DeviceIDFor(flags.configPath)
}

// runGenerationFlow is the shared tail of generate/describe/extend/cover/
// remaster: it submits the body, optionally waits for completion, optionally
// downloads the finished mp3s, and prints the result. captchaToken/noCaptcha
// gate is checked by the caller. wait/download are opt-in.
func runGenerationFlow(cmd *cobra.Command, flags *rootFlags, body sunoGenerateBody, wait bool, downloadDir string, workspaceID string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	resp, err := submitGeneration(cmd.Context(), c, flags.configPath, body)
	if err != nil {
		if isCaptchaRequired(err) {
			return captchaRequiredError()
		}
		return classifyAPIError(err, flags)
	}

	ids := make([]string, 0, len(resp.Clips))
	for _, raw := range resp.Clips {
		var cs clipStatus
		if json.Unmarshal(raw, &cs) == nil && cs.ID != "" {
			ids = append(ids, cs.ID)
		}
	}

	// --workspace destination: add the freshly generated clips to the target
	// workspace (Suno "project") via the confirmed add endpoint. Best-effort —
	// a failed add is a warning, not a generation failure.
	if workspaceID != "" && len(ids) > 0 {
		addPath := replacePathParam("/api/project/{workspace_id}/clips", "workspace_id", workspaceID)
		addBody := map[string]any{
			"update_type": "add",
			"metadata":    map[string]any{"clip_ids": ids},
		}
		if _, _, aerr := c.Post(cmd.Context(), addPath, addBody); aerr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: generated %d clip(s) but failed to add to workspace %s: %v\n", len(ids), workspaceID, aerr)
		} else if humanFriendly {
			fmt.Fprintf(cmd.ErrOrStderr(), "added %d clip(s) to workspace %s\n", len(ids), workspaceID)
		}
	}

	finalClips := resp.Clips
	if (wait || downloadDir != "") && len(ids) > 0 {
		waited, werr := waitForClips(cmd.Context(), c, ids, cmd)
		if werr != nil {
			return classifyAPIError(werr, flags)
		}
		if len(waited) > 0 {
			finalClips = waited
		}
	}

	var downloaded []string
	if downloadDir != "" {
		for _, raw := range finalClips {
			var cs clipStatus
			if json.Unmarshal(raw, &cs) != nil || cs.AudioURL == "" {
				continue
			}
			out, derr := downloadClipMP3(cmd.Context(), c, raw, downloadDir)
			if derr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: download of clip %s failed: %v\n", cs.ID, derr)
				continue
			}
			downloaded = append(downloaded, out)
		}
	}

	if flags.asJSON {
		var clipObjs []json.RawMessage = finalClips
		out := map[string]any{
			"status": resp.Status,
			"clips":  clipObjs,
		}
		if len(downloaded) > 0 {
			out["downloaded"] = downloaded
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}

	for _, raw := range finalClips {
		var cs clipStatus
		if json.Unmarshal(raw, &cs) != nil {
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  [%s]\n", cs.ID, cs.Title, cs.Status)
	}
	for _, d := range downloaded {
		fmt.Fprintf(cmd.OutOrStdout(), "downloaded: %s\n", d)
	}
	return nil
}
