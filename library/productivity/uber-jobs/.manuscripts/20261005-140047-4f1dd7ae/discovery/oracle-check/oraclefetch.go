// oraclefetch sends exactly one GET and records it. No retries, no redirects, no cookies.
//
//	go run . <url> <out-prefix> <accept> <ledger.tsv>
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: oraclefetch <url> <out-prefix> <accept> <ledger.tsv>")
		os.Exit(2)
	}
	url, out, accept, ledger := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	stamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	status, n := "ERR", 0
	resp, err := client.Do(req)
	if err != nil {
		appendLedger(ledger, stamp, url, "ERR:"+strings.ReplaceAll(err.Error(), "\t", " "), 0)
		fmt.Fprintln(os.Stderr, "transport error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	status, n = fmt.Sprint(resp.StatusCode), len(body)
	appendLedger(ledger, stamp, url, status, n)

	var h strings.Builder
	fmt.Fprintf(&h, "%s %s\nrequested_at: %s\nurl: %s\n", resp.Proto, resp.Status, stamp, url)
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := strings.Join(resp.Header[k], " | ")
		if strings.EqualFold(k, "Set-Cookie") {
			v = "[" + fmt.Sprint(len(resp.Header[k])) + " set-cookie values, names only] " + cookieNames(resp.Header[k])
		}
		fmt.Fprintf(&h, "%s: %s\n", k, v)
	}
	_ = os.WriteFile(out+"_headers.txt", []byte(h.String()), 0o644)
	_ = os.WriteFile(out+"_body", body, 0o644)
	fmt.Printf("status=%s bytes=%d proto=%s ct=%s\n", status, n, resp.Proto, resp.Header.Get("Content-Type"))
}

func cookieNames(vals []string) string {
	names := make([]string, 0, len(vals))
	for _, v := range vals {
		if i := strings.IndexByte(v, '='); i > 0 {
			names = append(names, v[:i])
		}
	}
	return strings.Join(names, ",")
}

func appendLedger(path, stamp, url, status string, n int) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ledger:", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\tgo-stdlib-oraclefetch\tGET\t%s\t%s\t%d\n", stamp, url, status, n)
}
