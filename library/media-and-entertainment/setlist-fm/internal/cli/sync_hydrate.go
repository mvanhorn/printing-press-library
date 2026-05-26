package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	// Extend sync with --artist and --user flags via a wrapper registered in root.go
}

func newSyncArtistCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "artist <name-or-mbid>",
		Short: "Sync all setlists for an artist into the local store",
		Long: `Fetches all setlists for the given artist from the Setlist.fm API and
populates the local SQLite database with artists, venues, cities, countries,
setlists, and songs. Subsequent analytics commands read from this local store.

Accepts either an artist name (resolved via search) or a MusicBrainz MBID.`,
		Example: `  # Sync by artist name
  setlist-fm-pp-cli sync artist "Radiohead"

  # Sync by MBID
  setlist-fm-pp-cli sync artist a74b1b7f-71a5-4011-9441-d0b5e4122711`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("setlist-fm-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			return syncArtistSetlists(c, db, args[0], maxPages)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum pages to fetch (0 = unlimited)")
	return cmd
}

func newSyncUserCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "user <userId>",
		Short:   "Sync a user's attended setlists into the local store",
		Example: `  setlist-fm-pp-cli sync user dave42`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("setlist-fm-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			return syncUserAttended(c, db, args[0])
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// resolveArtistMBID resolves an artist name to an MBID using the search API.
func resolveArtistMBID(c *client.Client, nameOrMBID string) (string, string, error) {
	// If it looks like an MBID (36 chars, 4 hyphens), use it directly
	if len(nameOrMBID) == 36 && countChar(nameOrMBID, '-') == 4 {
		// Fetch artist details to get the name
		data, err := c.Get("/1.0/artist/"+nameOrMBID, nil)
		if err != nil {
			return nameOrMBID, "", nil
		}
		var artist struct {
			MBID string `json:"mbid"`
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &artist) == nil {
			return artist.MBID, artist.Name, nil
		}
		return nameOrMBID, "", nil
	}

	// Search by name
	data, err := c.Get("/1.0/search/artists", map[string]string{
		"artistName": nameOrMBID,
		"sort":       "relevance",
	})
	if err != nil {
		return "", "", fmt.Errorf("searching for artist %q: %w", nameOrMBID, err)
	}

	var result struct {
		Artist []struct {
			MBID string `json:"mbid"`
			Name string `json:"name"`
		} `json:"artist"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", "", fmt.Errorf("parsing artist search response: %w", err)
	}
	if len(result.Artist) == 0 {
		return "", "", fmt.Errorf("no artist found matching %q", nameOrMBID)
	}

	return result.Artist[0].MBID, result.Artist[0].Name, nil
}

func countChar(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

// syncArtistSetlists paginates through all setlists for an artist and stores
// them with full hydration into dedicated tables.
func syncArtistSetlists(c *client.Client, db *store.Store, nameOrMBID string, maxPages int) error {
	mbid, name, err := resolveArtistMBID(c, nameOrMBID)
	if err != nil {
		return err
	}

	if name == "" {
		name = nameOrMBID
	}
	fmt.Fprintf(os.Stderr, "Syncing artist: %s (%s)\n", name, mbid)

	// Upsert the artist
	_ = db.UpsertArtist(mbid, name, "", "", "")

	page := 1
	totalSynced := 0
	totalSongs := 0

	for {
		data, err := c.Get("/1.0/artist/"+mbid+"/setlists", map[string]string{
			"p": strconv.Itoa(page),
		})
		if err != nil {
			return fmt.Errorf("fetching page %d: %w", page, err)
		}

		var resp struct {
			Setlist      []json.RawMessage `json:"setlist"`
			Total        int               `json:"total"`
			Page         int               `json:"page"`
			ItemsPerPage int               `json:"itemsPerPage"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("parsing setlists response: %w", err)
		}

		if len(resp.Setlist) == 0 {
			break
		}

		pageSongs := 0
		for _, raw := range resp.Setlist {
			songs, err := hydrateSetlist(db, raw, mbid, name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to hydrate setlist: %v\n", err)
				continue
			}
			pageSongs += songs
			totalSynced++
		}
		totalSongs += pageSongs

		totalPages := 1
		if resp.ItemsPerPage > 0 {
			totalPages = (resp.Total + resp.ItemsPerPage - 1) / resp.ItemsPerPage
		}
		fmt.Fprintf(os.Stderr, "  Page %d/%d: %d setlists (%d songs)\n", page, totalPages, len(resp.Setlist), pageSongs)

		if maxPages > 0 && page >= maxPages {
			break
		}
		if page >= totalPages {
			break
		}
		page++
	}

	fmt.Fprintf(os.Stderr, "Sync complete: %d setlists, %d songs for %s\n", totalSynced, totalSongs, name)
	return nil
}

