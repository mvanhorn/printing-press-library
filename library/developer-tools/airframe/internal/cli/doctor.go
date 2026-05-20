// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"

	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report DB freshness, schema version, mdbtools presence, flight-goat installation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context())
		},
	}
}

type doctorReport struct {
	DBPath           string              `json:"db_path"`
	DBExists         bool                `json:"db_exists"`
	DBSizeBytes      int64               `json:"db_size_bytes,omitempty"`
	SchemaVersion    int                 `json:"schema_version,omitempty"`
	SupportedVersion int                 `json:"supported_version"`
	SyncMeta         []store.SyncMetaRow `json:"sync_meta,omitempty"`
	MDBTools         tool                `json:"mdbtools"`
	FlightGoat       tool                `json:"flight_goat"`
}

type tool struct {
	Detected bool   `json:"detected"`
	Path     string `json:"path,omitempty"`
}

func runDoctor(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	dbPath := flagDBPath
	if dbPath == "" {
		dbPath = store.DefaultDBPath()
	}

	r := doctorReport{
		DBPath:           dbPath,
		SupportedVersion: store.StoreSchemaVersion,
		MDBTools:         detectTool("mdb-export"),
		FlightGoat:       detectTool("flight-goat-pp-cli"),
	}

	if fi, err := os.Stat(dbPath); err == nil {
		r.DBExists = true
		r.DBSizeBytes = fi.Size()

		s, err := store.OpenReadOnly(dbPath)
		if err != nil {
			return fmt.Errorf("opening store: %w", err)
		}
		defer s.Close()
		v, err := s.SchemaVersion(ctx)
		if err != nil {
			return fmt.Errorf("reading schema version: %w", err)
		}
		r.SchemaVersion = v
		rows, err := s.ListSyncMeta(ctx)
		if err != nil {
			return fmt.Errorf("listing sync_meta: %w", err)
		}
		r.SyncMeta = rows
	}

	if flagJSON {
		return printJSON(r)
	}
	return renderDoctorText(r)
}

func detectTool(name string) tool {
	p, err := exec.LookPath(name)
	if err != nil {
		return tool{Detected: false}
	}
	return tool{Detected: true, Path: p}
}

func renderDoctorText(r doctorReport) error {
	fmt.Printf("airframe-pp-cli doctor\n\n")
	fmt.Printf("Database\n")
	fmt.Printf("  path:              %s\n", r.DBPath)
	if !r.DBExists {
		fmt.Printf("  exists:            no  — run `airframe-pp-cli sync` to create and populate\n")
	} else {
		fmt.Printf("  exists:            yes (%s)\n", humanBytes(r.DBSizeBytes))
		fmt.Printf("  schema_version:    %d (binary supports %d)\n", r.SchemaVersion, r.SupportedVersion)
		if len(r.SyncMeta) == 0 {
			fmt.Printf("  sync_meta:         empty — never synced\n")
		} else {
			fmt.Printf("  sync_meta:\n")
			for _, m := range r.SyncMeta {
				fresh := freshnessLabel(m)
				fmt.Printf("    %-14s rows=%-8d last_synced=%s  %s\n", m.Source, m.RowCount, displayTime(m.LastSyncedAt), fresh)
			}
		}
	}

	fmt.Printf("\nExternal tools\n")
	fmt.Printf("  mdb-export:        %s\n", toolLine(r.MDBTools, "needed for NTSB ingest — install with `brew install mdbtools` or `apt install mdbtools`"))
	fmt.Printf("  flight-goat-pp-cli: %s\n", toolLine(r.FlightGoat, "optional — enables `airframe-pp-cli flight <ident>`"))
	return nil
}

func toolLine(t tool, hint string) string {
	if t.Detected {
		return "found (" + t.Path + ")"
	}
	return "not found  — " + hint
}

func freshnessLabel(m store.SyncMetaRow) string {
	if m.LastSyncedAt == "" {
		return "(never)"
	}
	ts, err := time.Parse(time.RFC3339, m.LastSyncedAt)
	if err != nil {
		return ""
	}
	age := time.Since(ts)
	threshold := 30 * 24 * time.Hour
	if m.Source == "faa_master" {
		threshold = 7 * 24 * time.Hour
	}
	if age > threshold {
		return fmt.Sprintf("STALE (%.0f days old)", age.Hours()/24)
	}
	return "fresh"
}

func displayTime(s string) string {
	if s == "" {
		return "never"
	}
	return s
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
