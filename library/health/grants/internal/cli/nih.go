package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

func cmdNIH(args []string) int {
	fs := flag.NewFlagSet("nih", flag.ContinueOnError)
	minAmount := fs.Int64("min-amount", 0, "min. award USD")
	year := fs.Int("year", 0, "fiskális év, pl. 2025")
	rows := fs.Int("rows", 15, "találatok száma")
	asJSON := fs.Bool("json", false, "JSON kimenet")
	pos, err := parseFlexible(fs, args)
	if err != nil {
		return 2
	}
	keyword := strings.Join(pos, " ")
	if keyword == "" {
		fmt.Fprintln(os.Stderr, "kell egy kulcsszó / a keyword is required: grants-pp-cli nih <kulcsszó>")
		return 2
	}

	projects, total, err := sources.SearchNIH(keyword, *year, *rows)
	if err != nil {
		return fail(err)
	}
	if *minAmount > 0 {
		var kept []sources.NIHProject
		for _, p := range projects {
			if int64(p.AwardAmount) >= *minAmount {
				kept = append(kept, p)
			}
		}
		projects = kept
	}

	if *asJSON {
		return printJSON(map[string]any{"keyword": keyword, "total": total, "shown": len(projects), "projects": projects})
	}

	fmt.Printf("🏥 NIH RePORTER %q — %d megítélt projekt összesen, %d mutatva (award szerint csökkenő)\n", keyword, total, len(projects))
	for _, p := range projects {
		fmt.Printf("  %-16s %12s  FY%-5d %-28s %s\n",
			p.ProjectNum, FormatMoney(int64(p.AwardAmount)), p.FiscalYear,
			truncate(p.Org.Name, 28), truncate(p.Title, 55))
	}
	if len(projects) == 0 {
		fmt.Println("  (nincs találat / no results)")
	}
	return 0
}
