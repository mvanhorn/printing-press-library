// Scoped source-file inputs and honest confined fixture metadata for the full live matrix.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const travelokaLiveFixtureHome = ".printing-press-fixtures/live-home"

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		fixtureHome, err := filepath.Abs(travelokaLiveFixtureHome)
		if err != nil {
			fixtureHome = travelokaLiveFixtureHome
		}
		for _, group := range []string{"flights", "hotels", "quotes"} {
			if c, _, err := root.Find([]string{group}); err == nil && c != root {
				c.Annotations["pp:parent-group"] = "true"
			}
		}
		for _, path := range [][]string{{"doctor"}, {"resolve"}, {"flights", "search"}, {"flights", "date-grid"}, {"hotels", "search"}, {"hotels", "rooms"}, {"hotels", "date-grid"}} {
			c, _, err := root.Find(path)
			if err != nil || c == root {
				continue
			}
			if c.Annotations == nil {
				c.Annotations = map[string]string{}
			}
			// Search endpoints are read-only remotely; these commands also persist public
			// snapshots, locations or doctor proof in the selected local home.
			c.Annotations["mcp:read-only"] = "false"
			c.Annotations["mcp:local-write"] = "true"
			c.Annotations["pp:happy-args"] = strings.Trim(c.Annotations["pp:happy-args"]+";--home="+fixtureHome, ";")
		}
		for _, path := range [][]string{{"flights", "inspect"}, {"flights", "shortlist"}, {"hotels", "flexibility"}, {"quotes", "compare"}, {"quotes", "diff"}} {
			c, _, err := root.Find(path)
			if err != nil || c == root {
				continue
			}
			// Prefer already-retrieved local history in the happy path, while retaining
			// the explicit snapshot examples as a second documented workflow.
			c.Example = "  traveloka-pp-cli " + strings.Join(path, " ") + " --data-source local --agent\n" + c.Example
			c.Annotations["pp:happy-args"] = "--data-source=local;--home=" + fixtureHome
		}
		for _, op := range []struct {
			path []string
			name string
		}{
			{[]string{"airport"}, "airport"}, {[]string{"flight", "initial"}, "flight-initial"},
			{[]string{"flight", "poll"}, "flight-poll"}, {[]string{"flight", "prefetch"}, "flight-prefetch"},
			{[]string{"hotel", "lookup"}, "hotel-lookup"}, {[]string{"hotel", "features"}, "hotel-features"},
			{[]string{"hotel", "catalog"}, "hotel-catalog"}, {[]string{"hotel", "rooms"}, "hotel-rooms"},
		} {
			c, _, err := root.Find(op.path)
			if err != nil || c == root {
				continue
			}
			c.Example = "  traveloka-pp-cli " + strings.Join(op.path, " ") + " --data-file /private/tmp/traveloka-" + op.name + "-data.json --agent"
			c.Annotations["pp:happy-args"] = "--data-file=.printing-press-fixtures/requests/" + op.name + ".json"
			addTravelokaDataFile(c, f)
		}
	})
}

// --data-file is the same public operation object as --data; it does not read
// cookies, headers or request envelopes. Required source input has no default.
func addTravelokaDataFile(c *cobra.Command, f *rootFlags) {
	var file string
	c.Flags().StringVar(&file, "data-file", "", "JSON file containing the explicit public operation data object (alternative to --data)")
	previous := c.RunE
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "POST "+c.Annotations["pp:path"]+" (read-only search)")
		}
		if file != "" {
			if cmd.Flags().Changed("data") || cmd.Flags().Changed("stdin") {
				return usageErr(fmt.Errorf("--data-file cannot be combined with --data or --stdin"))
			}
			raw, err := readTravelokaDataFile(file)
			if err != nil {
				return usageErr(err)
			}
			if err = cmd.Flags().Set("data", string(raw)); err != nil {
				return usageErr(err)
			}
		}
		return previous(cmd, args)
	}
}

func readTravelokaDataFile(path string) ([]byte, error) {
	const max = 2 << 20
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading --data-file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("--data-file must be a regular JSON file")
	}
	file, err := os.Open(path) // #nosec G304 -- Operator-selected public request file; regular-file and 2 MiB bounds are checked before decoding.
	if err != nil {
		return nil, fmt.Errorf("reading --data-file: %w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, fmt.Errorf("reading --data-file: %w", err)
	}
	if len(raw) > max {
		return nil, fmt.Errorf("--data-file exceeds the 2 MiB input bound")
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil || data == nil {
		return nil, fmt.Errorf("--data-file must contain one JSON object")
	}
	return raw, nil
}
