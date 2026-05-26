package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/internal/store"

	"github.com/spf13/cobra"
)

// newArtistCmd creates the 'artist' parent command with human-friendly subcommands.
func newArtistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artist",
		Short: "Look up artists, resolve names to MBIDs, and view setlists",
	}

	cmd.AddCommand(newArtistGetCmd(flags))
	cmd.AddCommand(newArtistSetlistsCmd(flags))
	cmd.AddCommand(newArtistResolveCmd(flags))
	return cmd
}

func newArtistGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name-or-mbid>",
		Short: "Get artist details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			mbid, _, err := resolveArtistMBID(c, args[0])
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/artist/"+mbid, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newArtistSetlistsCmd(flags *rootFlags) *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "setlists <name-or-mbid>",
		Short: "List setlists for an artist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			mbid, _, err := resolveArtistMBID(c, args[0])
			if err != nil {
				return err
			}
			params := map[string]string{}
			if page > 0 {
				params["p"] = fmt.Sprintf("%d", page)
			}
			data, err := c.Get("/1.0/artist/"+mbid+"/setlists", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	return cmd
}

func newArtistResolveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <name>",
		Short: "Resolve an artist name to a MusicBrainz MBID",
		Example: `  # Resolve an artist name
  setlist-fm-pp-cli artist resolve "Radiohead"

  # Resolve with JSON output
  setlist-fm-pp-cli artist resolve "Phoenix" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/search/artists", map[string]string{
				"artistName": args[0],
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

// newSetlistCmd creates the 'setlist' parent command.
func newSetlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setlist",
		Short: "Look up individual setlists and versions",
	}
	cmd.AddCommand(newSetlistGetCmd(flags))
	cmd.AddCommand(newSetlistVersionCmd(flags))
	return cmd
}

func newSetlistGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <setlistId>",
		Short: "Get a specific setlist by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/setlist/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newSetlistVersionCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version <versionId>",
		Short: "Get a setlist version by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/setlist/version/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

// newVenueCmd creates the 'venue' parent command.
func newVenueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "venue",
		Short: "Look up venues and their setlists",
	}
	cmd.AddCommand(newVenueGetCmd(flags))
	cmd.AddCommand(newVenueSetlistsCmd(flags))
	return cmd
}

func newVenueGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <venueId>",
		Short: "Get venue details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/venue/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newVenueSetlistsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "setlists <venueId>",
		Short: "List setlists at a venue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/venue/"+args[0]+"/setlists", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

// newCityCmd creates the 'city' parent command.
func newCityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "city",
		Short: "Look up city details",
	}
	cmd.AddCommand(newCityGetCmd(flags))
	return cmd
}

func newCityGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <geoId>",
		Short: "Get city details by GeoNames ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/city/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

// newUserCmd creates the 'user' parent command.
func newUserCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Look up users and their setlists",
	}
	cmd.AddCommand(newUserGetCmd(flags))
	cmd.AddCommand(newUserAttendedCmd(flags))
	cmd.AddCommand(newUserEditedCmd(flags))
	return cmd
}

func newUserGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <userId>",
		Short: "Get user details",
		Example: `  # Look up a user profile
  setlist-fm-pp-cli user get "davemorin"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/user/"+args[0], nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newUserAttendedCmd(flags *rootFlags) *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "attended <userId>",
		Short: "List setlists a user has attended",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if page > 0 {
				params["p"] = fmt.Sprintf("%d", page)
			}
			data, err := c.Get("/1.0/user/"+args[0]+"/attended", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	return cmd
}

func newUserEditedCmd(flags *rootFlags) *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "edited <userId>",
		Short: "List setlists a user has edited",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if page > 0 {
				params["p"] = fmt.Sprintf("%d", page)
			}
			data, err := c.Get("/1.0/user/"+args[0]+"/edited", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	return cmd
}

// newSearchCmd creates the 'search' parent command.
func newSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search artists, venues, cities, countries, and setlists",
	}
	cmd.AddCommand(newSearchArtistsCmd(flags))
	cmd.AddCommand(newSearchVenuesCmd(flags))
	cmd.AddCommand(newSearchCitiesCmd(flags))
	cmd.AddCommand(newSearchCountriesCmd(flags))
	cmd.AddCommand(newSearchSetlistsCmd(flags))
	return cmd
}

func newSearchArtistsCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "artists",
		Short: "Search for artists",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if name != "" {
				params["artistName"] = name
			}
			data, err := c.Get("/1.0/search/artists", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Artist name to search for")
	return cmd
}

func newSearchVenuesCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "venues",
		Short: "Search for venues",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if name != "" {
				params["name"] = name
			}
			data, err := c.Get("/1.0/search/venues", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Venue name to search for")
	return cmd
}

func newSearchCitiesCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "cities",
		Short: "Search for cities",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if name != "" {
				params["name"] = name
			}
			data, err := c.Get("/1.0/search/cities", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "City name to search for")
	return cmd
}

func newSearchCountriesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "countries",
		Short: "List all countries",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/1.0/search/countries", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
}

func newSearchSetlistsCmd(flags *rootFlags) *cobra.Command {
	var artist, year, venueName, cityName string
	cmd := &cobra.Command{
		Use:   "setlists",
		Short: "Search for setlists",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if artist != "" {
				params["artistName"] = artist
			}
			if year != "" {
				params["year"] = year
			}
			if venueName != "" {
				params["venueName"] = venueName
			}
			if cityName != "" {
				params["cityName"] = cityName
			}
			data, err := c.Get("/1.0/search/setlists", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&artist, "artist", "", "Artist name")
	cmd.Flags().StringVar(&year, "year", "", "Year (YYYY)")
	cmd.Flags().StringVar(&venueName, "venue", "", "Venue name")
	cmd.Flags().StringVar(&cityName, "city", "", "City name")
	return cmd
}

// openStoreOrFail opens the local store, failing with a helpful message if unavailable.
func openStoreOrFail(cmd *cobra.Command) (*store.Store, error) {
	dbPath := defaultDBPath("setlist-fm-pp-cli")
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'setlist-fm-pp-cli sync artist <name>' first", err)
	}
	return db, nil
}

// resolveArtistFromStore resolves an artist name or MBID from the local store.
func resolveArtistFromStore(db *store.Store, nameOrMBID string) (string, string, error) {
	mbid, name, err := db.ResolveArtist(nameOrMBID)
	if err != nil {
		return "", "", err
	}
	return mbid, name, nil
}

// outputJSON produces JSON output for a value.
func outputJSON(cmd *cobra.Command, v any, flags *rootFlags) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(raw), flags)
}
