// Copyright 2026 matysanchez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newDoctorCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check system requirements and Photos library access",
		Long: `Run pre-flight checks before using any other command.

Verifies: macOS, Photos.app installation, library path, read access,
database schema, and asset count.`,
		Example: `  icloud-pp-cli doctor
  icloud-pp-cli doctor --library "/Volumes/External/Photos Library.photoslibrary/database/Photos.sqlite"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			allOK := true

			check := func(label, hint string, ok bool) {
				allOK = allOK && ok
				if ok {
					fmt.Fprintf(out, "  %s %s\n", green(f, "✓"), label)
				} else {
					fmt.Fprintf(out, "  %s %s\n", red(f, "✗"), label)
					if hint != "" {
						fmt.Fprintf(out, "      %s\n", hint)
					}
				}
			}

			// ── System ────────────────────────────────────────────────
			fmt.Fprintln(out, bold(f, "System"))

			isDarwin := runtime.GOOS == "darwin"
			check("macOS required", "icloud-pp-cli only runs on macOS.", isDarwin)
			if !isDarwin {
				fmt.Fprintln(out)
				fmt.Fprintln(out, red(f, "Stopped — macOS required."))
				return nil
			}

			ver := macOSVersion()
			fmt.Fprintf(out, "      macOS %s\n", ver)

			photosPath := photosAppPath()
			check("Photos.app installed",
				"Photos.app not found in /System/Applications or /Applications.", photosPath != "")

			// ── Library ───────────────────────────────────────────────
			fmt.Fprintln(out)
			fmt.Fprintln(out, bold(f, "Library"))

			libPath := f.libraryPath
			if libPath == "" {
				libPath = defaultLibraryPath()
			}

			_, statErr := os.Stat(libPath)
			check("Library found", fmt.Sprintf(
				"Not found at:\n      %s\n      Use --library to specify a custom path.", libPath,
			), statErr == nil)

			if statErr != nil {
				fmt.Fprintln(out)
				fmt.Fprintln(out, red(f, "Stopped — library not found."))
				return nil
			}
			fmt.Fprintf(out, "      %s\n", libPath)

			db, openErr := openPhotosDB(libPath)
			check("Library readable (read-only)",
				"Try quitting Photos.app and running again.", openErr == nil)

			if openErr != nil {
				fmt.Fprintln(out)
				fmt.Fprintln(out, red(f, "Stopped — cannot read library."))
				return nil
			}
			defer db.Close()

			schemaOK := checkSchema(db)
			check("Schema valid (ZASSET + ZADDITIONALASSETATTRIBUTES)",
				"Unexpected schema — may be an unsupported Photos version.", schemaOK)

			// ── Assets ────────────────────────────────────────────────
			fmt.Fprintln(out)
			fmt.Fprintln(out, bold(f, "Assets"))

			count, sizeBytes, countErr := queryTotals(db)
			check("Can query assets", "Unexpected query error.", countErr == nil)

			if countErr == nil {
				gb := float64(sizeBytes) / (1 << 30)
				fmt.Fprintf(out, "      %s items · %.2f GB (original sizes)\n",
					formatInt(count), gb)

				byType, _ := queryStorageByType(db)
				for _, r := range byType {
					fmt.Fprintf(out, "      %-12s %s items\n",
						r.Label+":", formatInt(r.Count))
				}
			}

			// ── Result ────────────────────────────────────────────────
			fmt.Fprintln(out)
			if allOK {
				fmt.Fprintln(out, green(f, "All checks passed. Ready to use."))
			} else {
				fmt.Fprintln(out, yellow(f, "Some checks failed — see above."))
			}

			return nil
		},
	}

	return cmd
}

func photosAppPath() string {
	for _, p := range []string{
		"/System/Applications/Photos.app",
		"/Applications/Photos.app",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func macOSVersion() string {
	b, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func checkSchema(db *sql.DB) bool {
	for _, table := range []string{"ZASSET", "ZADDITIONALASSETATTRIBUTES"} {
		var name string
		if err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name); err != nil || name != table {
			return false
		}
	}
	return true
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

func formatInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	out := []byte{}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
