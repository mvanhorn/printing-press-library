// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

type duplicateSuspectItem struct {
	LeftID      string  `json:"left_id"`
	LeftEmail   string  `json:"left_email,omitempty"`
	RightID     string  `json:"right_id"`
	RightEmail  string  `json:"right_email,omitempty"`
	EmailSim    float64 `json:"email_sim"`
	NameSim     float64 `json:"name_sim"`
	CompanySim  float64 `json:"company_sim"`
	CombinedSim float64 `json:"combined_sim"`
}

const duplicateSuspectsPageSize = 200

func newNovelDuplicateSuspectsCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var limit int
	var maxScanPages int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "duplicate-suspects",
		Short: "Surface likely duplicate contacts via fuzzy match on email-local-part, normalized name, and company.",
		Long: strings.Trim(`
Stream contacts in pages of 200 ordered by id and, within each page, block
candidates by the first character of the lowercased email local-part to
bound pairwise comparison cost. Compute per-field Jaro-Winkler similarity
on email-local-part, "firstname lastname", and company; combine as
0.5*email + 0.3*name + 0.2*company. A pair qualifies when combined_sim is
at least --threshold AND the two emails are not byte-identical (we want
fuzzy duplicates, not exact ones — exact duplicates are the API's problem,
not the analyst's).

Scan is capped at --max-scan-pages pages so the command stays bounded on
large portals. The full scanned_contacts count is still reported even when
output is truncated by --limit.
`, "\n"),
		Example: strings.Trim(`
  # Find likely duplicates at >=0.85 combined similarity
  hubspot-pp-cli duplicate-suspects --threshold 0.85 --limit 50 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if threshold < 0 || threshold > 1 {
				return usageErr(fmt.Errorf("--threshold must be in [0.0, 1.0]"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if maxScanPages <= 0 {
				return usageErr(fmt.Errorf("--max-scan-pages must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			items := make([]duplicateSuspectItem, 0)
			scanned := 0
			parseErrors := 0
			capHit := false
			var lastID string
			for page := 0; page < maxScanPages; page++ {
				batch, batchParseErrors, err := loadContactsPage(cmd.Context(), db, lastID, duplicateSuspectsPageSize)
				if err != nil {
					return err
				}
				parseErrors += batchParseErrors
				if len(batch) == 0 {
					break
				}
				scanned += len(batch)
				pairs := findDuplicateSuspects(batch, threshold)
				items = append(items, pairs...)
				lastID = batch[len(batch)-1].ID
				if len(batch) < duplicateSuspectsPageSize {
					break
				}
				if page == maxScanPages-1 {
					capHit = true
				}
			}
			// Sort by combined similarity descending so the strongest matches
			// land at the top of the truncated output. Tie-break on left/right
			// id for a deterministic ordering across runs.
			sort.Slice(items, func(i, j int) bool {
				if items[i].CombinedSim != items[j].CombinedSim {
					return items[i].CombinedSim > items[j].CombinedSim
				}
				if items[i].LeftID != items[j].LeftID {
					return items[i].LeftID < items[j].LeftID
				}
				return items[i].RightID < items[j].RightID
			})
			matches := len(items)
			items = limitInt(items, limit)
			result := map[string]any{
				"threshold":        threshold,
				"max_scan_pages":   maxScanPages,
				"items":            items,
				"scanned_contacts": scanned,
				"matches":          matches,
			}
			if capHit {
				result["cap_hit"] = true
			}
			if capHit && matches == 0 {
				result["note"] = fmt.Sprintf("scan cap reached at %d pages (%d contacts) without finding pairs above threshold; raise --max-scan-pages to widen the scan", maxScanPages, scanned)
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 0.85, "Minimum combined Jaro-Winkler similarity (0.0-1.0) for a pair to qualify")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of items to return in output")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum number of 200-row pages to scan before stopping")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}

// loadContactsPage returns the next page of contacts ordered by id, with id
// strictly greater than afterID. Pages of 200 keep block-wise pairwise
// comparison cost bounded. Returns (page, parseErrors, err).
func loadContactsPage(ctx context.Context, db *store.Store, afterID string, pageSize int) ([]contactProps, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT "id", "data" FROM "crm" WHERE "id" > ? ORDER BY "id" LIMIT ?`,
		afterID, pageSize,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("querying contacts page: %w", err)
	}
	defer rows.Close()
	out := make([]contactProps, 0, pageSize)
	parseErrors := 0
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, parseErrors, fmt.Errorf("scanning contacts page row: %w", err)
		}
		p, perr := parseContactProps(id, json.RawMessage(data))
		if perr != nil {
			parseErrors++
			continue
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, parseErrors, fmt.Errorf("iterating contacts page: %w", err)
	}
	return out, parseErrors, nil
}

