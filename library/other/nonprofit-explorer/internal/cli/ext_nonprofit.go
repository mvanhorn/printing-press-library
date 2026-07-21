// Copyright 2026 Sean Fannan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command extensions for the Nonprofit Explorer CLI.
// Registered via registerNovelCommand so `generate --force` preserves them.
// These are analytical/derived verbs layered on the two raw endpoints
// (search.json, organizations/<ein>.json) that the generator mirrors as
// `search-json` and `organizations`. They always read live (derived views
// over the public API; no local-store dependency).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/nonprofit-explorer/internal/client"
)

// nteeMajor maps the leading NTEE letter to its human-readable major group.
var nteeMajor = map[byte]string{
	'A': "Arts, Culture & Humanities",
	'B': "Education",
	'C': "Environment",
	'D': "Animal-Related",
	'E': "Health Care",
	'F': "Mental Health & Crisis",
	'G': "Diseases & Disorders",
	'H': "Medical Research",
	'I': "Crime & Legal",
	'J': "Employment",
	'K': "Food, Agriculture & Nutrition",
	'L': "Housing & Shelter",
	'M': "Public Safety & Disaster",
	'N': "Recreation & Sports",
	'O': "Youth Development",
	'P': "Human Services",
	'Q': "International & Foreign Affairs",
	'R': "Civil Rights & Advocacy",
	'S': "Community Improvement",
	'T': "Philanthropy & Grantmaking",
	'U': "Science & Technology",
	'V': "Social Science",
	'W': "Public & Societal Benefit",
	'X': "Religion-Related",
	'Y': "Mutual & Membership Benefit",
	'Z': "Unknown",
}

func nteeCategory(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if name, ok := nteeMajor[code[0]]; ok {
		return name
	}
	return ""
}

// nteeName returns the standard NTEE-CC name for a full code (e.g. T23 ->
// "Private Operating Foundations") from the embedded table in
// ext_ntee_table.go, falling back to the letter major group, then "".
// Raw codes may carry suffixes (T23Z, B82Z); match on the leading 3 chars.
func nteeName(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) >= 3 {
		if name, ok := nteeCodeNames[code[:3]]; ok {
			return name
		}
	}
	return nteeCategory(code)
}

// printJSONLive marshals v and routes it through the standard output pipeline
// with the agent envelope's meta.source set to "live". The generated default
// (printJSON -> printOutputWithFlags) stamps source:"local", which mislabels
// these always-live analytical commands.
func printJSONLive(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": "live"})
}

// pctOf renders num/den as a percentage string, or em-dash when unavailable.
func pctOf(num, den *float64) string {
	if num == nil || den == nil || *den == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", *num / *den * 100)
}

var einDigits = regexp.MustCompile(`\D`)

// normalizeEIN strips any non-digit characters (e.g. the dash in 53-0196605)
// and validates the result is exactly 9 digits.
func normalizeEIN(raw string) (string, error) {
	d := einDigits.ReplaceAllString(raw, "")
	if len(d) != 9 {
		return "", fmt.Errorf("invalid EIN %q: expected 9 digits (got %d)", raw, len(d))
	}
	return d, nil
}

func fmtUSD(v *float64) string {
	if v == nil {
		return "—"
	}
	n := int64(*v)
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	res := "$" + string(out)
	if neg {
		res = "-" + res
	}
	return res
}

// --- API response types (only the fields we use) ---

type npSearchOrg struct {
	EIN         int64   `json:"ein"`
	StrEIN      string  `json:"strein"`
	Name        string  `json:"name"`
	SubName     string  `json:"sub_name"`
	City        string  `json:"city"`
	State       string  `json:"state"`
	NteeCode    string  `json:"ntee_code"`
	RawNtee     string  `json:"raw_ntee_code"`
	Subseccd    int     `json:"subseccd"`
	Score       float64 `json:"score"`
	HaveFilings bool    `json:"have_filings"`
}

type npSearchResp struct {
	TotalResults  int           `json:"total_results"`
	NumPages      int           `json:"num_pages"`
	CurPage       int           `json:"cur_page"`
	Organizations []npSearchOrg `json:"organizations"`
}

type npFiling struct {
	TaxPrdYr    int      `json:"tax_prd_yr"`
	FormType    int      `json:"formtype"`
	PdfURL      string   `json:"pdf_url"`
	TotRevenue  *float64 `json:"totrevenue"`
	TotExpenses *float64 `json:"totfuncexpns"`
	TotAssets   *float64 `json:"totassetsend"`
	TotLiab     *float64 `json:"totliabend"`
	// Revenue composition (Form 990 / 990-EZ extracts; nil on 990-PF).
	Contributions *float64 `json:"totcntrbgfts,omitempty"`
	ProgramRev    *float64 `json:"totprgmrevnue,omitempty"`
	InvestmentInc *float64 `json:"invstmntinc,omitempty"`
	// People / compensation aggregates (Form 990 extracts; nil on 990-PF).
	OfficerComp    *float64 `json:"compnsatncurrofcr,omitempty"`
	OfficerCompPct *float64 `json:"pct_compnsatncurrofcr,omitempty"`
	OtherSalaries  *float64 `json:"othrsalwages,omitempty"`
	PayrollTax     *float64 `json:"payrolltx,omitempty"`
	ProFundraising *float64 `json:"profndraising,omitempty"`
}

