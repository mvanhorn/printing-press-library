package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

func cmdNSF(args []string) int {
	fs := flag.NewFlagSet("nsf", flag.ContinueOnError)
	minAmount := fs.Int64("min-amount", 0, "min. összeg USD")
	rows := fs.Int("rows", 15, "találatok száma (max 25)")
	asJSON := fs.Bool("json", false, "JSON kimenet")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	keyword := strings.Join(pos, " ")
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "kell egy kulcsszó / a keyword is required: grants-pp-cli nsf <kulcsszó>")
		return 2
	}

	awards, err := sources.SearchNSF(keyword, *rows)
	if err != nil {
		return fail(err)
	}
	fetched := len(awards)
	if *minAmount > 0 {
		var kept []sources.NSFAward
		for _, a := range awards {
			if sources.ParseMoney(a.FundsObligated) >= *minAmount {
				kept = append(kept, a)
			}
		}
		awards = kept
		// The NSF API returns an unordered page, so a client-side amount filter
		// only ever sees the first `rows` records. A full page means there are
		// almost certainly matching awards we never fetched.
		if fetched == *rows {
			fmt.Fprintf(os.Stderr,
				"  (figyelem / warn: --min-amount csak az első %d, rendezetlen NSF találatra vonatkozik; növeld a --rows értékét / filter applied to the first %d unsorted results only)\n",
				*rows, *rows)
		}
	}

	if *asJSON {
		return printJSON(map[string]any{"keyword": keyword, "shown": len(awards), "awards": awards})
	}

	fmt.Printf("🔬 NSF %q — %d megítélt grant mutatva\n", keyword, len(awards))
	for _, a := range awards {
		fmt.Printf("  %-9s %12s  %-10s→%-10s %-26s %s\n",
			a.ID, FormatMoney(sources.ParseMoney(a.FundsObligated)),
			a.StartDate, a.ExpDate, truncate(a.Awardee, 26), truncate(a.Title, 48))
	}
	if len(awards) == 0 {
		fmt.Println("  (nincs találat / no results)")
	}
	return 0
}
