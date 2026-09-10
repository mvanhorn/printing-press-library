// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// This spec declares auth.type: none, so two generator-emitted helpers have no
// caller in the generated tree: readSecretFromStdin (interactive credential
// entry) and successfulNoop (the no-op-write classifier for auth-guarded
// paths). They are kept referenced rather than deleted because the next
// `generate --force` would restore them and re-introduce the same dead-code
// finding; deleting them here would also delete scaffolding the generator
// expects to own.
var (
	_ = readSecretFromStdin
	_ = successfulNoop
)
