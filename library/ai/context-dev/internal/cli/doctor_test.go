// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestLooksLikeDoctorInterstitialVercelBodyMarkers(t *testing.T) {
	t.Parallel()
	if got := looksLikeDoctorInterstitial([]byte(`<html><head><title>Vercel Security Challenge</title></head></html>`)); got != "Vercel" {
		t.Fatalf("looksLikeDoctorInterstitial Vercel body marker = %q, want Vercel", got)
	}
	if got := looksLikeDoctorInterstitial([]byte(`<html><head><title>x-vercel-mitigated</title></head></html>`)); got != "" {
		t.Fatalf("looksLikeDoctorInterstitial matched header name in body = %q, want empty", got)
	}
}
