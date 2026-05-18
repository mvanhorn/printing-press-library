// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

// authsilent-smoke is a tiny dev-only binary that proves the
// auth0silent pipeline end-to-end on the local machine.
//
// Usage:
//
//	go run ./cmd/authsilent-smoke -audience https://webservices.greatclips.com/customer
//
// Prints metadata about the minted token (audience, exp, length) but
// never the token value itself.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/greatclips/internal/auth0silent"
)

func main() {
	aud := flag.String("audience", "", "Auth0 audience to mint a token for")
	test := flag.String("test-url", "", "Optional: GET this URL with the minted Bearer to verify the JWT works (HEAD-style; prints status only)")
	flag.Parse()

	if *aud == "" {
		fmt.Fprintln(os.Stderr, "error: -audience is required")
		os.Exit(2)
	}

	fmt.Fprintln(os.Stderr, "[1/3] Extracting Chrome cookies for cid.greatclips.com...")
	cookies, err := auth0silent.ExtractAuth0Cookies()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cookie extract failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "    extracted %d cookies: %s\n", len(cookies), strings.Join(keys(cookies), ", "))

	fmt.Fprintf(os.Stderr, "[2/3] Minting access token for audience=%q...\n", *aud)
	tok, err := auth0silent.Mint(*aud, cookies)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mint failed: %v\n", err)
		os.Exit(1)
	}
	ttl := time.Until(tok.ExpiresAt).Round(time.Second)
	fmt.Fprintf(os.Stderr, "    minted: audience=%s, expires_in=%s, token_len=%dB\n", tok.Audience, ttl, len(tok.AccessToken))

	if *test != "" {
		fmt.Fprintf(os.Stderr, "[3/3] Probing %s with the minted Bearer...\n", *test)
		req, _ := http.NewRequest("GET", *test, nil)
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Origin", auth0silent.SPAOrigin)
		req.Header.Set("Referer", auth0silent.SPAOrigin+"/")
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    GET failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		fmt.Fprintf(os.Stderr, "    HTTP %d\n", resp.StatusCode)
	} else {
		fmt.Fprintln(os.Stderr, "[3/3] -test-url not provided; skipping live probe")
	}
	fmt.Fprintln(os.Stderr, "OK")
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
