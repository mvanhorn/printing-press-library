package cli

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type fairnessOpts struct {
	by                      string
	writeOffDays, ghostDays int
	minEpisodes             int
	currency                string
	since                   time.Time
	hasSince                bool
	friendID                int
	groupID                 int
	groupScoped             bool
}

type fairnessPerson struct {
	UserID                int                `json:"user_id"`
	Name                  string             `json:"name"`
	HasHistory            bool               `json:"has_history"`
	Paid                  float64            `json:"paid"`
	Owed                  float64            `json:"owed"`
	Net                   float64            `json:"net"`
	CarryRatio            *float64           `json:"carry_ratio"`
	ExpenseCount          int                `json:"expense_count"`
	PayerCount            int                `json:"payer_count"`
	Role                  string             `json:"role"`
	OutstandingByCurrency map[string]float64 `json:"outstanding_by_currency"`
	OutstandingTotal      float64            `json:"outstanding_total"`
	DebtAgeDays           *int               `json:"debt_age_days"`
	LastSettledDays       *int               `json:"last_settled_days"`
	AvgLatencyDays        *float64           `json:"avg_latency_days"`
	ProjectedDaysOut      *int               `json:"projected_days_out"`
	ProjectedSettleDate   *string            `json:"projected_settle_date"`
	RiskScore             *float64           `json:"risk_score"`
	RiskTier              string             `json:"risk_tier"`
	Action                string             `json:"action"`
}

type fairnessResult struct {
	By            string           `json:"by"`
	Scope         string           `json:"scope"`
	People        []fairnessPerson `json:"people"`
	AtRiskTotal   float64          `json:"at_risk_total"`
	WriteOffTotal float64          `json:"write_off_total"`
	NewMembers    int              `json:"new_members"`
	GroupCaveat   bool             `json:"group_caveat,omitempty"`
}

type subjectEvent struct {
	date    time.Time
	payment bool
}

type episodeState struct {
	debtAgeDays      *int
	lastSettledDays  *int
	avgLatencyDays   *float64
	lastActivityDays int
}

type fairnessSubject struct {
	id   int
	name string
}

