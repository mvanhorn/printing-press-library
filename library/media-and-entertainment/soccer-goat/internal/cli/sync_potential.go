// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command (not generated). `sync potential` loads the
// bundled FC potential dataset into the local store so player/team/potential-gap/
// wonderkids can show real potential offline. Registered from root.go.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/data"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/store"
)

// openPotentialStore opens the local store for potential lookups. Best-effort:
// on failure it returns (nil, no-op) so the aggregator falls back to the live
// path rather than failing the whole report. Callers must defer the cleanup and
// only call WithPotentialStore when the returned store is non-nil.
func openPotentialStore(cmd *cobra.Command) (*store.Store, func()) {
	s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("soccer-goat-pp-cli"))
	if err != nil {
		return nil, func() {}
	}
	return s, func() { _ = s.Close() }
}

func newSyncPotentialCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "potential",
		Short: "Load the bundled FIFA/FC potential dataset into the local store",
		Long: `Populates the local 'potential' table from the dataset bundled in the
binary (sofifa-derived, June 2025 snapshot, keyed by EA player id). After this runs, player,
team, potential-gap, and wonderkids show real potential ratings offline — no
network and no Cloudflare. Idempotent: safe to run repeatedly.`,
		Example:     "  soccer-goat-pp-cli sync potential",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would load the bundled potential dataset into the local store")
				return nil
			}
			rows, err := loadBundledPotential()
			if err != nil {
				return fmt.Errorf("read bundled potential dataset: %w", err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("soccer-goat-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			n, err := s.UpsertPotentialBatch(cmd.Context(), rows)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "loaded %d players with potential into the local store (source: dataset:sofifa-2025, June 2025 sofifa snapshot)\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// loadBundledPotential decompresses and parses the embedded dataset into rows.
// name_normalized is intentionally left empty so the store computes it with the
// same Go fold the lookup uses (see store.NormalizePotentialName).
func loadBundledPotential() ([]store.PotentialRow, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data.PotentialSofifa2025GZ))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	r := csv.NewReader(gz)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	for _, need := range []string{"ea_id", "name", "overall", "potential"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("dataset missing column %q", need)
		}
	}
	rows := make([]store.PotentialRow, 0, 18000)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		eaID, _ := strconv.Atoi(rec[col["ea_id"]])
		overall, _ := strconv.Atoi(rec[col["overall"]])
		pot, _ := strconv.Atoi(rec[col["potential"]])
		if eaID <= 0 || pot <= 0 {
			continue
		}
		rows = append(rows, store.PotentialRow{
			EAID:      eaID,
			Name:      rec[col["name"]],
			Overall:   overall,
			Potential: pot,
			Source:    "dataset:sofifa-2025",
		})
	}
	return rows, nil
}
