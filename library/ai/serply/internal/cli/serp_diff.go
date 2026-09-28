// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/cliutil"
)

// maxSerpSnapshots bounds how many runs of one query are kept on disk.
const maxSerpSnapshots = 10

type serpSnapshot struct {
	Query    string       `json:"query"`
	Location string       `json:"location,omitempty"`
	Gl       string       `json:"gl,omitempty"`
	Hl       string       `json:"hl,omitempty"`
	TakenAt  time.Time    `json:"taken_at"`
	Results  []serpResult `json:"results"`
}

type serpSnapshotFile struct {
	Snapshots []serpSnapshot `json:"snapshots"`
}

type serpMove struct {
	Link  string `json:"link"`
	Title string `json:"title"`
	From  int    `json:"from"`
	To    int    `json:"to"`
	Delta int    `json:"delta"`
}

type serpDiffView struct {
	Query      string       `json:"query"`
	Location   string       `json:"location,omitempty"`
	FirstRun   bool         `json:"first_run"`
	BaselineAt *time.Time   `json:"baseline_at,omitempty"`
	CurrentAt  *time.Time   `json:"current_at,omitempty"`
	Entered    []serpResult `json:"entered"`
	Left       []serpResult `json:"left"`
	Moved      []serpMove   `json:"moved"`
	Unchanged  int          `json:"unchanged"`
	Saved      bool         `json:"saved"`
	Note       string       `json:"note,omitempty"`
}

// diffSerps compares two result lists by normalized URL. Entered and moved
// rows use current positions; left rows keep their baseline positions.
func diffSerps(baseline, current []serpResult) (entered, left []serpResult, moved []serpMove, unchanged int) {
	entered, left, moved = []serpResult{}, []serpResult{}, []serpMove{}
	before := map[string]serpResult{}
	for _, r := range baseline {
		k := normalizeLink(r.Link)
		if _, dup := before[k]; !dup {
			before[k] = r
		}
	}
	seen := map[string]bool{}
	for _, r := range current {
		k := normalizeLink(r.Link)
		if seen[k] {
			continue
		}
		seen[k] = true
		old, ok := before[k]
		switch {
		case !ok:
			entered = append(entered, r)
		case old.Position != r.Position:
			moved = append(moved, serpMove{Link: r.Link, Title: r.Title, From: old.Position, To: r.Position, Delta: old.Position - r.Position})
		default:
			unchanged++
		}
	}
	for _, r := range baseline {
		if !seen[normalizeLink(r.Link)] {
			left = append(left, r)
			seen[normalizeLink(r.Link)] = true
		}
	}
	return entered, left, moved, unchanged
}

func activeClientProfile(flags *rootFlags) string {
	if flags == nil {
		return ""
	}
	if flags.platformSession != nil {
		if name := strings.TrimSpace(flags.platformSession.ProfileName); name != "" {
			return name
		}
	}
	if name := strings.TrimSpace(flags.clientProfileName); name != "" {
		return name
	}
	return strings.TrimSpace(os.Getenv("PRINTING_PRESS_CLIENT_PROFILE"))
}

func serpSnapshotPath(q string, opts serpOptions, profile string) (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	key := strings.Join([]string{strings.ToLower(strings.TrimSpace(q)), strings.ToUpper(opts.Location), strings.ToLower(opts.Device), strings.ToLower(opts.Gl), strings.ToLower(opts.Hl), strconv.Itoa(opts.Num), strings.TrimSpace(profile)}, "\x00")
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "serp-snapshots", hex.EncodeToString(sum[:8])+".json"), nil
}

func loadSerpSnapshots(path string) (serpSnapshotFile, error) {
	var f serpSnapshotFile
	data, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- app-derived data path.
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("reading snapshot history %s: %w", path, err)
	}
	return f, nil
}