// hydrateSetlist parses a raw setlist JSON and writes to all dedicated tables.
// Returns the number of songs stored.
func hydrateSetlist(db *store.Store, raw json.RawMessage, artistMBID, artistName string) (int, error) {
	var sl setlistJSON
	if err := json.Unmarshal(raw, &sl); err != nil {
		return 0, err
	}

	// Upsert venue and city if present
	if sl.Venue.ID != "" {
		lat := 0.0
		lon := 0.0
		cityID := ""
		cityName := ""
		stateName := ""
		stateCode := ""
		countryCode := ""
		countryName := ""
		if sl.Venue.City.ID != "" {
			cityID = sl.Venue.City.ID
			cityName = sl.Venue.City.Name
			stateName = sl.Venue.City.State
			stateCode = sl.Venue.City.StateCode
			if sl.Venue.City.Coords.Lat != 0 {
				lat = sl.Venue.City.Coords.Lat
				lon = sl.Venue.City.Coords.Long
			}
			if sl.Venue.City.Country.Code != "" {
				countryCode = sl.Venue.City.Country.Code
				countryName = sl.Venue.City.Country.Name
				_ = db.UpsertCountry(countryCode, countryName)
			}
			_ = db.UpsertCity(cityID, cityName, stateName, stateCode, countryCode, countryName, lat, lon)
		}
		_ = db.UpsertVenue(sl.Venue.ID, sl.Venue.Name, cityID, cityName, stateName, stateCode, countryCode, countryName, lat, lon, sl.Venue.URL)
	}

	// Count songs and encores
	songCount := 0
	encoreCount := 0
	for _, set := range sl.Sets.Set {
		songCount += len(set.Song)
		if set.Encore > 0 {
			encoreCount++
		}
	}

	// Venue info for the setlist
	venueID := sl.Venue.ID
	venueName := sl.Venue.Name
	cityName := ""
	countryCode := ""
	if sl.Venue.City.Name != "" {
		cityName = sl.Venue.City.Name
	}
	if sl.Venue.City.Country.Code != "" {
		countryCode = sl.Venue.City.Country.Code
	}

	tourName := ""
	if sl.Tour.Name != "" {
		tourName = sl.Tour.Name
	}

	// Use the artist info from the setlist if present, otherwise use what we have
	aName := artistName
	aMBID := artistMBID
	if sl.Artist.Name != "" {
		aName = sl.Artist.Name
	}
	if sl.Artist.MBID != "" {
		aMBID = sl.Artist.MBID
		_ = db.UpsertArtist(aMBID, aName, sl.Artist.SortName, sl.Artist.Disambiguation, sl.Artist.URL)
	}

	// Upsert setlist
	if err := db.UpsertSetlist(sl.ID, aMBID, aName, venueID, venueName, cityName, countryCode,
		sl.EventDate, tourName, sl.URL, sl.VersionID, sl.LastUpdated, songCount, encoreCount); err != nil {
		return 0, err
	}

	// Clear old songs and write new ones
	_ = db.ClearSongsForSetlist(sl.ID)

	songTotal := 0
	for setIdx, set := range sl.Sets.Set {
		isEncore := set.Encore > 0
		for pos, song := range set.Song {
			isCover := song.Cover.Name != ""
			coverName := song.Cover.Name
			coverMBID := song.Cover.MBID
			withName := song.With.Name
			withMBID := song.With.MBID
			isTape := song.Tape

			if err := db.UpsertSong(sl.ID, setIdx, pos, song.Name, song.Info,
				isCover, coverName, coverMBID, withName, withMBID, isTape, isEncore); err != nil {
				return songTotal, err
			}
			songTotal++
		}
	}

	return songTotal, nil
}

