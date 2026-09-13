// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newShowCmd(flags))
		addNovelCommandIfAbsent(root, newPhotosCmd(flags))
	})
}

func newShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var noStore bool
	cmd := &cobra.Command{
		Use:   "show [id-or-url]",
		Short: "Fetch one listing by ID or immoweb.be URL as a readable card: price, surfaces, EPC, cadastral income, agency contact, link",
		Long: `Fetch one Immoweb listing (by ID or any immoweb.be listing URL) and print the key facts.
The detail is recorded locally so price history, days on market and deal verdicts improve over time.
For the full raw JSON use 'immoweb-pp-cli listings get <id>'.`,
		Example: strings.Trim(`
  immoweb-pp-cli show 21828249
  immoweb-pp-cli show https://www.immoweb.be/fr/annonce/maison/a-vendre/dilbeek/1700/21828249 --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true", // reads Immoweb only; results land in the CLI's own cache
			"pp:data-source": "live",
			"pp:happy-args":  "id=21828249",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch one Immoweb listing")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a listing ID or URL is required"))
			}
			id, err := immo.ParseListingID(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			d, raw, err := fetchDetail(ctx, c, id)
			if err != nil {
				return err
			}
			type showView struct {
				immo.Detail
				DaysListed *int           `json:"days_listed,omitempty"`
				History    []priceObsView `json:"price_history,omitempty"`
			}
			view := showView{Detail: d}
			if dl, ok := immo.DaysListed(d.CreatedAt, time.Now()); ok {
				view.DaysListed = &dl
			}
			if !noStore {
				db, err := openImmoStore(ctx, dbPath)
				if err != nil {
					return err
				}
				defer db.Close()
				if err := db.SaveDetail(ctx, d, raw, time.Now()); err != nil {
					return err
				}
				hist, err := db.PriceHistory(ctx, id)
				if err != nil {
					return err
				}
				for _, h := range hist {
					view.History = append(view.History, priceObsView{ObservedAt: h.ObservedAt, Price: h.Price})
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", bold(termSafe(firstNonEmptyStr(d.Title, fmt.Sprintf("%s %s", strings.ToLower(d.Type), strings.ToLower(d.Deal))))))
			fmt.Fprintf(w, "%s\n\n", d.URL)
			line := func(k, v string) {
				if strings.TrimSpace(v) != "" {
					fmt.Fprintf(w, "  %-18s %s\n", k, termSafe(v))
				}
			}
			price := ""
			if d.Price != nil {
				price = fmtEUR(*d.Price)
				if d.Deal == "FOR_RENT" {
					price += " / month"
					if d.RentCosts != nil {
						price += fmt.Sprintf(" (+ %s charges)", fmtEUR(*d.RentCosts))
					}
				}
			}
			line("Price", price)
			if d.PricePerSqm != nil {
				line("Price per m²", fmtEUR(*d.PricePerSqm))
			}
			typ := strings.ToLower(d.Type)
			if d.Subtype != "" && !strings.EqualFold(d.Subtype, d.Type) {
				typ += " (" + strings.ToLower(strings.ReplaceAll(d.Subtype, "_", " ")) + ")"
			}
			line("Type", typ)
			line("Where", strings.TrimSpace(strings.Join(nonEmptyStrs(d.Street, locLabel(d.Listing), d.Province), ", ")))
			line("Bedrooms", intStr(d.Bedrooms))
			line("Bathrooms", intStr(d.Bathrooms))
			line("Living surface", floatUnit(d.Surface, " m²"))
			line("Land", floatUnit(d.Land, " m²"))
			line("Garden", floatUnit(d.Garden, " m²"))
			line("Terrace", floatUnit(d.Terrace, " m²"))
			line("Built", intStr(d.ConstructionYear))
			line("Condition", strings.ToLower(strings.ReplaceAll(d.Condition, "_", " ")))
			line("Facades", intStr(d.Facades))
			epc := d.EPC
			if d.EPCKwhPerSqm != nil {
				epc += fmt.Sprintf(" (%.0f kWh/m²/year)", *d.EPCKwhPerSqm)
			}
			if d.RenovationOblig != nil && *d.RenovationOblig {
				epc += " · renovation obligation"
			}
			line("EPC / PEB", epc)
			line("Heating", strings.ToLower(d.Heating))
			line("Cadastral income", floatUnit(d.CadastralIncome, " €"))
			if view.DaysListed != nil {
				line("Online for", fmt.Sprintf("%d days (since %s)", *view.DaysListed, d.CreatedAt[:10]))
			}
			if d.Views != nil || d.Bookmarks != nil {
				line("Demand", fmt.Sprintf("%s views · %s saves", intStr(d.Views), intStr(d.Bookmarks)))
			}
			status := []string{}
			if d.UnderOption {
				status = append(status, "under option")
			}
			if d.NewPrice {
				status = append(status, "price reduced")
			}
			if d.Sold {
				status = append(status, "sold/rented")
			}
			line("Status", strings.Join(status, ", "))
			seller := d.Agency
			if d.Private {
				seller = "private seller"
			}
			line("Seller", seller)
			line("Phone", d.AgencyPhone)
			line("Email", d.AgencyEmail)
			if len(view.History) > 1 {
				parts := []string{}
				for _, h := range view.History {
					parts = append(parts, fmt.Sprintf("%s %s", h.ObservedAt[:10], fmtEUR(h.Price)))
				}
				line("Price history", strings.Join(parts, " → "))
			}
			line("Photos", fmt.Sprintf("%d (download with: immoweb-pp-cli photos %d)", len(d.Pictures), d.ID))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not record the listing locally")
	return cmd
}

type priceObsView struct {
	ObservedAt string  `json:"observed_at"`
	Price      float64 `json:"price"`
}

func newPhotosCmd(flags *rootFlags) *cobra.Command {
	var dir, size string
	var listOnly bool
	cmd := &cobra.Command{
		Use:   "photos [id-or-url]",
		Short: "Download a listing's photos (or list their URLs with --list)",
		Example: strings.Trim(`
  immoweb-pp-cli photos 21828249 --dir ./photos/dilbeek
  immoweb-pp-cli photos 21828249 --list --size xl --json`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"pp:happy-args":  "id=21828249;--list",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "download listing photos")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a listing ID or URL is required"))
			}
			id, err := immo.ParseListingID(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, raw, err := fetchDetail(ctx, c, id)
			if err != nil {
				return err
			}
			urls, err := immo.PictureURLs(raw, size)
			if err != nil {
				return usageErr(err)
			}
			type photoOut struct {
				ID    int64    `json:"id"`
				Count int      `json:"count"`
				URLs  []string `json:"urls"`
				Files []string `json:"files,omitempty"`
			}
			out := photoOut{ID: id, Count: len(urls), URLs: urls}
			if listOnly || cliutil.IsAnyHarness() {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if dir == "" {
				dir = fmt.Sprintf("immoweb-%d", id)
			}
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			hc := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 || !immowebStaticURL(req.URL) {
					return fmt.Errorf("refusing redirect to %s", req.URL.Host)
				}
				return nil
			}}
			for i, u := range urls {
				pu, err := url.Parse(u)
				if err != nil || !immowebStaticURL(pu) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: photo %d: skipped non-Immoweb URL\n", i+1)
					continue
				}
				ext := strings.ToLower(filepath.Ext(pu.Path))
				if ext == "" || len(ext) > 5 {
					ext = ".jpg"
				}
				dest := filepath.Join(dir, fmt.Sprintf("%02d%s", i+1, ext))
				if err := downloadFile(ctx, hc, u, dest); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: photo %d: %v\n", i+1, err)
					continue
				}
				out.Files = append(out.Files, dest)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %d of %d photos to %s\n", len(out.Files), len(urls), dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Destination directory (default: ./immoweb-<id>)")
	cmd.Flags().StringVar(&size, "size", "large", "Photo size: small, medium, large or xl (2560px)")
	cmd.Flags().BoolVar(&listOnly, "list", false, "Only list photo URLs, do not download")
	return cmd
}

func downloadFile(ctx context.Context, hc *http.Client, rawURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// dest is <--dir>/<NN><ext>: the user picks the directory, the name is built here.
	f, err := os.Create(filepath.Clean(dest))
	if err != nil {
		return err
	}
	// Photos are a few MB at most; cap the body so a bad response cannot fill the disk.
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 50<<20)); err != nil {
		_ = f.Close()
		_ = os.Remove(dest)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(dest)
		return err
	}
	return nil
}

func intStr(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func floatUnit(p *float64, unit string) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%.0f%s", *p, unit)
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func nonEmptyStrs(vals ...string) []string {
	out := []string{}
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}

// immowebStaticURL accepts only https URLs on immowebstatic.be or its
// subdomains (Immoweb's picture CDN).
func immowebStaticURL(u *url.URL) bool {
	h := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (h == "immowebstatic.be" || strings.HasSuffix(h, ".immowebstatic.be"))
}
