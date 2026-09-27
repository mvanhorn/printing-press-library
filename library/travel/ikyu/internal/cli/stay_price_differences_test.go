package cli

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"math"
	"testing"
)

func pricedComparableFixture() ikyu.OfferResult {
	result := comparableFixture()
	p := result.Data.Offer.Price
	p.HeadlineBeforePoints = ptr(int64(30800))
	p.BaseDiscountAmount = ptr(int64(30800))
	p.DiscountAmount = ptr(int64(24640))
	p.DiscountAmountEarn = ptr(int64(30800))
	p.DiscountAmountWithoutCoupon = ptr(int64(24640))
	p.Point = ptr(int64(6160))
	p.InstantPoint = ptr(int64(6160))
	return result
}
func observedField(t *testing.T, assessment offerEquivalence, field string) observedPriceDifference {
	t.Helper()
	for _, row := range assessment.ObservedPriceDifferences {
		if row.Field == field {
			return row
		}
	}
	t.Fatalf("missing observation %s: %+v", field, assessment.ObservedPriceDifferences)
	return observedPriceDifference{}
}
func TestOfferObservedPricesKeepCompatibleTermsEquivalent(t *testing.T) {
	a := pricedComparableFixture()
	b := copyOffer(a)
	p := b.Data.Offer.Price
	p.Amount = ptr(int64(32000))
	p.HeadlineBeforePoints = ptr(int64(31000))
	p.BaseDiscountAmount = ptr(int64(31000))
	p.DiscountAmount = ptr(int64(26000))
	p.DiscountAmountEarn = ptr(int64(31000))
	p.DiscountAmountWithoutCoupon = ptr(int64(26000))
	p.Point = ptr(int64(6000))
	p.InstantPoint = ptr(int64(6500))
	p.Scenarios = []ikyu.PriceScenario{{Name: "earn_points", PublishedPayable: p.DiscountAmountEarn, PotentialPointsEarned: p.Point}, {Name: "use_points_now", PublishedPayable: p.DiscountAmount, PotentialPointsApplied: p.InstantPoint}}
	assessment := assessOffers(a, b)
	if !assessment.Equivalent || assessment.SourceAmountDifference == nil || *assessment.SourceAmountDifference != 1200 {
		t.Fatalf("prices changed term equivalence %+v", assessment)
	}
	expected := map[string]int64{"source_amount": 1200, "headline_before_points": 200, "base_discount_amount": 200, "instant_points_payable": 1360, "earn_points_payable": 200, "payable_without_coupon": 1360, "points_earned": -160, "points_applied": 340}
	if len(assessment.ObservedPriceDifferences) != 10 {
		t.Fatalf("unbounded/duplicated observations %+v", assessment.ObservedPriceDifferences)
	}
	for field, delta := range expected {
		row := observedField(t, assessment, field)
		if row.Delta == nil || *row.Delta != delta || row.Left == nil || row.Right == nil {
			t.Fatalf("incorrect %s observation %+v", field, row)
		}
		if field == "points_earned" || field == "points_applied" {
			if row.LeftCurrency != nil || row.RightCurrency != nil || row.LeftUnit == nil || *row.LeftUnit != "points" || row.RightUnit == nil || *row.RightUnit != "points" {
				t.Fatalf("points labelled as money %+v", row)
			}
		} else if row.LeftCurrency == nil || *row.LeftCurrency != "JPY" || row.RightCurrency == nil || *row.RightCurrency != "JPY" || row.LeftUnit == nil || *row.LeftUnit != "booking_total" || row.RightUnit == nil || *row.RightUnit != "booking_total" {
			t.Fatalf("missing source money basis %+v", row)
		}
	}
}
func TestOfferObservedPricesRemainObservationsForUnknownOrDifferentTerms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alter func(*ikyu.OfferResult)
		delta bool
	}{
		{"unknown eligibility", func(b *ikyu.OfferResult) { b.Data.Offer.Price.EligibilityKnown = false }, true},
		{"different room", func(b *ikyu.OfferResult) { b.Data.Room.ID = "10193727" }, true},
		{"missing currency", func(b *ikyu.OfferResult) { b.Data.Offer.Price.Currency = "" }, false},
		{"different currency", func(b *ikyu.OfferResult) { b.Data.Offer.Price.Currency = "USD" }, false},
		{"missing booking unit", func(b *ikyu.OfferResult) { b.Data.Offer.Price.Unit = "" }, false},
		{"different booking unit", func(b *ikyu.OfferResult) { b.Data.Offer.Price.Unit = "per_room" }, false},
		{"missing original amount", func(b *ikyu.OfferResult) { b.Data.Offer.Price.Amount = nil }, false},
		{"missing price", func(b *ikyu.OfferResult) { b.Data.Offer.Price = nil }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := pricedComparableFixture()
			b := copyOffer(a)
			b.Data.Offer.Price.Amount = ptr(int64(30000))
			b.Data.Offer.Price.Point = ptr(int64(6000))
			tc.alter(&b)
			assessment := assessOffers(a, b)
			if assessment.Equivalent || assessment.SourceAmountDifference != nil {
				t.Fatalf("unknown/incompatible savings fabricated %+v", assessment)
			}
			row := observedField(t, assessment, "source_amount")
			if row.Left == nil || *row.Left != 30800 || (row.Delta != nil) != tc.delta {
				t.Fatalf("observation lost or guessed basis %+v", row)
			}
			if tc.delta && *row.Delta != -800 {
				t.Fatalf("observed source difference %+v", row)
			}
			if b.Data.Offer.Price != nil && b.Data.Offer.Price.Amount != nil && (row.Right == nil || *row.Right != 30000) {
				t.Fatalf("right source price lost %+v", row)
			}
			if tc.name == "missing currency" && row.RightCurrency != nil {
				t.Fatal("missing currency made non-null")
			}
			if tc.name == "missing booking unit" && row.RightUnit != nil {
				t.Fatal("missing unit made non-null")
			}
			points := observedField(t, assessment, "points_earned")
			pointDeltaKnown := tc.delta || tc.name == "missing original amount"
			if (points.Delta != nil) != pointDeltaKnown || (pointDeltaKnown && *points.Delta != -160) {
				t.Fatalf("points delta must use its own known values/basis %+v", points)
			}
		})
	}
}
func TestOfferMissingObservedPricesAreExplicitAndCouponGateRemainsConservative(t *testing.T) {
	a := pricedComparableFixture()
	b := copyOffer(a)
	b.Data.Offer.Price.DiscountAmount = nil
	assessment := assessOffers(a, b)
	row := observedField(t, assessment, "instant_points_payable")
	if row.Left == nil || *row.Left != 24640 || row.Right != nil || row.Delta != nil {
		t.Fatalf("missing amount fabricated %+v", row)
	}
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]any
	_ = json.Unmarshal(raw, &encoded)
	if value, ok := encoded["right"]; !ok || value != nil {
		t.Fatal("missing right is not explicit JSON null")
	}
	if value, ok := encoded["delta"]; !ok || value != nil {
		t.Fatal("undefined delta is not explicit JSON null")
	}
	missing := observedField(t, assessment, "checkout_confirmed_payable")
	if missing.Left != nil || missing.Right != nil || missing.Delta != nil {
		t.Fatal("missing checkout prices inferred")
	}
	a.Data.Offer.Price.Coupon = &ikyu.Coupon{Name: ptr("公開クーポン"), DiscountAmount: ptr(int64(500)), EligibilityKnown: true}
	b = copyOffer(a)
	b.Data.Offer.Price.Coupon.DiscountAmount = ptr(int64(1000))
	assessment = assessOffers(a, b)
	row = observedField(t, assessment, "source_coupon.discount_amount")
	if assessment.Equivalent || assessment.SourceAmountDifference != nil || row.Delta == nil || *row.Delta != 500 || row.LeftCurrency == nil || *row.LeftCurrency != "JPY" {
		t.Fatalf("coupon rules inferred or amounts hidden %+v", assessment)
	}
}
func TestOfferObservedDeltaDoesNotOverflow(t *testing.T) {
	a := pricedComparableFixture()
	b := copyOffer(a)
	a.Data.Offer.Price.Amount = ptr(int64(math.MinInt64))
	b.Data.Offer.Price.Amount = ptr(int64(math.MaxInt64))
	assessment := assessOffers(a, b)
	row := observedField(t, assessment, "source_amount")
	if !assessment.Equivalent || assessment.SourceAmountDifference != nil || row.Delta != nil || row.Left == nil || row.Right == nil {
		t.Fatalf("overflowing amount difference %+v", assessment)
	}
}

