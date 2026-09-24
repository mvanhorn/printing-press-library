// Shared plumbing for the hand-written Immovlan commands: criteria flags and
// commune resolution, page fetching, store access, table/CSV rendering, the
// output sanitisers and the €/m² ranking shared by peb-trap and split-candidates.

package cli

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

const vlanPageSize = 20

// critFlags collects the search flags every live command shares.
type critFlags struct {
	cmd         *cobra.Command
	url         string
	types       string
	deal        string
	communes    string
	postcodes   string
	epc         string
	minPrice    int
	maxPrice    int
	minBedrooms int
	maxBedrooms int
	sort        string
}

func addCritFlags(cmd *cobra.Command, cf *critFlags, withSort bool) {
	cf.cmd = cmd
	f := cmd.Flags()
	f.StringVar(&cf.url, "url", "", "Start from an immovlan.be search URL (explicit flags override it)")
	f.StringVar(&cf.types, "type", "", "Property type(s), comma-separated: maison, appartement, terrain, garage, kot (house/apartment accepted)")
	f.StringVar(&cf.deal, "deal", "", "sale, rent, public-sale (notary sales) or colocation")
	f.StringVar(&cf.communes, "commune", "", "Commune name(s), comma-separated (Brussels communes resolve offline; others from stored localities or as <postcode>-<slug>)")
	f.StringVar(&cf.postcodes, "postcode", "", "Postal code(s), comma-separated (e.g. 1030,1210)")
	f.StringVar(&cf.epc, "epc", "", "EPC/PEB letters or Immovlan bands, comma-separated (F,G or bad,poor)")
	f.IntVar(&cf.minPrice, "min-price", 0, "Minimum price in EUR (monthly rent for rent)")
	f.IntVar(&cf.maxPrice, "max-price", 0, "Maximum price in EUR (monthly rent for rent)")
	f.IntVar(&cf.minBedrooms, "min-bedrooms", 0, "Minimum bedrooms")
	f.IntVar(&cf.maxBedrooms, "max-bedrooms", 0, "Maximum bedrooms")
	if withSort {
		f.StringVar(&cf.sort, "sort", "newest", "newest, price, price_desc, bedrooms, surface or postcode")
	}
}

func (cf *critFlags) changed(name string) bool {
	return cf.cmd != nil && cf.cmd.Flags().Changed(name)
}

// build turns the flags into criteria. Commune names that are not Brussels
// communes need the store (for localities seen before) or a <postcode>-<slug>.
func (cf *critFlags) build(ctx context.Context, db *store.Store) (immovlan.Criteria, error) {
	var c immovlan.Criteria
	var err error
	if cf.url != "" {
		if c, err = immovlan.ParseSearchURL(cf.url); err != nil {
			return c, err
		}
	}
	if cf.types != "" {
		c.Types = nil
		for _, t := range immovlan.SplitCSV(cf.types) {
			v, err := immovlan.NormalizeType(t)
			if err != nil {
				return c, err
			}
			c.Types = appendUniq(c.Types, v)
		}
	}
	if cf.deal != "" {
		if c.Deal, err = immovlan.NormalizeDeal(cf.deal); err != nil {
			return c, err
		}
	}
	if cf.postcodes != "" || cf.communes != "" {
		c.PostalCodes, c.Towns = nil, nil
	}
	pcs, err := parsePostcodes(cf.postcodes)
	if err != nil {
		return c, err
	}
	for _, pc := range pcs {
		c.PostalCodes = appendUniq(c.PostalCodes, pc)
	}
	for _, name := range immovlan.SplitCSV(cf.communes) {
		pcs, town, err := resolveCommune(ctx, db, name)
		if err != nil {
			return c, err
		}
		if town != "" {
			c.Towns = appendUniq(c.Towns, town)
		}
		for _, pc := range pcs {
			c.PostalCodes = appendUniq(c.PostalCodes, pc)
		}
	}
	if cf.epc != "" {
		c.EPC = nil
		for _, e := range immovlan.SplitCSV(cf.epc) {
			b, err := immovlan.EPCBand(e)
			if err != nil {
				return c, err
			}
			c.EPC = appendUniq(c.EPC, b)
		}
	}
	if cf.changed("min-price") {
		c.MinPrice = cf.minPrice
	}
	if cf.changed("max-price") {
		c.MaxPrice = cf.maxPrice
	}
	if cf.changed("min-bedrooms") {
		c.MinBedrooms = cf.minBedrooms
	}
	if cf.changed("max-bedrooms") {
		c.MaxBedrooms = cf.maxBedrooms
	}
	if cf.sort != "" && (cf.changed("sort") || c.SortBy == "") {
		if c.SortBy, c.SortDir, err = immovlan.NormalizeSort(cf.sort); err != nil {
			return c, err
		}
	}
	return c, nil
}

