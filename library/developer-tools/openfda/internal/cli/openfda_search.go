// OpenFDA search query builder.
// Translates CLI flag params into OpenFDA's Elasticsearch-style search= query syntax.

package cli

import (
	"fmt"
	"sort"
	"strings"
)

// searchField maps a CLI flag name to the OpenFDA API search field path.
type searchField struct {
	Flag  string
	Field string
	Type  string // "string", "boolean", "date_range_start", "date_range_end", "brand_or_generic"
}

// openFDASearchFields maps param names used by generated list commands to their
// OpenFDA field paths. Built from the spec's `in: search` / `field:` entries.
var openFDASearchFields = map[string]searchField{
	// drug-events
	"drug":       {Flag: "drug", Field: "patient.drug.medicinalproduct", Type: "brand_or_generic"},
	"reaction":   {Flag: "reaction", Field: "patient.reaction.reactionmeddrapt", Type: "string"},
	"serious":    {Flag: "serious", Field: "serious", Type: "boolean"},
	"outcome":    {Flag: "outcome", Field: "patient.reaction.reactionoutcome", Type: "string"},
	"country":    {Flag: "country", Field: "occurcountry", Type: "string"},
	"date_from":  {Flag: "date_from", Field: "receiptdate", Type: "date_range_start"},
	"date_to":    {Flag: "date_to", Field: "receiptdate", Type: "date_range_end"},
	"sex":        {Flag: "sex", Field: "patient.patientsex", Type: "string"},

	// drug-labels
	"brand_name":    {Flag: "brand_name", Field: "openfda.brand_name", Type: "string"},
	"generic_name":  {Flag: "generic_name", Field: "openfda.generic_name", Type: "string"},
	"manufacturer":  {Flag: "manufacturer", Field: "openfda.manufacturer_name", Type: "string"},
	"ingredient":    {Flag: "ingredient", Field: "spl_product_data_elements", Type: "string"},
	"indication":    {Flag: "indication", Field: "indications_and_usage", Type: "string"},
	"route":         {Flag: "route", Field: "openfda.route", Type: "string"},

	// drug-ndc
	"product_ndc":  {Flag: "product_ndc", Field: "product_ndc", Type: "string"},
	"ndc":          {Flag: "ndc", Field: "package_ndc", Type: "string"},
	"product_name": {Flag: "product_name", Field: "brand_name", Type: "string"},
	"dosage_form":  {Flag: "dosage_form", Field: "dosage_form", Type: "string"},

	// drug-recalls, device-recalls, food-recalls
	"firm":           {Flag: "firm", Field: "recalling_firm", Type: "string"},
	"classification": {Flag: "classification", Field: "classification", Type: "string"},
	"status":         {Flag: "status", Field: "status", Type: "string"},
	"reason":         {Flag: "reason", Field: "reason_for_recall", Type: "string"},
	"recall_from":    {Flag: "recall_from", Field: "report_date", Type: "date_range_start"},
	"recall_to":      {Flag: "recall_to", Field: "report_date", Type: "date_range_end"},

	// drug-approvals
	"sponsor":           {Flag: "sponsor", Field: "sponsor_name", Type: "string"},
	"brand":             {Flag: "brand", Field: "products.brand_name", Type: "string"},
	"generic":           {Flag: "generic", Field: "openfda.generic_name", Type: "string"},
	"application_type":  {Flag: "application_type", Field: "products.marketing_status", Type: "string"},

	// drug-shortages
	"product":        {Flag: "product", Field: "proprietary_name", Type: "string"},
	"shortage_status": {Flag: "shortage_status", Field: "status", Type: "string"},

	// device-events
	"device":           {Flag: "device", Field: "device.brand_name", Type: "string"},
	"device_manufacturer": {Flag: "device_manufacturer", Field: "device.manufacturer_d_name", Type: "string"},
	"event_type":       {Flag: "event_type", Field: "event_type", Type: "string"},
	"device_from":      {Flag: "device_from", Field: "date_received", Type: "date_range_start"},
	"device_to":        {Flag: "device_to", Field: "date_received", Type: "date_range_end"},

	// device-510k
	"applicant":        {Flag: "applicant", Field: "applicant", Type: "string"},
	"device_name":      {Flag: "device_name", Field: "device_name", Type: "string"},
	"product_code":     {Flag: "product_code", Field: "product_code", Type: "string"},
	"decision_code":    {Flag: "decision_code", Field: "decision_code", Type: "string"},

	// device-classification
	"class_name":       {Flag: "class_name", Field: "device_name", Type: "string"},
	"device_class":     {Flag: "device_class", Field: "device_class", Type: "string"},
	"medical_specialty": {Flag: "medical_specialty", Field: "medical_specialty_description", Type: "string"},

	// animal-events
	"animal":           {Flag: "animal", Field: "animal.species.name", Type: "string"},
	"animal_drug":      {Flag: "animal_drug", Field: "drug.active_ingredients.name", Type: "string"},
	"animal_reaction":  {Flag: "animal_reaction", Field: "reaction.veddra_term_name", Type: "string"},

	// food-events
	"food_product":     {Flag: "food_product", Field: "products.name_brand", Type: "string"},
	"food_reaction":    {Flag: "food_reaction", Field: "reactions", Type: "string"},
	"food_outcomes":    {Flag: "food_outcomes", Field: "outcomes", Type: "string"},

	// tobacco-problems
	"tobacco_product":  {Flag: "tobacco_product", Field: "tobacco_products", Type: "string"},
	"problem":          {Flag: "problem", Field: "reported_health_problems", Type: "string"},

	// substance
	"substance_name":   {Flag: "substance_name", Field: "names.name", Type: "string"},

	// device-pma
	"pma_applicant":    {Flag: "pma_applicant", Field: "applicant", Type: "string"},
	"pma_product":      {Flag: "pma_product", Field: "trade_name", Type: "string"},

	// device-recall-detail
	"recall_device":    {Flag: "recall_device", Field: "product_description", Type: "string"},

	// device-registration
	"reg_name":         {Flag: "reg_name", Field: "products.proprietary_name", Type: "string"},

	// device-udi
	"udi_device":       {Flag: "udi_device", Field: "brand_name", Type: "string"},

	// device-covid19
	"covid_manufacturer": {Flag: "covid_manufacturer", Field: "manufacturer", Type: "string"},

	// nsde
	"nsde_name":        {Flag: "nsde_name", Field: "proprietary_name", Type: "string"},
}

