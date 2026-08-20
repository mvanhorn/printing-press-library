// Hand-authored — NOT generated. Parses the IMO GISIS Ship Particulars HTML page
// (https://gisis.imo.org/Public/SHIPS/ShipDetails.aspx?IMONumber=<imo>) into a
// typed Ship view. Survives regen as a whole hand-authored unit.
package cli

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// shipHistoryEntry captures one row in an inline GISIS history table
// (#sNameHistory, #sFlagHistory, etc.). GISIS embeds the full historical
// changes in the same page that shows the current state — flag-hops, name
// changes, ownership changes all extractable from one fetch.
type shipHistoryEntry struct {
	Value     string `json:"value"`
	Effective string `json:"effective,omitempty"`
}

// shipParticulars is the typed output of parseShipParticularsHTML. Field
// presence depends on GISIS account tier and ship: a "Public User" sees a
// limited subset (no deadweight/builder/classification/operator for most
// ships). Fields that aren't present in the HTML are zero-valued.
type shipParticulars struct {
	IMONumber                    string             `json:"imo_number"`
	Name                         string             `json:"name,omitempty"`
	NameHistory                  []shipHistoryEntry `json:"name_history,omitempty"`
	Flag                         string             `json:"flag,omitempty"`
	FlagHistory                  []shipHistoryEntry `json:"flag_history,omitempty"`
	CallSign                     string             `json:"call_sign,omitempty"`
	MMSI                         string             `json:"mmsi,omitempty"`
	ShipUNSanction               string             `json:"ship_un_sanction,omitempty"`
	OwningEntityUNSanction       string             `json:"owning_entity_un_sanction,omitempty"`
	ShipType                     string             `json:"ship_type,omitempty"`
	ShipTypeHistory              []shipHistoryEntry `json:"ship_type_history,omitempty"`
	DateOfBuild                  string             `json:"date_of_build,omitempty"`
	GrossTonnage                 int                `json:"gross_tonnage,omitempty"`
	Deadweight                   int                `json:"deadweight,omitempty"`
	RegisteredOwner              string             `json:"registered_owner,omitempty"`
	RegisteredOwnerIMOCompanyNum string             `json:"registered_owner_imo_company_number,omitempty"`
	RegisteredOwnerHistory       []shipHistoryEntry `json:"registered_owner_history,omitempty"`
	Operator                     string             `json:"operator,omitempty"`
	OperatorIMOCompanyNum        string             `json:"operator_imo_company_number,omitempty"`
	ShipManager                  string             `json:"ship_manager,omitempty"`
	ShipManagerIMOCompanyNum     string             `json:"ship_manager_imo_company_number,omitempty"`
	ClassificationSociety        string             `json:"classification_society,omitempty"`
	DOCCompany                   string             `json:"doc_company,omitempty"`
	Status                       string             `json:"status,omitempty"`
	LastUpdated                  string             `json:"last_updated,omitempty"`
	SourceURL                    string             `json:"source_url"`
	FetchedAt                    string             `json:"fetched_at"`
	AuthenticatedAs              string             `json:"authenticated_as,omitempty"`
}

// errLoginWall is returned when the parser detects the WebLogin form instead
// of the Ship Particulars module. Callers should surface this as a clear
// "session expired, re-login required" error (typed exit code 4 per the
// generated CLI's auth-failure convention).
var errLoginWall = errors.New("GISIS returned the login form instead of ship particulars — session expired or cookies invalid; re-run press-auth login or refresh Brave session")

// errNotFound is returned when the page renders but reports the IMO doesn't
// exist. GISIS shows a friendly "no record" message rather than HTTP 404.
var errNotFound = errors.New("no ship record found for that IMO number")