func TestComparePublishesObservedPricesWithoutEquivalentSavings(t *testing.T) {
	reader := &workflowReader{behavior: func(request ikyu.OfferRequest) (ikyu.OfferResult, error) {
		result := pricedComparableFixture()
		result.Data.Plan.ID = request.PlanID
		result.Data.Offer.Stay = request.Stay
		result.Data.Offer.EchoStay = &request.Stay
		result.Data.Offer.Price.EligibilityKnown = false
		if request.PlanID == "11055981" {
			result.Data.Offer.Price.Amount = ptr(int64(32000))
		}
		return result, nil
	}}
	output, _, err := runStay(t, reader, "stay", "compare", "--offers", "00002889:10193741:11055986,00002889:10193741:11055981", "--check-in", "2026-10-18", "--check-out", "2026-10-19")
	if err != nil {
		t.Fatal(err)
	}
	comparisons := output["comparisons"].([]any)
	if len(comparisons) != 1 {
		t.Fatalf("comparison missing %v", output)
	}
	assessment := comparisons[0].(map[string]any)["assessment"].(map[string]any)
	if assessment["equivalent"] != false || assessment["source_amount_difference"] != nil {
		t.Fatalf("unverified savings published %v", assessment)
	}
	for _, value := range assessment["observed_price_differences"].([]any) {
		row := value.(map[string]any)
		if row["field"] == "source_amount" {
			if row["left"] != float64(30800) || row["right"] != float64(32000) || row["delta"] != float64(1200) || row["left_currency"] != "JPY" || row["left_unit"] != "booking_total" {
				t.Fatalf("price observation lost in CLI %v", row)
			}
			return
		}
	}
	t.Fatalf("non-equivalent price observations hidden %v", assessment)
}
