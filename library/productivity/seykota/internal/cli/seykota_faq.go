// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/store"
)

// topics is the curated FAQ topic vocabulary: a slug, a one-line gloss, and
// an FTS query that surfaces the FAQ months discussing it.
type topicDef struct {
	Slug  string `json:"slug"`
	About string `json:"about"`
	Query string `json:"-"`
}

var topics = []topicDef{
	{"trend-following", "Riding trends; cutting losses, letting profits run", `"trend" OR "trends" OR "trendfollowing"`},
	{"heat", "Portfolio heat / total risk exposure across positions", `"heat"`},
	{"risk-per-trade", "How much equity to risk on a single trade", `"risk" AND ("trade" OR "position" OR "stop")`},
	{"position-sizing", "Sizing positions; fixed-fraction betting", `"sizing" OR ("position" AND "size") OR "fixed-fraction"`},
	{"pyramiding", "Adding to winning positions; pyramiding vs martingale", `"pyramiding" OR "pyramid" OR "martingale"`},
	{"exits", "Exits, stops, trailing stops, getting out", `"exit" OR "exits" OR "stop" OR "trailing"`},
	{"whipsaw", "Whipsaws, choppy markets, false breakouts", `"whipsaw" OR "whipsaws" OR "chop"`},
	{"system-design", "Designing and testing mechanical trading systems", `"system" AND ("design" OR "test" OR "rules" OR "mechanical")`},
	{"diversification", "Diversifying across markets to lower portfolio heat", `"diversification" OR "diversify" OR "diversified"`},
	{"psychology", "Trading psychology; the Trading Tribe Process (TTP); feelings", `"psychology" OR "feelings" OR "ttp" OR "tribe" OR "fred"`},
	{"money-management", "Money management; equity curves; risk of ruin", `"money management" OR "ruin" OR "drawdown" OR "equity"`},
	{"discipline", "Discipline; following the system; not overriding signals", `"discipline" OR "disciplined" OR ("follow" AND "system")`},
}

func lookupTopic(slug string) (topicDef, bool) {
	n := strings.ToLower(strings.TrimSpace(slug))
	n = strings.ReplaceAll(n, "_", "-")
	n = strings.ReplaceAll(n, " ", "-")
	for _, t := range topics {
		if t.Slug == n {
			return t, true
		}
	}
	// allow a few aliases
	switch n {
	case "tribe", "ttp", "feelings", "fred":
		return findTopic("psychology"), true
	case "stops", "exit":
		return findTopic("exits"), true
	case "sizing":
		return findTopic("position-sizing"), true
	case "trend", "trends":
		return findTopic("trend-following"), true
	}
	return topicDef{}, false
}

func findTopic(slug string) topicDef {
	for _, t := range topics {
		if t.Slug == slug {
			return t
		}
	}
	return topicDef{}
}

func newFaqCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "faq",
		Short: "Browse and read Ed Seykota's FAQ — the dated reader-mailbag archive (2003–2023)",
		Long: `Ed Seykota's FAQ is two decades of reader mail and his replies, archived by
month. With no subcommand, 'faq' lists the months in the local archive
(filter with --year or --topic). 'faq show <year> <month>' prints a month;
'faq contributors' indexes who wrote in; 'faq topics' lists the topic
vocabulary used by --topic and search.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// `faq` with no subcommand acts as `faq list`.
			return runFaqList(cmd, flags, "", "", "")
		},
	}
	// list-mode flags on the parent so `faq --year 2007` works
	var dbPath, year, topic string
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&year, "year", "", "Only months in this 4-digit year")
	cmd.Flags().StringVar(&topic, "topic", "", "Only months discussing this topic (see 'faq topics')")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runFaqList(cmd, flags, dbPath, year, topic)
	}
	cmd.AddCommand(newFaqListCmd(flags))
	cmd.AddCommand(newFaqShowCmd(flags))
	cmd.AddCommand(newFaqContributorsCmd(flags))
	cmd.AddCommand(newFaqTopicsCmd(flags))
	return cmd
}

func newFaqListCmd(flags *rootFlags) *cobra.Command {
	var dbPath, year, topic string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the FAQ months in the local archive (filter with --year or --topic)",
		Example:     strings.Trim("\n  seykota-pp-cli faq list\n  seykota-pp-cli faq list --year 2007\n  seykota-pp-cli faq list --topic pyramiding\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFaqList(cmd, flags, dbPath, year, topic)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&year, "year", "", "Only months in this 4-digit year")
	cmd.Flags().StringVar(&topic, "topic", "", "Only months discussing this topic (see 'faq topics')")
	return cmd
}

type faqMonthView struct {
	Year         string   `json:"year"`
	Month        string   `json:"month"`
	Range        string   `json:"range,omitempty"`
	Contributors []string `json:"contributors,omitempty"`
	URL          string   `json:"url"`
}

func runFaqList(cmd *cobra.Command, flags *rootFlags, dbPath, year, topic string) error {
	s, err := openCorpus(cmd.Context(), dbPath)
	if err != nil {
		return err
	}
	defer s.Close()

	if strings.TrimSpace(topic) != "" {
		td, ok := lookupTopic(topic)
		if !ok {
			return usageErr(fmt.Errorf("unknown topic %q — run 'seykota-pp-cli faq topics' for the list", topic))
		}
		hits, err := s.SearchCorpus(td.Query, store.SearchOpts{Source: "faq", Year: year, Limit: 200})
		if err != nil {
			return err
		}
		// distinct months, FTS rank order preserved
		seen := map[string]bool{}
		var views []faqMonthView
		for _, h := range hits {
			key := h.Year + "/" + h.Month
			if seen[key] {
				continue
			}
			seen[key] = true
			views = append(views, faqMonthView{Year: h.Year, Month: h.Month, Range: h.Range, Contributors: h.Contributors, URL: h.URL})
		}
		if wantsJSON(cmd, flags) {
			return emitJSON(cmd, flags, map[string]any{"topic": td.Slug, "count": len(views), "months": views})
		}
		if len(views) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No FAQ months found for topic %q.\n", td.Slug)
			return nil
		}
		rows := make([][]string, 0, len(views))
		for _, v := range views {
			rows = append(rows, []string{v.Year, v.Month, v.URL})
		}
		_ = printRows(cmd, []string{"YEAR", "MONTH", "URL"}, rows)
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d FAQ month(s) discussing %q. Open one: seykota-pp-cli faq show <year> <month>\n", len(views), td.Slug)
		return nil
	}

	docs, err := s.ListDocs("faq")
	if err != nil {
		return err
	}
	year = strings.TrimSpace(year)
	var views []faqMonthView
	for _, d := range docs {
		if year != "" && d.Year != year {
			continue
		}
		views = append(views, faqMonthView{Year: d.Year, Month: d.Month, Range: d.Range, Contributors: d.Contributors, URL: d.URL})
	}
	if wantsJSON(cmd, flags) {
		return emitJSON(cmd, flags, map[string]any{"count": len(views), "months": views})
	}
	if len(views) == 0 {
		if year != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "No FAQ months in %s in the local archive.\n", year)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "No FAQ months in the local archive. Run 'seykota-pp-cli index build'.")
		}
		return nil
	}
	rows := make([][]string, 0, len(views))
	for _, v := range views {
		c := ""
		if len(v.Contributors) > 0 {
			c = fmt.Sprintf("%d", len(v.Contributors))
		}
		rows = append(rows, []string{v.Year, v.Month, v.Range, c, v.URL})
	}
	_ = printRows(cmd, []string{"YEAR", "MONTH", "DAYS", "#CONTRIB", "URL"}, rows)
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d FAQ month(s). Read one: seykota-pp-cli faq show <year> <month>\n", len(views))
	return nil
}

func newFaqShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxChars int
	cmd := &cobra.Command{
		Use:   "show [year] [month]",
		Short: "Print the full text of one FAQ month",
		Long: `Print the cleaned text of a single FAQ month-page from the local archive.
