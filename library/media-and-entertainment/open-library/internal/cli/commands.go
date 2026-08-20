// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

func newBookCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	var lang string
	cmd := &cobra.Command{
		Use:   "book <query>",
		Short: "Search Open Library for book/work candidates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			queryText := strings.TrimSpace(args[0])
			if queryText == "" {
				return usageErr("book requires a non-empty query")
			}
			limit = clamp(limit, 1, 20)
			if page < 1 {
				page = 1
			}
			client := newOpenLibraryClient(flags.timeout)
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()

			values := url.Values{}
			values.Set("q", queryText)
			values.Set("limit", strconv.Itoa(limit))
			values.Set("page", strconv.Itoa(page))
			values.Set("fields", "key,title,author_name,author_key,first_publish_year,edition_count,isbn,language")
			if strings.TrimSpace(lang) != "" {
				values.Set("lang", strings.TrimSpace(lang))
			}
			var apiResp searchResponse
			if err := client.getJSON(ctx, "/search.json", values, &apiResp); err != nil {
				return err
			}
			result := BookSearchResult{
				Source:     "Open Library Search API",
				Configured: true,
				Query:      map[string]any{"q": queryText, "limit": limit, "page": page, "lang": strings.TrimSpace(lang)},
				Request:    RequestInfo{Endpoint: "/search.json", Query: valuesToMap(values)},
				Total:      apiResp.NumFound,
				Results:    bookCandidates(apiResp.Docs, client.baseURL),
				Freshness:  freshnessNote(),
				Caveats:    sourceCaveats(client.identified),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum book candidates to return, capped at 20")
	cmd.Flags().IntVar(&page, "page", 1, "Search result page, starting at 1")
	cmd.Flags().StringVar(&lang, "lang", "", "Optional ISO 639-1 language preference")
	return cmd
}

func newISBNCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "isbn <isbn>",
		Short: "Resolve an ISBN to Open Library edition metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			isbn := normalizeISBN(args[0])
			if isbn == "" {
				return usageErr("isbn requires an ISBN-10 or ISBN-13")
			}
			client := newOpenLibraryClient(flags.timeout)
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()

			path := "/isbn/" + url.PathEscape(isbn) + ".json"
			var raw editionAPI
			if err := client.getJSON(ctx, path, nil, &raw); err != nil {
				return err
			}
			edition := raw.toSummary()
			result := EditionResult{
				Source:     "Open Library ISBN API",
				Configured: true,
				Query:      map[string]any{"isbn": isbn},
				Request:    RequestInfo{Endpoint: path},
				Edition:    edition,
				SourceURL:  client.sourceURL(firstNonEmpty(edition.Key, "/isbn/"+isbn)),
				Freshness:  freshnessNote(),
				Caveats:    sourceCaveats(client.identified),
			}
			return printResult(cmd, flags, result)
		},
	}
	return cmd
}

func newAuthorCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "author <query-or-author-id>",
		Short: "Search an author and fetch a bounded works sample",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			if input == "" {
				return usageErr("author requires a name or Open Library author ID")
			}
			limit = clamp(limit, 1, 50)
			client := newOpenLibraryClient(flags.timeout)
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()

			var author AuthorSummary
			var candidates []AuthorSummary
			authorID := findOpenLibraryID(input, "A")
			request := RequestInfo{}
			if authorID == "" {
				values := url.Values{}
				values.Set("q", input)
				values.Set("limit", strconv.Itoa(limit))
				var search authorSearchResponse
				if err := client.getJSON(ctx, "/search/authors.json", values, &search); err != nil {
					return err
				}
				candidates = authorCandidates(search.Docs, client.baseURL)
				if len(candidates) > 0 {
					author = candidates[0]
					authorID = findOpenLibraryID(author.Key, "A")
				}
				request = RequestInfo{Endpoint: "/search/authors.json", Query: valuesToMap(values)}
			} else {
				path := "/authors/" + authorID + ".json"
				var raw authorAPI
				if err := client.getJSON(ctx, path, nil, &raw); err != nil {
					return err
				}
				author = raw.toSummary(client.baseURL)
				request = RequestInfo{Endpoint: path}
			}
			works := []WorkSummary{}
			if authorID != "" {
				values := url.Values{}
				values.Set("limit", strconv.Itoa(limit))
				var worksResp authorWorksResponse
				if err := client.getJSON(ctx, "/authors/"+authorID+"/works.json", values, &worksResp); err != nil {
					return err
				}
				works = workSummaries(worksResp.Entries, client.baseURL)
			}
			result := AuthorResult{
				Source:     "Open Library Authors API",
				Configured: true,
				Query:      map[string]any{"input": input, "limit": limit},
				Request:    request,
				Author:     author,
				Candidates: candidates,
				Works:      works,
				SourceURL:  author.SourceURL,
				Freshness:  freshnessNote(),
				Caveats:    sourceCaveats(client.identified),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum author candidates and works to return, capped at 50")
	return cmd
}

func newWorkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work <work-id>",
		Short: "Fetch a specific Open Library work JSON record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workID := findOpenLibraryID(args[0], "W")
			if workID == "" {
				return usageErr("work requires an Open Library work ID like OL45804W")
			}
			client := newOpenLibraryClient(flags.timeout)
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()

			path := "/works/" + workID + ".json"
			var raw workAPI
			if err := client.getJSON(ctx, path, nil, &raw); err != nil {
				return err
			}
			work := raw.toSummary(client.baseURL)
			result := WorkResult{
				Source:     "Open Library Works API",
				Configured: true,
				Query:      map[string]any{"work_id": workID},
				Request:    RequestInfo{Endpoint: path},
				Work:       work,
				SourceURL:  work.SourceURL,
				Freshness:  freshnessNote(),
				Caveats:    sourceCaveats(client.identified),
			}
			return printResult(cmd, flags, result)
		},
	}
	return cmd
}

