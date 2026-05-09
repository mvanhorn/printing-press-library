// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import "os"

func getEnvImpl(k string) string { return os.Getenv(k) }