Month can be the folder name (e.g. JAN or Jan), a 3-letter prefix, or the
month number. Use --max to truncate a very long month.`,
		Example: strings.Trim("\n  seykota-pp-cli faq show 2007 Jul\n  seykota-pp-cli faq show 2019 11\n  seykota-pp-cli faq show 2023 JAN --max 4000\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) != 2 {
				return usageErr(fmt.Errorf("faq show needs a year and a month, e.g. seykota-pp-cli faq show 2019 Nov (got %d argument(s))", len(args)))
			}
			if dryRunOK(flags) {
				return nil
			}
			year, month := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
			s, err := openCorpus(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			d, err := s.FAQByYearMonth(year, month)
			if err != nil {
				if strings.Contains(err.Error(), "no rows") {
					return notFoundErr(fmt.Errorf("no FAQ month %s %s in the local archive — try 'seykota-pp-cli faq list --year %s'", month, year, year))
				}
				return err
			}
			body := d.Body
			if maxChars > 0 && len(body) > maxChars {
				body = body[:maxChars] + "\n…(truncated; use --max 0 for the full text)…"
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"year": d.Year, "month": d.Month, "range": d.Range, "url": d.URL,
					"contributors": d.Contributors, "title": d.Title, "body": body,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", d.Label(), d.URL)
			if len(d.Contributors) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Contributors: %s\n", strings.Join(d.Contributors, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", body)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().IntVar(&maxChars, "max", 0, "Truncate the body to this many characters (0 = full text)")
	return cmd
}

func newFaqContributorsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "contributors [name]",
		Short: "Who wrote into the FAQ — with per-contributor month counts; pass a name for the months they appear in",
		Long: `Indexes the contributor lines parsed from FAQ month-pages. With no argument,
lists every contributor and how many months they appear in (most frequent
first). With a name (case-insensitive substring), lists the months that
contributor shows up in.

Contributor parsing is best-effort over hand-written 1990s HTML — some
months don't carry a clean contributors block, so coverage is partial.`,
		Example: strings.Trim("\n  seykota-pp-cli faq contributors\n  seykota-pp-cli faq contributors --limit 25\n  seykota-pp-cli faq contributors \"Dave\"\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				return nil
			}
			s, err := openCorpus(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			cs, err := s.Contributors(name)
			if err != nil {
				return err
			}
			if limit > 0 && name == "" && len(cs) > limit {
				cs = cs[:limit]
			}
			if name != "" && len(cs) == 0 {
				return notFoundErr(fmt.Errorf("no FAQ contributor matching %q in the local archive (contributor parsing is best-effort; richer after 'seykota-pp-cli index build --full-archive') — try 'faq contributors' with no filter to see who is indexed", name))
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{"filter": name, "count": len(cs), "contributors": cs})
			}
			if len(cs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No FAQ contributors parsed from the local archive (contributor parsing is best-effort; richer after 'seykota-pp-cli index build --full-archive').")
				return nil
			}
			if name != "" {
				for _, c := range cs {
					fmt.Fprintf(cmd.OutOrStdout(), "%s — %d month(s): %s\n", c.Name, c.Months, strings.Join(c.When, "; "))
				}
				return nil
			}
			rows := make([][]string, 0, len(cs))
			for _, c := range cs {
				rows = append(rows, []string{c.Name, fmt.Sprintf("%d", c.Months)})
			}
			_ = printRows(cmd, []string{"CONTRIBUTOR", "MONTHS"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().IntVar(&limit, "limit", 40, "Max contributors to list (when no name filter)")
	return cmd
}

func newFaqTopicsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "topics",
		Short:       "List the curated FAQ topic vocabulary (use these with 'faq --topic' and as search terms)",
		Example:     "  seykota-pp-cli faq topics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ts := make([]topicDef, len(topics))
			copy(ts, topics)
			sort.Slice(ts, func(i, j int) bool { return ts[i].Slug < ts[j].Slug })
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{"count": len(ts), "topics": ts})
			}
			rows := make([][]string, 0, len(ts))
			for _, t := range ts {
				rows = append(rows, []string{t.Slug, t.About})
			}
			_ = printRows(cmd, []string{"TOPIC", "ABOUT"}, rows)
			fmt.Fprintln(cmd.OutOrStdout(), "\nUse a topic with:  seykota-pp-cli faq --topic <topic>   or   seykota-pp-cli search \"<topic words>\"")
			return nil
		},
	}
	return cmd
}
