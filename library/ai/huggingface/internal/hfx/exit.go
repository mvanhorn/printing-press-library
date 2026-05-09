// Package hfx contains huggingface-pp-cli-specific extensions to cliutil that
// implement the multi-runtime portability constraints from the seed plan
// (docs/plans/2026-05-09-hf-cli-printing-press-seed.md):
// stable JSON schemas, structured exit codes, flock-guarded state writes,
// shared rate-limit bucket, as_of stamps, state-dir resolution, and the
// embedded backend-support matrix.
package hfx

// Exit codes per seed plan. Generic CLI plumbing emits 0/1; these are the
// hf-specific structured codes callers (Rick at the terminal AND JARVIS et al.)
// branch on.
const (
	ExitOK                = 0 // success
	ExitGenericError      = 1 // unhandled error (Cobra default)
	ExitNotFound          = 2 // model/uploader/feature not found
	ExitBackendUnsupported = 3 // candidate's arch is not supported by the chosen backend
	ExitAlreadyCached     = 4 // operation was a no-op because the resource is already on disk / in store
	ExitRateLimited       = 5 // HF rate limit exceeded; shared bucket exhausted
	ExitConfigMissing     = 6 // openclaw.json / harness data dir / etc. not present at expected path
)

// SchemaVersion is stamped on every JSON response so agents can introspect
// structure stability. Bump when output shape changes incompatibly.
const SchemaVersion = 1
