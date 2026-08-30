// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// watch.go implements the `watch` command: track a search term across runs and
// report what changed since last time — newly-appeared trials, trials whose
// last-update date moved, and trials that have since been terminated. State is a
// per-term JSON snapshot under the resolved state dir; the command is read-write
// (it persists the new snapshot) so successive runs form a change feed.
package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// watchSnapshot is the persisted per-term state: one entry per trial.
type watchSnapshot struct {
	Term   string                `json:"term"`
	Trials map[string]watchEntry `json:"trials"`
}

type watchEntry struct {
	Title      string `json:"title,omitempty"`
	Status     string `json:"status,omitempty"`
	LastUpdate string `json:"last_update,omitempty"`
}

type watchChange struct {
	NCTID      string `json:"id"`
	Title      string `json:"title,omitempty"`
	Status     string `json:"status,omitempty"`
	WasStatus  string `json:"was_status,omitempty"`
	LastUpdate string `json:"last_update,omitempty"`
}

type watchView struct {
	Term       string        `json:"term"`
	FirstRun   bool          `json:"first_run"`
	Tracked    int           `json:"tracked"`
	New        []watchChange `json:"new,omitempty"`
	Changed    []watchChange `json:"changed,omitempty"`
	Terminated []watchChange `json:"terminated,omitempty"`
	Note       string        `json:"note,omitempty"`
}

// pp:data-source live
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var sample, maxScanPages int
	var reset bool
	cmd := &cobra.Command{
		Use:   "watch <term>",
		Short: "Track a term and report new, changed, and terminated trials since the last run.",
		Long: "Snapshot the trials matching a term and, on each subsequent run, report which\n" +
			"trials are new, which had their last-update date change, and which have since\n" +
			"been terminated/withdrawn/suspended. State persists per term under the resolved\n" +
			"state directory (respects --home and XDG_STATE_HOME). The first run establishes\n" +
			"the baseline.",
		Example: "  clinical-trials-pp-cli watch \"vitamin d\" --json\n" +
			"  clinical-trials-pp-cli watch alzheimer --reset",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would track a term and diff against the last run")
				return nil
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a term argument is required"))
			}
			statePath, err := watchStatePath(term)
			if err != nil {
				return err
			}
			if reset {
				_ = os.Remove(statePath)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			trials, err := ctgovFetch(ctx, c, ctgovParams("term", term), sample, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			current := watchSnapshot{Term: term, Trials: map[string]watchEntry{}}
			for _, t := range trials {
				current.Trials[t.NCTID] = watchEntry{Title: t.Title, Status: t.Status, LastUpdate: t.LastUpdate}
			}

			prev, hadPrev := loadWatchSnapshot(statePath)
			view := diffWatch(term, prev, hadPrev, current)

			if err := saveWatchSnapshot(statePath, current); err != nil {
				view.Note = strings.TrimSpace(view.Note + " (warning: could not persist state: " + err.Error() + ")")
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderWatchHuman(cmd, view); err != nil {
				return err
			}
			if view.Tracked == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&sample, "sample", 200, "Max trials to track")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 1, "Max ClinicalTrials.gov pages to scan")
	cmd.Flags().BoolVar(&reset, "reset", false, "Clear stored state for this term and re-establish the baseline")
	return cmd
}

// diffWatch computes the change feed between the previous and current snapshots.
func diffWatch(term string, prev watchSnapshot, hadPrev bool, current watchSnapshot) watchView {
	v := watchView{Term: term, Tracked: len(current.Trials)}
	if !hadPrev {
		v.FirstRun = true
		v.Note = fmt.Sprintf("baseline established for %d trials; re-run to see changes", len(current.Trials))
		return v
	}
	// Stable ordering for deterministic output.
	ids := make([]string, 0, len(current.Trials))
	for id := range current.Trials {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		cur := current.Trials[id]
		old, existed := prev.Trials[id]
		ch := watchChange{NCTID: id, Title: cur.Title, Status: cur.Status, LastUpdate: cur.LastUpdate}
		if !existed {
			v.New = append(v.New, ch)
			continue
		}
		if isTerminalStatus(cur.Status) && !isTerminalStatus(old.Status) {
			ch.WasStatus = old.Status
			v.Terminated = append(v.Terminated, ch)
			continue
		}
		if cur.LastUpdate != old.LastUpdate || cur.Status != old.Status {
			ch.WasStatus = old.Status
			v.Changed = append(v.Changed, ch)
		}
	}
	if len(v.New) == 0 && len(v.Changed) == 0 && len(v.Terminated) == 0 {
		v.Note = "no changes since last run"
	}
	return v
}

func isTerminalStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "TERMINATED", "WITHDRAWN", "SUSPENDED":
		return true
	}
	return false
}

// watchStatePath returns the per-term snapshot path under the resolved state
// directory (cliutil.StateDir, which honours --home, XDG_STATE_HOME, and the
// CLI's own env overrides), using a slug plus a short hash so distinct terms
// never collide.
func watchStatePath(term string) (string, error) {
	dir, err := cliutil.KindDir(cliutil.PathKindState)
	if err != nil {
		return "", fmt.Errorf("resolving state directory: %w", err)
	}
	dir = filepath.Join(dir, "watch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating state directory: %w", err)
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(term))))
	name := slugForFilename(term) + "-" + fmt.Sprintf("%x", sum[:4]) + ".json"
	return filepath.Join(dir, name), nil
}

// slugForFilename reduces a term to a filesystem-safe slug fragment.
func slugForFilename(term string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(term) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "term"
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func loadWatchSnapshot(path string) (watchSnapshot, bool) {
	// #nosec G304 -- path comes from watchStatePath: a sha256-derived filename
	// under the resolved state dir, never raw user input.
	data, err := os.ReadFile(path)
	if err != nil {
		return watchSnapshot{}, false
	}
	var snap watchSnapshot
	if err := json.Unmarshal(data, &snap); err != nil || snap.Trials == nil {
		return watchSnapshot{}, false
	}
	return snap, true
}

func saveWatchSnapshot(path string, snap watchSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func renderWatchHuman(cmd *cobra.Command, v watchView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Watch — %s (tracking %d trials)\n\n", v.Term, v.Tracked)
	if v.FirstRun {
		fmt.Fprintf(w, "%s\n", v.Note)
		return nil
	}
	printWatchSection(w, "New trials", v.New, false)
	printWatchSection(w, "Changed trials", v.Changed, true)
	printWatchSection(w, "Newly terminated", v.Terminated, true)
	if v.Note != "" {
		fmt.Fprintf(w, "note: %s\n", v.Note)
	}
	return nil
}

func printWatchSection(w interface{ Write([]byte) (int, error) }, heading string, changes []watchChange, showWas bool) {
	if len(changes) == 0 {
		return
	}
	fmt.Fprintf(w, "%s (%d):\n", heading, len(changes))
	for _, ch := range changes {
		fmt.Fprintf(w, "  %s  %s\n", ch.NCTID, truncate(ch.Title, 70))
		if showWas && ch.WasStatus != "" && ch.WasStatus != ch.Status {
			fmt.Fprintf(w, "      status: %s -> %s\n", ch.WasStatus, ch.Status)
		} else {
			fmt.Fprintf(w, "      status=%s last_update=%s\n", ch.Status, ch.LastUpdate)
		}
	}
	fmt.Fprintln(w)
}