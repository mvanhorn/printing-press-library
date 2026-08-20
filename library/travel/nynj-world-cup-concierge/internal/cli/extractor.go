// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEventGUID = "ef742ab9-0cc1-45dc-a173-739ec1eeb541"
	defaultAPIBase   = "https://nynj-ai.neurun.com/api"
	maxResponseBytes = 10 << 20
	sourceName       = "NYNJ World Cup Concierge"
)

var defaultSourceURLs = map[string]string{
	"destination": "https://nynjfwc26.com/destination/",
	"fan_events":  "https://nynjfwc26.com/fan-events/",
}

var months = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

type sourceOptions struct {
	EventGUID       string
	APIBase         string
	Lang            string
	TimeoutSeconds  int
	Categories      []string
	DateWindowStart string
	DateWindowEnd   string
	ExcludeUndated  bool
	Pretty          bool
	Agent           bool
	EventJSON       string
	PromptsJSON     string
	DestinationHTML string
	FanEventsHTML   string
}

type pageBundle struct {
	Destination string
	FanEvents   string
}

type link struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type prompt struct {
	GUID       string
	Order      int
	PromptText string
}

type categorySummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type candidate map[string]any

type payload struct {
	Meta       map[string]any    `json:"meta"`
	Categories []categorySummary `json:"categories"`
	Prompts    []map[string]any  `json:"prompts"`
	Candidates []candidate       `json:"candidates"`
}

func defaultSourceOptions() sourceOptions {
	return sourceOptions{
		EventGUID:      defaultEventGUID,
		APIBase:        defaultAPIBase,
		Lang:           "en",
		TimeoutSeconds: 20,
	}
}

func buildPayload(opts sourceOptions) (payload, error) {
	eventPayload, err := loadJSON(opts.EventJSON, fmt.Sprintf("%s/race/event/guid/%s", strings.TrimRight(opts.APIBase, "/"), opts.EventGUID), opts.TimeoutSeconds)
	if err != nil {
		return payload{}, err
	}
	promptsPayload, err := loadJSON(opts.PromptsJSON, fmt.Sprintf("%s/prompts/by-event/%s?lang=%s", strings.TrimRight(opts.APIBase, "/"), opts.EventGUID, opts.Lang), opts.TimeoutSeconds)
	if err != nil {
		return payload{}, err
	}
	pages, err := loadPages(opts)
	if err != nil {
		return payload{}, err
	}

	prompts := normalizePrompts(promptsPayload)
	candidates := []candidate{}
	candidates = append(candidates, extractCardItems(pages.Destination)...)
	candidates = append(candidates, extractFanExperienceItems(pages.FanEvents)...)
	candidates = append(candidates, extractWatchPartyItems(pages.Destination, prompts)...)
	candidates = filterCategories(candidates, opts.Categories)
	candidates, err = filterDateWindow(candidates, opts.DateWindowStart, opts.DateWindowEnd, opts.ExcludeUndated)
	if err != nil {
		return payload{}, err
	}

	categories := summarizeCategories(candidates)
	event := asMap(eventPayload)
	return payload{
		Meta: map[string]any{
			"source":            "nynj-world-cup-concierge",
			"source_name":       sourceName,
			"event_guid":        stringValue(event, "guid", defaultEventGUID),
			"event_name":        stringValue(event, "name", sourceName),
			"event_updated_at":  event["updated_at"],
			"source_urls":       defaultSourceURLs,
			"category_filters":  opts.Categories,
			"date_window_start": opts.DateWindowStart,
			"date_window_end":   opts.DateWindowEnd,
			"exclude_undated":   opts.ExcludeUndated,
		},
		Categories: categories,
		Prompts:    promptRecords(prompts),
		Candidates: candidates,
	}, nil
}

