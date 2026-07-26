// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/crate/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/crate/internal/crate"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// Paths are passed to the client relative. The generated client already
// prepends the configured base URL, so an absolute URL here produces
// "https://api.discogs.comhttps//api.discogs.com/...".

type crateHandle struct {
	store   *crate.Store
	closeFn func()
}

// openCrate opens the local collection database.
func openCrate(ctx context.Context) (crateHandle, error) {
	path := defaultDBPath("crate-pp-cli")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return crateHandle{}, configErr(fmt.Errorf("creating data directory: %w", err))
	}
	// busy_timeout matters because several crate commands hold a read while
	// the generated store may also have the file open.
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return crateHandle{}, configErr(fmt.Errorf("opening %s: %w", path, err))
	}
	st, err := crate.Open(ctx, db)
	if err != nil {
		db.Close()
		return crateHandle{}, configErr(err)
	}
	return crateHandle{store: st, closeFn: func() { db.Close() }}, nil
}

// loadShelf reads a user's synced records, syncing them first if this is the
// first time that user has been asked about.
//
// Auto-syncing rather than erroring is deliberate: every command here is about
// a specific person's shelf, so "run sync first" is a step the user would have
// to take every single time on a new machine. If the sync itself fails the
// error surfaces normally; what never happens is an empty list being mistaken
// for an empty collection.
func loadShelf(ctx context.Context, cmd *cobra.Command, c *client.Client, h crateHandle, user string, wanted bool) ([]crate.Record, error) {
	kind := "collection"
	// Escape the username: it lands in a URL path, and an unescaped one
	// containing "?" or "/" silently addresses a different endpoint whose
	// empty response would then be written over the local collection.
	esc := url.PathEscape(user)
	path := fmt.Sprintf("/users/%s/collection/folders/0/releases", esc)
	if wanted {
		kind, path = "wantlist", fmt.Sprintf("/users/%s/wants", esc)
	}

	if _, _, synced := h.store.SyncInfo(ctx, user, kind); !synced {
		if c == nil {
			return nil, usageErr(fmt.Errorf(
				"no %s synced for %q — run: crate-pp-cli shelf-sync --user %s", kind, user, user))
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "no local %s for %q yet; syncing it once...\n", kind, user)
		n, truncated, err := syncSide(ctx, c, h, user, path, wanted, defaultMaxPages)
		if err != nil {
			return nil, err
		}
		if truncated {
			// Say so. A silently capped sync makes every downstream count,
			// total, and ranking quietly describe part of the shelf.
			fmt.Fprintf(cmd.ErrOrStderr(),
				"warning: stopped after %d records (%d pages); this %s is larger than the auto-sync bound. Run: crate-pp-cli shelf-sync --user %s --max-pages 0\n",
				n, defaultMaxPages, kind, user)
		}
	}

	recs, err := h.store.Records(ctx, user, wanted)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, usageErr(fmt.Errorf("%s for %q is empty", kind, user))
	}
	return recs, nil
}

// defaultMaxPages bounds an automatic sync so a first run cannot silently
// spend the whole rate-limit budget on a huge collection.
const defaultMaxPages = 50

// pageFetch walks a paginated Discogs endpoint, collecting every page.
//
// maxPages bounds the walk so a 5,000-record collection cannot silently burn
// the whole rate-limit budget; the caller is told when the bound was hit
// rather than being handed a quietly truncated list.
func pageFetch(ctx context.Context, c *client.Client, path string, params map[string]string, maxPages int) ([]json.RawMessage, string, bool, error) {
	var out []json.RawMessage
	var itemsKey string
	truncated := false

	for page := 1; ; page++ {
		if maxPages > 0 && page > maxPages {
			truncated = true
			break
		}
		p := map[string]string{"per_page": "100", "page": strconv.Itoa(page)}
		for k, v := range params {
			p[k] = v
		}
		raw, err := c.Get(ctx, path, p)
		if err != nil {
			return nil, "", false, apiErr(err)
		}
		var env struct {
			Pagination struct {
				Page  int `json:"page"`
				Pages int `json:"pages"`
			} `json:"pagination"`
			Releases []json.RawMessage `json:"releases"`
			Wants    []json.RawMessage `json:"wants"`
			Results  []json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, "", false, apiErr(fmt.Errorf("parsing page %d: %w", page, err))
		}
		switch {
		case len(env.Releases) > 0:
			out, itemsKey = append(out, env.Releases...), "releases"
		case len(env.Wants) > 0:
			out, itemsKey = append(out, env.Wants...), "wants"
		case len(env.Results) > 0:
			out, itemsKey = append(out, env.Results...), "results"
		case env.Pagination.Pages == 0 && page == 1:
			// A 200 carrying none of the expected collections is not an empty
			// collection — it is an unrecognised response. Returning empty
			// here lets a sync DELETE a good local shelf and record the wipe
			// as a success, after which nothing re-syncs because a sync row
			// now exists.
			return nil, "", false, apiErr(fmt.Errorf(
				"unrecognised response from %s: no releases, wants, or results field", path))
		}
		if env.Pagination.Pages <= page {
			break
		}
	}
	return out, itemsKey, truncated, nil
}

