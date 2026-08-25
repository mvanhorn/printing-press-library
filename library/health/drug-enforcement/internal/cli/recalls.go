// Hand-authored novel commands for drug-enforcement-pp-cli.
//
// These wrap the openFDA /drug/enforcement.json endpoint with pre-built search
// expressions for the common recall-lookup workflows (by drug, by firm, by
// recency, by recall number). They enforce the CLI's safety contract: report
// only FDA enforcement facts, cite the recall_number source ID, print an FDA
// disclaimer, and NEVER characterize a drug as "safe" when no recall is found.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/drug-enforcement/internal/client"

	"github.com/spf13/cobra"
)

const enforcementPath = "/drug/enforcement.json"

// recallDisclaimer is printed on every human-readable recall output. openFDA
// enforcement data is FDA reporting, not medical advice.
const recallDisclaimer = "Disclaimer: openFDA recall data is FDA enforcement reporting, not medical advice. Consult a pharmacist or doctor."

// recallClassLegend explains the FDA recall classes so the classification field
// is interpretable in place.
const recallClassLegend = "Recall classes (FDA):\n" +
	"  Class I   - serious/life-threatening: reasonable probability of serious harm or death\n" +
	"  Class II  - temporary/reversible: may cause temporary or medically reversible harm\n" +
	"  Class III - unlikely to cause harm, but violates FDA regulations"

// classToLabel maps the user-facing numeric recall class to openFDA's text label.
var classToLabel = map[int]string{1: "Class I", 2: "Class II", 3: "Class III"}

// recallRecord is one enforcement record, limited to the reported fields.
type recallRecord struct {
	RecallNumber         string `json:"recall_number"`
	Classification       string `json:"classification"`
	Status               string `json:"status"`
	RecallingFirm        string `json:"recalling_firm"`
	ReasonForRecall      string `json:"reason_for_recall"`
	DistributionPattern  string `json:"distribution_pattern"`
	ProductDescription   string `json:"product_description"`
	RecallInitiationDate string `json:"recall_initiation_date"`
	ReportDate           string `json:"report_date"`
	State                string `json:"state"`
	Country              string `json:"country"`
}

type enforcementEnvelope struct {
	Meta struct {
		Results struct {
			Total int `json:"total"`
		} `json:"results"`
	} `json:"meta"`
	Results []recallRecord `json:"results"`
}

func newCheckCmd(flags *rootFlags) *cobra.Command {
	var class int
	var limit int
	cmd := &cobra.Command{
		Use:   "check <drug>",
		Short: "Find active recalls mentioning a drug (optionally filter by class)",
		Long: "Search FDA enforcement records whose product description mentions the drug.\n\n" +
			"A drug with no matching record is reported as \"no recall records found\" — this is\n" +
			"NOT a statement that the drug is safe.\n\n" + recallClassLegend,
		Example: "  drug-enforcement-pp-cli check \"ibuprofen\"\n  drug-enforcement-pp-cli check \"metformin\" --class 1",
		// An unknown drug is indistinguishable from a valid drug with no recalls;
		// both return "no recall records found" with exit 0. Skip the error-path probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search enforcement records for a drug")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a drug name is required, e.g. check \"ibuprofen\""))
			}
			term := strings.TrimSpace(args[0])
			if term == "" {
				return usageErr(fmt.Errorf("drug name must not be empty"))
			}
			clause := phrase("product_description", term)
			if cmd.Flags().Changed("class") {
				label, ok := classToLabel[class]
				if !ok {
					return usageErr(fmt.Errorf("--class must be 1, 2, or 3, got %d", class))
				}
				clause = clause + " AND " + phrase("classification", label)
			}
			return runRecallSearch(cmd, flags, clause, limit, term)
		},
	}
	cmd.Flags().IntVar(&class, "class", 0, "Filter by recall class: 1=Class I, 2=Class II, 3=Class III")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum records to return")
	return cmd
}

func newFirmCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "firm <name>",
		Short:   "List recalls by a recalling firm / manufacturer",
		Example: "  drug-enforcement-pp-cli firm \"Teva\"",
		// An unknown firm returns "no recall records found" with exit 0, same as
		// a real firm with no recalls; skip the error-path probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search enforcement records by firm")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a firm name is required, e.g. firm \"Teva\""))
			}
			term := strings.TrimSpace(args[0])
			if term == "" {
				return usageErr(fmt.Errorf("firm name must not be empty"))
			}
			return runRecallSearch(cmd, flags, phrase("recalling_firm", term), limit, term)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum records to return")
	return cmd
}

