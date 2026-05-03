package hostextract

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/vrbo"
)

type HostInfo struct {
	Name       string  `json:"name,omitempty"`
	Brand      string  `json:"brand,omitempty"`
	Type       string  `json:"type,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Source     string  `json:"source,omitempty"`
}

var brandPattern = regexp.MustCompile(`[A-Z][A-Za-z]+(?:\s+[A-Z][A-Za-z]+)?\s+(Getaways|Rentals|Vacation|Properties|Cabins|Retreats|Stays|Homes)`)
var titleByPattern = regexp.MustCompile(`\bby\s+([A-Z][A-Za-z0-9]+(?:\s+[A-Z][A-Za-z0-9]+){0,3})`)
var knownPMCs = []string{"Vacasa", "Evolve", "Turnkey", "RedAwning", "AvantStay", "Hostfully", "Lodgify"}

func FromAirbnbListing(l *airbnb.Listing) *HostInfo {
	if l == nil {
		return nil
	}
	if h := fromPMC(l.PropertyManagementName); h != nil {
		return h
	}
	for _, badge := range l.Badges {
		if h := fromPMC(badge); h != nil {
			return h
		}
	}
	if h := fromDisplay(l.HostName); h != nil {
		return h
	}
	if h := fromBio(l.Description + "\n" + l.HostBio); h != nil {
		return h
	}
	if h := fromTitle(l.Title); h != nil {
		return h
	}
	return plain(l.HostName)
}

func FromVRBOProperty(p *vrbo.Property) *HostInfo {
	if p == nil {
		return nil
	}
	if h := fromPMC(p.PropertyManagementName); h != nil {
		return h
	}
	if h := fromDisplay(p.HostName); h != nil {
		return h
	}
	if h := fromBio(p.Description + "\n" + p.HostBio); h != nil {
		return h
	}
	if h := fromTitle(p.Title); h != nil {
		return h
	}
	return plain(p.HostName)
}

func fromPMC(name string) *HostInfo {
	name = clean(name)
	if name == "" {
		return nil
	}
	for _, marker := range []string{"managed by", "property management", "hosted by"} {
		name = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(name), marker))
	}
	return &HostInfo{Name: name, Brand: name, Type: "pmc", Confidence: 0.95, Source: "property_management"}
}

func fromDisplay(name string) *HostInfo {
	name = clean(name)
	if name == "" {
		return nil
	}
	if brandPattern.MatchString(name) {
		brand := brandPattern.FindString(name)
		return &HostInfo{Name: brand, Brand: brand, Type: "pmc", Confidence: 0.85, Source: "host_display_name"}
	}
	return nil
}

func fromBio(text string) *HostInfo {
	lower := strings.ToLower(text)
	for _, pmc := range knownPMCs {
		if strings.Contains(lower, strings.ToLower(pmc)) {
			return &HostInfo{Name: pmc, Brand: pmc, Type: "pmc", Confidence: 0.7, Source: "description_bio"}
		}
	}
	return nil
}

func fromTitle(title string) *HostInfo {
	if m := titleByPattern.FindStringSubmatch(title); len(m) > 1 {
		name := clean(m[1])
		return &HostInfo{Name: name, Brand: name, Type: "pmc", Confidence: 0.6, Source: "title_brand"}
	}
	return nil
}

func plain(name string) *HostInfo {
	name = clean(name)
	if name == "" {
		return &HostInfo{Type: "individual", Confidence: 0}
	}
	return &HostInfo{Name: name, Type: "individual", Confidence: 0.4, Source: "host_display_name"}
}

func clean(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
