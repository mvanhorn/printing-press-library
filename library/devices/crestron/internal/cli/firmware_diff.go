// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Reads change logs from the local mirror when synced, otherwise fetches the
// two release pages live. Both paths need a signed-in session for note text.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronfw"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronstore"

	"github.com/spf13/cobra"
)

type firmwareDiffView struct {
	Model     string   `json:"model"`
	From      string   `json:"from_version"`
	To        string   `json:"to_version"`
	FromDate  string   `json:"from_date,omitempty"`
	ToDate    string   `json:"to_date,omitempty"`
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
	Unchanged int      `json:"unchanged_lines"`
	CoveredBy []string `json:"covered_by,omitempty"`
	NeedsAuth bool     `json:"requires_sign_in"`
	Note      string   `json:"note,omitempty"`
}

func newNovelFirmwareDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "diff <model> <from-version> <to-version>",
		Short: "Show what changed between two firmware versions for a model.",
		Long: strings.Trim(`
Show the change-log difference between two firmware versions of a model.

Crestron publishes change logs only as separate per-version pages behind a
sign-in, with no diff view anywhere. This resolves the model to the releases
that cover it — one release often covers a whole model family — fetches both
versions, and reports the lines added and removed.

Requires a signed-in session for the change-log text. Run
'crestron-pp-cli auth login --chrome' first.

Use this command to see what changed between two firmware versions.
Do NOT use it to read a single version's notes; use 'firmware notes'.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli firmware diff DM-NVX-384 7.3.0125 7.4.0255.22319 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a model and two versions are required"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "firmware diff")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			model, fromV, toV := args[0], args[1], args[2]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, haveMirror := openMirror(ctx, flagDB)
			if haveMirror {
				defer func() { _ = st.Close() }()
			}

			// Find the releases covering this model, preferring the local join.
			var releases []crestronstore.Release
			if haveMirror {
				releases, _ = st.ReleasesForModel(ctx, model)
			}
			if len(releases) == 0 {
				rows, err := fleetSearcher{flags: flags}.SearchFirmware(ctx, model, 50)
				if err != nil {
					return fmt.Errorf("finding releases for %s: %w", model, err)
				}
				for _, rel := range crestronfw.ReleasesFrom(rows) {
					releases = append(releases, crestronstore.Release{
						URL: rel.URL, Title: rel.Title, Version: rel.Version,
						Date: rel.Date, Models: rel.Models,
					})
				}
			}
			pick := func(version string) (crestronstore.Release, bool) {
				for _, r := range releases {
					if strings.EqualFold(r.Version, version) || strings.HasPrefix(r.Version, version) {
						return r, true
					}
				}
				return crestronstore.Release{}, false
			}
			from, okFrom := pick(fromV)
			to, okTo := pick(toV)
			if !okFrom || !okTo {
				have := make([]string, 0, len(releases))
				for _, r := range releases {
					if r.Version != "" {
						have = append(have, r.Version)
					}
				}
				missing := fromV
				if okFrom {
					missing = toV
				}
				return fmt.Errorf("no release found for %s version %q; known versions: %s",
					model, missing, strings.Join(have, ", "))
			}

			view := firmwareDiffView{
				Model: model, From: from.Version, To: to.Version,
				FromDate: from.Date, ToDate: to.Date,
				CoveredBy: []string{from.Title, to.Title},
			}

			text := func(r crestronstore.Release) (string, bool, error) {
				if strings.TrimSpace(r.ChangeLog) != "" {
					// Trim here too: mirrors synced before the parser learned the
					// footer boundary still hold page chrome in change_log.
					return crestronparse.TrimSiteChrome(r.ChangeLog), false, nil
				}
				body, err := c.Get(ctx, r.URL, nil)
				if err != nil {
					return "", false, err
				}
				fr, err := crestronparse.ParseFirmwareRelease(body)
				if err != nil {
					return "", false, err
				}
				if fr.RequiresAuth {
					return "", true, nil
				}
				if fr.ChangeLog != "" {
					return fr.ChangeLog, false, nil
				}
				return fr.ReleaseNotes, false, nil
			}

			fromText, needAuth1, err := text(from)
			if err != nil {
				return fmt.Errorf("fetching %s notes: %w", from.Version, err)
			}
			toText, needAuth2, err := text(to)
			if err != nil {
				return fmt.Errorf("fetching %s notes: %w", to.Version, err)
			}
			if needAuth1 || needAuth2 {
				view.NeedsAuth = true
				view.Note = "change logs require a signed-in Crestron account; run 'crestron-pp-cli auth login --chrome' then retry"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			view.Added, view.Removed, view.Unchanged = diffLines(fromText, toText)
			if len(view.Added) == 0 && len(view.Removed) == 0 {
				view.Note = fmt.Sprintf("no change-log differences between %s and %s", from.Version, to.Version)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s: %s (%s) -> %s (%s)\n\n", model, from.Version, from.Date, to.Version, to.Date)
			for _, l := range view.Added {
				fmt.Fprintln(out, "+ "+l)
			}
			for _, l := range view.Removed {
				fmt.Fprintln(out, "- "+l)
			}
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

// diffLines reports sentence-level additions and removals between two change
// logs. Crestron renders change logs as prose rather than line-oriented text,
// so splitting on sentence boundaries produces a more useful diff than a
// character diff would.
func diffLines(from, to string) (added, removed []string, unchanged int) {
	added, removed = make([]string, 0), make([]string, 0)
	split := func(s string) []string {
		s = strings.ReplaceAll(s, ". ", ".\n")
		out := make([]string, 0)
		for _, l := range strings.Split(s, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
		return out
	}
	// Walk the source slices, not the lookup sets: ranging over a Go map
	// randomizes order, so identical inputs produced differently-ordered
	// diffs on every run. Source order also reads the way the change log does.
	fromLines := split(from)
	toLines := split(to)
	fromSet := map[string]bool{}
	for _, l := range fromLines {
		fromSet[l] = true
	}
	toSet := map[string]bool{}
	for _, l := range toLines {
		toSet[l] = true
	}
	seen := map[string]bool{}
	for _, l := range toLines {
		if seen[l] {
			continue
		}
		seen[l] = true
		if !fromSet[l] {
			added = append(added, l)
		} else {
			unchanged++
		}
	}
	seen = map[string]bool{}
	for _, l := range fromLines {
		if seen[l] {
			continue
		}
		seen[l] = true
		if !toSet[l] {
			removed = append(removed, l)
		}
	}
	return added, removed, unchanged
}
