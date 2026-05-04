package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/internal/store"
)

// watchlistRow is the JSON row shape produced by `watchlist list`. It mirrors
// store.WatchlistEntry but uses snake_case fields and includes optional
// streamability data when --available is set.
type watchlistRow struct {
	ID         int64         `json:"id"`
	TMDBID     int           `json:"tmdb_id"`
	Kind       string        `json:"kind"`
	Title      string        `json:"title"`
	AddedAt    string        `json:"added_at"`
	Streamable bool          `json:"streamable,omitempty"`
	Providers  []tonightProv `json:"providers,omitempty"`
}

type watchlistListOutput struct {
	Watchlist []watchlistRow `json:"watchlist"`
}

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watchlist",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Local SQLite watchlist (add, list, remove)",
		Long: `Manage a local SQLite-backed watchlist of movies and TV shows. The watchlist
lives at the same store as the rest of the CLI's local data (see --db).

Commands:
  add <id-or-title>     resolve and add a row
  list                  print the current watchlist
  remove <id>           remove by tmdb id

The watchlist is local-only — adding rows does not contact TMDb except to
resolve the title. Use --available to flag rows streamable on a provider.`,
	}
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	return cmd
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var flagKind string
	var flagDB string
	cmd := &cobra.Command{
		Use:         "add <id-or-title>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Resolve a title or id and add it to the local watchlist",
		Example: `  movie-goat-pp-cli watchlist add 27205 --kind movie
  movie-goat-pp-cli watchlist add "Inception"
  movie-goat-pp-cli watchlist add 1396 --kind tv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			kind := strings.ToLower(strings.TrimSpace(flagKind))
			if kind == "" {
				kind = "movie"
			}
			if kind != "movie" && kind != "tv" {
				return usageErr(fmt.Errorf("--kind must be \"movie\" or \"tv\", got %q", flagKind))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			var id int
			var title string
			switch kind {
			case "movie":
				id, _, err = resolveMovieID(c, query)
				if err != nil {
					return classifyAPIError(err)
				}
				detail, _, dErr := getMovieDetail(c, id, "")
				if dErr != nil {
					return classifyAPIError(dErr)
				}
				title = detail.Title
			case "tv":
				id, _, err = resolveTVID(c, query)
				if err != nil {
					return classifyAPIError(err)
				}
				detail, _, dErr := getTVDetail(c, id, "")
				if dErr != nil {
					return classifyAPIError(dErr)
				}
				title = detail.Name
			}
			s, err := openStoreForWatchlist(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer s.Close()
			already, _ := s.WatchlistContains(cmd.Context(), kind, id)
			if !already {
				if err := s.WatchlistAdd(cmd.Context(), store.WatchlistEntry{
					TMDBID:  id,
					Kind:    kind,
					Title:   title,
					AddedAt: time.Now().UTC(),
				}); err != nil {
					return err
				}
			}
			out := watchlistRow{TMDBID: id, Kind: kind, Title: title, AddedAt: time.Now().UTC().Format(time.RFC3339)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added: %s [%s] tmdb:%d\n", title, kind, id)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKind, "kind", "movie", "Kind: movie | tv")
	cmd.Flags().StringVar(&flagDB, "db", "", "Override SQLite path (defaults to the CLI's standard location)")
	return cmd
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	var flagAvailable bool
	var flagProviders string
	var flagRegion string
	var flagDB string
	cmd := &cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Print the local watchlist; --available flags streamable rows",
		Example: `  movie-goat-pp-cli watchlist list
  movie-goat-pp-cli watchlist list --available --providers Netflix,Hulu --region US --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForWatchlist(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.WatchlistList(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([]watchlistRow, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, watchlistRow{
					ID:      e.ID,
					TMDBID:  e.TMDBID,
					Kind:    e.Kind,
					Title:   e.Title,
					AddedAt: e.AddedAt.Format(time.RFC3339),
				})
			}

			if flagAvailable && len(rows) > 0 {
				region := strings.ToUpper(strings.TrimSpace(flagRegion))
				if region == "" {
					region = "US"
				}
				want := parseProviderList(flagProviders)
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
				defer cancel()
				type provHit struct {
					id    int
					kind  string
					provs []tonightProv
					match bool
				}
				results, errs := cliutil.FanoutRun(ctx, rows,
					func(r watchlistRow) string { return r.Title },
					func(_ context.Context, r watchlistRow) (provHit, error) {
						provs, match, perr := fetchProviderInfo(c, r.Kind, r.TMDBID, region, want)
						return provHit{id: r.TMDBID, kind: r.Kind, provs: provs, match: match}, perr
					})
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				byKey := make(map[string]provHit, len(results))
				for _, r := range results {
					byKey[fmt.Sprintf("%s-%d", r.Value.kind, r.Value.id)] = r.Value
				}
				for i := range rows {
					hit, ok := byKey[fmt.Sprintf("%s-%d", rows[i].Kind, rows[i].TMDBID)]
					if !ok {
						continue
					}
					rows[i].Streamable = hit.match
					rows[i].Providers = hit.provs
				}
			}

			out := watchlistListOutput{Watchlist: rows}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "Watchlist empty. Add titles with `movie-goat-pp-cli watchlist add <id-or-title>`.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "ID\tKind\tTitle\tAdded\tStreamable")
			for _, r := range rows {
				stream := "-"
				if flagAvailable {
					if r.Streamable {
						stream = "yes"
					} else {
						stream = "no"
					}
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", r.TMDBID, r.Kind, truncate(r.Title, 40), r.AddedAt, stream)
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagAvailable, "available", false, "Fetch /watch/providers for each row and flag streamable matches")
	cmd.Flags().StringVar(&flagProviders, "providers", "", "Provider names to match for --available (comma-separated)")
	cmd.Flags().StringVar(&flagRegion, "region", "US", "Region for /watch/providers lookup")
	cmd.Flags().StringVar(&flagDB, "db", "", "Override SQLite path")
	return cmd
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var flagKind string
	var flagDB string
	cmd := &cobra.Command{
		Use:         "remove <tmdb-id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Remove a watchlist row by TMDb id",
		Example: `  movie-goat-pp-cli watchlist remove 27205 --kind movie
  movie-goat-pp-cli watchlist remove 1396 --kind tv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			id, perr := strconv.Atoi(strings.TrimSpace(args[0]))
			if perr != nil {
				return usageErr(fmt.Errorf("remove requires a numeric tmdb id, got %q", args[0]))
			}
			kind := strings.ToLower(strings.TrimSpace(flagKind))
			if kind == "" {
				kind = "movie"
			}
			if kind != "movie" && kind != "tv" {
				return usageErr(fmt.Errorf("--kind must be \"movie\" or \"tv\", got %q", flagKind))
			}
			s, err := openStoreForWatchlist(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.WatchlistRemove(cmd.Context(), kind, id); err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": true, "tmdb_id": id, "kind": kind}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s tmdb:%d\n", kind, id)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKind, "kind", "movie", "Kind: movie | tv")
	cmd.Flags().StringVar(&flagDB, "db", "", "Override SQLite path")
	return cmd
}

// openStoreForWatchlist resolves the SQLite path from the explicit flag or the
// CLI's default location and opens the store with the command's context.
func openStoreForWatchlist(ctx context.Context, dbFlag string) (*store.Store, error) {
	dbPath := strings.TrimSpace(dbFlag)
	if dbPath == "" {
		dbPath = defaultDBPath("movie-goat-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening watchlist store: %w", err)
	}
	return s, nil
}
