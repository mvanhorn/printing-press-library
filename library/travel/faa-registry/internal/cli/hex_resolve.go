// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

// hexResolution is one resolved Mode S hex code.
// No omitempty: batches mix registry hits with computed/invalid rows, and
// sparse keys would be pruned by --compact's fields-in-80%-of-rows rule —
// dropping owner/model exactly when a mixed ADS-B log is resolved.
type hexResolution struct {
	Hex     string `json:"hex"`
	NNumber string `json:"n_number"`
	Source  string `json:"source"` // "registry", "computed", or "invalid"
	Owner   string `json:"owner"`
	Model   string `json:"model"`
	Mfr     string `json:"manufacturer"`
	Note    string `json:"note"`
}

func stdinIsTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return true
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

func newNovelHexResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [hex...]",
		Short: "Resolve ADS-B Mode S hex codes to N-numbers, aircraft, and owners — offline",
		Long: `Resolve Mode S / ICAO 24-bit hex addresses to registrations. Codes come from
arguments or stdin (one per line, so receiver logs pipe straight in). Each hit
joins the local registry for owner and aircraft type; codes not in the local
database fall back to the pure FAA numbering algorithm (source: "computed").
Run sync first for registry-backed results.`,
		Example:     "  faa-registry-pp-cli hex resolve A008C5\n  cat hexes.txt | faa-registry-pp-cli hex resolve --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			codes := append([]string{}, args...)
			if len(codes) == 0 {
				if stdinIsTerminal(cmd.InOrStdin()) {
					return cmd.Help()
				}
				sc := bufio.NewScanner(cmd.InOrStdin())
				for sc.Scan() {
					line := strings.TrimSpace(sc.Text())
					if line != "" {
						codes = append(codes, line)
					}
				}
				if err := sc.Err(); err != nil {
					return err
				}
			}
			if len(codes) == 0 {
				return cmd.Help()
			}

			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			synced, err := db.Synced(cmd.Context())
			if err != nil {
				return err
			}
			emitRegistryStaleHint(cmd, db, flags)

			results := make([]hexResolution, 0, len(codes))
			for _, code := range codes {
				norm := strings.ToUpper(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(code)), "0x"))
				r := hexResolution{Hex: norm}
				if synced {
					ac, err := db.LookupHex(cmd.Context(), norm)
					if err != nil {
						return err
					}
					if ac != nil {
						r.NNumber = ac.NNumber
						r.Source = "registry"
						r.Owner = ac.OwnerName
						r.Model = ac.Model
						r.Mfr = ac.Manufacturer
						results = append(results, r)
						continue
					}
				}
				tail, err := registrydb.IcaoToTail(norm)
				if err != nil {
					r.Source = "invalid"
					r.Note = err.Error()
				} else {
					r.NNumber = tail
					r.Source = "computed"
					if !synced {
						r.Note = "local registry not synced — run sync for owner/model data"
					} else {
						r.Note = "not in the active registry; N-number derived from the FAA numbering algorithm"
					}
				}
				results = append(results, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	return cmd
}
