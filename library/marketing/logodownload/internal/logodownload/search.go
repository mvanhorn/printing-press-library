package logodownload

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var baseURL = "https://logodownload.org"

func Search(ctx context.Context, client *http.Client, query string, limit int) ([]LogoResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []LogoResult{}, nil
	}
	if limit <= 0 {
		limit = 10
	}

	results, err := searchHTML(ctx, client, query)
	if err != nil {
		log.Printf("Busca HTML falhou, tentando API WordPress: %v", err)
	}

	if len(results) == 0 {
		results, err = searchWordPressAPI(ctx, client, query, limit)
		if err != nil {
			return nil, err
		}
	}

	return dedupe(results), nil
}

func searchHTML(ctx context.Context, client *http.Client, query string) ([]LogoResult, error) {
	searchURL, err := buildURL("/", map[string]string{"s": query})
	if err != nil {
		return nil, err
	}

	log.Printf("Buscando HTML: %s", searchURL)

	resp, err := request(ctx, client, searchURL, "text/html,application/json;q=0.9,*/*;q=0.8")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler o HTML: %w", err)
	}

	results := make([]LogoResult, 0)
	doc.Find("article.grid-post, article").Each(func(_ int, article *goquery.Selection) {
		link := firstNonEmptyAttr(article, []string{
			"h2.entry-title a",
			"h2.title a",
			"h2.post-title a",
			"a[rel='bookmark']",
			".post-thumbnail a",
		}, "href")

		title := firstNonEmptyText(article, []string{
			"h2.entry-title a",
			"h2.title a",
			"h2.post-title a",
			"a[rel='bookmark']",
			".post-thumbnail a",
		})

		if title == "" {
			title, _ = article.Find(".post-thumbnail a").First().Attr("title")
		}

		imageURL := firstNonEmptyAttr(article, []string{
			".post-thumbnail img",
			"img.wp-post-image",
			"img",
		}, "src")

		title = strings.TrimSpace(html.UnescapeString(title))
		link = absoluteURL(link)
		imageURL = absoluteURL(imageURL)

		if title != "" && link != "" {
			results = append(results, LogoResult{
				Title:    title,
				URL:      link,
				ImageURL: imageURL,
			})
		}
	})

	return results, nil
}

func searchWordPressAPI(ctx context.Context, client *http.Client, query string, limit int) ([]LogoResult, error) {
	apiURL, err := buildURL("/wp-json/wp/v2/search", map[string]string{
		"search":   query,
		"subtype":  "post",
		"per_page": fmt.Sprintf("%d", limit),
	})
	if err != nil {
		return nil, err
	}

	log.Printf("Buscando API WordPress: %s", apiURL)

	resp, err := request(ctx, client, apiURL, "application/json,*/*;q=0.8")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResults []wpSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&apiResults); err != nil {
		return nil, fmt.Errorf("não foi possível ler a resposta JSON: %w", err)
	}

	results := make([]LogoResult, 0, len(apiResults))
	for _, item := range apiResults {
		title := strings.TrimSpace(html.UnescapeString(item.Title))
		link := absoluteURL(item.URL)
		if title != "" && link != "" {
			imageURL := ""
			if item.ID != 0 {
				imageURL, _ = fetchPostImageURL(ctx, client, item.ID)
			}
			results = append(results, LogoResult{Title: title, URL: link, ImageURL: imageURL})
		}
	}

	return results, nil
}

func fetchPostImageURL(ctx context.Context, client *http.Client, postID int) (string, error) {
	postURL, err := buildURL(fmt.Sprintf("/wp-json/wp/v2/posts/%d", postID), map[string]string{
		"_embed": "wp:featuredmedia",
	})
	if err != nil {
		return "", err
	}

	resp, err := request(ctx, client, postURL, "application/json,*/*;q=0.8")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var post wpPostResult
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return "", err
	}

	if len(post.Embedded.FeaturedMedia) == 0 {
		return "", nil
	}

	media := post.Embedded.FeaturedMedia[0]
	for _, size := range []string{"large", "full", "medium"} {
		if sourceURL := media.MediaDetails.Sizes[size].SourceURL; sourceURL != "" {
			return absoluteURL(sourceURL), nil
		}
	}

	return absoluteURL(media.SourceURL), nil
}

func request(ctx context.Context, client *http.Client, rawURL string, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PrintingPressDev/1.0; +https://logodownload.org)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("status HTTP inesperado: %s", resp.Status)
	}

	return resp, nil
}

func buildURL(path string, query map[string]string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	parsed.Path = path
	params := parsed.Query()
	for key, value := range query {
		params.Set(key, value)
	}
	parsed.RawQuery = params.Encode()

	return parsed.String(), nil
}

func firstNonEmptyText(parent *goquery.Selection, selectors []string) string {
	for _, selector := range selectors {
		value := strings.TrimSpace(parent.Find(selector).First().Text())
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyAttr(parent *goquery.Selection, selectors []string, attr string) string {
	for _, selector := range selectors {
		value, exists := parent.Find(selector).First().Attr(attr)
		if exists && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func absoluteURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	return base.ResolveReference(parsed).String()
}

func dedupe(results []LogoResult) []LogoResult {
	if len(results) == 0 {
		return []LogoResult{}
	}

	seen := make(map[string]struct{}, len(results))
	unique := make([]LogoResult, 0, len(results))

	for _, result := range results {
		key := result.URL
		if key == "" {
			key = strings.ToLower(result.Title)
		}

		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		unique = append(unique, result)
	}

	return unique
}
