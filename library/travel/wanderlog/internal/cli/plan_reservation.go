// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlog/internal/client"
	"github.com/spf13/cobra"
)

type planReservationOptions struct {
	planEditOptions
	kind                string
	title               string
	note                string
	confirmationNumber  string
	airline             string
	flightNumber        string
	departureAirport    string
	arrivalAirport      string
	departurePlaceID    string
	arrivalPlaceID      string
	pickupPlaceID       string
	dropoffPlaceID      string
	placeID             string
	startDate           string
	startTime           string
	endDate             string
	endTime             string
	carrier             string
	cruiseLine          string
	shipName            string
	voyageNumber        string
	travelerNames       []string
	partySize           int
	nameForReservation  string
	url                 string
	filename            string
	mimeType            string
	jsonValue           string
	lodgingOfferJSON    string
	field               string
	value               string
	remove              bool
	spanNights          bool
	displayName         string
	expectNameSubstring string
}

var reservationBlockKinds = map[string]bool{
	"flight":     true,
	"rentalCar":  true,
	"train":      true,
	"bus":        true,
	"ferry":      true,
	"cruise":     true,
	"attachment": true,
}

func newNovelPlanReservationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reservation",
		Short: "List, add, edit, or remove reservation and standalone attachment blocks",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPlanReservationListCmd(flags))
	cmd.AddCommand(newNovelPlanReservationAddCmd(flags))
	cmd.AddCommand(newNovelPlanReservationEditCmd(flags))
	cmd.AddCommand(newNovelPlanReservationRemoveCmd(flags))
	return cmd
}

