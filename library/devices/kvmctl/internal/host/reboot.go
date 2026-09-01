package host

import (
	"context"
	"fmt"
)

// Reboot is the legacy CLI entry point that bridges --yes to a confirmation token.
// It retains the original argv shape (systemctl reboot --yes) for backward compat
// with existing TestRebootRequiresYesAndBindsTarget while also offering the new
// confirmation-aware path via Adapter.Reboot.
func Reboot(ctx context.Context, r Runner, p Profile, target string, yes bool) (map[string]any, error) {
	if !yes {
		return nil, fmt.Errorf("host.reboot requires explicit --yes")
	}
	// Use identity probe for preflight (bounded).
	id, err := Probe(ctx, r, p)
	if err != nil {
		return nil, err
	}
	if id["hostname"] != target {
		return nil, fmt.Errorf("host identity mismatch")
	}
	// Preserve legacy argv for existing tests: systemctl reboot --yes
	if p.Timeout <= 0 {
		p.Timeout = 10 * 1e9 // fallback; Probe normalizes same
	}
	if _, err = r.Run(ctx, []string{"systemctl", "reboot", "--yes"}, p.Timeout); err != nil {
		return nil, err
	}
	return map[string]any{"operation": "host.reboot", "target": target, "requested": true}, nil
}