// pp:data-source local
func newFairnessCmd(flags *rootFlags) *cobra.Command {
	var by string
	var friendRef string
	var groupRef string
	var currency string
	var writeOffDays int
	var ghostDays int
	var minEpisodes int
	var since string

	cmd := &cobra.Command{
		Use:         "fairness",
		Short:       "Score who carries the group, who's a collection risk, and who to chase or write off",
		Example:     "  splitwise-pp-cli fairness --by risk --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute fairness")
				return nil
			}
			if by != "risk" && by != "contribution" && by != "collectability" {
				return usageErr(fmt.Errorf("invalid --by value %q: must be risk, contribution, or collectability", by))
			}
			if friendRef != "" && groupRef != "" {
				return usageErr(errors.New("--friend and --group are mutually exclusive"))
			}
			if writeOffDays <= 0 {
				return usageErr(fmt.Errorf("--write-off-days must be >= 1"))
			}
			if ghostDays <= 0 {
				return usageErr(fmt.Errorf("--ghost-days must be >= 1"))
			}
			if minEpisodes < 1 {
				return usageErr(fmt.Errorf("--min-episodes must be >= 1"))
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-friends")
			hintIfUnsynced(cmd, db, "get-groups")
			hintIfUnsynced(cmd, db, "get-expenses")
			hintIfStale(cmd, db, "get-friends", flags.maxAge)
			hintIfStale(cmd, db, "get-groups", flags.maxAge)
			hintIfStale(cmd, db, "get-expenses", flags.maxAge)

			friends, err := loadFriends(db)
			if err != nil {
				return err
			}
			groups, err := loadGroups(db)
			if err != nil {
				return err
			}
			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}
			youID := loadCurrentUserID(db)

			opts := fairnessOpts{by: by, writeOffDays: writeOffDays, ghostDays: ghostDays, minEpisodes: minEpisodes, currency: strings.TrimSpace(currency)}
			if strings.TrimSpace(since) != "" {
				t, err := time.Parse("2006-01-02", strings.TrimSpace(since))
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				opts.since = t
				opts.hasSince = true
			}

			scope := "all-friends"
			if strings.TrimSpace(friendRef) != "" {
				id, name, ok := resolveFairnessFriend(friendRef, friends)
				if !ok {
					return usageErr(fmt.Errorf("no friend matches %q; run sync first", friendRef))
				}
				opts.friendID = id
				scope = "friend:" + name
			}
			if strings.TrimSpace(groupRef) != "" {
				id, name, ok := resolveFairnessGroup(groupRef, groups)
				if !ok {
					return usageErr(fmt.Errorf("no group matches %q; run sync first", groupRef))
				}
				opts.groupID = id
				opts.groupScoped = true
				scope = "group:" + name
			}

			result := computeFairness(youID, friends, groups, expenses, time.Now().UTC(), opts)
			if result.Scope == "" {
				result.Scope = scope
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Fairness — %s (by %s)\n", result.Scope, result.By)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			switch result.By {
			case "risk":
				_, _ = fmt.Fprintln(tw, "WHO\tOUTSTANDING\tAGE\tLAST SETTLED\tTIER\tACTION")
				for _, p := range result.People {
					age := ageCell(p.DebtAgeDays)
					last := ageCell(p.LastSettledDays)
					_, _ = fmt.Fprintf(tw, "%s %s\t%.2f\t%s\t%s\t%s\t%s\n", tierGlyph(p.RiskTier), p.Name, p.OutstandingTotal, age, last, p.RiskTier, p.Action)
				}
			case "contribution":
				_, _ = fmt.Fprintln(tw, "WHO\tPAID\tOWED\tNET\tRATIO\tROLE")
				for _, p := range result.People {
					ratio := "-"
					if p.CarryRatio != nil {
						ratio = fmt.Sprintf("%.2f", *p.CarryRatio)
					}
					_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\t%s\t%s\n", p.Name, p.Paid, p.Owed, p.Net, ratio, p.Role)
				}
			case "collectability":
				_, _ = fmt.Fprintln(tw, "WHO\tOUTSTANDING\tAGE\tLAST SETTLED\tAVG LATENCY(d)\tPROJECTED")
				for _, p := range result.People {
					age := ageCell(p.DebtAgeDays)
					last := ageCell(p.LastSettledDays)
					avg := "-"
					if p.AvgLatencyDays != nil {
						avg = fmt.Sprintf("%.2f", *p.AvgLatencyDays)
					}
					projected := "-"
					if p.ProjectedDaysOut != nil {
						if *p.ProjectedDaysOut >= 0 {
							projected = humanizeDays(*p.ProjectedDaysOut) + " out"
						} else {
							projected = humanizeDays(-*p.ProjectedDaysOut) + " overdue"
						}
					}
					_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%s\t%s\t%s\t%s\n", p.Name, p.OutstandingTotal, age, last, avg, projected)
				}
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if result.By == "risk" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "At risk: %.2f  ·  Write-off candidates: %.2f  ·  New members: %d\n", result.AtRiskTotal, result.WriteOffTotal, result.NewMembers)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&friendRef, "friend", "", "Friend name or id")
	cmd.Flags().StringVar(&groupRef, "group", "", "Group name or id")
	cmd.Flags().StringVar(&by, "by", "risk", "Lens: risk|contribution|collectability")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency code filter")
	cmd.Flags().IntVar(&writeOffDays, "write-off-days", 365, "Days after which old debt may be written off")
	cmd.Flags().IntVar(&ghostDays, "ghost-days", 180, "Days of inactivity considered ghosted")
	cmd.Flags().IntVar(&minEpisodes, "min-episodes", 1, "Minimum closed episodes required for avg latency")
	cmd.Flags().StringVar(&since, "since", "", "Window contribution (paid/owed) to on/after YYYY-MM-DD; collectability and debt age always use full history")
	cmd.AddCommand(newFairnessNudgeCmd(flags))
	return cmd
}

