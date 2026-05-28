// Copyright 2026 Greg Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Magic-moment compound commands sit on top of /episodes/search.
// Each one stitches multiple primitive calls into a workflow that the
// raw REST API doesn't directly expose: mention monitoring, guest history,
// topic trend charting, idea sourcing (asks/complaints/praise), and a
// sponsor-overlap heuristic.

type psPodcast struct {
	PodcastID         string  `json:"podcast_id"`
	PodcastName       string  `json:"podcast_name"`
	PodcastURL        string  `json:"podcast_url"`
	PodcastReachScore float64 `json:"podcast_reach_score"`
}

type psEpisode struct {
	EpisodeID         string    `json:"episode_id"`
	EpisodeTitle      string    `json:"episode_title"`
	EpisodeURL        string    `json:"episode_url"`
	EpisodeTranscript string    `json:"episode_transcript"`
	EpisodeWordCount  int       `json:"episode_word_count"`
	PostedAt          string    `json:"posted_at"`
	Podcast           psPodcast `json:"podcast"`
}

type psSearchResp struct {
	Episodes []psEpisode `json:"episodes"`
}

func parseSince(spec string) (string, error) {
	if spec == "" {
		return "", nil
	}
	if len(spec) < 2 {
		return "", fmt.Errorf("invalid --since %q (use 7d, 30d, 12mo, 1y)", spec)
	}
	unit := spec[len(spec)-1]
	num := spec[:len(spec)-1]
	if strings.HasSuffix(spec, "mo") {
		unit = 'M'
		num = spec[:len(spec)-2]
	}
	var n int
	if _, err := fmt.Sscanf(num, "%d", &n); err != nil {
		return "", fmt.Errorf("invalid --since %q", spec)
	}
	now := time.Now().UTC()
	var t time.Time
	switch unit {
	case 'd':
		t = now.AddDate(0, 0, -n)
	case 'w':
		t = now.AddDate(0, 0, -7*n)
	case 'M':
		t = now.AddDate(0, -n, 0)
	case 'y':
		t = now.AddDate(-n, 0, 0)
	default:
		return "", fmt.Errorf("invalid --since unit in %q (use d, w, mo, y)", spec)
	}
	return t.Format("2006-01-02"), nil
}

func searchEpisodesAll(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, query, since string, maxPages int) ([]psEpisode, error) {
	postedAfter, err := parseSince(since)
	if err != nil {
		return nil, err
	}
	var all []psEpisode
	for page := 1; page <= maxPages; page++ {
		params := map[string]string{
			"query":    query,
			"per_page": "50",
			"page":     fmt.Sprintf("%d", page),
		}
		if postedAfter != "" {
			params["posted_after"] = postedAfter
		}
		raw, err := c.Get("/episodes/search", params)
		if err != nil {
			return nil, err
		}
		var resp psSearchResp
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, err
		}
		if len(resp.Episodes) == 0 {
			break
		}
		all = append(all, resp.Episodes...)
		if len(resp.Episodes) < 50 {
			break
		}
	}
	return all, nil
}

func extractSnippet(transcript, term string, ctx int) string {
	if transcript == "" || term == "" {
		return ""
	}
	low := strings.ToLower(transcript)
	idx := strings.Index(low, strings.ToLower(term))
	if idx < 0 {
		return ""
	}
	start := idx - ctx
	if start < 0 {
		start = 0
	}
	end := idx + len(term) + ctx
	if end > len(transcript) {
		end = len(transcript)
	}
	s := transcript[start:end]
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if start > 0 {
		s = "…" + s
	}
	if end < len(transcript) {
		s = s + "…"
	}
	return s
}

// ------------------------------------------------------------------ mentions

func newMentionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mentions <term>",
		Short: "Every podcast that mentioned a brand or term, ranked by audience",
		Long: `Search every transcript for a term and aggregate by podcast. Returns episodes
ranked by reach, with a one-line snippet of the surrounding transcript.

  podscan-pp-cli mentions "Anthropic" --since 30d
  podscan-pp-cli mentions "vibe coding" --since 7d --rank reach --json`,
		Args: cobra.ExactArgs(1),
	}
	var since, rank string
	var snippetCtx, maxPages int
	cmd.Flags().StringVar(&since, "since", "30d", "Look back window (e.g. 7d, 30d, 12mo)")
	cmd.Flags().StringVar(&rank, "rank", "reach", "Rank by: reach | recency | rating")
	cmd.Flags().IntVar(&snippetCtx, "context", 80, "Characters of transcript context around the mention")
	cmd.Flags().IntVar(&maxPages, "max-pages", 4, "Cap on result pages (50 per page)")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		term := args[0]
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		eps, err := searchEpisodesAll(c, term, since, maxPages)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		switch rank {
		case "reach":
			sort.SliceStable(eps, func(i, j int) bool { return eps[i].Podcast.PodcastReachScore > eps[j].Podcast.PodcastReachScore })
		case "rating":
			sort.SliceStable(eps, func(i, j int) bool { return eps[i].Podcast.PodcastReachScore > eps[j].Podcast.PodcastReachScore })
		case "recency":
			sort.SliceStable(eps, func(i, j int) bool { return eps[i].PostedAt > eps[j].PostedAt })
		}
		type row struct {
			Rank      int    `json:"rank"`
			Podcast   string `json:"podcast"`
			Reach     int64  `json:"reach"`
			Episode   string `json:"episode"`
			EpisodeID string `json:"episode_id"`
			Date      string `json:"date"`
			Snippet   string `json:"snippet"`
		}
		seenPod := map[string]bool{}
		out := []row{}
		for _, e := range eps {
			if seenPod[e.Podcast.PodcastID] {
				continue
			}
			seenPod[e.Podcast.PodcastID] = true
			out = append(out, row{
				Rank:      len(out) + 1,
				Podcast:   e.Podcast.PodcastName,
				Reach:     int64(e.Podcast.PodcastReachScore * 1000),
				Episode:   e.EpisodeTitle,
				EpisodeID: e.EpisodeID,
				Date:      strings.SplitN(strings.ReplaceAll(e.PostedAt, "T", " "), " ", 2)[0],
				Snippet:   extractSnippet(e.EpisodeTranscript, term, snippetCtx),
			})
		}
		summary := map[string]any{
			"term":              term,
			"since":             since,
			"matching_episodes": len(eps),
			"unique_podcasts":   len(out),
			"results":           out,
		}
		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			b, _ := json.MarshalIndent(summary, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Mentions of %q in last %s — %d episodes across %d podcasts\n\n",
			term, since, len(eps), len(out))
		for _, r := range out {
			if r.Rank > 25 {
				fmt.Fprintf(cmd.OutOrStdout(), "  …and %d more (use --json for full list)\n", len(out)-25)
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%2d. %s — %s\n      %s\n      \"%s\"\n",
				r.Rank, r.Podcast, r.Date, r.Episode, r.Snippet)
		}
		return nil
	}
	return cmd
}

// --------------------------------------------------------------------- guest

func newGuestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guest <name>",
		Short: "Every podcast appearance of a person, with topical analysis",
		Long: `Find every episode where a guest is mentioned in the title, description, or
transcript. Optionally filter quote count by an additional include term.

  podscan-pp-cli guest "Tyler Cowen" --since 1y
  podscan-pp-cli guest "Yann LeCun" --since 6mo --include-quotes "AI"`,
		Args: cobra.ExactArgs(1),
	}
	var since, includeQuotes string
	var maxPages int
	cmd.Flags().StringVar(&since, "since", "1y", "Look back window")
	cmd.Flags().StringVar(&includeQuotes, "include-quotes", "", "Count occurrences of this term in each transcript")
	cmd.Flags().IntVar(&maxPages, "max-pages", 4, "Cap on result pages")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		name := args[0]
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		eps, err := searchEpisodesAll(c, name, since, maxPages)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		type row struct {
			Show      string `json:"show"`
			Episode   string `json:"episode"`
			EpisodeID string `json:"episode_id"`
			Date      string `json:"date"`
			Quotes    int    `json:"quotes,omitempty"`
		}
		out := []row{}
		for _, e := range eps {
			r := row{Show: e.Podcast.PodcastName, Episode: e.EpisodeTitle, EpisodeID: e.EpisodeID,
				Date: strings.SplitN(strings.ReplaceAll(e.PostedAt, "T", " "), " ", 2)[0]}
			if includeQuotes != "" {
				r.Quotes = strings.Count(strings.ToLower(e.EpisodeTranscript), strings.ToLower(includeQuotes))
			}
			out = append(out, r)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
		summary := map[string]any{"guest": name, "since": since, "appearances": len(out), "results": out}
		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			b, _ := json.MarshalIndent(summary, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s — %d podcast appearances in the last %s\n\n", name, len(out), since)
		for _, r := range out {
			if includeQuotes != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s | %s | %s | %d × %q\n", r.Date, r.Show, r.Episode, r.Quotes, includeQuotes)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s | %s | %s\n", r.Date, r.Show, r.Episode)
			}
		}
		return nil
	}
	return cmd
}

