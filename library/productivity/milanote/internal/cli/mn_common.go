// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

// mnClient builds the hand-written Milanote client from the same config and
// credentials the generated commands use.
func mnClient(flags *rootFlags) (*milanote.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	cred := cfg.CookieCredential()
	if strings.TrimSpace(cred) == "" {
		return nil, authErr(errors.New("not logged in to Milanote: run 'milanote-pp-cli chrome-login' (or 'auth login'), or set MILANOTE_SESSION_TOKEN"))
	}
	opts, err := mnOptions(flags, cfg)
	if err != nil {
		return nil, err
	}
	c, err := milanote.New(cfg.BaseURL, cred, opts)
	if err != nil {
		return nil, mnErr(err)
	}
	return c, nil
}

// mnOptions applies the global --timeout, --rate-limit, --ca-cert and
// --tls-server-name settings to the hand-written client.
func mnOptions(flags *rootFlags, cfg *config.Config) (milanote.Options, error) {
	flags.applyTLSFlags(cfg)
	var transport http.RoundTripper
	if strings.TrimSpace(cfg.CACert) != "" || strings.TrimSpace(cfg.TLSServerName) != "" {
		hc, err := client.NewPlainHTTPClient(flags.timeout, cfg)
		if err != nil {
			return milanote.Options{}, configErr(err)
		}
		transport = hc.Transport
	}
	return milanote.Options{Timeout: flags.timeout, RateLimit: flags.rateLimit, Transport: transport}, nil
}

// mnErr maps Milanote errors onto the CLI's typed exit codes.
func mnErr(err error) error {
	if err == nil {
		return nil
	}
	var ce *cliError
	if errors.As(err, &ce) {
		return err
	}
	var rl *cliutil.RateLimitError
	switch {
	case errors.Is(err, milanote.ErrUnauthorized):
		return authErr(err)
	case errors.As(err, &rl):
		return rateLimitErr(err)
	case milanote.IsNotFound(err), errors.Is(err, milanote.ErrNoElement):
		return notFoundErr(err)
	case errors.Is(err, context.DeadlineExceeded):
		return apiErr(fmt.Errorf("timed out talking to Milanote (raise --timeout): %w", err))
	}
	return apiErr(err)
}

var boardURLID = regexp.MustCompile(`milanote\.com/([0-9A-Za-z]{10,})`)

// boardRef accepts a board id or a pasted app.milanote.com URL.
func boardRef(s string) string {
	s = strings.TrimSpace(s)
	if m := boardURLID.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return s
}

// loadBoard fetches one board and maps a missing board to exit code 3.
func loadBoard(ctx context.Context, c *milanote.Client, id string) (*milanote.BoardPayload, error) {
	p, err := c.Board(ctx, boardRef(id))
	if err != nil {
		return nil, mnErr(err)
	}
	return p, nil
}

// emit prints v as JSON for machine callers and calls human otherwise.
func emit(cmd *cobra.Command, flags *rootFlags, v any, human func(w io.Writer) error) error {
	w := cmd.OutOrStdout()
	if human != nil && wantsHumanTable(w, flags) {
		return human(w)
	}
	return printJSONFiltered(w, v, flags)
}

// needArg returns a usage error naming the missing argument.
func needArg(cmd *cobra.Command, what string) error {
	return usageErr(fmt.Errorf("missing %s\n\n%s", what, cmd.UseLine()))
}

// boardURL is the browser link for a board.
func boardURL(id string) string { return milanote.DefaultBaseURL + "/" + id }

// placeFlags are the shared "where does the new card go" flags.
type placeFlags struct {
	x, y     float64
	column   string
	after    string
	unsorted bool
	gutter   float64
	width    float64
	cmd      *cobra.Command
}

func (p *placeFlags) bind(cmd *cobra.Command) {
	p.cmd = cmd
	cmd.Flags().Float64Var(&p.x, "x", 0, "Canvas x position in grid units (1 unit = 9px); with --y places the card exactly")
	cmd.Flags().Float64Var(&p.y, "y", 0, "Canvas y position in grid units")
	cmd.Flags().StringVar(&p.column, "column", "", "Put the card at the bottom of this column (element id)")
	cmd.Flags().StringVar(&p.after, "after", "", "Put the card directly below this element, keeping the gutter")
	cmd.Flags().BoolVar(&p.unsorted, "unsorted", false, "Put the card in the board's Unsorted panel")
	cmd.Flags().Float64Var(&p.gutter, "gutter", milanote.DefaultGutter, "Gap in grid units when placing below other cards")
	cmd.Flags().Float64Var(&p.width, "width", 0, "Card width in grid units (default 34 = 304px)")
}

// exact reports whether the caller pinned x/y.
func (p *placeFlags) exact() bool {
	return p.cmd != nil && (p.cmd.Flags().Changed("x") || p.cmd.Flags().Changed("y"))
}

func (p *placeFlags) validate() error {
	n := 0
	for _, set := range []bool{p.exact(), p.column != "", p.after != "", p.unsorted} {
		if set {
			n++
		}
	}
	if n > 1 {
		return usageErr(errors.New("choose one of --x/--y, --column, --after or --unsorted"))
	}
	if p.width < 0 || p.gutter < 0 {
		return usageErr(errors.New("--width and --gutter must not be negative"))
	}
	return nil
}

