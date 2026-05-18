// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newCollectCmd(flags *rootFlags) *cobra.Command {
	var since, until, outDir string
	var minWords int
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect microphone-source segments across meetings into daily files",
		Long: `For each meeting since DATE, writes daily-named files (YYYY-MM-DD.md)
containing only microphone-source transcript segments, one paragraph
per segment, filtered to segments with >= --min-words words.`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return ioErr(err)
			}
			var from, to time.Time
			if since != "" {
				t, err := parseAnyDate(since)
				if err != nil {
					return usageErr(err)
				}
				from = t
			}
			if until != "" {
				t, err := parseAnyDate(until)
				if err != nil {
					return usageErr(err)
				}
				to = t
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			perDay := map[string][]string{}
			perDayOrder := []string{}
			for _, id := range c.SortedDocumentIDs() {
				d := c.Documents[id]
				segs := c.TranscriptByID(id)
				if len(segs) == 0 {
					continue
				}
				ts, _ := granola.ParseISO(d.CreatedAt)
				if from != (time.Time{}) && ts.Before(from) {
					continue
				}
				if to != (time.Time{}) && ts.After(to) {
					continue
				}
				day := ts.Format("2006-01-02")
				if _, seen := perDay[day]; !seen {
					perDayOrder = append(perDayOrder, day)
				}
				for _, s := range segs {
					if !strings.EqualFold(s.Source, "microphone") {
						continue
					}
					if minWords > 0 && len(strings.Fields(s.Text)) < minWords {
						continue
					}
					perDay[day] = append(perDay[day], s.Text)
				}
			}
			w := cmd.OutOrStdout()
			for _, day := range perDayOrder {
				path := filepath.Join(outDir, day+".md")
				body := "# " + day + "\n\n" + strings.Join(perDay[day], "\n\n") + "\n"
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					fmt.Fprintf(w, `{"day":%q,"error":%q}`+"\n", day, err.Error())
					continue
				}
				fmt.Fprintf(w, `{"day":%q,"file":%q,"segments":%d}`+"\n", day, path, len(perDay[day]))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start date (default: all time)")
	cmd.Flags().StringVar(&until, "until", "", "End date")
	cmd.Flags().StringVarP(&outDir, "out", "o", "", "Output directory")
	cmd.Flags().IntVar(&minWords, "min-words", 0, "Minimum words per segment")
	return cmd
}