func newEditionsCmd(flags *rootFlags) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "editions <work-id>",
		Short: "Fetch bounded editions for an Open Library work",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workID := findOpenLibraryID(args[0], "W")
			if workID == "" {
				return usageErr("editions requires an Open Library work ID like OL45804W")
			}
			limit = clamp(limit, 1, 50)
			if offset < 0 {
				offset = 0
			}
			client := newOpenLibraryClient(flags.timeout)
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()

			values := url.Values{}
			values.Set("limit", strconv.Itoa(limit))
			values.Set("offset", strconv.Itoa(offset))
			path := "/works/" + workID + "/editions.json"
			var raw editionsResponse
			if err := client.getJSON(ctx, path, values, &raw); err != nil {
				return err
			}
			editions := make([]EditionSummary, 0, len(raw.Entries))
			for _, entry := range raw.Entries {
				editions = append(editions, entry.toSummary())
			}
			sourceURL := client.sourceURL("/works/" + workID + "/editions")
			result := EditionsResult{
				Source:     "Open Library Editions API",
				Configured: true,
				Query:      map[string]any{"work_id": workID, "limit": limit, "offset": offset},
				Request:    RequestInfo{Endpoint: path, Query: valuesToMap(values)},
				Total:      firstPositive(raw.Size, len(editions)),
				Editions:   editions,
				SourceURL:  sourceURL,
				Freshness:  freshnessNote(),
				Caveats:    sourceCaveats(client.identified),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum editions to return, capped at 50")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func newSubjectsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var details bool
	cmd := &cobra.Command{
		Use:   "subjects <subject>",
		Short: "Fetch works and facets for an Open Library subject",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			subject := strings.TrimSpace(args[0])
			slug := subjectSlug(subject)
			if slug == "" {
				return usageErr("subjects requires a subject name")
			}
			limit = clamp(limit, 1, 50)
			client := newOpenLibraryClient(flags.timeout)
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()

			values := url.Values{}
			values.Set("limit", strconv.Itoa(limit))
			if details {
				values.Set("details", "true")
			}
			path := "/subjects/" + slug + ".json"
			var raw subjectResponse
			if err := client.getJSON(ctx, path, values, &raw); err != nil {
				return err
			}
			var facets *SubjectFacets
			if details {
				facets = &SubjectFacets{
					Subjects:   trimFacets(raw.Subjects, 10),
					Authors:    trimFacets(raw.Authors, 10),
					Publishers: trimFacets(raw.Publishers, 10),
				}
			}
			result := SubjectResult{
				Source:     "Open Library Subjects API",
				Configured: true,
				Query:      map[string]any{"subject": subject, "slug": slug, "limit": limit, "details": details},
				Request:    RequestInfo{Endpoint: path, Query: valuesToMap(values)},
				Subject: SubjectSummary{
					Key:         raw.Key,
					Name:        firstNonEmpty(raw.Name, subject),
					Type:        raw.SubjectType,
					WorkCount:   raw.WorkCount,
					EBookCount:  raw.EBookCount,
					DetailsUsed: details,
				},
				Works:     subjectWorks(raw.Works, client.baseURL),
				Facets:    facets,
				SourceURL: client.sourceURL("/subjects/" + slug),
				Freshness: freshnessNote(),
				Caveats: append(sourceCaveats(client.identified),
					"The Open Library Subjects API is documented as experimental and may change."),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum subject works to return, capped at 50")
	cmd.Flags().BoolVar(&details, "details", false, "Request subject facets such as related subjects, authors, and publishers")
	return cmd
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "Describe Open Library source coverage, rate limits, and non-goals",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newOpenLibraryClient(flags.timeout)
			result := SourcesResult{
				Source:     "open-library-pp-cli",
				Auth:       "none",
				Configured: true,
				BaseURL:    client.baseURL,
				Identified: client.identified,
				RateLimit:  rateLimitText(client.identified),
				Endpoints: []SourceInfo{
					{Name: "Book Search API", URL: "https://openlibrary.org/search.json", Use: "Book/work search and bibliography seeds"},
					{Name: "ISBN API", URL: "https://openlibrary.org/isbn/<isbn>.json", Use: "Edition lookup by ISBN"},
					{Name: "Works API", URL: "https://openlibrary.org/works/<id>.json", Use: "Canonical work metadata"},
					{Name: "Editions API", URL: "https://openlibrary.org/works/<id>/editions.json", Use: "Bounded edition lists"},
					{Name: "Authors API", URL: "https://openlibrary.org/search/authors.json", Use: "Author search and works"},
					{Name: "Subjects API", URL: "https://openlibrary.org/subjects/<subject>.json", Use: "Subject browsing", Status: "experimental"},
				},
				Freshness: freshnessNote(),
				Caveats:   sourceCaveats(client.identified),
				Environment: []EnvInfo{
					{Name: userAgentEnv, Configured: strings.TrimSpace(env(userAgentEnv)) != "", Description: "Optional custom application identity for the User-Agent header"},
					{Name: contactEmailEnv, Configured: strings.TrimSpace(env(contactEmailEnv)) != "", Description: "Optional contact email for identified regular/frequent use"},
				},
			}
			return printResult(cmd, flags, result)
		},
	}
}

type searchResponse struct {
	NumFound int       `json:"numFound"`
	Docs     []bookDoc `json:"docs"`
}

type bookDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	AuthorKey        []string `json:"author_key"`
	FirstPublishYear int      `json:"first_publish_year"`
	EditionCount     int      `json:"edition_count"`
	ISBN             []string `json:"isbn"`
	Language         []string `json:"language"`
}

