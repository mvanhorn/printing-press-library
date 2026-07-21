package namethatui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
)

const maxPageBytes = 4 << 20

func Sync(ctx context.Context, client *http.Client, baseURL string, db *store.Store, includeComponents, includeStyles bool) (Report, error) {
	if client == nil {
		client = http.DefaultClient
	}
	baseURL = strings.TrimRight(baseURL, "/")
	report := Report{}
	var componentRecords, styleRecords []store.AuthoritativeRecord
	if includeComponents {
		body, err := fetch(ctx, client, baseURL+"/")
		if err != nil {
			return report, fmt.Errorf("fetching NameThatUI components: %w", err)
		}
		xs, err := ParseComponents(body, baseURL)
		if err != nil {
			return report, err
		}
		componentRecords, err = encodeComponentRecords(xs)
		if err != nil {
			return report, err
		}
	}
	if includeStyles {
		body, err := fetch(ctx, client, baseURL+"/styles")
		if err != nil {
			return report, fmt.Errorf("fetching NameThatUI styles: %w", err)
		}
		xs, err := ParseStylesIndex(body, baseURL)
		if err != nil {
			return report, err
		}
		for i := range xs {
			detail, err := fetch(ctx, client, xs[i].SourceURL)
			if err != nil {
				return report, fmt.Errorf("fetching style %s: %w", xs[i].Slug, err)
			}
			xs[i], err = ParseStylePage(detail, xs[i])
			if err != nil {
				return report, fmt.Errorf("parsing style %s: %w", xs[i].Slug, err)
			}
		}
		styleRecords, err = encodeStyleRecords(xs)
		if err != nil {
			return report, err
		}
	}
	// Do no local writes until every requested authoritative page has been
	// fetched and parsed. Each resource then lands atomically with its current
	// mirror, immutable history, reconciliation, and sync state.
	if includeComponents {
		if err := db.ApplyAuthoritativeSync("components", "component_snapshots", "", componentRecords); err != nil {
			return report, fmt.Errorf("storing components: %w", err)
		}
		report.Components = len(componentRecords)
	}
	if includeStyles {
		if err := db.ApplyAuthoritativeSync("style_details", "style_snapshots", "", styleRecords); err != nil {
			return report, fmt.Errorf("storing styles: %w", err)
		}
		report.Styles = len(styleRecords)
	}
	return report, nil
}

func encodeComponentRecords(items []Component) ([]store.AuthoritativeRecord, error) {
	records := make([]store.AuthoritativeRecord, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("encoding component %s: %w", item.ID, err)
		}
		records = append(records, store.AuthoritativeRecord{ID: item.ID, Data: raw, SourceURL: item.SourceURL})
	}
	return records, nil
}

func encodeStyleRecords(items []Style) ([]store.AuthoritativeRecord, error) {
	records := make([]store.AuthoritativeRecord, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("encoding style %s: %w", item.ID, err)
		}
		records = append(records, store.AuthoritativeRecord{ID: item.ID, Data: raw, SourceURL: item.SourceURL})
	}
	return records, nil
}

func rscTexts(page []byte) []string {
	s, out := string(page), []string{}
	for {
		i := strings.Index(s, "self.__next_f.push(")
		if i < 0 {
			return out
		}
		s = s[i+len("self.__next_f.push("):]
		if s == "" {
			return out
		}
		dec := json.NewDecoder(bytes.NewBufferString(s))
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			s = s[1:]
			continue
		}
		var a []json.RawMessage
		if json.Unmarshal(raw, &a) == nil && len(a) > 1 {
			var text string
			if json.Unmarshal(a[1], &text) == nil {
				out = append(out, text)
			}
		}
		s = s[dec.InputOffset():]
	}
}

func balancedAfter(s, marker string) ([]byte, bool) {
	i := strings.Index(s, marker)
	if i < 0 {
		return nil, false
	}
	s = s[i+len(marker):]
	if len(s) == 0 || s[0] != '[' {
		return nil, false
	}
	depth, quote, escape := 0, false, false
	for i := range s {
		c := s[i]
		if quote {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quote = false
			}
			continue
		}
		if c == '"' {
			quote = true
		} else if c == '[' {
			depth++
		} else if c == ']' {
			depth--
			if depth == 0 {
				return []byte(s[:i+1]), true
			}
		}
	}
	return nil, false
}

func fetch(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "name-that-ui-pp-cli NameThatUI mirror")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, maxPageBytes))
		return nil, fmt.Errorf("GET %s returned HTTP %d", u, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPageBytes {
		return nil, fmt.Errorf("GET %s response exceeds 4 MiB", u)
	}
	return body, nil
}
