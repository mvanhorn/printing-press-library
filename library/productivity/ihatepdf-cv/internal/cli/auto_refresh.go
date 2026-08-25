package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv/internal/cliutil"
)

// autoRefreshIfStale attaches local-store freshness metadata to read
// provenance. It never fabricates a remote sync: ihatepdf.cv exposes no
// document API, so stale local data is reported instead of silently refreshed.
func autoRefreshIfStale(ctx context.Context, flags *rootFlags) {
	if flags == nil {
		return
	}
	report, err := cliutil.EnsureFresh(ctx, defaultDBPath("ihatepdf-cv-pp-cli"))
	if err == nil {
		flags.freshnessMeta = report
	}
}
