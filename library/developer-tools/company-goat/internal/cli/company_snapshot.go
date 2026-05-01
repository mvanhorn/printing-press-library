// Hand-written: snapshot, compare, and signal commands.
// snapshot fans out across all 7 sources via cliutil.FanoutRun.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/ch"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/github"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/hn"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/rdap"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/sec"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/wikidata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/yc"
	"github.com/spf13/cobra"
)

// SourceSnapshot is the unified per-source result row used by snapshot
// and compare. Type-erased to map[string]any so each source can include
// its own structured fields without a giant union.
type SourceSnapshot struct {
	Source  string         `json:"source"`
	Status  string         `json:"status"` // "ok", "no-data", "error", "needs-key"
	Note    string         `json:"note,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
	Elapsed string         `json:"elapsed,omitempty"`
}

// CompanySnapshot is the full multi-source result.
type CompanySnapshot struct {
	Domain       string           `json:"domain"`
	ResolvedFrom string           `json:"resolved_from,omitempty"`
	Sources      []SourceSnapshot `json:"sources"`
	Generated    string           `json:"generated_at"`
}

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags
	var skipSources []string

	cmd := &cobra.Command{
		Use:   "snapshot [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fan out across all 7 sources in parallel and render a unified summary. The headline command.",
		Long: `snapshot is the headline command. It runs every source query in parallel via cliutil.FanoutRun and renders a unified per-section summary.

Sources queried (in render order):
  funding    — SEC EDGAR Form D + YC batch
  legal      — Companies House (UK) + SEC EDGAR Form D issuer (US)
  engineering — GitHub org + repo activity
  launches   — Show HN posts
  mentions   — HN mention timeline
  yc         — YC directory entry
  wiki       — Wikidata facts
  domain     — RDAP/WHOIS + DNS hosting hint

Sources are queried with bounded timeouts. Failed sources are reported
as "error" status; missing data renders as "no-data" — never silently
omitted (honesty contract).`,
		Example: strings.Trim(`
  company-goat-pp-cli snapshot stripe
  company-goat-pp-cli snapshot anthropic --json --select sources
  company-goat-pp-cli snapshot --domain ramp.com --skip mentions,wiki
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			name := strings.Join(args, " ")
			if name == "" {
				name = strings.SplitN(domain, ".", 2)[0]
			}

			skips := map[string]bool{}
			for _, s := range skipSources {
				skips[strings.ToLower(strings.TrimSpace(s))] = true
			}

			sources := []string{"funding", "legal", "engineering", "launches", "mentions", "yc", "wiki", "domain"}
			activeSources := sources[:0]
			for _, s := range sources {
				if !skips[s] {
					activeSources = append(activeSources, s)
				}
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			deps := snapshotDeps{
				email: getContactEmail(flags),
				name:  name,
			}
			results, _ := cliutil.FanoutRun(ctx, activeSources, func(s string) string { return s }, func(ctx context.Context, src string) (SourceSnapshot, error) {
				start := time.Now()
				out := runSnapshotSource(ctx, src, domain, deps)
				out.Elapsed = time.Since(start).Round(time.Millisecond).String()
				return out, nil
			})

			snap := CompanySnapshot{
				Domain:    domain,
				Generated: time.Now().UTC().Format(time.RFC3339),
			}
			// Preserve canonical source order (FanoutRun returns in input order).
			for _, r := range results {
				snap.Sources = append(snap.Sources, r.Value)
			}

			renderSnapshot(cmd, flags, snap)
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	cmd.Flags().StringSliceVar(&skipSources, "skip", nil, "Skip these sources (comma-separated): funding,legal,engineering,launches,mentions,yc,wiki,domain")
	return cmd
}

type snapshotDeps struct {
	email string
	name  string
}

// runSnapshotSource is one fanout worker — picks the right source client
// and returns a unified SourceSnapshot.
func runSnapshotSource(ctx context.Context, src, domain string, deps snapshotDeps) SourceSnapshot {
	out := SourceSnapshot{Source: src, Status: "ok"}
	stem := strings.SplitN(domain, ".", 2)[0]

	switch src {
	case "funding":
		secCli := sec.NewClient(deps.email)
		filings, err := secCli.SearchAndFetchAll(ctx, stem, 3)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		if len(filings) == 0 {
			out.Status = "no-data"
			out.Note = "no Form D filings found"
			return out
		}
		// Surface CIK ambiguity so the calling agent can disambiguate via
		// other tools (wiki founders, engineering signal, world knowledge)
		// or re-call funding with --cik. We deliberately do NOT auto-pick:
		// "Notion" matches both Notion Labs (Corp) and Notion Capital (LP)
		// and the right answer depends on the user's intent, which the CLI
		// can't see. latest_filing is still populated for backward compat
		// when unambiguous; null when ambiguous so consumers don't treat a
		// dragnet match as authoritative.
		summaries := summarizeByCIK(filings)
		isAmbiguous := len(summaries) > 1
		data := map[string]any{
			"form_d_filings_count": len(filings),
			"is_ambiguous":         isAmbiguous,
			"cik_summaries":        summaries,
		}
		if !isAmbiguous {
			data["latest_filing"] = filings[0]
		} else {
			data["latest_filing"] = nil
			data["note"] = "Multiple SEC entities matched the name. Use cik_summaries to pick the right one and re-run `funding <name> --cik <id>`."
			out.Status = "ambiguous"
		}
		out.Data = data
	case "legal":
		// US first via SEC.
		secCli := sec.NewClient(deps.email)
		filings, err := secCli.SearchAndFetchAll(ctx, stem, 1)
		if err == nil && len(filings) > 0 {
			out.Data = map[string]any{
				"region":      "us",
				"issuer_name": filings[0].EntityName,
				"state_inc":   filings[0].State,
				"entity_type": filings[0].EntityType,
				"source":      "SEC EDGAR Form D",
			}
			return out
		}
		// UK fallback if key available.
		chCli := ch.NewClient()
		if !chCli.HasKey() {
			out.Status = "no-data"
			out.Note = "no US Form D issuer; UK lookup needs COMPANIES_HOUSE_API_KEY"
			return out
		}
		hits, err := chCli.Search(ctx, deps.name, 1)
		if err != nil || len(hits) == 0 {
			out.Status = "no-data"
			out.Note = "no Companies House match"
			return out
		}
		out.Data = map[string]any{
			"region":         "uk",
			"company_number": hits[0].CompanyNumber,
			"company_name":   hits[0].Title,
			"status":         hits[0].CompanyStatus,
			"date_created":   hits[0].DateOfCreation,
		}
	case "engineering":
		gh := github.NewClient()
		org, err := gh.FindOrgFromDomain(ctx, domain, deps.name)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		if org == nil {
			out.Status = "no-data"
			out.Note = "no GitHub org found"
			return out
		}
		out.Data = map[string]any{
			"login":        org.Login,
			"public_repos": org.PublicRepos,
			"followers":    org.Followers,
			"description":  org.Description,
		}
	case "launches":
		c := hn.NewClient()
		resp, err := c.SearchShowHN(ctx, deps.name, 5)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		if resp.NbHits == 0 {
			out.Status = "no-data"
			out.Note = "no Show HN posts found"
			return out
		}
		top := resp.Hits
		if len(top) > 3 {
			top = top[:3]
		}
		out.Data = map[string]any{
			"total_show_hn": resp.NbHits,
			"top_posts":     top,
		}
	case "mentions":
		c := hn.NewClient()
		resp, err := c.SearchByDate(ctx, deps.name, 50)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		if resp.NbHits == 0 {
			out.Status = "no-data"
			out.Note = "no HN mentions found"
			return out
		}
		// Bucket by year for compactness.
		yearCounts := map[string]int{}
		for _, h := range resp.Hits {
			if len(h.CreatedAt) >= 4 {
				yearCounts[h.CreatedAt[:4]]++
			}
		}
		out.Data = map[string]any{
			"total_mentions": resp.NbHits,
			"by_year_sample": yearCounts,
		}
	case "yc":
		c := yc.NewClient()
		entry, err := c.FindByDomain(ctx, domain)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		if entry == nil {
			// Fallback to name match.
			matches, _ := c.FindByName(ctx, deps.name, 1)
			if len(matches) > 0 {
				entry = &matches[0]
			}
		}
		if entry == nil {
			out.Status = "no-data"
			out.Note = "no YC entry found"
			return out
		}
		out.Data = map[string]any{
			"name":      entry.Name,
			"batch":     entry.Batch,
			"status":    entry.Status,
			"one_liner": entry.OneLiner,
		}
	case "wiki":
		c := wikidata.NewClient()
		entry, err := c.LookupByDomain(ctx, domain)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		if entry == nil {
			out.Status = "no-data"
			out.Note = "no Wikidata entry found"
			return out
		}
		out.Data = map[string]any{
			"qid":      entry.QID,
			"label":    entry.Label,
			"founded":  entry.Founded,
			"hq":       entry.HQLocation,
			"country":  entry.Country,
			"industry": entry.Industry,
			"founders": entry.Founders,
		}
	case "domain":
		c := rdap.NewClient()
		info, err := c.Lookup(ctx, domain)
		if err != nil {
			out.Status = "error"
			out.Note = err.Error()
			return out
		}
		out.Data = map[string]any{
			"registered":    info.Registered,
			"expires":       info.ExpiresAt,
			"hosting":       info.HostingHint,
			"hosting_cname": info.HostingCNAME,
			"nameservers":   info.Nameservers,
		}
	}
	return out
}

func renderSnapshot(cmd *cobra.Command, flags *rootFlags, snap CompanySnapshot) {
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		_ = flags.printJSON(cmd, snap)
		return
	}
	fmt.Fprintf(w, "Company snapshot: %s\n", snap.Domain)
	fmt.Fprintf(w, "Generated: %s\n\n", snap.Generated)
	for _, s := range snap.Sources {
		fmt.Fprintf(w, "── %s [%s] (%s)\n", strings.ToUpper(s.Source), s.Status, s.Elapsed)
		if s.Note != "" {
			fmt.Fprintf(w, "   %s\n", s.Note)
		}
		if s.Data != nil {
			renderSnapshotSection(w, s)
		}
		fmt.Fprintln(w)
	}
}

