// Helpers shared by domain-goat commands: store open, normalization, output.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/normalize"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/store"
)

// openStore opens the local SQLite store and runs domain-goat schema migration.
func openStore(ctx context.Context) (*store.Store, error) {
	path := defaultDBPath("domain-goat-pp-cli")
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := s.MigrateDomainSchema(ctx); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return s, nil
}

// normalizeAll cleans up domain inputs (lowercase, punycode IDN, no trailing dot).
func normalizeAll(inputs []string) ([]string, error) {
	out := make([]string, 0, len(inputs))
	for _, in := range inputs {
		in = strings.TrimSpace(in)
		if in == "" {
			continue
		}
		n, err := normalize.FQDN(in)
		if err != nil {
			return nil, fmt.Errorf("invalid domain %q: %w", in, err)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid domains provided")
	}
	return out, nil
}

// emitJSON marshals v via the printJSONFiltered path (respects --select/--compact/etc).
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// wantJSON returns true when output should be JSON (explicit --json or non-TTY).
func wantJSON(cmd *cobra.Command, flags *rootFlags) bool {
	if flags.asJSON {
		return true
	}
	if flags.csv || flags.plain || flags.quiet {
		return false
	}
	return !isTerminal(cmd.OutOrStdout()) && !humanFriendly
}

// rawJSON returns json.RawMessage from any value (panic on encode error — caller guarantees encodable).
func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// joinTLDs accepts a comma-separated string and returns lowercase trimmed TLDs.
func joinTLDs(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(p, ".")))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
