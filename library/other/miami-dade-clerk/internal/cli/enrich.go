// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// newEnrichCmd reads a CSV of folios, computes lien-chain +
// surviving-liens + chain-of-title for each in parallel, and writes a
// JSONL file of summary rows. Workers are capped at 4 to keep SQLite
// read contention bounded — modernc.org/sqlite serializes writes on
// writeMu and benefits from a small reader pool.
func newEnrichCmd(flags *rootFlags) *cobra.Command {
	var (
		folioListPath string
		outPath       string
		saleType      string
		workers       int
	)
	cmd := &cobra.Command{
		Use:         "enrich",
		Short:       "For a CSV of folios, compute lien-chain + surviving-liens + chain-of-title and emit a JSONL summary row per folio.",
		Long:        "For a CSV of folios, compute lien-chain + surviving-liens + chain-of-title and emit a JSONL summary row per folio. Up to 4 workers in parallel.",
		Example:     "  miami-dade-clerk-pp-cli enrich --folio-list folios.csv --out enriched.jsonl",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if folioListPath == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "folio-list")
			}
			if outPath == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "out")
			}
			if dryRunOK(flags) {
				return nil
			}
			if workers <= 0 || workers > 8 {
				workers = 4
			}
			folios, err := readFolioList(folioListPath)
			if err != nil {
				return fmt.Errorf("reading folio list: %w", err)
			}
			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}

			results := enrichParallel(cmd.Context(), s, folios, workers, saleType)

			outFile, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("create %s: %w", outPath, err)
			}
			defer outFile.Close()

			enc := json.NewEncoder(outFile)
			for _, r := range results {
				if err := enc.Encode(r); err != nil {
					return fmt.Errorf("encoding row: %w", err)
				}
			}

			// Also echo a summary to stdout (compact JSON) so non-file
			// consumers can see what happened.
			return flags.printJSON(cmd, map[string]any{
				"folios_processed": len(results),
				"out_path":         outPath,
			})
		},
	}
	cmd.Flags().StringVar(&folioListPath, "folio-list", "", "Path to CSV of folios (first column)")
	cmd.Flags().StringVar(&outPath, "out", "", "Output JSONL path")
	cmd.Flags().StringVar(&saleType, "assume-sale-type", "foreclosure", "Sale type for surviving-liens: foreclosure | taxdeed")
	cmd.Flags().IntVar(&workers, "workers", 4, "Parallel workers (1-8)")
	return cmd
}

// readFolioList parses a CSV file and returns the first column of every
// non-empty row, dashes stripped. Comment lines starting with '#' are
// ignored.
func readFolioList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var folios []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Use csv.NewReader on each line so quoted fields work without
		// loading the entire file. The first field is the folio.
		r := csv.NewReader(strings.NewReader(line))
		row, err := r.Read()
		if err != nil || len(row) == 0 {
			continue
		}
		folios = append(folios, strings.TrimSpace(row[0]))
	}
	return folios, scanner.Err()
}

// enrichParallel fans out folio enrichment across N workers and returns
// the result list in the order folios were submitted. Workers panic
// recovery is deliberately omitted — a corrupted DB or panic in
// downstream queries should bubble up and fail the entire enrich run
// rather than silently dropping rows.
func enrichParallel(ctx context.Context, s *store.Store, folios []string, workers int, saleType string) []map[string]any {
	results := make([]map[string]any, len(folios))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for i, folio := range folios {
		// Honor cancellation between submissions so a SIGINT mid-batch
		// stops queuing new work. Already-launched goroutines still
		// drain to completion (one folio query is O(ms) — bounded).
		if ctx.Err() != nil {
			results[i] = map[string]any{"folio": folio, "error": ctx.Err().Error()}
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			results[i] = map[string]any{"folio": folio, "error": ctx.Err().Error()}
			continue
		}
		wg.Add(1)
		go func(i int, folio string) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				results[i] = map[string]any{"folio": folio, "error": ctx.Err().Error()}
				return
			}
			results[i] = enrichOneFolio(s, folio, saleType)
		}(i, folio)
	}
	wg.Wait()
	return results
}

// enrichOneFolio computes the summary row for a single folio. Returns
// a row with error=<msg> when the folio is malformed so the output JSONL
// is line-aligned with the input CSV (caller can grep for "error":).
func enrichOneFolio(s *store.Store, folio, saleType string) map[string]any {
	folioN := NormalizeFolio(folio)
	if folioN == 0 {
		return map[string]any{"folio": folio, "error": "invalid folio"}
	}
	all, err := s.QueryRecordings(store.RecordingFilter{FolioNumber: folioN})
	if err != nil {
		return map[string]any{"folio": folio, "error": err.Error()}
	}

	deeds := filterByDocType(all, deedDocTypes)
	mortgages := filterByDocType(all, []string{"MOR"})
	ftls := filterByDocType(all, []string{"FTL"})

	releases := map[string][]*store.Recording{}
	for _, r := range all {
		releases[r.DocTypeCode] = append(releases[r.DocTypeCode], r)
	}

	usedReleases := map[int64]bool{}
	var surviving []*store.Recording
	var totalCents int64
	for _, r := range all {
		rule, ok := flSurvivability[r.DocTypeCode]
		if !ok {
			continue
		}
		if matchedReleaseID := findMatchingRelease(r, releases, usedReleases); matchedReleaseID != 0 {
			usedReleases[matchedReleaseID] = true
			continue
		}
		survives := rule.SurvivesForeclosure
		if saleType == "taxdeed" {
			survives = rule.SurvivesTaxDeed
		}
		if !survives {
			continue
		}
		surviving = append(surviving, r)
		totalCents += r.ConsiderationCents
	}

	var oldestDeed, lastDeed, currentOwner string
	if len(deeds) > 0 {
		oldestDeed = deeds[0].RecordingDate
		lastDeed = deeds[len(deeds)-1].RecordingDate
		currentOwner = deeds[len(deeds)-1].SecondParty
	}

	return map[string]any{
		"folio":                folio,
		"totals_cents":         totalCents,
		"surviving_lien_count": len(surviving),
		"ftl_count":            len(ftls),
		"oldest_deed_date":     oldestDeed,
		"last_deed_date":       lastDeed,
		"current_owner":        currentOwner,
		"deed_count":           len(deeds),
		"mortgage_count":       len(mortgages),
	}
}

// filterByDocType is a small helper kept local to enrich.go because it
// would otherwise pollute the package namespace. Linear scan is fine —
// recordings on a single folio rarely exceed a few hundred.
func filterByDocType(in []*store.Recording, codes []string) []*store.Recording {
	out := make([]*store.Recording, 0, len(in))
	for _, r := range in {
		for _, c := range codes {
			if r.DocTypeCode == c {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
