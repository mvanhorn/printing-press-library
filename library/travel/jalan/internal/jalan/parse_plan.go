package jalan

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func ParsePlan(doc, sourceURL, propertyID, planID, roomID string) (Plan, error) {
	out := Plan{Offer: emptyOffer(propertyID, sourceURL), Restrictions: []string{}}
	root, err := parseHTML(doc)
	if err != nil {
		return out, err
	}
	overview := classNode(root, "p-planOverview")
	m := offerIDRE.FindStringSubmatch(attr(overview, "id"))
	if len(m) != 4 || m[1] != propertyID || m[2] != planID || m[3] != roomID {
		return out, fmt.Errorf("jalan: exact room/plan details unavailable or identity does not match request")
	}
	out.PlanID = planID
	out.RoomID = roomID
	out.PlanName = classText(overview, "p-planOverview__title")
	out.RoomName = classText(idNode(root, "roomTypeNameId"), "jlnpc-planDetailInfo__planTitle__text")
	if out.PlanName == "" || out.RoomName == "" {
		return out, fmt.Errorf("jalan: exact plan details lack plan or room name")
	}
	out.RoomDescription = classText(root, "jlnpc-planDetailInfo__roomDetail")
	out.Description = classText(root, "jlnpc-planDetailInfo__planDetail")
	labels := []string{out.RoomName}
	for _, label := range classNodes(classNode(overview, "p-planOverview__room"), "c-label") {
		labels = append(labels, textOf(label))
	}
	for _, label := range classNodes(classNode(root, "jlnpc-planDetailInfo__facilityLabel"), "c-label") {
		labels = append(labels, textOf(label))
	}
	out.Baths = roomBaths(append(labels, out.RoomDescription), sourceURL)
	out.Smoking = smoking(labels)
	if out.Smoking == "unknown" && !strings.Contains(strings.Join(labels, " "), "禁煙") && !strings.Contains(strings.Join(labels, " "), "喫煙") {
		// Description fallback requires an explicit room declaration; incidental
		// mentions of alternative room types do not establish this room's status.
		declarations := regexp.MustCompile(`(?:^|■)(禁煙室|喫煙室|全室禁煙室|全室喫煙室)(?:[ 。■]|$)`).FindAllStringSubmatch(out.RoomDescription, -1)
		parts := []string{}
		for _, d := range declarations {
			parts = append(parts, d[1])
		}
		out.Smoking = smoking(parts)
	}
	mealText := labelledDL(overview, "食事")
	out.Meals = meals(mealText)
	out.CheckIn = labelledDL(overview, "チェックイン")
	out.CheckOut = labelledDL(overview, "チェックアウト")
	// The charge is the whole stay's observed quote. The neighboring coupon
	// and points elements are intentionally parsed into separate fields.
	charge := classText(overview, "p-planOverview__charge")
	dateText := classText(overview, "p-planOverview__summary__date")
	out.Price = priceQuote(charge, charge+" "+dateText, sourceURL)
	out.Price.ConditionalDiscount = classText(overview, "couponCollection__head")
	if s := classText(overview, "p-planOverview__couponDesc"); s != "" {
		out.Price.ConditionalDiscount = clean(out.Price.ConditionalDiscount + " " + s)
	}
	out.Price.Points = classText(overview, "p-planOverview__point")
	out.Fees = classText(root, "jlnpc-planRoomPrice__notices")
	out.Price.ExtraFees = out.Fees
	out.Payment = classText(root, "jlnpc-planRoomPrice__payment")
	out.BookingDeadline = classText(root, "jlnpc-planRoomPrice__deadline")
	out.PriceBreakdown = classText(root, "jlnpc-planRoomPriceDetai__chargeDetails")
	out.OccupancyText = clean(dateText + " " + out.PriceBreakdown)
	out.Price.OccupancyText = out.OccupancyText
	cancellation := classNode(root, "jlnpc-planRoomPrice__flexArea__right")
	if cancellation != nil {
		parts := []string{}
		for c := cancellation.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "p" || c.Data == "table") {
				if s := textOf(c); s != "" {
					parts = append(parts, s)
				}
			}
		}
		out.Cancellation = strings.Join(parts, " ")
	}
	if out.Cancellation == "" {
		out.Cancellation = classText(overview, "p-planOverview__tooltip")
	}
	for _, n := range classNodes(overview, "p-planOverview__note") {
		if s := textOf(n); s != "" {
			out.Restrictions = append(out.Restrictions, s)
		}
	}
	status := classText(overview, "p-planOverview__roomRemain")
	if status == "" {
		status = classText(root, "jlnpc-searchPlanPanel--dated__title")
	}
	if strings.Contains(status, "空室なし") || strings.Contains(status, "満室") || strings.Contains(status, "残室数： 0") {
		out.Availability = "unavailable"
	} else if status != "" && (strings.Contains(status, "部屋") || strings.Contains(status, "空室")) {
		out.Availability = "available"
	}
	// Jalan can return HTTP 200 while silently replacing a requested date with
	// an undated minimum. The explanatory rejection, rather than that minimum,
	// determines the requested offer's availability.
	rejection := classText(root, "p-details01__chargeInfo")
	if strings.Contains(rejection, "ご指定された条件") && strings.Contains(rejection, "ご利用いただけません") {
		out.Availability = "unavailable"
		out.Price = emptyPrice()
		out.Price.ReferenceQuote = charge
		addEvidence(&out.Price.Evidence, "price.reference_quote", charge+" "+dateText, sourceURL)
		addEvidence(&out.Evidence, "availability", rejection, sourceURL)
		out.Restrictions = append(out.Restrictions, rejection)
	} else if strings.Contains(dateText, "日付未定") || strings.Contains(charge, "円～") {
		return out, fmt.Errorf("jalan: dated plan response fell back to undated reference prices")
	}
	if out.Price.Amount == nil && out.Availability != "unavailable" {
		return out, fmt.Errorf("jalan: exact plan has neither a base quote nor explicit no-inventory evidence")
	}
	addEvidence(&out.Evidence, "plan_name", out.PlanName, sourceURL)
	addEvidence(&out.Evidence, "room_name", out.RoomName, sourceURL)
	addEvidence(&out.Evidence, "meals", mealText, sourceURL)
	for _, label := range labels {
		if strings.Contains(label, "禁煙") || strings.Contains(label, "喫煙") {
			addEvidence(&out.Evidence, "smoking", label, sourceURL)
		}
	}
	addEvidence(&out.Evidence, "check_in", out.CheckIn, sourceURL)
	addEvidence(&out.Evidence, "check_out", out.CheckOut, sourceURL)
	addEvidence(&out.Evidence, "availability", status, sourceURL)
	addEvidence(&out.Price.Evidence, "price.conditional_discount", out.Price.ConditionalDiscount, sourceURL)
	addEvidence(&out.Price.Evidence, "price.points", out.Price.Points, sourceURL)
	addEvidence(&out.Price.Evidence, "price.extra_fees", out.Fees, sourceURL)
	addEvidence(&out.Evidence, "cancellation", out.Cancellation, sourceURL)
	addEvidence(&out.Evidence, "payment", out.Payment, sourceURL)
	addEvidence(&out.Evidence, "booking_deadline", out.BookingDeadline, sourceURL)
	return out, nil
}
