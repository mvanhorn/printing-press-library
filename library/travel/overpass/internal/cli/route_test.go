// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestProjectedRouteProgressOrdersByTravelDirection(t *testing.T) {
	earlierOffRoute := projectedRouteProgressKM(0, 0, 0, 10, 5, 3)
	laterOnRoute := projectedRouteProgressKM(0, 0, 0, 10, 0, 4)
	if earlierOffRoute >= laterOnRoute {
		t.Fatalf("off-route point at longitude 3 has progress %.2f km; on-route point at longitude 4 has %.2f km", earlierOffRoute, laterOnRoute)
	}
	if earlierOffRoute < 320 || earlierOffRoute > 350 {
		t.Errorf("earlier progress = %.2f km, want approximately 333 km", earlierOffRoute)
	}
}

func TestProjectedRouteProgressClampsOutsideSegment(t *testing.T) {
	if got := projectedRouteProgressKM(0, 0, 0, 10, 0, -2); got != 0 {
		t.Errorf("point behind start = %.2f km, want 0", got)
	}
	total := projectedRouteProgressKM(0, 0, 0, 10, 0, 10)
	if got := projectedRouteProgressKM(0, 0, 0, 10, 0, 12); got != total {
		t.Errorf("point beyond end = %.2f km, want %.2f km", got, total)
	}
}
