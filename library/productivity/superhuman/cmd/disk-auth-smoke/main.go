package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

func main() {
	// PATCH(pii-scrub): default smoke-test inputs use example.com placeholders
	// instead of real session-specific addresses. Real accounts come through
	// os.Args at invocation time.
	email := "user@example.com"
	googleID := "123456789012345678901"
	if len(os.Args) > 1 {
		email = os.Args[1]
	}
	if len(os.Args) > 2 {
		googleID = os.Args[2]
	}
	fmt.Fprintf(os.Stderr, "=== Refreshing tokens for %s (googleId=%s) ===\n", email, googleID)
	r, err := auth.RefreshFromChromeCookies(context.Background(), email, googleID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "  email: %s\n", r.Email)
	fmt.Fprintf(os.Stderr, "  externalId: %s\n", r.ExternalID)
	fmt.Fprintf(os.Stderr, "  idToken: len=%d prefix=%s\n", len(r.IDToken), r.IDToken[:25])
	expSec := (r.IDTokenExpires - time.Now().UnixMilli()) / 1000
	fmt.Fprintf(os.Stderr, "  expires in: %d seconds\n", expSec)
	fmt.Fprintf(os.Stderr, "  deviceId: %s\n", r.DeviceID)
	fmt.Fprintln(os.Stderr)

	fmt.Fprintln(os.Stderr, "=== GET users.achievements ===")
	req, _ := http.NewRequest("GET", "https://mail.superhuman.com/~backend/v3/users.achievements", nil)
	auth.AddSuperhumanBackendHeaders(req, r)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d\n", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "hoursSaved") {
		fmt.Fprintf(os.Stderr, "*** SUCCESS: real achievements data ***\n")
		preview := string(body)
		if len(preview) > 250 {
			preview = preview[:250] + "..."
		}
		fmt.Fprintf(os.Stderr, "  preview: %s\n", preview)
	} else {
		fmt.Fprintf(os.Stderr, "Body: %s\n", string(body))
	}
}
