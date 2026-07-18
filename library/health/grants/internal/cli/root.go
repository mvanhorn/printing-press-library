// Package cli — grants-pp-cli command dispatcher.
package cli

import (
	"fmt"
	"os"
)

var version = "1.0.0"

const usage = `grants-pp-cli %s — nyitott kutatási pályázatok, kulcs nélkül / open research grants, keyless

HASZNÁLAT / USAGE:
  grants-pp-cli search <kulcsszó>   nyitott kiírások (Grants.gov: NIH, NSF, minden szövetségi)
      --closing-before YYYY-MM-DD   csak eddig a határidőig nyitottak
      --agency KÓD                  ügynökség-szűrő (pl. HHS-NIH11, NSF)
      --rows N                      találatok száma (default 15)
      --details                     keret + jogosultság lekérése soronként
      --min-award N                 min. keretösszeg USD (bekapcsolja a --details-t)
      --eligibility SZÖVEG          jogosultság-szűrő, pl. "small business" (bekapcsolja a --details-t)
      --json                        nyers JSON kimenet

  grants-pp-cli nih <kulcsszó>      megítélt NIH grantek (RePORTER) — "mennyit adnak erre"
      --min-amount N  --year YYYY  --rows N  --json

  grants-pp-cli nsf <kulcsszó>      megítélt NSF grantek
      --min-amount N  --rows N (max 25)  --json

  grants-pp-cli doctor              mindhárom API elérhetőségének ellenőrzése
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
		fmt.Fprintf(os.Stderr, "ismeretlen parancs / unknown command: %q\n\n", args[0])
		fmt.Fprintf(os.Stderr, usage, version)
		return 2
	}
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "hiba / error:", err)
	return 1
}