// ----------------------------------------------------------------- topic trend

func newTopicCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "topic", Short: "Topic-level trend analysis"}
	cmd.AddCommand(newTopicTrendCmd(flags))
	return cmd
}

func newTopicTrendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend <term>",
		Short: "Track an idea's rise or decline across the podcast ecosystem",
		Long: `Bucket episodes mentioning a term by week or month and render an ASCII chart.

  podscan-pp-cli topic trend "vibe coding" --window 365d --bucket weekly
  podscan-pp-cli topic trend "agentic AI" --window 90d --bucket weekly --json`,
		Args: cobra.ExactArgs(1),
	}
	var window, bucket string
	var maxPages int
	cmd.Flags().StringVar(&window, "window", "180d", "Total time window (e.g. 90d, 365d)")
	cmd.Flags().StringVar(&bucket, "bucket", "weekly", "Bucket: daily | weekly | monthly")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Cap on result pages")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		term := args[0]
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		eps, err := searchEpisodesAll(c, term, window, maxPages)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		buckets := map[string]int{}
		for _, e := range eps {
			d := strings.SplitN(strings.ReplaceAll(e.PostedAt, "T", " "), " ", 2)[0]
			t, err := time.Parse("2006-01-02", d)
			if err != nil {
				continue
			}
			var key string
			switch bucket {
			case "daily":
				key = d
			case "monthly":
				key = t.Format("2006-01")
			default:
				y, w := t.ISOWeek()
				key = fmt.Sprintf("%04d-W%02d", y, w)
			}
			buckets[key]++
		}
		keys := make([]string, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		max := 0
		for _, k := range keys {
			if buckets[k] > max {
				max = buckets[k]
			}
		}
		type bucketRow struct {
			Bucket string `json:"bucket"`
			Count  int    `json:"count"`
		}
		rows := make([]bucketRow, 0, len(keys))
		for _, k := range keys {
			rows = append(rows, bucketRow{k, buckets[k]})
		}
		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			b, _ := json.MarshalIndent(map[string]any{
				"term": term, "window": window, "bucket": bucket,
				"total_episodes": len(eps), "buckets": rows,
			}, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Mentions of %q across all podcasts (%s, %s buckets)\n\n", term, window, bucket)
		bars := []rune("▁▂▃▄▅▆▇█")
		var line strings.Builder
		for _, k := range keys {
			n := buckets[k]
			idx := 0
			if max > 0 {
				idx = (n * (len(bars) - 1)) / max
			}
			line.WriteRune(bars[idx])
		}
		if len(keys) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n  %s%s%s\n",
				keys[0], keys[len(keys)-1], strings.Repeat(" ", 0), line.String(), "")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nTotal episodes: %d   Peak bucket: %d\n", len(eps), max)
		return nil
	}
	return cmd
}

// ----------------------------------------------------------------------- asks

func newAsksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asks <topic>",
		Short: "Questions hosts and guests are asking about a topic",
		Long: `Idea sourcing: extracts question-shaped sentences from transcripts that mention
the topic. Useful for product positioning, content ideation, market research.

  podscan-pp-cli asks "small business AI" --since 30d`,
		Args: cobra.ExactArgs(1),
	}
	var since string
	var maxPages, maxQuestions int
	cmd.Flags().StringVar(&since, "since", "30d", "Look back window")
	cmd.Flags().IntVar(&maxPages, "max-pages", 4, "Cap on result pages")
	cmd.Flags().IntVar(&maxQuestions, "max-questions", 30, "Max questions to surface")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		topic := args[0]
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		eps, err := searchEpisodesAll(c, topic, since, maxPages)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		type qRow struct {
			Question string `json:"question"`
			Show     string `json:"show"`
			Episode  string `json:"episode"`
		}
		out := []qRow{}
		seen := map[string]bool{}
		for _, e := range eps {
			t := e.EpisodeTranscript
			if t == "" {
				continue
			}
			low := strings.ToLower(t)
			if !strings.Contains(low, strings.ToLower(topic)) {
				continue
			}
			// Split on '?'; take the trailing 140 chars of each segment.
			parts := strings.Split(t, "?")
			for i := 0; i < len(parts)-1; i++ {
				seg := parts[i]
				start := len(seg) - 140
				if start < 0 {
					start = 0
				}
				q := strings.TrimSpace(seg[start:]) + "?"
				q = strings.Join(strings.Fields(q), " ")
				if len(q) < 20 || len(q) > 200 {
					continue
				}
				lq := strings.ToLower(q)
				if !strings.Contains(lq, strings.ToLower(topic)) {
					continue
				}
				if seen[lq] {
					continue
				}
				seen[lq] = true
				out = append(out, qRow{Question: q, Show: e.Podcast.PodcastName, Episode: e.EpisodeTitle})
				if len(out) >= maxQuestions {
					break
				}
			}
			if len(out) >= maxQuestions {
				break
			}
		}
		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			b, _ := json.MarshalIndent(map[string]any{
				"topic": topic, "since": since, "questions": out,
			}, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Questions about %q in last %s — %d found\n\n", topic, since, len(out))
		for i, r := range out {
			fmt.Fprintf(cmd.OutOrStdout(), "%2d. %s\n     — %s · %s\n", i+1, r.Question, r.Show, r.Episode)
		}
		return nil
	}
	return cmd
}