// formTypeName renders the IRS form type code from the extract.
func formTypeName(t int) string {
	switch t {
	case 0:
		return "990"
	case 1:
		return "990-EZ"
	case 2:
		return "990-PF"
	default:
		return fmt.Sprintf("form %d", t)
	}
}

type npOrganization struct {
	EIN      int64  `json:"ein"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	City     string `json:"city"`
	State    string `json:"state"`
	Zipcode  string `json:"zipcode"`
	NteeCode string `json:"ntee_code"`
	Subseccd int    `json:"subsection_code"`
	Ruling   string `json:"ruling_date"`
}

type npOrgResp struct {
	Organization       npOrganization `json:"organization"`
	FilingsWithData    []npFiling     `json:"filings_with_data"`
	FilingsWithoutData []npFiling     `json:"filings_without_data"`
}

func fetchSearch(ctx context.Context, c *client.Client, q, state string, ntee, ccode, page int) (*npSearchResp, error) {
	params := url.Values{}
	if q != "" {
		params.Set("q", q)
	}
	if state != "" {
		params.Set("state[id]", strings.ToUpper(state))
	}
	if ntee > 0 {
		params.Set("ntee[id]", strconv.Itoa(ntee))
	}
	if ccode > 0 {
		params.Set("c_code[id]", strconv.Itoa(ccode))
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	raw, err := c.GetWithHeadersValues(ctx, "/search.json", params, nil)
	if err != nil {
		return nil, err
	}
	var resp npSearchResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	return &resp, nil
}

func fetchOrg(ctx context.Context, c *client.Client, ein string) (*npOrgResp, error) {
	raw, err := c.Get(ctx, "/organizations/"+ein+".json", nil)
	if err != nil {
		return nil, err
	}
	var resp npOrgResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing organization response: %w", err)
	}
	return &resp, nil
}

// resolution records how a name argument was auto-resolved to an EIN. It is
// nil when the caller passed a raw EIN (no lookup happened).
type resolution struct {
	Input string `json:"input"`
	EIN   string `json:"ein"`
	Name  string `json:"name"`
	City  string `json:"city"`
	State string `json:"state"`
}

// resolveEINOrName accepts either an EIN (with or without dash) or a nonprofit
// name. If the arg normalizes to a valid 9-digit EIN it is used directly. Otherwise
// the arg is treated as a name and resolved to the TOP search result's EIN. For
// human output the resolution is printed to stderr; for --agent/--json the returned
// *resolution is embedded in the caller's envelope under "resolved". A name with
// zero matches returns a non-zero notFoundErr (and a JSON error envelope in agent mode).
func resolveEINOrName(ctx context.Context, c *client.Client, flags *rootFlags, cmd *cobra.Command, arg string) (string, *resolution, error) {
	if ein, err := normalizeEIN(arg); err == nil {
		return ein, nil, nil
	}
	resp, err := fetchSearch(ctx, c, arg, "", 0, 0, 0)
	if err != nil {
		// ProPublica's search.json returns HTTP 404 (not a 200 with an empty
		// array) when a query matches zero organizations. Treat that as a clean
		// "not found" rather than surfacing the raw API error body.
		if strings.Contains(err.Error(), "HTTP 404") {
			resp = &npSearchResp{}
		} else {
			return "", nil, classifyAPIError(err, flags)
		}
	}
	if len(resp.Organizations) == 0 {
		nf := notFoundErr(fmt.Errorf("no nonprofit found matching %q", arg))
		writeAPIErrorEnvelope(flags, nf, ExitCode(nf))
		return "", nil, nf
	}
	top := resp.Organizations[0]
	ein := fmt.Sprintf("%09d", top.EIN)
	res := &resolution{Input: arg, EIN: einDash(ein), Name: top.Name, City: top.City, State: top.State}
	if !flags.asJSON {
		fmt.Fprintf(cmd.ErrOrStderr(), "Resolved %q → EIN %s (%s, %s, %s)\n",
			arg, res.EIN, top.Name, top.City, top.State)
	}
	return ein, res, nil
}

// latestFiling returns the filing with data for the most recent tax year.
func latestFiling(f []npFiling) *npFiling {
	if len(f) == 0 {
		return nil
	}
	best := &f[0]
	for i := 1; i < len(f); i++ {
		if f[i].TaxPrdYr > best.TaxPrdYr {
			best = &f[i]
		}
	}
	return best
}
