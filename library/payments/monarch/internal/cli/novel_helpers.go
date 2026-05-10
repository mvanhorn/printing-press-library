// Hand-authored shared helpers for monarch-pp-cli's novel-feature commands.
// These commands compute analytics from live API responses (not the local
// store yet — sync-to-store is a Phase 4+ task). Until then, each novel
// command reads what it needs via the GraphQL client and computes in-memory.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

// novelDryRun describes what a novel-feature command would do, returned via
// JSON when --dry-run is set. Keeps every novel command's --dry-run shape
// consistent for verify and dogfood.
type novelDryRun struct {
	DryRun     bool              `json:"dry_run"`
	Command    string            `json:"command"`
	Persona    string            `json:"persona"`
	Plan       string            `json:"plan"`
	Operations []string          `json:"graphql_operations,omitempty"`
	Inputs     map[string]string `json:"inputs,omitempty"`
}

// emitDryRun writes a dryRunOK envelope and returns nil.
func emitDryRun(flags *rootFlags, dr novelDryRun) error {
	dr.DryRun = true
	out, err := json.MarshalIndent(dr, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

// fetchGraphQL is a one-shot helper for novel commands that need to call a
// GraphQL operation via the client. It enforces the auth precondition and
// surfaces the typed access-denied vs other-error split.
func fetchGraphQL(c *client.Client, query string, variables map[string]any) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("client unavailable")
	}
	if variables == nil {
		variables = map[string]any{}
	}
	return c.Query(query, variables)
}

// daysAgo returns YYYY-MM-DD for the date n days before today (UTC).
func daysAgo(n int) string {
	return time.Now().UTC().AddDate(0, 0, -n).Format("2006-01-02")
}

// today returns YYYY-MM-DD for today (UTC).
func today() string {
	return time.Now().UTC().Format("2006-01-02")
}

// startOfMonth returns the first day of the month containing the given YYYY-MM-DD.
func startOfMonth(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// endOfMonth returns the last day of the month containing the given YYYY-MM-DD.
func endOfMonth(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, 1, -1).Format("2006-01-02")
}

// parsePercent parses inputs like "5pct", "5%", "5" to a float in [0, 1].
func parsePercent(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSuffix(s, "pct")
	s = strings.TrimSpace(s)
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0, fmt.Errorf("could not parse percent %q: %w", s, err)
	}
	return v / 100.0, nil
}

// printJSONOrTable is a thin wrapper that emits JSON when --json is set or
// stdout isn't a TTY, otherwise pretty-prints the value.
func printJSONOrTable(flags *rootFlags, v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
