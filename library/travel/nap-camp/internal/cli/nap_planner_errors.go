// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import "fmt"

func napWindowNightsError() error { return fmt.Errorf("--nights must be 1..14") }
