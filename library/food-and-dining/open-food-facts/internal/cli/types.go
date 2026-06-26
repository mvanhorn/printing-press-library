// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"
)

const (
	baseURLEnv      = "OPEN_FOOD_FACTS_BASE_URL"
	userAgentEnv    = "OPEN_FOOD_FACTS_USER_AGENT"
	contactEmailEnv = "OPEN_FOOD_FACTS_CONTACT_EMAIL"
	defaultBaseURL  = "https://world.openfoodfacts.org"
)

type config struct {
	BaseURL      string
	UserAgent    string
	ContactEmail string
}

func currentConfig() config {
	baseURL := strings.TrimRight(os.Getenv(baseURLEnv), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	contact := strings.TrimSpace(os.Getenv(contactEmailEnv))
	userAgent := strings.TrimSpace(os.Getenv(userAgentEnv))
	if userAgent == "" {
		if contact != "" {
			userAgent = fmt.Sprintf("open-food-facts-pp-cli/%s (%s)", version, contact)
		} else {
			userAgent = fmt.Sprintf("open-food-facts-pp-cli/%s (https://github.com/mvanhorn/printing-press-library)", version)
		}
	} else if contact != "" && !strings.Contains(userAgent, contact) {
		userAgent = fmt.Sprintf("%s (%s)", userAgent, contact)
	}
	return config{BaseURL: baseURL, UserAgent: userAgent, ContactEmail: contact}
}

type productResponse struct {
	Code    string        `json:"code"`
	Status  string        `json:"status"`
	Product productRecord `json:"product"`
	Errors  []apiError    `json:"errors"`
}

type searchResponse struct {
	Count    int             `json:"count"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Products []productRecord `json:"products"`
}

type apiError struct {
	Message string `json:"message"`
}

type productRecord struct {
	Code                string         `json:"code"`
	ProductName         string         `json:"product_name"`
	Brands              string         `json:"brands"`
	Quantity            string         `json:"quantity"`
	ServingSize         string         `json:"serving_size"`
	CategoriesTags      []string       `json:"categories_tags"`
	LabelsTags          []string       `json:"labels_tags"`
	CountriesTags       []string       `json:"countries_tags"`
	NutriScoreGrade     string         `json:"nutriscore_grade"`
	NovaGroup           int            `json:"nova_group"`
	EcoScoreGrade       string         `json:"ecoscore_grade"`
	IngredientsText     string         `json:"ingredients_text"`
	IngredientsTags     []string       `json:"ingredients_tags"`
	IngredientsAnalysis []string       `json:"ingredients_analysis_tags"`
	AllergensTags       []string       `json:"allergens_tags"`
	TracesTags          []string       `json:"traces_tags"`
	AdditivesTags       []string       `json:"additives_tags"`
	Nutriments          map[string]any `json:"nutriments"`
	DataQualityTags     []string       `json:"data_quality_tags"`
	ImageURL            string         `json:"image_url"`
	LastModified        int64          `json:"last_modified_t"`
}

type productSummary struct {
	Barcode             string         `json:"barcode"`
	Name                string         `json:"name,omitempty"`
	Brands              string         `json:"brands,omitempty"`
	Quantity            string         `json:"quantity,omitempty"`
	ServingSize         string         `json:"serving_size,omitempty"`
	Categories          []string       `json:"categories,omitempty"`
	Labels              []string       `json:"labels,omitempty"`
	Countries           []string       `json:"countries,omitempty"`
	NutriScoreGrade     string         `json:"nutriscore_grade,omitempty"`
	NovaGroup           int            `json:"nova_group,omitempty"`
	EcoScoreGrade       string         `json:"ecoscore_grade,omitempty"`
	IngredientsText     string         `json:"ingredients_text,omitempty"`
	Ingredients         []string       `json:"ingredients,omitempty"`
	IngredientsAnalysis []string       `json:"ingredients_analysis,omitempty"`
	Allergens           []string       `json:"allergens,omitempty"`
	Traces              []string       `json:"traces,omitempty"`
	Additives           []string       `json:"additives,omitempty"`
	Nutriments          map[string]any `json:"nutriments,omitempty"`
	DataQuality         []string       `json:"data_quality,omitempty"`
	ImageURL            string         `json:"image_url,omitempty"`
	SourceURL           string         `json:"source_url"`
}
