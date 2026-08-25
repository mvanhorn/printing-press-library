// Copyright 2026 Keith Herrington and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored "assume a car" feature for the Tessie CLI.
// `tessie use <name-or-vin>` persists an assumed vehicle (VIN + display name)
// to a small JSON store beside the config file (mode 600, no credentials).
// Vehicle subcommands fall back to the assumed VIN when no VIN positional is
// given; `tessie current` prints it.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tessie/internal/cliutil"
)

// assumeFileName sits next to the config file holding the assumed car.
const assumeFileName = "assumed.json"

// assumeStore is the persisted assumed-car record. VIN is authoritative; the
// display name is stored for human output only.
type assumeStore struct {
	AssumedVIN  string `json:"assumed_vin"`
	AssumedName string `json:"assumed_name"`
}

// configDir returns the directory containing the resolved config file.
// It follows the same --config / TESSIE_CONFIG / --home / TESSIE_HOME
// resolution as config.Load so assumed.json stays next to the live config.
func (f *rootFlags) configDir() string {
	if strings.TrimSpace(f.configPath) != "" {
		return filepath.Dir(f.configPath)
	}
	if path := os.Getenv("TESSIE_CONFIG"); path != "" {
		return filepath.Dir(path)
	}
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return filepath.Dir(defaultConfigPath())
	}
	return dir
}

// defaultConfigPath mirrors the generator's default config location.
func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "tessie-pp-cli", "config.toml")
}

// assumePath returns the assume-store path next to the config file.
func (f *rootFlags) assumePath() string {
	return filepath.Join(f.configDir(), assumeFileName)
}

// loadAssumed reads the assumed-car record. A missing or empty store yields an
// empty record with no error.
func (f *rootFlags) loadAssumed() (assumeStore, error) {
	var s assumeStore
	b, err := os.ReadFile(f.assumePath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("reading assume store: %w", err)
	}
	return s, nil
}

// saveAssumed atomically writes the assume store with 600 perms.
func (f *rootFlags) saveAssumed(s assumeStore) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return cliutil.AtomicWritePrivateFile(f.assumePath(), data, 0o600, 0o700)
}

// vehicleRow is a normalized vehicle row used for selection output.
type vehicleRow struct {
	DisplayName string `json:"display_name"`
	VIN         string `json:"vin"`
	State       string `json:"state"`
	Plate       string `json:"plate,omitempty"`
}

