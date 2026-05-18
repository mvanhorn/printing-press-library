// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/ingest"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"

	"github.com/spf13/cobra"
)

type syncFlags struct {
	full             bool
	includeDereg     bool
	includePre1982   bool
	fullNarratives   bool
	includeAddresses bool
	withFTS          bool
	everything       bool
	source           string // faa | ntsb | all (default faa — NTSB requires mdbtools)
}

func newSyncCmd() *cobra.Command {
	var f syncFlags
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download FAA bulk data and refresh the local store. NTSB sync is opt-in via --source.",
		Long: `Download FAA bulk data and refresh the local store.

The default profile syncs only the FAA Aircraft Registry — it has no system
dependencies. NTSB accident data lives behind --source ntsb (or --source all)
and requires the 'mdbtools' package to be installed (the NTSB ships its data
as a Microsoft Access .mdb file). Run 'airframe-pp-cli doctor' to confirm.

  airframe-pp-cli sync                         # FAA only, ~80 MB, no deps
  airframe-pp-cli sync --source ntsb           # NTSB only, requires mdbtools
  airframe-pp-cli sync --source all --with-fts # everything + full-text index

Flags like --include-dereg, --include-addresses apply to FAA; --include-pre1982
and --full-narratives apply to NTSB.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd, f)
		},
	}
	cmd.Flags().BoolVar(&f.full, "full", false, "Ignore If-Modified-Since cache and force a full re-download from each source")
	cmd.Flags().BoolVar(&f.includeDereg, "include-dereg", false, "Also ingest FAA DEREG (historical deregistrations, +100-200 MB)")
	cmd.Flags().BoolVar(&f.includePre1982, "include-pre1982", false, "Also ingest NTSB PRE1982.zip (older events, less detail; requires --source ntsb or all)")
	cmd.Flags().BoolVar(&f.fullNarratives, "full-narratives", false, "Store full NTSB narratives (zstd-compressed) instead of truncated summaries; requires --source ntsb or all")
	cmd.Flags().BoolVar(&f.includeAddresses, "include-addresses", false, "Also ingest FAA owner street/city/state/zip/country")
	cmd.Flags().BoolVar(&f.withFTS, "with-fts", false, "Build FTS5 full-text index over narratives and owner names")
	cmd.Flags().BoolVar(&f.everything, "everything", false, "Enable every optional tier (dereg, addresses, ntsb, pre1982, full-narratives, fts). Requires mdbtools.")
	cmd.Flags().StringVar(&f.source, "source", "faa", "Which source to sync: faa (default, zero deps) | ntsb (requires mdbtools) | all (both)")
	return cmd
}

func runSync(cmd *cobra.Command, f syncFlags) error {
	ctx := cmd.Context()
	if f.everything {
		f.includeDereg = true
		f.includePre1982 = true
		f.fullNarratives = true
		f.includeAddresses = true
		f.withFTS = true
		// --everything flips the source to all so NTSB is included.
		// The mdbtools pre-flight in SyncNTSB will fail fast with a
		// precise install hint if the user hasn't installed it.
		f.source = "all"
	}

	// PATCH: reject unknown --source values before opening the store.
	// Without this, a typo like `--source both` silently fell through both
	// switch blocks, ran VACUUM, and emitted `sync_done` — leaving the user
	// believing the store was fresh when nothing was actually ingested.
	switch f.source {
	case "faa", "ntsb", "all":
	default:
		return fmt.Errorf("invalid --source %q (want one of: faa, ntsb, all)", f.source)
	}

	dbPath := flagDBPath
	if dbPath == "" {
		dbPath = store.DefaultDBPath()
	}
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("opening store at %s: %w", dbPath, err)
	}
	defer st.Close()

	progress := newProgressEmitter(flagJSON)

	switch f.source {
	case "faa", "all":
		_, err := ingest.SyncFAA(ctx, st, ingest.FAAOptions{
			IncludeDereg:     f.includeDereg,
			IncludeAddresses: f.includeAddresses,
			Force:            f.full,
		}, progress)
		if err != nil {
			return fmt.Errorf("FAA sync: %w", err)
		}
	}

	switch f.source {
	case "ntsb", "all":
		_, err := ingest.SyncNTSB(ctx, st, ingest.NTSBOptions{
			IncludePre1982: f.includePre1982,
			FullNarratives: f.fullNarratives,
			Force:          f.full,
		}, nil, progress)
		if err != nil {
			return fmt.Errorf("NTSB sync: %w", err)
		}
	}

	if f.withFTS {
		progress("fts_build_start", nil)
		if err := st.BuildFTS(ctx, func(stage string) {
			progress("fts_stage", map[string]any{"stage": stage})
		}); err != nil {
			return fmt.Errorf("FTS build: %w", err)
		}
		progress("fts_build_done", nil)
	}

	// VACUUM lives outside the transaction; it can't run in one.
	progress("vacuum_start", nil)
	if _, err := st.DB().ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("wal_checkpoint: %w", err)
	}
	if _, err := st.DB().ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	progress("vacuum_done", nil)
	progress("sync_done", nil)
	return nil
}

// newProgressEmitter returns a ProgressFunc that writes NDJSON to stdout when
// json mode is on, or a friendly line otherwise.
func newProgressEmitter(jsonMode bool) ingest.ProgressFunc {
	return func(event string, payload map[string]any) {
		if jsonMode {
			rec := map[string]any{"event": event}
			for k, v := range payload {
				rec[k] = v
			}
			enc := json.NewEncoder(os.Stdout)
			_ = enc.Encode(rec)
			return
		}
		fmt.Printf("[%s]", event)
		for k, v := range payload {
			fmt.Printf(" %s=%v", k, v)
		}
		fmt.Println()
	}
}