// findDuplicateSuspects performs the blocked pairwise comparison inside a
// single page. Blocking by the first character of the lowercased email
// local-part keeps the worst-case pair count to sum_b (n_b choose 2) per
// page rather than n*(n-1)/2.
func findDuplicateSuspects(contacts []contactProps, threshold float64) []duplicateSuspectItem {
	type prepared struct {
		props        contactProps
		emailLocal   string
		nameNorm     string
		companyNorm  string
		blockKey     rune
		emailLowered string
	}
	prepped := make([]prepared, 0, len(contacts))
	for _, c := range contacts {
		emailLocal, _ := splitEmail(c.Email)
		if emailLocal == "" {
			continue
		}
		p := prepared{
			props:        c,
			emailLocal:   normalizeForCompare(emailLocal),
			nameNorm:     normalizeForCompare(strings.TrimSpace(c.FirstName + " " + c.LastName)),
			companyNorm:  normalizeForCompare(c.Company),
			emailLowered: strings.ToLower(strings.TrimSpace(c.Email)),
		}
		if p.emailLocal == "" {
			continue
		}
		// blockKey indexes emailLocal[0] as a single byte. This is an ASCII
		// assumption: real-world HubSpot email local-parts are virtually all
		// ASCII (RFC 5321 5.1, SMTPUTF8 still rare). When a non-ASCII rune
		// somehow survives normalizeForCompare, the byte-level block key will
		// be the leading byte of the UTF-8 sequence, which still groups
		// records consistently — just less semantically.
		p.blockKey = rune(p.emailLocal[0])
		prepped = append(prepped, p)
	}
	blocks := map[rune][]prepared{}
	for _, p := range prepped {
		blocks[p.blockKey] = append(blocks[p.blockKey], p)
	}
	var out []duplicateSuspectItem
	for _, bucket := range blocks {
		for i := 0; i < len(bucket); i++ {
			for j := i + 1; j < len(bucket); j++ {
				a, b := bucket[i], bucket[j]
				if a.emailLowered != "" && a.emailLowered == b.emailLowered {
					continue
				}
				emailSim := jaroWinkler(a.emailLocal, b.emailLocal)
				nameSim := jaroWinkler(a.nameNorm, b.nameNorm)
				companySim := jaroWinkler(a.companyNorm, b.companyNorm)
				combined := 0.5*emailSim + 0.3*nameSim + 0.2*companySim
				if combined < threshold {
					continue
				}
				out = append(out, duplicateSuspectItem{
					LeftID:      a.props.ID,
					LeftEmail:   a.props.Email,
					RightID:     b.props.ID,
					RightEmail:  b.props.Email,
					EmailSim:    roundTo(emailSim, 3),
					NameSim:     roundTo(nameSim, 3),
					CompanySim:  roundTo(companySim, 3),
					CombinedSim: roundTo(combined, 3),
				})
			}
		}
	}
	return out
}

// splitEmail returns (local, domain) for an email address; both are empty
// when no "@" is present.
func splitEmail(email string) (string, string) {
	t := strings.TrimSpace(email)
	if t == "" {
		return "", ""
	}
	at := strings.LastIndex(t, "@")
	if at < 0 {
		return t, ""
	}
	return t[:at], t[at+1:]
}

// normalizeForCompare lowercases and strips non-alphanumeric characters so
// "Jane Doe" / "jane.doe" / "jane_doe" compare equal under Jaro-Winkler.
func normalizeForCompare(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// jaro returns the Jaro similarity in [0.0, 1.0]. Both empty strings score 1;
// one empty and one non-empty scores 0.
func jaro(s1, s2 string) float64 {
	if s1 == "" && s2 == "" {
		return 1
	}
	if s1 == "" || s2 == "" {
		return 0
	}
	r1 := []rune(s1)
	r2 := []rune(s2)
	matchDistance := len(r1)
	if len(r2) > matchDistance {
		matchDistance = len(r2)
	}
	matchDistance = matchDistance/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}
	matches1 := make([]bool, len(r1))
	matches2 := make([]bool, len(r2))
	matches := 0
	for i := range r1 {
		start := i - matchDistance
		if start < 0 {
			start = 0
		}
		end := i + matchDistance + 1
		if end > len(r2) {
			end = len(r2)
		}
		for j := start; j < end; j++ {
			if matches2[j] {
				continue
			}
			if r1[i] != r2[j] {
				continue
			}
			matches1[i] = true
			matches2[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}
	// Count transpositions.
	k := 0
	transpositions := 0
	for i := range r1 {
		if !matches1[i] {
			continue
		}
		for !matches2[k] {
			k++
		}
		if r1[i] != r2[k] {
			transpositions++
		}
		k++
	}
	m := float64(matches)
	return (m/float64(len(r1)) + m/float64(len(r2)) + (m-float64(transpositions)/2)/m) / 3
}

// jaroWinkler boosts the Jaro score by a 0.1 prefix factor on up to 4
// shared leading characters. Wins where contact records share an obvious
// prefix ("janedoe" vs "janeddoe") but diverge slightly later in the string.
func jaroWinkler(s1, s2 string) float64 {
	j := jaro(s1, s2)
	if j < 0.7 {
		return j
	}
	r1 := []rune(s1)
	r2 := []rune(s2)
	prefix := 0
	maxPrefix := 4
	if len(r1) < maxPrefix {
		maxPrefix = len(r1)
	}
	if len(r2) < maxPrefix {
		maxPrefix = len(r2)
	}
	for i := 0; i < maxPrefix; i++ {
		if r1[i] != r2[i] {
			break
		}
		prefix++
	}
	return j + float64(prefix)*0.1*(1-j)
}
