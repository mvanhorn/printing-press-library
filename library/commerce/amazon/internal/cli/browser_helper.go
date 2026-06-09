package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// browserHelperPath returns the path to amazon-checkout.mjs. Searched in this
// order, taking the first that exists:
//  1. $AMAZON_PP_BROWSER_HELPER (explicit override; useful for tests)
//  2. /usr/local/lib/amazon-pp-cli/amazon-checkout.mjs (container install path)
//  3. ../checkout-helper/amazon-checkout.mjs relative to this source dir (dev)
func browserHelperPath() (string, error) {
	candidates := []string{
		os.Getenv("AMAZON_PP_BROWSER_HELPER"),
		"/usr/local/lib/amazon-pp-cli/amazon-checkout.mjs",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("amazon-checkout.mjs not found (set AMAZON_PP_BROWSER_HELPER or install to /usr/local/lib/amazon-pp-cli/)")
}

// BrowserResult is the JSON contract emitted by amazon-checkout.mjs on stdout.
type BrowserResult struct {
	Status             string         `json:"status"` // ok | review_ready | placed | placed_unconfirmed | manual_required | added | add_failed
	Kind               string         `json:"kind,omitempty"`
	Deeplink           string         `json:"deeplink,omitempty"`
	Stage              string         `json:"stage,omitempty"`
	OrderID            string         `json:"order_id,omitempty"`
	ConfirmationURL    string         `json:"confirmation_url,omitempty"`
	ReviewURL          string         `json:"review_url,omitempty"`
	Items              []BrowserItem  `json:"items,omitempty"`
	Subtotal           string         `json:"subtotal,omitempty"`
	DefaultAddress     string         `json:"default_address,omitempty"`
	DefaultCardLast4   string         `json:"default_card_last4,omitempty"`
	// add-to-cart fields
	ASIN              string `json:"asin,omitempty"`
	Title             string `json:"title,omitempty"`
	Quantity          int    `json:"quantity,omitempty"`
	CartItems         int    `json:"cart_items,omitempty"`
	WasAlreadyInCart  bool   `json:"was_already_in_cart,omitempty"`
	Reason            string `json:"reason,omitempty"`
	// history-sync fields
	OrdersCount       int    `json:"orders_count,omitempty"`
	JSONL             string `json:"jsonl,omitempty"`
}

type BrowserItem struct {
	ASIN     string `json:"asin,omitempty"`
	Title    string `json:"title,omitempty"`
	Quantity int    `json:"quantity"`
	Price    string `json:"price,omitempty"`
}

// runBrowserHelper invokes amazon-checkout.mjs for cart-show / checkout.
func runBrowserHelper(ctx context.Context, action, cookiesPath string, placeOrder bool) (*BrowserResult, error) {
	extra := []string{}
	if placeOrder {
		extra = append(extra, "--place-order")
	}
	return runBrowserHelperRaw(ctx, action, cookiesPath, extra)
}

// runBrowserHelperAdd invokes amazon-checkout.mjs add-to-cart with the given
// ASIN + quantity.
func runBrowserHelperAdd(ctx context.Context, cookiesPath, asin string, quantity int) (*BrowserResult, error) {
	args := []string{asin}
	if quantity > 1 {
		args = append(args, "--quantity", fmt.Sprintf("%d", quantity))
	}
	return runBrowserHelperRaw(ctx, "add-to-cart", cookiesPath, args)
}

func runBrowserHelperRaw(ctx context.Context, action, cookiesPath string, extraArgs []string) (*BrowserResult, error) {
	helper, err := browserHelperPath()
	if err != nil {
		return nil, coded(ExitTransient, "%v", err)
	}
	args := append([]string{helper, action, cookiesPath}, extraArgs...)
	cmd := exec.CommandContext(ctx, "node", args...)
	cmd.Env = append(os.Environ(),
		"PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH="+os.Getenv("PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH"),
	)
	// Capture stdout and stderr separately. stdout carries the JSON contract,
	// stderr carries diagnostic messages from transientExit() and selector
	// failures. Earlier we used cmd.Output() which discards stderr, so a
	// stale-selector failure surfaced as opaque "helper exit 7" with no clue.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := stdout.Bytes()
	errOut := strings.TrimSpace(stderr.String())
	exitCode := 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return nil, coded(ExitTransient, "node helper: %v\nstderr: %s", runErr, errOut)
		}
	}
	var result BrowserResult
	if len(out) > 0 {
		if jerr := json.Unmarshal(out, &result); jerr != nil {
			return nil, coded(ExitTransient, "helper returned non-JSON: %v\nraw: %s\nstderr: %s", jerr, string(out), errOut)
		}
	}
	if exitCode == ExitManual {
		return &result, coded(ExitManual, "manual gate (%s); finish at %s", result.Kind, result.Deeplink)
	}
	if exitCode != 0 {
		// Include stderr so the agent (and the user) can see WHY the helper
		// exited — stale selectors, unexpected page, timeouts, etc.
		return &result, coded(ExitTransient, "helper exit %d: %s", exitCode, errOut)
	}
	return &result, nil
}

// formatBrowserCart renders a BrowserResult as text (for non-JSON callers).
func formatBrowserCart(r *BrowserResult) string {
	if r == nil {
		return ""
	}
	s := ""
	for _, it := range r.Items {
		s += fmt.Sprintf("  %dx %s\n", it.Quantity, truncate(it.Title, 70))
	}
	if r.Subtotal != "" {
		s += "Subtotal: " + r.Subtotal + "\n"
	}
	if r.DefaultAddress != "" {
		s += "Ships to: " + truncate(r.DefaultAddress, 70) + "\n"
	}
	if r.DefaultCardLast4 != "" {
		s += "Card: ····" + r.DefaultCardLast4 + "\n"
	}
	return s
}
