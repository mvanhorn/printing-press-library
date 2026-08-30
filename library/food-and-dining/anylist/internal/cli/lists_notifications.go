package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/spf13/cobra"
)

func newListsNotificationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "notifications", Short: "Manage list notification locations"}
	cmd.AddCommand(newListsNotificationsAddCmd(flags))
	cmd.AddCommand(newListsNotificationsRemoveCmd(flags))
	return cmd
}

func newListsNotificationsAddCmd(flags *rootFlags) *cobra.Command {
	var listName, locationID, locationName, address string
	var latitude, longitude float64
	var latitudeSet, longitudeSet, stdinBody, apply bool
	cmd := &cobra.Command{
		Use:         "add",
		Short:       "Add a notification location (preview unless --apply)",
		Example:     "  anylist-pp-cli lists notifications add --list Groceries --location-name Home --address \"123 Main St\" --latitude 33.66 --longitude -95.55",
		Annotations: map[string]string{"pp:endpoint": "lists.notifications.add", "pp:method": "POST", "pp:path": "/data/shopping-lists/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				listName = stringFromBody(body, "list")
				if listName == "" {
					listName = stringFromBody(body, "list_name")
				}
				locationID = stringFromBody(body, "id")
				locationName = stringFromBody(body, "location_name")
				if locationName == "" {
					locationName = stringFromBody(body, "name")
				}
				address = stringFromBody(body, "address")
				latitude, latitudeSet = floatFromBody(body, "latitude")
				longitude, longitudeSet = floatFromBody(body, "longitude")
				apply = boolFromBody(body, "apply")
			}
			if !stdinBody {
				latitudeSet = cmd.Flags().Changed("latitude")
				longitudeSet = cmd.Flags().Changed("longitude")
			}
			listName, locationID, locationName, address = strings.TrimSpace(listName), strings.TrimSpace(locationID), strings.TrimSpace(locationName), strings.TrimSpace(address)
			if !apply || flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "preview", "action": "add", "list": listName, "location_id": locationID, "location_name": locationName, "address": address, "latitude": latitude, "longitude": longitude, "apply": apply}, flags)
			}
			if listName == "" || locationName == "" || address == "" || !latitudeSet || !longitudeSet {
				return fmt.Errorf("list, location name, address, latitude, and longitude are required")
			}
			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			client := anylist.New(cfg)
			data, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live lists: %w", err)
			}
			list, err := exactLiveShoppingListByName(data, listName)
			if err != nil {
				return err
			}
			id, err := client.AddListNotificationLocation(ctx, list.GetIdentifier(), &pb.PBNotificationLocation{Identifier: locationID, Latitude: latitude, Longitude: longitude, Name: locationName, Address: address})
			if err != nil {
				return fmt.Errorf("adding notification location to %q: %w", list.GetName(), err)
			}
			verifiedData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying notification location: %w", err)
			}
			verifiedList, found := findLiveShoppingListByID(verifiedData, list.GetIdentifier())
			if !found {
				return fmt.Errorf("notification location verification failed: list disappeared")
			}
			verified := false
			for _, location := range verifiedList.GetNotificationLocations() {
				if location.GetIdentifier() == id {
					verified = true
					break
				}
			}
			if !verified {
				return fmt.Errorf("notification location verification failed: ID %q was not read back", id)
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after notification location add: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"added": true, "list": list.GetName(), "list_id": list.GetIdentifier(), "location_id": id, "verified": true}, flags)
		},
	}
	cmd.Flags().StringVar(&listName, "list", "", "Shopping list name")
	cmd.Flags().StringVar(&locationID, "id", "", "Optional stable notification-location ID")
	cmd.Flags().StringVar(&locationName, "location-name", "", "Location name")
	cmd.Flags().StringVar(&address, "address", "", "Location address")
	cmd.Flags().Float64Var(&latitude, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&longitude, "longitude", 0, "Longitude")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the add; preview is the default")
	return cmd
}

func newListsNotificationsRemoveCmd(flags *rootFlags) *cobra.Command {
	var listName, locationID, locationName string
	var stdinBody, apply bool
	cmd := &cobra.Command{
		Use:         "remove",
		Short:       "Remove a notification location (preview unless --apply)",
		Example:     "  anylist-pp-cli lists notifications remove --list Groceries --location-name Home",
		Annotations: map[string]string{"pp:endpoint": "lists.notifications.remove", "pp:method": "POST", "pp:path": "/data/shopping-lists/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdinBody {
				body, err := readStdinJSONMap()
				if err != nil {
					return err
				}
				listName = stringFromBody(body, "list")
				if listName == "" {
					listName = stringFromBody(body, "list_name")
				}
				locationID = stringFromBody(body, "id")
				locationName = stringFromBody(body, "location_name")
				if locationName == "" {
					locationName = stringFromBody(body, "name")
				}
				apply = boolFromBody(body, "apply")
			}
			listName, locationID, locationName = strings.TrimSpace(listName), strings.TrimSpace(locationID), strings.TrimSpace(locationName)
			if !apply || flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "preview", "action": "remove", "list": listName, "location_id": locationID, "location_name": locationName, "apply": apply}, flags)
			}
			if listName == "" || (locationID == "" && locationName == "") {
				return fmt.Errorf("list and either location ID or exact location name are required")
			}
			ctx := cmd.Context()
			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			client := anylist.New(cfg)
			data, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live lists: %w", err)
			}
			list, err := exactLiveShoppingListByName(data, listName)
			if err != nil {
				return err
			}
			var target *pb.PBNotificationLocation
			for _, candidate := range list.GetNotificationLocations() {
				if (locationID != "" && candidate.GetIdentifier() == locationID) || (locationID == "" && strings.EqualFold(candidate.GetName(), locationName)) {
					if target != nil {
						return fmt.Errorf("notification location selector is ambiguous")
					}
					target = candidate
				}
			}
			if target == nil {
				return fmt.Errorf("notification location not found on list %q", list.GetName())
			}
			if err := client.RemoveListNotificationLocation(ctx, list.GetIdentifier(), target); err != nil {
				return fmt.Errorf("removing notification location from %q: %w", list.GetName(), err)
			}
			verifiedData, err := client.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("verifying notification location removal: %w", err)
			}
			verifiedList, found := findLiveShoppingListByID(verifiedData, list.GetIdentifier())
			if !found {
				return fmt.Errorf("notification location removal verification failed: list disappeared")
			}
			for _, candidate := range verifiedList.GetNotificationLocations() {
				if candidate.GetIdentifier() == target.GetIdentifier() {
					return fmt.Errorf("notification location removal verification failed: ID %q remains", target.GetIdentifier())
				}
			}
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("updating local cache after notification location removal: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": true, "list": list.GetName(), "list_id": list.GetIdentifier(), "location_id": target.GetIdentifier(), "verified": true}, flags)
		},
	}
	cmd.Flags().StringVar(&listName, "list", "", "Shopping list name")
	cmd.Flags().StringVar(&locationID, "id", "", "Stable notification-location ID")
	cmd.Flags().StringVar(&locationName, "location-name", "", "Exact location name when ID is unavailable")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply the removal; preview is the default")
	return cmd
}
