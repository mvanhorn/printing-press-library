// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/lacourt-holidays/internal/courtdays"
)

type icsFileView struct {
	Year   int    `json:"year"`
	Path   string `json:"path"`
	Events int    `json:"events"`
}

type icsView struct {
	Years  []int         `json:"years"`
	Events int           `json:"events"`
	Files  []icsFileView `json:"files,omitempty"`
	ICS    string        `json:"ics,omitempty"`
}

func newNovelIcsCmd(flags *rootFlags) *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "ics [year]",
		Short: "Export LASC holidays as calendar files, for one year or every published year.",
		Long: "Use this command to export holidays as calendar files. With no year it covers every year the court has published.\n\n" +
			"Without --out-dir it prints one VCALENDAR to stdout (redirect it to a .ics file).\n" +
			"With --out-dir it writes one <year>.ics per year, ready to import into Apple Calendar, Outlook, or Google Calendar.",
		Example: "  lacourt-holidays-pp-cli ics 2027\n  lacourt-holidays-pp-cli ics --out-dir ./lasc-holidays\n  lacourt-holidays-pp-cli ics > lasc-all-years.ics",
		Annotations: map[string]string{
			"mcp:local-write":       "true",
			"pp:data-source":        "live",
			"pp:happy-args":         "year=2027",
			"pp:preview-happy-path": "true", // without --out-dir it only prints to stdout
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ics")
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("expected at most one year, got %d", len(args)))
			}
			cal, err := newLASCCalendar(cmd, flags)
			if err != nil {
				return err
			}
			var years []int
			if len(args) == 1 {
				y, convErr := strconv.Atoi(args[0])
				if convErr != nil || y < 1900 || y > 3000 {
					return usageErr(fmt.Errorf("invalid year %q", args[0]))
				}
				if _, err := cal.Year(y); err != nil {
					return calendarErr(err)
				}
				years = []int{y}
			} else {
				years, err = publishedYears(cal)
				if err != nil {
					return err
				}
				if len(years) == 0 {
					return notFoundErr(fmt.Errorf("the court has not published any holiday years around %d", time.Now().Year()))
				}
			}
			stamp := time.Now().UTC()
			view := icsView{Years: years}
			all := make([]courtdays.Holiday, 0)
			for _, y := range years {
				hs, err := cal.Year(y)
				if err != nil {
					return calendarErr(err)
				}
				view.Events += len(hs)
				if outDir != "" {
					if err := os.MkdirAll(outDir, 0o750); err != nil {
						return fmt.Errorf("creating %s: %w", outDir, err)
					}
					p := filepath.Join(outDir, fmt.Sprintf("%d.ics", y))
					if err := os.WriteFile(p, []byte(courtdays.ICS(hs, stamp)), 0o600); err != nil {
						return fmt.Errorf("writing %s: %w", p, err)
					}
					view.Files = append(view.Files, icsFileView{Year: y, Path: p, Events: len(hs)})
				} else {
					all = append(all, hs...)
				}
			}
			if outDir == "" {
				body := courtdays.ICS(all, stamp)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) && (flags.asJSON || flags.selectFields != "") {
					view.ICS = body
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), body)
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, f := range view.Files {
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d holidays)\n", f.Path, f.Events)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out-dir", "", "Write one <year>.ics file per year into this directory")
	return cmd
}
