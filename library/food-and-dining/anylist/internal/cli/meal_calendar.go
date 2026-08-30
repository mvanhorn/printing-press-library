package cli

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

type mealEventInput struct {
	Date        string
	RecipeName  string
	Title       string
	LabelName   string
	Details     string
	ScaleFactor float64 // 0 = not provided; set explicitly for create or update
}

func applyMealEventCreate(ctx context.Context, flags *rootFlags, input mealEventInput) (map[string]any, error) {
	if err := validateMealDate(input.Date); err != nil {
		return nil, err
	}
	cfg, st, err := openAuthedLocalStore(flags)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	client := anylist.New(cfg)
	userData, calendarID, err := currentMealCalendar(ctx, client)
	if err != nil {
		return nil, err
	}
	recipeID, err := resolveMealRecipeID(userData, input.RecipeName)
	if err != nil {
		return nil, err
	}
	labelID, err := resolveMealLabelID(userData, input.LabelName)
	if err != nil {
		return nil, err
	}
	scaleFactor := 1.0
	if input.ScaleFactor > 0 {
		scaleFactor = input.ScaleFactor
	}
	event := &pb.PBCalendarEvent{
		Identifier:        strings.ReplaceAll(uuid.NewString(), "-", ""),
		LogicalTimestamp:  uint64(time.Now().UnixMilli()),
		CalendarId:        calendarID,
		Date:              input.Date,
		Title:             input.Title,
		Details:           input.Details,
		RecipeId:          recipeID,
		LabelId:           labelID,
		RecipeScaleFactor: scaleFactor,
	}
	if err := client.SaveCalendarEvent(ctx, calendarID, event); err != nil {
		return nil, fmt.Errorf("creating meal event: %w", err)
	}
	verifiedData, err := client.GetUserData(ctx)
	if err != nil {
		return nil, fmt.Errorf("verifying meal event create: %w", err)
	}
	actual, found := findMealEventByID(verifiedData, event.GetIdentifier())
	if !found {
		return nil, fmt.Errorf("meal event create verification failed: event %q is absent from fresh user data", event.GetIdentifier())
	}
	if err := validateMealEventReadBack(event, actual); err != nil {
		return nil, err
	}
	if err := st.SyncFromUserData(verifiedData); err != nil {
		return nil, fmt.Errorf("updating local cache after meal event create: %w", err)
	}
	return map[string]any{
		"created":  true,
		"verified": true,
		"event_id": event.GetIdentifier(),
	}, nil
}

func applyMealEventUpdate(ctx context.Context, flags *rootFlags, eventID string, input mealEventInput) (map[string]any, error) {
	if input.Date != "" {
		if err := validateMealDate(input.Date); err != nil {
			return nil, err
		}
	}
	cfg, st, err := openAuthedLocalStore(flags)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	client := anylist.New(cfg)
	userData, calendarID, err := currentMealCalendar(ctx, client)
	if err != nil {
		return nil, err
	}
	current, found := findMealEventByID(userData, eventID)
	if !found {
		return nil, fmt.Errorf("meal event %q not found in fresh user data", eventID)
	}
	updated := proto.Clone(current).(*pb.PBCalendarEvent)
	if input.Date != "" {
		updated.Date = input.Date
	}
	if input.Title != "" {
		updated.Title = input.Title
	}
	if input.Details != "" {
		updated.Details = input.Details
	}
	if input.RecipeName != "" {
		recipeID, err := resolveMealRecipeID(userData, input.RecipeName)
		if err != nil {
			return nil, err
		}
		updated.RecipeId = recipeID
	}
	if input.LabelName != "" {
		labelID, err := resolveMealLabelID(userData, input.LabelName)
		if err != nil {
			return nil, err
		}
		updated.LabelId = labelID
	}
	if input.ScaleFactor > 0 {
		updated.RecipeScaleFactor = input.ScaleFactor
	}
	if err := client.UpdateCalendarEvent(ctx, calendarID, updated, current); err != nil {
		return nil, fmt.Errorf("updating meal event: %w", err)
	}
	verifiedData, err := client.GetUserData(ctx)
	if err != nil {
		return nil, fmt.Errorf("verifying meal event update: %w", err)
	}
	actual, found := findMealEventByID(verifiedData, eventID)
	if !found {
		return nil, fmt.Errorf("meal event update verification failed: event %q is absent from fresh user data", eventID)
	}
	if err := validateMealEventReadBack(updated, actual); err != nil {
		return nil, err
	}
	if err := st.SyncFromUserData(verifiedData); err != nil {
		return nil, fmt.Errorf("updating local cache after meal event update: %w", err)
	}
	return map[string]any{
		"updated":  true,
		"verified": true,
		"event_id": eventID,
	}, nil
}

// validateScaleFactor ensures scale is a finite positive number.
func validateScaleFactor(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("scale factor must be a finite number")
	}
	if v <= 0 {
		return fmt.Errorf("scale factor must be positive, got %v", v)
	}
	return nil
}