// resolveCommune turns one --commune value into postcodes (bare postcode,
// Brussels commune name, locality seen in the store) or a <postcode>-<slug>
// town value the site understands. The one resolver for find, saved, dump.
func resolveCommune(ctx context.Context, db *store.Store, name string) (postcodes []string, town string, err error) {
	if pc := immovlan.NormalizePostalCode(name); pc != "" {
		return []string{pc}, "", nil
	}
	if codes := immovlan.BrusselsPostcodes(name); codes != nil {
		return codes, "", nil
	}
	if parts := strings.SplitN(name, "-", 2); len(parts) == 2 && immovlan.NormalizePostalCode(parts[0]) != "" {
		return nil, strings.ToLower(name), nil
	}
	if db != nil {
		if pcs, err := db.LocalPostcodesFor(ctx, name); err == nil && len(pcs) > 0 {
			return pcs, "", nil
		}
	}
	return nil, "", usageErr(fmt.Errorf("cannot resolve commune %q: use --postcode, a Brussels commune name, or <postcode>-<slug> (e.g. 4000-liege)", name))
}

// parsePostcodes validates a comma-separated --postcode value.
func parsePostcodes(csv string) ([]string, error) {
	var out []string
	for _, p := range immovlan.SplitCSV(csv) {
		pc := immovlan.NormalizePostalCode(p)
		if pc == "" {
			return nil, usageErr(fmt.Errorf("--postcode %q is not a 4-digit Belgian postal code", p))
		}
		out = appendUniq(out, pc)
	}
	return out, nil
}

// parseDealFlag normalises an optional --deal value ("" stays "").
func parseDealFlag(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	d, err := immovlan.NormalizeDeal(s)
	if err != nil {
		return "", usageErr(err)
	}
	return d, nil
}

// parseEPCLetters turns "F,G", "A+" or "bad,poor" into the set of PEB letters
// to keep: a letter (A-G, A+, A++) means that letter only, a band expands to
// its letters.
func parseEPCLetters(csv string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, e := range immovlan.SplitCSV(csv) {
		if up := strings.ToUpper(e); epcLetterFlag.MatchString(up) {
			out[up] = true
			continue
		}
		if len(e) == 1 {
			return nil, usageErr(fmt.Errorf("unknown PEB letter %q (A-G, A+, A++ or a band: bad, poor, good, excellent)", e))
		}
		b, err := immovlan.EPCBand(e)
		if err != nil {
			return nil, usageErr(err)
		}
		for _, l := range immovlan.BandLetters(b) {
			out[l] = true
		}
	}
	return out, nil
}

