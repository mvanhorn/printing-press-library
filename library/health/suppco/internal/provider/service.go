// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.
// Handwritten SuppCo provider service; retained through regeneration in .printing-press-patches.

// Package provider performs bounded read-only SuppCo extraction and projects
// only the fields owned by this provider package.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/regimen"
)

const compactPath = "/api/users/me_compact/"

type Reader interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}

type Projection[T any] struct {
	Data       T                   `json:"data"`
	Provenance regimen.Observation `json:"provenance"`
}

type Service struct {
	reader Reader
	now    func() time.Time
}

func New(reader Reader) *Service {
	return NewWithClock(reader, time.Now)
}

func NewWithClock(reader Reader, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{reader: reader, now: now}
}

type compactResponse struct {
	Products json.RawMessage `json:"products"`
}

type compactProduct struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	ServingSize string          `json:"serving_size"`
	Ingredients json.RawMessage `json:"ingredients"`
}

type compactIngredient struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Amount   *float64        `json:"amount"`
	Units    *string         `json:"units"`
	Ancestry json.RawMessage `json:"ancestry"`
}

type scheduleResponse struct {
	Date       string                  `json:"date"`
	Activities *[]scheduleActivityWire `json:"activities"`
}

type scheduleActivityWire struct {
	Activity string                  `json:"activity"`
	Products *[]scheduledProductWire `json:"products"`
	Reminder *scheduleReminderWire   `json:"reminder"`
}

type scheduleReminderWire struct {
	Enabled *bool `json:"enabled"`
}

type scheduledProductWire struct {
	ProductID              string    `json:"product_id"`
	ProductName            string    `json:"product_name"`
	ProductBrand           string    `json:"product_brand"`
	ServingSizeAmount      *float64  `json:"serving_size_amount"`
	ServingSizeAmountTaken *float64  `json:"serving_size_amount_taken"`
	ServingUnit            string    `json:"serving_unit"`
	ScheduleType           string    `json:"schedule_type"`
	ScheduleDays           *[]string `json:"schedule_days"`
	EveryOtherDayStart     *string   `json:"every_other_day_start"`
	AsNeeded               *bool     `json:"as_needed"`
	ScheduledNotToday      *bool     `json:"scheduled_not_today"`
	WithFood               *bool     `json:"with_food"`
	ScheduleNotes          *string   `json:"schedule_notes"`
}

func (s *Service) Products(ctx context.Context) (Projection[[]regimen.Product], error) {
	compact, err := s.fetchCompact(ctx)
	if err != nil {
		return Projection[[]regimen.Product]{}, err
	}
	products, err := decodeProducts(compact.Products)
	if err != nil {
		return Projection[[]regimen.Product]{}, err
	}
	normalized, _, err := regimen.NormalizeStack(products, nil)
	if err != nil {
		return Projection[[]regimen.Product]{}, fmt.Errorf("project stack products: %w", err)
	}
	return Projection[[]regimen.Product]{Data: normalized, Provenance: s.observation(compactPath)}, nil
}

func (s *Service) Nutrients(ctx context.Context) (Projection[[]regimen.Component], error) {
	compact, err := s.fetchCompact(ctx)
	if err != nil {
		return Projection[[]regimen.Component]{}, err
	}
	products, nutrients, err := decodeStack(compact)
	if err != nil {
		return Projection[[]regimen.Component]{}, err
	}
	_, components, err := regimen.NormalizeStack(products, nutrients)
	if err != nil {
		return Projection[[]regimen.Component]{}, fmt.Errorf("project stack nutrients: %w", err)
	}
	return Projection[[]regimen.Component]{Data: components, Provenance: s.observation(compactPath)}, nil
}

func (s *Service) Schedule(ctx context.Context, date string) (Projection[regimen.ProviderSchedule], error) {
	if err := ValidateDate(date); err != nil {
		return Projection[regimen.ProviderSchedule]{}, err
	}
	schedule, path, err := s.fetchSchedule(ctx, date)
	if err != nil {
		return Projection[regimen.ProviderSchedule]{}, err
	}
	return Projection[regimen.ProviderSchedule]{Data: schedule, Provenance: s.observation(path)}, nil
}

