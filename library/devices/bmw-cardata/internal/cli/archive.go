// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local

// newArchiveCmd groups archive import commands.
func newArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Import and inspect BMW CarData Customer Archive (.zip) bundles",
	}
	cmd.AddCommand(newArchiveReadCmd(flags))
	return cmd
}

func newArchiveReadCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB        string
		flagVIN       string
		flagInspect   bool
	)
	cmd := &cobra.Command{
		Use:   "read <zip>",
		Short: "Import a Customer Archive .zip into the local store (telematic + charging history)",
		Long: `Parse a BMW CarData Customer Archive downloaded from the portal and import its
recognizable data (telematic descriptor snapshots and charging sessions) into the local
SQLite store so transcendence commands can reason over historical data.

The archive is per-vehicle, so --vin is required.`,
		Example:     "  bmw-cardata-pp-cli archive read ~/Downloads/cardata-archive.zip --vin WBAJB3105JUV12345",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would import a Customer Archive .zip into the local store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("path to a Customer Archive .zip is required"))
			}
			zipPath := args[0]
			if _, err := os.Stat(zipPath); err != nil {
				return usageErr(fmt.Errorf("archive not found: %w", err))
			}
			vin := flagVIN
			if vin == "" {
				vin = os.Getenv("BMW_CARDATA_VIN")
			}
			if vin == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--vin is required (the archive is per-vehicle)"))
			}

			r, err := zip.OpenReader(zipPath)
			if err != nil {
				return configErr(fmt.Errorf("opening archive: %w", err))
			}
			defer r.Close()

			dbPath := resolveDBPath(flagDB)
			var files, telematicFiles, chargingFiles, descriptors, sessions int
			var fileSummary []map[string]any

			for _, f := range r.File {
				if f.FileInfo().IsDir() {
					continue
				}
				files++
				summary := map[string]any{"name": f.Name, "size": f.UncompressedSize64}
				content, err := readZipFile(f)
				if err != nil {
					summary["error"] = err.Error()
					fileSummary = append(fileSummary, summary)
					continue
				}
				if strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
					summary["kind"] = "xml-index"
					fileSummary = append(fileSummary, summary)
					continue
				}
				trimmed := bytes.TrimSpace(content)
				if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
					summary["kind"] = "binary/other"
					fileSummary = append(fileSummary, summary)
					continue
				}

				kind, count := classifyAndImport(dbPath, vin, trimmed, flagInspect)
				summary["kind"] = kind
				switch kind {
				case "telematic":
					telematicFiles++
					descriptors += count
				case "charging":
					chargingFiles++
					sessions += count
				}
				fileSummary = append(fileSummary, summary)
			}

			view := map[string]any{
				"vin":             vin,
				"archive":         filepath.Base(zipPath),
				"files":           files,
				"telematic_files": telematicFiles,
				"charging_files":  chargingFiles,
				"descriptors":     descriptors,
				"sessions":        sessions,
				"inspect_only":    flagInspect,
			}
			if !flagInspect {
				view["db"] = dbPath
			}
			if files > 0 && fileSummary != nil {
				view["file_summary"] = fileSummary
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported archive %s for %s\n", filepath.Base(zipPath), vin)
			fmt.Fprintf(cmd.OutOrStdout(), "  files:           %d\n", files)
			fmt.Fprintf(cmd.OutOrStdout(), "  telematic files: %d (%d descriptors)\n", telematicFiles, descriptors)
			fmt.Fprintf(cmd.OutOrStdout(), "  charging files:  %d (%d sessions)\n", chargingFiles, sessions)
			if flagInspect {
				fmt.Fprintln(cmd.OutOrStdout(), "  (inspect-only; nothing written)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagVIN, "vin", "", "Vehicle VIN (required; the archive is per-vehicle)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().BoolVar(&flagInspect, "inspect", false, "List archive contents without importing")
	return cmd
}

// classifyAndImport recognizes telematic-data and charging-session JSON
// shapes and imports them (unless inspectOnly). Returns (kind, count).
func classifyAndImport(dbPath, vin string, raw []byte, inspectOnly bool) (string, int) {
	var tele struct {
		TelematicData map[string]json.RawMessage `json:"telematicData"`
	}
	if json.Unmarshal(raw, &tele) == nil && len(tele.TelematicData) > 0 {
		if !inspectOnly {
			persistCardataTelematicData(dbPath, vin, raw)
		}
		return "telematic", len(tele.TelematicData)
	}
	// Charging: single session or array of sessions.
	sessions := collectChargingSessions(raw)
	if len(sessions) > 0 {
		if !inspectOnly {
			wrapped, _ := json.Marshal(map[string]any{"data": sessions})
			persistCardataChargingHistory(dbPath, vin, wrapped)
		}
		return "charging", len(sessions)
	}
	return "other", 0
}

// collectChargingSessions returns charging-session objects found in raw: a
// single session object or every element of an array that has startTime.
func collectChargingSessions(raw []byte) []json.RawMessage {
	if objHasAny(raw, "startTime", "energyConsumedFromPowerGridKwh", "totalChargingDurationSec") {
		return []json.RawMessage{raw}
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		out := arr[:0]
		for _, e := range arr {
			if objHasAny(e, "startTime") {
				out = append(out, e)
			}
		}
		return out
	}
	return nil
}

func objHasAny(raw []byte, keys ...string) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
