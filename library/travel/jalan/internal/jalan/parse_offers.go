package jalan

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func ParseOffers(doc, sourceURL, propertyID string) (OfferPage, error) {
	out := OfferPage{Items: []Offer{}, Warnings: []string{}}
	root, err := parseHTML(doc)
	if err != nil {
		return out, err
	}
	count := parseCount(classText(idNode(root, "planlist-header"), "volume"))
	if count == nil {
		count = parseCount(textOf(idNode(root, "planlist-header")))
	}
	out.PlanTotal = count
	out.HasNext = nextPage(root)
	seen := map[string]bool{}
	for _, card := range classNodes(root, "p-planCassette") {
		planID := attr(card, "data-plancode")
		planName := classText(card, "p-searchResultItem__catchPhrase")
		mealText := classText(card, "p-mealType__value")
		basisText := classText(card, "p-searchResultItem__headCell--total")
		for _, row := range classNodes(card, "js-searchYadoRoomPlanCd") {
			m := offerIDRE.FindStringSubmatch(attr(row, "id"))
			if len(m) != 4 || m[1] != propertyID || (planID != "" && m[2] != planID) {
				return out, fmt.Errorf("jalan: offer identity does not match requested property/plan")
			}
			key := m[1] + "/" + m[2] + "/" + m[3]
			if seen[key] {
				continue
			}
			seen[key] = true
			o := emptyOffer(propertyID, sourceURL)
			o.PlanID = m[2]
			o.RoomID = m[3]
			if planName != "" {
				o.PlanName = planName
			}
			link := findFirst(row, func(n *html.Node) bool {
				return n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "p-searchResultItem__planName")
			})
			if link == nil {
				return out, fmt.Errorf("jalan: offer has no room-detail link")
			}
			o.RoomName = textOf(link)
			if o.RoomName == "" {
				o.RoomName = "unknown"
			}
			if u := resolveURL(sourceURL, attr(link, "href")); u != "" {
				o.URL = u
			}
			labels := []string{}
			for _, n := range classNodes(row, "p-searchResultItem__horizontalLabel") {
				labels = append(labels, textOf(n))
			}
			o.Baths = roomBaths(append([]string{o.RoomName}, labels...), sourceURL)
			o.Smoking = smoking(append([]string{o.RoomName}, labels...))
			o.Meals = meals(mealText)
			o.Price = priceQuote(classText(row, "p-searchResultItem__total"), basisText, sourceURL)
			o.Price.ConditionalDiscount = classText(row, "jlnpc-coupon-inner")
			o.Price.Points = classText(row, "p-searchResultItem__point")
			addEvidence(&o.Price.Evidence, "price.conditional_discount", o.Price.ConditionalDiscount, sourceURL)
			addEvidence(&o.Price.Evidence, "price.points", o.Price.Points, sourceURL)
			status := classText(row, "p-searchResultItem__rest")
			if status == "" {
				for sibling := row.NextSibling; sibling != nil; sibling = sibling.NextSibling {
					if sibling.Type != html.ElementNode {
						continue
					}
					if sibling.Data == "tr" && !hasClass(sibling, "js-searchYadoRoomPlanCd") {
						status = classText(sibling, "p-searchResultItem__rest")
					}
					break
				}
			}
			if strings.Contains(status, "空室なし") || strings.Contains(status, "満室") || strings.Contains(status, "売り切れ") {
				o.Availability = "unavailable"
			} else if strings.Contains(status, "空室") || strings.Contains(status, "部屋") {
				o.Availability = "available"
			}
			addEvidence(&o.Evidence, "plan_name", o.PlanName, sourceURL)
			addEvidence(&o.Evidence, "room_name", o.RoomName, sourceURL)
			addEvidence(&o.Evidence, "meals", mealText, sourceURL)
			addEvidence(&o.Evidence, "availability", status, sourceURL)
			for _, s := range labels {
				if strings.Contains(s, "禁煙") || strings.Contains(s, "喫煙") {
					addEvidence(&o.Evidence, "smoking", s, sourceURL)
				}
			}
			out.Items = append(out.Items, o)
		}
	}
	if len(out.Items) == 0 {
		message := textOf(idNode(root, "check-infobox-txt01"))
		if strings.Contains(message, "設定された条件でご利用できるプランがない") {
			z := 0
			out.Total = &z
			out.NoResults = true
			out.HasNext = false
			out.Warnings = append(out.Warnings, excerpt(message))
			return out, nil
		}
		if count != nil && *count == 0 {
			z := 0
			out.Total = &z
			out.NoResults = true
			out.HasNext = false
			return out, nil
		}
		return out, fmt.Errorf("jalan: response is not a recognized property offer page")
	}
	if count != nil {
		out.Warnings = append(out.Warnings, "source total counts plans, not plan/room offers; offer total is unknown")
	}
	return out, nil
}
