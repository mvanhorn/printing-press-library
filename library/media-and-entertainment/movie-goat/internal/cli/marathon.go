package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
)

// marathonEntry is one row in a planned franchise marathon.
type marathonEntry struct {
	Order             int    `json:"order"`
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Year              string `json:"year"`
	ReleaseDate       string `json:"release_date"`
	Runtime           int    `json:"runtime"`
	CumulativeRuntime int    `json:"cumulative_runtime"`
}

// marathonOutput is the typed JSON contract for the marathon command.
type marathonOutput struct {
	Collection   string          `json:"collection"`
	CollectionID int             `json:"collection_id"`
	Order        string          `json:"order"`
	Entries      []marathonEntry `json:"entries"`
	TotalRuntime int             `json:"total_runtime"`
	Breakpoints  []int           `json:"breakpoints"` // entry orders after which a break is suggested
}

func newMarathonCmd(flags *rootFlags) *cobra.Command {
	var flagOrder string
	var flagBreaksEvery int
	var flagIncludeUnreleased bool

	cmd := &cobra.Command{
		Use:         "marathon <title-or-collection-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Plan a franchise marathon with watch order, total runtime, and breaks",
		Long: `Resolve a movie title (uses belongs_to_collection on the top result) or a
numeric collection id, fetch the parts list from /collection/{id}, fan out
/movie/{id} per part to gather runtime and release date, then suggest break
points every --breaks-every minutes (default 240).

--order release sorts ascending by release_date (default).
--order inuniverse falls back to release order with a warning when the
parts list lacks a canonical chronology field.`,
		Example: `  movie-goat-pp-cli marathon "Star Wars"
  movie-goat-pp-cli marathon 10 --order release
  movie-goat-pp-cli marathon "Lord of the Rings" --breaks-every 180 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			arg := strings.TrimSpace(strings.Join(args, " "))
			order := strings.ToLower(strings.TrimSpace(flagOrder))
			if order == "" {
				order = "release"
			}
			if order != "release" && order != "inuniverse" {
				return usageErr(fmt.Errorf("--order must be \"release\" or \"inuniverse\", got %q", flagOrder))
			}
			breaksEvery := flagBreaksEvery
			if breaksEvery <= 0 {
				breaksEvery = 240
			}

			// Resolve collection id from numeric arg or via movie -> belongs_to_collection.
			var collectionID int
			var collectionName string
			if id, perr := strconv.Atoi(arg); perr == nil {
				collectionID = id
			} else {
				movieID, _, mlerr := searchMovieByTitle(c, arg)
				if mlerr != nil {
					return classifyAPIError(mlerr)
				}
				detail, _, dErr := getMovieDetail(c, movieID, "")
				if dErr != nil {
					return classifyAPIError(dErr)
				}
				if detail.BelongsToCollection == nil || detail.BelongsToCollection.ID == 0 {
					return notFoundErr(fmt.Errorf("%q is not part of a TMDb collection", arg))
				}
				collectionID = detail.BelongsToCollection.ID
				collectionName = detail.BelongsToCollection.Name
			}

			// Fetch collection parts.
			collectionData, err := c.Get(fmt.Sprintf("/collection/%d", collectionID), map[string]string{})
			if err != nil {
				return classifyAPIError(err)
			}
			var collection struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Parts []struct {
					ID          int    `json:"id"`
					Title       string `json:"title"`
					ReleaseDate string `json:"release_date"`
				} `json:"parts"`
			}
			if err := json.Unmarshal(collectionData, &collection); err != nil {
				return fmt.Errorf("parsing collection: %w", err)
			}
			if collectionName == "" {
				collectionName = collection.Name
			}
			if len(collection.Parts) == 0 {
				return notFoundErr(fmt.Errorf("collection %q has no parts", collectionName))
			}

			// Sort. Release order is well-defined; in-universe falls back to
			// release order (TMDb's collection schema has no chronology field
			// today, so honoring "inuniverse" without a manual override is
			// best-effort). Surface the fallback to stderr so users know.
			sort.Slice(collection.Parts, func(i, j int) bool {
				return collection.Parts[i].ReleaseDate < collection.Parts[j].ReleaseDate
			})
			if order == "inuniverse" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warn: TMDb does not expose canonical in-universe ordering; falling back to release order.")
			}

			// Fan out runtimes.
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			type partInfo struct {
				ID      int
				Title   string
				Release string
				Runtime int
			}
			// Filter out unreleased entries by default — they have runtime 0
			// and pollute the marathon plan (cumulative runtime stalls, the
			// row implies the title is watchable now). Users can opt back in
			// with --include-unreleased to scope the future plan.
			today := time.Now().UTC().Format("2006-01-02")
			rawParts := collection.Parts
			filtered := rawParts[:0]
			skipped := 0
			for _, p := range rawParts {
				if !flagIncludeUnreleased && p.ReleaseDate != "" && p.ReleaseDate > today {
					skipped++
					continue
				}
				filtered = append(filtered, p)
			}
			if skipped > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "info: skipped %d unreleased title(s); pass --include-unreleased to include them\n", skipped)
			}
			collection.Parts = filtered
			if len(collection.Parts) == 0 {
				return notFoundErr(fmt.Errorf("collection %q has no released parts (use --include-unreleased to include upcoming titles)", collectionName))
			}
			parts := make([]partInfo, len(collection.Parts))
			for i, p := range collection.Parts {
				parts[i] = partInfo{ID: p.ID, Title: p.Title, Release: p.ReleaseDate}
			}
			type runtimeRow struct {
				id      int
				runtime int
			}
			results, errs := cliutil.FanoutRun(ctx, parts,
				func(p partInfo) string { return p.Title },
				func(_ context.Context, p partInfo) (runtimeRow, error) {
					detail, _, derr := getMovieDetail(c, p.ID, "")
					if derr != nil {
						return runtimeRow{id: p.ID}, derr
					}
					return runtimeRow{id: p.ID, runtime: detail.Runtime}, nil
				})
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
			runtimes := make(map[int]int, len(results))
			for _, r := range results {
				runtimes[r.Value.id] = r.Value.runtime
			}
			for i := range parts {
				parts[i].Runtime = runtimes[parts[i].ID]
			}

			// Build entries + cumulative runtime + breakpoints.
			entries := make([]marathonEntry, 0, len(parts))
			total := 0
			var breakpoints []int
			cumulativeSinceLastBreak := 0
			for i, p := range parts {
				e := marathonEntry{
					Order:       i + 1,
					ID:          p.ID,
					Title:       p.Title,
					ReleaseDate: p.Release,
					Runtime:     p.Runtime,
				}
				if len(p.Release) >= 4 {
					e.Year = p.Release[:4]
				}
				total += p.Runtime
				e.CumulativeRuntime = total
				cumulativeSinceLastBreak += p.Runtime
				entries = append(entries, e)
				// Suggest a break after this entry when accumulated runtime
				// since the last break crosses the threshold and there's
				// still more to watch.
				if cumulativeSinceLastBreak >= breaksEvery && i < len(parts)-1 {
					breakpoints = append(breakpoints, e.Order)
					cumulativeSinceLastBreak = 0
				}
			}

			out := marathonOutput{
				Collection:   collectionName,
				CollectionID: collectionID,
				Order:        order,
				Entries:      entries,
				TotalRuntime: total,
				Breakpoints:  breakpoints,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s Marathon\n", collectionName)
			fmt.Fprintln(w, strings.Repeat("=", len(collectionName)+9))
			fmt.Fprintf(w, "%d titles | total %s\n\n", len(entries), formatRuntimeMinutes(total))
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "#\tTitle\tYear\tRuntime\tCumulative")
			for _, e := range entries {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
					e.Order,
					truncate(e.Title, 40),
					e.Year,
					formatRuntimeMinutes(e.Runtime),
					formatRuntimeMinutes(e.CumulativeRuntime))
			}
			tw.Flush()
			if len(breakpoints) > 0 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Suggested breaks:")
				for _, bp := range breakpoints {
					fmt.Fprintf(w, "  after #%d\n", bp)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOrder, "order", "release", "Watch order: release | inuniverse")
	cmd.Flags().IntVar(&flagBreaksEvery, "breaks-every", 240, "Insert a break suggestion after every N minutes")
	cmd.Flags().BoolVar(&flagIncludeUnreleased, "include-unreleased", false, "Include franchise entries with future release dates (default: skip them)")
	return cmd
}
