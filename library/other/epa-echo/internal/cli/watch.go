// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/epa-echo/internal/store"
	"github.com/spf13/cobra"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var portfolio string

	cmd := &cobra.Command{
		Use:         "watch --portfolio FILE",
		Short:       "Fetch a bounded CSV portfolio of facility IDs and atomically report changed report sections since the prior run.",
		Example:     "  epa-echo-pp-cli watch --portfolio examples/facilities.csv --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ids, err := readPortfolioIDs(portfolio)
			if err != nil {
				return err
			}
			if len(ids) > 50 {
				return fmt.Errorf("portfolio contains %d facilities; maximum is 50", len(ids))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			key, err := filepath.Abs(portfolio)
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("epa-echo-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			previous := map[string]map[string]any{}
			raw, getErr := db.Get("echo-portfolio-snapshot", key)
			baseline := errors.Is(getErr, sql.ErrNoRows)
			if getErr != nil && !baseline {
				return getErr
			}
			if getErr == nil {
				if err := json.Unmarshal(raw, &previous); err != nil {
					return err
				}
			}
			current := map[string]map[string]any{}
			rows := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				report, fetchErr := getDFR(ctx, flags, id)
				if fetchErr != nil {
					return fetchErr
				}
				current[id] = report
				changes := []map[string]any{}
				_, facilityHadBaseline := previous[id]
				if !baseline && facilityHadBaseline {
					changes = changedSections(previous[id], report)
				}
				rows = append(rows, map[string]any{"facility_id": id, "baseline_created": baseline || !facilityHadBaseline, "changed_section_count": len(changes), "changed_sections": changes})
			}
			next, _ := json.Marshal(current)
			if err := db.Upsert("echo-portfolio-snapshot", key, next); err != nil {
				return err
			}
			return emitECHO(cmd, flags, "mixed", map[string]any{"portfolio": portfolio, "baseline_created": baseline, "facilities": rows, "note": "The portfolio snapshot advances atomically only after every bounded live report succeeds.", "caveats": echoCaveats()})
		},
	}
	cmd.Flags().StringVar(&portfolio, "portfolio", "", "CSV file containing a registry_id or facility_id column")
	return cmd
}

func readPortfolioIDs(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("--portfolio is required")
	}
	// #nosec G304 -- the portfolio path is explicit operator input to this local-file command.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	index := -1
	for i, name := range header {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if normalized == "registry_id" || normalized == "facility_id" || normalized == "frs_id" {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, errors.New("portfolio CSV needs registry_id, facility_id, or frs_id column")
	}
	var ids []string
	seen := map[string]bool{}
	for {
		row, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if index >= len(row) {
			return nil, fmt.Errorf("portfolio CSV row has %d columns; facility ID is column %d", len(row), index+1)
		}
		id := strings.TrimSpace(row[index])
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}
