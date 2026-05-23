// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
)

// resolveWorkspaceArg accepts a workspace ref, index, or title-substring and
// returns the canonical ref. Returns "" when arg is empty (caller decides).
func resolveWorkspaceArg(ctx context.Context, arg string) (string, error) {
	if arg == "" {
		return "", nil
	}
	return cmuxclient.ResolveWorkspaceRef(ctx, arg)
}

// parseSinceUnix accepts a duration string ("1h", "30m") OR a unix
// timestamp ("1778900000") and returns a unix-seconds float anchored to now.
// Distinct from the generator-emitted parseSinceDuration which returns
// time.Time and supports the "Nw" weeks shorthand the sync command uses.
func parseSinceUnix(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return float64(time.Now().Add(-d).Unix()), nil
	}
	if ts, err := strconv.ParseFloat(s, 64); err == nil {
		return ts, nil
	}
	return 0, fmt.Errorf("invalid duration: %q (use 30m, 2h, 24h, or a unix timestamp)", s)
}

// indexWorkspacesByRef returns a {ref -> Workspace} index.
func indexWorkspacesByRef(wss []cmuxclient.Workspace) map[string]cmuxclient.Workspace {
	out := make(map[string]cmuxclient.Workspace, len(wss))
	for _, w := range wss {
		out[w.Ref] = w
	}
	return out
}

// titleByWorkspace builds a {workspace_ref -> []surface_title} map by
// listing surfaces in each workspace, used by CanonicalState.
func titleByWorkspace(ctx context.Context, wss []cmuxclient.Workspace) map[string][]string {
	out := make(map[string][]string, len(wss))
	for _, w := range wss {
		surfaces, err := cmuxclient.ListSurfaces(ctx, w.Ref)
		if err != nil {
			continue
		}
		titles := make([]string, 0, len(surfaces))
		for _, s := range surfaces {
			titles = append(titles, s.Title)
		}
		out[w.Ref] = titles
	}
	return out
}

// strandedCountByWorkspace builds a {workspace_ref -> stranded count}.
func strandedCountByWorkspace(ctx context.Context, wss []cmuxclient.Workspace) map[string]int {
	out := make(map[string]int, len(wss))
	for _, w := range wss {
		health, err := cmuxclient.SurfaceHealth(ctx, w.Ref)
		if err != nil {
			continue
		}
		count := 0
		for _, h := range health {
			if !h.InWindow {
				count++
			}
		}
		out[w.Ref] = count
	}
	return out
}

// humanDuration formats a positive seconds delta as e.g. "32m" or "2h17m".
func humanDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", int(seconds))
	}
	d := time.Duration(seconds * float64(time.Second))
	str := d.Round(time.Minute).String()
	// strip trailing 0s like "2h0m0s"
	str = strings.TrimSuffix(str, "0s")
	return str
}
