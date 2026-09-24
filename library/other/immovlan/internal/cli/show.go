// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newShowCmd(flags))
		addNovelCommandIfAbsent(root, newPhotosCmd(flags))
	})
}

type showView struct {
	immovlan.Detail
	DaysListed *int             `json:"days_listed,omitempty"`
	History    []store.PriceObs `json:"price_history,omitempty"`
	FirstSeen  string           `json:"first_seen,omitempty"`
}

func newShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "show <reference-or-url>",
		Short: "Read one Immovlan listing: PEB letter, address, surfaces, year, condition, cadastral income, seller, dates, photos",
		Long: `Fetch a listing page (reference such as vbe69761, or the full URL), parse its
JSON-LD, feature table and seller block, store the detail fields locally and
print a readable card. With --data-source local the stored row is printed
without a network call.`,
		Example: strings.Trim(`
  immovlan-pp-cli show vbe69761
  immovlan-pp-cli show https://immovlan.be/fr/detail/maison/a-vendre/1030/schaerbeek/vbe69761 --agent
  immovlan-pp-cli show vbe69761 --agent --select price,surface_m2,epc,rented,cadastral_income,created_at`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "reference-or-url=vbe69761",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch an Immovlan listing")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("give one listing reference or URL"))
			}
			ref, err := immovlan.ParseReference(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view := showView{}
			if flags.dataSource == "local" {
				flags.agentSource = "local"
				rows, err := db.QueryVlanListings(ctx, store.ListingFilter{IDs: []string{ref}, IncludeGone: true})
				if err != nil {
					return err
				}
				if len(rows) == 0 {
					return notFoundErr(fmt.Errorf("listing %s is not in the local store; run without --data-source local", ref))
				}
				view.Detail = storedToDetail(rows[0])
				view.FirstSeen = rows[0].FirstSeen
			} else {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				d, err := fetchDetail(ctx, c, ref)
				if err != nil {
					return err
				}
				if err := db.SaveVlanDetail(ctx, d, time.Now()); err != nil {
					return err
				}
				view.Detail = d
			}
			if view.CreatedAt != "" {
				if days, ok := immovlan.DaysListed(view.CreatedAt, time.Now()); ok {
					view.DaysListed = &days
				}
			}
			if view.History, err = db.VlanPriceHistory(ctx, ref); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), view, flags)
			}
			printDetailCard(cmd.OutOrStdout(), view)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

func storedToDetail(r store.StoredListing) immovlan.Detail {
	return immovlan.Detail{Listing: r.Listing, Condition: r.Condition, Rented: r.Rented, CadastralIncome: r.CadastralIncome, Year: r.Year,
		Garden: r.Garden, Terrace: r.Terrace, Heating: r.Heating, Software: r.Software, AgencyURL: r.AgencyURL, Photos: r.Photos}
}

func printDetailCard(w io.Writer, v showView) {
	d := v.Detail
	title := d.Title
	if title == "" {
		title = d.Type + " " + d.Deal
	}
	fmt.Fprintf(w, "%s\n%s\n\n", bold(termSafe(title)), termSafe(d.URL))
	line := func(k, val string) {
		if strings.TrimSpace(val) != "" {
			fmt.Fprintf(w, "  %-18s %s\n", k, termSafe(val))
		}
	}
	price := fmtPtrEUR(d.Price)
	if d.Deal == immovlan.DealRent || d.Deal == immovlan.DealColocation {
		price += " / month"
	}
	line("Price", price)
	if d.PricePerSqm != nil {
		line("Price per m²", fmtEUR(*d.PricePerSqm))
	}
	typ := d.Type
	if d.Subtype != "" && !strings.EqualFold(d.Subtype, d.Type) {
		typ += " (" + strings.ToLower(d.Subtype) + ")"
	}
	line("Type", typ+" · "+d.Deal)
	line("Where", strings.TrimSpace(strings.Join(nonEmpty(d.Street, locLabel(d.Listing)), ", ")))
	line("Bedrooms", intStr(d.Bedrooms))
	line("Bathrooms", intStr(d.Bathrooms))
	line("Living surface", floatUnit(d.Surface, " m²"))
	line("Land", floatUnit(d.Land, " m²"))
	line("Garden", floatUnit(d.Garden, " m²"))
	line("Terrace", floatUnit(d.Terrace, " m²"))
	line("Built", intStr(d.Year))
	line("Condition", d.Condition)
	if d.Rented != nil {
		if *d.Rented {
			line("Rented", "yes (tenant in place)")
		} else {
			line("Rented", "no")
		}
	}
	epc := epcLabel(d.Listing)
	line("PEB", epc)
	line("Heating", d.Heating)
	if d.CadastralIncome != nil {
		line("Cadastral income", fmtEUR(*d.CadastralIncome))
	}
	if v.DaysListed != nil {
		line("Online for", fmt.Sprintf("%d days (since %s)", *v.DaysListed, d.CreatedAt[:10]))
	}
	seller := d.Agency
	if d.Private {
		seller = "private seller"
	}
	if d.Software != "" {
		seller += " · feed: " + d.Software
	}
	line("Seller", seller)
	line("Phone", d.Phone)
	if len(v.History) > 1 {
		parts := []string{}
		for _, h := range v.History {
			parts = append(parts, fmt.Sprintf("%s %s", h.ObservedAt[:10], fmtEUR(h.Price)))
		}
		line("Price history", strings.Join(parts, " → "))
	}
	if len(d.Photos) > 0 {
		line("Photos", fmt.Sprintf("%d (immovlan-pp-cli photos %s)", len(d.Photos), strings.ToLower(d.ID)))
	}
	if d.Description != "" {
		fmt.Fprintf(w, "\n%s\n", termSafe(truncate(d.Description, 600)))
	}
}

