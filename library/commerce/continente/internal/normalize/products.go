package normalize

import (
	"fmt"
	"strings"

	"continente-pp-cli/internal/domain"
)

type StorefrontProductRecord struct {
	ID                 string
	Name               string
	Brand              string
	SKU                string
	MPN                string
	Category           string
	Categories         []string
	URL                string
	Image              string
	DisplayPrice       float64
	OriginalPrice      float64
	DiscountAmount     float64
	UnitPrice          float64
	UnitLabel          string
	PackLabel          string
	PromotionText      []string
	Currency           string
	Availability       string
	RatingValue        float64
	RatingCount        int
	NutritionalInfoURL string
}

func ProductFromStorefront(record StorefrontProductRecord) (domain.Product, error) {
	if strings.TrimSpace(record.ID) == "" {
		return domain.Product{}, fmt.Errorf("normalize storefront product: missing id")
	}
	if strings.TrimSpace(record.Name) == "" {
		return domain.Product{}, fmt.Errorf("normalize storefront product: missing name")
	}

	out := domain.Product{
		ID:                 strings.TrimSpace(record.ID),
		Name:               strings.TrimSpace(record.Name),
		Brand:              strings.TrimSpace(record.Brand),
		SKU:                strings.TrimSpace(record.SKU),
		MPN:                strings.TrimSpace(record.MPN),
		Category:           strings.TrimSpace(record.Category),
		Categories:         trimNonEmpty(record.Categories),
		URL:                strings.TrimSpace(record.URL),
		Image:              strings.TrimSpace(record.Image),
		Availability:       strings.TrimSpace(record.Availability),
		RatingValue:        record.RatingValue,
		RatingCount:        record.RatingCount,
		NutritionalInfoURL: strings.TrimSpace(record.NutritionalInfoURL),
		Price: domain.Price{
			DisplayAmount: record.DisplayPrice,
			UnitLabel:     strings.TrimSpace(record.UnitLabel),
			PackLabel:     strings.TrimSpace(record.PackLabel),
			Currency:      strings.TrimSpace(record.Currency),
			Confidence:    "storefront_display",
			PromotionText: trimNonEmpty(record.PromotionText),
		},
	}
	if out.Price.DisplayAmount != 0 {
		out.Price.Effective = &out.Price.DisplayAmount
	}
	if record.OriginalPrice != 0 {
		out.Price.OriginalAmount = floatPtr(record.OriginalPrice)
	}
	if record.DiscountAmount != 0 {
		out.Price.DiscountAmount = floatPtr(record.DiscountAmount)
	}
	if record.UnitPrice != 0 {
		out.Price.UnitAmount = floatPtr(record.UnitPrice)
	}
	if out.Brand == "" {
		out.MissingFields = append(out.MissingFields, "brand")
	}
	if out.Image == "" {
		out.MissingFields = append(out.MissingFields, "image")
	}
	if len(out.Categories) == 0 && out.Category != "" {
		out.Categories = splitCategoryPath(out.Category)
	}
	if out.Category == "" && len(out.Categories) > 0 {
		out.Category = strings.Join(out.Categories, "/")
	}
	out.Price = PriceSummary(out)
	return out, nil
}

func splitCategoryPath(raw string) []string {
	if raw == "" {
		return nil
	}
	return trimNonEmpty(strings.Split(raw, "/"))
}

func trimNonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func floatPtr(v float64) *float64 {
	return &v
}