func newRecentCmd(flags *rootFlags) *cobra.Command {
	var days int
	var limit int
	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "List recalls initiated in the last N days",
		Example:     "  drug-enforcement-pp-cli recent --days 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search enforcement records by recency")
				return nil
			}
			if !cmd.Flags().Changed("days") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days is required, e.g. recent --days 30"))
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be >= 1, got %d", days))
			}
			now := time.Now().UTC()
			from := now.AddDate(0, 0, -(days - 1))
			search := fmt.Sprintf("recall_initiation_date:[%s TO %s]", from.Format("20060102"), now.Format("20060102"))
			return runRecallSearch(cmd, flags, search, limit, fmt.Sprintf("last %d day(s)", days))
		},
	}
	cmd.Flags().IntVar(&days, "days", 0, "Window in days (>= 1)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum records to return")
	return cmd
}

func newReferenceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reference <recall_number>",
		Short:   "Show full detail for a single recall number",
		Long:    "Fetch the enforcement record for an exact recall number and print full detail.\n\n" + recallClassLegend,
		Example: "  drug-enforcement-pp-cli reference D-1234-2026",
		// An unknown recall number returns "no recall records found" with exit 0,
		// same as any valid-but-absent number; skip the error-path probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch a single recall record")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a recall number is required, e.g. reference D-1234-2026"))
			}
			num := strings.TrimSpace(args[0])
			if num == "" {
				return usageErr(fmt.Errorf("recall number must not be empty"))
			}
			return runRecallSearch(cmd, flags, phrase("recall_number", num), 1, num)
		},
	}
	return cmd
}

// runRecallSearch executes an enforcement search, enforcing the safety contract:
// a 404 "no matches" is reported as "no recall records found" (never "safe"),
// and human output always carries the recall number, class legend, and disclaimer.
func runRecallSearch(cmd *cobra.Command, flags *rootFlags, search string, limit int, subject string) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	params := map[string]string{
		"search": search,
		"sort":   "recall_initiation_date:desc",
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}

	data, err := c.Get(ctx, enforcementPath, params)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			// openFDA reports zero matches as HTTP 404 NOT_FOUND. This is an
			// empty result, not a failure — and explicitly NOT "safe".
			return emitNoRecords(cmd, flags, subject)
		}
		return classifyAPIError(err, flags)
	}

	// Machine output only on an explicit request (--json/--agent). A piped
	// stdout must NOT silently switch to the machine envelope: plain-text
	// consumers (e.g. `... | head`) still need the human class legend and
	// disclaimer guardrail, which only the formatted branch below emits.
	if flags.asJSON || flags.agent {
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}

	var env enforcementEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		// Fall back to raw output rather than dropping data.
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	if len(env.Results) == 0 {
		return emitNoRecords(cmd, flags, subject)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Total matches: %d (showing %d)\n\n", env.Meta.Results.Total, len(env.Results))
	for _, r := range env.Results {
		// Lead with the recall number — the FDA source record ID.
		fmt.Fprintf(w, "[%s] %s\n", dash(r.RecallNumber), dash(r.Classification))
		fmt.Fprintf(w, "  Firm:         %s\n", dash(r.RecallingFirm))
		fmt.Fprintf(w, "  Initiated:    %s\n", dash(normalizeRecallDate(r.RecallInitiationDate)))
		fmt.Fprintf(w, "  Status:       %s\n", dash(r.Status))
		fmt.Fprintf(w, "  Product:      %s\n", dash(clip(r.ProductDescription, 100)))
		fmt.Fprintf(w, "  Reason:       %s\n", dash(clip(r.ReasonForRecall, 100)))
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, recallClassLegend)
	fmt.Fprintln(w)
	fmt.Fprintln(w, recallDisclaimer)
	return nil
}

// emitNoRecords prints the guardrail "no recall records found" result. It never
// implies the drug/firm is safe.
func emitNoRecords(cmd *cobra.Command, flags *rootFlags, subject string) error {
	if flags.asJSON || flags.agent {
		out := map[string]any{
			"total":      0,
			"records":    []any{},
			"message":    fmt.Sprintf("no recall records found for %q in openFDA enforcement data", subject),
			"disclaimer": recallDisclaimer,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return printOutput(cmd.OutOrStdout(), json.RawMessage(b), true)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "No recall records found for %q in openFDA enforcement data.\n\n", subject)
	fmt.Fprintln(cmd.OutOrStdout(), recallDisclaimer)
	return nil
}

// phrase builds an openFDA field phrase clause. openFDA reads spaces (encoded as
// "+" on the wire by the HTTP client) as term separators, so build with literal
// spaces and let the client percent-encode.
func phrase(field, term string) string {
	term = strings.ReplaceAll(strings.TrimSpace(term), `"`, "")
	return fmt.Sprintf(`%s:"%s"`, field, term)
}

// normalizeRecallDate converts openFDA compact YYYYMMDD dates to ISO YYYY-MM-DD,
// passing through anything that does not parse.
func normalizeRecallDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return s
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