type editionAPI struct {
	Key         string      `json:"key"`
	Title       string      `json:"title"`
	PublishDate string      `json:"publish_date"`
	Publishers  []string    `json:"publishers"`
	ISBN10      []string    `json:"isbn_10"`
	ISBN13      []string    `json:"isbn_13"`
	Works       []KeyRef    `json:"works"`
	Authors     []KeyRef    `json:"authors"`
	Covers      []int       `json:"covers"`
	Identifiers interface{} `json:"identifiers"`
}

type authorSearchResponse struct {
	NumFound int         `json:"numFound"`
	Docs     []authorAPI `json:"docs"`
}

type authorAPI struct {
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	TopWork        string   `json:"top_work"`
	WorkCount      int      `json:"work_count"`
	BirthDate      string   `json:"birth_date"`
	DeathDate      string   `json:"death_date"`
	AlternateNames []string `json:"alternate_names"`
}

type authorWorksResponse struct {
	Size    int       `json:"size"`
	Entries []workAPI `json:"entries"`
}

type workAPI struct {
	Key              string          `json:"key"`
	Title            string          `json:"title"`
	Description      json.RawMessage `json:"description"`
	FirstPublishDate string          `json:"first_publish_date"`
	FirstPublishYear int             `json:"first_publish_year"`
	EditionCount     int             `json:"edition_count"`
	Authors          []struct {
		Author KeyRef `json:"author"`
		Key    string `json:"key"`
	} `json:"authors"`
	Subjects       []string `json:"subjects"`
	Covers         []int    `json:"covers"`
	LatestRevision int      `json:"latest_revision"`
}

type editionsResponse struct {
	Size    int          `json:"size"`
	Entries []editionAPI `json:"entries"`
}

type subjectResponse struct {
	Key         string        `json:"key"`
	Name        string        `json:"name"`
	SubjectType string        `json:"subject_type"`
	WorkCount   int           `json:"work_count"`
	EBookCount  int           `json:"ebook_count"`
	Works       []subjectWork `json:"works"`
	Subjects    []Facet       `json:"subjects"`
	Authors     []Facet       `json:"authors"`
	Publishers  []Facet       `json:"publishers"`
}

type subjectWork struct {
	Key          string `json:"key"`
	Title        string `json:"title"`
	EditionCount int    `json:"edition_count"`
	Authors      []struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	} `json:"authors"`
	FirstPublishYear int `json:"first_publish_year"`
}

func (c *openLibraryClient) sourceURL(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return c.baseURL
	}
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		return key
	}
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	return c.baseURL + strings.TrimSuffix(key, ".json")
}

func (e editionAPI) toSummary() EditionSummary {
	return EditionSummary{
		Key:         e.Key,
		Title:       e.Title,
		PublishDate: e.PublishDate,
		Publishers:  trimStrings(e.Publishers, 10),
		ISBN10:      trimStrings(e.ISBN10, 10),
		ISBN13:      trimStrings(e.ISBN13, 10),
		Works:       e.Works,
		Authors:     e.Authors,
		Covers:      trimInts(e.Covers, 10),
		Identifiers: e.Identifiers,
	}
}

func (a authorAPI) toSummary(baseURL string) AuthorSummary {
	key := a.Key
	if key != "" && !strings.HasPrefix(key, "/authors/") {
		key = "/authors/" + key
	}
	return AuthorSummary{
		Key:            key,
		Name:           a.Name,
		TopWork:        a.TopWork,
		WorkCount:      a.WorkCount,
		BirthDate:      a.BirthDate,
		DeathDate:      a.DeathDate,
		AlternateNames: trimStrings(a.AlternateNames, 10),
		SourceURL:      strings.TrimRight(baseURL, "/") + key,
	}
}