// buildOpenFDAParams converts CLI flag params into OpenFDA API query params.
// Params with keys found in openFDASearchFields are composed into a single
// "search" query param. Other params are passed through unchanged.
func buildOpenFDAParams(params map[string]string) map[string]string {
	result := make(map[string]string)
	var searchParts []string
	var dateRanges = make(map[string][2]string) // field -> [start, end]

	// Sort keys for deterministic output
	sortedKeys := make([]string, 0, len(params))
	for key := range params {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		val := params[key]
		if val == "" {
			continue
		}
		sf, ok := openFDASearchFields[key]
		if !ok {
			// Not a search field — pass through as-is (e.g., limit, skip, count).
			// Rename "field" → "count" for OpenFDA's count endpoint.
			outKey := key
			if key == "field" {
				outKey = "count"
			}
			result[outKey] = val
			continue
		}

		switch sf.Type {
		case "brand_or_generic":
			// Search both brand name (exact) and generic/medicinal product name.
			// Brand names are stored uppercase in OpenFDA; OR clause ensures both match.
			upper := strings.ToUpper(val)
			searchParts = append(searchParts, fmt.Sprintf(
				"(patient.drug.openfda.brand_name.exact:\"%s\"+OR+%s:\"%s\")",
				upper, sf.Field, val))
		case "boolean":
			// Boolean: serious=true -> serious:1
			if val == "true" {
				searchParts = append(searchParts, fmt.Sprintf("%s:1", sf.Field))
			}
		case "date_range_start":
			dr := dateRanges[sf.Field]
			dr[0] = val
			dateRanges[sf.Field] = dr
		case "date_range_end":
			dr := dateRanges[sf.Field]
			dr[1] = val
			dateRanges[sf.Field] = dr
		default:
			// String: drug=acetaminophen -> patient.drug.medicinalproduct:"acetaminophen"
			searchParts = append(searchParts, fmt.Sprintf("%s:\"%s\"", sf.Field, val))
		}
	}

	// Build date ranges
	dateKeys := make([]string, 0, len(dateRanges))
	for field := range dateRanges {
		dateKeys = append(dateKeys, field)
	}
	sort.Strings(dateKeys)
	for _, field := range dateKeys {
		dr := dateRanges[field]
		start, end := dr[0], dr[1]
		if start != "" && end != "" {
			searchParts = append(searchParts, fmt.Sprintf("%s:[%s+TO+%s]", field, start, end))
		} else if start != "" {
			searchParts = append(searchParts, fmt.Sprintf("%s:[%s+TO+%s]", field, start, "21000101"))
		} else if end != "" {
			searchParts = append(searchParts, fmt.Sprintf("%s:[%s+TO+%s]", field, "19000101", end))
		}
	}

	if len(searchParts) > 0 {
		result["search"] = strings.Join(searchParts, "+AND+")
	}

	return result
}
