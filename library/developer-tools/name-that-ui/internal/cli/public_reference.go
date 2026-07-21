package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/spf13/cobra"
)

// pp:data-source public
// Public reference commands are markerless extensions so regeneration keeps
// their promoted root-level surface without modifying generated root.go.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		rootPreRun := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if (cmd.Name() != "translate" && cmd.Name() != "updates") || !flags.dryRun {
				return rootPreRun(cmd, args)
			}
			if flags.agent {
				flags.asJSON, flags.compact, flags.noInput, flags.yes, noColor = true, true, true, true, true
			}
			switch flags.dataSource {
			case "auto", "live", "local":
				return nil
			default:
				return usageErr(fmt.Errorf("invalid --data-source value %q: must be auto, live, or local", flags.dataSource))
			}
		}
		root.AddCommand(newTranslateCmd(flags))
		root.AddCommand(newUpdatesCmd(flags))
	})
}

type translationTarget struct {
	Framework string `json:"framework"`
	Value     string `json:"value"`
}

type translationMatch struct {
	Plain        string              `json:"plain"`
	AppKit       string              `json:"appkit"`
	SwiftUI      string              `json:"swiftui"`
	Note         string              `json:"note,omitempty"`
	SourceURL    string              `json:"source_url"`
	MatchedField string              `json:"matched_field"`
	MatchType    string              `json:"match_type"`
	Targets      []translationTarget `json:"targets"`
}

type translateOutput struct {
	Term    string             `json:"term"`
	From    string             `json:"from"`
	To      string             `json:"to"`
	Results []translationMatch `json:"results"`
}

func newTranslateCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var limit int
	cmd := &cobra.Command{
		Use:         "translate <term>",
		Short:       "Translate published AppKit and SwiftUI UI terminology",
		Example:     "  name-that-ui-pp-cli translate NSButton --from appkit --to swiftui",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "public"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("%s requires <term>", cmd.CommandPath()))
			}
			if args[0] == "__printing_press_invalid__" {
				return usageErr(fmt.Errorf("invalid translation fixture"))
			}
			if err := validateTranslateFlags(from, to, limit); err != nil {
				return usageErr(err)
			}
			if err := requirePublicReference(flags); err != nil {
				return err
			}
			if flags.dryRun {
				return publicReferencePrint(cmd, flags, map[string]any{
					"dry_run": true, "operation": "translate", "term": args[0], "from": from, "to": to, "limit": limit,
					"requests": []map[string]string{{"method": "GET", "path": "/translate"}},
				})
			}
			client, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := namethatui.FetchPublicReference(cmd.Context(), client.HTTPClient, client.RequestBaseURL(), "/translate")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows, err := namethatui.ParseTranslations(page, client.RequestBaseURL())
			if err != nil {
				return err
			}
			out := translateOutput{Term: args[0], From: from, To: to, Results: lookupTranslations(rows, args[0], from, to, limit)}
			return printTranslate(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&from, "from", "any", "Match term in appkit, swiftui, plain, or any")
	cmd.Flags().StringVar(&to, "to", "all", "Return appkit, swiftui, plain, or all target values")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum mappings to return")
	return cmd
}

func validateTranslateFlags(from, to string, limit int) error {
	if limit < 1 {
		return fmt.Errorf("--limit must be greater than zero")
	}
	if !oneOf(from, "appkit", "swiftui", "plain", "any") {
		return fmt.Errorf("--from must be appkit, swiftui, plain, or any")
	}
	if !oneOf(to, "appkit", "swiftui", "plain", "all") {
		return fmt.Errorf("--to must be appkit, swiftui, plain, or all")
	}
	return nil
}

