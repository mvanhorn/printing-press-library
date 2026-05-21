package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// diary log <food-query> --grams N --meal SLOT [--date]
// Resolves the food query against the live /autocomplete/foodstuff
// endpoint (first hit, or refuses if ambiguous unless --pick=<n>), then
// POSTs the diary add to /user/foodstuff/add?format=json with the
// addFoodstuffForm payload shape (guid, multiplier=grams, unitGuid='g',
// diaryTimeGuid=<slot>, title, etc.).
func newKTDiaryLogCmd(flags *rootFlags) *cobra.Command {
	var grams float64
	var meal string
	var dateFlag string
	var pickIdx int
	var commit bool

	cmd := &cobra.Command{
		Use:   "log [food-query]",
		Short: "Resolve a food by Czech-text query and log it to the diary in one command",
		Long: `Search the live autocomplete endpoint for the food, take the top hit
(or --pick=<n> for the n-th result, 1-indexed), and POST it to your
diary at the chosen meal slot for the chosen date.

Examples:
  kaloricke-tabulky-pp-cli diary log 'tvaroh' --grams 150 --meal lunch
  kaloricke-tabulky-pp-cli diary log 'jablko' --grams 80 --meal afternoon-snack --commit
  kaloricke-tabulky-pp-cli diary log 'oves' --grams 50 --meal breakfast --pick 3

Defaults to dry-run mode (shows the resolved food + payload) until you
pass --commit. This is a write to your live account.`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			if grams <= 0 {
				return fmt.Errorf("--grams must be > 0")
			}
			slotID, err := mealSlotID(meal)
			if err != nil {
				return err
			}
			ddmmyyyy, err := parseFlexDate(dateFlag)
			if err != nil {
				return err
			}

			c, cfg, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}

			// Resolve the food.
			raw, err := c.GetNoCache("/autocomplete/foodstuff", map[string]string{
				"query":  query,
				"format": "json",
			})
			if err != nil {
				return fmt.Errorf("food search: %w", err)
			}
			var hits []ktAutocompleteHit
			if err := json.Unmarshal(raw, &hits); err != nil {
				return fmt.Errorf("parsing autocomplete: %w", err)
			}
			if len(hits) == 0 {
				return fmt.Errorf("no food matched %q (try a different spelling or run `food search %q` first)", query, query)
			}
			idx := pickIdx - 1
			if idx < 0 {
				idx = 0
			}
			if idx >= len(hits) {
				return fmt.Errorf("--pick %d out of range (%d results)", pickIdx, len(hits))
			}
			chosen := hits[idx]

			payload := map[string]interface{}{
				"guid":          chosen.ID,
				"guidType":      chosen.ID,
				"title":         chosen.Title,
				"url":           chosen.URL,
				"multiplier":    grams,
				"unitGuid":      "g",
				"unit":          chosen.Unit,
				"diaryTimeGuid": slotID,
				"date":          ddmmyyyy,
				"favorite":      chosen.Favorite,
				"status":        chosen.Status,
				"visibility":    chosen.Visibility,
				"isLiquid":      chosen.IsLiquid,
			}

			summary := map[string]interface{}{
				"action":          "diary-log",
				"food_id":         chosen.ID,
				"food_title":      chosen.Title,
				"food_slug":       chosen.URL,
				"grams":           grams,
				"meal_slot_id":    slotID,
				"meal_slot_label": mealSlotLabel(slotID),
				"date":            ddmmyyyy,
				"date_iso":        ddmmyyyy_to_iso(ddmmyyyy),
				"matched_results": len(hits),
				"picked_index":    idx + 1,
			}

			if !commit {
				summary["committed"] = false
				summary["note"] = "Pass --commit to actually post this to your diary."
				return ktEmit(cmd.OutOrStdout(), flags, summary)
			}

			// POST
			body, _ := json.Marshal(payload)
			req, err := http.NewRequest("POST",
				"https://www.kaloricketabulky.cz/user/foodstuff/add?format=json",
				bytes.NewReader(body),
			)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Cookie", cfg.AuthHeader())
			req.Header.Set("User-Agent", "kaloricke-tabulky-pp-cli/1.0")
			httpClient := &http.Client{Timeout: 30 * time.Second}
			resp, err := httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("posting diary entry: %w", err)
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			if resp.StatusCode != 200 {
				return fmt.Errorf("diary add HTTP %d: %s", resp.StatusCode, string(respBody))
			}
			var env ktApiResponse
			if err := json.Unmarshal(respBody, &env); err != nil {
				return fmt.Errorf("parsing diary add response: %w (body: %.200s)", err, respBody)
			}
			if env.Code != 0 {
				msg := "diary add rejected"
				if env.Message != nil && *env.Message != "" {
					msg = *env.Message
				}
				return fmt.Errorf("%s (code %d)", msg, env.Code)
			}
			summary["committed"] = true
			summary["api_code"] = env.Code
			return ktEmit(cmd.OutOrStdout(), flags, summary)
		},
	}
	cmd.Flags().Float64Var(&grams, "grams", 0, "Portion in grams (or ml for liquids; required)")
	cmd.Flags().StringVar(&meal, "meal", "lunch", "Meal slot: breakfast|morning-snack|lunch|afternoon-snack|dinner|evening-snack")
	cmd.Flags().StringVar(&dateFlag, "date", "today", "Date (today|yesterday|-N|YYYY-MM-DD|DD.MM.YYYY)")
	cmd.Flags().IntVar(&pickIdx, "pick", 1, "Pick the n-th result (1-indexed) when search has multiple hits")
	cmd.Flags().BoolVar(&commit, "commit", false, "Actually POST the entry; without --commit, prints the resolved payload")
	return cmd
}

// ktAutocompleteHit is the typed projection of one autocomplete result.
type ktAutocompleteHit struct {
	Clazz      string `json:"clazz"`
	ID         string `json:"id"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Unit       string `json:"unit"`
	Value      string `json:"value"`
	Favorite   bool   `json:"favorite"`
	Status     int    `json:"status"`
	Visibility string `json:"visibility"`
	IsLiquid   bool   `json:"isLiquid"`
	BrandName  string `json:"brandName"`
	HasImage   bool   `json:"hasImage"`
}

func ddmmyyyy_to_iso(in string) string {
	t, err := time.Parse("02.01.2006", in)
	if err != nil {
		return in
	}
	return t.Format("2006-01-02")
}
