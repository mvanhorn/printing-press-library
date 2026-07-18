// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/espn"
)

// espnCanaryAthlete is a stable, well-known ESPN soccer athlete (João Neves,
// id 355061) used to detect endpoint drift. The former ESPN search endpoint died
// silently to count:0 with no alarm; this canary turns the next such drift into a
// visible doctor WARN instead of a 100% silent miss.
const espnCanaryAthlete = "Joao Neves"

// espnCanary exercises the full ESPN chain (resolve + enrich) against a known
// athlete and returns a doctor status string. A degraded result is reported as a
// WARN, never a FAIL: ESPN is a best-effort context source, not core to a report.
func espnCanary(ctx context.Context) string {
	if cliutil.IsVerifyEnv() {
		return "skipped (verify mode)"
	}
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	c := espn.New()
	resolved, ok, err := c.Lookup(probeCtx, espnCanaryAthlete)
	if err != nil {
		return "WARN canary lookup errored; search endpoint may have drifted"
	}
	if !ok || resolved.AthleteID == "" {
		return "WARN canary athlete did not resolve; search endpoint may have drifted"
	}
	enr, err := c.Enrich(probeCtx, resolved.AthleteID)
	if err != nil {
		return "WARN canary enrich errored; overview endpoint may have drifted"
	}
	if enr == nil || enr.Stats == nil {
		return "WARN canary returned no stats; overview endpoint may have drifted"
	}
	return fmt.Sprintf("healthy (resolved %s, season stats present)", resolved.DisplayName)
}
