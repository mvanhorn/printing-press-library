package jalan

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func ParseProperty(doc, sourceURL, propertyID string) (Property, error) {
	out := emptyProperty(propertyID, sourceURL)
	root, err := parseHTML(doc)
	if err != nil {
		return out, err
	}
	if idNode(root, "shisetsu-main01") == nil {
		return out, fmt.Errorf("jalan: response is not a recognized property details page")
	}
	header := idNode(root, "yado_header_740")
	nameNode := classNode(header, "yado-name")
	if nameNode == nil {
		nameNode = findFirst(root, func(n *html.Node) bool {
			return n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "yado-name")
		})
	}
	out.NameJa = textOf(nameNode)
	if out.NameJa == "" {
		return out, fmt.Errorf("jalan: property details have no Japanese name")
	}
	href := attr(nameNode, "href")
	if href != "" && !strings.Contains(href, "'"+propertyID+"'") && !strings.Contains(href, "\""+propertyID+"\"") && !strings.Contains(href, "yad"+propertyID) {
		return out, fmt.Errorf("jalan: property details identity does not match requested property")
	}
	for _, n := range classNodes(root, "yado-name") {
		if m := regexp.MustCompile(`^\[([^\]]+)\]`).FindStringSubmatch(textOf(n)); len(m) == 2 {
			out.LodgingType = m[1]
			break
		}
	}
	main := classNode(idNode(root, "shisetsu-main01"), "facility-overview-text")
	if main == nil {
		main = idNode(root, "shisetsu-main01")
	}
	descriptions := []string{}
	for _, p := range findAll(main, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "p" && !hasClass(n, "jlnpc-reservationBtn")
	}) {
		if s := textOf(p); s != "" {
			descriptions = append(descriptions, s)
		}
	}
	out.Description = strings.Join(descriptions, " ")
	accessRows := tablePairs(classNode(root, "jlnpc-shisetsu-accessparking-table"))
	out.Address = strings.TrimSpace(strings.ReplaceAll(accessRows["住所"], "大きな地図をみる", ""))
	out.Access = accessRows["アクセス"]
	out.Baths = propertyBaths(tablePairs(classNode(root, "shisetsu-bath_body")), sourceURL)
	roomRows := tablePairs(classNode(root, "shisetsu-roomsetsubi_body"))
	out.RoomBaths = roomBaths([]string{roomRows["部屋補足"], roomRows["標準的な部屋設備"]}, sourceURL)
	for _, table := range classNodes(root, "shisetsu-amenityspec_body") {
		for _, row := range findAll(table, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "tr" }) {
			cells := []string{}
			for c := row.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "td" {
					cells = append(cells, textOf(c))
				}
			}
			for i := 0; i+1 < len(cells); i += 2 {
				if cells[i+1] != "" && (cells[i] == "○" || cells[i] == "×" || cells[i] == "△") {
					out.Amenities = append(out.Amenities, cells[i]+" "+cells[i+1])
				}
			}
		}
	}
	review := classNode(root, "shisetsu-kuchikomi_sougou_body_wrap")
	if average := classText(review, "jlnpc-average-num"); average != "" {
		out.ReviewCategories["総合"] = average
	}
	labels := map[string]bool{"部屋": true, "風呂": true, "料理（朝食）": true, "料理（夕食）": true, "接客・サービス": true, "清潔感": true}
	categories := classNode(root, "jlnpc-shisetsu-kuchikomi-table")
	for _, row := range findAll(categories, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "tr" }) {
		var key string
		for c := row.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if c.Data == "th" && labels[textOf(c)] {
				key = textOf(c)
			} else if c.Data == "td" && key != "" {
				if s := textOf(c); s != "" {
					out.ReviewCategories[key] = s
				}
				key = ""
			}
		}
	}
	addEvidence(&out.Evidence, "name_ja", out.NameJa, sourceURL)
	addEvidence(&out.Evidence, "lodging_type", out.LodgingType, sourceURL)
	addEvidence(&out.Evidence, "address", out.Address, sourceURL)
	addEvidence(&out.Evidence, "access", out.Access, sourceURL)
	return out, nil
}