// setlistJSON maps the Setlist.fm API setlist response shape.
type setlistJSON struct {
	ID          string `json:"id"`
	VersionID   string `json:"versionId"`
	EventDate   string `json:"eventDate"`
	LastUpdated string `json:"lastUpdated"`
	Artist      struct {
		MBID           string `json:"mbid"`
		Name           string `json:"name"`
		SortName       string `json:"sortName"`
		Disambiguation string `json:"disambiguation"`
		URL            string `json:"url"`
	} `json:"artist"`
	Venue struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
		City struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			StateCode string `json:"stateCode"`
			State     string `json:"state"`
			Coords    struct {
				Lat  float64 `json:"lat"`
				Long float64 `json:"long"`
			} `json:"coords"`
			Country struct {
				Code string `json:"code"`
				Name string `json:"name"`
			} `json:"country"`
		} `json:"city"`
	} `json:"venue"`
	Tour struct {
		Name string `json:"name"`
	} `json:"tour"`
	Sets struct {
		Set []struct {
			Name   string `json:"name"`
			Encore int    `json:"encore"`
			Song   []struct {
				Name  string `json:"name"`
				Info  string `json:"info"`
				Cover struct {
					MBID string `json:"mbid"`
					Name string `json:"name"`
				} `json:"cover"`
				With struct {
					MBID string `json:"mbid"`
					Name string `json:"name"`
				} `json:"with"`
				Tape bool `json:"tape"`
			} `json:"song"`
		} `json:"set"`
	} `json:"sets"`
	URL string `json:"url"`
}

// syncUserAttended fetches a user's attended setlists and stores them.
func syncUserAttended(c *client.Client, db *store.Store, userID string) error {
	fmt.Fprintf(os.Stderr, "Syncing attended setlists for user: %s\n", userID)

	page := 1
	totalSynced := 0

	for {
		data, err := c.Get("/1.0/user/"+userID+"/attended", map[string]string{
			"p": strconv.Itoa(page),
		})
		if err != nil {
			return fmt.Errorf("fetching page %d: %w", page, err)
		}

		var resp struct {
			Setlist      []json.RawMessage `json:"setlist"`
			Total        int               `json:"total"`
			Page         int               `json:"page"`
			ItemsPerPage int               `json:"itemsPerPage"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("parsing attended response: %w", err)
		}

		if len(resp.Setlist) == 0 {
			break
		}

		for _, raw := range resp.Setlist {
			var sl setlistJSON
			if json.Unmarshal(raw, &sl) != nil {
				continue
			}
			// Hydrate the setlist
			_, _ = hydrateSetlist(db, raw, sl.Artist.MBID, sl.Artist.Name)
			// Record attendance
			_ = db.UpsertAttended(userID, sl.ID)
			totalSynced++
		}

		totalPages := 1
		if resp.ItemsPerPage > 0 {
			totalPages = (resp.Total + resp.ItemsPerPage - 1) / resp.ItemsPerPage
		}
		fmt.Fprintf(os.Stderr, "  Page %d/%d: %d setlists\n", page, totalPages, len(resp.Setlist))

		if page >= totalPages {
			break
		}
		page++
	}

	fmt.Fprintf(os.Stderr, "Sync complete: %d attended setlists for %s\n", totalSynced, userID)
	return nil
}
