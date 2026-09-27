// pp:data-source computed
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"math"
	"reflect"
)

// observedPriceDifference describes source observations, not equivalent-offer savings.
// Delta is right minus left and is absent when values or their basis are unknown.
type observedPriceDifference struct {
	Field         string  `json:"field"`
	Left          *int64  `json:"left"`
	Right         *int64  `json:"right"`
	LeftCurrency  *string `json:"left_currency"`
	RightCurrency *string `json:"right_currency"`
	LeftUnit      *string `json:"left_unit"`
	RightUnit     *string `json:"right_unit"`
	Delta         *int64  `json:"delta"`
}

type offerEquivalence struct {
	Equivalent               bool                      `json:"equivalent"`
	Differences              []string                  `json:"differences"`
	Unknowns                 []string                  `json:"unknowns"`
	Basis                    string                    `json:"basis"`
	SourceAmountDifference   *int64                    `json:"source_amount_difference"`
	ObservedPriceDifferences []observedPriceDifference `json:"observed_price_differences"`
}

func assessOffers(a, b ikyu.OfferResult) offerEquivalence {
	out := offerEquivalence{Differences: []string{}, Unknowns: []string{}, Basis: "same verified stay, physical room, included meals, cancellation, payment and relevant price eligibility; source amounts only"}
	diff := func(s string) { out.Differences = append(out.Differences, s) }
	unknown := func(s string) { out.Unknowns = append(out.Unknowns, s) }
	x, y := a.Data, b.Data
	if x.Property.ID == "" || x.Room.ID == "" || y.Property.ID == "" || y.Room.ID == "" {
		unknown("property/room identity")
	} else if x.Property.ID != y.Property.ID || x.Room.ID != y.Room.ID {
		diff("property/room identity")
	}
	if x.Offer.Stay != y.Offer.Stay {
		diff("dates and full per-room party")
	}
	if !x.Offer.DateVerified || !y.Offer.DateVerified || x.Offer.EchoStay == nil || y.Offer.EchoStay == nil || *x.Offer.EchoStay != x.Offer.Stay || *y.Offer.EchoStay != y.Offer.Stay {
		unknown("source date/party verification")
	}
	if x.Offer.Available == nil || y.Offer.Available == nil || !*x.Offer.Available || !*y.Offer.Available {
		unknown("available exact quote")
	}
	if a.Freshness.Stale || b.Freshness.Stale {
		unknown("stale source observation")
	}
	if x.Plan.PointVariation == nil || y.Plan.PointVariation == nil {
		unknown("point variation identity")
	} else if *x.Plan.PointVariation != *y.Plan.PointVariation {
		diff("point variation identity")
	}
	if x.Plan.Meal.Code == "" || y.Plan.Meal.Code == "" {
		unknown("included meal code")
	} else if x.Plan.Meal.Code != y.Plan.Meal.Code {
		diff("included meal code")
	} else if x.Plan.Meal.Code != "000" {
		if !x.Plan.MealDetailsKnown || !y.Plan.MealDetailsKnown || len(x.Plan.MealDetails) == 0 || len(y.Plan.MealDetails) == 0 {
			unknown("meal inclusions/venue/menu")
		} else if !reflect.DeepEqual(x.Plan.MealDetails, y.Plan.MealDetails) {
			diff("meal inclusions/venue/menu")
		}
	}
	if !knownCancellation(x.Plan.Cancellation) || !knownCancellation(y.Plan.Cancellation) {
		unknown("complete cancellation rules")
	} else if !reflect.DeepEqual(x.Plan.Cancellation.Rules, y.Plan.Cancellation.Rules) {
		diff("cancellation rules")
	}
	compareText := func(label string, p, q *string) {
		if p == nil || q == nil {
			unknown(label)
		} else if *p != *q {
			diff(label)
		}
	}
	compareText("payment settlement", x.Plan.Payment, y.Plan.Payment)
	compareText("published plan content/benefits", x.Plan.Content, y.Plan.Content)
	compareText("property tax/extra-fee notes", x.Property.Notes, y.Property.Notes)
	if !reflect.DeepEqual(x.Plan.MemberRank, y.Plan.MemberRank) {
		diff("published member-rank restriction")
	}
	if !reflect.DeepEqual(x.Plan.MinRooms, y.Plan.MinRooms) || !reflect.DeepEqual(x.Plan.MaxRooms, y.Plan.MaxRooms) || !reflect.DeepEqual(x.Plan.MinNights, y.Plan.MinNights) || !reflect.DeepEqual(x.Plan.MaxNights, y.Plan.MaxNights) {
		diff("room/night restrictions")
	}
	hx, okx := effectiveHours(x)
	hy, oky := effectiveHours(y)
	if !okx || !oky {
		unknown("effective check-in/out hours")
	} else if hx != hy {
		diff("effective check-in/out hours")
	}
	px, py := x.Offer.Price, y.Offer.Price
	out.ObservedPriceDifferences = observePriceDifferences(px, py)
	if px == nil || py == nil || px.Amount == nil || py.Amount == nil {
		unknown("source price basis")
	} else {
		if px.Currency == "" || py.Currency == "" || px.Unit == "" || py.Unit == "" {
			unknown("currency/price units")
		} else if px.Currency != py.Currency || px.Unit != py.Unit {
			diff("currency/price units")
		}
		if !px.EligibilityKnown || !py.EligibilityKnown {
			unknown("conditional price/payment/member eligibility")
		}
		// Published coupon metadata does not establish an equal rule/formula when its discount changes.
		if !reflect.DeepEqual(px.Coupon, py.Coupon) {
			diff("source coupon conditions")
		}
		if px.PointRate == nil || py.PointRate == nil || px.InstantPointRate == nil || py.InstantPointRate == nil {
			unknown("points scenario terms")
		} else if *px.PointRate != *py.PointRate || *px.InstantPointRate != *py.InstantPointRate {
			diff("points scenario terms")
		}
	}
	out.Equivalent = len(out.Differences) == 0 && len(out.Unknowns) == 0
	if out.Equivalent {
		out.SourceAmountDifference = observedDelta(px.Amount, py.Amount)
	}
	return out
}
func knownCancellation(c ikyu.Cancellation) bool {
	if !c.Known || len(c.Rules) == 0 {
		return false
	}
	for _, r := range c.Rules {
		if r.Type != "CancelPolicyRuleDay" && r.Type != "CancelPolicyRuleNoShow" {
			return false
		}
		if r.Type == "CancelPolicyRuleDay" && r.Day == nil {
			return false
		}
		if r.Amount == nil && r.Rate == nil {
			return false
		}
	}
	return true
}
func effectiveHours(o ikyu.OfferData) ([3]string, bool) {
	p := o.Plan
	r := o.Room
	var values []*string
	if p.UseCheckInOut == nil {
		return [3]string{}, false
	}
	if *p.UseCheckInOut {
		values = []*string{p.CheckInFrom, p.CheckInTo, p.CheckOut}
	} else {
		values = []*string{r.CheckInFrom, r.CheckInTo, r.CheckOut}
	}
	var out [3]string
	for i, v := range values {
		if v == nil || *v == "" {
			return out, false
		}
		out[i] = *v
	}
	return out, true
}

