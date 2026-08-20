package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// titleRegex captures the contents of the first <title> element.
var titleRegex = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// authInterstitialTitleMarkers are decisive <title> substrings (lowercased)
// that an authenticated order page never carries. Amazon titles its
// order-history page "Your Orders"; the sign-in, claim, robot-check and
// identity-challenge pages title themselves differently.
var authInterstitialTitleMarkers = []string{
	"sign-in",
	"sign in",
	"signin",
	"iniciar sesión",
	"iniciar sesion",
	"robot check",
	"authentication required",
	"captcha",
}

var orderHistoryTitleMarkers = []string{
	"your orders",
	"mis pedidos",
	"tus pedidos",
	"meine bestellungen",
	"ihre bestellungen",
	"vos commandes",
	"mes commandes",
	"i miei ordini",
	"meus pedidos",
	"seus pedidos",
	"注文履歴",
	"我的订单",
	"我的訂單",
}

// authInterstitialPageError describes an Amazon sign-in, account-claim, CAPTCHA,
// or identity-verification page served in place of authenticated buyer HTML.
type authInterstitialPageError struct {
	Reason string
}

func (e *authInterstitialPageError) Error() string {
	return fmt.Sprintf("amazon returned a sign-in/interstitial page (%s); the browser session is not authenticated — run 'amazon-orders-pp-cli auth login --chrome' (or 'auth refresh') and retry", e.Reason)
}

func IsAuthInterstitialError(err error) bool {
	var target *authInterstitialPageError
	return errors.As(err, &target)
}

// DetectAuthInterstitial reports whether htmlBytes is an Amazon sign-in,
// account-claim (/ax/claim), CAPTCHA, or identity-verification challenge page
// served in place of the requested content. When true, reason is a short
// human-readable description of which signal matched.
//
// Amazon publishes no buyer-side API; an expired or never-authenticated cookie
// jar is answered with HTTP 200 and one of these interstitials rather than a
// 4xx, so status-code checks alone cannot tell a logged-in session from a
// logged-out one. Content detection is the only reliable signal available
// without a final-redirect URL (the HTTP client does not surface one here).
func DetectAuthInterstitial(htmlBytes []byte) (bool, string) {
	if len(htmlBytes) == 0 {
		return false, ""
	}
	body := string(htmlBytes)
	lower := strings.ToLower(body)

	// Title-based detection is the most reliable signal.
	if m := titleRegex.FindStringSubmatch(body); len(m) > 1 {
		title := strings.ToLower(strings.TrimSpace(m[1]))
		for _, marker := range authInterstitialTitleMarkers {
			if strings.Contains(title, marker) {
				return true, "page title is " + strconv.Quote(strings.TrimSpace(m[1]))
			}
		}
	}

	// A full sign-in form: a password input alongside Amazon's auth-form
	// markers. We require the password field so a stray "/ap/signin" href in a
	// header nav can't trip detection on an authenticated page.
	if strings.Contains(lower, `name="password"`) &&
		(strings.Contains(lower, "ap_password") ||
			strings.Contains(lower, "signinsubmit") ||
			strings.Contains(lower, "/ap/signin")) {
		return true, "response contains an Amazon sign-in form"
	}

	// The /ax/claim re-authentication interstitial.
	if strings.Contains(lower, "/ax/claim") {
		return true, "response is an Amazon account-claim interstitial (/ax/claim)"
	}

	// CAPTCHA / automated-traffic challenge.
	if strings.Contains(lower, "captchacharacters") || strings.Contains(lower, "/errors/validatecaptcha") {
		return true, "response is an Amazon CAPTCHA challenge"
	}

	// CVF / additional identity-verification challenge.
	if strings.Contains(lower, "cvf-widget") || strings.Contains(lower, "auth-error-message-box") {
		return true, "response is an Amazon identity-verification challenge"
	}

	return false, ""
}

// DetectOrderHistoryPage reports whether htmlBytes looks like Amazon's
// authenticated order-history surface, not just any non-login HTTP 200 HTML.
func DetectOrderHistoryPage(htmlBytes []byte) bool {
	if len(htmlBytes) == 0 {
		return false
	}
	body := string(htmlBytes)
	lower := strings.ToLower(body)

	if m := titleRegex.FindStringSubmatch(body); len(m) > 1 {
		title := strings.ToLower(strings.TrimSpace(m[1]))
		for _, marker := range orderHistoryTitleMarkers {
			if strings.Contains(title, marker) {
				return true
			}
		}
	}

	return strings.Contains(lower, "order-card") ||
		strings.Contains(lower, "js-order-card") ||
		strings.Contains(lower, "/your-orders/order-details") ||
		strings.Contains(lower, "order #") ||
		strings.Contains(lower, "pedido #") ||
		strings.Contains(lower, "pedido realizado")
}

// AuthInterstitialError returns a non-nil error describing the interstitial
// when htmlBytes is an Amazon sign-in/claim/challenge page, and nil otherwise.
// Callers fetching live HTML use it to fail loudly with a re-auth hint instead
// of silently parsing zero orders out of a sign-in page.
func AuthInterstitialError(htmlBytes []byte) error {
	if ok, reason := DetectAuthInterstitial(htmlBytes); ok {
		return &authInterstitialPageError{Reason: reason}
	}
	return nil
}