func applyMealEventDelete(ctx context.Context, flags *rootFlags, eventID string) (map[string]any, error) {
	cfg, st, err := openAuthedLocalStore(flags)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	client := anylist.New(cfg)
	userData, calendarID, err := currentMealCalendar(ctx, client)
	if err != nil {
		return nil, err
	}
	event, found := findMealEventByID(userData, eventID)
	if !found {
		return nil, fmt.Errorf("meal event %q not found in fresh user data", eventID)
	}
	if err := client.RemoveCalendarEvent(ctx, calendarID, event); err != nil {
		return nil, fmt.Errorf("deleting meal event: %w", err)
	}
	verifiedData, err := client.GetUserData(ctx)
	if err != nil {
		return nil, fmt.Errorf("verifying meal event deletion: %w", err)
	}
	if _, found := findMealEventByID(verifiedData, eventID); found {
		return nil, fmt.Errorf("meal event deletion verification failed: event %q is still present in fresh user data", eventID)
	}
	if err := st.SyncFromUserData(verifiedData); err != nil {
		return nil, fmt.Errorf("updating local cache after meal event deletion: %w", err)
	}
	return map[string]any{
		"deleted":  true,
		"verified": true,
		"event_id": eventID,
	}, nil
}

func currentMealCalendar(ctx context.Context, client *anylist.Client) (*pb.PBUserDataResponse, string, error) {
	userData, err := client.GetUserData(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("reading live meal calendar: %w", err)
	}
	calendar := userData.GetMealPlanningCalendarResponse()
	if calendar == nil || strings.TrimSpace(calendar.GetCalendarId()) == "" {
		return nil, "", fmt.Errorf("AnyList user data did not include a meal-planning calendar ID")
	}
	return userData, calendar.GetCalendarId(), nil
}

func validateMealDate(date string) error {
	if strings.TrimSpace(date) == "" {
		return fmt.Errorf("event date must not be empty")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("invalid event date: %w", err)
	}
	return nil
}

func findMealEventByID(userData *pb.PBUserDataResponse, eventID string) (*pb.PBCalendarEvent, bool) {
	calendar := userData.GetMealPlanningCalendarResponse()
	if calendar == nil {
		return nil, false
	}
	for _, event := range calendar.GetEvents() {
		if event.GetIdentifier() == eventID {
			return event, true
		}
	}
	return nil, false
}

func resolveMealRecipeID(userData *pb.PBUserDataResponse, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	data := userData.GetRecipeDataResponse()
	if data == nil {
		return "", fmt.Errorf("recipe %q cannot be resolved: recipe data is absent", name)
	}
	var match *pb.PBRecipe
	for _, recipe := range data.GetRecipes() {
		if strings.EqualFold(strings.TrimSpace(recipe.GetName()), name) {
			if match != nil {
				return "", fmt.Errorf("recipe name %q is ambiguous", name)
			}
			match = recipe
		}
	}
	if match == nil {
		return "", fmt.Errorf("recipe %q not found in fresh user data", name)
	}
	return match.GetIdentifier(), nil
}

func resolveMealLabelID(userData *pb.PBUserDataResponse, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	calendar := userData.GetMealPlanningCalendarResponse()
	if calendar == nil {
		return "", fmt.Errorf("meal label %q cannot be resolved: calendar data is absent", name)
	}
	var match *pb.PBCalendarLabel
	for _, label := range calendar.GetLabels() {
		if strings.EqualFold(strings.TrimSpace(label.GetName()), name) {
			if match != nil {
				return "", fmt.Errorf("meal label name %q is ambiguous", name)
			}
			match = label
		}
	}
	if match == nil {
		return "", fmt.Errorf("meal label %q not found in fresh user data", name)
	}
	return match.GetIdentifier(), nil
}

func validateMealEventReadBack(expected, actual *pb.PBCalendarEvent) error {
	if expected == nil || actual == nil {
		return fmt.Errorf("meal event verification failed: event was not returned")
	}
	checks := []struct {
		field string
		want  string
		got   string
	}{
		{"identifier", expected.GetIdentifier(), actual.GetIdentifier()},
		{"calendar_id", expected.GetCalendarId(), actual.GetCalendarId()},
		{"date", expected.GetDate(), actual.GetDate()},
		{"title", expected.GetTitle(), actual.GetTitle()},
		{"details", expected.GetDetails(), actual.GetDetails()},
		{"recipe_id", expected.GetRecipeId(), actual.GetRecipeId()},
		{"label_id", expected.GetLabelId(), actual.GetLabelId()},
	}
	for _, check := range checks {
		if check.want != check.got {
			return fmt.Errorf("meal event verification failed: %s read back as %q, want %q", check.field, check.got, check.want)
		}
	}
	if expected.GetRecipeScaleFactor() != 0 && expected.GetRecipeScaleFactor() != actual.GetRecipeScaleFactor() {
		return fmt.Errorf("meal event verification failed: recipe scale factor read back as %v, want %v", actual.GetRecipeScaleFactor(), expected.GetRecipeScaleFactor())
	}
	return nil
}

func printMealCalendarResult(cmd *cobra.Command, flags *rootFlags, result map[string]any) error {
	if flags.quiet {
		return nil
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if result["created"] == true {
		fmt.Fprintf(cmd.OutOrStdout(), "Created meal event %q (verified)\n", result["event_id"])
	} else if result["updated"] == true {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated meal event %q (verified)\n", result["event_id"])
	} else if result["deleted"] == true {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted meal event %q (verified)\n", result["event_id"])
	}
	return nil
}
