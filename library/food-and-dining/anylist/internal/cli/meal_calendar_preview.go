package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func printMealCalendarPreview(cmd *cobra.Command, flags *rootFlags, action string, fields map[string]any) error {
	preview := map[string]any{
		"action":   action,
		"resource": "meal",
		"dry_run":  true,
	}
	for key, value := range fields {
		if value != nil && value != "" {
			preview[key] = value
		}
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
	}
	switch action {
	case "create":
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would add a meal-calendar event on %q\n", fields["date"])
	case "update":
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would update meal event %q\n", fields["event_id"])
	case "delete":
		fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would delete meal event %q\n", fields["event_id"])
	}
	return nil
}
