package toyota

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type OptionFee struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	FeeJPY      int    `json:"fee_jpy"`
	Basis       string `json:"basis"`
	TaxIncluded bool   `json:"tax_included"`
	Condition   string `json:"condition,omitempty"`
	SourceURL   string `json:"source_url"`
}
type OptionsResult struct {
	Meta           Metadata    `json:"meta"`
	Options        []OptionFee `json:"options"`
	BasicInsurance string      `json:"basic_insurance"`
	Notes          []string    `json:"notes"`
	SourceURLs     []string    `json:"source_urls"`
}

var feeRE = regexp.MustCompile(`(?i)(?:JPY\s*|[¥￥])([0-9][0-9,]*)|([0-9][0-9,]*)\s*(?:yen|JPY)`)

func fees(s string) []int {
	out := []int{}
	for _, m := range feeRE.FindAllStringSubmatch(s, -1) {
		v := m[1]
		if v == "" {
			v = m[2]
		}
		if n := integer(v); n != nil {
			out = append(out, *n)
		}
	}
	return out
}

func optionSummary(doc *html.Node, label string) string {
	for _, a := range nodes(doc, func(n *html.Node) bool {
		return n.Data == "a" || strings.Contains(" "+attr(n, "class")+" ", " nolink ")
	}) {
		title := byClass(a, "linkTtl")
		if title == nil {
			title = byClass(a, "linkTtl2")
		}
		if title == nil {
			title = byClass(a, "linkTtl3")
		}
		if text(title) == label {
			contents := byClass(a, "linkTxt")
			if contents == nil {
				contents = byClass(a, "linkTxt2")
			}
			return text(contents)
		}
	}
	return ""
}

func ParseOptions(options, insurance []byte) (OptionsResult, error) {
	doc, err := parseHTML(options)
	if err != nil {
		return OptionsResult{}, err
	}
	ins, err := parseHTML(insurance)
	if err != nil {
		return OptionsResult{}, err
	}
	insText := text(byID(ins, "pageArea"))
	if insText == "" {
		insText = text(ins)
	}
	if err := requireToken(insText, "Insurance/Compensation is Included in the Rental Car's Basic Rate."); err != nil {
		return OptionsResult{}, err
	}
	out := OptionsResult{Options: []OptionFee{}, BasicInsurance: "Basic insurance/compensation is included in the basic rental rate; deductibles,exclusions and non-operation charges still apply.",
		SourceURLs: []string{Origin + OptionsPath, Origin + InsurancePath},
		Notes: []string{"Policy fees are separate from a confirmed rental quote. Do not infer their inclusion in a class-page price.",
			"ETC device and ETC card are distinct: the device is standard on passenger classes except some models/areas; the card is a paid rental and tolls are charged separately.",
			"All online passenger vehicles are non-smoking. Equipment,ETC/JAF cards and seats depend on shop/model stock.",
			"Children under six require an appropriate child/infant seat under Toyota's source guidance; fit depends on age,height,weight and vehicle. Toyota verifies suitability.",
			"Winter tires may be standard in certain regions/seasons with separate seasonal rates; the booking breakdown must be checked.",
			"Coverage is subject to exclusions and reporting requirements. Read the full insurance policy before deciding on a waiver."}}
	add := func(code, label string, index int, basis, condition string) error {
		f := fees(optionSummary(doc, label))
		if len(f) <= index {
			return &SourceError{"Toyota option fee changed or is missing for " + label + "; open " + Origin + OptionsPath}
		}
		out.Options = append(out.Options, OptionFee{Code: code, Label: label, FeeJPY: f[index], Basis: basis, TaxIncluded: true, Condition: condition, SourceURL: Origin + OptionsPath})
		return nil
	}
	for _, x := range []struct {
		code, label      string
		index            int
		basis, condition string
	}{
		{"etc_card", "ETC Card Rentals", 0, "per_rental", "Card availability is not guaranteed; toll charges are separate."},
		{"jaf", "JAF Membership Certificate Rental", 0, "per_rental", "Certificate availability is not guaranteed."},
		{"model_selection", "Model Selection Option", 0, "per_rental", "Selected model inventory must be confirmed; class-only offers do not guarantee a model."},
		{"model_selection_c5_w3_suv4", "Model Selection Option", 1, "per_rental", "Applies to C5,W3,SUV4 classes."},
		{"4wd_passenger", "4WD (Four-Wheel Drive Vehicle)*", 0, "per_24_hours", "SUV4 has 4WD included in its base rate."},
		{"4wd_bus", "4WD (Four-Wheel Drive Vehicle)*", 1, "per_24_hours", "Bus class policy only; not offered by the one-way simulator."},
		{"child_or_infant_seat", "Child Seats", 0, "per_rental_per_seat", "Choose an appropriate seat; shop/model stock and capacity apply."},
		{"booster", "Child Seats", 1, "per_rental_per_seat", "Junior/booster seat; shop/model stock and capacity apply."},
		{"winter_passenger_van", "Winter Tires", 0, "per_24_hours", "Excludes regions/periods with standard winter equipment and seasonal rates."},
		{"winter_wagon_suv", "Winter Tires", 1, "per_24_hours", "Excludes regions/periods with standard winter equipment and seasonal rates."},
	} {
		if err := add(x.code, x.label, x.index, x.basis, x.condition); err != nil {
			return OptionsResult{}, err
		}
	}
	daily := regexp.MustCompile(`JPY\s*([0-9][0-9,]*)(?:\s*\(tax included 10%\))?\s*/\s*24 hours`).FindAllStringSubmatch(insText, -1)
	if len(daily) < 2 {
		return OptionsResult{}, &SourceError{"Toyota waiver/NOC rate format changed; open " + Origin + InsurancePath}
	}
	noc := integer(daily[0][1])
	waiver := integer(daily[1][1])
	if noc == nil || waiver == nil {
		return OptionsResult{}, &SourceError{"Toyota waiver/NOC fees are not numeric"}
	}
	for _, x := range []struct {
		code, label string
		fee         int
		condition   string
	}{
		{"waiver", "Exclusion of Liability Compensation System", *waiver, "Passenger classes; buses/large trucks use the separate higher rate."},
		{"double_protection", "TOYOTA Rent a Car Double Protection Package", *waiver + *noc, "Combines the policy waiver and NOC exemption rates; subject to coverage exclusions."},
		{"noc_increment", "NOC exemption increment", *noc, "Added to the waiver as part of Double Protection; not a standalone quoted rental option."},
	} {
		out.Options = append(out.Options, OptionFee{Code: x.code, Label: x.label, FeeJPY: x.fee, Basis: "per_24_hours", TaxIncluded: true, Condition: x.condition, SourceURL: Origin + InsurancePath})
	}
	return out, nil
}