func newNovelPlanReservationListCmd(flags *rootFlags) *cobra.Command {
	opts := planReservationOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List reservation and standalone attachment blocks in a Wanderlog plan",
		Example:     "  wanderlog-pp-cli plan reservation list --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --kind flight --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			key, err := resolveEditablePlanKey(opts.planEditOptions)
			if err != nil {
				return usageErr(err)
			}
			trip, _, err := fetchPlanSnapshotViaShareDB(ctx, c, key, opts.clientSchemaVersion)
			if err != nil {
				trip, _, err = fetchPlan(ctx, c, key, opts.clientSchemaVersion)
				if err != nil {
					return err
				}
			}
			kind, err := normalizeReservationKind(opts.kind)
			if err != nil {
				return usageErr(err)
			}
			items, err := collectReservationBlocks(trip, opts.planEditOptions, kind)
			if err != nil {
				return usageErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan reservation list", "target_key": key, "kind": kind, "items": items}, flags)
		},
	}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	addPlanSectionFlags(cmd, &opts.planEditOptions)
	cmd.Flags().StringVar(&opts.kind, "kind", "", "Optional kind filter: flight, lodging, rental-car, restaurant, train, bus, ferry, cruise, attachment")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanReservationAddCmd(flags *rootFlags) *cobra.Command {
	opts := planReservationOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, position: -1, radius: 50000, language: "en", applyRetries: 2}}
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a flight, lodging, rental car, restaurant, transit, cruise, or standalone attachment block",
		Example: "  wanderlog-pp-cli plan reservation add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --day 1 --kind flight --airline NH --flight-number 463 --start-date 2026-09-01 --departure-airport HND --arrival-airport OKA --dry-run --agent\n  wanderlog-pp-cli plan reservation add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --kind lodging --query 'Hotel Moon Beach' --lat 26.32 --lng 127.76 --start-date 2026-09-01 --end-date 2026-09-03 --display-name 'Hotel Moon Beach' --expect-name-substring Moon --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := normalizeReservationKind(opts.kind)
			if err != nil {
				return usageErr(err)
			}
			if opts.jsonValue == "" {
				if err := validateReservationAddFlags(kind, opts); err != nil {
					return usageErr(err)
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			block, echo, err := buildReservationAdd(ctx, c, kind, opts)
			if err != nil {
				if strings.Contains(err.Error(), "--expect-name-substring") {
					return usageErr(err)
				}
				return classifyAPIError(err, flags)
			}
			return runPlanEditWithClient(cmd, flags, c, opts.planEditOptions, "plan reservation add", func(target map[string]any) (planEditBuildResult, error) {
				if kind == "lodging" {
					return buildLodgingReservationInsert(target, opts, block, echo)
				}
				sec, err := resolveSection(target, opts.day, opts.sectionIndex, opts.sectionID)
				if err != nil {
					return planEditBuildResult{}, err
				}
				idx := normalizeInsertPosition(opts.position, len(sec.Blocks))
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "li": block}}
				report := baseEditReport("plan reservation add", opts.planEditOptions, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "insert " + kind + " block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	addPlanSectionFlags(cmd, &opts.planEditOptions)
	addReservationAddFlags(cmd, &opts)
	return cmd
}

func newNovelPlanReservationEditCmd(flags *rootFlags) *cobra.Command {
	opts := planReservationOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, applyRetries: 2}}
	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "Set (--field/--value or --json-value) or remove (--remove) a field on a reservation or attachment block; --kind selects the block shape",
		Example: "  wanderlog-pp-cli plan reservation edit --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --section-index 1 --block-index 0 --field confirmationNumber --value ABC123 --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, value, err := parseFieldMutation(opts.field, opts.value, opts.jsonValue, opts.remove)
			if err != nil {
				return usageErr(err)
			}
			kind, err := normalizeReservationKind(opts.kind)
			if err != nil {
				return usageErr(err)
			}
			return runPlanEdit(cmd, flags, opts.planEditOptions, "plan reservation edit", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				if !isReservationKind(block, kind) {
					return planEditBuildResult{}, fmt.Errorf("selected block is %q, not a reservation/attachment block matching %q", stringField(block, "type"), firstNonEmpty(kind, "any"))
				}
				old, exists := getMapPath(block, path)
				if opts.remove && !exists {
					return planEditBuildResult{}, fmt.Errorf("field %q does not exist", opts.field)
				}
				ops := []map[string]any{objectSetOp(append([]any{"itinerary", "sections", sec.Index, "blocks", idx}, pathToAny(path)...), old, exists, value, opts.remove)}
				report := baseEditReport("plan reservation edit", opts.planEditOptions, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "set reservation field"
				if opts.remove {
					report.Operation = "remove reservation field"
				}
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	addPlanSectionFlags(cmd, &opts.planEditOptions)
	addPlanBlockFlags(cmd, &opts.planEditOptions)
	cmd.Flags().StringVar(&opts.kind, "kind", "", "Optional guard kind: flight, lodging, rental-car, restaurant, train, bus, ferry, cruise, attachment")
	cmd.Flags().StringVar(&opts.field, "field", "", "Field path to set, for example confirmationNumber, hotel.checkIn, depart.time, attachments.0.title")
	cmd.Flags().StringVar(&opts.value, "value", "", "String value to set; text fields become Wanderlog rich text")
	cmd.Flags().StringVar(&opts.jsonValue, "json-value", "", "JSON value to set")
	cmd.Flags().BoolVar(&opts.remove, "remove", false, "Remove the field")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanReservationRemoveCmd(flags *rootFlags) *cobra.Command {
	opts := planReservationOptions{planEditOptions: planEditOptions{clientSchemaVersion: 2, sectionIndex: -1, blockIndex: -1, applyRetries: 2}}
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove a reservation or standalone attachment block",
		Example: "  wanderlog-pp-cli plan reservation remove --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --section-index 1 --block-index 0 --kind flight --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := normalizeReservationKind(opts.kind)
			if err != nil {
				return usageErr(err)
			}
			return runPlanEdit(cmd, flags, opts.planEditOptions, "plan reservation remove", func(target map[string]any) (planEditBuildResult, error) {
				sec, block, idx, err := resolveBlock(target, opts.day, opts.sectionIndex, opts.sectionID, opts.blockID, opts.blockIndex)
				if err != nil {
					return planEditBuildResult{}, err
				}
				if !isReservationKind(block, kind) {
					return planEditBuildResult{}, fmt.Errorf("selected block is %q, not a reservation/attachment block matching %q", stringField(block, "type"), firstNonEmpty(kind, "any"))
				}
				ops := []map[string]any{{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "ld": block}}
				report := baseEditReport("plan reservation remove", opts.planEditOptions, target)
				report.Section = ptrSectionReport(sec.Report)
				report.Block = summarizeBlock(block)
				report.BlockID = intAny(block["id"])
				report.BlockIndex = idx
				report.Operation = "remove " + firstNonEmpty(kind, "reservation") + " block"
				report.OpPaths = opPaths(ops)
				return planEditBuildResult{Ops: ops, Report: report}, nil
			})
		},
	}
	addPlanTargetFlags(cmd, &opts.planEditOptions)
	addPlanSectionFlags(cmd, &opts.planEditOptions)
	addPlanBlockFlags(cmd, &opts.planEditOptions)
	cmd.Flags().StringVar(&opts.kind, "kind", "", "Optional guard kind: flight, lodging, rental-car, restaurant, train, bus, ferry, cruise, attachment")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func addReservationAddFlags(cmd *cobra.Command, opts *planReservationOptions) {
	cmd.Flags().StringVar(&opts.kind, "kind", "", "Required kind: flight, lodging, rental-car, restaurant, train, bus, ferry, cruise, attachment")
	cmd.Flags().StringVar(&opts.title, "title", "", "Title for attachment blocks")
	cmd.Flags().StringVar(&opts.note, "note", "", "Optional note text")
	cmd.Flags().StringVar(&opts.confirmationNumber, "confirmation-number", "", "Confirmation, booking, ticket, or reservation number")
	cmd.Flags().StringVar(&opts.airline, "airline", "", "Airline name or IATA code for flights")
	cmd.Flags().StringVar(&opts.flightNumber, "flight-number", "", "Flight number")
	cmd.Flags().StringVar(&opts.departureAirport, "departure-airport", "", "Departure airport IATA/name for flights")
	cmd.Flags().StringVar(&opts.arrivalAirport, "arrival-airport", "", "Arrival airport IATA/name for flights")
	cmd.Flags().StringVar(&opts.departurePlaceID, "departure-place-id", "", "Departure Google/Wanderlog place id for transit or cruise")
	cmd.Flags().StringVar(&opts.arrivalPlaceID, "arrival-place-id", "", "Arrival Google/Wanderlog place id for transit or cruise")
	cmd.Flags().StringVar(&opts.pickupPlaceID, "pickup-place-id", "", "Pickup place id for rental cars")
	cmd.Flags().StringVar(&opts.dropoffPlaceID, "dropoff-place-id", "", "Drop-off place id for rental cars")
	cmd.Flags().StringVar(&opts.placeID, "place-id", "", "Place id for lodging or restaurant reservations")
	cmd.Flags().StringVar(&opts.query, "query", "", "Search query to resolve --place-id for lodging or restaurant")
	cmd.Flags().Float64Var(&opts.lat, "lat", 0, "Latitude for --query autocomplete bias")
	cmd.Flags().Float64Var(&opts.lng, "lng", 0, "Longitude for --query autocomplete bias")
	cmd.Flags().IntVar(&opts.radius, "radius", 50000, "Autocomplete radius in meters for --query")
	cmd.Flags().StringVar(&opts.language, "language", "en", "Language code for place details")
	cmd.Flags().StringVar(&opts.startDate, "start-date", "", "Start/departure/check-in date YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.startTime, "start-time", "", "Start/departure/check-in time HH:MM")
	cmd.Flags().StringVar(&opts.endDate, "end-date", "", "End/arrival/check-out date YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.endTime, "end-time", "", "End/arrival/check-out time HH:MM")
	cmd.Flags().StringVar(&opts.carrier, "carrier", "", "Carrier/operator for train, bus, or ferry")
	cmd.Flags().StringVar(&opts.cruiseLine, "cruise-line", "", "Cruise line")
	cmd.Flags().StringVar(&opts.shipName, "ship-name", "", "Cruise ship name")
	cmd.Flags().StringVar(&opts.voyageNumber, "voyage-number", "", "Cruise voyage number")
	cmd.Flags().StringArrayVar(&opts.travelerNames, "traveler-name", nil, "Traveler/guest name; repeatable")
	cmd.Flags().IntVar(&opts.partySize, "party-size", 0, "Restaurant party size")
	cmd.Flags().StringVar(&opts.nameForReservation, "name-for-reservation", "", "Restaurant reservation name")
	cmd.Flags().StringVar(&opts.url, "url", "", "Attachment URL")
	cmd.Flags().StringVar(&opts.filename, "filename", "", "Attachment filename")
	cmd.Flags().StringVar(&opts.mimeType, "mime-type", "", "Attachment MIME type")
	cmd.Flags().StringVar(&opts.jsonValue, "json-value", "", "Full block JSON object; overrides kind-specific fields")
	cmd.Flags().StringVar(&opts.lodgingOfferJSON, "lodging-offer-json", "", "One offer object from lodging search; used with --kind lodging when no Google place id is available")
	cmd.Flags().BoolVar(&opts.spanNights, "span-nights", true, "Copy lodging onto each dated day section in [start-date, end-date). Default true when --end-date is after --start-date")
	cmd.Flags().StringVar(&opts.displayName, "display-name", "", "Override lodging place.name after geocode")
	cmd.Flags().StringVar(&opts.expectNameSubstring, "expect-name-substring", "", "Fail if the geocoded lodging name does not contain this substring")
	cmd.Flags().IntVar(&opts.position, "position", -1, "Zero-based insertion position within section blocks; default appends")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the edit through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
}

func normalizeReservationKind(kind string) (string, error) {
	k := strings.TrimSpace(strings.ToLower(kind))
	k = strings.ReplaceAll(k, "_", "-")
	switch k {
	case "":
		return "", nil
	case "rental-car", "rentalcar", "car":
		return "rentalCar", nil
	case "hotel", "lodging":
		return "lodging", nil
	case "flight", "restaurant", "train", "bus", "ferry", "cruise", "attachment":
		return k, nil
	default:
		return "", fmt.Errorf("--kind must be flight, lodging, rental-car, restaurant, train, bus, ferry, cruise, or attachment")
	}
}

func validateReservationAddFlags(kind string, opts planReservationOptions) error {
	if kind == "" {
		return errors.New("--kind is required")
	}
	if opts.startTime != "" && !validClock(opts.startTime) {
		return errors.New("--start-time must be HH:MM")
	}
	if opts.endTime != "" && !validClock(opts.endTime) {
		return errors.New("--end-time must be HH:MM")
	}
	switch kind {
	case "flight":
		if opts.airline == "" {
			return errors.New("--airline is required for --kind flight")
		}
	case "lodging":
		if opts.placeID == "" && opts.query == "" && opts.lodgingOfferJSON == "" {
			return errors.New("--place-id, --query, or --lodging-offer-json is required for lodging reservations")
		}
	case "restaurant":
		if opts.placeID == "" && opts.query == "" {
			return errors.New("--place-id or --query is required for restaurant reservations")
		}
	case "rentalCar":
		if opts.pickupPlaceID == "" || opts.dropoffPlaceID == "" {
			return errors.New("--pickup-place-id and --dropoff-place-id are required for --kind rental-car")
		}
	case "train", "bus", "ferry", "cruise":
		if opts.departurePlaceID == "" || opts.arrivalPlaceID == "" {
			return errors.New("--departure-place-id and --arrival-place-id are required for transit/cruise reservations")
		}
	case "attachment":
		if opts.title == "" && opts.url == "" && opts.filename == "" {
			return errors.New("at least one of --title, --url, or --filename is required for --kind attachment")
		}
	}
	return nil
}

func buildReservationBlock(ctx context.Context, c *client.Client, kind string, opts planReservationOptions) (map[string]any, error) {
	if opts.jsonValue != "" {
		var block map[string]any
		if err := json.Unmarshal([]byte(opts.jsonValue), &block); err != nil {
			return nil, fmt.Errorf("parse --json-value: %w", err)
		}
		if block["id"] == nil {
			block["id"] = randomWanderlogID()
		}
		if block["addedBy"] == nil {
			block["addedBy"] = map[string]any{"type": "user"}
		}
		return block, nil
	}
	switch kind {
	case "flight":
		return newFlightReservationBlock(opts), nil
	case "lodging":
		block, _, err := newLodgingReservationWithEcho(ctx, c, opts)
		return block, err
	case "restaurant":
		place, err := resolveReservationPlace(ctx, c, opts, opts.placeID)
		if err != nil {
			return nil, err
		}
		return newRestaurantReservationBlock(place, opts), nil
	case "rentalCar":
		pickUp, err := fetchPlaceDetailsForBlock(ctx, c, opts.pickupPlaceID, opts.language)
		if err != nil {
			return nil, err
		}
		dropOff, err := fetchPlaceDetailsForBlock(ctx, c, opts.dropoffPlaceID, opts.language)
		if err != nil {
			return nil, err
		}
		return newRentalCarReservationBlock(pickUp, dropOff, opts), nil
	case "train", "bus", "ferry":
		depart, err := fetchPlaceDetailsForBlock(ctx, c, opts.departurePlaceID, opts.language)
		if err != nil {
			return nil, err
		}
		arrive, err := fetchPlaceDetailsForBlock(ctx, c, opts.arrivalPlaceID, opts.language)
		if err != nil {
			return nil, err
		}
		return newTransitReservationBlock(kind, depart, arrive, opts), nil
	case "cruise":
		depart, err := fetchPlaceDetailsForBlock(ctx, c, opts.departurePlaceID, opts.language)
		if err != nil {
			return nil, err
		}
		arrive, err := fetchPlaceDetailsForBlock(ctx, c, opts.arrivalPlaceID, opts.language)
		if err != nil {
			return nil, err
		}
		return newCruiseReservationBlock(depart, arrive, opts), nil
	case "attachment":
		return newStandaloneAttachmentBlock(opts), nil
	default:
		return nil, fmt.Errorf("unsupported reservation kind %q", kind)
	}
}

func buildReservationAdd(ctx context.Context, c *client.Client, kind string, opts planReservationOptions) (map[string]any, map[string]any, error) {
	if kind == "lodging" && opts.jsonValue == "" {
		return newLodgingReservationWithEcho(ctx, c, opts)
	}
	block, err := buildReservationBlock(ctx, c, kind, opts)
	if err != nil {
		return nil, nil, err
	}
	if kind != "lodging" {
		return block, nil, nil
	}
	echo, err := finalizeLodgingPlace(mapField(block, "place"), opts)
	if err != nil {
		return nil, echo, err
	}
	return block, echo, nil
}

func resolveReservationPlace(ctx context.Context, c *client.Client, opts planReservationOptions, placeID string) (map[string]any, error) {
	id := strings.TrimSpace(placeID)
	if id == "" && strings.TrimSpace(opts.query) != "" {
		if opts.lat == 0 && opts.lng == 0 {
			return nil, errors.New("--query requires --lat and --lng; use places autocomplete first if you do not have coordinates")
		}
		var err error
		id, err = firstAutocompletePlaceID(ctx, c, opts.planEditOptions)
		if err != nil {
			return nil, err
		}
	}
	if id == "" {
		return nil, errors.New("place id is required")
	}
	return fetchPlaceDetailsForBlock(ctx, c, id, opts.language)
}

func newFlightReservationBlock(opts planReservationOptions) map[string]any {
	block := map[string]any{
		"id":   randomWanderlogID(),
		"type": "flight",
		"flightInfo": map[string]any{
			"airline": map[string]any{"iata": opts.airline, "name": opts.airline},
			"number":  opts.flightNumber,
		},
		"depart":             airportStop("depart", opts.departureAirport, opts.startDate, opts.startTime),
		"arrive":             airportStop("arrive", opts.arrivalAirport, opts.endDate, opts.endTime),
		"addedBy":            map[string]any{"type": "user"},
		"text":               richText(opts.note),
		"travelerNames":      stringSliceAny(opts.travelerNames),
		"confirmationNumber": opts.confirmationNumber,
		"attachments":        []any{},
	}
	return block
}

func airportStop(stopType string, airport string, date string, time string) map[string]any {
	stop := map[string]any{"type": stopType, "airport": nil, "date": emptyStringNil(date), "time": emptyStringNil(time)}
	if airport != "" {
		stop["airport"] = map[string]any{"iata": strings.ToUpper(airport), "name": airport, "cityName": ""}
	}
	return stop
}

func newLodgingReservationWithEcho(ctx context.Context, c *client.Client, opts planReservationOptions) (map[string]any, map[string]any, error) {
	if opts.lodgingOfferJSON != "" {
		block, err := newLodgingReservationBlockFromOffer(opts)
		if err != nil {
			return nil, nil, err
		}
		echo, err := finalizeLodgingPlace(mapField(block, "place"), opts)
		if err != nil {
			return nil, echo, err
		}
		return block, echo, nil
	}
	place, err := resolveReservationPlace(ctx, c, opts, opts.placeID)
	if err != nil {
		return nil, nil, err
	}
	echo, err := finalizeLodgingPlace(place, opts)
	if err != nil {
		return nil, echo, err
	}
	return newLodgingReservationBlock(place, opts), echo, nil
}

func newLodgingReservationBlock(place map[string]any, opts planReservationOptions) map[string]any {
	block := newPlaceBlock(place, opts.note)
	block["hotel"] = map[string]any{
		"checkIn":            emptyStringNil(opts.startDate),
		"checkOut":           emptyStringNil(opts.endDate),
		"travelerNames":      stringSliceAny(opts.travelerNames),
		"confirmationNumber": emptyStringNil(opts.confirmationNumber),
	}
	return block
}

func newLodgingReservationBlockFromOffer(opts planReservationOptions) (map[string]any, error) {
	var offer map[string]any
	if err := json.Unmarshal([]byte(opts.lodgingOfferJSON), &offer); err != nil {
		return nil, fmt.Errorf("parse --lodging-offer-json: %w", err)
	}
	place, err := lodgingOfferPlace(offer)
	if err != nil {
		return nil, err
	}
	block := newLodgingReservationBlock(place, opts)
	if meta := summarizeLodgingOffer(offer); len(meta) > 0 {
		block["lodgingOffer"] = meta
	}
	return block, nil
}

func lodgingOfferPlace(offer map[string]any) (map[string]any, error) {
	lodging := mapField(offer, "lodging")
	if lodging == nil {
		lodging = offer
	}
	name := strings.TrimSpace(stringField(lodging, "name"))
	if name == "" {
		return nil, errors.New("--lodging-offer-json missing lodging.name")
	}
	location := mapField(lodging, "location")
	lat := firstNonZeroFloat(floatAny(location["latitude"]), floatAny(location["lat"]))
	lng := firstNonZeroFloat(floatAny(location["longitude"]), floatAny(location["lng"]))
	place := map[string]any{
		"name":     name,
		"place_id": lodgingOfferSyntheticPlaceID(offer, lodging),
		"types":    []any{"lodging"},
	}
	if lat != 0 || lng != 0 {
		place["geometry"] = map[string]any{"location": map[string]any{"lat": lat, "lng": lng}}
	}
	if photos := lodgingOfferPhotoURLs(lodging); len(photos) > 0 {
		place["photo_urls"] = photos
	}
	if priceRate := mapField(offer, "priceRate"); priceRate != nil {
		if url := stringField(priceRate, "bookingUrl"); url != "" {
			place["website"] = url
		}
	}
	if rating := mapField(lodging, "rating"); rating != nil {
		if value := floatAny(rating["value"]); value != 0 {
			place["rating"] = value
		}
	}
	if count := intAny(lodging["ratingCount"]); count != 0 {
		place["user_ratings_total"] = count
	}
	return place, nil
}

func lodgingOfferSyntheticPlaceID(offer map[string]any, lodging map[string]any) string {
	id := mapField(lodging, "id")
	idType := stringField(id, "type")
	for _, key := range []string{"listingId", "propertyId", "kayakKey"} {
		if value := strings.TrimSpace(fmt.Sprint(id[key])); value != "" && value != "<nil>" {
			if idType != "" {
				return "lodging:" + idType + ":" + value
			}
			return "lodging:" + value
		}
	}
	if offerID := strings.TrimSpace(stringField(offer, "offerId")); offerID != "" {
		return "lodging:offer:" + offerID
	}
	return "lodging:" + fmt.Sprint(randomWanderlogID())
}

func lodgingOfferPhotoURLs(lodging map[string]any) []any {
	images, _ := lodging["images"].([]any)
	out := make([]any, 0, len(images))
	for _, raw := range images {
		image, _ := raw.(map[string]any)
		url := firstNonEmpty(stringField(image, "url"), stringField(image, "thumbnailUrl"))
		if url != "" {
			out = append(out, url)
		}
	}
	return out
}

func summarizeLodgingOffer(offer map[string]any) map[string]any {
	lodging := mapField(offer, "lodging")
	if lodging == nil {
		lodging = offer
	}
	priceRate := mapField(offer, "priceRate")
	rating := mapField(lodging, "rating")
	location := mapField(lodging, "location")
	out := map[string]any{
		"offerId":         stringField(offer, "offerId"),
		"source":          firstNonEmpty(stringField(offer, "source"), stringField(priceRate, "source")),
		"lodgingId":       mapField(lodging, "id"),
		"name":            stringField(lodging, "name"),
		"wanderlogRating": lodging["wanderlogRating"],
		"ratingCount":     lodging["ratingCount"],
	}
	if location != nil {
		out["location"] = map[string]any{"lat": firstNonZeroFloat(floatAny(location["latitude"]), floatAny(location["lat"])), "lng": firstNonZeroFloat(floatAny(location["longitude"]), floatAny(location["lng"]))}
	}
	if rating != nil {
		out["rating"] = map[string]any{"source": stringField(rating, "source"), "value": rating["value"]}
	}
	if priceRate != nil {
		out["priceRate"] = map[string]any{
			"site":                stringField(priceRate, "site"),
			"source":              stringField(priceRate, "source"),
			"amount":              priceRate["amount"],
			"currencyCode":        stringField(priceRate, "currencyCode"),
			"total":               mapField(priceRate, "total"),
			"frequency":           stringField(priceRate, "frequency"),
			"bookingUrl":          stringField(priceRate, "bookingUrl"),
			"hasFreeCancellation": priceRate["hasFreeCancellation"],
		}
	}
	return out
}

func newRestaurantReservationBlock(place map[string]any, opts planReservationOptions) map[string]any {
	block := newPlaceBlock(place, opts.note)
	block["date"] = emptyStringNil(opts.startDate)
	if opts.startTime != "" {
		block["startTime"] = opts.startTime
	}
	if opts.endTime != "" {
		block["endTime"] = opts.endTime
	}
	if opts.partySize > 0 {
		block["partySize"] = opts.partySize
	}
	if opts.nameForReservation != "" {
		block["nameForReservation"] = opts.nameForReservation
	}
	if opts.confirmationNumber != "" {
		block["confirmationNumber"] = opts.confirmationNumber
	}
	return block
}

func newRentalCarReservationBlock(pickUp map[string]any, dropOff map[string]any, opts planReservationOptions) map[string]any {
	return map[string]any{
		"id":      randomWanderlogID(),
		"type":    "rentalCar",
		"addedBy": map[string]any{"type": "user"},
		"pickUp": map[string]any{
			"date":         opts.startDate,
			"time":         emptyStringNil(opts.startTime),
			"place":        pickUp,
			"airportOrGeo": airportOrGeoForPlace(pickUp),
		},
		"dropOff": map[string]any{
			"date":         opts.endDate,
			"time":         emptyStringNil(opts.endTime),
			"place":        dropOff,
			"airportOrGeo": airportOrGeoForPlace(dropOff),
		},
		"text":               richText(opts.note),
		"confirmationNumber": opts.confirmationNumber,
		"attachments":        []any{},
	}
}

func newTransitReservationBlock(kind string, depart map[string]any, arrive map[string]any, opts planReservationOptions) map[string]any {
	return map[string]any{
		"id":                 randomWanderlogID(),
		"type":               kind,
		"carrier":            opts.carrier,
		"depart":             placeStop(opts.startDate, opts.startTime, depart),
		"arrive":             placeStop(opts.endDate, opts.endTime, arrive),
		"addedBy":            map[string]any{"type": "user"},
		"text":               richText(opts.note),
		"attachments":        []any{},
		"confirmationNumber": opts.confirmationNumber,
	}
}

func newCruiseReservationBlock(depart map[string]any, arrive map[string]any, opts planReservationOptions) map[string]any {
	return map[string]any{
		"id":                 randomWanderlogID(),
		"type":               "cruise",
		"shipName":           opts.shipName,
		"cruiseLine":         opts.cruiseLine,
		"voyageNumber":       opts.voyageNumber,
		"depart":             placeStop(opts.startDate, opts.startTime, depart),
		"arrive":             placeStop(opts.endDate, opts.endTime, arrive),
		"portsOfCall":        []any{},
		"addedBy":            map[string]any{"type": "user"},
		"text":               richText(opts.note),
		"attachments":        []any{},
		"confirmationNumber": opts.confirmationNumber,
	}
}

func newStandaloneAttachmentBlock(opts planReservationOptions) map[string]any {
	title := firstNonEmpty(opts.title, opts.filename, opts.url, "Attachment")
	block := map[string]any{"id": randomWanderlogID(), "type": "attachment", "title": title, "attachments": []any{}, "addedBy": map[string]any{"type": "user"}}
	if opts.url != "" {
		attachment := map[string]any{"id": randomWanderlogID(), "type": "attachment", "url": opts.url}
		if opts.title != "" {
			attachment["title"] = opts.title
		}
		if opts.filename != "" {
			attachment["filename"] = opts.filename
		}
		if opts.mimeType != "" {
			attachment["mimeType"] = opts.mimeType
		}
		block["attachments"] = []any{attachment}
	}
	return block
}

func placeStop(date string, time string, place map[string]any) map[string]any {
	return map[string]any{"date": date, "time": emptyStringNil(time), "place": place}
}

func airportOrGeoForPlace(place map[string]any) map[string]any {
	return map[string]any{"type": "geo", "value": map[string]any{"id": intAny(place["geoId"]), "name": stringField(place, "name")}}
}

func emptyStringNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stringSliceAny(values []string) []any {
	out := []any{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func collectReservationBlocks(trip map[string]any, opts planEditOptions, kind string) ([]map[string]any, error) {
	var selected []resolvedSection
	if opts.day > 0 || opts.sectionIndex >= 0 || opts.sectionID != 0 {
		sec, err := resolveSection(trip, opts.day, opts.sectionIndex, opts.sectionID)
		if err != nil {
			return nil, err
		}
		selected = []resolvedSection{sec}
	} else {
		reports := sectionReports(trip)
		for i, raw := range sections(trip) {
			sec, _ := raw.(map[string]any)
			selected = append(selected, makeResolvedSection(i, reports[i], sec))
		}
	}
	var out []map[string]any
	for _, sec := range selected {
		for idx, raw := range sec.Blocks {
			block, _ := raw.(map[string]any)
			if !isReservationKind(block, kind) {
				continue
			}
			item := summarizeBlock(block)
			item["section"] = sec.Report
			item["block_index"] = idx
			item["kind"] = reservationKindForBlock(block)
			out = append(out, item)
		}
	}
	return out, nil
}

func isReservationKind(block map[string]any, kind string) bool {
	actual := reservationKindForBlock(block)
	if actual == "" {
		return false
	}
	return kind == "" || actual == kind
}

func reservationKindForBlock(block map[string]any) string {
	t := stringField(block, "type")
	if reservationBlockKinds[t] {
		return t
	}
	if t == "place" {
		if mapField(block, "hotel") != nil {
			return "lodging"
		}
		if block["partySize"] != nil || block["nameForReservation"] != nil || block["confirmationNumber"] != nil && block["date"] != nil {
			return "restaurant"
		}
	}
	return ""
}

// PATCH(amend-2026-08-23: geocode echo, --display-name, fail-closed --expect-name-substring)
func finalizeLodgingPlace(place map[string]any, opts planReservationOptions) (map[string]any, error) {
	echo := lodgingPlaceResolveEcho(place, opts.lat, opts.lng)
	if err := expectNameSubstring(stringField(place, "name"), opts.expectNameSubstring); err != nil {
		return echo, err
	}
	applyLodgingDisplayName(place, opts.displayName)
	return echo, nil
}

func applyLodgingDisplayName(place map[string]any, displayName string) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || place == nil {
		return
	}
	place["name"] = displayName
}

func expectNameSubstring(resolvedName, substr string) error {
	substr = strings.TrimSpace(substr)
	if substr == "" {
		return nil
	}
	if resolvedName == "" || !strings.Contains(strings.ToLower(resolvedName), strings.ToLower(substr)) {
		return fmt.Errorf("--expect-name-substring %q did not match resolved name %q", substr, resolvedName)
	}
	return nil
}

func lodgingPlaceResolveEcho(place map[string]any, lat, lng float64) map[string]any {
	echo := map[string]any{
		"resolved_name":    stringField(place, "name"),
		"resolved_address": firstNonEmpty(stringField(place, "formatted_address"), stringField(place, "vicinity")),
	}
	if plat, plng, ok := placeLatLng(place); ok && (lat != 0 || lng != 0) {
		echo["distance_m"] = int(math.Round(metersBetween(lat, lng, plat, plng)))
	}
	return echo
}

func placeLatLng(place map[string]any) (float64, float64, bool) {
	if place == nil {
		return 0, 0, false
	}
	loc := mapField(mapField(place, "geometry"), "location")
	if loc == nil {
		loc = mapField(place, "location")
	}
	if loc == nil {
		return 0, 0, false
	}
	latKey, lngKey := "lat", "lng"
	if _, ok := loc["lat"]; !ok {
		latKey = "latitude"
	}
	if _, ok := loc["lng"]; !ok {
		lngKey = "longitude"
	}
	if _, ok := loc[latKey]; !ok {
		return 0, 0, false
	}
	if _, ok := loc[lngKey]; !ok {
		return 0, 0, false
	}
	return floatAny(loc[latKey]), floatAny(loc[lngKey]), true
}

func metersBetween(lat1, lng1, lat2, lng2 float64) float64 {
	const earthM = 6371000.0
	p1 := lat1 * math.Pi / 180
	p2 := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthM * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// PATCH(amend-2026-08-23: copy lodging onto each night in [checkIn, checkOut) as one JSON0 li array)
func buildLodgingReservationInsert(target map[string]any, opts planReservationOptions, block map[string]any, echo map[string]any) (planEditBuildResult, error) {
	nights, err := lodgingInsertTargets(target, opts, block)
	if err != nil {
		return planEditBuildResult{}, err
	}
	ops, inserted, nightReports := lodgingInsertOps(nights, block, opts.position)
	report := baseEditReport("plan reservation add", opts.planEditOptions, target)
	first := inserted[0]
	report.Section = ptrSectionReport(nights[0].Report)
	report.Block = summarizeBlock(first)
	report.BlockID = intAny(first["id"])
	report.BlockIndex = normalizeInsertPosition(opts.position, len(nights[0].Blocks))
	report.Operation = "insert lodging block"
	if len(nights) > 1 {
		report.Operation = "insert lodging block on each night"
	}
	report.OpPaths = opPaths(ops)
	if report.Block == nil {
		report.Block = map[string]any{}
	}
	for key, value := range echo {
		report.Block[key] = value
	}
	report.Block["nights"] = nightReports
	return planEditBuildResult{Ops: ops, Report: report}, nil
}

func lodgingInsertTargets(trip map[string]any, opts planReservationOptions, block map[string]any) ([]resolvedSection, error) {
	start, end := lodgingStayDates(opts, block)
	if shouldSpanLodgingNights(opts.spanNights, start, end) {
		return lodgingNightSections(trip, start, end)
	}
	if opts.day > 0 || opts.sectionIndex >= 0 || opts.sectionID != 0 {
		sec, err := resolveSection(trip, opts.day, opts.sectionIndex, opts.sectionID)
		if err != nil {
			return nil, err
		}
		return []resolvedSection{sec}, nil
	}
	if start != "" {
		return lodgingNightSections(trip, start, start)
	}
	return nil, errors.New("one of --day, --section-index, --section-id, or --start-date is required for lodging")
}

func lodgingStayDates(opts planReservationOptions, block map[string]any) (string, string) {
	start := strings.TrimSpace(opts.startDate)
	end := strings.TrimSpace(opts.endDate)
	hotel := mapField(block, "hotel")
	if start == "" {
		start = stringField(hotel, "checkIn")
	}
	if end == "" {
		end = stringField(hotel, "checkOut")
	}
	return start, end
}

func shouldSpanLodgingNights(spanNights bool, start, end string) bool {
	if !spanNights {
		return false
	}
	startDay, err1 := time.Parse("2006-01-02", strings.TrimSpace(start))
	endDay, err2 := time.Parse("2006-01-02", strings.TrimSpace(end))
	if err1 != nil || err2 != nil {
		return false
	}
	return endDay.After(startDay)
}

func lodgingStayNightDates(checkIn, checkOut string) ([]string, error) {
	checkIn = strings.TrimSpace(checkIn)
	checkOut = strings.TrimSpace(checkOut)
	if checkIn == "" {
		return nil, errors.New("--start-date is required to place lodging on day sections")
	}
	start, err := time.Parse("2006-01-02", checkIn)
	if err != nil {
		return nil, fmt.Errorf("--start-date must be YYYY-MM-DD")
	}
	dates := []string{start.Format("2006-01-02")}
	if checkOut == "" {
		return dates, nil
	}
	end, err := time.Parse("2006-01-02", checkOut)
	if err != nil {
		return nil, fmt.Errorf("--end-date must be YYYY-MM-DD")
	}
	if !end.After(start) {
		return dates, nil
	}
	dates = dates[:0]
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		dates = append(dates, day.Format("2006-01-02"))
	}
	return dates, nil
}

func lodgingNightSections(trip map[string]any, checkIn, checkOut string) ([]resolvedSection, error) {
	dates, err := lodgingStayNightDates(checkIn, checkOut)
	if err != nil {
		return nil, err
	}
	reports := sectionReports(trip)
	secs := sections(trip)
	byDate := map[string]int{}
	for i, raw := range secs {
		sec, _ := raw.(map[string]any)
		date := stringField(sec, "date")
		if date == "" {
			continue
		}
		mode := stringField(sec, "mode")
		if mode != "dayPlan" && mode != "guideDayPlan" {
			continue
		}
		if _, exists := byDate[date]; !exists {
			byDate[date] = i
		}
	}
	var out []resolvedSection
	var missing []string
	for _, date := range dates {
		index, ok := byDate[date]
		if !ok {
			missing = append(missing, date)
			continue
		}
		sec, _ := secs[index].(map[string]any)
		out = append(out, makeResolvedSection(index, reports[index], sec))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no dated day section for lodging night(s) %s", strings.Join(missing, ", "))
	}
	if len(out) == 0 {
		return nil, errors.New("no dated day sections matched lodging stay dates")
	}
	return out, nil
}

func lodgingInsertOps(nights []resolvedSection, template map[string]any, position int) ([]map[string]any, []map[string]any, []map[string]any) {
	ops := make([]map[string]any, 0, len(nights))
	blocks := make([]map[string]any, 0, len(nights))
	reports := make([]map[string]any, 0, len(nights))
	seen := map[int]bool{}
	for _, sec := range nights {
		block := cloneJSONMap(template)
		block["id"] = uniqueWanderlogID(seen)
		idx := normalizeInsertPosition(position, len(sec.Blocks))
		ops = append(ops, map[string]any{"p": []any{"itinerary", "sections", sec.Index, "blocks", idx}, "li": block})
		blocks = append(blocks, block)
		reports = append(reports, map[string]any{
			"day":      sec.Report.Day,
			"date":     sec.Report.Date,
			"block_id": intAny(block["id"]),
			"name":     stringField(mapField(block, "place"), "name"),
		})
	}
	return ops, blocks, reports
}

func uniqueWanderlogID(seen map[int]bool) int {
	for range 16 {
		id := randomWanderlogID()
		if !seen[id] {
			seen[id] = true
			return id
		}
	}
	id := randomWanderlogID()
	seen[id] = true
	return id
}