func computeFairness(youID int, friends []Friend, groups []Group, expenses []Expense, now time.Time, opts fairnessOpts) fairnessResult {
	result := fairnessResult{By: opts.by, Scope: "all-friends", People: make([]fairnessPerson, 0)}
	if result.By == "" {
		result.By = "risk"
	}

	subjects := make([]fairnessSubject, 0)
	outstanding := make(map[int]map[string]float64)

	if opts.groupScoped {
		for _, g := range groups {
			if g.ID != opts.groupID {
				continue
			}
			result.Scope = "group:" + strings.TrimSpace(g.Name)
			result.GroupCaveat = true
			for _, m := range g.Members {
				if m.ID == youID {
					continue
				}
				name := strings.TrimSpace(strings.TrimSpace(m.FirstName) + " " + strings.TrimSpace(m.LastName))
				if name == "" {
					name = fmt.Sprintf("user %d", m.ID)
				}
				subjects = append(subjects, fairnessSubject{id: m.ID, name: name})
			}
			for _, d := range g.SimplifiedDebts {
				if d.To != youID || d.From == youID {
					continue
				}
				if opts.currency != "" && !strings.EqualFold(strings.TrimSpace(d.CurrencyCode), opts.currency) {
					continue
				}
				amt := parseAmount(d.Amount)
				if amt <= 0 {
					continue
				}
				if outstanding[d.From] == nil {
					outstanding[d.From] = make(map[string]float64)
				}
				cc := strings.ToUpper(strings.TrimSpace(d.CurrencyCode))
				outstanding[d.From][cc] = round2(outstanding[d.From][cc] + amt)
			}
			break
		}
	} else {
		for _, f := range friends {
			if opts.friendID != 0 && f.ID != opts.friendID {
				continue
			}
			name := friendDisplayName(f)
			if name == "" {
				name = fmt.Sprintf("friend %d", f.ID)
			}
			if opts.friendID != 0 {
				result.Scope = "friend:" + name
			}
			subjects = append(subjects, fairnessSubject{id: f.ID, name: name})
			for _, b := range f.Balance {
				if opts.currency != "" && !strings.EqualFold(strings.TrimSpace(b.CurrencyCode), opts.currency) {
					continue
				}
				amt := parseAmount(b.Amount)
				if amt <= 0 {
					continue
				}
				if outstanding[f.ID] == nil {
					outstanding[f.ID] = make(map[string]float64)
				}
				cc := strings.ToUpper(strings.TrimSpace(b.CurrencyCode))
				outstanding[f.ID][cc] = round2(outstanding[f.ID][cc] + amt)
			}
		}
	}

	if len(subjects) == 0 {
		return result
	}

	type rawStats struct {
		person           fairnessPerson
		lastActivityDays int
	}

	raw := make([]rawStats, 0, len(subjects))
	maxOut := 0.0
	for _, s := range subjects {
		p := fairnessPerson{UserID: s.id, Name: s.name, OutstandingByCurrency: make(map[string]float64), RiskTier: "new", Action: "no history yet — nothing to collect"}
		if currencies := outstanding[s.id]; currencies != nil {
			for cc, amt := range currencies {
				if amt > 0 {
					p.OutstandingByCurrency[cc] = round2(amt)
					p.OutstandingTotal += amt
				}
			}
			p.OutstandingTotal = round2(p.OutstandingTotal)
		}

		matchedAny := false
		events := make([]subjectEvent, 0)
		for _, e := range expenses {
			if expenseDeleted(e.DeletedAt) {
				continue
			}
			if opts.groupScoped && e.GroupID != opts.groupID {
				continue
			}
			if opts.currency != "" && !strings.EqualFold(strings.TrimSpace(e.CurrencyCode), opts.currency) {
				continue
			}
			member := false
			var row ExpenseUser
			for _, u := range e.Users {
				if u.UserID == s.id {
					row = u
					member = true
					break
				}
			}
			if !member {
				continue
			}
			matchedAny = true
			// Collectability signals (debt age / settle latency / last-settled /
			// ghost) use FULL history -- a debt's age is an absolute fact, not
			// scoped by --since; only the contribution numbers below are windowed,
			// so --since can never suppress an old debt's write_off tier.
			if d, ok := parseSplitwiseDate(e.Date); ok {
				events = append(events, subjectEvent{date: d, payment: e.Payment})
			}
			if opts.hasSince {
				if t, ok := parseSplitwiseDate(e.Date); !ok || t.Before(opts.since) {
					continue
				}
			}
			if !e.Payment {
				p.Paid += parseAmount(row.PaidShare)
				p.Owed += parseAmount(row.OwedShare)
				p.ExpenseCount++
				if parseAmount(row.PaidShare) > 0 {
					p.PayerCount++
				}
			}
		}

		// A positive outstanding balance is a current fact from Friend.Balance /
		// SimplifiedDebts and is NOT date-filtered. A debtor whose every in-scope
		// expense predates --since must still count as "has history" — otherwise
		// they'd be miscounted as a new member and dropped from every view,
		// silently hiding a real (often old) outstanding debt.
		p.HasHistory = matchedAny || p.OutstandingTotal > 0
		p.Paid = round2(p.Paid)
		p.Owed = round2(p.Owed)
		p.Net = round2(p.Paid - p.Owed)
		if p.Owed > 0 {
			r := round2(p.Paid / p.Owed)
			p.CarryRatio = &r
		}
		p.Role = roleForContribution(p.HasHistory, p.Paid, p.Owed, p.CarryRatio)

		ep := episodeMetrics(now, events, opts.minEpisodes)
		p.DebtAgeDays = ep.debtAgeDays
		p.LastSettledDays = ep.lastSettledDays
		p.AvgLatencyDays = ep.avgLatencyDays
		// Project a settle date only for people who actually owe right now. The
		// episode model can leave an "open" episode (and thus a debtAge) for a
		// fully-settled person whose most recent shared expense wasn't followed by
		// a payment record; projecting a settle date for someone who owes 0 is
		// noise ("overdue by 2459d" for a $0 balance), so gate on outstanding>0.
		if p.OutstandingTotal > 0 {
			p.ProjectedDaysOut, p.ProjectedSettleDate = projectSettle(p.DebtAgeDays, p.AvgLatencyDays, now)
		}

		if !p.HasHistory {
			result.NewMembers++
		}
		if p.HasHistory && p.OutstandingTotal > 0 && p.OutstandingTotal > maxOut {
			maxOut = p.OutstandingTotal
		}
		raw = append(raw, rawStats{person: p, lastActivityDays: ep.lastActivityDays})
	}

	if maxOut <= 0 {
		maxOut = 1
	}

	for i := range raw {
		p := &raw[i].person
		if !p.HasHistory {
			continue
		}
		if p.OutstandingTotal <= 0 {
			p.RiskTier = "settled"
			p.Action = ""
			continue
		}

		debtAge := 0.0
		if p.DebtAgeDays != nil {
			debtAge = float64(*p.DebtAgeDays)
		}
		// lastActivityDays was derived once in episodeMetrics (large sentinel when the
		// subject has no parseable-date events, pinning ghostScore to its max).
		lastActivity := float64(raw[i].lastActivityDays)
		ageScore := clampUnit(debtAge/float64(opts.writeOffDays)) * 40
		ghostScore := clampUnit(lastActivity/float64(opts.ghostDays)) * 30
		// latScore: 0-20 points when average settle latency is known; a neutral
		// 10 (the midpoint) when fewer than --min-episodes closed cycles exist.
		// This intentionally caps the achievable risk score at 90 for
		// unknown-latency debtors -- they get the benefit of the doubt versus a
		// confirmed slow payer (score 100).
		latScore := 10.0
		if p.AvgLatencyDays != nil {
			latScore = clampUnit(*p.AvgLatencyDays/float64(opts.ghostDays)) * 20
		}
		magScore := clampUnit(p.OutstandingTotal/maxOut) * 10
		score := round2(ageScore + ghostScore + latScore + magScore)
		p.RiskScore = &score

		if p.DebtAgeDays != nil && *p.DebtAgeDays >= opts.writeOffDays && lastActivity >= float64(opts.ghostDays) {
			p.RiskTier = "write_off"
			p.Action = "consider writing off — old and gone quiet"
		} else if score >= 60 {
			p.RiskTier = "chase"
			p.Action = "follow up directly"
		} else if score >= 30 {
			p.RiskTier = "nudge"
			p.Action = "send a reminder"
		} else {
			p.RiskTier = "on_track"
			p.Action = ""
		}
	}

	people := make([]fairnessPerson, 0)
	for _, row := range raw {
		p := row.person
		switch result.By {
		case "risk":
			if p.HasHistory && p.OutstandingTotal > 0 {
				people = append(people, p)
				result.AtRiskTotal += p.OutstandingTotal
				if p.RiskTier == "write_off" {
					result.WriteOffTotal += p.OutstandingTotal
				}
			}
		case "collectability", "contribution":
			if p.HasHistory {
				people = append(people, p)
			}
		default:
			if p.HasHistory && p.OutstandingTotal > 0 {
				people = append(people, p)
			}
		}
	}
	result.AtRiskTotal = round2(result.AtRiskTotal)
	result.WriteOffTotal = round2(result.WriteOffTotal)

	switch result.By {
	case "risk":
		sort.Slice(people, func(i, j int) bool {
			si := 0.0
			sj := 0.0
			if people[i].RiskScore != nil {
				si = *people[i].RiskScore
			}
			if people[j].RiskScore != nil {
				sj = *people[j].RiskScore
			}
			if si != sj {
				return si > sj
			}
			if people[i].OutstandingTotal != people[j].OutstandingTotal {
				return people[i].OutstandingTotal > people[j].OutstandingTotal
			}
			return people[i].Name < people[j].Name
		})
	case "collectability":
		sort.Slice(people, func(i, j int) bool {
			if people[i].DebtAgeDays == nil && people[j].DebtAgeDays != nil {
				return false
			}
			if people[i].DebtAgeDays != nil && people[j].DebtAgeDays == nil {
				return true
			}
			if people[i].DebtAgeDays != nil && people[j].DebtAgeDays != nil && *people[i].DebtAgeDays != *people[j].DebtAgeDays {
				return *people[i].DebtAgeDays > *people[j].DebtAgeDays
			}
			if people[i].OutstandingTotal != people[j].OutstandingTotal {
				return people[i].OutstandingTotal > people[j].OutstandingTotal
			}
			return people[i].Name < people[j].Name
		})
	case "contribution":
		sort.Slice(people, func(i, j int) bool {
			if people[i].Net != people[j].Net {
				return people[i].Net > people[j].Net
			}
			return people[i].Name < people[j].Name
		})
	}

	result.People = people
	return result
}

