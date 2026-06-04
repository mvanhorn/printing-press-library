// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newArtistCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sourceFilter string
	var limit int
	var arc bool

	cmd := &cobra.Command{
		Use:   "artist <name>",
		Short: "Find works by a creator across sources",
		Long: `Look up works whose canonical creator name contains the given query
(case-insensitive substring match on creator_canonical). Results span
every synced source by default; pass --source to restrict to one.

Substring match on the canonical form is intentional: FTS5 over the
full creator field over-matches on first names and shared particles
(e.g. "Katsushika Hokusai" picks up unrelated "Katsushika" rows). For
broader recall, use 'similar <id>'.

Pass --arc to group results into stylistic periods (using each work's
period field and date_start) and render a short career narrative
instead of a flat listing.`,
		Example: `  art-goat-pp-cli artist hokusai
  art-goat-pp-cli artist van gogh --limit 5
  art-goat-pp-cli artist rembrandt --source harvard --json
  art-goat-pp-cli artist hokusai --arc`,
		Args: cobra.MinimumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitArtistVerifyEnvelope(cmd, flags)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			name := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			if name == "" {
				return fmt.Errorf("artist name is required")
			}

			// --arc needs the full chronological set, not the default limit,
			// so the bucketing can find period boundaries. Honor an explicit
			// --limit when the user set one above the default; otherwise
			// fetch up to 500 so the arc has enough material to read as a
			// narrative.
			effectiveLimit := limit
			if arc && !cmd.Flags().Changed("limit") {
				effectiveLimit = 500
			}

			hits, err := db.WorksByCreator(cmd.Context(), name, sourceFilter, effectiveLimit)
			if err != nil {
				return err
			}

			if arc {
				arcResult := buildArtistArc(name, hits)
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), arcResult.envelope(), flags)
				}
				renderArtistArc(cmd, arcResult)
				return nil
			}

			if flags.asJSON {
				envelopes := make([]map[string]any, 0, len(hits))
				for _, w := range hits {
					envelopes = append(envelopes, workToEnvelope(w))
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"query":   name,
					"source":  sourceFilter,
					"count":   len(envelopes),
					"results": envelopes,
				}, flags)
			}

			renderArtist(cmd, name, sourceFilter, hits)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of works to return in the matched-creator listing")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Restrict results to one source slug (e.g. aic, met, harvard)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().BoolVar(&arc, "arc", false, "Group works into stylistic periods and render a short career narrative")
	if cliutil.IsStrictFlagsEnv() {
		// In audit mode, force the caller to declare an explicit cap so
		// an unbounded cross-source pull can't accidentally fan out.
		// --source has no audit value: the whole point of federated
		// lookup is "search every source" and the empty default is right.
		_ = cmd.MarkFlagRequired("limit")
	}
	return cmd
}

func renderArtist(cmd *cobra.Command, name, sourceFilter string, hits []store.Work) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	if sourceFilter != "" {
		fmt.Fprintf(out, "Artist query: %q  (source=%s)\n", name, sourceFilter)
	} else {
		fmt.Fprintf(out, "Artist query: %q\n", name)
	}
	fmt.Fprintln(out, "")
	if len(hits) == 0 {
		fmt.Fprintln(out, "No matches in the local corpus. Try `sync` first or broaden the query.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSOURCE\tTITLE\tCREATOR\tDATE")
	for _, w := range hits {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			w.ID, w.Source,
			truncate(coalesce(w.Title, "(untitled)"), 50),
			truncate(coalesce(w.Creator, "(unknown)"), 30),
			coalesce(w.DateText, ""),
		)
	}
	_ = tw.Flush()
	fmt.Fprintln(out, "")
}

func emitArtistVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "artist",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "artist reads the local store; PRINTING_PRESS_VERIFY=1 short-circuits the table rendering. Pass --json to get the data envelope.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