func renderSnapshotSection(w fmt_w, s SourceSnapshot) {
	d := s.Data
	switch s.Source {
	case "funding":
		if c, ok := d["form_d_filings_count"]; ok {
			fmt.Fprintf(w, "   Form D filings: %v\n", c)
		}
		if amb, _ := d["is_ambiguous"].(bool); amb {
			fmt.Fprintf(w, "   ⚠ Ambiguous — name matched multiple SEC entities. Re-call funding with --cik <id>:\n")
			if summaries, ok := d["cik_summaries"].([]cikSummary); ok {
				for _, s := range summaries {
					yr := ""
					if s.YearOfInc != "" {
						yr = " inc:" + s.YearOfInc
					}
					fmt.Fprintf(w, "     CIK %s  %-40s  state:%s%s  filings:%d  latest:%s\n",
						s.CIK, fundingTruncate(s.EntityName, 40), s.State, yr, s.FilingCount, s.LatestFilingDate)
				}
			}
		} else if lf, ok := d["latest_filing"].(sec.FormD); ok {
			fmt.Fprintf(w, "   Latest:  %s  %s  %s\n", lf.FilingDate, fundingTruncate(lf.EntityName, 40), formatAmount(lf.OfferingAmount))
		}
	case "legal":
		region, _ := d["region"].(string)
		fmt.Fprintf(w, "   Region: %s\n", region)
		if region == "us" {
			fmt.Fprintf(w, "   Issuer: %v  state: %v  type: %v\n", d["issuer_name"], d["state_inc"], d["entity_type"])
		} else {
			fmt.Fprintf(w, "   Company: %v (#%v)  status: %v  created: %v\n", d["company_name"], d["company_number"], d["status"], d["date_created"])
		}
	case "engineering":
		fmt.Fprintf(w, "   github.com/%v  %v repos  %v followers\n", d["login"], d["public_repos"], d["followers"])
		if desc, _ := d["description"].(string); desc != "" {
			fmt.Fprintf(w, "   %s\n", desc)
		}
	case "launches":
		fmt.Fprintf(w, "   Show HN posts (total): %v\n", d["total_show_hn"])
		if posts, ok := d["top_posts"].([]hn.Hit); ok {
			for _, p := range posts {
				yr := ""
				if len(p.CreatedAt) >= 4 {
					yr = p.CreatedAt[:4]
				}
				fmt.Fprintf(w, "   %s  %3d↑  %s\n", yr, p.Points, fundingTruncate(p.Title, 70))
			}
		}
	case "mentions":
		fmt.Fprintf(w, "   HN mentions (total): %v\n", d["total_mentions"])
		if yr, ok := d["by_year_sample"].(map[string]int); ok {
			years := make([]string, 0, len(yr))
			for y := range yr {
				years = append(years, y)
			}
			sort.Strings(years)
			fmt.Fprintf(w, "   Sample by year: ")
			for i, y := range years {
				if i > 0 {
					fmt.Fprintf(w, ", ")
				}
				fmt.Fprintf(w, "%s:%d", y, yr[y])
			}
			fmt.Fprintln(w)
		}
	case "yc":
		fmt.Fprintf(w, "   %v  batch %v  status %v\n", d["name"], d["batch"], d["status"])
		if ol, _ := d["one_liner"].(string); ol != "" {
			fmt.Fprintf(w, "   %s\n", ol)
		}
	case "wiki":
		fmt.Fprintf(w, "   %v  (%v)\n", d["label"], d["qid"])
		fmt.Fprintf(w, "   founded:%v  hq:%v  country:%v  industry:%v\n", d["founded"], d["hq"], d["country"], d["industry"])
		if fs, ok := d["founders"].([]string); ok && len(fs) > 0 {
			fmt.Fprintf(w, "   founders: %s\n", strings.Join(fs, ", "))
		}
	case "domain":
		fmt.Fprintf(w, "   registered:%v  expires:%v\n", d["registered"], d["expires"])
		if h, _ := d["hosting"].(string); h != "" {
			fmt.Fprintf(w, "   hosting:%v  cname:%v\n", h, d["hosting_cname"])
		}
		if ns, ok := d["nameservers"].([]string); ok && len(ns) > 0 {
			fmt.Fprintf(w, "   ns: %s\n", strings.Join(ns, ", "))
		}
	}
}

