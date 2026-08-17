package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources"
)

// doctor — live reachability check against all three keyless APIs.
func cmdDoctor() int {
	fmt.Println("🩺 grants-pp-cli doctor — élő API-ellenőrzés / live API check")
	failed := false

	if _, total, err := sources.SearchOpportunities("health", "", 1); err != nil {
		failed = true
		fmt.Printf("  ✘ Grants.gov   %v\n", err)
	} else {
		fmt.Printf("  ✔ Grants.gov   OK (%d nyitott 'health' kiírás)\n", total)
	}

	if _, total, err := sources.SearchNIH("cancer", 0, 1); err != nil {
		failed = true
		fmt.Printf("  ✘ NIH RePORTER %v\n", err)
	} else {
		fmt.Printf("  ✔ NIH RePORTER OK (%d 'cancer' projekt)\n", total)
	}

	if awards, err := sources.SearchNSF("science", 1); err != nil {
		failed = true
		fmt.Printf("  ✘ NSF          %v\n", err)
	} else {
		fmt.Printf("  ✔ NSF          OK (%d találat lekérve)\n", len(awards))
	}

	if failed {
		return 1
	}
	fmt.Println("  Minden forrás él. / All sources up.")
	return 0
}
