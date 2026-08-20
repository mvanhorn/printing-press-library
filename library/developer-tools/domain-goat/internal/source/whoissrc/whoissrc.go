// Package whoissrc wraps github.com/likexian/whois with parsed-output convenience.
package whoissrc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// Result is a parsed WHOIS response.
type Result struct {
	FQDN        string          `json:"fqdn"`
	Available   bool            `json:"available"`
	Registrar   string          `json:"registrar,omitempty"`
	Status      []string        `json:"status,omitempty"`
	CreatedAt   string          `json:"created_at,omitempty"`
	ExpiresAt   string          `json:"expires_at,omitempty"`
	UpdatedAt   string          `json:"updated_at,omitempty"`
	NameServers []string        `json:"name_servers,omitempty"`
	Raw         string          `json:"raw,omitempty"`
	ParsedJSON  json.RawMessage `json:"-"`
	Source      string          `json:"source"`
}

// PATCH(whois-goroutine-bounded): per-call whois.NewClient().SetTimeout(15s) so the goroutine terminates on TCP deadline rather than the library's 30s default; likexian/whois has no ctx hook, so without this the goroutine could be stranded on an open WHOIS socket for ~30s after the outer ctx returned.
// Lookup performs a WHOIS query.
//
// The likexian/whois client is synchronous and offers no context hook, so
// the actual lookup runs in a goroutine that we cannot cancel directly. We
// set a per-call SetTimeout(15s) on the dialer so the goroutine terminates
// promptly when the TCP read deadline fires — without this, ctx.Done()
// returning would leave the goroutine stranded on an open WHOIS connection
// until the library's 30s default timeout, which under check --parallel 8
// against a slow server is ~8× the desirable connection-hold time.
func Lookup(ctx context.Context, fqdn string) (*Result, error) {
	if fqdn == "" {
		return nil, errors.New("empty fqdn")
	}

	type queryResult struct {
		raw string
		err error
	}
	ch := make(chan queryResult, 1)
	go func() {
		client := whois.NewClient().SetTimeout(15 * time.Second)
		raw, err := client.Whois(fqdn)
		ch <- queryResult{raw: raw, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("whois lookup timeout (%s)", fqdn)
	case res := <-ch:
		if res.err != nil {
			return &Result{FQDN: fqdn, Source: "whois", Raw: res.raw}, res.err
		}
		out := &Result{FQDN: fqdn, Source: "whois", Raw: res.raw}
		// Try to parse — failures are not fatal, we still return raw.
		parsed, perr := whoisparser.Parse(res.raw)
		if perr == nil {
			if parsed.Domain != nil {
				out.Status = parsed.Domain.Status
				out.CreatedAt = parsed.Domain.CreatedDate
				out.ExpiresAt = parsed.Domain.ExpirationDate
				out.UpdatedAt = parsed.Domain.UpdatedDate
				out.NameServers = parsed.Domain.NameServers
			}
			if parsed.Registrar != nil {
				out.Registrar = parsed.Registrar.Name
			}
			if b, err := json.Marshal(parsed); err == nil {
				out.ParsedJSON = b
			}
		} else if isUnavailableErr(perr) {
			// whois-parser raises ErrNotFoundDomain for unregistered names.
			out.Available = true
		}

		// Some registries return "Domain not found" / "NOT FOUND" — flag.
		if !out.Available && looksAvailable(res.raw) {
			out.Available = true
		}
		return out, nil
	}
}

func isUnavailableErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, whoisparser.ErrNotFoundDomain) ||
		strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "domain not found")
}

func looksAvailable(raw string) bool {
	r := strings.ToLower(raw)
	for _, marker := range []string{
		"no match for", "not found", "no entries found", "no data found",
		"domain not found", "status: free", "status: available", "no object found",
	} {
		if strings.Contains(r, marker) {
			return true
		}
	}
	return false
}

// ParsedJSONString returns the parsed JSON as a string for storage.
func (r *Result) ParsedJSONString() string {
	if len(r.ParsedJSON) == 0 {
		return "{}"
	}
	return string(r.ParsedJSON)
}