func loadPages(opts sourceOptions) (pageBundle, error) {
	destination, err := loadText(opts.DestinationHTML, defaultSourceURLs["destination"], opts.TimeoutSeconds)
	if err != nil {
		return pageBundle{}, err
	}
	fanEvents, err := loadText(opts.FanEventsHTML, defaultSourceURLs["fan_events"], opts.TimeoutSeconds)
	if err != nil {
		return pageBundle{}, err
	}
	return pageBundle{Destination: destination, FanEvents: fanEvents}, nil
}

func loadJSON(path string, url string, timeoutSeconds int) (any, error) {
	body, err := loadText(path, url, timeoutSeconds)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func loadText(path string, url string, timeoutSeconds int) (string, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	client := http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nynj-world-cup-concierge-pp-cli/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(data) > maxResponseBytes {
		return "", fmt.Errorf("GET %s response exceeded %d bytes", url, maxResponseBytes)
	}
	return string(data), nil
}

func normalizeSpace(value string) string {
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\u2011", "-")
	return strings.Join(strings.Fields(value), " ")
}

func slugify(value string) string {
	value = strings.ToLower(value)
	slug := slugRe.ReplaceAllString(value, "-")
	return strings.Trim(slug, "-")
}

func sectionByID(pageHTML string, sectionID string) string {
	pattern := regexp.MustCompile(`<div class="section-inner[^"]*"[^>]*id="` + regexp.QuoteMeta(sectionID) + `"`)
	match := pattern.FindStringIndex(pageHTML)
	if match == nil {
		return ""
	}
	tail := pageHTML[match[1]:]
	stopPattern := regexp.MustCompile(`<div class="(?:section-inner|footer-section)`)
	stop := stopPattern.FindStringIndex(tail)
	if stop == nil {
		return pageHTML[match[0]:]
	}
	return pageHTML[match[0] : match[1]+stop[0]]
}

func stripTags(fragment string) string {
	return normalizeSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(fragment, " "))
}

func firstMatch(pattern string, fragment string) string {
	re := regexp.MustCompile(`(?is)` + pattern)
	match := re.FindStringSubmatch(fragment)
	if len(match) < 2 {
		return ""
	}
	return stripTags(match[1])
}

func extractLinks(fragment string) []link {
	re := regexp.MustCompile(`(?is)<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(fragment, -1)
	links := make([]link, 0, len(matches))
	for _, match := range matches {
		links = append(links, link{Text: stripTags(match[2]), URL: html.UnescapeString(match[1])})
	}
	return links
}

func extractImages(fragment string) []string {
	re := regexp.MustCompile(`(?is)<img[^>]+src="([^"]+)"`)
	matches := re.FindAllStringSubmatch(fragment, -1)
	images := make([]string, 0, len(matches))
	for _, match := range matches {
		images = append(images, html.UnescapeString(match[1]))
	}
	return images
}

func fragmentText(fragment string) string {
	return stripTags(fragment)
}

func extractCardItems(destinationHTML string) []candidate {
	region := sectionByID(destinationHTML, "ID1")
	re := regexp.MustCompile(`(?is)<div class="card-title altfont">\s*(.*?)\s*</div>\s*<div class="card-subtitle">(.*?)</div>`)
	matches := re.FindAllStringSubmatchIndex(region, -1)
	items := []candidate{}
	for _, idx := range matches {
		title := stripTags(region[idx[2]:idx[3]])
		body := fragmentText(region[idx[4]:idx[5]])
		if title == "" {
			continue
		}
		image := ""
		before := region[:idx[0]]
		images := extractImages(before)
		if len(images) > 0 {
			image = images[len(images)-1]
		}
		items = append(items, candidate{
			"candidate_id":    "nynj-explore-" + slugify(title),
			"type":            "activity",
			"title":           title,
			"name":            title,
			"category":        "Explore NYNJ",
			"source_category": "Explore NYNJ",
			"description":     body,
			"url":             defaultSourceURLs["destination"],
			"image_url":       image,
			"provider":        sourceName,
		})
	}
	return items
}

