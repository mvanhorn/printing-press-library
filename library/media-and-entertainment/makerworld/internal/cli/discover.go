// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: rank the locally synced design mirror by a
// composite quality signal, with optional live printer-fit enrichment.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/store"

	"github.com/spf13/cobra"
)

// pp:data-source local

type discoverItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Creator     string   `json:"creator"`
	DesignScore float64  `json:"design_score"`
	HotScore    float64  `json:"hot_score"`
	Likes       int      `json:"likes"`
	Collections int      `json:"collections"`
	Prints      int      `json:"prints"`
	Downloads   int      `json:"downloads"`
	SaveRate    float64  `json:"save_rate"`
	StaffPicked bool     `json:"staff_picked"`
	Printable   bool     `json:"printable"`
	URL         string   `json:"url"`
	NeedAms     *bool    `json:"need_ams,omitempty"`
	WeightG     *float64 `json:"weight_g,omitempty"`
}

func saveRate(collection, download int) float64 {
	if download <= 0 {
		return 0
	}
	return float64(collection) / float64(download)
}

func toDiscoverItem(r designRow) discoverItem {
	return discoverItem{
		ID:          r.ID,
		Title:       r.Title,
		Creator:     r.CreatorName,
		DesignScore: r.DesignScore,
		HotScore:    r.HotScore,
		Likes:       r.Like,
		Collections: r.Collection,
		Prints:      r.Print,
		Downloads:   r.Download,
		SaveRate:    saveRate(r.Collection, r.Download),
		StaffPicked: r.StaffPicked,
		Printable:   r.Printable,
		URL:         "https://makerworld.com/en/models/" + r.ID,
	}
}

// lessBySort reports whether a should sort before b for the given mode.
func lessBySort(a, b designRow, mode string) bool {
	switch mode {
	case "popular":
		return a.Download > b.Download
	case "hot":
		return a.HotScore > b.HotScore
	case "saves":
		return saveRate(a.Collection, a.Download) > saveRate(b.Collection, b.Download)
	default: // quality and staff-picks both rank by MakerWorld's designScore
		if a.DesignScore != b.DesignScore {
			return a.DesignScore > b.DesignScore
		}
		return a.Collection > b.Collection
	}
}

// instanceFit holds the per-design printer-fit signals read from design detail.
type instanceFit struct {
	needAms bool
	weightG float64
}

// fitEnricher fetches per-design instance data (AMS/weight) live, bounded by cap.
type fitEnricher struct {
	client *client.Client
	cap    int
	used   int
}

func (e *fitEnricher) fetch(ctx context.Context, id string) (instanceFit, bool) {
	e.used++
	data, err := e.client.Get(ctx, "/design-service/design/"+id, nil)
	if err != nil {
		return instanceFit{}, false
	}
	var d struct {
		Instances []struct {
			IsDefault bool    `json:"isDefault"`
			NeedAms   bool    `json:"needAms"`
			Weight    float64 `json:"weight"`
		} `json:"instances"`
	}
	if json.Unmarshal(data, &d) != nil || len(d.Instances) == 0 {
		return instanceFit{}, false
	}
	inst := d.Instances[0]
	for _, in := range d.Instances {
		if in.IsDefault {
			inst = in
			break
		}
	}
	return instanceFit{needAms: inst.NeedAms, weightG: inst.Weight}, true
}