// --------------------------------------------------------------- sponsor intel

func newSponsorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sponsor", Short: "Sponsor / advertiser intelligence"}
	cmd.AddCommand(newSponsorCompetitorsCmd(flags))
	return cmd
}

func newSponsorCompetitorsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "competitors <brand>",
		Short: "Brands sponsoring the same shows as a target brand",
		Long: `Heuristic: search transcripts for the target brand and a candidate brand list,
report shows where both appeared. Useful for competitive media planning.

  podscan-pp-cli sponsor competitors "Mercury" --since 90d --against "Brex,Ramp,QuickBooks,Bench,Gusto"`,
		Args: cobra.ExactArgs(1),
	}
	var since, against string
	var maxPages int
	cmd.Flags().StringVar(&since, "since", "90d", "Look back window")
	cmd.Flags().StringVar(&against, "against", "", "Comma-separated candidate brand list")
	cmd.Flags().IntVar(&maxPages, "max-pages", 4, "Cap on result pages per brand")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		target := args[0]
		if against == "" {
			return fmt.Errorf("--against is required (comma-separated brand list)")
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		targetEps, err := searchEpisodesAll(c, target, since, maxPages)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		targetShows := map[string]string{}
		for _, e := range targetEps {
			targetShows[e.Podcast.PodcastID] = e.Podcast.PodcastName
		}
		type compRow struct {
			Brand       string   `json:"brand"`
			SharedShows int      `json:"shared_shows"`
			Examples    []string `json:"examples,omitempty"`
		}
		out := []compRow{}
		for _, brand := range strings.Split(against, ",") {
			brand = strings.TrimSpace(brand)
			if brand == "" {
				continue
			}
			eps, err := searchEpisodesAll(c, brand, since, maxPages)
			if err != nil {
				continue
			}
			shared := 0
			seen := map[string]bool{}
			examples := []string{}
			for _, e := range eps {
				if _, ok := targetShows[e.Podcast.PodcastID]; ok && !seen[e.Podcast.PodcastID] {
					seen[e.Podcast.PodcastID] = true
					shared++
					if len(examples) < 3 {
						examples = append(examples, e.Podcast.PodcastName)
					}
				}
			}
			out = append(out, compRow{Brand: brand, SharedShows: shared, Examples: examples})
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].SharedShows > out[j].SharedShows })
		summary := map[string]any{
			"target": target, "since": since,
			"target_shows": len(targetShows), "competitors": out,
		}
		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			b, _ := json.MarshalIndent(summary, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Brands overlapping with %q sponsorship footprint (last %s)\n\n", target, since)
		fmt.Fprintf(cmd.OutOrStdout(), "Target appears on %d distinct shows.\n\n", len(targetShows))
		for _, r := range out {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s — %d shared shows  %s\n", r.Brand, r.SharedShows, strings.Join(r.Examples, ", "))
		}
		return nil
	}
	return cmd
}

// ---------------------------------------------------------------------- watch

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Standing alerts via Podscan Firehose webhooks",
		Long: `Register and manage standing alerts. Uses Podscan's /alerts API; configure the
webhook destination separately in the Podscan dashboard or via 'alerts create
--webhook-url'.`,
	}
	cmd.AddCommand(newWatchListCmd(flags))
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured standing alerts",
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, err := c.Get("/alerts", nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return nil
	}
	return cmd
}