func observePriceDifferences(left, right *ikyu.Price) []observedPriceDifference {
	fields := []struct {
		name   string
		points bool
		value  func(*ikyu.Price) *int64
	}{
		{"source_amount", false, func(p *ikyu.Price) *int64 { return p.Amount }},
		{"headline_before_points", false, func(p *ikyu.Price) *int64 { return p.HeadlineBeforePoints }},
		{"base_discount_amount", false, func(p *ikyu.Price) *int64 { return p.BaseDiscountAmount }},
		{"instant_points_payable", false, func(p *ikyu.Price) *int64 { return p.DiscountAmount }},
		{"earn_points_payable", false, func(p *ikyu.Price) *int64 { return p.DiscountAmountEarn }},
		{"payable_without_coupon", false, func(p *ikyu.Price) *int64 { return p.DiscountAmountWithoutCoupon }},
		{"points_earned", true, func(p *ikyu.Price) *int64 { return p.Point }},
		{"points_applied", true, func(p *ikyu.Price) *int64 { return p.InstantPoint }},
		{"source_coupon.discount_amount", false, func(p *ikyu.Price) *int64 {
			if p.Coupon == nil {
				return nil
			}
			return p.Coupon.DiscountAmount
		}},
		{"checkout_confirmed_payable", false, func(p *ikyu.Price) *int64 { return p.CheckoutConfirmedPayable }},
	}
	text := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	context := func(p *ikyu.Price, points bool) (*string, *string) {
		if points {
			unit := "points"
			return nil, &unit
		}
		if p == nil {
			return nil, nil
		}
		return text(p.Currency), text(p.Unit)
	}
	sameBasis := left != nil && right != nil && left.Currency != "" && left.Currency == right.Currency && left.Unit != "" && left.Unit == right.Unit
	rows := []observedPriceDifference{}
	for _, field := range fields {
		var l, r *int64
		if left != nil {
			l = field.value(left)
		}
		if right != nil {
			r = field.value(right)
		}
		if l != nil && r != nil && *l == *r && sameBasis {
			continue
		}
		lc, lu := context(left, field.points)
		rc, ru := context(right, field.points)
		row := observedPriceDifference{Field: field.name, Left: l, Right: r, LeftCurrency: lc, RightCurrency: rc, LeftUnit: lu, RightUnit: ru}
		if sameBasis {
			row.Delta = observedDelta(l, r)
		}
		rows = append(rows, row)
	}
	return rows
}

func observedDelta(left, right *int64) *int64 {
	if left == nil || right == nil {
		return nil
	}
	if (*left > 0 && *right < math.MinInt64+*left) || (*left < 0 && *right > math.MaxInt64+*left) {
		return nil
	}
	delta := *right - *left
	return &delta
}