func (s *Service) Snapshot(ctx context.Context, date string) (regimen.Snapshot, error) {
	if err := ValidateDate(date); err != nil {
		return regimen.Snapshot{}, err
	}
	compact, err := s.fetchCompact(ctx)
	if err != nil {
		return regimen.Snapshot{}, err
	}
	products, nutrients, err := decodeStack(compact)
	if err != nil {
		return regimen.Snapshot{}, err
	}
	stackObservation := s.observation(compactPath)

	schedule, schedulePath, err := s.fetchSchedule(ctx, date)
	if err != nil {
		return regimen.Snapshot{}, err
	}
	scheduleObservation := s.observation(schedulePath)
	return regimen.Build(s.timestamp(), stackObservation, scheduleObservation, products, nutrients, schedule)
}

func ValidateDate(date string) error {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil || parsed.Format("2006-01-02") != date {
		return fmt.Errorf("date must use YYYY-MM-DD format")
	}
	return nil
}

func (s *Service) fetchCompact(ctx context.Context) (compactResponse, error) {
	if s == nil || s.reader == nil {
		return compactResponse{}, fmt.Errorf("SuppCo provider reader is required")
	}
	raw, err := s.reader.Get(ctx, compactPath, nil)
	if err != nil {
		return compactResponse{}, sanitizeReaderError("read SuppCo stack", err)
	}
	var compact compactResponse
	if err := json.Unmarshal(raw, &compact); err != nil {
		return compactResponse{}, fmt.Errorf("decode SuppCo stack response: %w", err)
	}
	return compact, nil
}

func decodeProducts(raw json.RawMessage) ([]regimen.Product, error) {
	rows, err := decodeCompactProducts(raw)
	if err != nil {
		return nil, err
	}
	return projectProducts(rows), nil
}

func decodeCompactProducts(raw json.RawMessage) ([]compactProduct, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("SuppCo stack response is missing products")
	}
	var rows []compactProduct
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode SuppCo stack products: %w", err)
	}
	if rows == nil {
		rows = []compactProduct{}
	}
	return rows, nil
}

func projectProducts(rows []compactProduct) []regimen.Product {
	products := make([]regimen.Product, len(rows))
	for index, row := range rows {
		products[index] = regimen.Product{ID: row.ID, Name: row.Name, ServingSize: row.ServingSize}
	}
	return products
}

func decodeStack(compact compactResponse) ([]regimen.Product, []regimen.Nutrient, error) {
	rows, err := decodeCompactProducts(compact.Products)
	if err != nil {
		return nil, nil, err
	}
	products := projectProducts(rows)
	nutrients := make([]regimen.Nutrient, 0)
	for _, product := range rows {
		if len(product.Ingredients) == 0 || string(product.Ingredients) == "null" {
			return nil, nil, fmt.Errorf("SuppCo stack product is missing ingredients")
		}
		var ingredients []compactIngredient
		if err := json.Unmarshal(product.Ingredients, &ingredients); err != nil {
			return nil, nil, fmt.Errorf("decode SuppCo stack product ingredients: %w", err)
		}
		if ingredients == nil {
			ingredients = []compactIngredient{}
		}
		for _, ingredient := range ingredients {
			if ingredient.ID == "" || ingredient.Name == "" || ingredient.Amount == nil || ingredient.Units == nil || len(ingredient.Ancestry) == 0 {
				return nil, nil, fmt.Errorf("every SuppCo product ingredient requires id, name, amount, units, and ancestry")
			}
			parentID, err := decodeParentID(ingredient.Ancestry)
			if err != nil {
				return nil, nil, err
			}
			nutrients = append(nutrients, regimen.Nutrient{
				ID: ingredient.ID, ProductID: product.ID, Name: ingredient.Name,
				Amount: *ingredient.Amount, Unit: *ingredient.Units, ParentID: parentID,
			})
		}
	}
	return products, nutrients, nil
}

func decodeParentID(raw json.RawMessage) (string, error) {
	if string(raw) == "null" {
		return "", nil
	}
	var ancestry string
	if err := json.Unmarshal(raw, &ancestry); err != nil {
		return "", fmt.Errorf("decode SuppCo product ingredient ancestry: %w", err)
	}
	parts := strings.Split(ancestry, "/")
	for index := len(parts) - 1; index >= 0; index-- {
		if parts[index] != "" {
			return parts[index], nil
		}
	}
	return "", fmt.Errorf("SuppCo product ingredient ancestry is empty")
}

