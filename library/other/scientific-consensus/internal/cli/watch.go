// Hand-authored novel command: topic watch / new-publication alerting. Not generated.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

type watchBaseline struct {
	Query     string            `json:"query"`
	UpdatedAt string            `json:"updated_at"`
	SeenIDs   map[string]bool   `json:"seen_ids"`
}

type watchOutput struct {
	Query        string      `json:"query"`
	BaselineDate string      `json:"baseline_date,omitempty"`
	FirstRun     bool        `json:"first_run"`
	NewWorks     []workBrief `json:"new_works"`
	NewCount     int         `json:"new_count"`
	TrackedTotal int         `json:"tracked_total"`
	Note         string      `json:"note,omitempty"`
}

func watchDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "scientific-consensus-pp-cli", "watch")
	return dir, os.MkdirAll(dir, 0o755)
}

func watchPath(query string) (string, error) {
	dir, err := watchDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(query))
	return filepath.Join(dir, hex.EncodeToString(sum[:8])+".json"), nil
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var reset bool

	cmd := &cobra.Command{
		Use:   "watch <query>",
		Short: "Monitor a topic and report new publications since the last run",
		Long: "Persist a baseline of seen works for a topic and, on each subsequent run, report\n" +
			"works that are new since the baseline. Use --reset to clear the baseline. State\n" +
			"is stored locally under your user config directory.",
		Example:     "  scientific-consensus-pp-cli watch \"GLP-1 cardiovascular outcomes\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check for new publications since last run")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			path, err := watchPath(query)
			if err != nil {
				return err
			}
			if reset {
				_ = os.Remove(path)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			works, _, err := fetchWorks(ctx, c, query, "", "publication_date:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			base := loadBaseline(path, query)
			out := watchOutput{Query: query, BaselineDate: base.UpdatedAt, FirstRun: base.UpdatedAt == ""}
			prog := newProgress(flags, "scanning works", len(works))
			for i, wk := range works {
				if !base.SeenIDs[wk.ID] {
					if !out.FirstRun {
						out.NewWorks = append(out.NewWorks, workBrief{
							Title: wk.Title, Year: wk.Year, DOI: wk.DOI, CitedBy: wk.CitedBy,
						})
					}
					base.SeenIDs[wk.ID] = true
				}
				prog.update(i + 1)
			}
			prog.done()
			out.NewCount = len(out.NewWorks)
			out.TrackedTotal = len(base.SeenIDs)
			base.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := saveBaseline(path, base); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist watch baseline: %v\n", err)
			}
			switch {
			case out.FirstRun:
				out.Note = fmt.Sprintf("baseline established with %d works; re-run later to see new publications", out.TrackedTotal)
			case out.NewCount == 0:
				out.Note = "no new publications since last run"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderWatch(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "number of most-recent works to check (max 200)")
	cmd.Flags().BoolVar(&reset, "reset", false, "clear the stored baseline before checking")
	return cmd
}

func loadBaseline(path, query string) watchBaseline {
	b := watchBaseline{Query: query, SeenIDs: map[string]bool{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return b
	}
	var loaded watchBaseline
	if json.Unmarshal(data, &loaded) == nil && loaded.SeenIDs != nil {
		return loaded
	}
	return b
}

func saveBaseline(path string, b watchBaseline) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func renderWatch(w io.Writer, o watchOutput) {
	fmt.Fprintf(w, "Watch: %s\n", o.Query)
	if o.FirstRun {
		fmt.Fprintf(w, "  First run — baseline of %d works established.\n", o.TrackedTotal)
		return
	}
	fmt.Fprintf(w, "  %d new publication(s) since %s (tracking %d total)\n\n", o.NewCount, o.BaselineDate, o.TrackedTotal)
	for _, b := range o.NewWorks {
		fmt.Fprintf(w, "  • [%d] %s\n", b.Year, truncate(b.Title, 80))
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