// resolve picks the location for a new element on boardID.
func (p *placeFlags) resolve(payload *milanote.BoardPayload, boardID string) (milanote.Location, error) {
	switch {
	case p.column != "":
		col, err := milanote.Find(payload, p.column)
		if err != nil {
			return milanote.Location{}, notFoundErr(fmt.Errorf("column %s: %w", p.column, err))
		}
		if col.ElementType != milanote.TypeColumn {
			return milanote.Location{}, usageErr(fmt.Errorf("%s is a %s, not a column", col.ID, strings.ToLower(col.ElementType)))
		}
		return milanote.ColumnEnd(payload, col.ID), nil
	case p.unsorted:
		return milanote.Unsorted(payload, boardID), nil
	case p.after != "":
		el, err := milanote.Find(payload, p.after)
		if err != nil {
			return milanote.Location{}, notFoundErr(fmt.Errorf("--after %s: %w", p.after, err))
		}
		if el.Location.Section == milanote.SectionInbox {
			return afterInList(payload, el), nil
		}
		b := milanote.BoxOf(el)
		return milanote.CanvasAt(boardID, b.X, b.Bottom()+p.gutter, milanote.MaxScore(payload, boardID)+milanote.ScoreStep), nil
	case p.exact():
		return milanote.CanvasAt(boardID, milanote.Snap(p.x), milanote.Snap(p.y), milanote.MaxScore(payload, boardID)+milanote.ScoreStep), nil
	}
	x, y := milanote.FreeOrigin(milanote.CanvasBoxes(payload, boardID), p.gutter)
	return milanote.CanvasAt(boardID, x, y, milanote.MaxScore(payload, boardID)+milanote.ScoreStep), nil
}

// afterInList slots a new element right after el inside its column or the
// unsorted panel, scoring it between el and its next sibling.
func afterInList(payload *milanote.BoardPayload, el *milanote.Element) milanote.Location {
	kids := payload.Children(el.Location.ParentID, milanote.SectionInbox)
	score := el.Location.Score() + milanote.ScoreStep
	for i, k := range kids {
		if k.ID == el.ID && i+1 < len(kids) {
			score = (el.Location.Score() + kids[i+1].Location.Score()) / 2
		}
	}
	return milanote.ListAt(el.Location.ParentID, int(el.Location.Index())+1, score)
}

// withWidth sets a card width when one was asked for.
func (p *placeFlags) withWidth(content map[string]any) map[string]any {
	if p.width > 0 {
		content["width"] = milanote.Snap(p.width)
	}
	return content
}

// readTextArg returns the text argument, reading stdin for "-" and a file
// for "@path".
func readTextArg(cmd *cobra.Command, s string) (string, error) {
	switch {
	case s == "-":
		b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 4<<20))
		return strings.TrimRight(string(b), "\n"), err
	case strings.HasPrefix(s, "@") && len(s) > 1:
		b, err := readLocalFile(s[1:])
		if err != nil {
			return "", usageErr(err)
		}
		return strings.TrimRight(string(b), "\n"), nil
	}
	return s, nil
}

// elementRow is the compact JSON shape returned for created or listed
// elements.
type elementRow struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	BoardID string   `json:"board_id,omitempty"`
	Parent  string   `json:"parent_id,omitempty"`
	Section string   `json:"section,omitempty"`
	X       *float64 `json:"x,omitempty"`
	Y       *float64 `json:"y,omitempty"`
	Index   *float64 `json:"index,omitempty"`
	Width   float64  `json:"width,omitempty"`
	Height  float64  `json:"height_estimate,omitempty"`
	Color   string   `json:"color,omitempty"`
	Text    string   `json:"text,omitempty"`
	URL     string   `json:"board_url,omitempty"`
}

func rowOf(el *milanote.Element, boardID string) elementRow {
	r := elementRow{ID: el.ID, Type: strings.ToLower(el.ElementType), BoardID: boardID,
		Parent: el.Location.ParentID, Section: strings.ToLower(el.Location.Section), Color: el.Color(), Text: el.Text()}
	if _, ok := el.Location.Position["x"]; ok {
		x, y := el.Location.X(), el.Location.Y()
		r.X, r.Y = &x, &y
	}
	if _, ok := el.Location.Position["index"]; ok {
		i := el.Location.Index()
		r.Index = &i
	}
	if el.Location.Section != milanote.SectionTrash {
		b := milanote.BoxOf(el)
		r.Width, r.Height = b.W, b.H
	}
	if el.ElementType == milanote.TypeBoard {
		r.URL = boardURL(el.ID)
	}
	return r
}

// createdRow describes an element the CLI just made.
func createdRow(id, elementType, boardID string, loc milanote.Location, content map[string]any) elementRow {
	el := &milanote.Element{ID: id, ElementType: elementType, Location: loc, Content: content}
	return rowOf(el, boardID)
}

func printCreated(cmd *cobra.Command, flags *rootFlags, row elementRow) error {
	return emit(cmd, flags, row, func(w io.Writer) error {
		where := "unsorted"
		if row.X != nil {
			where = fmt.Sprintf("x=%g y=%g", *row.X, *row.Y)
		} else if row.Index != nil {
			where = fmt.Sprintf("position %g in %s", *row.Index+1, row.Parent)
		}
		_, err := fmt.Fprintf(w, "%s %s added at %s on %s\n", green("✓"), row.Type, where, boardURL(row.BoardID))
		if err == nil {
			_, err = fmt.Fprintf(w, "  id: %s\n", row.ID)
		}
		return err
	})
}

// jsonClone deep-copies a JSON-shaped value.
func jsonClone(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
