// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/config"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
)

// newSvcClient builds an internal/svc.Client from the resolved config and the
// root flags. cookies come from config.AnkiwebCookies (ANKIWEB_COOKIES env or
// the config-file `cookies` key); public shared/* endpoints ignore them.
func (f *rootFlags) newSvcClient() (*svc.Client, *config.Config, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	return svc.New(cfg.BaseURL, cfg.AnkiwebCookies, f.timeout, f.rateLimit), cfg, nil
}

// newEditorClient builds a svc.Client for AnkiWeb's editor endpoints
// (/svc/editor/*). These are served from ankiuser.net and authenticate with
// that domain's session cookie, which is distinct from the ankiweb.net cookie
// (AnkiWeb issues a separate session per domain). It deliberately does NOT fall
// back to the ankiweb.net cookie — the editor rejects it with an HTTP 404.
func (f *rootFlags) newEditorClient() (*svc.Client, *config.Config, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	if strings.TrimSpace(cfg.AnkiuserCookies) == "" {
		return nil, cfg, authErr(errAuthEditor())
	}
	return svc.New("https://ankiuser.net", cfg.AnkiuserCookies, f.timeout, f.rateLimit), cfg, nil
}

// listDecks fetches and decodes the public shared-deck catalog for term.
// An empty term returns no decks (AnkiWeb returns 0 bytes).
func listDecks(ctx context.Context, c *svc.Client, term string) ([]svc.SharedDeck, error) {
	q := url.Values{}
	if term != "" {
		q.Set("search", term)
	}
	data, _, err := c.GetBytes(ctx, "/svc/shared/list-decks", q)
	if err != nil {
		return nil, err
	}
	return svc.DecodeListDecks(data)
}

// modifiedDate renders a unix-seconds modified timestamp as YYYY-MM-DD, or ""
// when zero.
func modifiedDate(unixSec int) string {
	if unixSec <= 0 {
		return ""
	}
	return time.Unix(int64(unixSec), 0).UTC().Format("2006-01-02")
}

// approvalPct renders a SharedDeck's approval rate as a percentage string.
func approvalPct(d svc.SharedDeck) string {
	if d.TotalVotes() == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", d.ApprovalRate()*100)
}

// sortByApproval sorts decks by approval rate descending, tie-breaking by total
// votes descending then title.
func sortByApproval(decks []svc.SharedDeck) {
	sort.SliceStable(decks, func(i, j int) bool {
		ai, aj := decks[i].ApprovalRate(), decks[j].ApprovalRate()
		if ai != aj {
			return ai > aj
		}
		if decks[i].TotalVotes() != decks[j].TotalVotes() {
			return decks[i].TotalVotes() > decks[j].TotalVotes()
		}
		return decks[i].Title < decks[j].Title
	})
}

// parseSinceDate parses a YYYY-MM-DD string into a unix-seconds threshold.
func parseSinceDate(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: expected YYYY-MM-DD", s)
	}
	return int(t.Unix()), nil
}
