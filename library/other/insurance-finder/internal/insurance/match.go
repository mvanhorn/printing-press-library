package insurance

import "sort"

// Scoring constants. The matcher is intentionally simple and data-driven: it
// reads each provider's Appetite ratings from the registry and converts them to
// a score, then applies channel/type bonuses and penalties. The key business
// rule — an importer / private-label / manufacturer ("deemed manufacturer")
// must be routed to specialty markets, never to mainstream instant-quote
// carriers that decline the class — falls out of the data: those carriers carry
// a "decline" importer appetite, which scoreFit maps to a large negative.
const (
	scoreBest    = 40
	scoreGood    = 28
	scorePartial = 14
	scorePoor    = 6
	scoreDecline = -60
	scoreUnknown = 0

	bonusInstantNonImporter = 10 // speed matters for low-hazard classes
	penaltyUnverified       = 10

	// Tier thresholds.
	tierRecommendedAt = 30
	tierAvoidBelow    = 0
)

// importerTypeBonus rewards markets that can actually place the importer /
// deemed-manufacturer class.
var importerTypeBonus = map[string]int{
	ProviderTypeSurplusSpecialty: 12, // built for hard-to-place product liability
	ProviderTypeWholesalerAgent:  8,  // writes the class, but agent-only
	ProviderTypeBrokerMarket:     6,  // can route to specialty markets
	ProviderTypeDigitalAgency:    6,  // shops many carriers
	ProviderTypeDirectCarrier:    0,  // a single carrier's own appetite
}

func scoreFit(rating string) int {
	switch rating {
	case FitBest:
		return scoreBest
	case FitGood:
		return scoreGood
	case FitPartial:
		return scorePartial
	case FitPoor:
		return scorePoor
	case FitDecline:
		return scoreDecline
	default:
		return scoreUnknown
	}
}

// Match ranks providers for the given profile, best fit first. It is a pure
// function: same inputs always yield the same ordered output.
func Match(p Profile, providers []Provider) []Recommendation {
	recs := make([]Recommendation, 0, len(providers))
	for _, prov := range providers {
		recs = append(recs, scoreProvider(p, prov))
	}
	// Stable ordering: score desc, then name asc for deterministic ties.
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].Score != recs[j].Score {
			return recs[i].Score > recs[j].Score
		}
		return recs[i].Provider.Name < recs[j].Provider.Name
	})
	for i := range recs {
		recs[i].Rank = i + 1
	}
	return recs
}

func scoreProvider(p Profile, prov Provider) Recommendation {
	rec := Recommendation{Provider: prov}

	if p.IsImporterClass() {
		scoreImporter(p, prov, &rec)
	} else {
		scoreStandard(p, prov, &rec)
	}

	if prov.Unverified {
		rec.Score -= penaltyUnverified
		rec.Cautions = append(rec.Cautions,
			"UNVERIFIED registry entry - confirm the URL, carrier panel, appetite, and AM Best rating before relying on this.")
	}

	rec.Tier = tierFor(rec.Score)
	addChannelCautions(prov, &rec)
	return rec
}

// scoreImporter handles the importer / private-label / manufacturer class.
func scoreImporter(p Profile, prov Provider, rec *Recommendation) {
	// Collect the appetite ratings that actually apply to this applicant.
	applicable := []string{prov.Appetite.Importer}
	if p.PrivateLabel {
		applicable = append(applicable, prov.Appetite.PrivateLabel)
	}
	if p.Manufacturer {
		applicable = append(applicable, prov.Appetite.Manufacturer)
	}

	// A single decline is disqualifying: if the carrier won't write importers,
	// nothing else matters. Otherwise the best applicable rating leads — a
	// specialty market strong on importers should not be dragged down because
	// its private-label cell happens to read "good".
	declines := false
	best := scoreUnknown
	for _, r := range applicable {
		if r == FitDecline {
			declines = true
		}
		if s := scoreFit(r); s > best {
			best = s
		}
	}

	if declines {
		rec.Score = scoreDecline
		rec.Reasons = append(rec.Reasons,
			"Declines the imported private-label \"deemed manufacturer\" class (confirmed in live testing for mainstream instant-quote carriers).")
		return
	}

	rec.Score = best + importerTypeBonus[prov.Type]
	rec.Reasons = append(rec.Reasons, importerReasons(prov)...)

	// Standard / retail-class GL that will write the risk but is not built for
	// imported goods: the #1 importer landmine is a foreign-products exclusion.
	if isStandardMarket(prov) && best <= scorePartial {
		rec.Cautions = append(rec.Cautions,
			"Standard / retail-class GL - confirm IN WRITING there is NO foreign-products / imported-goods exclusion before binding.")
	}
}