func saveSerpSnapshots(path string, f serpSnapshotFile) error {
	if len(f.Snapshots) > maxSerpSnapshots {
		f.Snapshots = f.Snapshots[len(f.Snapshots)-maxSerpSnapshots:]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// A unique temp file keeps two concurrent runs from clobbering each
	// other's half-written snapshot before the rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// recordSerpSnapshot appends current under an inter-process lock that covers
// the load, append, and rename. A temp file alone still loses a concurrent
// run that loaded the same history and renamed over it.
func recordSerpSnapshot(path string, current serpSnapshot) (*serpSnapshot, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	var baseline *serpSnapshot
	err := cliutil.WithFileLock(path, func() error {
		history, err := loadSerpSnapshots(path)
		if err != nil {
			return err
		}
		if n := len(history.Snapshots); n > 0 {
			prev := history.Snapshots[n-1]
			baseline = &prev
		}
		history.Snapshots = append(history.Snapshots, current)
		return saveSerpSnapshots(path, history)
	})
	if err != nil {
		return nil, err
	}
	return baseline, nil
}

func newNovelSerpDiffCmd(flags *rootFlags) *cobra.Command {
	var flagQ string
	var opts serpOptions
	var noSave, offline bool

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "See which URLs entered, left, or moved in a results page since the last time you ran the same query.",
		Long: strings.Trim(`
Run a Google web search and compare it with the last stored run of the same
query, location, device, result depth (--num), gl, hl and client profile. The first run stores a baseline and
reports first_run=true. Later runs list the URLs that entered, left or moved.
Snapshots live in the CLI data directory; the last 10 runs per query are kept.
--offline compares the two most recent stored runs without spending a credit.

Use this command to see what changed in a SERP since the last run. Do NOT use
it for a one-off search; use 'web' instead.`, "\n"),
		Example: strings.Trim(`
  serply-pp-cli serp diff --q "best static site generator" --agent
  serply-pp-cli serp diff --q "serp api" --x-proxy-location GB
  serply-pp-cli serp diff --q "serp api" --x-proxy-location GB --offline`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "false",
			"pp:data-source": "live",
			"pp:happy-args":  "--q=best static site generator;--num=10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "serp diff")
			}
			if strings.TrimSpace(flagQ) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--q is required"))
			}
			if err := validateDevice(opts.Device); err != nil {
				return err
			}
			path, err := serpSnapshotPath(flagQ, opts, activeClientProfile(flags))
			if err != nil {
				return err
			}

			var baseline *serpSnapshot
			var current serpSnapshot
			saved := false
			if offline {
				history, err := loadSerpSnapshots(path)
				if err != nil {
					return err
				}
				n := len(history.Snapshots)
				if n < 2 {
					view := serpDiffView{Query: flagQ, Location: strings.ToUpper(opts.Location), FirstRun: n == 0,
						Entered: []serpResult{}, Left: []serpResult{}, Moved: []serpMove{},
						Note: fmt.Sprintf("%d stored run(s) for this query; --offline needs two. Run without --offline to take one.", n)}
					if n == 1 {
						at := history.Snapshots[0].TakenAt
						view.CurrentAt = &at
					}
					return printSerpDiff(cmd, flags, view)
				}
				baseline, current = &history.Snapshots[n-2], history.Snapshots[n-1]
			} else {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				results, err := fetchVertical(ctx, c, serpVerticals["web"], flagQ, opts, true)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				current = serpSnapshot{Query: flagQ, Location: strings.ToUpper(opts.Location), Gl: opts.Gl, Hl: opts.Hl, TakenAt: time.Now().UTC(), Results: results}
				if noSave {
					history, err := loadSerpSnapshots(path)
					if err != nil {
						return err
					}
					if n := len(history.Snapshots); n > 0 {
						prev := history.Snapshots[n-1]
						baseline = &prev
					}
				} else {
					baseline, err = recordSerpSnapshot(path, current)
					if err != nil {
						return fmt.Errorf("saving snapshot: %w", err)
					}
					saved = true
				}
			}

			currentAt := current.TakenAt
			view := serpDiffView{Query: flagQ, Location: strings.ToUpper(opts.Location), CurrentAt: &currentAt}
			if baseline == nil {
				view.FirstRun = true
				view.Entered, view.Left, view.Moved = []serpResult{}, []serpResult{}, []serpMove{}
				view.Unchanged = 0
				view.Note = fmt.Sprintf("baseline stored with %d results; run again later to see changes", len(current.Results))
			} else {
				at := baseline.TakenAt
				view.BaselineAt = &at
				view.Entered, view.Left, view.Moved, view.Unchanged = diffSerps(baseline.Results, current.Results)
			}
			if saved {
				view.Saved = true
			}
			if view.FirstRun && noSave {
				view.Note = "no stored run for this query and --no-save kept this one out of history"
			}
			return printSerpDiff(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagQ, "q", "", "Search query to track.")
	cmd.Flags().IntVar(&opts.Num, "num", 10, "How many results to compare (Serply returns up to about 10 per call).")
	cmd.Flags().StringVar(&opts.Location, "x-proxy-location", "", "Two-letter country code to search from, for example US or GB.")
	cmd.Flags().StringVar(&opts.Device, "x-user-agent", "", "Device type to emulate: desktop or mobile.")
	cmd.Flags().StringVar(&opts.Gl, "gl", "", "Country code for results, for example us.")
	cmd.Flags().StringVar(&opts.Hl, "hl", "", "Interface language code, for example en.")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Compare without storing this run as the new baseline.")
	cmd.Flags().BoolVar(&offline, "offline", false, "Compare the two most recent stored runs; no API call.")
	return cmd
}

func printSerpDiff(cmd *cobra.Command, flags *rootFlags, view serpDiffView) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return flags.printJSON(cmd, view)
	}
	w := cmd.OutOrStdout()
	if view.BaselineAt == nil {
		fmt.Fprintf(w, "%q: %s\n", view.Query, view.Note)
		return nil
	}
	fmt.Fprintf(w, "%q: changes since %s\n", view.Query, view.BaselineAt.Format(time.RFC3339))
	for _, r := range view.Entered {
		fmt.Fprintf(w, "  + #%-3d %s\n", r.Position, r.Link)
	}
	for _, r := range view.Left {
		fmt.Fprintf(w, "  - was #%-3d %s\n", r.Position, r.Link)
	}
	for _, m := range view.Moved {
		fmt.Fprintf(w, "  ~ #%d -> #%d %s\n", m.From, m.To, m.Link)
	}
	fmt.Fprintf(w, "  %d entered, %d left, %d moved, %d unchanged\n", len(view.Entered), len(view.Left), len(view.Moved), view.Unchanged)
	return nil
}