func extractFanExperienceItems(fanEventsHTML string) []candidate {
	markerRe := regexp.MustCompile(`(?is)<div class="section-inner s27[^"]*"[^>]*id="([^"]+)"`)
	markers := markerRe.FindAllStringSubmatch(fanEventsHTML, -1)
	items := []candidate{}
	for _, marker := range markers {
		sectionID := marker[1]
		fragment := sectionByID(fanEventsHTML, sectionID)
		title := firstMatch(`<h2>(.*?)</h2>`, fragment)
		if title == "" {
			continue
		}
		detailLines := extractListItems(fragment, title)
		descriptionParts := extractParagraphs(fragment)
		description := strings.Join(descriptionParts, " ")
		location := ""
		if len(detailLines) >= 2 {
			location = detailLines[0] + " / " + detailLines[1]
		} else if len(detailLines) == 1 {
			location = detailLines[0]
		}
		dateText := ""
		if len(detailLines) >= 3 {
			dateText = detailLines[2]
		}
		links := extractLinks(fragment)
		url := defaultSourceURLs["fan_events"]
		for _, link := range links {
			if strings.HasPrefix(link.URL, "http") {
				url = link.URL
				break
			}
		}
		images := extractImages(fragment)
		imageURL := ""
		if len(images) > 0 {
			imageURL = images[0]
		}
		if description == "" {
			description = strings.Join(detailLines, " ")
		}
		items = append(items, candidate{
			"candidate_id":    "nynj-fan-experience-" + slugify(firstNonEmpty(sectionID, title)),
			"type":            "activity",
			"title":           title,
			"name":            title,
			"category":        "Fan Experiences",
			"source_category": "Fan Experiences",
			"description":     description,
			"url":             url,
			"source_url":      defaultSourceURLs["fan_events"] + "#" + sectionID,
			"image_url":       imageURL,
			"location":        location,
			"date_text":       dateText,
			"details":         detailLines,
			"provider":        sourceName,
		})
	}
	return items
}

func extractListItems(fragment string, title string) []string {
	re := regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	matches := re.FindAllStringSubmatch(fragment, -1)
	items := []string{}
	for _, match := range matches {
		item := stripTags(match[1])
		if item != "" && item != title {
			items = append(items, item)
		}
	}
	return items
}

func extractParagraphs(fragment string) []string {
	re := regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	matches := re.FindAllStringSubmatch(fragment, -1)
	paragraphs := []string{}
	for _, match := range matches {
		text := stripTags(match[1])
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	return paragraphs
}

func extractWatchPartyItems(destinationHTML string, prompts []prompt) []candidate {
	items := []candidate{}
	section := sectionByID(destinationHTML, "events-and-toolkit")
	links := []link{}
	for _, link := range extractLinks(section) {
		search := strings.ToLower(link.Text + " " + link.URL)
		if strings.Contains(search, "watch") || strings.Contains(search, "viewing") || strings.Contains(search, "event") || strings.Contains(search, "calendar") {
			links = append(links, link)
		}
	}
	if section != "" {
		url := defaultSourceURLs["destination"] + "#events-and-toolkit"
		if len(links) > 0 {
			url = links[0].URL
		}
		items = append(items, candidate{
			"candidate_id":    "nynj-watch-party-public-viewing-guidance",
			"type":            "activity",
			"title":           "Watch Parties and Public Viewing Guidance",
			"name":            "Watch Parties and Public Viewing Guidance",
			"category":        "Watch Parties",
			"source_category": "Watch Parties",
			"description":     fragmentText(section),
			"url":             url,
			"source_url":      defaultSourceURLs["destination"] + "#events-and-toolkit",
			"links":           links,
			"provider":        sourceName,
		})
	}
	for _, prompt := range prompts {
		if prompt.PromptText == "" || !strings.Contains(strings.ToLower(prompt.PromptText), "watch") {
			continue
		}
		items = append(items, candidate{
			"candidate_id":    "nynj-watch-party-prompt-" + firstNonEmpty(prompt.GUID, slugify(prompt.PromptText)),
			"type":            "activity",
			"title":           prompt.PromptText,
			"name":            prompt.PromptText,
			"category":        "Watch Parties",
			"source_category": "Concierge Prompts",
			"description":     "Official Concierge prompt related to match viewing and watch-party planning.",
			"url":             defaultSourceURLs["destination"],
			"provider":        sourceName,
			"prompt_guid":     prompt.GUID,
		})
	}
	return items
}

func normalizePrompts(value any) []prompt {
	rawPrompts := []any{}
	switch typed := value.(type) {
	case []any:
		rawPrompts = typed
	case map[string]any:
		if promptsValue, ok := typed["prompts"].([]any); ok {
			rawPrompts = promptsValue
		}
	}
	prompts := []prompt{}
	for _, raw := range rawPrompts {
		item, ok := raw.(map[string]any)
		if !ok || boolValue(item, "is_partner_bot") {
			continue
		}
		prompts = append(prompts, prompt{
			GUID:       stringValue(item, "guid", ""),
			Order:      intValue(item, "order", 999),
			PromptText: normalizeSpace(firstNonEmpty(stringValue(item, "prompt_text", ""), stringValue(item, "promptText", ""))),
		})
	}
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].Order < prompts[j].Order })
	return prompts
}

