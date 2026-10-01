// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

// Audit queries over the mirror: payee spelling-variant clusters, rule health,
// schedule health, and category suggestions learned from history.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Payee clustering

var (
	payeeStarSuffix = regexp.MustCompile(`\*.*$`)
	payeeStoreNum   = regexp.MustCompile(`#\s*\d+`)
	payeeNonAlpha   = regexp.MustCompile(`[^\p{L}]+`)
)

// payeeNoise are tokens that carry no identity in bank payee strings.
var payeeNoise = map[string]bool{
	"inc": true, "llc": true, "com": true, "mktp": true, "mktplace": true, "marketplace": true,
	"pmts": true, "pmt": true, "us": true, "www": true, "co": true, "corp": true, "ltd": true,
}

// payeeProcessors are payment-processor prefixes written before a "*" in bank
// payee strings ("SQ *JOES PIZZA", "TST* BLUE BOTTLE", "PAYPAL *NETFLIX"). For
// these the merchant is the text AFTER the "*", not before it.
var payeeProcessors = map[string]bool{"sq": true, "tst": true, "paypal": true, "pp": true, "sp": true}

// NormalizePayeeName reduces a payee name to its identifying core: lowercase,
// no "*..." suffix (or, after a payment-processor prefix such as "SQ *" or
// "PAYPAL *", the merchant text after the "*"), no store numbers, digits or
// punctuation, and no noise tokens (inc, llc, com, mktp, marketplace, pmts,
// us, www, ...). "AMAZON MKTPLACE", "Amazon.com" and "Amazon" all normalize
// to "amazon"; "SQ *JOES PIZZA" normalizes to "joes pizza".
func NormalizePayeeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if before, after, ok := strings.Cut(s, "*"); ok && payeeProcessors[strings.TrimSpace(before)] {
		s = after
	}
	s = payeeStarSuffix.ReplaceAllString(s, "")
	s = payeeStoreNum.ReplaceAllString(s, " ")
	s = payeeNonAlpha.ReplaceAllString(s, " ")
	var keep []string
	for _, tok := range strings.Fields(s) {
		if payeeNoise[tok] {
			continue
		}
		keep = append(keep, tok)
	}
	return strings.Join(keep, " ")
}

// JaroWinkler returns the Jaro-Winkler similarity of a and b in [0,1].
func JaroWinkler(a, b string) float64 {
	if a == b {
		return 1
	}
	return jaroWinkler([]rune(a), []rune(b), nil)
}

// LevenshteinSimilarity returns 1 - edit distance / longer length, in [0,1].
func LevenshteinSimilarity(a, b string) float64 {
	return levenshtein([]rune(a), []rune(b), nil)
}

// simScratch holds buffers reused across jaroWinkler/levenshtein calls on
// one goroutine. A nil *simScratch allocates fresh buffers per call.
type simScratch struct {
	ma, mb    []bool
	prev, cur []int
}

func (s *simScratch) flags(la, lb int) (ma, mb []bool) {
	if s == nil {
		return make([]bool, la), make([]bool, lb)
	}
	if cap(s.ma) < la {
		s.ma = make([]bool, la)
	}
	if cap(s.mb) < lb {
		s.mb = make([]bool, lb)
	}
	ma, mb = s.ma[:la], s.mb[:lb]
	clear(ma)
	clear(mb)
	return ma, mb
}

// rows returns two int rows of length n; levenshtein overwrites every cell
// before reading it, so they are not cleared.
func (s *simScratch) rows(n int) (prev, cur []int) {
	if s == nil {
		return make([]int, n), make([]int, n)
	}
	if cap(s.prev) < n || cap(s.cur) < n {
		s.prev, s.cur = make([]int, n), make([]int, n)
	}
	return s.prev[:n], s.cur[:n]
}