func (c *Client) Options(ctx context.Context) (OptionsResult, error) {
	options, _, _, err := c.request(ctx, OptionsPath, nil)
	if err != nil {
		return OptionsResult{}, err
	}
	insurance, _, _, err := c.request(ctx, InsurancePath, nil)
	if err != nil {
		return OptionsResult{}, err
	}
	result, err := ParseOptions(options, insurance)
	if err != nil {
		return OptionsResult{}, err
	}
	result.Meta = c.Meta()
	return result, nil
}

type EligibilityPathInfo struct {
	License    string   `json:"license"`
	Documents  []string `json:"documents"`
	Conditions []string `json:"conditions"`
}
type EligibilityResult struct {
	Meta                  Metadata              `json:"meta"`
	IndividualEligibility string                `json:"individual_eligibility"`
	Paths                 []EligibilityPathInfo `json:"license_paths"`
	Notes                 []string              `json:"notes"`
	SourceURLs            []string              `json:"source_urls"`
}

func ParseEligibility(data []byte) (EligibilityResult, error) {
	doc, err := parseHTML(data)
	if err != nil {
		return EligibilityResult{}, err
	}
	s := text(byID(doc, "pageArea"))
	for _, token := range []string{"Geneva Convention", "1949", "Switzerland", "German", "France", "Taiwan", "Belgium", "Monaco"} {
		if err := requireToken(s, token); err != nil {
			return EligibilityResult{}, err
		}
	}
	return EligibilityResult{IndividualEligibility: "not_assessed",
		Paths: []EligibilityPathInfo{
			{License: "Japanese driver's license", Documents: []string{"valid Japanese license"}, Conditions: []string{"Appropriate vehicle category and valid license required."}},
			{License: "1949 Geneva Convention International Driving Permit", Documents: []string{"qualifying booklet-type IDP", "passport with landing evidence"}, Conditions: []string{"Valid within one year of issue and the permitted period after landing,subject to Toyota's document verification.", "Correct vehicle-category stamp and an eligible issuing authority are required; other treaty-format permits are not inferred eligible."}},
			{License: "License issued in Switzerland,Germany,France,Taiwan,Belgium or Monaco", Documents: []string{"valid qualifying foreign license", "authorized Japanese translation", "passport with landing evidence"}, Conditions: []string{"Translation authority and period after landing must meet the source rules; Toyota verifies the actual documents."}},
		},
		Notes: []string{"This is source guidance,not a personal eligibility decision. Toyota may refuse a rental when documents do not meet its requirements.",
			"Obtain an entry/exit stamp when using an automated gate,or bring the accepted specific registrant card. Cruise arrivals need the accepted landing-period evidence.",
			"Resident-register/re-entry rules may prevent a short trip abroad from resetting the driving period. Read the qualification chart and source before departure.",
			"All drivers need the required documents and the correct vehicle class; no personal data is collected by this CLI."},
		SourceURLs: []string{Origin + EligibilityPath, Origin + "/global_eng/license-flow/"}}, nil
}

func (c *Client) Eligibility(ctx context.Context) (EligibilityResult, error) {
	data, _, _, err := c.request(ctx, EligibilityPath, nil)
	if err != nil {
		return EligibilityResult{}, err
	}
	out, err := ParseEligibility(data)
	if err != nil {
		return EligibilityResult{}, err
	}
	out.Meta = c.Meta()
	return out, nil
}

func (r OptionsResult) Find(code string) (OptionFee, error) {
	for _, o := range r.Options {
		if o.Code == code {
			return o, nil
		}
	}
	return OptionFee{}, fmt.Errorf("unknown option code %q", code)
}

// Keep title whitespace stable across the source's line breaks and tabs.