var epcLetterFlag = regexp.MustCompile(`^[A-G](\+\+?)?$`)

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// sinceCutoff turns a --since window (30d, 72h) into the RFC3339 instant rows
// must be at or after; "" when the flag is empty.
func sinceCutoff(flag string) (string, error) {
	if flag == "" {
		return "", nil
	}
	d, err := parseWindow(flag)
	if err != nil {
		return "", usageErr(fmt.Errorf("--since: %w", err))
	}
	return time.Now().Add(-d).UTC().Format(time.RFC3339), nil
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefI(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// epcLabel is the letter when known, else the search-card band.
func epcLabel(l immovlan.Listing) string {
	if l.EPC != "" {
		return l.EPC
	}
	return l.EPCBand
}

// isNotFound reports a cliError carrying the not-found exit code.
func isNotFound(err error) bool {
	var ce *cliError
	return errors.As(err, &ce) && ce.code == 3
}

// noStoreNote is the one wording every offline command uses when data.db
// does not exist yet.
func noStoreNote(path, hint string) string {
	return fmt.Sprintf("no local store at %s; run %s first", path, hint)
}

func appendUniq(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// fetchHTML GETs a page through the generated client (Chrome UA, rate
// limit, retries) and classifies bot-protection answers.
func fetchHTML(ctx context.Context, c *client.Client, path string, params map[string]string) ([]byte, error) {
	data, err := c.GetWithHeadersNoCache(ctx, path, params, map[string]string{client.HTMLResponseHeader: "true"})
	if err != nil {
		var apiE *client.APIError
		if errors.As(err, &apiE) {
			switch apiE.StatusCode {
			case 403, 429:
				return nil, apiErr(fmt.Errorf("immovlan.be refused the request (HTTP %d): the site blocks automated browsers; keep the default User-Agent and retry from a normal network", apiE.StatusCode))
			case 404, 410:
				return nil, notFoundErr(fmt.Errorf("not found on immovlan.be (HTTP %d)", apiE.StatusCode))
			}
			// Never echo a CDN/WAF body to the terminal: the status is the message.
			return nil, apiErr(fmt.Errorf("immovlan.be returned HTTP %d for %s; retry later or check immovlan-pp-cli doctor", apiE.StatusCode, path))
		}
		return nil, err
	}
	body := []byte(data)
	low := strings.ToLower(string(body[:min(len(body), 4000)]))
	if strings.Contains(low, "<title>just a moment") || strings.Contains(low, "verifying you are human") {
		return nil, apiErr(fmt.Errorf("immovlan.be answered with a bot challenge instead of the page; retry later or from another network"))
	}
	return body, nil
}

// fetchSearchPage returns one parsed result page.
func fetchSearchPage(ctx context.Context, c *client.Client, crit immovlan.Criteria, page int) (immovlan.SearchPage, error) {
	params := crit.Params()
	if page > 1 {
		params["page"] = strconv.Itoa(page)
	}
	body, err := fetchHTML(ctx, c, "/fr/immobilier", params)
	if err != nil {
		return immovlan.SearchPage{}, err
	}
	sp, err := immovlan.ParseSearch(body)
	if err != nil {
		return sp, apiErr(fmt.Errorf("parsing search page: %w", err))
	}
	// The requested page is authoritative: the active pagination link may
	// omit page=, in which case the parser cannot tell this is the last page.
	if sp.Page != page {
		sp.Page = page
		sp.HasNext = sp.LastPage > page
	}
	return sp, nil
}

// fetchDetail returns one parsed listing page by reference.
func fetchDetail(ctx context.Context, c *client.Client, ref string) (immovlan.Detail, error) {
	body, err := fetchHTML(ctx, c, "/fr/detail/"+strings.ToLower(ref), nil)
	if err != nil {
		return immovlan.Detail{}, err
	}
	d, err := immovlan.ParseDetail(body)
	if errors.Is(err, immovlan.ErrNoListing) {
		return d, notFoundErr(fmt.Errorf("listing %s not found on Immovlan (withdrawn or wrong reference)", ref))
	}
	if err != nil {
		return d, apiErr(fmt.Errorf("parsing listing %s: %w", ref, err))
	}
	// The page's own reference must be the one we asked for: it becomes the
	// store key, a request path and (photos) a directory name.
	if !strings.EqualFold(d.ID, ref) {
		return d, apiErr(fmt.Errorf("listing %s: page reports reference %q instead", ref, d.ID))
	}
	return d, nil
}

// harvestCtx gives search-heavy commands a 5-minute budget unless --timeout
// was set explicitly.
func harvestCtx(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	if flags.timeoutExplicit {
		return boundCtx(cmd.Context(), flags)
	}
	return context.WithTimeout(cmd.Context(), 5*time.Minute)
}

// openVlanStore opens (creating when needed) the local store with the
// Immovlan tables ready.
func openVlanStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("immovlan-pp-cli")
	}
	if strings.ContainsAny(dbPath, "?#") {
		return nil, usageErr(fmt.Errorf("--db must be a plain file path"))
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store %s: %w", dbPath, err)
	}
	if err := db.EnsureVlanSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// localStoreExists reports whether the store file exists (offline commands).
func localStoreExists(dbPath string) (string, bool) {
	if dbPath == "" {
		dbPath = defaultDBPath("immovlan-pp-cli")
	}
	_, err := os.Stat(dbPath)
	return dbPath, err == nil
}

func rejectDataSource(flags *rootFlags, strategy string) error {
	switch {
	case strategy == "live" && flags.dataSource == "local":
		return fmt.Errorf("this command has no local data source; drop --data-source local")
	case strategy == "local" && flags.dataSource == "live":
		return fmt.Errorf("this command has no live equivalent; drop --data-source live")
	}
	return nil
}

// dogfoodPages caps page walks under the dogfood harness.
func dogfoodPages(n int) int {
	if cliutil.IsAnyHarness() && n > 1 {
		return 1
	}
	return n
}

func fmtEUR(v float64) string {
	n := int64(v + 0.5)
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String() + " €"
}

func fmtPtrEUR(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmtEUR(*p)
}

func intStr(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p)
}

func floatUnit(p *float64, unit string) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64) + unit
}

