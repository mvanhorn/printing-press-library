package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type personalizationAuditRow struct {
	Position       string   `json:"position"`
	VariantIDs     []string `json:"variant_ids,omitempty"`
	ImageID        string   `json:"image_id,omitempty"`
	Name           string   `json:"name,omitempty"`
	Type           string   `json:"type,omitempty"`
	InputText      string   `json:"input_text,omitempty"`
	FontFamily     string   `json:"font_family,omitempty"`
	FontSize       float64  `json:"font_size,omitempty"`
	FontColor      string   `json:"font_color,omitempty"`
	TextAlign      string   `json:"text_align,omitempty"`
	Supported      bool     `json:"supported"`
	MissingFields  []string `json:"missing_fields,omitempty"`
	UnsupportedGap string   `json:"unsupported_gap,omitempty"`
}

type placementMatrixRow struct {
	VariantID      string  `json:"variant_id,omitempty"`
	PrintArea      string  `json:"print_area,omitempty"`
	ImageID        string  `json:"image_id,omitempty"`
	ImageName      string  `json:"image_name,omitempty"`
	UploadFileName string  `json:"upload_file_name,omitempty"`
	X              float64 `json:"x,omitempty"`
	Y              float64 `json:"y,omitempty"`
	Scale          float64 `json:"scale,omitempty"`
	Angle          float64 `json:"angle,omitempty"`
	Missing        bool    `json:"missing"`
}

