// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

// hexConversion is one algorithmic hex↔tail conversion.
// No omitempty: mixed valid/invalid batches must keep all keys under --compact.
type hexConversion struct {
	Input   string `json:"input"`
	Hex     string `json:"hex"`
	NNumber string `json:"n_number"`
	Error   string `json:"error"`
}

func newHexToTailCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "to-tail [hex...]",
		Short: "Convert ICAO 24-bit hex addresses to N-numbers (pure algorithm, no database needed)",
		Long: `Convert US ICAO 24-bit Mode S hex addresses (block A00001-ADF7C7) to their
N-numbers using the FAA's sequential numbering algorithm. Works with no local
database and no network. For owner and aircraft data, use "hex resolve".`,
		Example:     "  faa-registry-pp-cli hex to-tail A008C5\n  faa-registry-pp-cli hex to-tail A00001 ADF7C7 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			out := make([]hexConversion, 0, len(args))
			for _, a := range args {
				c := hexConversion{Input: a}
				tail, err := registrydb.IcaoToTail(a)
				if err != nil {
					c.Error = err.Error()
				} else {
					c.NNumber = tail
				}
				out = append(out, c)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newHexFromTailCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "from-tail [n-number...]",
		Short: "Convert N-numbers to ICAO 24-bit hex addresses (pure algorithm, no database needed)",
		Long: `Convert US N-numbers to their ICAO 24-bit Mode S hex addresses using the
FAA's sequential numbering algorithm. Works with no local database and no
network.`,
		Example:     "  faa-registry-pp-cli hex from-tail N101DQ\n  faa-registry-pp-cli hex from-tail N1 N12345 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			out := make([]hexConversion, 0, len(args))
			for _, a := range args {
				c := hexConversion{Input: a}
				hex, err := registrydb.TailToIcao(a)
				if err != nil {
					c.Error = err.Error()
				} else {
					c.Hex = hex
					c.NNumber = "N" + registrydb.NormalizeTail(a)
				}
				out = append(out, c)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