func locLabel(l immovlan.Listing) string {
	return termSafe(strings.TrimSpace(l.PostalCode + " " + l.Locality))
}

func sellerLabel(l immovlan.Listing) string {
	if l.Private {
		return "private"
	}
	if l.Agency != "" {
		return termSafe(truncate(l.Agency, 24))
	}
	if l.AgencyID != "" {
		return "agency " + termSafe(l.AgencyID)
	}
	return "agency"
}

// vlanCSVHeader and vlanCSVRow are the one CSV shape of this CLI (find --csv
// and dump --format csv); the columns mirror immoweb-pp-cli dump so both
// portals load into the same spreadsheet.
var vlanCSVHeader = []string{"id", "url", "deal", "type", "subtype", "title", "postal_code", "locality", "street", "lat", "lng", "price", "bedrooms", "surface_m2", "land_m2", "price_per_m2", "epc", "condition", "rented", "cadastral_income", "construction_year", "agency", "private_seller", "software", "created_at", "first_seen", "last_seen", "gone_at"}

func vlanCSVRow(r store.StoredListing) []string {
	fs := func(p *float64) string {
		if p == nil {
			return ""
		}
		return strconv.FormatFloat(*p, 'f', -1, 64)
	}
	bs := func(p *bool) string {
		if p == nil {
			return ""
		}
		return strconv.FormatBool(*p)
	}
	is := func(p *int) string {
		if p == nil {
			return ""
		}
		return strconv.Itoa(*p)
	}
	return []string{csvSafe(r.ID), csvSafe(r.URL), r.Deal, r.Type, csvSafe(r.Subtype), csvSafe(r.Title), csvSafe(r.PostalCode), csvSafe(r.Locality), csvSafe(r.Street),
		fs(r.Lat), fs(r.Lng), fs(r.Price), is(r.Bedrooms), fs(r.Surface), fs(r.Land), fs(r.PricePerSqm), csvSafe(r.EPC), csvSafe(r.Condition), bs(r.Rented), fs(r.CadastralIncome), is(r.Year),
		csvSafe(r.Agency), strconv.FormatBool(r.Private), csvSafe(r.Software), csvSafe(r.CreatedAt), csvSafe(r.FirstSeen), csvSafe(r.LastSeen), csvSafe(r.GoneAt)}
}

func writeVlanCSV(w io.Writer, rows []store.StoredListing) error {
	cw := csv.NewWriter(w)
	_ = cw.Write(vlanCSVHeader)
	for _, r := range rows {
		_ = cw.Write(vlanCSVRow(r))
	}
	cw.Flush()
	return cw.Error()
}