// jaroWinkler is JaroWinkler on pre-converted runes, minus the equal-string
// shortcut (identical non-empty inputs still score exactly 1).
func jaroWinkler(ra, rb []rune, s *simScratch) float64 {
	la, lb := len(ra), len(rb)
	if la == 0 || lb == 0 {
		return 0
	}
	window := max(la, lb)/2 - 1
	if window < 0 {
		window = 0
	}
	ma, mb := s.flags(la, lb)
	matches := 0
	for i := 0; i < la; i++ {
		lo, hi := max(0, i-window), min(lb-1, i+window)
		for j := lo; j <= hi; j++ {
			if mb[j] || ra[i] != rb[j] {
				continue
			}
			ma[i], mb[j] = true, true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions, k := 0, 0
	for i := 0; i < la; i++ {
		if !ma[i] {
			continue
		}
		for !mb[k] {
			k++
		}
		if ra[i] != rb[k] {
			transpositions++
		}
		k++
	}
	m := float64(matches)
	jaro := (m/float64(la) + m/float64(lb) + (m-float64(transpositions)/2)/m) / 3
	prefix := 0
	for i := 0; i < min(4, min(la, lb)); i++ {
		if ra[i] != rb[i] {
			break
		}
		prefix++
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

// levenshtein is LevenshteinSimilarity on pre-converted runes.
func levenshtein(ra, rb []rune, s *simScratch) float64 {
	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	prev, cur := s.rows(len(rb) + 1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return 1 - float64(prev[len(rb)])/float64(max(len(ra), len(rb)))
}

// PayeeSimilarity scores two raw payee names on their normalized forms:
// 1 when the normalized names are equal; wordPrefixScore when one is a
// whole-word prefix (>= 4 chars) of the other and every extra word is a
// generic location/suffix token ("kroger" / "kroger fuel"); when the extra
// words carry identity ("delta" / "delta dental", "uber" / "uber eats") or the
// shorter is a prefix of the longer that ends mid-word ("apple" / "applebee s")
// only normalized Levenshtein is used, so Jaro-Winkler's shared-prefix bonus
// cannot join distinct merchants (whole-word prefixes are additionally capped
// at nonGenericPrefixCap); otherwise the larger of Jaro-Winkler and
// normalized Levenshtein. Names that normalize to nothing score 0.
func PayeeSimilarity(a, b string) float64 {
	return normalizedSimilarity(NormalizePayeeName(a), NormalizePayeeName(b))
}

// prefixKind classifies how two normalized payee names relate.
type prefixKind int

const (
	prefixNone         prefixKind = iota // not a prefix (or a short whole-word prefix): fuzzy scores
	prefixEqual                          // identical
	prefixGeneric                        // whole-word prefix of 4+ chars; extra words are generic
	prefixWordSpecific                   // whole-word prefix of 4+ chars; extra words carry identity
	prefixMidWord                        // shorter is a prefix ending inside a word of the longer
)

// genericPayeeSuffix are location/suffix words that do not change the merchant
// when appended to its name ("Kroger Fuel", "Walmart Supermarket"). Store
// numbers and digits are already stripped by NormalizePayeeName, and so are
// corporate/marketplace tokens (inc, llc, co, corp, com, mktp, ...): see
// payeeNoise, so they never need listing here.
var genericPayeeSuffix = map[string]bool{
	"fuel": true, "gas": true, "station": true, "store": true, "stores": true,
	"market": true, "supermarket": true, "online": true,
}

// prefixRelation classifies already-normalized names.
func prefixRelation(na, nb string) prefixKind {
	if na == nb {
		return prefixEqual
	}
	short, long := na, nb
	if len(short) > len(long) {
		short, long = long, short
	}
	if !strings.HasPrefix(long, short) {
		return prefixNone
	}
	if long[len(short)] != ' ' {
		return prefixMidWord
	}
	// Whole-word prefix: short tokens fall back to fuzzy scores.
	if len(short) < 4 {
		return prefixNone
	}
	for w := range strings.FieldsSeq(long[len(short):]) {
		if !genericPayeeSuffix[w] {
			return prefixWordSpecific
		}
	}
	return prefixGeneric
}

// wordPrefixScore is the similarity of a generic whole-word prefix match
// ("kroger" vs "kroger fuel"): likely the same merchant, but ranked below
// names that are identical after normalization.
const wordPrefixScore = 0.9

// nonGenericPrefixCap caps the score of a whole-word prefix whose extra words
// carry identity ("delta" vs "delta dental") below the default 0.85
// clustering threshold.
const nonGenericPrefixCap = 0.84

// directionPairs are words that flip the meaning of an otherwise identical
// payee ("transfer to savings" vs "transfer from savings").
var directionPairs = [][2]string{{"to", "from"}, {"in", "out"}, {"credit", "debit"}, {"deposit", "withdrawal"}, {"sent", "received"}}

// directionBit maps each direction word to its bit: pair i's first word is
// bit 2i and its second word bit 2i+1. directionFirstBits has every 2i bit.
var directionBit, directionFirstBits = func() (map[string]uint16, uint16) {
	m := map[string]uint16{}
	var first uint16
	for i, p := range directionPairs {
		m[p[0]], m[p[1]] = 1<<(2*i), 1<<(2*i+1)
		first |= 1 << (2 * i)
	}
	return m, first
}()

// directionMask returns the set of direction words in a normalized name.
func directionMask(norm string) uint16 {
	var m uint16
	for w := range strings.FieldsSeq(norm) {
		m |= directionBit[w]
	}
	return m
}

// opposingDirections reports whether, for some direction pair (x, y), one
// name has x but not y and the other has y but not x. Such payees must
// never be merged. Arguments are directionMask values.
func opposingDirections(a, b uint16) bool {
	onlyFirst := func(m uint16) uint16 { return m &^ (m >> 1) & directionFirstBits }
	onlySecond := func(m uint16) uint16 { return (m >> 1) &^ m & directionFirstBits }
	return onlyFirst(a)&onlySecond(b) != 0 || onlySecond(a)&onlyFirst(b) != 0
}

// payeeKey is a normalized payee name with everything pairScore needs
// precomputed once.
type payeeKey struct {
	norm  string
	runes []rune
	dir   uint16 // directionMask(norm)
}

func newPayeeKey(norm string) payeeKey {
	return payeeKey{norm: norm, runes: []rune(norm), dir: directionMask(norm)}
}

func normalizedSimilarity(na, nb string) float64 {
	a, b := newPayeeKey(na), newPayeeKey(nb)
	return pairScore(&a, &b, math.Inf(-1), nil)
}

// boundSlack widens the pruning bounds so float rounding can never prune a
// pair whose exact score reaches minSim.
const boundSlack = 1e-9

// pairScore is the single payee scoring policy described on PayeeSimilarity.
// It may stop early: the result is exact whenever it is >= minSim, and is
// otherwise only guaranteed to be below minSim. minSim = -Inf scores exactly.
//
// Pruning bounds, with r = shorter/longer rune length: normalized
// Levenshtein <= r (the edit distance is at least the length difference) and
// Jaro <= (2+r)/3, so Jaro-Winkler (<= 0.6*Jaro + 0.4 with a 4-char prefix)
// <= 0.8 + 0.2r. A pair whose bound for its prefix kind is below minSim is
// skipped, and Levenshtein only runs when it could raise the score. The
// direction check runs last, only for a pair that would otherwise match.
func pairScore(a, b *payeeKey, minSim float64, s *simScratch) float64 {
	if a.norm == "" || b.norm == "" {
		return 0
	}
	kind := prefixRelation(a.norm, b.norm)
	if kind == prefixEqual {
		return 1 // identical names cannot carry opposing directions
	}
	la, lb := len(a.runes), len(b.runes)
	r := float64(min(la, lb)) / float64(max(la, lb))
	var score float64
	switch kind {
	case prefixGeneric:
		score = wordPrefixScore
	case prefixWordSpecific:
		if math.Min(r, nonGenericPrefixCap)+boundSlack < minSim {
			return 0
		}
		score = math.Min(levenshtein(a.runes, b.runes, s), nonGenericPrefixCap)
	case prefixMidWord:
		if r+boundSlack < minSim {
			return 0
		}
		score = levenshtein(a.runes, b.runes, s)
	default:
		if 0.8+0.2*r+boundSlack < minSim {
			return 0
		}
		score = jaroWinkler(a.runes, b.runes, s)
		if r+boundSlack > score && r+boundSlack >= minSim {
			score = math.Max(score, levenshtein(a.runes, b.runes, s))
		}
	}
	if score < minSim {
		return score
	}
	if opposingDirections(a.dir, b.dir) {
		return 0
	}
	return score
}

// PayeeCluster is a group of payees that look like spelling variants. Target
// is the most-used member; Merge are the others.
type PayeeCluster struct {
	Target     Payee
	Merge      []Payee
	Similarity float64 // lowest target-to-member similarity in the cluster
}

// ClusterPayees is ClusterPayeesContext without cancellation.
func ClusterPayees(payees []Payee, minSim float64) []PayeeCluster {
	out, _ := ClusterPayeesContext(context.Background(), payees, minSim)
	return out
}

// ClusterPayeesContext groups payees whose names collide at minSim or above.
// Transfer payees and payees whose names normalize to nothing are ignored.
// Candidates are walked most used first (then by name, id); each payee not
// yet in a cluster becomes a target and takes every still-unclustered payee
// that matches it directly, so a merge never chains A~B~C where A and C fail
// the pairwise checks. Clusters are ordered by total usage, most used first.
// Each name is normalized once; ctx is checked once per target.
func ClusterPayeesContext(ctx context.Context, payees []Payee, minSim float64) ([]PayeeCluster, error) {
	type candidate struct {
		p   Payee
		key payeeKey
	}
	var cand []candidate
	for _, p := range payees {
		if p.TransferAcct != "" {
			continue
		}
		n := NormalizePayeeName(p.Name)
		if n == "" {
			continue
		}
		cand = append(cand, candidate{p: p, key: newPayeeKey(n)})
	}
	sort.SliceStable(cand, func(x, y int) bool {
		a, b := cand[x].p, cand[y].p
		if a.TxnCount != b.TxnCount {
			return a.TxnCount > b.TxnCount
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	assigned := make([]bool, len(cand))
	var scratch simScratch
	out := make([]PayeeCluster, 0)
	for i := range cand {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if assigned[i] {
			continue
		}
		t := &cand[i]
		c := PayeeCluster{Target: t.p, Similarity: 1}
		for j := i + 1; j < len(cand); j++ {
			if assigned[j] {
				continue
			}
			s := pairScore(&t.key, &cand[j].key, minSim, &scratch)
			if !(s >= minSim) { // also rejects a NaN minSim
				continue
			}
			assigned[j] = true
			c.Merge = append(c.Merge, cand[j].p)
			c.Similarity = math.Min(c.Similarity, s)
		}
		if len(c.Merge) > 0 {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := clusterUse(out[i]), clusterUse(out[j])
		if ti != tj {
			return ti > tj
		}
		return out[i].Target.Name < out[j].Target.Name
	})
	return out, nil
}

func clusterUse(c PayeeCluster) int {
	n := c.Target.TxnCount
	for _, m := range c.Merge {
		n += m.TxnCount
	}
	return n
}

// ---------------------------------------------------------------------------
// Rules

// RuleConditionOptions are the per-condition options Actual stores. For
// amount conditions, Inflow ("amount (inflow)") matches only amounts >= 0
// and Outflow ("amount (outflow)") matches only amounts <= 0, comparing the
// outflow's magnitude (the value is stored positive).
type RuleConditionOptions struct {
	Inflow  bool `json:"inflow,omitempty"`
	Outflow bool `json:"outflow,omitempty"`
}

// RuleCondition is one condition of an Actual rule.
type RuleCondition struct {
	Op      string               `json:"op"`
	Field   string               `json:"field"`
	Value   json.RawMessage      `json:"value"`
	Type    string               `json:"type,omitempty"`
	Options RuleConditionOptions `json:"options,omitempty"`
}

// RuleAction is one action of an Actual rule.
type RuleAction struct {
	Op    string          `json:"op"`
	Field string          `json:"field,omitempty"`
	Value json.RawMessage `json:"value"`
	Type  string          `json:"type,omitempty"`
}

// Rule is one live rule with parsed conditions and actions.
type Rule struct {
	ID           string
	Stage        string
	ConditionsOp string
	Conditions   []RuleCondition
	Actions      []RuleAction
	ParseError   string
}

// ScheduleID returns the schedule a rule is linked to (link-schedule action).
func (r Rule) ScheduleID() string {
	for _, a := range r.Actions {
		if a.Op == "link-schedule" {
			var s string
			if json.Unmarshal(a.Value, &s) == nil {
				return s
			}
			return "?"
		}
	}
	return ""
}

// Rules returns live rules in id order.
func (l *Ledger) Rules(ctx context.Context) ([]Rule, error) {
	out := make([]Rule, 0)
	if !l.HasTable(ctx, "rules") {
		return out, nil
	}
	rows, err := l.DB.QueryContext(ctx, `SELECT id, COALESCE(stage, ''), COALESCE(conditions_op, 'and'), COALESCE(conditions, '[]'), COALESCE(actions, '[]')
		FROM rules WHERE COALESCE(tombstone, 0) = 0 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("querying rules: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r Rule
		var conds, acts string
		if err := rows.Scan(&r.ID, &r.Stage, &r.ConditionsOp, &conds, &acts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(conds), &r.Conditions); err != nil {
			r.ParseError = "conditions: " + err.Error()
		}
		if err := json.Unmarshal([]byte(acts), &r.Actions); err != nil {
			r.ParseError = strings.TrimPrefix(r.ParseError+"; actions: "+err.Error(), "; ")
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Indexes of the string fields rule conditions can test (see txnStrings).
const (
	strPayee = iota
	strImported
	strNotes
	strAccount
	strCategory
	numStrFields
)

// ruleStrField maps a string rule field to its txnStrings index, or -1.
func ruleStrField(field string) int {
	switch field {
	case "payee", "description":
		return strPayee
	case "imported_payee", "imported_description":
		return strImported
	case "notes":
		return strNotes
	case "account", "acct":
		return strAccount
	case "category":
		return strCategory
	}
	return -1
}

func txnStrings(t *Txn) [numStrFields]string {
	return [numStrFields]string{t.PayeeID, t.ImportedDescription, t.Notes, t.AccountID, t.CategoryID}
}

// txnField returns a transaction's value for a rule field; ok is false for
// fields this evaluator does not model.
func txnField(t Txn, field string) (any, bool) {
	if field == "amount" {
		return t.Amount, true
	}
	if i := ruleStrField(field); i >= 0 {
		return txnStrings(&t)[i], true
	}
	return nil, false
}

func isIDField(field string) bool {
	switch field {
	case "payee", "description", "account", "acct", "category":
		return true
	}
	return false
}

// txnView is a transaction with its rule-testable string fields extracted
// and lowercased once, so many conditions can test it without re-lowering.
type txnView struct {
	t     *Txn
	raw   [numStrFields]string
	lower [numStrFields]string
}

func newTxnView(t *Txn) txnView {
	v := txnView{t: t, raw: txnStrings(t)}
	for i, s := range v.raw {
		v.lower[i] = strings.ToLower(s)
	}
	return v
}

// compiledCondition is a RuleCondition with its value decoded (and regexp
// compiled) once, so it can be evaluated against many transactions cheaply.
type compiledCondition struct {
	c         RuleCondition
	supported bool
	isAmount  bool
	idField   bool
	field     int            // txnStrings index for string fields
	str       string         // is/isNot/contains/doesNotContain (lowercased unless idField)
	list      []string       // oneOf/notOneOf (lowercased unless idField)
	re        *regexp.Regexp // matches
	num       float64        // amount comparisons
	lo, hi    float64        // amount isbetween
}

func compileCondition(c RuleCondition) compiledCondition {
	cc := compiledCondition{c: c}
	if _, ok := txnField(Txn{}, c.Field); !ok {
		return cc
	}
	if c.Field == "amount" {
		cc.isAmount = true
		if c.Op == "isbetween" {
			var r struct {
				Num1 *float64 `json:"num1"`
				Num2 *float64 `json:"num2"`
			}
			if json.Unmarshal(c.Value, &r) != nil || r.Num1 == nil || r.Num2 == nil {
				return cc
			}
			cc.lo, cc.hi = math.Min(*r.Num1, *r.Num2), math.Max(*r.Num1, *r.Num2)
			cc.supported = true
			return cc
		}
		switch c.Op {
		case "is", "isNot", "isapprox", "gt", "gte", "lt", "lte":
		default:
			return cc
		}
		if json.Unmarshal(c.Value, &cc.num) != nil {
			return cc
		}
		cc.supported = true
		return cc
	}
	cc.field = ruleStrField(c.Field)
	cc.idField = isIDField(c.Field)
	switch c.Op {
	case "is", "isNot", "contains", "doesNotContain", "matches":
		var want string
		if err := json.Unmarshal(c.Value, &want); err != nil {
			return cc
		}
		switch c.Op {
		case "contains", "doesNotContain":
			cc.str = strings.ToLower(want)
		case "matches":
			re, err := regexp.Compile("(?i)" + want)
			if err != nil {
				return cc
			}
			cc.re = re
		default:
			cc.str = cc.normVal(want)
		}
	case "oneOf", "notOneOf":
		var list []string
		if err := json.Unmarshal(c.Value, &list); err != nil {
			return cc
		}
		cc.list = make([]string, len(list))
		for i, v := range list {
			cc.list[i] = cc.normVal(v)
		}
	default:
		return cc
	}
	cc.supported = true
	return cc
}

func (cc *compiledCondition) eval(t Txn) (matched, supported bool) {
	v := newTxnView(&t)
	return cc.evalView(&v)
}

// evalView evaluates the condition against a prepared transaction view.
// contains/doesNotContain always compare lowercased text (ids included);
// is/isNot/oneOf/notOneOf compare ids verbatim and other text lowercased.
func (cc *compiledCondition) evalView(v *txnView) (matched, supported bool) {
	if !cc.supported {
		return false, false
	}
	if cc.isAmount {
		return cc.evalAmount(v.t.Amount), true
	}
	s, lower := v.raw[cc.field], v.lower[cc.field]
	norm := lower
	if cc.idField {
		norm = s
	}
	switch cc.c.Op {
	case "is":
		return norm == cc.str, true
	case "isNot":
		return norm != cc.str, true
	case "contains":
		return strings.Contains(lower, cc.str), true
	case "doesNotContain":
		return !strings.Contains(lower, cc.str), true
	case "matches":
		return cc.re.MatchString(s), true
	case "oneOf", "notOneOf":
		in := slices.Contains(cc.list, norm)
		return in == (cc.c.Op == "oneOf"), true
	}
	return false, false
}

func (cc *compiledCondition) normVal(v string) string {
	if cc.idField {
		return v
	}
	return strings.ToLower(v)
}

// evalAmount applies an amount condition. With options.inflow only
// non-negative amounts can match; with options.outflow only non-positive
// amounts can match and their magnitude (-amount) is compared, because the
// condition's value is stored as a positive number.
func (cc *compiledCondition) evalAmount(n int64) bool {
	switch {
	case cc.c.Options.Inflow:
		if n < 0 {
			return false
		}
	case cc.c.Options.Outflow:
		if n > 0 {
			return false
		}
		n = -n
	}
	v := float64(n)
	want := cc.num
	switch cc.c.Op {
	case "isbetween":
		return v >= cc.lo && v <= cc.hi
	case "is":
		return v == want
	case "isNot":
		return v != want
	case "isapprox":
		// Actual treats "approximately" as within 7.5% of the value.
		return math.Abs(v-want) <= math.Abs(want)*0.075
	case "gt":
		return v > want
	case "gte":
		return v >= want
	case "lt":
		return v < want
	case "lte":
		return v <= want
	}
	return false
}

// EvalCondition evaluates one rule condition against a transaction. Payee,
// account and category compare against the transaction's RESOLVED ids (after
// payee/category merges), which is what Actual rewrites rules to on merge.
// String comparisons are case-insensitive like Actual's. Amount conditions
// honour options.inflow / options.outflow. supported is false when the
// op/field/value combination is not modelled.
func EvalCondition(c RuleCondition, t Txn) (matched, supported bool) {
	cc := compileCondition(c)
	return cc.eval(t)
}

// compiledRule is a Rule with every condition pre-compiled.
type compiledRule struct {
	or        bool
	conds     []compiledCondition
	supported bool
}

func compileRule(r Rule) compiledRule {
	cr := compiledRule{or: strings.EqualFold(r.ConditionsOp, "or"), supported: len(r.Conditions) > 0}
	cr.conds = make([]compiledCondition, len(r.Conditions))
	for i, c := range r.Conditions {
		cr.conds[i] = compileCondition(c)
		if !cr.conds[i].supported {
			cr.supported = false
		}
	}
	return cr
}

// matches evaluates the rule, stopping at the first condition that decides
// it (the first true one for "or", the first false one for "and").
func (cr *compiledRule) matches(v *txnView) (matched, supported bool) {
	if !cr.supported {
		return false, false
	}
	for i := range cr.conds {
		if m, _ := cr.conds[i].evalView(v); m == cr.or {
			return m, true
		}
	}
	return !cr.or, true
}

// RuleMatches evaluates all of a rule's conditions with its and/or operator.
// supported is false when any condition cannot be evaluated.
func RuleMatches(r Rule, t Txn) (matched, supported bool) {
	cr := compileRule(r)
	v := newTxnView(&t)
	return cr.matches(&v)
}

// RuleAudit is the health report for one rule.
type RuleAudit struct {
	ID         string   `json:"id"`
	Stage      string   `json:"stage"`
	Summary    string   `json:"summary"`
	ScheduleID string   `json:"schedule_id,omitempty"`
	Evaluated  bool     `json:"evaluated"`
	MatchCount int      `json:"match_count"`
	LastMatch  string   `json:"last_match,omitempty"`
	Findings   []string `json:"findings"`
	Dangling   []string `json:"dangling,omitempty"`
}

// entityRef describes a referenced id in a raw table, including tombstoned rows.
type entityRef struct {
	Name    string
	Deleted bool
}

// rawEntities loads id -> name/deleted for a table, tombstones included.
func (l *Ledger) rawEntities(ctx context.Context, table string) (map[string]entityRef, error) {
	out := map[string]entityRef{}
	if err := checkTable(table); err != nil {
		return nil, err
	}
	if !l.HasTable(ctx, table) {
		return out, nil
	}
	rows, err := l.DB.QueryContext(ctx, "SELECT id, COALESCE(name, ''), COALESCE(tombstone, 0) FROM "+table) // #nosec G202 -- table passed checkTable allowlist
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		var tomb int64
		if err := rows.Scan(&id, &name, &tomb); err != nil {
			return nil, err
		}
		out[id] = entityRef{Name: name, Deleted: tomb != 0}
	}
	return out, rows.Err()
}

type entityTables struct {
	payees, categories, accounts, schedules map[string]entityRef
}

func (l *Ledger) loadEntityTables(ctx context.Context) (*entityTables, error) {
	var e entityTables
	var err error
	if e.payees, err = l.rawEntities(ctx, "payees"); err != nil {
		return nil, err
	}
	if e.categories, err = l.rawEntities(ctx, "categories"); err != nil {
		return nil, err
	}
	if e.accounts, err = l.rawEntities(ctx, "accounts"); err != nil {
		return nil, err
	}
	if e.schedules, err = l.rawEntities(ctx, "schedules"); err != nil {
		return nil, err
	}
	return &e, nil
}

func (e *entityTables) table(field string) (map[string]entityRef, string) {
	switch field {
	case "payee", "description":
		return e.payees, "payee"
	case "category":
		return e.categories, "category"
	case "account", "acct":
		return e.accounts, "account"
	}
	return nil, ""
}

func jsonIDs(v json.RawMessage) []string {
	var s string
	if json.Unmarshal(v, &s) == nil {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	var list []string
	if json.Unmarshal(v, &list) == nil {
		return list
	}
	return nil
}

func (e *entityTables) label(field string, v json.RawMessage) string {
	tbl, _ := e.table(field)
	if tbl == nil {
		var s any
		if json.Unmarshal(v, &s) == nil {
			if f, ok := s.(float64); ok && field == "amount" {
				return FormatAmount(int64(f))
			}
			b, _ := json.Marshal(s)
			return string(b)
		}
		return string(v)
	}
	ids := jsonIDs(v)
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		ref, ok := tbl[id]
		switch {
		case !ok:
			names = append(names, strconv.Quote(id)+" (missing)")
		case ref.Deleted:
			names = append(names, strconv.Quote(ref.Name)+" (deleted)")
		default:
			names = append(names, strconv.Quote(ref.Name))
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	return "[" + strings.Join(names, ", ") + "]"
}

// dangling lists references to missing or tombstoned payees, categories,
// accounts or schedules in a rule's conditions and actions.
func (e *entityTables) dangling(r Rule) []string {
	var out []string
	check := func(field string, v json.RawMessage, where string) {
		tbl, kind := e.table(field)
		if tbl == nil {
			return
		}
		for _, id := range jsonIDs(v) {
			ref, ok := tbl[id]
			switch {
			case !ok:
				out = append(out, fmt.Sprintf("%s %s %s (missing)", where, kind, id))
			case ref.Deleted:
				out = append(out, fmt.Sprintf("%s %s %s %q (deleted)", where, kind, id, ref.Name))
			}
		}
	}
	for _, c := range r.Conditions {
		check(c.Field, c.Value, "condition")
	}
	for _, a := range r.Actions {
		if a.Op == "link-schedule" {
			for _, id := range jsonIDs(a.Value) {
				if ref, ok := e.schedules[id]; !ok || ref.Deleted {
					out = append(out, fmt.Sprintf("action schedule %s (missing or deleted)", id))
				}
			}
			continue
		}
		check(a.Field, a.Value, "action")
	}
	return out
}

func (e *entityTables) summary(r Rule) string {
	join := " and "
	if strings.EqualFold(r.ConditionsOp, "or") {
		join = " or "
	}
	conds := make([]string, 0, len(r.Conditions))
	for _, c := range r.Conditions {
		conds = append(conds, fmt.Sprintf("%s %s %s", c.Field, c.Op, e.label(c.Field, c.Value)))
	}
	acts := make([]string, 0, len(r.Actions))
	for _, a := range r.Actions {
		if a.Op == "link-schedule" {
			acts = append(acts, "link schedule "+strings.Join(jsonIDs(a.Value), ","))
			continue
		}
		if a.Field != "" {
			acts = append(acts, fmt.Sprintf("%s %s %s", a.Op, a.Field, e.label(a.Field, a.Value)))
		} else {
			acts = append(acts, a.Op)
		}
	}
	return "if " + strings.Join(conds, join) + " then " + strings.Join(acts, ", ")
}

func canonicalConditions(r Rule) string {
	parts := make([]string, 0, len(r.Conditions))
	for _, c := range r.Conditions {
		var v any
		_ = json.Unmarshal(c.Value, &v)
		key := []any{c.Field, c.Op, v}
		if c.Options != (RuleConditionOptions{}) {
			key = append(key, c.Options)
		}
		b, _ := json.Marshal(key)
		parts = append(parts, string(b))
	}
	sort.Strings(parts)
	return strings.ToLower(r.ConditionsOp) + "|" + strings.Join(parts, "|")
}

func setActions(r Rule) map[string]string {
	out := map[string]string{}
	for _, a := range r.Actions {
		if a.Op == "set" && a.Field != "" {
			out[a.Field] = string(a.Value)
		}
	}
	return out
}

// AuditRulesOptions controls AuditRules.
type AuditRulesOptions struct {
	From int // only transactions dated on/after this YYYYMMDD count (0 = all)
	To   int // only transactions dated on/before this YYYYMMDD count (0 = all)
}

// AuditRules evaluates every live rule against live leaf transactions in the
// window and reports findings: "never-matches", "dangling-reference",
// "shadowed-by:<rule id>" (identical conditions and stage, conflicting set
// action — reported on both rules), and "unevaluated" (unsupported
// op/field). Schedule-linked rules are never reported as never-matches or
// unevaluated (their date conditions are managed by the schedule).
func (l *Ledger) AuditRules(ctx context.Context, opt AuditRulesOptions) ([]RuleAudit, error) {
	rules, err := l.Rules(ctx)
	if err != nil {
		return nil, err
	}
	ents, err := l.loadEntityTables(ctx)
	if err != nil {
		return nil, err
	}
	txns, err := l.Txns(ctx, TxnFilter{From: opt.From, To: opt.To, Leaves: true})
	if err != nil {
		return nil, err
	}
	// Lowercase each transaction's string fields once, not once per rule.
	views := make([]txnView, len(txns))
	for i := range txns {
		views[i] = newTxnView(&txns[i])
	}
	out := make([]RuleAudit, 0, len(rules))
	for _, r := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a := RuleAudit{ID: r.ID, Stage: r.Stage, Summary: ents.summary(r), ScheduleID: r.ScheduleID(), Findings: []string{}}
		if a.Stage == "" {
			a.Stage = "default"
		}
		if r.ParseError != "" {
			a.Findings = append(a.Findings, "unparseable: "+r.ParseError)
		}
		supported := r.ParseError == "" && len(r.Conditions) > 0
		if supported && a.ScheduleID == "" {
			cr := compileRule(r)
			supported = cr.supported
			if supported {
				last := 0
				for i := range views {
					if i&4095 == 0 {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
					}
					if m, _ := cr.matches(&views[i]); m {
						a.MatchCount++
						if d := views[i].t.DateInt; d > last {
							last = d
						}
					}
				}
				a.LastMatch = FormatDate(last)
			}
		}
		a.Evaluated = supported && a.ScheduleID == ""
		if a.ScheduleID == "" {
			switch {
			case !supported && r.ParseError == "":
				a.Findings = append(a.Findings, "unevaluated")
			case a.Evaluated && a.MatchCount == 0:
				a.Findings = append(a.Findings, "never-matches")
			}
		}
		if d := ents.dangling(r); len(d) > 0 {
			a.Dangling = d
			a.Findings = append(a.Findings, "dangling-reference")
		}
		out = append(out, a)
	}
	// Conflicts: identical conditions in the same stage setting the same field
	// to different values. Canonical forms are computed once per rule.
	canon := make([]string, len(rules))
	sets := make([]map[string]string, len(rules))
	for i, r := range rules {
		if r.ParseError == "" {
			canon[i] = canonicalConditions(r)
			sets[i] = setActions(r)
		}
	}
	for i := range rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for j := range rules {
			if i == j || rules[i].Stage != rules[j].Stage || rules[i].ParseError != "" || rules[j].ParseError != "" {
				continue
			}
			if canon[i] != canon[j] {
				continue
			}
			for field, v := range sets[i] {
				if w, ok := sets[j][field]; ok && w != v {
					out[i].Findings = append(out[i].Findings, "shadowed-by:"+rules[j].ID)
					break
				}
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Schedules

// ScheduleAudit is the health report for one active schedule.
type ScheduleAudit struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	PayeeID        string   `json:"payee_id,omitempty"`
	Payee          string   `json:"payee,omitempty"`
	AccountID      string   `json:"account_id,omitempty"`
	AmountOp       string   `json:"amount_op,omitempty"`
	ExpectedAmount int64    `json:"expected_amount"`
	ExpectedMin    *int64   `json:"expected_min,omitempty"`
	ExpectedMax    *int64   `json:"expected_max,omitempty"`
	NextDate       string   `json:"next_date,omitempty"`
	PostedCount    int      `json:"posted_count"`
	LastPosted     string   `json:"last_posted,omitempty"`
	LastAmount     *int64   `json:"last_amount,omitempty"`
	DriftPct       *float64 `json:"drift_pct,omitempty"`
	Findings       []string `json:"findings"`
}

// AuditSchedules reports on live, active, non-completed schedules as of asOf
// (YYYYMMDD): "overdue" when the next date is before asOf and nothing linked
// to the schedule posted on/after it; "amount-drift" when the latest linked
// posting is more than tolerancePct percent away from the expected amount
// (outside the range for isbetween); "never-posted" when no transaction was
// ever linked to the schedule. When the mirror's transactions table has no
// schedule column, postings cannot be linked and every schedule gets only a
// "postings-unavailable" finding instead.
func (l *Ledger) AuditSchedules(ctx context.Context, asOf int, tolerancePct float64) ([]ScheduleAudit, error) {
	out := make([]ScheduleAudit, 0)
	// HasTable reports false on a dead context; surface the real error.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !l.HasTable(ctx, "schedules") {
		return out, nil
	}
	nextJoin, nextCol := "", "NULL"
	if l.HasTable(ctx, "schedules_next_date") {
		nextJoin = " LEFT JOIN schedules_next_date n ON n.schedule_id = s.id AND COALESCE(n.tombstone, 0) = 0"
		nextCol = "n.local_next_date"
	}
	rows, err := l.DB.QueryContext(ctx, `SELECT s.id, COALESCE(s.name, ''), COALESCE(s.rule, ''), `+nextCol+`
		FROM schedules s`+nextJoin+`
		WHERE COALESCE(s.tombstone, 0) = 0 AND COALESCE(s.active, 1) = 1 AND COALESCE(s.completed, 0) = 0
		ORDER BY s.name, s.id`)
	if err != nil {
		return nil, fmt.Errorf("querying schedules: %w", err)
	}
	type sched struct {
		ScheduleAudit
		rule string
		next int
	}
	var list []sched
	for rows.Next() {
		var s sched
		var next sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Name, &s.rule, &next); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.next = int(next.Int64)
		list = append(list, s)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	rules, err := l.Rules(ctx)
	if err != nil {
		return nil, err
	}
	ruleByID := map[string]Rule{}
	for _, r := range rules {
		ruleByID[r.ID] = r
	}
	ents, err := l.loadEntityTables(ctx)
	if err != nil {
		return nil, err
	}
	type posting struct {
		date   int
		amount int64
	}
	// All schedule-linked postings in one query, grouped by schedule id.
	hasSchedCol := l.HasColumn(ctx, "transactions", "schedule")
	postsBySched := map[string][]posting{}
	if hasSchedCol {
		prow, err := l.DB.QueryContext(ctx, `SELECT schedule, COALESCE(date, 0), COALESCE(amount, 0) FROM transactions
			WHERE schedule IS NOT NULL AND tombstone = 0 AND COALESCE(isChild, 0) = 0 ORDER BY schedule, date`)
		if err != nil {
			return nil, fmt.Errorf("querying schedule postings: %w", err)
		}
		for prow.Next() {
			var id string
			var p posting
			if err := prow.Scan(&id, &p.date, &p.amount); err != nil {
				_ = prow.Close()
				return nil, err
			}
			postsBySched[id] = append(postsBySched[id], p)
		}
		if err := prow.Close(); err != nil {
			return nil, err
		}
	}
	for _, s := range list {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a := s.ScheduleAudit
		a.Findings = []string{}
		a.NextDate = FormatDate(s.next)
		var lo, hi float64
		hasRange := false
		for _, c := range ruleByID[s.rule].Conditions {
			switch c.Field {
			case "payee", "description":
				if ids := jsonIDs(c.Value); len(ids) > 0 {
					a.PayeeID = ids[0]
					a.Payee = ents.payees[ids[0]].Name
				}
			case "account", "acct":
				if ids := jsonIDs(c.Value); len(ids) > 0 {
					a.AccountID = ids[0]
				}
			case "amount":
				a.AmountOp = c.Op
				var n float64
				if json.Unmarshal(c.Value, &n) == nil {
					a.ExpectedAmount = int64(n)
				} else {
					var r struct {
						Num1 float64 `json:"num1"`
						Num2 float64 `json:"num2"`
					}
					if json.Unmarshal(c.Value, &r) == nil {
						lo, hi = math.Min(r.Num1, r.Num2), math.Max(r.Num1, r.Num2)
						hasRange = true
						mn, mx := int64(lo), int64(hi)
						a.ExpectedMin, a.ExpectedMax = &mn, &mx
						a.ExpectedAmount = int64(math.Round((lo + hi) / 2))
					}
				}
			}
		}
		if !hasSchedCol {
			// Without transactions.schedule nothing can be linked to the
			// schedule, so never-posted/overdue/drift would all be guesses.
			a.Findings = append(a.Findings, "postings-unavailable")
			out = append(out, a)
			continue
		}
		posts := postsBySched[s.ID]
		a.PostedCount = len(posts)
		if len(posts) == 0 {
			a.Findings = append(a.Findings, "never-posted")
		} else {
			last := posts[len(posts)-1]
			a.LastPosted = FormatDate(last.date)
			la := last.amount
			a.LastAmount = &la
			v := float64(la)
			drift := false
			if hasRange {
				tol := tolerancePct / 100
				if v < lo-math.Abs(lo)*tol || v > hi+math.Abs(hi)*tol {
					drift = true
				}
			} else if a.AmountOp != "" && a.ExpectedAmount != 0 {
				exp := float64(a.ExpectedAmount)
				pct := math.Abs(v-exp) / math.Abs(exp) * 100
				pct = math.Round(pct*100) / 100
				a.DriftPct = &pct
				drift = pct > tolerancePct
			}
			if drift {
				a.Findings = append(a.Findings, "amount-drift")
			}
		}
		if s.next > 0 && s.next < asOf {
			posted := false
			for _, p := range posts {
				if p.date >= s.next {
					posted = true
					break
				}
			}
			if !posted {
				a.Findings = append(a.Findings, "overdue")
			}
		}
		out = append(out, a)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Category suggestions

// CategorySuggestion is a learned category for an uncategorized transaction.
type CategorySuggestion struct {
	TransactionID       string  `json:"transaction_id"`
	Date                string  `json:"date"`
	Amount              int64   `json:"amount"`
	AccountID           string  `json:"account_id"`
	PayeeID             string  `json:"payee_id"`
	Payee               string  `json:"payee"`
	SuggestedCategoryID string  `json:"suggested_category_id"`
	SuggestedCategory   string  `json:"suggested_category"`
	Confidence          float64 `json:"confidence"`
	HistoryCount        int     `json:"history_count"`
}

// SuggestCategories suggests, for each uncategorized transaction, the most
// frequent category used historically with the same resolved payee (live,
// categorized, non-transfer leaf transactions; merged categories resolved).
// Confidence is that category's share of the payee's history. Suggestions
// need at least minHistory history rows and minConfidence confidence.
func (l *Ledger) SuggestCategories(ctx context.Context, minHistory int, minConfidence float64) ([]CategorySuggestion, error) {
	all, err := l.Txns(ctx, TxnFilter{Leaves: true})
	if err != nil {
		return nil, err
	}
	type tally struct {
		total  int
		counts map[string]int
		names  map[string]string
	}
	hist := map[string]*tally{}
	for _, t := range all {
		if t.CategoryID == "" || t.PayeeID == "" || t.TransferAccountID != "" {
			continue
		}
		h := hist[t.PayeeID]
		if h == nil {
			h = &tally{counts: map[string]int{}, names: map[string]string{}}
			hist[t.PayeeID] = h
		}
		h.total++
		h.counts[t.CategoryID]++
		h.names[t.CategoryID] = t.Category
	}
	unc, err := l.Txns(ctx, TxnFilter{Uncategorized: true})
	if err != nil {
		return nil, err
	}
	out := make([]CategorySuggestion, 0)
	for _, t := range unc {
		h := hist[t.PayeeID]
		if t.PayeeID == "" || h == nil || h.total < minHistory {
			continue
		}
		best, bestN := "", 0
		for id, n := range h.counts {
			if n > bestN || (n == bestN && (h.names[id] < h.names[best] || (h.names[id] == h.names[best] && id < best))) {
				best, bestN = id, n
			}
		}
		conf := float64(bestN) / float64(h.total)
		if conf < minConfidence {
			continue
		}
		out = append(out, CategorySuggestion{
			TransactionID: t.ID, Date: t.Date, Amount: t.Amount, AccountID: t.AccountID,
			PayeeID: t.PayeeID, Payee: t.Payee,
			SuggestedCategoryID: best, SuggestedCategory: h.names[best],
			Confidence: math.Round(conf*1000) / 1000, HistoryCount: h.total,
		})
	}
	return out, nil
}
