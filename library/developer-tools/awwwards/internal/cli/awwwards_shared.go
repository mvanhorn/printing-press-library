// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared plumbing for the hand-written awwwards design-data commands.
package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/store"
)

const mirrorHint = "run: awwwards-pp-cli mirror --pages 5 --details"

// openMirror opens (or creates) the local store and ensures the typed
// awwwards tables exist.
func openMirror(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.EnsureAwwwardsTables(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// requireMirror opens the mirror for a local-data command. When the mirror is
// missing or empty it prints the standard hint to stderr, emits the documented
// empty-mirror sentinel `[]` for machine consumers (NOT the command's usual
// object shape - agents should treat a top-level array as "mirror empty"),
// and returns (nil, true) so the caller can return nil cleanly.
func requireMirror(cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool) {
	ctx := cmd.Context()
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local design mirror at %s\n%s\n", dbPath, mirrorHint)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, true
	}
	db, err := openMirror(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "cannot open local design mirror: %v\n%s\n", err, mirrorHint)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, true
	}
	if !db.HasAwwwardsMirror(ctx) {
		_ = db.Close()
		fmt.Fprintf(cmd.ErrOrStderr(), "local design mirror is empty\n%s\n", mirrorHint)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, true
	}
	return db, false
}

// rejectLiveDataSource enforces `// pp:data-source local` semantics.
func rejectLiveDataSource(flags *rootFlags, command string) error {
	if flags.dataSource == "live" {
		return usageErr(fmt.Errorf("%s reads the local design mirror and has no live equivalent; drop --data-source live (refresh the mirror with 'awwwards-pp-cli mirror')", command))
	}
	return nil
}

// fetchListingCards GETs one listing page and parses its embedded cards.
// filter is a /websites/<filter>/ segment ("" for the main feed).
func fetchListingCards(ctx context.Context, c *client.Client, filter string, page int, text string) ([]awwwards.Card, error) {
	path := "/websites/"
	if filter != "" {
		path = "/websites/" + url.PathEscape(strings.TrimSuffix(strings.TrimPrefix(filter, "/"), "/")) + "/"
	}
	params := map[string]string{}
	if page > 1 {
		params["page"] = strconv.Itoa(page)
	}
	if text != "" {
		params["text"] = text
	}
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return awwwards.ParseCards(string(raw)), nil
}

// fetchDetail GETs and parses one /sites/<slug> page.
func fetchDetail(ctx context.Context, c *client.Client, slug string) (awwwards.Detail, error) {
	raw, err := c.Get(ctx, "/sites/"+url.PathEscape(slug), nil)
	if err != nil {
		return awwwards.Detail{}, err
	}
	return awwwards.ParseDetail(slug, string(raw)), nil
}

// cardView is the agent-facing card shape shared by latest/find output.
type cardView struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	CreatedAt    int64    `json:"created_at"`
	Tags         []string `json:"tags"`
	ThumbnailURL string   `json:"thumbnail_url"`
	DetailURL    string   `json:"detail_url"`
	ScoreOverall *float64 `json:"score_overall,omitempty"`
}

func cardToView(c awwwards.Card) cardView {
	return cardView{
		Slug:         c.Slug,
		Title:        c.Title,
		CreatedAt:    c.CreatedAt,
		Tags:         c.Tags,
		ThumbnailURL: awwwards.ThumbnailURL(c.Thumbnail(), ""),
		DetailURL:    "https://www.awwwards.com/sites/" + c.Slug,
	}
}

// queryStrings runs a single-column string query and returns the fully
// drained result, honoring the single-connection drain-first discipline.
func queryStrings(ctx context.Context, db *store.Store, q string, args ...any) ([]string, error) {
	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying: %w", err)
	}
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing rows: %w", err)
	}
	return out, nil
}

// scoreColumn whitelists the --dim flag values against SQL column names.
func scoreColumn(dim string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(dim)) {
	case "", "overall":
		return "score_overall", nil
	case "design":
		return "score_design", nil
	case "usability":
		return "score_usability", nil
	case "creativity":
		return "score_creativity", nil
	case "content":
		return "score_content", nil
	}
	return "", usageErr(fmt.Errorf("invalid --dim %q: want design, usability, creativity, content, or overall", dim))
}
