// Public-source examples supply explicit observed inputs to raw source routes.
// Keep this metadata separate from generated constructors so regeneration
// preserves usable examples without changing their supported flags or behavior.
package cli

import (
	"github.com/spf13/cobra"
	"strings"
)

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, _ *rootFlags) {
		for _, sample := range []struct{ path, example, happyArgs string }{
			{"hotel get-51870.html", "  rakuten-travel-pp-cli hotel get-51870.html 51870", "hotel_id=51870"},
			{"hotel get-facilities-policies", "  rakuten-travel-pp-cli hotel get-facilities-policies 51870", "hotel_id=51870"},
			{"yado list-e.html", "  rakuten-travel-pp-cli yado list-e.html tokyo E", "prefecture=tokyo;area=E"},
			{"keyword", "  rakuten-travel-pp-cli keyword --charset utf-8 --query 品川 --page-size 3 --page 1", "--charset=utf-8;--query=品川;--page-size=3;--page=1"},
			{"hotelinfo", "  rakuten-travel-pp-cli hotelinfo 51870 --f-flg PLAN --rooms 1 --adults-per-room 2 --checkin-year 2026 --checkin-month 11 --checkin-day 8 --checkout-year 2026 --checkout-month 11 --checkout-day 10 --page 1", "hotel_id=51870;--f-flg=PLAN;--rooms=1;--adults-per-room=2;--checkin-year=2026;--checkin-month=11;--checkin-day=8;--checkout-year=2026;--checkout-month=11;--checkout-day=10;--page=1"},
		} {
			cmd, remaining, err := rootCmd.Find(strings.Fields(sample.path))
			if err != nil || len(remaining) != 0 {
				continue
			}
			cmd.Example = sample.example
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations["pp:happy-args"] = sample.happyArgs
		}
	})
}
