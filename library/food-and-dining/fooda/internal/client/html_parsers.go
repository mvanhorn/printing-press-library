package client

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
)

func findPropsByID(htmlStr string, targetID string) (string, error) {
	doc, err := nethtml.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var propsVal string
	var found bool
	var f func(*nethtml.Node)
	f = func(n *nethtml.Node) {
		if found {
			return
		}
		if n.Type == nethtml.ElementNode {
			var idVal, propsValLocal string
			var hasID, hasProps bool
			for _, a := range n.Attr {
				if a.Key == "id" {
					idVal = a.Val
					hasID = true
				} else if a.Key == "props" {
					propsValLocal = a.Val
					hasProps = true
				}
			}
			if hasID && idVal == targetID && hasProps {
				propsVal = propsValLocal
				found = true
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	if !found {
		return "", fmt.Errorf("could not find element with id=%q and props attribute", targetID)
	}
	return propsVal, nil
}

type PastOrdersProps struct {
	Presenter []struct {
		Order struct {
			ID                     int      `json:"id"`
			EventType              string   `json:"event_type"`
			ItemNames              []string `json:"item_names"`
			OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
			VendorNames            []string `json:"vendor_names"`
			Status                 string   `json:"status"`
			Delivery               string   `json:"delivery"`
			RequestID              string   `json:"request_id"`
			PaymentCents           int      `json:"payment_cents"`
		} `json:"order"`
	} `json:"presenter"`
}

type OrderDetailProps struct {
	OrderData struct {
		FormattedVendorNames string `json:"formattedVendorNames"`
		Canceled             bool   `json:"canceled"`
		CanceledDate         string `json:"canceledDate"`
		ActivityDate         string `json:"activityDate"`
		LocationDetails      string `json:"locationDetails"`
		LocationSpot         string `json:"locationSpot"`
		OrderUuid            string `json:"orderUuid"`
		OrderItems           []struct {
			Name       string `json:"name"`
			OrderItems []struct {
				Quantity            int    `json:"quantity"`
				Name                string `json:"name"`
				Amount              string `json:"amount"`
				Refunded            bool   `json:"refunded"`
				SpecialInstructions string `json:"specialInstructions"`
				ItemOptions         []struct {
					Name   string `json:"name"`
					Amount string `json:"amount"`
				} `json:"itemOptions"`
			} `json:"orderItems"`
		} `json:"orderItems"`
		FormattedSubtotalAmount string `json:"formattedSubtotalAmount"`
		FormattedTaxAmount      string `json:"formattedTaxAmount"`
		FormattedSubsidyAmount  string `json:"formattedSubsidyAmount"`
		FormattedTotalAmount    string `json:"formattedTotalAmount"`
		LocalCardholderName     string `json:"localCardholderName"`
	} `json:"orderData"`
}

type MyEventListProps struct {
	Filters struct {
		AccountID []string `json:"account_id"`
		Locations struct {
			BuildingID []string `json:"building_id"`
		} `json:"locations"`
	} `json:"filters"`
	UserID int `json:"userId"`
}

type MenuItem struct {
	ID                  string   `json:"id"`
	VendorName          string   `json:"vendor_name"`
	Category            string   `json:"category"`
	Name                string   `json:"name"`
	Price               float64  `json:"price"`
	Description         string   `json:"description"`
	DietaryRestrictions []string `json:"dietary_restrictions"`
}

func ParsePastOrders(htmlStr string) (*PastOrdersProps, error) {
	propsVal, err := findPropsByID(htmlStr, "order")
	if err != nil {
		return nil, fmt.Errorf("could not find past orders props in HTML: %w", err)
	}
	var props PastOrdersProps
	if err := json.Unmarshal([]byte(propsVal), &props); err != nil {
		return nil, fmt.Errorf("failed to unmarshal past orders props: %w", err)
	}
	return &props, nil
}

func ParseOrderDetail(htmlStr string) (*OrderDetailProps, error) {
	propsVal, err := findPropsByID(htmlStr, "app_order_details")
	if err != nil {
		return nil, fmt.Errorf("could not find order details props in HTML: %w", err)
	}
	var props OrderDetailProps
	if err := json.Unmarshal([]byte(propsVal), &props); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order details props: %w", err)
	}
	return &props, nil
}

func ParseMyPage(htmlStr string) (string, string, error) {
	propsVal, err := findPropsByID(htmlStr, "my-event-list")
	if err != nil {
		acctReg := regexp.MustCompile(`"account_id"\s*:\s*\[\s*"([^"]+)"\s*\]`)
		bldgReg := regexp.MustCompile(`"building_id"\s*:\s*\[\s*"([^"]+)"\s*\]`)
		acctMatch := acctReg.FindStringSubmatch(htmlStr)
		bldgMatch := bldgReg.FindStringSubmatch(htmlStr)
		if len(acctMatch) >= 2 && len(bldgMatch) >= 2 {
			return acctMatch[1], bldgMatch[1], nil
		}
		return "", "", fmt.Errorf("could not find my-event-list props or fallback in HTML: %w", err)
	}
	var props MyEventListProps
	if err := json.Unmarshal([]byte(propsVal), &props); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal my event list props: %w", err)
	}
	acctID := ""
	if len(props.Filters.AccountID) > 0 {
		acctID = props.Filters.AccountID[0]
	}
	bldgID := ""
	if len(props.Filters.Locations.BuildingID) > 0 {
		bldgID = props.Filters.Locations.BuildingID[0]
	}
	return acctID, bldgID, nil
}

func ParseMenuItems(htmlStr string) ([]MenuItem, error) {
	doc, err := nethtml.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse menu items HTML: %w", err)
	}

	var items []MenuItem
	var f func(*nethtml.Node)
	f = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "div" {
			hasVendor := false
			var vendor, category, restrictions string
			for _, a := range n.Attr {
				if a.Key == "data-vendor_name" {
					vendor = a.Val
					hasVendor = true
				} else if a.Key == "data-category" {
					category = a.Val
				} else if a.Key == "data-dietary_restriction" {
					restrictions = a.Val
				}
			}
			if hasVendor {
				item := MenuItem{
					VendorName: vendor,
					Category:   category,
				}
				if restrictions != "" {
					parts := strings.Split(restrictions, ",")
					for _, p := range parts {
						p = strings.TrimSpace(p)
						if p != "" {
							item.DietaryRestrictions = append(item.DietaryRestrictions, p)
						}
					}
				}

				item.ID = findItemID(n)
				item.Name = findTextByClass(n, "item__name")
				priceStr := findTextByClass(n, "item__price")
				if priceStr != "" {
					priceStr = strings.ReplaceAll(priceStr, "$", "")
					if p, err := strconv.ParseFloat(priceStr, 64); err == nil {
						item.Price = p
					}
				}
				descFull := findTextByClass(n, "item__desc__text")
				vendorPrefix := findTextByClass(n, "item__desc__text__name")
				if vendorPrefix != "" {
					descFull = strings.TrimPrefix(descFull, vendorPrefix)
				}
				item.Description = strings.TrimSpace(descFull)

				items = append(items, item)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return items, nil
}

func findItemID(n *nethtml.Node) string {
	var val string
	var f func(*nethtml.Node)
	f = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode {
			for _, a := range node.Attr {
				if a.Key == "href" && strings.Contains(a.Val, "/items/") {
					parts := strings.Split(a.Val, "/items/")
					if len(parts) > 1 {
						itemPart := parts[1]
						if idx := strings.Index(itemPart, "?"); idx != -1 {
							itemPart = itemPart[:idx]
						}
						val = strings.TrimSpace(itemPart)
						return
					}
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
			if val != "" {
				return
			}
		}
	}
	f(n)
	return val
}

func findTextByClass(n *nethtml.Node, className string) string {
	var val string
	var f func(*nethtml.Node)
	f = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode {
			for _, a := range node.Attr {
				if a.Key == "class" && strings.Contains(a.Val, className) {
					val = getNodeText(node)
					return
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
			if val != "" {
				return
			}
		}
	}
	f(n)
	return val
}

func getNodeText(n *nethtml.Node) string {
	var b strings.Builder
	var f func(*nethtml.Node)
	f = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.TrimSpace(b.String())
}
