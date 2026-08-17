// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.
// Handwritten SuppCo projection; retained through regeneration in .printing-press-patches.

// Package regimen owns the narrow provider snapshot contract. It preserves
// provider component relationships and never calculates nutrient exposure.
package regimen

import (
	"fmt"
	"sort"
	"strings"
)

const SchemaVersion = "suppco.regimen_snapshot.v1"

type Product struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ServingSize string `json:"serving_size,omitempty"`
}

type Nutrient struct {
	ID        string  `json:"id"`
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"`
	Unit      string  `json:"unit"`
	ParentID  string  `json:"parent_id,omitempty"`
}

type Component struct {
	ID           string   `json:"id"`
	ProductID    string   `json:"product_id"`
	Name         string   `json:"name"`
	Amount       float64  `json:"amount"`
	Unit         string   `json:"unit"`
	ParentID     string   `json:"parent_id,omitempty"`
	ComponentIDs []string `json:"component_ids"`
}

type ScheduledProduct struct {
	ProductID              string   `json:"product_id"`
	ProductName            string   `json:"product_name"`
	ProductBrand           string   `json:"product_brand"`
	ServingSizeAmount      float64  `json:"serving_size_amount"`
	ServingSizeAmountTaken float64  `json:"serving_size_amount_taken"`
	ServingUnit            string   `json:"serving_unit"`
	ScheduleType           string   `json:"schedule_type"`
	ScheduleDays           []string `json:"schedule_days"`
	EveryOtherDayStart     *string  `json:"every_other_day_start"`
	AsNeeded               bool     `json:"as_needed"`
	ScheduledNotToday      bool     `json:"scheduled_not_today"`
	WithFood               bool     `json:"with_food"`
	ScheduleNotes          *string  `json:"schedule_notes"`
}

type ScheduleReminder struct {
	Enabled bool `json:"enabled"`
}

type ScheduleActivity struct {
	Activity string             `json:"activity"`
	Products []ScheduledProduct `json:"products"`
	Reminder ScheduleReminder   `json:"reminder"`
}

type ProviderSchedule struct {
	Date       string             `json:"date"`
	Activities []ScheduleActivity `json:"activities"`
}

type Observation struct {
	Provider   string `json:"provider"`
	Path       string `json:"path"`
	ObservedAt string `json:"observed_at"`
}

type Provenance struct {
	Stack    Observation `json:"stack"`
	Schedule Observation `json:"schedule"`
}

type Snapshot struct {
	SchemaVersion    string            `json:"schema_version"`
	AsOf             string            `json:"as_of"`
	Provenance       Provenance        `json:"provenance"`
	Products         []Product         `json:"products"`
	Components       []Component       `json:"nutrient_components"`
	ProviderSchedule ProviderSchedule  `json:"provider_schedule"`
	UserOverride     *ProviderSchedule `json:"user_override,omitempty"`
	EffectiveSource  string            `json:"effective_source"`
	EffectiveRegimen ProviderSchedule  `json:"effective_regimen"`
}

// NormalizeStack validates provider identities and relationships, returns
// stable ordering, and preserves every row. It never adds quantities.
func NormalizeStack(products []Product, nutrients []Nutrient) ([]Product, []Component, error) {
	normalizedProducts := append([]Product{}, products...)
	productIDs := make(map[string]struct{}, len(normalizedProducts))
	for _, product := range normalizedProducts {
		if strings.TrimSpace(product.ID) == "" || strings.TrimSpace(product.Name) == "" {
			return nil, nil, fmt.Errorf("every product requires id and name")
		}
		if _, exists := productIDs[product.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate product id")
		}
		productIDs[product.ID] = struct{}{}
	}

	components := make(map[string]Component, len(nutrients))
	children := make(map[string][]string)
	for _, nutrient := range nutrients {
		if strings.TrimSpace(nutrient.ID) == "" || strings.TrimSpace(nutrient.ProductID) == "" || strings.TrimSpace(nutrient.Name) == "" || strings.TrimSpace(nutrient.Unit) == "" {
			return nil, nil, fmt.Errorf("every nutrient requires id, product_id, name, and unit")
		}
		if _, exists := productIDs[nutrient.ProductID]; !exists {
			return nil, nil, fmt.Errorf("nutrient references an unknown product")
		}
		if _, exists := components[nutrient.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate nutrient id")
		}
		components[nutrient.ID] = Component{
			ID: nutrient.ID, ProductID: nutrient.ProductID, Name: nutrient.Name,
			Amount: nutrient.Amount, Unit: nutrient.Unit, ParentID: nutrient.ParentID,
		}
		if nutrient.ParentID != "" {
			children[nutrient.ParentID] = append(children[nutrient.ParentID], nutrient.ID)
		}
	}
	for parentID := range children {
		if _, exists := components[parentID]; !exists {
			return nil, nil, fmt.Errorf("nutrient parent is not present")
		}
	}
	for id, component := range components {
		if component.ParentID != "" && components[component.ParentID].ProductID != component.ProductID {
			return nil, nil, fmt.Errorf("nutrient parent relationship crosses products")
		}
		seen := map[string]struct{}{}
		for current := id; current != ""; current = components[current].ParentID {
			if _, repeated := seen[current]; repeated {
				return nil, nil, fmt.Errorf("nutrient relationship contains a cycle")
			}
			seen[current] = struct{}{}
		}
	}

	componentList := make([]Component, 0, len(components))
	for id, component := range components {
		component.ComponentIDs = append([]string{}, children[id]...)
		sort.Strings(component.ComponentIDs)
		componentList = append(componentList, component)
	}
	sort.Slice(componentList, func(i, j int) bool { return componentList[i].ID < componentList[j].ID })
	sort.Slice(normalizedProducts, func(i, j int) bool { return normalizedProducts[i].ID < normalizedProducts[j].ID })
	return normalizedProducts, componentList, nil
}

