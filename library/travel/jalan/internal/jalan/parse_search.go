package jalan

import (
	"fmt"
	"strings"
)

func ParseSearch(doc, sourceURL string) (SearchPage, error) {
	out := SearchPage{Items: []Property{}, Warnings: []string{}}
	root, err := parseHTML(doc)
	if err != nil {
		return out, err
	}
	countNode := classNode(root, "jlnpc-planListCnt-header")
	out.Total = parseCount(textOf(countNode))
	out.HasNext = nextPage(root)
	seen := map[string]bool{}
	for _, card := range classNodes(root, "p-yadoCassette") {
		// Paid recommendations are outside the native, paginated search result.
		if hasClass(card, "p-yadoCassette--pr") || strings.HasPrefix(attr(card, "id"), "sa_") {
			continue
		}
		idMatch := propertyIDRE.FindStringSubmatch(attr(card, "id"))
		link := classNode(card, "jlnpc-yadoCassette__link")
		if len(idMatch) != 2 {
			idMatch = propertyIDRE.FindStringSubmatch(attr(link, "href"))
		}
		name := classText(card, "p-searchResultItem__facilityName")
		if len(idMatch) != 2 || name == "" {
			return out, fmt.Errorf("jalan: search result has no property identity or Japanese name")
		}
		id := idMatch[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		propertyURL := resolveURL(sourceURL, attr(link, "href"))
		if propertyURL == "" {
			propertyURL = "https://www.jalan.net/yad" + id + "/"
		}
		p := emptyProperty(id, propertyURL)
		p.NameJa = name
		p.Access = classText(card, "p-searchResultItem__accessValue")
		p.Description = classText(card, "p-searchResultItem__description")
		p.Price = priceQuote(classText(card, "p-searchResultItem__lowestPriceValue"), classText(card, "p-searchResultItem__lowestPriceHead"), sourceURL)
		if score := classText(card, "p-searchResultItem__summaryaverage-num"); score != "" {
			p.ReviewCategories["総合"] = score
		}
		addEvidence(&p.Evidence, "name_ja", name, sourceURL)
		addEvidence(&p.Evidence, "access", p.Access, sourceURL)
		addEvidence(&p.Evidence, "description", p.Description, sourceURL)
		out.Items = append(out.Items, p)
	}
	if len(out.Items) == 0 {
		if out.Total != nil && *out.Total == 0 {
			out.NoResults = true
			out.HasNext = false
			return out, nil
		}
		// Accept an explicit native empty-search message only inside the results.
		container := idNode(root, "search-result-list")
		if container == nil {
			container = idNode(root, "searchResultList")
		}
		if container != nil && explicitNoMatches(textOf(container)) {
			z := 0
			out.Total = &z
			out.NoResults = true
			out.HasNext = false
			return out, nil
		}
		return out, fmt.Errorf("jalan: response is not a recognized search results page")
	}
	return out, nil
}
func explicitNoMatches(s string) bool {
	return strings.Contains(s, "条件に該当する宿") && (strings.Contains(s, "ありません") || strings.Contains(s, "見つかりません")) || strings.Contains(s, "該当する宿泊施設がありません") || strings.Contains(s, "該当する宿泊プランがありません")
}