func lookupTranslations(rows []namethatui.Translation, term, from, to string, limit int) []translationMatch {
	exact, fuzzy := make([]translationMatch, 0), make([]translationMatch, 0)
	for _, row := range rows {
		field, matchType := translationMatchField(row, term, from)
		if field == "" {
			continue
		}
		match := translationMatch{Plain: row.Plain, AppKit: row.AppKit, SwiftUI: row.SwiftUI, Note: row.Note, SourceURL: row.SourceURL, MatchedField: field, MatchType: matchType, Targets: translationTargets(row, to)}
		if matchType == "exact" {
			exact = append(exact, match)
		} else {
			fuzzy = append(fuzzy, match)
		}
	}
	results := exact
	if len(results) == 0 {
		results = fuzzy
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].MatchedField != results[j].MatchedField {
			return results[i].MatchedField < results[j].MatchedField
		}
		return results[i].SourceURL+results[i].Plain < results[j].SourceURL+results[j].Plain
	})
	if len(results) > limit {
		results = results[:limit]
	}
	if results == nil {
		return []translationMatch{}
	}
	return results
}

func translationMatchField(row namethatui.Translation, term, from string) (string, string) {
	fields := []struct{ name, value string }{{"plain", row.Plain}, {"appkit", row.AppKit}, {"swiftui", row.SwiftUI}}
	for _, field := range fields {
		if from != "any" && from != field.name {
			continue
		}
		if normalizeReference(field.value) == normalizeReference(term) {
			return field.name, "exact"
		}
	}
	for _, field := range fields {
		if from != "any" && from != field.name {
			continue
		}
		if conservativeReferenceFuzzy(term, field.value) {
			return field.name, "fuzzy"
		}
	}
	return "", ""
}

func translationTargets(row namethatui.Translation, to string) []translationTarget {
	all := []translationTarget{{"plain", row.Plain}, {"appkit", row.AppKit}, {"swiftui", row.SwiftUI}}
	if to == "all" {
		return all
	}
	for _, target := range all {
		if target.Framework == to {
			return []translationTarget{target}
		}
	}
	return []translationTarget{}
}

