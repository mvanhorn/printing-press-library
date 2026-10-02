// pp:data-source live
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/repark"
)

// resolveReparkSyncRange applies the same explicit environment fallback to
// live requests and previews. It performs no IO.
func resolveReparkSyncRange(p *syncUserParams) (string, bool) {
	v := map[string]string{}
	p.applyTo("site", v, false)
	// An operator-supplied environment window is a reusable explicit scope.
	// Flags take precedence, including an explicitly empty (invalid) range.
	if _, supplied := v["range"]; !supplied {
		if window := strings.TrimSpace(os.Getenv("REPARK_SYNC_RANGE")); window != "" {
			p.flatGlobal["range"] = window
			v["range"] = window
		}
	}
	window, supplied := v["range"]
	return window, supplied
}

func validateReparkSyncParams(p *syncUserParams) error {
	window, _ := resolveReparkSyncRange(p)
	if window == "" {
		return fmt.Errorf("sync requires an explicit bounded marker window: --param 'range=C34.663534,135.516310N34.664W135.515S34.663E135.517'; use parking commands for live planning facts")
	}
	return repark.ValidateMarkerRange(window)
}
