// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// collectBaseURL is Techmeme's site; tests point it at a stub.
var collectBaseURL = "https://www.techmeme.com"

const collectMaxBody = 16 << 20

var collectDateRe = regexp.MustCompile(`^\d{6}$`)

// rssValidators are the RSS feed's conditional-GET validators.
type rssValidators struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

func newCollectCmd(flags *rootFlags) *cobra.Command {
	var source, date, statePath string
	var v rssValidators
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Print Techmeme headlines from the river, the RSS feed or one dated page as JSON lines, with publisher URLs",
		Long: `Print headlines as one JSON object per line, for a collector that keeps its
own copy: {id, title, publication, url, techmeme_permalink, published_at, date}.

url is always the publisher's own link, never a Techmeme redirect (/goto/,
/r/) or any techmeme.com page; a headline without one is skipped.
techmeme_permalink is the headline's Techmeme page. published_at is UTC
RFC 3339, or null on dated pages, which show no times. date is the day in
the headline's Techmeme id.

--source river reads /river (about five days). --source rss reads
/feed.xml, strips the "(Publication)" suffix from titles, and sends a
conditional GET: pass the validators of the last 200 with --etag and
--last-modified, or keep them in a file with --state, which is read before
the request and rewritten after a 200. A 304 prints nothing and exits 0.
--source day reads the dated snapshot /<YYMMDD>/h2000 for --date.

collect reads only those three pages; it never calls Techmeme's search,
which robots.txt disallows.`,
		Example: `  # The last five days of headlines
  techmeme-pp-cli collect --source river

  # New RSS items only, keeping the validators between runs
  techmeme-pp-cli collect --source rss --state ~/.techmeme-rss.json

  # Backfill one day
  techmeme-pp-cli collect --source day --date 261120`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := collectPath(source, date)
			if err != nil {
				return usageErr(err)
			}
			if statePath != "" && source == "rss" {
				saved, err := readValidators(statePath)
				if err != nil {
					return usageErr(err)
				}
				if v.ETag == "" {
					v.ETag = saved.ETag
				}
				if v.LastModified == "" {
					v.LastModified = saved.LastModified
				}
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "would GET %s%s\n", collectBaseURL, path)
				return nil
			}
			cond := rssValidators{}
			if source == "rss" {
				cond = v
			}
			body, got, notModified, err := collectFetch(cmd.Context(), path, cond, flags.timeout)
			if err != nil {
				return apiErr(err)
			}
			if notModified {
				return nil
			}
			var rows []collectRow
			switch source {
			case "river":
				rows = parseRiver(body)
			case "day":
				rows = parseDay(body)
			case "rss":
				if rows, err = parseRSS(body); err != nil {
					return apiErr(err)
				}
			}
			if len(rows) == 0 {
				return apiErr(fmt.Errorf("no headlines found at %s; Techmeme's page layout may have changed", path))
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, r := range rows {
				if err := enc.Encode(r); err != nil {
					return err
				}
			}
			if source == "rss" && statePath != "" {
				if err := writeValidators(statePath, got); err != nil {
					return fmt.Errorf("saving RSS validators: %w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Where to read: river | rss | day (required)")
	cmd.Flags().StringVar(&date, "date", "", "Day for --source day, as YYMMDD (e.g. 261120)")
	cmd.Flags().StringVar(&v.ETag, "etag", "", "RSS: ETag from the last 200, sent as If-None-Match")
	cmd.Flags().StringVar(&v.LastModified, "last-modified", "", "RSS: Last-Modified from the last 200, sent as If-Modified-Since")
	cmd.Flags().StringVar(&statePath, "state", "", "RSS: JSON file holding the validators; read before the request, rewritten after a 200")
	return cmd
}

// collectPath maps --source and --date to the one page collect reads.
func collectPath(source, date string) (string, error) {
	switch source {
	case "river":
		return "/river", nil
	case "rss":
		return "/feed.xml", nil
	case "day":
		if !collectDateRe.MatchString(date) {
			return "", fmt.Errorf("--source day needs --date as YYMMDD (e.g. 261120), got %q", date)
		}
		if _, err := time.Parse("060102", date); err != nil {
			return "", fmt.Errorf("--date %q is not a real day", date)
		}
		return "/" + date + "/h2000", nil
	case "":
		return "", errors.New("--source is required: river, rss or day")
	}
	return "", fmt.Errorf("invalid --source %q; valid: river, rss, day", source)
}

// collectFetch GETs one page. With validators it sends a conditional GET
// and reports a 304 as notModified. got holds the response's validators.
func collectFetch(ctx context.Context, path string, cond rssValidators, timeout time.Duration) (body []byte, got rssValidators, notModified bool, err error) {
	if strings.HasPrefix(path, "/search") {
		return nil, got, false, errors.New("collect never reads /search/, which Techmeme's robots.txt disallows")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, collectBaseURL+path, nil)
	if err != nil {
		return nil, got, false, err
	}
	req.Header.Set("User-Agent", "techmeme-pp-cli/"+version)
	if cond.ETag != "" {
		req.Header.Set("If-None-Match", cond.ETag)
	}
	if cond.LastModified != "" {
		req.Header.Set("If-Modified-Since", cond.LastModified)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, got, false, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, got, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, got, false, fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, collectMaxBody))
	if err != nil {
		return nil, got, false, fmt.Errorf("GET %s: %w", path, err)
	}
	got = rssValidators{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	return body, got, false, nil
}

// readValidators reads a --state file. A missing file means no validators.
func readValidators(path string) (rssValidators, error) {
	var v rssValidators
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, fmt.Errorf("reading --state: %w", err)
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("--state %s is not a validators file: %w", path, err)
	}
	return v, nil
}

// writeValidators replaces a --state file atomically.
func writeValidators(path string, v rssValidators) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".techmeme-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
