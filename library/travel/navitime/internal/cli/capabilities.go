// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"github.com/spf13/cobra"
)

func newCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Report verified, advertised, gated and unavailable source capabilities.",
		Example: strings.Trim(`
  navitime-pp-cli capabilities
  navitime-pp-cli capabilities --select capabilities.id,capabilities.status
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return navitimeDryRun(cmd, flags, "capabilities")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("capabilities accepts no positional arguments; use --select for projection"))
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("capabilities is dated research metadata; use --data-source auto or local"))
			}
			started := time.Now()
			// pp:novel-static-reference
			// This is dated research metadata, never a fabricated API response.
			value := struct {
				Meta         map[string]any   `json:"meta"`
				Capabilities []map[string]any `json:"capabilities"`
			}{
				Meta: map[string]any{"data_kind": "research_reference", "research_date": "2026-09-27", "source_url": "https://japantravel.navitime.com/en/area/jp/route/", "source_updated_at": nil, "credentials_required": false, "browser_runtime_required": false},
				Capabilities: []map[string]any{
					{"id": "places", "status": "verified", "implemented": true, "command": "places search", "source_url": "https://japantravel.navitime.com/en/area/jp/route/", "basis": "Live English and Japanese station and spot autocomplete; ambiguous names remain candidates."},
					{"id": "scheduled_routes", "status": "verified", "implemented": true, "command": "routes search", "source_url": "https://japantravel.navitime.com/en/area/jp/route/result/", "basis": "Dated depart, arrive, first and last modes; station, spot and overnight routes."},
					{"id": "snapshot_detail", "status": "verified", "implemented": true, "command": "routes show", "source_url": "https://japantravel.navitime.com/en/area/jp/route/result/", "basis": "Stored route detail, ordered legs and source fare groups; no additional HTTP request."},
					{"id": "alternative_comparison", "status": "verified", "implemented": true, "command": "routes compare", "source_url": "https://japantravel.navitime.com/en/area/jp/route/result/", "basis": "Local ranking and caps over this response's alternatives; unknown metrics remain unknown."},
					{"id": "pass_catalogue", "status": "advertised", "implemented": true, "command": "passes list", "source_url": "https://japantravel.navitime.com/en/area/jp/route/result/", "basis": "53 source-advertised pass IDs; catalogue count can change. One pass per anonymous query. Representative JR Pass behaviour was live tested; not every pass was tested."},
					{"id": "official_paid_api", "status": "credential_gated", "implemented": false, "command": nil, "source_url": "https://api-sdk.navitime.co.jp/", "basis": "Separate official API service requiring its own access; this CLI uses the public Japan Travel website."},
					{"id": "live_status_and_seats", "status": "unavailable", "implemented": false, "command": nil, "source_url": nil, "basis": "Timetable routes do not establish live operational status or seat availability."},
					{"id": "booking_and_pass_economics", "status": "unavailable", "implemented": false, "command": nil, "source_url": nil, "basis": "No booking, multi-pass query, pass purchase optimizer or inferred pass-holder cost."},
				},
			}
			n, err := printNavitime(cmd, flags, value)
			if flags.metrics {
				emitNavitimeMetrics(cmd.ErrOrStderr(), navitime.Metrics{}, time.Since(started), n)
			}
			return err
		},
	}
	return cmd
}