func promptRecords(prompts []prompt) []map[string]any {
	records := make([]map[string]any, 0, len(prompts))
	for _, prompt := range prompts {
		records = append(records, map[string]any{
			"guid":        prompt.GUID,
			"order":       prompt.Order,
			"prompt_text": prompt.PromptText,
		})
	}
	return records
}

func filterCategories(candidates []candidate, categories []string) []candidate {
	if len(categories) == 0 {
		return candidates
	}
	allowed := map[string]bool{}
	for _, category := range categories {
		allowed[strings.ToLower(category)] = true
	}
	filtered := []candidate{}
	for _, item := range candidates {
		if allowed[strings.ToLower(fmt.Sprint(item["category"]))] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterDateWindow(candidates []candidate, startText string, endText string, excludeUndated bool) ([]candidate, error) {
	if startText == "" && endText == "" {
		return candidates, nil
	}
	start := time.Time{}
	end := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	var err error
	if startText != "" {
		start, err = time.Parse("2006-01-02", startText)
		if err != nil {
			return nil, fmt.Errorf("invalid --date-window-start: %w", err)
		}
	}
	if endText != "" {
		end, err = time.Parse("2006-01-02", endText)
		if err != nil {
			return nil, fmt.Errorf("invalid --date-window-end: %w", err)
		}
	}
	if start.After(end) {
		return nil, fmt.Errorf("--date-window-start must be before or equal to --date-window-end")
	}
	filtered := []candidate{}
	for _, item := range candidates {
		itemStart, itemEnd, ok := parseDateRangeText(fmt.Sprint(item["date_text"]))
		if !ok {
			if !excludeUndated {
				filtered = append(filtered, item)
			}
			continue
		}
		if !itemStart.After(end) && !itemEnd.Before(start) {
			copyItem := candidate{}
			for key, value := range item {
				copyItem[key] = value
			}
			copyItem["date_window_start"] = itemStart.Format("2006-01-02")
			copyItem["date_window_end"] = itemEnd.Format("2006-01-02")
			filtered = append(filtered, copyItem)
		}
	}
	return filtered, nil
}

func parseDateRangeText(value string) (time.Time, time.Time, bool) {
	text := normalizeSpace(value)
	text = strings.ReplaceAll(text, "\u2013", "-")
	text = strings.ReplaceAll(text, "\u2014", "-")
	text = regexp.MustCompile(`(?i)^select dates\s+`).ReplaceAllString(text, "")
	crossMonth := regexp.MustCompile(`(?i)\b([a-z]+)\s+(\d{1,2})\s*-\s*([a-z]+)\s+(\d{1,2}),\s*(\d{4})\b`)
	sameMonth := regexp.MustCompile(`(?i)\b([a-z]+)\s+(\d{1,2})\s*-\s*(\d{1,2}),\s*(\d{4})\b`)
	singleDay := regexp.MustCompile(`(?i)\b([a-z]+)\s+(\d{1,2}),\s*(\d{4})\b`)
	if match := crossMonth.FindStringSubmatch(text); match != nil {
		startMonth, ok1 := months[strings.ToLower(match[1])]
		endMonth, ok2 := months[strings.ToLower(match[3])]
		if !ok1 || !ok2 {
			return time.Time{}, time.Time{}, false
		}
		startDay, ok1 := atoi(match[2])
		endDay, ok2 := atoi(match[4])
		year, ok3 := atoi(match[5])
		if !ok1 || !ok2 || !ok3 {
			return time.Time{}, time.Time{}, false
		}
		return dateOnly(year, startMonth, startDay), dateOnly(year, endMonth, endDay), true
	}
	if match := sameMonth.FindStringSubmatch(text); match != nil {
		month, ok := months[strings.ToLower(match[1])]
		if !ok {
			return time.Time{}, time.Time{}, false
		}
		startDay, ok1 := atoi(match[2])
		endDay, ok2 := atoi(match[3])
		year, ok3 := atoi(match[4])
		if !ok1 || !ok2 || !ok3 {
			return time.Time{}, time.Time{}, false
		}
		return dateOnly(year, month, startDay), dateOnly(year, month, endDay), true
	}
	if match := singleDay.FindStringSubmatch(text); match != nil {
		month, ok := months[strings.ToLower(match[1])]
		if !ok {
			return time.Time{}, time.Time{}, false
		}
		dayNum, ok1 := atoi(match[2])
		year, ok2 := atoi(match[3])
		if !ok1 || !ok2 {
			return time.Time{}, time.Time{}, false
		}
		day := dateOnly(year, month, dayNum)
		return day, day, true
	}
	return time.Time{}, time.Time{}, false
}

func summarizeCategories(candidates []candidate) []categorySummary {
	counts := map[string]int{}
	for _, item := range candidates {
		counts[fmt.Sprint(item["category"])]++
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	summary := make([]categorySummary, 0, len(names))
	for _, name := range names {
		summary = append(summary, categorySummary{Name: name, Count: counts[name]})
	}
	return summary
}

func doctorPayload(data payload) (map[string]any, int) {
	required := map[string]bool{"Explore NYNJ": false, "Fan Experiences": false, "Watch Parties": false}
	for _, category := range data.Categories {
		if _, ok := required[category.Name]; ok {
			required[category.Name] = true
		}
	}
	missing := []string{}
	for category, present := range required {
		if !present {
			missing = append(missing, category)
		}
	}
	sort.Strings(missing)
	status := "healthy"
	exitCode := 0
	if len(missing) > 0 || len(data.Candidates) == 0 {
		status = "needs-review"
		exitCode = 2
	}
	return map[string]any{
		"status":             status,
		"source":             "nynj-world-cup-concierge",
		"candidate_count":    len(data.Candidates),
		"categories":         data.Categories,
		"missing_categories": missing,
	}, exitCode
}

func asMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringValue(item map[string]any, key string, fallback string) string {
	value, ok := item[key]
	if !ok || value == nil {
		return fallback
	}
	text := fmt.Sprint(value)
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func intValue(item map[string]any, key string, fallback int) int {
	switch value := item[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return fallback
	}
}

func boolValue(item map[string]any, key string) bool {
	value, _ := item[key].(bool)
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func dateOnly(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func atoi(value string) (int, bool) {
	result, err := strconv.Atoi(value)
	return result, err == nil
}
