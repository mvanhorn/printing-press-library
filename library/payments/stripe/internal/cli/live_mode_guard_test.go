// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCheckLiveModeGuard(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		envKey      string
		envConfirm  string
		flagConfirm bool
		wantBlock   bool
	}{
		{
			name:        "framework command no annotation passes",
			annotations: nil,
			envKey:      "sk_live_REAL",
			wantBlock:   false,
		},
		{
			name:        "GET with live key passes",
			annotations: map[string]string{"pp:method": "GET"},
			envKey:      "sk_live_REAL",
			wantBlock:   false,
		},
		{
			name:        "POST with test key passes",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_test_FINE",
			wantBlock:   false,
		},
		{
			name:        "POST with live key and no confirmation blocks",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "DELETE with live key and no confirmation blocks",
			annotations: map[string]string{"pp:method": "DELETE"},
			envKey:      "sk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "PATCH with live key and no confirmation blocks",
			annotations: map[string]string{"pp:method": "PATCH"},
			envKey:      "sk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "POST with live key but --confirm-live passes",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_live_REAL",
			flagConfirm: true,
			wantBlock:   false,
		},
		{
			name:        "POST with live key but STRIPE_CONFIRM_LIVE=1 passes",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_live_REAL",
			envConfirm:  "1",
			wantBlock:   false,
		},
		{
			name:        "rk_live_ restricted-key prefix also triggers guard",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "rk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "no key set passes (no live-mode signal)",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "",
			wantBlock:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STRIPE_SECRET_KEY", tc.envKey)
			t.Setenv("STRIPE_CONFIRM_LIVE", tc.envConfirm)

			cmd := &cobra.Command{Use: "test"}
			if tc.annotations != nil {
				cmd.Annotations = tc.annotations
			}
			flags := &rootFlags{confirmLive: tc.flagConfirm}

			err := checkLiveModeGuard(cmd, flags)
			gotBlock := err != nil
			if gotBlock != tc.wantBlock {
				t.Errorf("checkLiveModeGuard: wantBlock=%v gotBlock=%v err=%v",
					tc.wantBlock, gotBlock, err)
			}
		})
	}
}
