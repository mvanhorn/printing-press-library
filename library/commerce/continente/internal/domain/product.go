package domain

type Product struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Brand              string   `json:"brand,omitempty"`
	SKU                string   `json:"sku,omitempty"`
	MPN                string   `json:"mpn,omitempty"`
	Category           string   `json:"category,omitempty"`
	Categories         []string `json:"categories,omitempty"`
	URL                string   `json:"url,omitempty"`
	Image              string   `json:"image,omitempty"`
	Availability       string   `json:"availability,omitempty"`
	RatingValue        float64  `json:"rating_value,omitempty"`
	RatingCount        int      `json:"rating_count,omitempty"`
	NutritionalInfoURL string   `json:"nutritional_info_url,omitempty"`
	NutritionStatus    string   `json:"nutrition_status,omitempty"`
	Price              Price    `json:"price,omitempty"`
	MissingFields      []string `json:"missing_fields,omitempty"`
}

func (p Product) LegacyPrice() float64 {
	if p.Price.DisplayAmount != 0 {
		return p.Price.DisplayAmount
	}
	if p.Price.Effective != nil {
		return *p.Price.Effective
	}
	return 0
}
