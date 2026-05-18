// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newMemoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memo",
		Short: "MEMO pipeline: preflight + extract for one or many meetings",
	}
	cmd.AddCommand(newMemoRunCmd(flags))
	cmd.AddCommand(newMemoQueueCmd(flags))
	return cmd
}

func newMemoRunCmd(flags *rootFlags) *cobra.Command {
	var last, since, until, outDir, root string
	var limit int
	cmd := &cobra.Command{
		Use:   "run [<id>]",
		Short: "Run preflight + extract for one meeting, or every new meeting in --since",
		Long: `Without an id, iterates every meeting in --since/--last/--until and runs
the pipeline for those whose full_<id>.md does not yet exist in --to.

Emits ndjson one line per meeting: {id, status: new|skipped|duplicate|error|missing_transcript, files, error}.`,
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,1,2,3",
			// not read-only — writes files
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			home, _ := os.UserHomeDir()
			if root == "" {
				root = filepath.Join(home, PreflightDefaultRoot)
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return ioErr(err)
			}
			w := cmd.OutOrStdout()
			var ids []string
			if len(args) > 0 {
				ids = []string{args[0]}
			} else {
				from, to, err := parseTimeWindow(last, since, until)
				if err != nil {
					return usageErr(err)
				}
				c, err := openGranolaCache()
				if err != nil {
					return err
				}
				ids = selectDocsInWindow(c, from, to, limit)
			}
			for _, id := range ids {
				rec := runOneMemo(id, outDir, root, flags.dataSource != "local")
				_ = emitNDJSONLine(w, rec)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&last, "last", "", "Time window (e.g. 7d)")
	cmd.Flags().StringVar(&since, "since", "", "Start date")
	cmd.Flags().StringVar(&until, "until", "", "End date")
	cmd.Flags().StringVarP(&outDir, "out", "o", "", "Output directory (alias: --to)")
	cmd.Flags().StringVar(&outDir, "to", "", "Output directory alias")
	cmd.Flags().StringVar(&root, "root", "", "Existing transcripts root (for duplicate check)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap meetings processed")
	return cmd
}

type memoRecord struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Files       []string `json:"files,omitempty"`
	DuplicateOf string   `json:"duplicate_of,omitempty"`
	Error       string   `json:"error,omitempty"`
}

func runOneMemo(id, outDir, root string, allowLive bool) memoRecord {
	rec := memoRecord{ID: id}
	c, err := openGranolaCache()
	if err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		return rec
	}
	d := c.DocumentByID(id)
	if d == nil {
		rec.Status = "error"
		rec.Error = "meeting not in cache"
		return rec
	}
	segs := c.TranscriptByID(id)
	mic, sys := countSources(segs)
	if mic < 5 || sys < 5 {
		rec.Status = "missing_transcript"
		return rec
	}
	if dup := findDuplicateFile(root, id, d.Title); dup != "" {
		rec.Status = "duplicate"
		rec.DuplicateOf = dup
		return rec
	}
	res, err := runExtract(id, outDir, "", allowLive)
	if err != nil {
		var ce *cliError
		if errors.As(err, &ce) {
			switch ce.code {
			case 2:
				rec.Status = "missing_transcript"
			case 3:
				rec.Status = "duplicate"
				rec.DuplicateOf = "full_" + id + ".md (in --out dir)"
			default:
				rec.Status = "error"
			}
		} else {
			rec.Status = "error"
		}
		rec.Error = err.Error()
		return rec
	}
	rec.Status = "new"
	rec.Files = res.Files
	return rec
}

func newMemoQueueCmd(flags *rootFlags) *cobra.Command {
	var last, since, until, root string
	var limit int
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "List meetings whose transcript is cached but whose full_<id>.md is missing",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			home, _ := os.UserHomeDir()
			if root == "" {
				root = filepath.Join(home, PreflightDefaultRoot)
			}
			from, to, err := parseTimeWindow(last, since, until)
			if err != nil {
				return usageErr(err)
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			ids := selectDocsInWindow(c, from, to, 0)
			emitted := 0
			for _, id := range ids {
				segs := c.TranscriptByID(id)
				mic, sys := countSources(segs)
				if mic < 5 || sys < 5 {
					continue
				}
				d := c.DocumentByID(id)
				if d == nil {
					continue
				}
				if dup := findDuplicateFile(root, id, d.Title); dup != "" {
					continue
				}
				_ = emitNDJSONLine(w, map[string]any{
					"id":           id,
					"title":        d.Title,
					"started_at":   d.CreatedAt,
					"mic_count":    mic,
					"system_count": sys,
				})
				emitted++
				if limit > 0 && emitted >= limit {
					break
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&last, "last", "", "Time window (e.g. 7d)")
	cmd.Flags().StringVar(&since, "since", "", "Start date")
	cmd.Flags().StringVar(&until, "until", "", "End date")
	cmd.Flags().StringVar(&root, "root", "", "Existing transcripts root (for duplicate check)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap result count")
	return cmd
}

// Ensure time + granola references are present.
var (
	_ = time.Now
	_ = granola.ParseISO
)
