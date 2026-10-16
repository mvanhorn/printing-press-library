// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package seohead

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil"
	"golang.org/x/net/html"
)

type Head struct {
	URL           string    `json:"url"`
	FinalURL      string    `json:"final_url"`
	Status        int       `json:"status"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Canonical     string    `json:"canonical"`
	Robots        string    `json:"robots"`
	OGTitle       string    `json:"og_title"`
	OGDescription string    `json:"og_description"`
	SchemaTypes   []string  `json:"schema_types"`
	Noindex       bool      `json:"noindex"`
	FetchedAt     time.Time `json:"fetched_at"`
}

func Parse(pageURL string, source []byte) Head {
	h := Head{URL: pageURL, SchemaTypes: []string{}}
	doc, err := html.Parse(bytes.NewReader(source))
	if err != nil {
		return h
	}
	inHead, inTitle, inBodyTitle, inJSON := false, false, false, false
	headTitleFound := false
	headTagCounts := sourceHeadTagCounts(source)
	tagCounts := make(map[string]int)
	title := strings.Builder{}
	bodyTitle := strings.Builder{}
	jsonLD := strings.Builder{}
	types := map[string]struct{}{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "head" {
			inHead = true
		}
		if n.Type == html.ElementNode && n.Data == "body" && inHead {
			inHead = false
		}
		isHeadTag := false
		if n.Type == html.ElementNode && (n.Data == "meta" || n.Data == "link" || n.Data == "title" || n.Data == "script") && (n.Data != "title" || n.Namespace == "") {
			tagCounts[n.Data]++
			isHeadTag = tagCounts[n.Data] <= headTagCounts[n.Data]
		}
		if n.Type == html.ElementNode && n.Data == "title" && n.Namespace == "" {
			if isHeadTag && !headTitleFound {
				headTitleFound = true
				inTitle = true
			} else if !headTitleFound && bodyTitle.Len() == 0 {
				inBodyTitle = true
			}
		}
		if n.Type == html.ElementNode && n.Data == "script" && attr(n, "type") == "application/ld+json" {
			inJSON = true
			jsonLD.Reset()
		}
		if (inHead || isHeadTag) && n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				name, prop, content := strings.ToLower(attr(n, "name")), strings.ToLower(attr(n, "property")), clean(attr(n, "content"))
				switch name {
				case "description":
					setIfEmpty(&h.Description, content)
				case "robots":
					setIfEmpty(&h.Robots, content)
				}
				switch prop {
				case "og:title":
					setIfEmpty(&h.OGTitle, content)
				case "og:description":
					setIfEmpty(&h.OGDescription, content)
				}
			case "link":
				if hasToken(attr(n, "rel"), "canonical") {
					setIfEmpty(&h.Canonical, clean(attr(n, "href")))
				}
			}
		}

		if n.Type == html.TextNode {
			if inTitle {
				title.WriteString(n.Data)
			}
			if inBodyTitle {
				bodyTitle.WriteString(n.Data)
			}
			if inJSON {
				jsonLD.WriteString(n.Data)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			if n.Data == "title" && n.Namespace == "" {
				inTitle = false
				inBodyTitle = false
			}
			if n.Data == "script" && inJSON {
				collectJSONTypes([]byte(jsonLD.String()), types)
				inJSON = false
			}
			if n.Data == "head" {
				inHead = false
			}
		}
	}
	walk(doc)
	h.Title = clean(title.String())
	if !headTitleFound {
		h.Title = clean(bodyTitle.String())
	}
	h.Noindex = hasToken(h.Robots, "noindex")
	for typ := range types {
		h.SchemaTypes = append(h.SchemaTypes, typ)
	}
	sort.Strings(h.SchemaTypes)
	return h
}

func sourceHeadTagCounts(source []byte) map[string]int {
	counts := map[string]int{}
	inHead := true
	foreignDepth := 0
	tokenizer := html.NewTokenizer(bytes.NewReader(source))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken && tokenType != html.EndTagToken {
			continue
		}
		nameBytes, _ := tokenizer.TagName()
		name := strings.ToLower(string(nameBytes))
		if tokenType == html.EndTagToken && (name == "svg" || name == "math") {
			if foreignDepth > 0 {
				foreignDepth--
			}
			continue
		}
		if tokenType == html.StartTagToken && (name == "svg" || name == "math") {
			foreignDepth++
			continue
		}
		if tokenType == html.EndTagToken && name == "head" {
			inHead = false
			continue
		}
		if tokenType == html.StartTagToken && name == "body" {
			break
		}
		if name == "head" && tokenType != html.EndTagToken {
			inHead = true
			continue
		}
		if inHead && (name == "meta" || name == "link" || name == "script" || (name == "title" && foreignDepth == 0)) && tokenType != html.EndTagToken {
			counts[name]++
		}
	}
	return counts
}

func setIfEmpty(target *string, value string) {
	if *target == "" {
		*target = value
	}
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}
func clean(s string) string { return strings.TrimSpace(html.UnescapeString(s)) }
func hasToken(s, token string) bool {
	for _, v := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
		if v == token {
			return true
		}
	}
	return false
}
func collectJSONTypes(raw []byte, out map[string]struct{}) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	var visit func(any)
	visit = func(x any) {
		switch z := x.(type) {
		case map[string]any:
			if t, ok := z["@type"]; ok {
				switch q := t.(type) {
				case string:
					if q != "" {
						out[q] = struct{}{}
					}
				case []any:
					for _, e := range q {
						if s, ok := e.(string); ok && s != "" {
							out[s] = struct{}{}
						}
					}
				}
			}
			for _, e := range z {
				visit(e)
			}
		case []any:
			for _, e := range z {
				visit(e)
			}
		}
	}
	visit(v)
}

type Fetcher struct {
	client  *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New(timeout time.Duration, rps float64) *Fetcher {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Fetcher{client: &http.Client{Timeout: timeout}, limiter: cliutil.NewAdaptiveLimiter(rps)}
}
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (Head, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if err := f.limiter.Wait(ctx); err != nil {
			return Head{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return Head{}, err
		}
		req.Header.Set("User-Agent", "rankmath-pp-cli")
		resp, err := f.client.Do(req)
		if err != nil {
			return Head{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if readErr != nil {
			return Head{}, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			h := Head{URL: rawURL, FinalURL: resp.Request.URL.String(), Status: resp.StatusCode, FetchedAt: time.Now().UTC()}
			f.limiter.OnRateLimit()
			last = &cliutil.RateLimitError{URL: rawURL, Body: string(body)}
			if attempt < 2 {
				continue
			}
			return h, last
		}
		h := Parse(rawURL, body)
		h.FinalURL = resp.Request.URL.String()
		h.Status = resp.StatusCode
		h.FetchedAt = time.Now().UTC()
		f.limiter.OnSuccess()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return h, fmt.Errorf("fetch %s: HTTP %d", rawURL, resp.StatusCode)
		}
		return h, nil
	}
	return Head{}, last
}