func Build(asOf string, stackObservation, scheduleObservation Observation, products []Product, nutrients []Nutrient, providerSchedule ProviderSchedule) (Snapshot, error) {
	if asOf == "" || stackObservation.ObservedAt == "" || scheduleObservation.ObservedAt == "" {
		return Snapshot{}, fmt.Errorf("snapshot observation timestamps are required")
	}
	if stackObservation.Provider != "suppco" || scheduleObservation.Provider != "suppco" || stackObservation.Path == "" || scheduleObservation.Path == "" {
		return Snapshot{}, fmt.Errorf("complete SuppCo provenance is required")
	}
	normalizedSchedule, err := NormalizeProviderSchedule(providerSchedule)
	if err != nil {
		return Snapshot{}, err
	}
	normalizedProducts, components, err := NormalizeStack(products, nutrients)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		SchemaVersion: SchemaVersion,
		AsOf:          asOf,
		Provenance: Provenance{
			Stack: stackObservation, Schedule: scheduleObservation,
		},
		Products:         normalizedProducts,
		Components:       components,
		ProviderSchedule: normalizedSchedule,
		EffectiveSource:  "provider_schedule",
		EffectiveRegimen: cloneProviderSchedule(normalizedSchedule),
	}, nil
}

func cloneProviderSchedule(schedule ProviderSchedule) ProviderSchedule {
	cloned := ProviderSchedule{Date: schedule.Date, Activities: make([]ScheduleActivity, len(schedule.Activities))}
	for activityIndex, activity := range schedule.Activities {
		cloned.Activities[activityIndex] = ScheduleActivity{
			Activity: activity.Activity,
			Reminder: activity.Reminder,
			Products: make([]ScheduledProduct, len(activity.Products)),
		}
		for productIndex, product := range activity.Products {
			product.ScheduleDays = append([]string{}, product.ScheduleDays...)
			product.EveryOtherDayStart = cloneString(product.EveryOtherDayStart)
			product.ScheduleNotes = cloneString(product.ScheduleNotes)
			cloned.Activities[activityIndex].Products[productIndex] = product
		}
	}
	return cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// NormalizeProviderSchedule validates the minimized provider facts, clones
// caller-owned slices, and applies a stable activity/product order. It does not
// infer cadence or change any schedule value.
func NormalizeProviderSchedule(schedule ProviderSchedule) (ProviderSchedule, error) {
	if schedule.Date == "" || schedule.Activities == nil {
		return ProviderSchedule{}, fmt.Errorf("provider_schedule date and activities are required")
	}
	normalized := ProviderSchedule{Date: schedule.Date, Activities: make([]ScheduleActivity, len(schedule.Activities))}
	activityNames := make(map[string]struct{}, len(schedule.Activities))
	for index, activity := range schedule.Activities {
		if activity.Activity == "" || activity.Products == nil {
			return ProviderSchedule{}, fmt.Errorf("every schedule activity requires activity and products")
		}
		if _, exists := activityNames[activity.Activity]; exists {
			return ProviderSchedule{}, fmt.Errorf("duplicate schedule activity")
		}
		activityNames[activity.Activity] = struct{}{}
		normalized.Activities[index] = ScheduleActivity{
			Activity: activity.Activity,
			Reminder: activity.Reminder,
			Products: make([]ScheduledProduct, len(activity.Products)),
		}
		productIDs := make(map[string]struct{}, len(activity.Products))
		for productIndex, product := range activity.Products {
			if product.ProductID == "" || product.ProductName == "" || product.ServingUnit == "" || product.ScheduleType == "" || product.ScheduleDays == nil {
				return ProviderSchedule{}, fmt.Errorf("every scheduled product requires product_id, product_name, serving_unit, schedule_type, and schedule_days")
			}
			if _, exists := productIDs[product.ProductID]; exists {
				return ProviderSchedule{}, fmt.Errorf("duplicate scheduled product within activity")
			}
			productIDs[product.ProductID] = struct{}{}
			product.ScheduleDays = append([]string{}, product.ScheduleDays...)
			normalized.Activities[index].Products[productIndex] = product
		}
		sort.Slice(normalized.Activities[index].Products, func(i, j int) bool {
			left, right := normalized.Activities[index].Products[i], normalized.Activities[index].Products[j]
			if left.ProductID != right.ProductID {
				return left.ProductID < right.ProductID
			}
			return left.ProductName < right.ProductName
		})
	}
	sort.Slice(normalized.Activities, func(i, j int) bool {
		return normalized.Activities[i].Activity < normalized.Activities[j].Activity
	})
	return normalized, nil
}
