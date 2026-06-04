// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// Hand-authored gate-aware reachability probe for `doctor --probe-gate`. Suno's
// generation gate is adaptive: it returns 422 token_validation_failed for all
// clients when tripped and reopens after a cooldown. The standard doctor
// reachability check (GET /api/billing/info/) cannot see the gate — it is open
// at the billing layer while generation is blocked, which is a false green.
// This probe issues a real minimal inspiration-mode generation so the verdict
// reflects what `generate` actually experiences. It is opt-in because it spends
// credits and creates a clip when the gate is open (best-effort trashed).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
)

// gate-probe verdict classes.
const (
	gateOpen           = "open"
	gateTripped        = "tripped"
	gateAuthFailure    = "auth-failure"
	gateReachableOther = "reachable-other"
	gateUnreachable    = "unreachable"
)

// classifyGateProbe maps a generate-probe error to a verdict class. Pure and
// table-testable; the network call and side effects live in runGateProbe.
//   - nil error            -> gate open (a clip was created)
//   - captcha gate         -> tripped (free, no clip)
//   - HTTP 401             -> auth failure
//   - other HTTP error     -> reachable but unexpected (gate not tripped)
//   - transport error      -> unreachable
func classifyGateProbe(err error) string {
	switch {
	case err == nil:
		return gateOpen
	case isCaptchaRequired(err):
		return gateTripped
	default:
		var apiErr *client.APIError
		if As(err, &apiErr) {
			if apiErr.StatusCode == 401 {
				return gateAuthFailure
			}
			return gateReachableOther
		}
		return gateUnreachable
	}
}

// runGateProbe issues a minimal inspiration-mode generation through c (which
// carries the Device-Id + Browser-Token transport) and returns a human verdict
// string for the doctor report. On an open gate it best-effort trashes the clip
// it created — trashing does not refund the spent credits, it only avoids
// littering the library.
func runGateProbe(ctx context.Context, c *client.Client) string {
	// Respect a configured budget cap: the probe spends credits when the gate
	// is open, so honor the same cap that submitGeneration enforces rather than
	// silently breaching it from a diagnostic command.
	if bs, berr := openExistingStore(ctx); berr == nil && bs != nil {
		capCredits, period, exceeded, cerr := budgetCapExceeded(ctx, bs)
		_ = bs.Close()
		if cerr == nil && exceeded {
			return fmt.Sprintf("skipped (%s budget cap of %d credits reached; raise it with 'budget set %s <N>' or clear it before probing)", period, capCredits, period)
		}
	}
	mv, _ := resolveModel("", sunoGenerateModels, sunoGenerateModelOrder)
	body := buildGenerateBody(generateInput{
		createMode: "inspiration",
		mv:         mv,
		prompt:     "doctor gate probe",
	})
	data, _, err := c.Post(ctx, sunoGeneratePath, body)
	switch classifyGateProbe(err) {
	case gateOpen:
		ids := probeClipIDs(data)
		trashProbeClips(ctx, c, ids)
		if len(ids) > 0 {
			return fmt.Sprintf("open — generation reachable; probe created and trashed clip(s) %s (this spent generation credits)", strings.Join(ids, ", "))
		}
		return "open — generation reachable (a probe generation was submitted; this spent generation credits)"
	case gateTripped:
		return "tripped — the adaptive hCaptcha gate is active right now; no clip created and no credits spent. Wait for the cooldown or pass --token."
	case gateAuthFailure:
		return "auth-failure (HTTP 401) at the generate endpoint — credentials were rejected"
	case gateReachableOther:
		return fmt.Sprintf("reachable — gate not tripped, but the probe returned an unexpected error: %v", err)
	default:
		return fmt.Sprintf("unreachable: %v", err)
	}
}

// probeClipIDs extracts clip IDs from a generate response body (best-effort).
func probeClipIDs(data []byte) []string {
	var resp sunoGenerateResponse
	if json.Unmarshal(data, &resp) != nil {
		return nil
	}
	ids := make([]string, 0, len(resp.Clips))
	for _, raw := range resp.Clips {
		var cs clipStatus
		if json.Unmarshal(raw, &cs) == nil && cs.ID != "" {
			ids = append(ids, cs.ID)
		}
	}
	return ids
}

// trashProbeClips moves probe-created clips to trash (POST /api/feed/trash).
// Best-effort: a failure is ignored since the probe verdict is already known.
func trashProbeClips(ctx context.Context, c *client.Client, ids []string) {
	if len(ids) == 0 {
		return
	}
	// Detached short context so cleanup still fires if the probe's request
	// context was cancelled (Ctrl-C / doctor timeout) right after the create —
	// otherwise the billable clip would litter the library.
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	_, _, _ = c.Post(cctx, "/api/feed/trash", map[string]any{"ids": ids})
}