func (w workAPI) toSummary(baseURL string) WorkSummary {
	authors := make([]KeyRef, 0, len(w.Authors))
	for _, item := range w.Authors {
		key := firstNonEmpty(item.Author.Key, item.Key)
		if key != "" {
			authors = append(authors, KeyRef{Key: key})
		}
	}
	return WorkSummary{
		Key:              w.Key,
		Title:            w.Title,
		Description:      shorten(stringFromRaw(w.Description), 700),
		FirstPublishDate: w.FirstPublishDate,
		FirstPublishYear: w.FirstPublishYear,
		EditionCount:     w.EditionCount,
		Authors:          authors,
		Subjects:         trimStrings(w.Subjects, 20),
		Covers:           trimInts(w.Covers, 10),
		LatestRevision:   w.LatestRevision,
		SourceURL:        strings.TrimRight(baseURL, "/") + w.Key,
	}
}

func bookCandidates(docs []bookDoc, baseURL string) []BookCandidate {
	out := make([]BookCandidate, 0, len(docs))
	for _, doc := range docs {
		out = append(out, BookCandidate{
			Key:              doc.Key,
			Title:            doc.Title,
			Authors:          trimStrings(doc.AuthorName, 5),
			AuthorKeys:       trimStrings(doc.AuthorKey, 5),
			FirstPublishYear: doc.FirstPublishYear,
			EditionCount:     doc.EditionCount,
			ISBNs:            trimStrings(doc.ISBN, 5),
			Languages:        trimStrings(doc.Language, 5),
			SourceURL:        strings.TrimRight(baseURL, "/") + doc.Key,
		})
	}
	return out
}

func authorCandidates(docs []authorAPI, baseURL string) []AuthorSummary {
	out := make([]AuthorSummary, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toSummary(baseURL))
	}
	return out
}

func workSummaries(entries []workAPI, baseURL string) []WorkSummary {
	out := make([]WorkSummary, 0, len(entries))
	for _, entry := range entries {
		summary := entry.toSummary(baseURL)
		summary.Description = ""
		out = append(out, summary)
	}
	return out
}

func subjectWorks(entries []subjectWork, baseURL string) []BookCandidate {
	out := make([]BookCandidate, 0, len(entries))
	for _, entry := range entries {
		authors := make([]string, 0, len(entry.Authors))
		authorKeys := make([]string, 0, len(entry.Authors))
		for _, author := range entry.Authors {
			authors = append(authors, author.Name)
			authorKeys = append(authorKeys, author.Key)
		}
		out = append(out, BookCandidate{
			Key:              entry.Key,
			Title:            entry.Title,
			Authors:          trimStrings(authors, 5),
			AuthorKeys:       trimStrings(authorKeys, 5),
			FirstPublishYear: entry.FirstPublishYear,
			EditionCount:     entry.EditionCount,
			SourceURL:        strings.TrimRight(baseURL, "/") + entry.Key,
		})
	}
	return out
}

func rateLimitText(identified bool) string {
	if identified {
		return "Identified requests: 3 requests per second per Open Library guidance."
	}
	return "Non-identified requests: 1 request per second per Open Library guidance."
}

func normalizeISBN(input string) string {
	replacer := strings.NewReplacer("-", "", " ", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(input)))
}

var openLibraryIDPattern = regexp.MustCompile(`OL[0-9A-Z]+([A-Z])`)

func findOpenLibraryID(input, suffix string) string {
	upper := strings.ToUpper(input)
	matches := openLibraryIDPattern.FindAllStringSubmatch(upper, -1)
	for _, match := range matches {
		if len(match) > 1 && match[1] == suffix {
			return match[0]
		}
	}
	return ""
}

func subjectSlug(input string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func valuesToMap(values url.Values) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		if len(value) == 1 {
			out[key] = value[0]
		} else {
			out[key] = value
		}
	}
	return out
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func trimStrings(values []string, limit int) []string {
	if len(values) > limit {
		values = values[:limit]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func trimInts(values []int, limit int) []int {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func trimFacets(values []Facet, limit int) []Facet {
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func shorten(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}