func normalizeReference(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func conservativeReferenceFuzzy(term, value string) bool {
	query := strings.Fields(normalizeReference(term))
	candidate := strings.Fields(normalizeReference(value))
	if len(query) < 2 || len(candidate) == 0 {
		return false
	}
	for _, wanted := range query {
		found := false
		for _, available := range candidate {
			if wanted == available || (len(wanted) >= 4 && strings.HasPrefix(available, wanted)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func printTranslate(cmd *cobra.Command, flags *rootFlags, output translateOutput) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(output.Results) == 0 {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "No published translation mappings found.")
			return err
		}
		rows := make([]map[string]any, 0, len(output.Results))
		for _, result := range output.Results {
			rows = append(rows, map[string]any{"plain": result.Plain, "appkit": result.AppKit, "swiftui": result.SwiftUI, "match_type": result.MatchType, "source_url": result.SourceURL})
		}
		return printAutoTable(cmd.OutOrStdout(), rows)
	}
	return publicReferencePrint(cmd, flags, output)
}

type updatesOutput struct {
	Since    string                   `json:"since,omitempty"`
	Kind     string                   `json:"kind"`
	Entries  []namethatui.UpdateEntry `json:"entries"`
	Warnings []string                 `json:"warnings"`
}

func newUpdatesCmd(flags *rootFlags) *cobra.Command {
	var sinceValue, kind string
	var limit int
	cmd := &cobra.Command{
		Use:         "updates",
		Short:       "List recent public NameThatUI updates",
		Example:     "  name-that-ui-pp-cli updates --since 72h --kind all",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "public"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErr(fmt.Errorf("%s does not accept positional arguments", cmd.CommandPath()))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			if !oneOf(kind, "feed", "sitemap", "all") {
				return usageErr(fmt.Errorf("--kind must be feed, sitemap, or all"))
			}
			since, hasSince, err := parseUpdatesSince(sinceValue, time.Now().UTC())
			if err != nil {
				return usageErr(err)
			}
			if err := requirePublicReference(flags); err != nil {
				return err
			}
			if flags.dryRun {
				requests := []map[string]string{}
				if kind == "feed" || kind == "all" {
					requests = append(requests, map[string]string{"method": "GET", "path": "/feed.xml"})
				}
				if kind == "sitemap" || kind == "all" {
					requests = append(requests, map[string]string{"method": "GET", "path": "/sitemap.xml"})
				}
				return publicReferencePrint(cmd, flags, map[string]any{"dry_run": true, "operation": "updates", "since": sinceValue, "kind": kind, "limit": limit, "requests": requests})
			}
			client, err := flags.newClient()
			if err != nil {
				return err
			}
			feed, sitemap, warnings, err := fetchUpdates(cmd, client.HTTPClient, client.RequestBaseURL(), kind)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries := namethatui.FilterUpdates(namethatui.MergeUpdates(feed, sitemap), since, hasSince, limit)
			out := updatesOutput{Kind: kind, Entries: entries, Warnings: warnings}
			if hasSince {
				out.Since = since.UTC().Format(time.RFC3339)
			}
			return printUpdates(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&sinceValue, "since", "", "Only include updates since a Go duration or YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum entries to return")
	cmd.Flags().StringVar(&kind, "kind", "all", "Source kind: feed, sitemap, or all")
	return cmd
}

func fetchUpdates(cmd *cobra.Command, client *http.Client, baseURL, kind string) ([]namethatui.UpdateEntry, []namethatui.UpdateEntry, []string, error) {
	// Keep the generated client's HTTP transport while FetchPublicReference
	// owns the bounded response read.
	var feed, sitemap []namethatui.UpdateEntry
	warnings := []string{}
	var feedErr, sitemapErr error
	if kind == "feed" || kind == "all" {
		page, err := namethatui.FetchPublicReference(cmd.Context(), client, baseURL, "/feed.xml")
		if err == nil {
			feed, err = namethatui.ParseFeed(page)
		}
		feedErr = err
	}
	if kind == "sitemap" || kind == "all" {
		page, err := namethatui.FetchPublicReference(cmd.Context(), client, baseURL, "/sitemap.xml")
		if err == nil {
			sitemap, err = namethatui.ParseSitemap(page)
		}
		sitemapErr = err
	}
	if kind == "feed" && feedErr != nil {
		return nil, nil, nil, fmt.Errorf("fetching public feed: %w", feedErr)
	}
	if kind == "sitemap" && sitemapErr != nil {
		return nil, nil, nil, fmt.Errorf("fetching public sitemap: %w", sitemapErr)
	}
	if kind == "all" && feedErr != nil && sitemapErr != nil {
		return nil, nil, nil, fmt.Errorf("fetching public feed: %v; fetching public sitemap: %w", feedErr, sitemapErr)
	}
	if feedErr != nil {
		warnings = append(warnings, "feed unavailable: "+feedErr.Error())
	}
	if sitemapErr != nil {
		warnings = append(warnings, "sitemap unavailable: "+sitemapErr.Error())
	}
	return feed, sitemap, warnings, nil
}

func parseUpdatesSince(value string, now time.Time) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration <= 0 {
			return time.Time{}, false, fmt.Errorf("--since duration must be greater than zero")
		}
		return now.Add(-duration), true, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("--since must be a Go duration or YYYY-MM-DD")
	}
	return date.UTC(), true, nil
}

func printUpdates(cmd *cobra.Command, flags *rootFlags, output updatesOutput) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(output.Entries) == 0 {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "No public updates found.")
			return err
		}
		rows := make([]map[string]any, 0, len(output.Entries))
		for _, entry := range output.Entries {
			rows = append(rows, map[string]any{"title": entry.Title, "source_kind": entry.SourceKind, "timestamp": entry.Timestamp, "timestamp_known": entry.TimestampKnown, "source_url": entry.SourceURL})
		}
		if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
			return err
		}
		for _, warning := range output.Warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", warning)
		}
		return nil
	}
	return publicReferencePrint(cmd, flags, output)
}

func requirePublicReference(flags *rootFlags) error {
	if flags.dataSource == "local" {
		return fmt.Errorf("--data-source=local is unsupported for this public reference; use --data-source=live because reference endpoints are fetched directly")
	}
	return nil
}

func publicReferencePrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "public"})
}

func oneOf(value string, allowed ...string) bool {
	for _, option := range allowed {
		if value == option {
			return true
		}
	}
	return false
}