func clampUnit(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func projectSettle(debtAgeDays *int, avgLatencyDays *float64, now time.Time) (*int, *string) {
	if debtAgeDays == nil || avgLatencyDays == nil {
		return nil, nil
	}
	daysOut := int(math.Round(*avgLatencyDays)) - *debtAgeDays
	iso := now.AddDate(0, 0, daysOut).Format("2006-01-02")
	return &daysOut, &iso
}

func episodeMetrics(now time.Time, events []subjectEvent, minEpisodes int) episodeState {
	sort.Slice(events, func(i, j int) bool { return events[i].date.Before(events[j].date) })
	var openStart *time.Time
	var lastPayment *time.Time
	latencies := make([]float64, 0)
	for _, e := range events {
		if !e.payment {
			if openStart == nil {
				d := e.date
				openStart = &d
			}
			continue
		}
		if openStart != nil {
			days := e.date.Sub(*openStart).Hours() / 24
			if days < 0 {
				days = 0
			}
			latencies = append(latencies, days)
			openStart = nil
		}
		d := e.date
		lastPayment = &d
	}

	st := episodeState{lastActivityDays: 100000}
	if len(events) > 0 {
		last := events[len(events)-1].date
		st.lastActivityDays = clampDays(int(now.Sub(last).Hours() / 24))
	}
	if openStart != nil {
		d := clampDays(int(now.Sub(*openStart).Hours() / 24))
		st.debtAgeDays = &d
	}
	if lastPayment != nil {
		d := clampDays(int(now.Sub(*lastPayment).Hours() / 24))
		st.lastSettledDays = &d
	}
	if len(latencies) >= minEpisodes {
		total := 0.0
		for _, v := range latencies {
			total += v
		}
		avg := round2(total / float64(len(latencies)))
		st.avgLatencyDays = &avg
	}
	return st
}

func classifyRole(hasHistory bool, ratio *float64) string {
	if !hasHistory {
		return "new"
	}
	if ratio == nil || *ratio < 0.90 {
		return "rider"
	}
	if *ratio > 1.10 {
		return "carrier"
	}
	return "even"
}

// roleForContribution wraps classifyRole to correct the one case classifyRole
// cannot see: a member who paid into the group while owing nothing (excluded
// from the split) has a nil CarryRatio, which classifyRole defaults to "rider".
// That member is a pure carrier — an unbounded carry ratio — not a rider.
func roleForContribution(hasHistory bool, paid, owed float64, ratio *float64) string {
	role := classifyRole(hasHistory, ratio)
	if role == "rider" && owed == 0 && paid > 0 {
		return "carrier"
	}
	return role
}

func clampDays(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func resolveFairnessFriend(input string, friends []Friend) (int, string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, "", false
	}
	if isAllDigits(trimmed) {
		id, _ := strconv.Atoi(trimmed)
		for _, f := range friends {
			if f.ID == id {
				name := friendDisplayName(f)
				if name == "" {
					name = fmt.Sprintf("friend %d", f.ID)
				}
				return f.ID, name, true
			}
		}
		return 0, "", false
	}
	for _, f := range friends {
		if strings.EqualFold(friendDisplayName(f), trimmed) {
			name := friendDisplayName(f)
			if name == "" {
				name = fmt.Sprintf("friend %d", f.ID)
			}
			return f.ID, name, true
		}
	}
	return 0, "", false
}

func resolveFairnessGroup(input string, groups []Group) (int, string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, "", false
	}
	if isAllDigits(trimmed) {
		id, _ := strconv.Atoi(trimmed)
		for _, g := range groups {
			if g.ID == id {
				return g.ID, strings.TrimSpace(g.Name), true
			}
		}
		return 0, "", false
	}
	for _, g := range groups {
		if strings.EqualFold(strings.TrimSpace(g.Name), trimmed) {
			return g.ID, strings.TrimSpace(g.Name), true
		}
	}
	return 0, "", false
}

