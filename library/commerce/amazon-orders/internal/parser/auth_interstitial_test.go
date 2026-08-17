package parser

import (
	"strings"
	"testing"
)

func TestDetectAuthInterstitial(t *testing.T) {
	tests := []struct {
		name   string
		html   string
		want   bool
		reason string // substring expected in reason when want is true
	}{
		{
			name: "amazon sign-in title",
			html: `<html><head><title>Amazon Sign-In</title></head><body>...</body></html>`,
			want: true, reason: "Amazon Sign-In",
		},
		{
			name: "amazon mexico sign-in title",
			html: `<html><head><title dir="ltr">Amazon Iniciar sesión</title></head><body>...</body></html>`,
			want: true, reason: "Amazon Iniciar sesión",
		},
		{
			name: "amazon mexico sign-in title without accent",
			html: `<html><head><title>Amazon Iniciar sesion</title></head><body>...</body></html>`,
			want: true, reason: "Amazon Iniciar sesion",
		},
		{
			name: "ax/claim interstitial",
			html: `<html><head><title>Amazon</title></head><body><form action="/ax/claim?arb=193cca18">...</form></body></html>`,
			want: true, reason: "/ax/claim",
		},
		{
			name: "sign-in form with password field",
			html: `<html><head><title>Amazon</title></head><body><form action="/ap/signin"><input name="email"><input name="password" id="ap_password"></form></body></html>`,
			want: true, reason: "sign-in form",
		},
		{
			name: "robot check / captcha",
			html: `<html><head><title>Robot Check</title></head><body><input id="captchacharacters"></body></html>`,
			want: true, reason: "Robot Check",
		},
		{
			name: "identity verification challenge",
			html: `<html><head><title>Amazon</title></head><body><div id="cvf-widget"></div></body></html>`,
			want: true, reason: "identity-verification",
		},
		{
			name: "authenticated orders page is not an interstitial",
			html: `<html><head><title>Amazon.in - Your Orders</title></head><body>
				<div class="order-card js-order-card">ORDER PLACED May 5, 2026 TOTAL ₹1,234.56 ORDER # 408-1234567-1234567</div>
				<a href="/ap/signin">Sign Out</a>
			</body></html>`,
			want: false,
		},
		{
			name: "empty body",
			html: ``,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := DetectAuthInterstitial([]byte(tt.html))
			if got != tt.want {
				t.Fatalf("DetectAuthInterstitial() = %v, want %v (reason=%q)", got, tt.want, reason)
			}
			if tt.want && tt.reason != "" && !strings.Contains(reason, tt.reason) {
				t.Errorf("reason = %q, want it to contain %q", reason, tt.reason)
			}
			err := AuthInterstitialError([]byte(tt.html))
			if tt.want && err == nil {
				t.Errorf("AuthInterstitialError() = nil, want non-nil")
			}
			if !tt.want && err != nil {
				t.Errorf("AuthInterstitialError() = %v, want nil", err)
			}
		})
	}
}

// A real order page must still parse to non-empty orders after the
// interstitial guard — detection must not false-positive on authenticated HTML.
func TestDetectAuthInterstitial_DoesNotBreakRealOrderPage(t *testing.T) {
	html := `<html><head><title>Your Orders</title></head><body>
		<div class="order-card js-order-card">ORDER PLACED May 5, 2026 TOTAL $51.46 SHIP TO Jane ORDER # 111-1111111-1111111</div>
	</body></html>`
	if ok, reason := DetectAuthInterstitial([]byte(html)); ok {
		t.Fatalf("real order page flagged as interstitial: %s", reason)
	}
	page, err := ParseOrderList([]byte(html))
	if err != nil {
		t.Fatalf("ParseOrderList: %v", err)
	}
	if len(page.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(page.Orders))
	}
}

func TestDetectOrderHistoryPage(t *testing.T) {
	tests := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "your orders title",
			html: `<html><head><title>Your Orders</title></head><body></body></html>`,
			want: true,
		},
		{
			name: "spanish orders title",
			html: `<html><head><title>Mis pedidos</title></head><body></body></html>`,
			want: true,
		},
		{
			name: "german empty orders title",
			html: `<html><head><title>Ihre Bestellungen</title></head><body></body></html>`,
			want: true,
		},
		{
			name: "french empty orders title",
			html: `<html><head><title>Vos commandes</title></head><body></body></html>`,
			want: true,
		},
		{
			name: "japanese empty orders title",
			html: `<html><head><title>注文履歴</title></head><body></body></html>`,
			want: true,
		},
		{
			name: "chinese empty orders title",
			html: `<html><head><title>我的订单</title></head><body></body></html>`,
			want: true,
		},
		{
			name: "order card",
			html: `<html><body><div class="order-card js-order-card">ORDER # 111-1111111-1111111</div></body></html>`,
			want: true,
		},
		{
			name: "generic html",
			html: `<html><head><title>Amazon.com</title></head><body>hello</body></html>`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectOrderHistoryPage([]byte(tt.html)); got != tt.want {
				t.Fatalf("DetectOrderHistoryPage() = %v, want %v", got, tt.want)
			}
		})
	}
}