// scoreStandard handles non-importer low-hazard retail/service classes, where
// fast mainstream instant-quote carriers are the right first stop.
func scoreStandard(p Profile, prov Provider, rec *Recommendation) {
	rating := prov.Appetite.Retail
	if rating == "" {
		rating = prov.Appetite.Service
	}
	rec.Score = scoreFit(rating)
	if prov.InstantQuote {
		rec.Score += bonusInstantNonImporter
		rec.Reasons = append(rec.Reasons, "Instant online quote - fast turnaround for a low-hazard class.")
	}
	rec.Reasons = append(rec.Reasons, standardReasons(prov)...)
	if rating == FitDecline {
		rec.Reasons = append(rec.Reasons, "Carrier declines this class.")
	}
}

func tierFor(score int) string {
	switch {
	case score >= tierRecommendedAt:
		return TierRecommended
	case score <= tierAvoidBelow:
		return TierAvoid
	default:
		return TierConsider
	}
}

func isStandardMarket(prov Provider) bool {
	return prov.Type == ProviderTypeDirectCarrier || prov.Type == ProviderTypeBrokerMarket
}

func importerReasons(prov Provider) []string {
	switch prov.Type {
	case ProviderTypeSurplusSpecialty:
		return []string{"Surplus-lines specialty market built for hard-to-place imported / private-label product liability."}
	case ProviderTypeWholesalerAgent:
		return []string{"Wholesaler that writes product / foreign / international liability (agent-placed)."}
	case ProviderTypeBrokerMarket:
		return []string{"Marketplace that can route the importer class to a specialty carrier."}
	case ProviderTypeDigitalAgency:
		return []string{"Multi-carrier agency that can shop the importer class to specialty markets."}
	default:
		return []string{"Direct carrier - confirm it will write an importer / deemed-manufacturer."}
	}
}

func standardReasons(prov Provider) []string {
	switch prov.Type {
	case ProviderTypeDirectCarrier:
		return []string{"Direct carrier with appetite for low-hazard retail/service."}
	case ProviderTypeBrokerMarket:
		return []string{"Marketplace comparing multiple carriers for a fast quote."}
	case ProviderTypeDigitalAgency:
		return []string{"Multi-carrier agency for low-hazard classes."}
	case ProviderTypeSurplusSpecialty:
		return []string{"Specialty market - usually overkill for a low-hazard class, but available."}
	default:
		return nil
	}
}

// addChannelCautions surfaces where the real submission / lead capture happens
// so a multi-step wizard does not surprise the applicant.
func addChannelCautions(prov Provider, rec *Recommendation) {
	switch prov.QuoteChannel {
	case QuoteChannelLeadMatch:
		rec.Cautions = append(rec.Cautions,
			"Lead-match model: entering your phone number connects you to a carrier by phone - not an instant self-serve quote.")
	case QuoteChannelOnlineApplication:
		rec.Cautions = append(rec.Cautions,
			"Multi-step wizard: the lead is often captured at the CONTACT step (phone + Next = submit + marketing consent).")
	case QuoteChannelPhoneAgent:
		rec.Cautions = append(rec.Cautions,
			"No online consumer form - this market is agent / phone only.")
	}
}

// Shortlist returns only the recommendations in the "recommended" and "consider"
// tiers (i.e. drops the "avoid" tier), preserving rank order.
func Shortlist(recs []Recommendation) []Recommendation {
	out := make([]Recommendation, 0, len(recs))
	for _, r := range recs {
		if r.Tier != TierAvoid {
			out = append(out, r)
		}
	}
	return out
}
