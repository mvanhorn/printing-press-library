// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import "github.com/mvanhorn/printing-press-library/library/commerce/square/internal/cliutil"

func verifyPrivateFile(path string) error {
	return cliutil.VerifyCredsPerms(path)
}