// parseShipParticularsHTML extracts ship particulars from the GISIS HTML
// detail page. The selectors here are tuned against a real authenticated
// sample (IMO 9866641 — SIDER ABIDJAN, captured 2026-05-27). Field set will
// expand as we see more vessels with richer particulars (deadweight,
// classification, operator etc. don't appear for all account tiers).
func parseShipParticularsHTML(body []byte, imo, sourceURL string) (shipParticulars, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return shipParticulars{}, fmt.Errorf("parsing HTML: %w", err)
	}

	if isLoginWall(doc) {
		return shipParticulars{}, errLoginWall
	}
	if isNotFound(doc) {
		return shipParticulars{}, errNotFound
	}

	out := shipParticulars{
		IMONumber: imo,
		SourceURL: sourceURL,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Authenticated-as marker (account name visible in header on logged-in pages).
	if name := strings.TrimSpace(doc.Find("div.imo-theme-login.userdetails .imo-theme-user-name span").First().Text()); name != "" {
		out.AuthenticatedAs = name
	}

	// Ship name from #bodyheading .desc (most reliable — even when historical names exist).
	if desc := strings.TrimSpace(doc.Find("#bodyheading .desc").First().Text()); desc != "" {
		out.Name = desc
	}

	// "Updated" date from .gisis-meta.
	if upd := strings.TrimSpace(doc.Find("td.gisis-meta .value").First().Text()); upd != "" {
		out.LastUpdated = upd
	}

	// Walk every <table class="table"> row: label cell -> value cell.
	// GISIS renders particulars as label/value pairs across multiple tables
	// (Identity, Characteristics, Companies). Walk all and dispatch by label.
	doc.Find("table.table tbody tr").Each(func(_ int, row *goquery.Selection) {
		labelText := strings.TrimSpace(row.Find("td.label .infoicon").First().Text())
		label := strings.TrimRight(labelText, ":")
		if label == "" {
			return
		}
		valueCell := row.Find("td.content").First()
		if valueCell.Length() == 0 {
			return
		}
		// Try to find the *Cur "current value" span first (used for fields with
		// history toggles like name/flag/type/owner). Fall back to the whole
		// content-cell text for fields without history.
		curVal := ""
		valueCell.Find("span[id$=Cur]").First().Each(func(_ int, s *goquery.Selection) {
			curVal = cleanWhitespace(s.Text())
		})
		if curVal == "" {
			curVal = cleanWhitespace(valueCell.Text())
		}

		// Look for the parallel history table (#sNameHistory, #sFlagHistory, etc.).
		// They live in the same content cell, hidden by display:none.
		var hist []shipHistoryEntry
		valueCell.Find("table.blank[id$=History] tr").Each(func(_ int, hrow *goquery.Selection) {
			tds := hrow.Find("td")
			if tds.Length() < 2 {
				return
			}
			val := cleanWhitespace(tds.Eq(0).Text())
			eff := extractEffectiveDate(tds.Eq(1).Text())
			if val != "" {
				hist = append(hist, shipHistoryEntry{Value: val, Effective: eff})
			}
		})

		// Dispatch by label (case-sensitive match against GISIS's exact labels).
		switch label {
		case "Name":
			out.Name = curVal
			out.NameHistory = hist
		case "IMO Number":
			// Value cell contains "IMO 9866641" — strip the IMO prefix.
			out.IMONumber = strings.TrimSpace(strings.TrimPrefix(curVal, "IMO"))
		case "Flag":
			out.Flag = curVal
			out.FlagHistory = hist
		case "Call sign":
			out.CallSign = curVal
		case "MMSI":
			out.MMSI = curVal
		case "Ship UN Sanction":
			out.ShipUNSanction = curVal
		case "Owning/operating entity under UN Sanction":
			out.OwningEntityUNSanction = curVal
		case "Type":
			out.ShipType = curVal
			out.ShipTypeHistory = hist
		case "Date of build":
			out.DateOfBuild = curVal
		case "Gross tonnage":
			out.GrossTonnage = parseTonnage(curVal)
		case "Deadweight":
			out.Deadweight = parseTonnage(curVal)
		case "Registered owner":
			out.RegisteredOwner, out.RegisteredOwnerIMOCompanyNum = splitCompanyAndNumber(valueCell.Find("span[id$=Cur]").First())
			if out.RegisteredOwner == "" {
				out.RegisteredOwner = curVal
			}
			// The owner history table is structured differently from name/flag/
			// type (top-level entry rows interleaved with colspan company-detail
			// sub-rows), so it needs a dedicated parser rather than the generic
			// value+effective `hist` extraction.
			out.RegisteredOwnerHistory = parseOwnerHistory(valueCell)
		case "Operator":
			out.Operator, out.OperatorIMOCompanyNum = splitCompanyAndNumber(valueCell.Find("span[id$=Cur]").First())
			if out.Operator == "" {
				out.Operator = curVal
			}
		case "Ship manager":
			out.ShipManager, out.ShipManagerIMOCompanyNum = splitCompanyAndNumber(valueCell.Find("span[id$=Cur]").First())
			if out.ShipManager == "" {
				out.ShipManager = curVal
			}
		case "DOC company":
			out.DOCCompany = curVal
		case "Classification society":
			out.ClassificationSociety = curVal
		case "Status":
			out.Status = curVal
		}
	})

	if out.IMONumber == "" {
		out.IMONumber = imo
	}

	return out, nil
}

// isLoginWall detects the WebLogin.aspx response. Reliable signals: the form
// posts to WebLogin.aspx and exposes ctl00$cpMain$txtUsername. The Object Moved
// 302 redirect HTML (very short body) is also a login-wall signal.
func isLoginWall(doc *goquery.Document) bool {
	bodyText := doc.Find("body").Text()
	if strings.Contains(bodyText, "Object moved") {
		return true
	}
	formActionMatches := false
	doc.Find("form#aspnetForm").Each(func(_ int, s *goquery.Selection) {
		if action, ok := s.Attr("action"); ok && strings.Contains(action, "WebLogin.aspx") {
			formActionMatches = true
		}
	})
	if formActionMatches {
		return true
	}
	if doc.Find(`input[name="ctl00$cpMain$txtUsername"]`).Length() > 0 {
		return true
	}
	return false
}