// humanizeDays renders an integer day count as a readable duration for the
// human table (e.g. 1558 -> "4y 3mo 8d", 19 -> "2w 5d", 5 -> "5d"). Display aid
// only: the JSON output keeps the raw `*_days` integers so agents and analytics
// tools do their own (calendar-accurate) conversion. Uses approximate units —
// year=365d, month=30d, week=7d — fine for a readability hint and deterministic
// without needing the start date.
func humanizeDays(days int) string {
	if days < 0 {
		days = 0
	}
	switch {
	case days < 7:
		return fmt.Sprintf("%dd", days)
	case days < 30:
		w, d := days/7, days%7
		if d == 0 {
			return fmt.Sprintf("%dw", w)
		}
		return fmt.Sprintf("%dw %dd", w, d)
	case days < 365:
		mo, d := days/30, days%30
		if d == 0 {
			return fmt.Sprintf("%dmo", mo)
		}
		return fmt.Sprintf("%dmo %dd", mo, d)
	default:
		y := days / 365
		rem := days % 365
		mo, d := rem/30, rem%30
		parts := []string{fmt.Sprintf("%dy", y)}
		if mo > 0 {
			parts = append(parts, fmt.Sprintf("%dmo", mo))
		}
		if d > 0 {
			parts = append(parts, fmt.Sprintf("%dd", d))
		}
		return strings.Join(parts, " ")
	}
}

// ageCell renders a *int day-age for a human table: "-" when the age is unknown
// (nil — e.g. the contributing expense is outside the synced window), otherwise
// the friendly humanizeDays form. Shared by the `fairness` and `debts` reports so
// their AGE columns render identically; JSON keeps the raw `*_days` integer.
func ageCell(days *int) string {
	if days == nil {
		return "-"
	}
	return humanizeDays(*days)
}

func tierGlyph(tier string) string {
	switch tier {
	case "write_off":
		return "🔴"
	case "chase":
		return "🟠"
	case "nudge":
		return "🟡"
	default:
		return "🟢"
	}
}