// fetchVehicleRows lists vehicles via the live API and normalizes rows.
func fetchVehicleRows(cmd *cobra.Command, flags *rootFlags) ([]vehicleRow, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{}
	data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "auto", "vehicles", true, "/vehicles", params, nil, "", cmd.ErrOrStderr())
	if err != nil {
		return nil, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	// The live response is wrapped as {"results":[...]}; tolerate a bare array
	// too (the strategy layer may return the envelope or the parsed array).
	var list []json.RawMessage
	if json.Unmarshal(data, &list) != nil {
		var wrap struct {
			Results []json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(data, &wrap); err != nil {
			return nil, fmt.Errorf("parsing vehicle list: %w", err)
		}
		list = wrap.Results
	}
	rows := make([]vehicleRow, 0, len(list))
	for _, raw := range list {
		var r struct {
			VIN   string `json:"vin"`
			Plate string `json:"plate"`
			Last  struct {
				DisplayName string `json:"display_name"`
				State       string `json:"state"`
			} `json:"last_state"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			continue // skip malformed rows rather than failing the whole list
		}
		rows = append(rows, vehicleRow{DisplayName: r.Last.DisplayName, VIN: r.VIN, State: r.Last.State, Plate: r.Plate})
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].DisplayName) < strings.ToLower(rows[j].DisplayName)
	})
	return rows, nil
}

// matchVehicles finds vehicles matching a selector: exact display name, exact
// VIN, or VIN suffix preferred; display-name substring only as a fallback.
func matchVehicles(rows []vehicleRow, selector string) []vehicleRow {
	sel := strings.TrimSpace(selector)
	if sel == "" {
		return nil
	}
	lower := strings.ToLower(sel)
	var exact []vehicleRow
	for _, r := range rows {
		if strings.EqualFold(sel, r.DisplayName) || (r.VIN != "" && strings.EqualFold(sel, r.VIN)) || (r.VIN != "" && strings.HasSuffix(strings.ToLower(r.VIN), lower)) {
			exact = append(exact, r)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	var out []vehicleRow
	for _, r := range rows {
		if r.DisplayName != "" && strings.Contains(strings.ToLower(r.DisplayName), lower) {
			out = append(out, r)
		}
	}
	return out
}

// displayNameOr falls back from display name to plate to a masked VIN.
func displayNameOr(r vehicleRow) string {
	if strings.TrimSpace(r.DisplayName) != "" {
		return r.DisplayName
	}
	if strings.TrimSpace(r.Plate) != "" {
		return r.Plate
	}
	return maskVIN(r.VIN)
}

// maskVIN hides everything but the last four VIN characters.
func maskVIN(vin string) string {
	if len(vin) <= 6 {
		return "***"
	}
	return "***" + vin[len(vin)-4:]
}

// isFullVIN reports whether s looks like a complete 17-character VIN
// (alphanumeric, excluding I/O/Q per ISO 3779).
func isFullVIN(s string) bool {
	if len(s) != 17 {
		return false
	}
	for _, r := range s {
		okDigit := r >= '0' && r <= '9'
		okUpper := r >= 'A' && r <= 'Z' && r != 'I' && r != 'O' && r != 'Q'
		okLower := r >= 'a' && r <= 'z' && r != 'i' && r != 'o' && r != 'q'
		if !okDigit && !okUpper && !okLower {
			return false
		}
	}
	return true
}

// newUseCmd: `use <name-or-vin>` persists the assumed vehicle.
func newUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name-or-vin>",
		Short: "Set the assumed vehicle (persisted)",
		Long: `Set the "assumed" vehicle that vehicle subcommands target when no VIN positional
is given. Matches by display name, VIN, or VIN suffix.`,
		Example:     "  tessie-pp-cli use Car",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"pp:happy-args": "name-or-vin=VCK5YJ3E1EA000001", "mcp:hidden": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := strings.TrimSpace(args[0])
			// A full 17-character VIN needs no live lookup: persist it directly
			// so `use <VIN>` works offline and under the verify mock.
			if isFullVIN(sel) {
				if err := flags.saveAssumed(assumeStore{AssumedVIN: strings.ToUpper(sel), AssumedName: sel}); err != nil {
					return err
				}
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"assumed_vin": strings.ToUpper(sel)}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Now assuming VIN %s\n", maskVIN(sel))
				return nil
			}
			rows, err := fetchVehicleRows(cmd, flags)
			if err != nil {
				return err
			}
			matches := matchVehicles(rows, args[0])
			if len(matches) == 0 {
				return usageErr(fmt.Errorf("no vehicle matched %q", args[0]))
			}
			if len(matches) > 1 {
				names := make([]string, len(matches))
				for i, m := range matches {
					names[i] = fmt.Sprintf("%s (%s)", displayNameOr(m), maskVIN(m.VIN))
				}
				return usageErr(fmt.Errorf("ambiguous match %q: %s (use a VIN suffix or full display name)", args[0], strings.Join(names, ", ")))
			}
			m := matches[0]
			if err := flags.saveAssumed(assumeStore{AssumedVIN: m.VIN, AssumedName: m.DisplayName}); err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"assumed_vin":  m.VIN,
					"assumed_name": m.DisplayName,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Now assuming %s (%s)\n", displayNameOr(m), maskVIN(m.VIN))
			return nil
		},
	}
}

// newCurrentCmd: show the assumed vehicle.
func newCurrentCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "current",
		Short:   "Show the assumed vehicle",
		Example: "  tessie-pp-cli current",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := flags.loadAssumed()
			if err != nil {
				return err
			}
			if strings.TrimSpace(s.AssumedVIN) == "" {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"assumed": false}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No assumed vehicle (use: tessie use <name-or-vin>)")
				return nil
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"assumed":      true,
					"vin":          s.AssumedVIN,
					"assumed_name": s.AssumedName,
				}, flags)
			}
			name := strings.TrimSpace(s.AssumedName)
			if name == "" {
				name = maskVIN(s.AssumedVIN)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Assuming %s (%s)\n", name, maskVIN(s.AssumedVIN))
			return nil
		},
	}
}

// resolveVehicleArg resolves the VIN for a vehicle subcommand: a provided
// positional wins, otherwise the assumed car, else a usage error.
func (f *rootFlags) resolveVehicleArg(args []string) (string, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0]), nil
	}
	s, err := f.loadAssumed()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(s.AssumedVIN) != "" {
		return s.AssumedVIN, nil
	}
	return "", usageErr(fmt.Errorf("no VIN given and no assumed vehicle\nhint: tessie use <name-or-vin>  or pass a VIN positional"))
}