// printView is printJSONFiltered for hand-written views. Titles, streets and
// agency names are advertiser-written, so the terminal-oriented formats
// (--plain, --quiet) strip control characters and --csv additionally
// neutralises formula prefixes; JSON stays verbatim (it is escaped).
func printView(w io.Writer, v any, flags *rootFlags) error {
	if flags == nil || !(flags.csv || flags.plain || flags.quiet) {
		return printJSONFiltered(w, v, flags)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	generic = sanitizeStrings(generic, termSafe)
	if flags.csv {
		generic = sanitizeStrings(generic, csvSafe)
	}
	return printJSONFiltered(w, generic, flags)
}

// sanitizeStrings applies fn to every string in a decoded JSON value.
func sanitizeStrings(v any, fn func(string) string) any {
	switch t := v.(type) {
	case string:
		return fn(t)
	case []any:
		for i := range t {
			t[i] = sanitizeStrings(t[i], fn)
		}
		return t
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fn(k)] = sanitizeStrings(val, fn) // keys too: Detail.Features is keyed by page labels
		}
		return out
	}
	return v
}

// rankedRow is a stored listing with its €/m² percentile inside its postcode
// and its age, shared by peb-trap and split-candidates.
type rankedRow struct {
	store.StoredListing
	PercentileInPostcode *float64 `json:"percentile_in_postcode,omitempty"`
	DaysListed           *int     `json:"days_listed,omitempty"`
}

// minPercentilePool is the smallest postcode pool a percentile is computed on.
const minPercentilePool = 5

func ppsPoolByPostcode(rows []store.StoredListing) map[string][]float64 {
	pool := map[string][]float64{}
	for _, r := range rows {
		if r.PricePerSqm != nil {
			pool[r.PostalCode] = append(pool[r.PostalCode], *r.PricePerSqm)
		}
	}
	return pool
}

func rankRow(r store.StoredListing, pool map[string][]float64, now time.Time) rankedRow {
	row := rankedRow{StoredListing: r}
	if r.PricePerSqm != nil && len(pool[r.PostalCode]) >= minPercentilePool {
		p := percentileRank(pool[r.PostalCode], *r.PricePerSqm)
		row.PercentileInPostcode = &p
	}
	if d, ok := immovlan.DaysListed(r.CreatedAt, now); ok {
		row.DaysListed = &d
	}
	return row
}

func pctStr(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f", *p)
}

func byPricePerSqm(a, b rankedRow) bool {
	if a.PricePerSqm == nil || b.PricePerSqm == nil {
		return a.PricePerSqm != nil
	}
	return *a.PricePerSqm < *b.PricePerSqm
}

func printListingTable(w io.Writer, listings []immovlan.Listing) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "ID\tTYPE\tPRICE\tBEDS\tM²\t€/M²\tPEB\tLOCALITY\tSTREET\tFLAG\tSELLER")
	intOf := func(p *float64) string {
		if p == nil {
			return ""
		}
		return strconv.Itoa(int(*p))
	}
	for _, l := range listings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", l.ID, l.Type, fmtPtrEUR(l.Price), intStr(l.Bedrooms), intOf(l.Surface), intOf(l.PricePerSqm), termSafe(epcLabel(l)), locLabel(l), termSafe(truncate(l.Street, 28)), l.Flag, sellerLabel(l))
	}
	return tw.Flush()
}

// termSafe strips control and format characters (ESC sequences, bidi
// overrides, zero-width joiners) from advertiser-written text.
func termSafe(s string) string {
	unsafe := func(r rune) bool { return r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) }
	if strings.IndexFunc(s, unsafe) < 0 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if unsafe(r) {
			return -1
		}
		return r
	}, s)
}

// csvSafe neutralises spreadsheet formula prefixes (OWASP list).
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// pct1 rounds a ratio to a percentage with one decimal, symmetric around zero.
func pct1(ratio float64) float64 { return math.Round(ratio*1000) / 10 }

// parseWindow accepts 72h, 7d, 2w.
func parseWindow(s string) (time.Duration, error) {
	return cliutil.ParseDurationLoose(s)
}

// median of a float slice (0, false when empty).
func median(v []float64) (float64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2], true
	}
	return (s[n/2-1] + s[n/2]) / 2, true
}

// percentileRank returns the share of pool values at or below v (0-100).
func percentileRank(pool []float64, v float64) float64 {
	if len(pool) == 0 {
		return 0
	}
	n := 0
	for _, p := range pool {
		if p <= v {
			n++
		}
	}
	return pct1(float64(n) / float64(len(pool)))
}