func nonEmpty(v ...string) []string {
	out := []string{}
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func newPhotosCmd(flags *rootFlags) *cobra.Command {
	var listOnly bool
	var dir string
	cmd := &cobra.Command{
		Use:   "photos <reference-or-url>",
		Short: "List or download a listing's photos",
		Example: strings.Trim(`
  immovlan-pp-cli photos vbe69761 --list --agent
  immovlan-pp-cli photos vbe69761 --dir ./vbe69761`, "\n"),
		// Hidden from the MCP mirror: --dir writes files wherever the server
		// account can; agents read photo URLs from 'show --agent' (pictures).
		Annotations: map[string]string{
			"mcp:hidden":     "true",
			"pp:data-source": "live",
			"pp:happy-args":  "reference-or-url=vbe69761;--list",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list or download listing photos")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("give one listing reference or URL"))
			}
			ref, err := immovlan.ParseReference(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			d, err := fetchDetail(ctx, c, ref)
			if err != nil {
				return err
			}
			type photoOut struct {
				ID       string   `json:"id"`
				Count    int      `json:"count"`
				Pictures []string `json:"pictures"`
				Files    []string `json:"files,omitempty"`
				Skipped  []string `json:"skipped_existing,omitempty"`
			}
			out := photoOut{ID: ref, Count: len(d.Photos), Pictures: d.Photos}
			if listOnly || cliutil.IsAnyHarness() {
				return printView(cmd.OutOrStdout(), out, flags)
			}
			if dir == "" {
				dir = "immovlan-" + strings.ToLower(ref)
			}
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			hc := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 || !immovlanImageURL(req.URL) {
					return fmt.Errorf("refusing redirect to %s", req.URL.Host)
				}
				return nil
			}}
			for i, u := range d.Photos {
				pu, err := url.Parse(u)
				if err != nil || !immovlanImageURL(pu) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: photo %d: skipped non-Immovlan URL\n", i+1)
					continue
				}
				ext := strings.ToLower(filepath.Ext(pu.Path))
				if !photoExt.MatchString(ext) {
					ext = ".jpg"
				}
				dest := filepath.Join(dir, fmt.Sprintf("%02d%s", i+1, ext))
				err = downloadPhoto(ctx, hc, u, dest)
				switch {
				case errors.Is(err, os.ErrExist):
					out.Skipped = append(out.Skipped, dest)
					continue
				case err != nil:
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: photo %d: %v\n", i+1, err)
					continue
				}
				out.Files = append(out.Files, dest)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %d of %d photos to %s", len(out.Files), len(d.Photos), dir)
			if len(out.Skipped) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), " (%d already there, left untouched)", len(out.Skipped))
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().BoolVar(&listOnly, "list", false, "Print photo URLs without downloading")
	cmd.Flags().StringVar(&dir, "dir", "", "Destination directory (default: ./immovlan-<ref>)")
	return cmd
}

func immovlanImageURL(u *url.URL) bool {
	h := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (h == "api-image.immovlan.be" || strings.HasSuffix(h, ".immovlan.be"))
}

func downloadPhoto(ctx context.Context, hc *http.Client, u, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// Never truncate or follow an existing path (a planted symlink included):
	// write to a fresh temp file, then rename into place only if dest is new.
	if _, err := os.Lstat(dest); err == nil {
		return os.ErrExist
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".photo-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 50<<20)); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Link(tmp.Name(), dest); err != nil {
		if errors.Is(err, os.ErrExist) {
			_ = os.Remove(tmp.Name())
			return os.ErrExist
		}
		// Filesystems without hard links (exFAT, SMB): O_EXCL gives the same
		// no-follow / no-truncate guarantee.
		if copyErr := copyExclusive(tmp.Name(), dest); copyErr != nil {
			_ = os.Remove(tmp.Name())
			return copyErr
		}
	}
	// CreateTemp and copyExclusive both create the photo 0600 (owner-only),
	// so both paths leave the same permissions; no chmod needed.
	return os.Remove(tmp.Name())
}

var photoExt = regexp.MustCompile(`^\.[a-z0-9]{2,4}$`)

func copyExclusive(src, dest string) error {
	in, err := os.Open(src) // #nosec G304 -- src is the CreateTemp file this process just wrote next to dest
	if err != nil {
		return err
	}
	defer in.Close()
	// #nosec G304 -- dest is <dir>/<NN><ext>: dir is the user's --dir (or ./immovlan-<ref>) and ext passed photoExt
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dest) // never leave a partial photo that later runs would skip as "existing"
		return err
	}
	return nil
}