func (s *Service) fetchSchedule(ctx context.Context, date string) (regimen.ProviderSchedule, string, error) {
	path := "/api/schedules/" + date
	if s == nil || s.reader == nil {
		return regimen.ProviderSchedule{}, path, fmt.Errorf("SuppCo provider reader is required")
	}
	raw, err := s.reader.Get(ctx, path, nil)
	if err != nil {
		return regimen.ProviderSchedule{}, path, sanitizeReaderError("read SuppCo schedule", err)
	}
	var response scheduleResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return regimen.ProviderSchedule{}, path, fmt.Errorf("decode SuppCo schedule response: %w", err)
	}
	if response.Date == "" || response.Date != date {
		return regimen.ProviderSchedule{}, path, fmt.Errorf("SuppCo schedule date does not match requested date")
	}
	if response.Activities == nil {
		return regimen.ProviderSchedule{}, path, fmt.Errorf("SuppCo schedule response is missing activities")
	}
	schedule := regimen.ProviderSchedule{Date: response.Date, Activities: make([]regimen.ScheduleActivity, len(*response.Activities))}
	for activityIndex, activity := range *response.Activities {
		if activity.Activity == "" || activity.Products == nil || activity.Reminder == nil || activity.Reminder.Enabled == nil {
			return regimen.ProviderSchedule{}, path, fmt.Errorf("every SuppCo schedule activity requires activity, products, and reminder.enabled")
		}
		schedule.Activities[activityIndex] = regimen.ScheduleActivity{
			Activity: activity.Activity,
			Reminder: regimen.ScheduleReminder{Enabled: *activity.Reminder.Enabled},
			Products: make([]regimen.ScheduledProduct, len(*activity.Products)),
		}
		for productIndex, product := range *activity.Products {
			if product.ProductID == "" || product.ProductName == "" || strings.TrimSpace(product.ProductBrand) == "" || product.ServingSizeAmount == nil || product.ServingSizeAmountTaken == nil || product.ServingUnit == "" || product.ScheduleType == "" || product.ScheduleDays == nil || product.AsNeeded == nil || product.ScheduledNotToday == nil || product.WithFood == nil {
				return regimen.ProviderSchedule{}, path, fmt.Errorf("SuppCo scheduled product is missing required fields")
			}
			schedule.Activities[activityIndex].Products[productIndex] = regimen.ScheduledProduct{
				ProductID: product.ProductID, ProductName: product.ProductName, ProductBrand: product.ProductBrand,
				ServingSizeAmount: *product.ServingSizeAmount, ServingSizeAmountTaken: *product.ServingSizeAmountTaken,
				ServingUnit: product.ServingUnit, ScheduleType: product.ScheduleType,
				ScheduleDays: *product.ScheduleDays, EveryOtherDayStart: product.EveryOtherDayStart,
				AsNeeded: *product.AsNeeded, ScheduledNotToday: *product.ScheduledNotToday, WithFood: *product.WithFood,
				ScheduleNotes: product.ScheduleNotes,
			}
		}
	}
	normalized, err := regimen.NormalizeProviderSchedule(schedule)
	if err != nil {
		return regimen.ProviderSchedule{}, path, fmt.Errorf("project SuppCo schedule: %w", err)
	}
	return normalized, path, nil
}

func (s *Service) observation(path string) regimen.Observation {
	return regimen.Observation{Provider: "suppco", Path: path, ObservedAt: s.timestamp()}
}

func (s *Service) timestamp() string {
	return s.now().UTC().Format(time.RFC3339Nano)
}

type readerError struct {
	action string
	cause  error
}

func (e *readerError) Error() string { return e.action + ": provider request failed" }
func (e *readerError) Unwrap() error { return e.cause }

// sanitizeReaderError keeps provider-controlled response bodies out of CLI and
// MCP error text while retaining enough typed status information for exit-code
// classification. Non-HTTP causes remain available to errors.Is/As without
// becoming part of the displayed message.
func sanitizeReaderError(action string, err error) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s: %w", action, &client.APIError{
			Method: apiErr.Method, Path: apiErr.Path, StatusCode: apiErr.StatusCode,
		})
	}
	return &readerError{action: action, cause: err}
}
