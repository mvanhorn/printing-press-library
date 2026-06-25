// Copyright 2026 adbonnet and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored compound command (NOT generator-emitted). Lives in its own file
// so it survives generator regen, the same convention as
// internal/config/podcastindex_secret.go. Registered from channel_workflow.go.
// See .printing-press-patches/find-appearances-and-auth-config.json.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// newWorkflowFindAppearancesCmd builds the resolution-layer "find a person's or
// company's appearances" compound command. PodcastIndex has no episode-content
// or guest search — byterm matches feed metadata, byperson matches person tags —
// so it cannot discover an untagged guest appearance from a name alone. The
// discovery step (name -> which show) is a web search performed upstream; this
// command takes the resulting show(s) and resolves them to the matching
// episodes' enclosure URLs, collapsing the proven byterm->byfeedid->filter chain
// into one call.
func newWorkflowFindAppearancesCmd(flags *rootFlags) *cobra.Command {
	var matchTerms []string
	var shows []string
	var feedIDs []string
	var maxFeeds int
	var maxPerFeed int
	var includeByperson bool

	cmd := &cobra.Command{
		Use:   "find-appearances",
		Short: "Find a person's or company's appearances within known shows (resolution layer)",
		Long: `find-appearances resolves "where did <person/company> appear" for shows you
already know. PodcastIndex cannot discover a guest from a name — it has no
episode-content search, and byperson only matches explicit person tags — so the
name->show step is a web search done upstream. Supply the show(s) here.

For each --show (resolved via search-byterm) and each explicit --feed, it lists
the feed's episodes and keeps those whose title or description contains a --match
term, emitting feedId, feedTitle, guid, datePublished and enclosureUrl — ready to
hand to a transcriber. Results are deduped by guid. --byperson additionally folds
in PodcastIndex person-tag hits (noisy on common first names; still filtered
through --match).`,
		Example: `  # A guest on a known show
  podcastindex-pp-cli workflow find-appearances --match "Arthur Mensch" --show "Big Technology"

  # Company + founder across a candidate show, as JSON
  podcastindex-pp-cli workflow find-appearances --match "Implicity" --match "Arnaud Rosier" \
    --show "Med in Tech" --json --select guid,title,enclosureUrl

  # Scan an explicit feed id directly
  podcastindex-pp-cli workflow find-appearances --match "Rosier" --feed 4712435`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(matchTerms) == 0 {
				return fmt.Errorf("at least one --match term is required")
			}
			if len(shows) == 0 && len(feedIDs) == 0 && !includeByperson {
				return fmt.Errorf("supply at least one --show or --feed " +
					"(PodcastIndex cannot discover a show from a name — find the show via web search first), " +
					"or pass --byperson to use person-tag search")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			lowerMatches := make([]string, 0, len(matchTerms))
			for _, m := range matchTerms {
				if t := strings.ToLower(strings.TrimSpace(m)); t != "" {
					lowerMatches = append(lowerMatches, t)
				}
			}

			// Candidate feed IDs: explicit --feed, plus those resolved from each
			// --show via search-byterm. Ordered, deduped. feedTitles backfills
			// feedTitle on emitted episodes (episodes/byfeedid rows do not always
			// carry it).
			feedSet := map[string]bool{}
			feedTitles := map[string]string{}
			var feedOrder []string
			addFeed := func(id string) {
				id = strings.TrimSpace(id)
				if id == "" || feedSet[id] {
					return
				}
				feedSet[id] = true
				feedOrder = append(feedOrder, id)
			}
			for _, f := range feedIDs {
				addFeed(f)
			}
			for _, show := range shows {
				params := map[string]string{"q": formatCLIParamValue(show)}
				if maxFeeds > 0 {
					params["max"] = formatCLIParamValue(maxFeeds)
				}
				raw, err := c.Get(ctx, "/search/byterm", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, item := range extractAppearanceItems(raw) {
					if id := jsonFieldString(item, "id"); id != "" {
						addFeed(id)
						if t := jsonFieldString(item, "title"); t != "" {
							feedTitles[id] = t
						}
					}
				}
			}

			seen := map[string]bool{}
			matched := []map[string]any{}

			scanEpisodes := func(items []json.RawMessage) {
				for _, raw := range items {
					var ep map[string]any
					if json.Unmarshal(raw, &ep) != nil {
						continue
					}
					hay := strings.ToLower(asAppearanceString(ep["title"]) + " \n " + asAppearanceString(ep["description"]))
					if !appearanceContainsAny(hay, lowerMatches) {
						continue
					}
					key := asAppearanceString(ep["guid"])
					if key == "" {
						key = asAppearanceString(ep["enclosureUrl"])
					}
					if key != "" {
						if seen[key] {
							continue
						}
						seen[key] = true
					}
					feedTitle := ep["feedTitle"]
					if asAppearanceString(feedTitle) == "" {
						if t, ok := feedTitles[asAppearanceString(ep["feedId"])]; ok {
							feedTitle = t
						}
					}
					matched = append(matched, map[string]any{
						"feedId":              ep["feedId"],
						"feedTitle":           feedTitle,
						"guid":                ep["guid"],
						"title":               ep["title"],
						"datePublished":       ep["datePublished"],
						"datePublishedPretty": ep["datePublishedPretty"],
						"enclosureUrl":        ep["enclosureUrl"],
					})
				}
			}

			for _, id := range feedOrder {
				params := map[string]string{"id": id, "fulltext": "true"}
				if maxPerFeed > 0 {
					params["max"] = formatCLIParamValue(maxPerFeed)
				}
				raw, err := c.Get(ctx, "/episodes/byfeedid", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				scanEpisodes(extractAppearanceItems(raw))
			}

			if includeByperson {
				for _, term := range matchTerms {
					params := map[string]string{"q": formatCLIParamValue(term), "fulltext": "true"}
					raw, err := c.Get(ctx, "/search/byperson", params)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					scanEpisodes(extractAppearanceItems(raw))
				}
			}

			if len(matched) == 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.OutOrStdout(),
					"No matching appearances found. Verify the --show name with "+
						"`find search-byterm --q \"<show>\"`, or widen --match.")
				return nil
			}

			out, err := json.Marshal(matched)
			if err != nil {
				return err
			}
			data := json.RawMessage(out)
			if flags.selectFields != "" {
				data = filterFields(data, flags.selectFields)
			} else if flags.compact {
				data = compactFields(data)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAutoTable(cmd.OutOrStdout(), matched)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringArrayVar(&matchTerms, "match", nil, "Person/company name to match in an episode's title or description (repeatable; OR-matched). Required.")
	cmd.Flags().StringArrayVar(&shows, "show", nil, "Show name to resolve via search-byterm, then scan its episodes (repeatable)")
	cmd.Flags().StringArrayVar(&feedIDs, "feed", nil, "Explicit PodcastIndex feed ID to scan directly (repeatable)")
	cmd.Flags().IntVar(&maxFeeds, "max-feeds", 10, "Max candidate feeds to take per --show")
	// Default scans full feed history: an appearance is often months old, and the
	// API's small default page would miss it on a high-frequency show. 0 = API default.
	cmd.Flags().IntVar(&maxPerFeed, "max-per-feed", 1000, "Max episodes to scan per feed (0 = API default)")
	cmd.Flags().BoolVar(&includeByperson, "byperson", false, "Also fold in PodcastIndex person-tag hits (noisy on common first names; filtered through --match)")

	return cmd
}

// extractAppearanceItems pulls the list of rows from a PodcastIndex response,
// whether the body is a bare JSON array or an envelope keyed by feeds/items/
// data/results. Kept local so this command does not couple to an unexported
// shared extractor's signature.
func extractAppearanceItems(raw json.RawMessage) []json.RawMessage {
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr
	}
	var env map[string]json.RawMessage
	if json.Unmarshal(raw, &env) != nil {
		return nil
	}
	for _, key := range []string{"feeds", "items", "data", "results"} {
		if v, ok := env[key]; ok {
			var inner []json.RawMessage
			if json.Unmarshal(v, &inner) == nil && len(inner) > 0 {
				return inner
			}
		}
	}
	return nil
}

func jsonFieldString(raw json.RawMessage, key string) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	return asAppearanceString(m[key])
}

// asAppearanceString renders a decoded JSON value as a string, printing
// integer-valued numbers (feed/episode IDs) without a decimal point.
func asAppearanceString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func appearanceContainsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
