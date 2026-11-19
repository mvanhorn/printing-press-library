// Hand-authored (Slice 2): listings copy.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if p := s2FindChild(root, "listings"); p != nil {
			addNovelCommandIfAbsent(p, newS2ListingsCopyCmd(flags))
		}
	})
}

// s2DraftFields are the createDraftListing body properties (from the Etsy
// OpenAPI spec). Anything else on a fetched listing (ids, state, timestamps,
// url, views, images...) is stripped from the copy.
var s2DraftFields = []string{
	"quantity", "title", "description", "price", "who_made", "when_made", "taxonomy_id",
	"shipping_profile_id", "return_policy_id", "materials", "shop_section_id",
	"processing_min", "processing_max", "readiness_state_id", "tags", "styles",
	"item_weight", "item_length", "item_width", "item_height", "item_weight_unit",
	"item_dimensions_unit", "production_partner_ids",
	"ecgt_garan_brand", "ecgt_garan_years", "ecgt_garan_model", "ecgt_garan_guarantee_details",
	"ecgt_other_commercial_guarantee_details", "ecgt_after_sales_service_info", "ecgt_software_update_details",
	"gpsr_manufacturer_type", "gpsr_manufacturer_id", "gpsr_responsible_person_type", "gpsr_responsible_person_id",
	"gpsr_serial_number", "gpsr_safety_warnings",
	"is_supply", "is_customizable", "should_auto_renew", "is_taxable", "type",
}

var s2DraftRequired = []string{"quantity", "title", "description", "price", "who_made", "when_made", "taxonomy_id"}

// s2DraftFromListing builds the createDraftListing payload from a fetched
// listing and reports which source keys were stripped.
func s2DraftFromListing(src map[string]any) (draft map[string]any, stripped []string) {
	draft = map[string]any{}
	keep := map[string]bool{}
	for _, k := range s2DraftFields {
		keep[k] = true
	}
	for k, v := range src {
		if !keep[k] {
			stripped = append(stripped, k)
			continue
		}
		if v == nil {
			continue
		}
		if k == "price" {
			if f, ok := s2MoneyFloat(v); ok {
				draft[k] = f
			}
			continue
		}
		switch t := v.(type) {
		case string:
			if t == "" {
				continue
			}
		case []any:
			if len(t) == 0 {
				continue
			}
		}
		draft[k] = v
	}
	sort.Strings(stripped)
	return draft, stripped
}

func s2DraftForm(draft map[string]any) url.Values {
	form := url.Values{}
	for k, v := range draft {
		switch t := v.(type) {
		case []any:
			parts := make([]string, 0, len(t))
			for _, e := range t {
				parts = append(parts, fmt.Sprint(e))
			}
			form.Set(k, strings.Join(parts, ","))
		case bool:
			form.Set(k, strconv.FormatBool(t))
		case float64:
			form.Set(k, strconv.FormatFloat(t, 'f', -1, 64))
		default:
			form.Set(k, fmt.Sprint(t))
		}
	}
	return form
}

type s2CopyResult struct {
	Mode            string          `json:"mode"`
	SourceListing   int64           `json:"source_listing_id"`
	Draft           map[string]any  `json:"draft"`
	Stripped        []string        `json:"stripped_fields"`
	MissingRequired []string        `json:"missing_required"`
	CreatedListing  json.RawMessage `json:"created,omitempty"`
	Note            string          `json:"note"`
}

func newS2ListingsCopyCmd(flags *rootFlags) *cobra.Command {
	var shopFlag, title string
	cmd := &cobra.Command{
		Use:   "copy [listing_id]",
		Short: "Duplicate a listing as a new draft; use --dry-run to preview or --yes to create.",
		Long: strings.Trim(`
Fetches the listing and builds a createDraftListing payload from it, with ids,
state, URL, view/favorite counts and timestamps removed. With --dry-run it only
PRINTS that draft payload (one read request, nothing created). Without --yes or
--dry-run it refuses before fetching. With --yes it
POSTs it and creates a new draft in your shop. Images are not copied; upload
them to the draft afterwards. Use --title to give the copy its own title.
Requires the listings_w OAuth scope.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli listings copy 1000000001 --dry-run
  etsy-pp-cli listings copy 1000000001 --title "Linen apron, forest green" --yes
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flags.dryRun && len(args) == 0 {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings copy")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("listing_id is required\nUsage: %s <listing_id>", cmd.CommandPath()))
			}
			id, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || id <= 0 {
				return usageErr(fmt.Errorf("listing_id %q must be a positive integer", args[0]))
			}
			// Refuse before the source fetch; --dry-run explicitly opts into a read-only preview.
			if !flags.yes && !flags.dryRun {
				return classifyAPIError(cmd.OutOrStdout(), &client.WriteNotConfirmedError{Method: "POST", Path: "/shops/" + bulkShopIDForCmd(cmd, shopFlag) + "/listings"}, flags)
			}
			apply := flags.yes && !flags.dryRun
			if apply && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "listings copy --yes")
			}
			shop := bulkShopIDForCmd(cmd, shopFlag)
			if apply && shop == "" {
				return usageErr(fmt.Errorf("a shop id is required to create the draft: set ETSY_SHOP_ID or pass --shop-id"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// read-before-write / snapshot / refresh reads must see live data, never the 5-minute response cache
			c.NoCache = true
			if flags.dryRun {
				c.DryRun = false
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			raw, err := c.Get(ctx, "/listings/"+strconv.FormatInt(id, 10), nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			src, err := s2DecodeObject(raw)
			if err != nil {
				return apiErr(fmt.Errorf("parsing listing %d: %w", id, err))
			}
			draft, stripped := s2DraftFromListing(src)
			if t := strings.TrimSpace(title); t != "" {
				draft["title"] = t
			}
			missing := make([]string, 0)
			for _, k := range s2DraftRequired {
				if _, ok := draft[k]; !ok {
					missing = append(missing, k)
				}
			}
			res := s2CopyResult{Mode: "preview", SourceListing: id, Draft: draft, Stripped: stripped, MissingRequired: missing,
				Note: "preview only; re-run with --yes to create this draft (images are not copied)"}
			if stripped == nil {
				res.Stripped = make([]string, 0)
			}
			if !apply {
				return s2PrintJSON(cmd, flags, res)
			}
			if len(missing) > 0 {
				_ = s2PrintJSON(cmd, flags, res)
				return usageErr(fmt.Errorf("cannot create: draft is missing required field(s) %s; pass --title or fix the source listing", strings.Join(missing, ", ")))
			}
			data, _, err := c.PostFormWithParams(ctx, "/shops/"+url.PathEscape(shop)+"/listings", nil, s2DraftForm(draft))
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			res.Mode, res.Note = "created", "draft created (not published); add images, then activate it"
			if json.Valid(data) {
				res.CreatedListing = data
			}
			return s2PrintJSON(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Title for the copy (default: the source title)")
	cmd.Flags().StringVar(&shopFlag, "shop-id", "", "Shop id to create the draft in (default: ETSY_SHOP_ID)")
	return cmd
}
