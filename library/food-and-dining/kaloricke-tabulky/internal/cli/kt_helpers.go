package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/kaloricke-tabulky/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/kaloricke-tabulky/internal/config"
)

// ktNewAuthenticatedClient builds a Client that carries the cookie auth header.
func ktNewAuthenticatedClient(flags *rootFlags) (*client.Client, *config.Config, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg.AuthHeader() == "" {
		return nil, cfg, fmt.Errorf("not authenticated; run 'kaloricke-tabulky-pp-cli auth password-login --email <email>' or 'auth login --chrome'")
	}
	c := client.New(cfg, 30*time.Second, 0)
	return c, cfg, nil
}

// ktApiResponse is the universal Kalorické Tabulky envelope.
type ktApiResponse struct {
	RequestID *string         `json:"requestId"`
	Code      int             `json:"code"`
	Message   *string         `json:"message"`
	Data      json.RawMessage `json:"data"`
}

// ktUnwrapEnvelope parses the envelope and returns the .data payload (or an
// error if code != 0). For endpoints whose response is already unwrapped
// (e.g. /autocomplete/*, which returns a bare array), pass the bytes through
// unchanged.
func ktUnwrapEnvelope(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty response body")
	}
	// Heuristic: arrays at the top level are already unwrapped
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		return raw, nil
	}
	var env ktApiResponse
	if err := json.Unmarshal(raw, &env); err != nil {
		return raw, nil // not enveloped
	}
	if env.Code != 0 {
		msg := "API error"
		if env.Message != nil && *env.Message != "" {
			msg = *env.Message
		}
		return nil, fmt.Errorf("%s (code %d)", msg, env.Code)
	}
	return env.Data, nil
}

// ktDiaryFoodstuff is the typed projection of a single foodstuff entry
// inside a diary meal slot.
type ktDiaryFoodstuff struct {
	ID                 string  `json:"id"`
	GuidType           string  `json:"guidType"`
	Title              string  `json:"title"`
	URL                string  `json:"url"`
	Unit               string  `json:"unit"`
	Multiplier         float64 `json:"multiplier"`
	Energy             string  `json:"energy"`
	EnergyUnit         string  `json:"energyUnit"`
	Protein            float64 `json:"protein"`
	Carbohydrate       float64 `json:"carbohydrate"`
	Fat                float64 `json:"fat"`
	Fiber              float64 `json:"fiber"`
	Sugar              float64 `json:"sugar"`
	SaturatedFattyAcid float64 `json:"saturatedFattyAcid"`
	Calcium            float64 `json:"calcium"`
	Salt               float64 `json:"salt"`
	Favorite           bool    `json:"favorite"`
}

// ktMealSlot is the typed projection of a diary meal slot.
type ktMealSlot struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Foodstuff []ktDiaryFoodstuff `json:"foodstuff"`
}

// ktDiaryDay is the typed projection of /user/diary/<date>/get.
type ktDiaryDay struct {
	Date       int64        `json:"date"` // epoch ms
	EnergyUnit string       `json:"energyUnit"`
	Times      []ktMealSlot `json:"times"`
}

// ktSummaryDay is the typed projection of /statistic/summary/<date>/get.
// Numeric fields arrive as strings with Czech locale formatting (space as
// thousand separator, comma as decimal point) — parse via ktParseCzechNum.
type ktSummaryDay struct {
	Date              string                 `json:"date"`
	TodayEnergy       string                 `json:"todayEnergy"`
	TodayEnergyTarget string                 `json:"todayEnergyTarget"`
	EnergyUnit        string                 `json:"energyUnit"`
	EnergyUnitCode    string                 `json:"energyUnitCode"`
	TodayActivity     string                 `json:"todayActivity"`
	TodayDrink        string                 `json:"todayDrink"`
	TodayDrinkTarget  string                 `json:"todayDrinkTarget"`
	Basal             string                 `json:"basal"`
	AMR               string                 `json:"amr"`
	AMREnergy         string                 `json:"amrEnergy"`
	BMI               string                 `json:"bmi"`
	BMICategory       string                 `json:"bmiCategory"`
	WeekDate          string                 `json:"weekDate"`
	MonthWeight       []ktWeightRecord       `json:"monthWeight"`
	Extra             map[string]interface{} `json:"-"`
}

// ktWeightRecord is one entry in summary.monthWeight.
type ktWeightRecord struct {
	Description string  `json:"description"`
	Value       float64 `json:"value"`
}

