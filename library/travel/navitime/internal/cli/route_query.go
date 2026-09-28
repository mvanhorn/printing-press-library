package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"github.com/spf13/cobra"
)

type routeQueryFlags struct {
	query   navitime.Query
	limit   int
	refresh bool
	passes  []string
}

func (f *routeQueryFlags) validate(cmd *cobra.Command, args []string, flags *rootFlags) error {
	if len(args) != 0 {
		return usageErr(fmt.Errorf("%s accepts flags only; use --from REF --to REF and exactly one time mode", cmd.CommandPath()))
	}
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("%s needs the public website or response cache; use --data-source auto or live", cmd.CommandPath()))
	}
	if strings.TrimSpace(f.query.From) == "" {
		return usageErr(fmt.Errorf("--from is required; use a reference returned by places search"))
	}
	if strings.TrimSpace(f.query.To) == "" {
		return usageErr(fmt.Errorf("--to is required; use a reference returned by places search"))
	}
	if err := boundedLimit(f.limit, 10); err != nil {
		return err
	}
	if len(f.passes) > 1 {
		return usageErr(fmt.Errorf("--pass accepts one ID per anonymous query; select one ID from passes list"))
	}
	if len(f.passes) == 1 {
		f.query.Pass = f.passes[0]
	}
	if err := navitime.ValidateQuery(f.query); err != nil {
		return navitimeError(err)
	}
	return nil
}
