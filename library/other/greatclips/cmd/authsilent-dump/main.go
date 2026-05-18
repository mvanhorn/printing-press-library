package main

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/greatclips/internal/auth0silent"
	"os"
)

func main() {
	cookies, err := auth0silent.ExtractAuth0Cookies()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tok, err := auth0silent.Mint(os.Args[1], cookies)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(tok.AccessToken)
}