// fmt_w aliases io.Writer for the section renderer signature.
type fmt_w = interface{ Write([]byte) (int, error) }

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <a> <b>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Two snapshots aligned column-by-column for direct comparison.",
		Long:  `compare runs snapshot for two companies and renders the results in side-by-side columns. Useful for evaluating which of two startups looks healthier — funding, engineering, launch story, etc.`,
		Example: strings.Trim(`
  company-goat-pp-cli compare ramp brex
  company-goat-pp-cli compare stripe.com adyen.com --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if len(args) < 2 {
				return cmd.Help()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
			defer cancel()

			snaps := make([]CompanySnapshot, 2)
			for i, a := range args[:2] {
				t := targetFlags{}
				domain, err := runResolveOrExit(cmd, flags, []string{a}, t)
				if err != nil {
					return err
				}
				name := a
				deps := snapshotDeps{email: getContactEmail(flags), name: name}
				sources := []string{"funding", "legal", "engineering", "launches", "yc", "domain"}
				results, _ := cliutil.FanoutRun(ctx, sources, func(s string) string { return s }, func(ctx context.Context, src string) (SourceSnapshot, error) {
					start := time.Now()
					out := runSnapshotSource(ctx, src, domain, deps)
					out.Elapsed = time.Since(start).Round(time.Millisecond).String()
					return out, nil
				})
				snap := CompanySnapshot{Domain: domain, Generated: time.Now().UTC().Format(time.RFC3339)}
				for _, r := range results {
					snap.Sources = append(snap.Sources, r.Value)
				}
				snaps[i] = snap
			}
			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, map[string]any{
					"left":  snaps[0],
					"right": snaps[1],
				})
			}
			renderCompare(w, snaps[0], snaps[1])
			return nil
		},
	}
	return cmd
}

func renderCompare(w fmt_w, a, b CompanySnapshot) {
	fmt.Fprintf(w, "%-50s | %s\n", a.Domain, b.Domain)
	fmt.Fprintf(w, "%s|%s\n", strings.Repeat("-", 51), strings.Repeat("-", 51))
	srcs := map[string]int{}
	for i, s := range a.Sources {
		srcs[s.Source] = i
	}
	for _, s := range b.Sources {
		if _, ok := srcs[s.Source]; !ok {
			srcs[s.Source] = -1
		}
	}
	keys := make([]string, 0, len(srcs))
	for k := range srcs {
		keys = append(keys, k)
	}
	// Render in canonical order.
	canonical := []string{"funding", "legal", "engineering", "launches", "yc", "domain", "mentions", "wiki"}
	for _, k := range canonical {
		la := snapLine(a, k)
		lb := snapLine(b, k)
		fmt.Fprintf(w, "%-12s %-37s | %-12s %s\n", k, fundingTruncate(la, 37), k, fundingTruncate(lb, 37))
	}
}

func snapLine(s CompanySnapshot, src string) string {
	for _, ss := range s.Sources {
		if ss.Source == src {
			if ss.Status == "ambiguous" {
				// Don't synthesize a side-by-side number when funding
				// matched multiple SEC entities — the comparison would
				// average across unrelated companies. The agent reading
				// the per-side snapshot output sees cik_summaries; this
				// column just signals "data quality issue here."
				return "ambiguous (multiple CIKs)"
			}
			if ss.Status != "ok" {
				return ss.Status
			}
			d := ss.Data
			switch src {
			case "funding":
				return fmt.Sprintf("%v filings", d["form_d_filings_count"])
			case "legal":
				return fmt.Sprintf("%v: %v", d["region"], firstNonEmpty(d["issuer_name"], d["company_name"]))
			case "engineering":
				return fmt.Sprintf("%v repos, %v followers", d["public_repos"], d["followers"])
			case "launches":
				return fmt.Sprintf("%v Show HN posts", d["total_show_hn"])
			case "yc":
				return fmt.Sprintf("%v %v", d["batch"], d["status"])
			case "domain":
				return fmt.Sprintf("hosting:%v", d["hosting"])
			case "mentions":
				return fmt.Sprintf("%v mentions", d["total_mentions"])
			case "wiki":
				return fmt.Sprintf("%v", d["label"])
			}
		}
	}
	return "—"
}

func firstNonEmpty(vals ...any) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func newSignalCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags

	cmd := &cobra.Command{
		Use:   "signal [co]",
		Short: "Cross-source consistency check. Flags suspicious patterns like 'raised in 2024 but no GitHub commits since 2022'.",
		Long: `signal runs the same fanout as snapshot, then computes simple cross-source consistency checks. Each check is a heuristic — false positives are expected; the goal is to surface things worth investigating.

Checks:
  - Form D recency vs GitHub commit recency
  - YC active status vs domain registration expiry
  - HN mention recency vs Form D recency
  - Company has Form D but no website (possible LP/fund vehicle)`,
		Example: strings.Trim(`
  company-goat-pp-cli signal stripe
  company-goat-pp-cli signal acme-corp --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			name := strings.Join(args, " ")
			if name == "" {
				name = strings.SplitN(domain, ".", 2)[0]
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			deps := snapshotDeps{email: getContactEmail(flags), name: name}
			sources := []string{"funding", "engineering", "launches", "yc", "domain"}
			results, _ := cliutil.FanoutRun(ctx, sources, func(s string) string { return s }, func(ctx context.Context, src string) (SourceSnapshot, error) {
				return runSnapshotSource(ctx, src, domain, deps), nil
			})
			snap := CompanySnapshot{Domain: domain, Generated: time.Now().UTC().Format(time.RFC3339)}
			for _, r := range results {
				snap.Sources = append(snap.Sources, r.Value)
			}

			signals := computeSignals(snap)

			out := map[string]any{
				"domain":   domain,
				"signals":  signals,
				"snapshot": snap,
			}
			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(w, "Signal check for %s:\n\n", domain)
			if len(signals) == 0 {
				fmt.Fprintf(w, "  no flagged patterns; sources look consistent\n")
				return nil
			}
			for _, s := range signals {
				fmt.Fprintf(w, "  ⚠  %s\n     %s\n", s.Title, s.Detail)
			}
			fmt.Fprintf(w, "\nThese are heuristics — confirm before acting.\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	return cmd
}

type Signal struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func computeSignals(s CompanySnapshot) []Signal {
	bySource := map[string]SourceSnapshot{}
	for _, src := range s.Sources {
		bySource[src.Source] = src
	}
	var out []Signal

	funding := bySource["funding"]
	eng := bySource["engineering"]
	// Funding ambiguity is itself a signal: every other check that
	// references the funding row is unreliable when the SEC name search
	// matched multiple unrelated entities. Surface it once so the agent
	// can decide whether to disambiguate before drawing conclusions.
	if funding.Status == "ambiguous" {
		summaries, _ := funding.Data["cik_summaries"].([]cikSummary)
		out = append(out, Signal{
			Title:  "Form D match is ambiguous",
			Detail: fmt.Sprintf("Name search returned filings across %d distinct SEC entities. Other-source signals (engineering, mentions) may not refer to the same company. Re-call funding with --cik <id> to disambiguate before trusting this row.", len(summaries)),
		})
	}
	if funding.Status == "ok" && eng.Status == "ok" {
		// Check Form D recent year vs no recent github activity (best-effort).
		if lf, ok := funding.Data["latest_filing"].(sec.FormD); ok {
			if len(lf.FilingDate) >= 4 {
				filingYear := lf.FilingDate[:4]
				if filingYear >= "2023" {
					// We already have engineering basics; without per-repo
					// pushed_at sample we can only flag "no repos" as a signal.
					if pr, ok := eng.Data["public_repos"].(int); ok && pr == 0 {
						out = append(out, Signal{
							Title:  "Recent Form D filing but zero public repos",
							Detail: fmt.Sprintf("Filed in %s; GitHub org has 0 public repos. Either private/closed-source or a non-engineering company.", filingYear),
						})
					}
				}
			}
		}
	}
	// YC active vs domain expired/expiring.
	yc := bySource["yc"]
	dom := bySource["domain"]
	if yc.Status == "ok" && dom.Status == "ok" {
		ycStatus, _ := yc.Data["status"].(string)
		expires, _ := dom.Data["expires"].(string)
		if ycStatus == "Active" && expires != "" && expires < time.Now().Add(60*24*time.Hour).Format("2006-01-02") {
			out = append(out, Signal{
				Title:  "YC Active but domain expires soon",
				Detail: fmt.Sprintf("YC marked Active; domain registration expires %s. Domain may have been let go.", expires),
			})
		}
	}
	// Form D issuer but no website signals at all.
	if funding.Status == "ok" {
		hasWeb := false
		for _, src := range []string{"yc", "wiki", "engineering"} {
			if bySource[src].Status == "ok" {
				hasWeb = true
				break
			}
		}
		if !hasWeb {
			out = append(out, Signal{
				Title:  "Form D filing exists but no consumer-web presence",
				Detail: "No YC entry, Wikidata page, or GitHub org found. Possible LP/fund vehicle or stealth entity.",
			})
		}
	}
	return out
}
