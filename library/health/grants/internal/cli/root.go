// Package cli — grants-pp-cli command dispatcher.
package cli

import (
	"fmt"
	"os"
)

var version = "2026.8.2"

const usage = `grants-pp-cli %s — open research grants, keyless

USAGE:
  grants-pp-cli search <keyword>    open opportunities (Grants.gov: NIH, NSF, all federal)
      --closing-before YYYY-MM-DD   only those open until this deadline
      --agency CODE                 agency filter (e.g. HHS-NIH11, NSF)
      --rows N                      number of results (default 15)
      --details                     fetch award ceiling + eligibility per row
      --min-award N                 min. award amount USD (implies --details)
      --eligibility TEXT            eligibility filter, e.g. "small business" (implies --details)
      --json                        raw JSON output

  grants-pp-cli nih <keyword>       awarded NIH grants (RePORTER) — "how much do they give for this"
      --min-amount N  --year YYYY  --rows N  --json

  grants-pp-cli nsf <keyword>       awarded NSF grants
      --min-amount N  --rows N (max 25)  --json

  grants-pp-cli doctor              check reachability of all three APIs
  grants-pp-cli version | help
`

// Run dispatches a subcommand; returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
	switch args[0] {
	case "search":
		return cmdSearch(args[1:])
	case "nih":
		return cmdNIH(args[1:])
	case "nsf":
		return cmdNSF(args[1:])
	case "doctor":
		return cmdDoctor()
	case "version", "--version", "-v":
		fmt.Println("grants-pp-cli", version)
		return 0
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", args[0])
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}
