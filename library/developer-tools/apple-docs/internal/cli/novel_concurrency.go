// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

package cli

// novelHTTPConcurrency caps the number of in-flight HTTP fetches the
// hand-authored novel commands (deprecation-cliff, wwdc symbols) launch
// in parallel. Apple's DocC CDN is generous, but four feels right for a
// terminal command — fast enough to make a difference on framework-wide
// scans, slow enough to stay polite. Shared so both commands agree.
const novelHTTPConcurrency = 4
