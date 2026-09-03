// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"
)

func TestClassifyAPIErrorDoesNotConvertConflictToSuccess(t *testing.T) {
	err := classifyAPIError(errors.New("POST /ship/v1/shipments: HTTP 409: conflict"))
	if err == nil {
		t.Fatal("HTTP 409 was incorrectly converted to success")
	}
}
