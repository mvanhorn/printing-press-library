package cli

// PATCH: Hand-built local catalog export command.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var format, kind, output string
	cmd := &cobra.Command{
		Use:         "export [--format json|csv|sqlite] [--kind bluray|4k|dvd|...] [--output PATH]",
		Short:       "Export the local Blu-ray.com catalog to a file or stdout.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  blu-ray-pp-cli export --format json
  blu-ray-pp-cli export --format csv --kind 4k --output 4k-releases.csv
  blu-ray-pp-cli export --format sqlite --output catalog-snapshot.db
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch format {
			case "json", "csv", "sqlite":
			default:
				return usageErr(fmt.Errorf("--format must be json, csv, or sqlite"))
			}
			if format == "sqlite" && output == "" {
				return usageErr(fmt.Errorf("--output is required for sqlite export"))
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				return writeExportRows(cmd, flags, format, output, nil)
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			rows, err := s.ListCatalog(cmd.Context(), kind, 0)
			if err != nil {
				return err
			}
			if format == "sqlite" {
				return exportCatalogSQLite(cmd, output, rows)
			}
			return writeExportRows(cmd, flags, format, output, rows)
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Export format: json, csv, or sqlite.")
	cmd.Flags().StringVar(&kind, "kind", "", "Catalog kind filter: bluray, 4k, dvd, digital, itunes, ma, or uv.")
	cmd.Flags().StringVar(&output, "output", "", "Write export to PATH instead of stdout.")
	return cmd
}

func writeExportRows(cmd *cobra.Command, flags *rootFlags, format, output string, rows []store.CatalogRow) error {
	if rows == nil {
		rows = []store.CatalogRow{}
	}
	if format == "sqlite" {
		return nil
	}
	if format == "json" && output == "" {
		return flags.printJSON(cmd, rows)
	}
	var f *os.File
	var err error
	if output == "" {
		f = nil
	} else {
		// #nosec G304 -- output is the operator-supplied --output path for an
		// export-to-file feature; writing to a caller-chosen path is the intended
		// behavior, and the user already controls their own filesystem.
		f, err = os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	w := cmd.OutOrStdout()
	if f != nil {
		w = f
	}
	if format == "json" {
		return json.NewEncoder(w).Encode(rows)
	}
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "kind", "slug", "title_normalized", "country", "year_hint", "lastmod"}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write([]string{strconv.Itoa(r.ID), r.Kind, r.Slug, r.TitleNormalized, r.Country, strconv.Itoa(r.YearHint), r.Lastmod}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func exportCatalogSQLite(cmd *cobra.Command, output string, rows []store.CatalogRow) error {
	if err := os.Remove(output); err != nil && !os.IsNotExist(err) {
		return err
	}
	target, err := store.OpenWithContext(cmd.Context(), output)
	if err != nil {
		return err
	}
	defer target.Close()
	if err := target.MigrateBluRayCatalog(); err != nil {
		return err
	}
	return target.UpsertCatalogRows(cmd.Context(), rows)
}
