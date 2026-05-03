package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/vrbo"
)

type Fingerprint struct {
	Hash       string            `json:"hash"`
	Components map[string]string `json:"components"`
}

func FromAirbnb(l *airbnb.Listing) *Fingerprint {
	c := map[string]string{}
	if l != nil {
		c["city"] = norm(l.City)
		if l.Coordinate != nil {
			c["lat"] = fmt.Sprintf("%.3f", round(l.Coordinate.Latitude))
			c["lng"] = fmt.Sprintf("%.3f", round(l.Coordinate.Longitude))
		}
		c["beds"] = fmt.Sprintf("%d", l.Beds)
		c["baths"] = fmt.Sprintf("%.1f", l.Baths)
		c["sleeps_max"] = fmt.Sprintf("%d", l.SleepsMax)
	}
	return build(c)
}

func FromVRBO(p *vrbo.Property) *Fingerprint {
	c := map[string]string{}
	if p != nil {
		c["city"] = norm(p.City)
		if p.Coordinate != nil {
			c["lat"] = fmt.Sprintf("%.3f", round(p.Coordinate.Latitude))
			c["lng"] = fmt.Sprintf("%.3f", round(p.Coordinate.Longitude))
		}
		c["beds"] = fmt.Sprintf("%d", p.Beds)
		c["baths"] = fmt.Sprintf("%.1f", p.Baths)
		c["sleeps_max"] = fmt.Sprintf("%d", p.SleepsMax)
	}
	return build(c)
}

func build(c map[string]string) *Fingerprint {
	parts := []string{c["city"], c["lat"], c["lng"], c["beds"], c["baths"], c["sleeps_max"]}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return &Fingerprint{Hash: hex.EncodeToString(sum[:]), Components: c}
}

func norm(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func round(f float64) float64 {
	return math.Round(f*1000) / 1000
}