type productDriftRow struct {
	Path     string `json:"path"`
	Status   string `json:"status"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

func newPersonalizationAuditCmd(flags *rootFlags) *cobra.Command {
	var productID, productFile, dbPath string
	cmd := &cobra.Command{
		Use:     "personalization-audit",
		Short:   "Audit documented Printify personalization placeholder fields",
		Example: "  printify-pp-cli personalization-audit --product-file ./examples/sample-product.json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if productID == "" && productFile == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			product, err := ppLoadProduct(productFile, dbPath, productID)
			if err != nil {
				return err
			}
			rows := buildPersonalizationAudit(product)
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&productID, "product-id", "", "Product ID to read from the local store")
	cmd.Flags().StringVar(&productFile, "product-file", "", "Product JSON file to audit")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite store path")
	return cmd
}

func newPlacementMatrixCmd(flags *rootFlags) *cobra.Command {
	var productID, productFile, uploadsFile, dbPath string
	cmd := &cobra.Command{
		Use:     "placement-matrix",
		Short:   "Show variant and print-area artwork placement rows",
		Example: "  printify-pp-cli placement-matrix --product-file ./examples/sample-product.json --uploads-file ./examples/sample-uploads.json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if productID == "" && productFile == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			product, err := ppLoadProduct(productFile, dbPath, productID)
			if err != nil {
				return err
			}
			uploads, uploadsErr := ppLoadObjectsFromFileOrStore(uploadsFile, dbPath, []string{"uploads-json", "uploads_json"}, 10000)
			if uploadsErr != nil && uploadsFile != "" {
				return uploadsErr
			}
			rows := buildPlacementMatrix(product, uploads)
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&productID, "product-id", "", "Product ID to read from the local store")
	cmd.Flags().StringVar(&productFile, "product-file", "", "Product JSON file to inspect")
	cmd.Flags().StringVar(&uploadsFile, "uploads-file", "", "Uploads JSON file for image names")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite store path")
	return cmd
}

func newProductDriftCmd(flags *rootFlags) *cobra.Command {
	var productID, productFile, manifestFile, dbPath string
	cmd := &cobra.Command{
		Use:     "product-drift",
		Short:   "Compare a product manifest with current Printify product JSON",
		Example: "  printify-pp-cli product-drift --product-file ./examples/current-product.json --manifest ./examples/sample-product.json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if manifestFile == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			product, err := ppLoadProduct(productFile, dbPath, productID)
			if err != nil {
				return err
			}
			rawManifest, err := ppLoadJSONFile(manifestFile)
			if err != nil {
				return err
			}
			manifest, err := ppDecodeObject(rawManifest)
			if err != nil {
				return err
			}
			rows := buildProductDrift(manifest, product)
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&productID, "product-id", "", "Product ID to read from the local store")
	cmd.Flags().StringVar(&productFile, "product-file", "", "Current product JSON file")
	cmd.Flags().StringVar(&manifestFile, "manifest", "", "Intended product manifest JSON file")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite store path")
	return cmd
}

func buildPersonalizationAudit(product ppJSONObj) []personalizationAuditRow {
	var rows []personalizationAuditRow
	for _, printAreaValue := range ppArray(ppLookup(product, "print_areas")) {
		printArea := ppObject(printAreaValue)
		var variantIDs []string
		for _, variantValue := range ppArray(ppLookup(printArea, "variant_ids")) {
			variantIDs = append(variantIDs, fmt.Sprint(variantValue))
		}
		for _, placeholderValue := range ppArray(ppLookup(printArea, "placeholders")) {
			placeholder := ppObject(placeholderValue)
			position := ppString(placeholder, "position")
			images := ppArray(ppLookup(placeholder, "images"))
			if len(images) == 0 {
				rows = append(rows, personalizationAuditRow{
					Position:       position,
					VariantIDs:     variantIDs,
					Supported:      false,
					MissingFields:  []string{"images"},
					UnsupportedGap: "Printify UI personalization enablement is not exposed by the public API",
				})
				continue
			}
			for _, imageValue := range images {
				image := ppObject(imageValue)
				row := personalizationAuditRow{
					Position:   position,
					VariantIDs: variantIDs,
					ImageID:    ppString(image, "id"),
					Name:       ppString(image, "name"),
					Type:       ppString(image, "type"),
					InputText:  ppString(image, "input_text"),
					FontFamily: ppString(image, "font_family"),
					FontSize:   ppFloat(image, "font_size"),
					FontColor:  ppString(image, "font_color"),
					TextAlign:  ppString(image, "text_align"),
					Supported:  true,
				}
				for _, field := range []string{"id", "name", "type", "input_text"} {
					if ppString(image, field) == "" {
						row.MissingFields = append(row.MissingFields, field)
					}
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func buildPlacementMatrix(product ppJSONObj, uploads []ppJSONObj) []placementMatrixRow {
	uploadNames := map[string]string{}
	for _, upload := range uploads {
		id := ppString(upload, "id")
		if id != "" {
			uploadNames[id] = ppString(upload, "file_name", "filename", "name")
		}
	}
	var rows []placementMatrixRow
	for _, printAreaValue := range ppArray(ppLookup(product, "print_areas")) {
		printArea := ppObject(printAreaValue)
		variantIDs := ppArray(ppLookup(printArea, "variant_ids"))
		for _, placeholderValue := range ppArray(ppLookup(printArea, "placeholders")) {
			placeholder := ppObject(placeholderValue)
			position := ppString(placeholder, "position")
			images := ppArray(ppLookup(placeholder, "images"))
			if len(images) == 0 {
				rows = appendPlacementRows(rows, variantIDs, placementMatrixRow{PrintArea: position, Missing: true})
				continue
			}
			for _, imageValue := range images {
				image := ppObject(imageValue)
				imageID := ppString(image, "id")
				base := placementMatrixRow{
					PrintArea:      position,
					ImageID:        imageID,
					ImageName:      ppString(image, "name"),
					UploadFileName: uploadNames[imageID],
					X:              ppFloat(image, "x"),
					Y:              ppFloat(image, "y"),
					Scale:          ppFloat(image, "scale"),
					Angle:          ppFloat(image, "angle"),
				}
				rows = appendPlacementRows(rows, variantIDs, base)
			}
		}
	}
	return rows
}

func appendPlacementRows(rows []placementMatrixRow, variantIDs []any, base placementMatrixRow) []placementMatrixRow {
	if len(variantIDs) == 0 {
		return append(rows, base)
	}
	for _, variantID := range variantIDs {
		row := base
		row.VariantID = fmt.Sprint(variantID)
		rows = append(rows, row)
	}
	return rows
}

func buildProductDrift(expected, actual ppJSONObj) []productDriftRow {
	paths := []string{"title", "description", "blueprint_id", "print_provider_id", "variants", "print_areas"}
	for _, key := range ppSortedKeys(expected) {
		if !containsString(paths, key) {
			paths = append(paths, key)
		}
	}
	sort.Strings(paths)
	rows := make([]productDriftRow, 0, len(paths))
	for _, path := range paths {
		expectedValue := ppLookup(expected, path)
		actualValue := ppLookup(actual, path)
		status := "match"
		if !reflect.DeepEqual(expectedValue, actualValue) {
			status = "drift"
		}
		rows = append(rows, productDriftRow{Path: path, Status: status, Expected: expectedValue, Actual: actualValue})
	}
	return rows
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}
