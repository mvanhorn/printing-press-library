package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, entry := range []struct {
			parent string
			cmd    *cobra.Command
		}{
			{"shops", newToyotaShopGetCmd(flags)}, {"rental", newToyotaEligibilityCmd(flags)},
		} {
			parent, _, err := root.Find([]string{entry.parent})
			if err == nil && parent.Name() == entry.parent {
				addNovelCommandIfAbsent(parent, entry.cmd)
			}
		}
	})
}

func toyotaError(err error) error {
	var input *toyota.InputError
	var missing *toyota.NotFoundError
	var limited *cliutil.RateLimitError
	switch {
	case errors.As(err, &input):
		return usageErr(err)
	case errors.As(err, &missing):
		return notFoundErr(err)
	case errors.As(err, &limited):
		return rateLimitErr(err)
	default:
		return classifyAPIErrorOnly(err)
	}
}

func toyotaGuard(cmd *cobra.Command, args []string, flags *rootFlags, action string) (bool, error) {
	if dryRunOK(flags) {
		return true, writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	if len(args) > 0 {
		return true, usageErr(fmt.Errorf("%s accepts flags only; run %s --help", action, cmd.CommandPath()))
	}
	if flags.dataSource == "local" {
		return true, usageErr(fmt.Errorf("%s requires live Toyota data; use --data-source live or auto", action))
	}
	return false, nil
}

// Declare real inputs to MCP discovery while preserving input-free dry runs.
func toyotaRequireFlags(cmd *cobra.Command, flags *rootFlags, names ...string) {
	mark := func() {
		for _, name := range names {
			if err := cmd.MarkFlagRequired(name); err != nil {
				panic(err) // Constructor bug: every listed flag is defined.
			}
		}
	}
	mark()
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if dryRunOK(flags) {
			for _, name := range names {
				delete(cmd.Flags().Lookup(name).Annotations, cobra.BashCompOneRequiredFlag)
			}
			return nil
		}
		mark()
		if err := cmd.ValidateRequiredFlags(); err != nil {
			return usageErr(err)
		}
		return nil
	}
}

type toyotaShopResult struct {
	Meta toyota.Metadata `json:"meta"`
	Shop toyota.Shop     `json:"shop"`
	Note string          `json:"note"`
}

// Quiet output uses domain identities. Agent JSON keeps its structured result
// and provenance even when quiet is also requested.
func toyotaPrint(cmd *cobra.Command, flags *rootFlags, value any) error {
	if flags.quiet && flags.agent && flags.asJSON && !flags.csv && !flags.plain {
		structured := *flags
		structured.quiet = false
		return structured.printJSON(cmd, value)
	}
	if !flags.quiet || flags.selectFields != "" {
		return flags.printJSON(cmd, value)
	}
	var identities []string
	switch out := value.(type) {
	case toyotaShopResult:
		identities = []string{out.Shop.ID}
	case toyota.ShopsResult:
		for _, shop := range out.Shops {
			identities = append(identities, shop.ID)
		}
	case toyota.QuoteResult:
		for _, offer := range out.Offers {
			identities = append(identities, offer.Class)
		}
	case toyota.OptionsResult:
		for _, option := range out.Options {
			identities = append(identities, option.Code)
		}
	case toyota.EligibilityResult:
		for _, path := range out.Paths {
			identities = append(identities, path.License)
		}
	case toyota.OneWayResult:
		identities = []string{out.PickupShop.ID + "->" + out.DropoffShop.ID + ":" + out.Family}
	case toyota.Handoff:
		identities = []string{out.BookingURL}
	default:
		return flags.printJSON(cmd, value)
	}
	for _, identity := range identities {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), identity); err != nil {
			return err
		}
	}
	return nil
}

func newToyotaShopGetCmd(flags *rootFlags) *cobra.Command {
	var id string
	cmd := &cobra.Command{Use: "get", Short: "Read one exact Toyota branch and return restrictions",
		Example:     "  toyota-rentacar-pp-cli shops get --id 63601:01V --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=63601:01V"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := toyotaGuard(cmd, args, flags, "shops get"); stop {
				return err
			}
			if id == "" {
				return usageErr(fmt.Errorf("--id is required; resolve an ID with shops search"))
			}
			if _, _, err := toyota.ParseShopID(id); err != nil {
				return toyotaError(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := toyota.NewClient(flags.rateLimit)
			out, err := c.GetShop(ctx, id)
			if err != nil {
				return toyotaError(err)
			}
			return toyotaPrint(cmd, flags, toyotaShopResult{Meta: c.Meta(), Shop: out, Note: "Operating hours do not establish dated vehicle inventory."})
		}}
	cmd.Flags().StringVar(&id, "id", "", "Exact company:branch source ID,e.g. 63601:01V")
	toyotaRequireFlags(cmd, flags, "id")
	return cmd
}

type toyotaRentalFlags struct {
	pickupID, dropoffID, pickup, dropoff, transmission, seats string
	fourWD, winter                                            bool
}

func (f *toyotaRentalFlags) parse() (toyota.Period, toyota.SearchOptions, error) {
	if f.pickupID == "" || f.pickup == "" || f.dropoff == "" {
		return toyota.Period{}, toyota.SearchOptions{}, &toyota.InputError{Message: "--pickup-shop,--pickup and --dropoff are required; see --help for a dated example"}
	}
	if f.dropoffID == "" {
		f.dropoffID = f.pickupID
	}
	if _, _, err := toyota.ParseShopID(f.pickupID); err != nil {
		return toyota.Period{}, toyota.SearchOptions{}, err
	}
	if _, _, err := toyota.ParseShopID(f.dropoffID); err != nil {
		return toyota.Period{}, toyota.SearchOptions{}, err
	}
	period, err := toyota.ParsePeriod(f.pickup, f.dropoff, time.Now())
	if err != nil {
		return period, toyota.SearchOptions{}, err
	}
	o := toyota.SearchOptions{Transmission: f.transmission, FourWD: f.fourWD, WinterTires: f.winter, Seats: []string{}}
	if f.seats != "" {
		for _, s := range strings.Split(f.seats, ",") {
			o.Seats = append(o.Seats, strings.TrimSpace(s))
		}
	}
	return period, o, o.Validate()
}

func newToyotaEligibilityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "eligibility", Short: "Read Toyota's official driving-document guidance",
		Long:        "Return source license/document paths and landing-evidence notes. This CLI collects no personal documents and does not determine anyone's eligibility; Toyota verifies the actual documents at rental.",
		Example:     "  toyota-rentacar-pp-cli rental eligibility --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := toyotaGuard(cmd, args, flags, "rental eligibility"); stop {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			out, err := toyota.NewClient(flags.rateLimit).Eligibility(ctx)
			if err != nil {
				return toyotaError(err)
			}
			return toyotaPrint(cmd, flags, out)
		}}
	return cmd
}
