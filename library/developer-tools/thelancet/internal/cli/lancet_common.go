// Shared helpers for the hand-authored Lancet analytics commands. Not generated.

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/store"
)

// resolveJournalISSN maps a journal flag to a single ISSN scope. An empty flag
// returns "" (no journal scoping). "all" also returns "" (whole store). A named
// slug/ISSN returns that journal's ISSN. Unknown values return an error.
func resolveJournalISSN(journal string) (string, error) {
	if journal == "" || journal == "all" || journal == "family" {
		return "", nil
	}
	js, ok := lancet.Lookup(journal)
	if !ok || len(js) == 0 {
		return "", usageErr(fmt.Errorf("unknown journal %q; run 'thelancet-pp-cli refresh --help' for slugs, or use 'all'", journal))
	}
	return js[0].ISSN, nil
}

// ensureLancetStore opens the local analytics store and, when it is empty and
// the caller has not forced --data-source local, auto-syncs a bounded flagship
// sample from OpenAlex so first-run analytics return data instead of nothing.
// Returns the store; the caller must Close it.
func ensureLancetStore(cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("thelancet-pp-cli")
	}
	if dir := filepath.Dir(dbPath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	st, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	// Pin to a single connection so the lancet DDL (EnsureSchema) and the
	// subsequent reads always share one SQLite connection. Without this, a
	// pooled second connection can miss freshly-created tables in certain
	// runtime environments (e.g. non-shared in-memory or uncheckpointed WAL).
	st.DB().SetMaxOpenConns(1)
	if err := lancet.EnsureSchema(cmd.Context(), st.DB()); err != nil {
		st.Close()
		return nil, err
	}
	n, _ := lancet.WorkCount(cmd.Context(), st.DB(), "")
	if n == 0 && flags.dataSource != "local" {
		c, cerr := flags.newClient()
		if cerr != nil {
			return st, nil // serve empty rather than fail; caller handles no-data
		}
		maxPages := 3
		if cliutil.IsDogfoodEnv() {
			maxPages = 2
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "local store empty; auto-syncing a flagship Lancet sample (run 'thelancet-pp-cli refresh --journal all' for full coverage)")
		journs, _ := lancet.Lookup("lancet")
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		if _, rerr := lancet.Refresh(ctx, c, st.DB(), journs, 0, maxPages, nil); rerr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "auto-sync failed (%v); returning local data only\n", rerr)
		}
	}
	return st, nil
}

// emitLancet writes a result slice as JSON for machine consumers, or hands the
// caller a flag indicating it should render a human table itself.
func emitLancet(cmd *cobra.Command, flags *rootFlags, v any) (handledAsJSON bool, err error) {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return true, printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}
	return false, nil
}
