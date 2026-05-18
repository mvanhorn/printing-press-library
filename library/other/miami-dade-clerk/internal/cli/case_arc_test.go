// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

func rec(code, date, first, second string) *store.Recording {
	return &store.Recording{DocTypeCode: code, RecordingDate: date, FirstParty: first, SecondParty: second}
}

func TestClassifyCaseStatus(t *testing.T) {
	cases := []struct {
		name       string
		docs       []*store.Recording
		wantStatus string
		wantOwner  string
	}{
		{
			name:       "open / only LIS filed",
			docs:       []*store.Recording{rec("LIS", "2024-01-01", "BANK", "OWNER")},
			wantStatus: "open",
		},
		{
			name:       "judgment entered, not yet resolved",
			docs:       []*store.Recording{rec("LIS", "2024-01-01", "BANK", "OWNER"), rec("JUD", "2024-06-01", "BANK", "OWNER")},
			wantStatus: "judgment-entered",
		},
		{
			name:       "sale completed via certificate of title",
			docs:       []*store.Recording{rec("JUD", "2024-06-01", "BANK", "OWNER"), rec("CTI", "2024-09-01", "CLERK", "INVESTOR LLC")},
			wantStatus: "sale-complete",
			wantOwner:  "INVESTOR LLC",
		},
		{
			name:       "satisfied via SJU (Satisfaction of Judgment)",
			docs:       []*store.Recording{rec("JUD", "2024-06-01", "BANK", "OWNER"), rec("SJU", "2024-07-15", "BANK", "OWNER")},
			wantStatus: "satisfied",
		},
		{
			name:       "SAT (Satisfaction of Mortgage) does NOT close a court case",
			docs:       []*store.Recording{rec("JUD", "2024-06-01", "BANK", "OWNER"), rec("SAT", "2024-07-15", "BANK", "OWNER")},
			wantStatus: "judgment-entered",
		},
		{
			name:       "dismissed without sale",
			docs:       []*store.Recording{rec("LIS", "2024-01-01", "BANK", "OWNER"), rec("DIS", "2024-03-01", "COURT", "OWNER")},
			wantStatus: "dismissed",
		},
		{
			name:       "post-sale dismissal does not override sale-complete",
			docs:       []*store.Recording{rec("JUD", "2024-06-01", "BANK", "OWNER"), rec("CTI", "2024-09-01", "CLERK", "INVESTOR LLC"), rec("DIS", "2024-12-01", "COURT", "OWNER")},
			wantStatus: "sale-complete",
			wantOwner:  "INVESTOR LLC",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotOwner := classifyCaseStatus(tc.docs)
			if gotStatus != tc.wantStatus {
				t.Errorf("status = %q, want %q", gotStatus, tc.wantStatus)
			}
			if gotOwner != tc.wantOwner {
				t.Errorf("owner = %q, want %q", gotOwner, tc.wantOwner)
			}
		})
	}
}