// basicInfo is the shape Discogs nests inside collection and wantlist rows.
type basicInfo struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Year    int    `json:"year"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Formats []struct {
		Name string `json:"name"`
		// Descriptions carries LP, Album, 7", Reissue and similar. The help
		// advertises --format LP, which only ever matches from here.
		Descriptions []string `json:"descriptions"`
	} `json:"formats"`
	Genres []string `json:"genres"`
	Styles []string `json:"styles"`
}

func names(v []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(v))
	for _, x := range v {
		if x.Name != "" {
			out = append(out, x.Name)
		}
	}
	return out
}

// decodeRecord turns one collection or wantlist row into a Record.
func decodeRecord(raw json.RawMessage, wanted bool) (crate.Record, bool) {
	var row struct {
		ID        int64     `json:"id"`
		Rating    int       `json:"rating"`
		DateAdded string    `json:"date_added"`
		Basic     basicInfo `json:"basic_information"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return crate.Record{}, false
	}
	b := row.Basic
	id := b.ID
	if id == 0 {
		id = row.ID
	}
	if id == 0 {
		return crate.Record{}, false
	}
	added, _ := time.Parse(time.RFC3339, row.DateAdded)
	formats := make([]string, 0, len(b.Formats)*2)
	for _, f := range b.Formats {
		if f.Name != "" {
			formats = append(formats, f.Name)
		}
		formats = append(formats, f.Descriptions...)
	}
	return crate.Record{
		ReleaseID: id, Title: b.Title, Year: b.Year,
		Artists: names(b.Artists), Labels: names(b.Labels), Formats: formats,
		Genres: b.Genres, Styles: b.Styles,
		Rating: row.Rating, DateAdded: added, Wanted: wanted,
	}, true
}

// fetchPrice reads current marketplace stats for one release.
//
// A release with nothing for sale returns HasPrice false rather than a zero
// price: totalling absent listings as zero would understate every floor.
func fetchPrice(ctx context.Context, c *client.Client, releaseID int64, currency string) (crate.Price, error) {
	params := map[string]string{}
	if currency != "" {
		params["curr_abbr"] = currency
	}
	raw, err := c.Get(ctx, fmt.Sprintf("/marketplace/stats/%d", releaseID), params)
	if err != nil {
		return crate.Price{}, err
	}
	var out struct {
		NumForSale  int `json:"num_for_sale"`
		LowestPrice *struct {
			Value    float64 `json:"value"`
			Currency string  `json:"currency"`
		} `json:"lowest_price"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return crate.Price{}, err
	}
	p := crate.Price{ReleaseID: releaseID, NumForSale: out.NumForSale, FetchedAt: time.Now().UTC()}
	if out.LowestPrice != nil && out.NumForSale > 0 {
		p.Lowest, p.Currency, p.HasPrice = out.LowestPrice.Value, out.LowestPrice.Currency, true
	}
	return p, nil
}

// userFlagHelp is shared so every command describes --user identically.
const userFlagHelp = "Discogs username whose collection to use (case-sensitive)"

// resolveUser returns the username to operate on.
func resolveUser(user string) (string, error) {
	if user == "" {
		return "", usageErr(fmt.Errorf("name a Discogs user with --user"))
	}
	return user, nil
}

// priceRecords prices a bounded set of releases, reusing the cache first.
//
// Returns the prices it has, how many it fetched live, and how many it left
// unpriced. Callers must report the unpriced count: a total computed over half
// a collection is not a total, and silently rounding it up to one would be the
// most misleading thing this CLI could do.
func priceRecords(ctx context.Context, cmd *cobra.Command, c *client.Client, h crateHandle,
	recs []crate.Record, limit int, currency string, maxAge time.Duration) (map[int64]crate.Price, int, int, error) {

	cached, err := h.store.Prices(ctx, maxAge, currency)
	if err != nil {
		return nil, 0, 0, err
	}
	prices := map[int64]crate.Price{}
	var todo []crate.Record
	for _, r := range recs {
		if p, ok := cached[r.ReleaseID]; ok {
			prices[r.ReleaseID] = p
			continue
		}
		todo = append(todo, r)
	}

	fetched := 0
	for _, r := range todo {
		if limit > 0 && fetched >= limit {
			break
		}
		p, err := fetchPrice(ctx, c, r.ReleaseID, currency)
		p.ReqCurrency = currency
		if err != nil {
			// Stop on the first failure rather than grinding through a
			// rate-limit wall; what we have is still reportable.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped pricing after %d releases: %v\n", fetched, err)
			break
		}
		if err := h.store.PutPrice(ctx, p); err != nil {
			return nil, 0, 0, err
		}
		prices[r.ReleaseID] = p
		fetched++
	}

	unpriced := 0
	for _, r := range recs {
		if _, ok := prices[r.ReleaseID]; !ok {
			unpriced++
		}
	}
	return prices, fetched, unpriced, nil
}