// isNotFound matches GISIS's "no record" page shape for an unknown IMO. The
// concrete signal is TBD until we see a real not-found page; current
// implementation treats absence of #bodyheading h1 plus presence of typical
// page chrome as the not-found signal. Refine after observing a real example.
func isNotFound(doc *goquery.Document) bool {
	// Logged-in page chrome present (so it's not the login wall) but no ship
	// heading — likely the not-found state. Conservative: only flag when the
	// breadcrumb says "Ship details" but the H1 is missing/empty.
	if doc.Find("#bodyheading h1").Length() == 0 && doc.Find("#gisis-trailbar").Length() > 0 {
		breadcrumb := strings.ToLower(doc.Find("#ctl00_SiteMapPath").Text())
		if strings.Contains(breadcrumb, "ship details") {
			return true
		}
	}
	return false
}

var whitespaceRE = regexp.MustCompile(`\s+`)

func cleanWhitespace(s string) string {
	return strings.TrimSpace(whitespaceRE.ReplaceAllString(s, " "))
}

// extractEffectiveDate pulls the date from history-table notations like
// "(effective 2025-12)" or "(effective 2025-12-15)".
var effectiveDateRE = regexp.MustCompile(`effective\s+([0-9]{4}(?:-[0-9]{2}){0,2})`)

func extractEffectiveDate(s string) string {
	m := effectiveDateRE.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// parseTonnage converts GISIS's comma-separated tonnage strings ("23,232") to int.
// Returns 0 on parse failure (the omitempty JSON tag hides the zero).
func parseTonnage(s string) int {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// splitCompanyAndNumber extracts company name + IMO company number from the
// owner/operator/manager cells. GISIS renders these as
//
//	"KIVIK SHIPPING LTD (<a href=".../CompanyDetails.aspx?IMOCompanyNumber=6101937">6101937</a>)"
//
// Returns (name, imoCompanyNumber). If no anchor exists, name is the full
// text and number is empty.
var companyNumberRE = regexp.MustCompile(`IMOCompanyNumber=([0-9]+)`)

func splitCompanyAndNumber(span *goquery.Selection) (string, string) {
	if span.Length() == 0 {
		return "", ""
	}
	imoCompanyNum := ""
	// Cascadia (goquery's CSS engine) treats unquoted `.` in attribute values as
	// class selectors. Use a dot-free substring instead of `CompanyDetails.aspx`.
	span.Find("a[href*=CompanyDetails]").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		if href, ok := a.Attr("href"); ok {
			if m := companyNumberRE.FindStringSubmatch(href); len(m) >= 2 {
				imoCompanyNum = m[1]
				return false
			}
		}
		return true
	})
	// Name is everything before the "(<number>)" suffix. Cleanest: strip
	// the anchor and parens after extracting.
	clone := span.Clone()
	clone.Find("a").Remove()
	name := cleanWhitespace(clone.Text())
	// Trim trailing "(" left over after anchor removal.
	name = strings.TrimRight(name, "( )")
	name = cleanWhitespace(name)
	return name, imoCompanyNum
}

// parseOwnerHistory extracts owner-change entries from the GISIS registered-owner
// history table (#sRegOwnerHistory). Its row shape differs from the name/flag/
// type history tables: each ownership entry is a top-level row carrying the
// owner name plus an "(effective …)" date in a trailing cell, followed by
// indented company-detail sub-rows (IMO Company Number, Nationality, Address,
// Company status) whose first cell uses a colspan. Only the top-level entry
// rows are ownership changes — the colspan sub-rows are attributes of each
// owner. The generic value+effective extraction used for the other history
// tables would otherwise emit those field labels as bogus history entries (and
// miss the effective date, which lives in a later cell than for name/flag/type).
func parseOwnerHistory(valueCell *goquery.Selection) []shipHistoryEntry {
	var hist []shipHistoryEntry
	valueCell.Find("table.blank[id$=History] tr").Each(func(_ int, row *goquery.Selection) {
		first := row.Find("td").First()
		if first.Length() == 0 {
			return
		}
		// Indented company-detail sub-rows span the first two columns; skip them.
		if _, hasColspan := first.Attr("colspan"); hasColspan {
			return
		}
		name := cleanWhitespace(first.Text())
		if name == "" {
			return
		}
		hist = append(hist, shipHistoryEntry{
			Value:     name,
			Effective: extractEffectiveDate(row.Text()),
		})
	})
	return hist
}