// ktParseCzechNum parses a Czech-formatted number string. Empty/null returns 0, ok=false.
// "8 417" -> 8417, "12,5" -> 12.5, "1 094,5" -> 1094.5.
func ktParseCzechNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 0, false
	}
	// Strip both ASCII and U+00A0 (non-breaking) spaces
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.Replace(s, ",", ".", 1)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// ktFetchDiaryDay calls /user/diary/<date>/get and returns the typed diary.
func ktFetchDiaryDay(c *client.Client, ddmmyyyy string) (*ktDiaryDay, error) {
	raw, err := c.GetWithHeadersNoCache(
		"/user/diary/"+ddmmyyyy+"/get",
		map[string]string{"format": "json"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching diary for %s: %w", ddmmyyyy, err)
	}
	data, err := ktUnwrapEnvelope(raw)
	if err != nil {
		return nil, err
	}
	var d ktDiaryDay
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parsing diary for %s: %w", ddmmyyyy, err)
	}
	return &d, nil
}

// ktFetchSummaryDay calls /statistic/summary/<date>/get and returns the typed summary.
func ktFetchSummaryDay(c *client.Client, ddmmyyyy string) (*ktSummaryDay, error) {
	raw, err := c.GetWithHeadersNoCache(
		"/statistic/summary/"+ddmmyyyy+"/get",
		map[string]string{"format": "json"},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("fetching summary for %s: %w", ddmmyyyy, err)
	}
	data, err := ktUnwrapEnvelope(raw)
	if err != nil {
		return nil, err
	}
	var s ktSummaryDay
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing summary for %s: %w", ddmmyyyy, err)
	}
	return &s, nil
}

// ktDayMacros aggregates a diary day's foodstuff entries into per-macro totals.
type ktDayMacros struct {
	Date          string                  `json:"date"`
	EnergyKJ      float64                 `json:"energy_kj"`
	EnergyKcal    float64                 `json:"energy_kcal"`
	ProteinG      float64                 `json:"protein_g"`
	CarbG         float64                 `json:"carb_g"`
	FatG          float64                 `json:"fat_g"`
	FiberG        float64                 `json:"fiber_g"`
	SugarG        float64                 `json:"sugar_g"`
	SaturatedFatG float64                 `json:"saturated_fat_g"`
	CalciumMg     float64                 `json:"calcium_mg"`
	SaltG         float64                 `json:"salt_g"`
	EntryCount    int                     `json:"entry_count"`
	BySlot        map[string]ktSlotMacros `json:"by_slot,omitempty"`
}

type ktSlotMacros struct {
	SlotID     string  `json:"slot_id"`
	SlotLabel  string  `json:"slot_label"`
	EnergyKJ   float64 `json:"energy_kj"`
	ProteinG   float64 `json:"protein_g"`
	CarbG      float64 `json:"carb_g"`
	FatG       float64 `json:"fat_g"`
	FiberG     float64 `json:"fiber_g"`
	EntryCount int     `json:"entry_count"`
}

// ktAggregateDay sums foodstuff macros for a single diary day.
func ktAggregateDay(d *ktDiaryDay, isoDate string) ktDayMacros {
	out := ktDayMacros{Date: isoDate, BySlot: map[string]ktSlotMacros{}}
	for _, slot := range d.Times {
		s := ktSlotMacros{SlotID: slot.ID, SlotLabel: mealSlotLabel(slot.ID)}
		for _, f := range slot.Foodstuff {
			ekj, _ := ktParseCzechNum(f.Energy)
			if strings.EqualFold(f.EnergyUnit, "kcal") {
				out.EnergyKcal += ekj
				ekj = ekj * 4.184 // convert for unified storage
			}
			out.EnergyKJ += ekj
			s.EnergyKJ += ekj
			out.ProteinG += f.Protein
			s.ProteinG += f.Protein
			out.CarbG += f.Carbohydrate
			s.CarbG += f.Carbohydrate
			out.FatG += f.Fat
			s.FatG += f.Fat
			out.FiberG += f.Fiber
			s.FiberG += f.Fiber
			out.SugarG += f.Sugar
			out.SaturatedFatG += f.SaturatedFattyAcid
			out.CalciumMg += f.Calcium
			out.SaltG += f.Salt
			out.EntryCount++
			s.EntryCount++
		}
		out.BySlot[slot.ID] = s
	}
	if out.EnergyKcal == 0 && out.EnergyKJ > 0 {
		out.EnergyKcal = out.EnergyKJ / 4.184
	}
	return out
}

// ktJSONContainer mirrors the wrapper shape generated commands emit.
type ktJSONContainer struct {
	Meta    map[string]any `json:"meta"`
	Results any            `json:"results"`
}

// ktEmit renders the value through the same output pipeline as generated
// commands. Uses printJSONFiltered so --select / --compact / --csv work
// uniformly with hand-coded transcendence commands.
func ktEmit(w io.Writer, flags *rootFlags, v any) error {
	wrapped := &ktJSONContainer{Meta: map[string]any{"source": "computed"}, Results: v}
	return printJSONFiltered(w, wrapped, flags)
}
