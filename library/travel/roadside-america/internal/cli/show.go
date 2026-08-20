package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/roadside"
	"github.com/spf13/cobra"
)

func newShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id-or-name>",
		Short: "Show the full writeup, address, and source for one attraction",
		Long: strings.Trim(`
Show the full writeup, address, directions, and source link for a single
attraction. Pass a numeric RoadsideAmerica.com id (e.g. 2055) to fetch live, or
a name that has already been seen by a previous 'state'/'near'/'search' to
resolve it from the local cache. Detail pages are cached on read (fresh-on-read,
30-day window).`, "\n"),
		Example: strings.Trim(`
  roadside-america-pp-cli show 2055
  roadside-america-pp-cli show 2055 --json
  roadside-america-pp-cli show "World's Largest Alligator"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch attraction detail")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an attraction id or name is required"))
			}
			input := strings.Join(args, " ")

			ctx := cmd.Context()
			s, err := openRoadsideStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			id := ""
			if isNumericID(input) {
				id = input
			} else {
				rid, rerr := s.ResolveByName(roadside.ResourceType, input, "name")
				if rerr != nil {
					return notFoundErr(fmt.Errorf("%q not found in local cache; pass a numeric id, or run 'state'/'near'/'search' first: %w", input, rerr))
				}
				id = rid
			}

			// Local-only: serve cached detail, then fall back to the list row.
			if flags.dataSource == "local" {
				if d, ok, _ := getCachedDetail(s, id); ok {
					return emitDetail(cmd, flags, d)
				}
				if a, ok := cachedAttraction(s, id); ok {
					d := roadside.Detail{Attraction: a, Writeup: "", Summary: "(run without --data-source local for the full writeup)"}
					return emitDetail(cmd, flags, d)
				}
				return notFoundErr(fmt.Errorf("attraction %s not in local cache", id))
			}

			// Auto: serve fresh cached detail without a network call.
			if flags.dataSource != "live" {
				if d, ok, fresh := getCachedDetail(s, id); ok && fresh {
					return emitDetail(cmd, flags, d)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			d, err := fetchDetail(ctx, c, id)
			if err != nil {
				if flags.dataSource == "auto" {
					if cd, ok, _ := getCachedDetail(s, id); ok {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: live fetch failed (%v); serving cached detail\n", err)
						return emitDetail(cmd, flags, cd)
					}
				}
				return err // fetchDetail already returns a typed not-found / api error
			}
			cacheDetail(s, d)
			return emitDetail(cmd, flags, d)
		},
	}
	return cmd
}

func isNumericID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cachedAttraction(s *storeHandle, id string) (roadside.Attraction, bool) {
	raw, err := s.Get(roadside.ResourceType, id)
	if err != nil {
		return roadside.Attraction{}, false
	}
	var a roadside.Attraction
	if json.Unmarshal(raw, &a) != nil || a.ID == "" {
		return roadside.Attraction{}, false
	}
	return a, true
}
