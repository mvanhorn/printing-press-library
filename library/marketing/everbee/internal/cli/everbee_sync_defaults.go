package cli

func applyEverbeeSyncDefaults(resource string, params map[string]string) {
	switch resource {
	case "folders":
		setDefaultParam(params, "type", "listing")
	case "keyword_research":
		setDefaultParam(params, "type_of_search", "keywords_suggestion")
		setDefaultParam(params, "order_by", "new_volume")
		setDefaultParam(params, "order_direction", "desc")
		setDefaultParam(params, "page", "1")
	case "product_analytics":
		setDefaultParam(params, "time_range", "last_1_month")
		setDefaultParam(params, "order_by", "est_mo_revenue")
		setDefaultParam(params, "order_direction", "desc")
		setDefaultParam(params, "page", "1")
		setDefaultParam(params, "per_page", "20")
	case "shops":
		setDefaultParam(params, "order_by", "revenue")
		setDefaultParam(params, "order_direction", "desc")
		setDefaultParam(params, "page", "1")
		setDefaultParam(params, "per_page", "20")
	}
}

func setDefaultParam(params map[string]string, key, value string) {
	if _, exists := params[key]; exists {
		return
	}
	params[key] = value
}