func newNovelDiscoverCmd(flags *rootFlags) *cobra.Command {
	var flagSort string
	var flagNoAms bool
	var flagPrintable bool
	var flagMaxWeight int
	var flagMinDownloads int
	var flagIncludeNSFW bool
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "discover [keyword]",
		Short: "Find models that are both highly rated and printable on your setup",
		Long: "Rank your locally synced MakerWorld designs by a composite quality signal " +
			"(MakerWorld's own designScore plus engagement), filter by printability and " +
			"popularity, and optionally constrain to your printer setup with --no-ams and " +
			"--max-weight (which enrich the top candidates live). Run 'sync' first.\n\n" +
			"Use this to surface models that are good, not just popular. For what is newly " +
			"rising between syncs use 'movers'; for live keyword search use 'designs search'.",
		Example: strings.Trim(`
  makerworld-pp-cli discover --sort quality --limit 20 --agent
  makerworld-pp-cli discover dragon --printable --no-ams --max-weight 60 --agent
  makerworld-pp-cli discover --sort staff-picks --min-downloads 500`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank locally synced designs by quality")
				return nil
			}
			sortMode := strings.ToLower(strings.TrimSpace(flagSort))
			if sortMode == "" {
				sortMode = "quality"
			}
			switch sortMode {
			case "quality", "popular", "hot", "saves", "staff-picks":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort must be one of: quality, popular, hot, saves, staff-picks"))
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("discover ranks your local mirror and has no live-only mode; run 'sync' then 'discover'"))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be >= 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("makerworld-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: makerworld-pp-cli sync --resources designs --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "designs") {
				hintIfStale(cmd, db, "designs", flags.maxAge)
			}

			rows, err := loadDesignRows(ctx, db.DB())
			if err != nil {
				return err
			}
			scanned := len(rows)

			keyword := ""
			if len(args) > 0 {
				keyword = strings.ToLower(strings.TrimSpace(args[0]))
			}

			filtered := make([]designRow, 0, len(rows))
			for _, r := range rows {
				if !flagIncludeNSFW && r.NSFW {
					continue
				}
				if flagPrintable && !r.Printable {
					continue
				}
				if r.Download < flagMinDownloads {
					continue
				}
				if sortMode == "staff-picks" && !r.StaffPicked {
					continue
				}
				if keyword != "" && !strings.Contains(strings.ToLower(r.Title), keyword) {
					continue
				}
				filtered = append(filtered, r)
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				return lessBySort(filtered[i], filtered[j], sortMode)
			})

			needEnrich := flagNoAms || flagMaxWeight > 0
			var enricher *fitEnricher
			if needEnrich {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				enrichCap := flagLimit * 4
				if enrichCap < 20 {
					enrichCap = 20
				}
				if cliutil.IsDogfoodEnv() && enrichCap > 3 {
					enrichCap = 3
				}
				enricher = &fitEnricher{client: c, cap: enrichCap}
			}

			result := make([]discoverItem, 0)
			enrichFailures := 0
			enrichCapHit := false
			for _, r := range filtered {
				if flagLimit > 0 && len(result) >= flagLimit {
					break
				}
				item := toDiscoverItem(r)
				if needEnrich {
					if enricher.used >= enricher.cap {
						enrichCapHit = true
						break
					}
					fit, ok := enricher.fetch(ctx, r.ID)
					if !ok {
						enrichFailures++
						continue
					}
					if flagNoAms && fit.needAms {
						continue
					}
					if flagMaxWeight > 0 && fit.weightG > float64(flagMaxWeight) {
						continue
					}
					needAms := fit.needAms
					weight := fit.weightG
					item.NeedAms = &needAms
					item.WeightG = &weight
				}
				result = append(result, item)
			}

			if needEnrich && enrichFailures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d candidate(s) excluded — could not fetch printer-fit detail (network or parse error)\n", enrichFailures)
			}
			if enrichCapHit && (flagLimit == 0 || len(result) < flagLimit) {
				if flagLimit == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: printer-fit enrichment is bounded to %d live lookups (live API calls can't be unlimited); pass a specific --limit (e.g. --limit 50) to raise the budget, or use fewer filters\n", enricher.cap)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: printer-fit enrichment budget (%d) exhausted before filling the result; re-run with a higher --limit or fewer filters\n", enricher.cap)
				}
			}
			if len(result) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no designs matched in a local mirror of %d designs; sync more (--max-pages) or relax filters\n", scanned)
			}
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			items := make([]map[string]any, 0, len(result))
			for _, it := range result {
				items = append(items, map[string]any{
					"id": it.ID, "title": it.Title, "creator": it.Creator,
					"design_score": it.DesignScore, "downloads": it.Downloads, "likes": it.Likes,
				})
			}
			if len(items) > 0 {
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSort, "sort", "quality", "Ranking: quality, popular, hot, saves, staff-picks")
	cmd.Flags().BoolVar(&flagPrintable, "printable", false, "Only models flagged printable")
	cmd.Flags().BoolVar(&flagNoAms, "no-ams", false, "Only models whose default plate prints without AMS (enriches live)")
	cmd.Flags().IntVar(&flagMaxWeight, "max-weight", 0, "Only models whose default plate is at most N grams (enriches live; 0 disables)")
	cmd.Flags().IntVar(&flagMinDownloads, "min-downloads", 0, "Minimum download count (popularity floor)")
	cmd.Flags().BoolVar(&flagIncludeNSFW, "include-nsfw", false, "Include NSFW-flagged models")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum models to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
