// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package slackanalytics holds the pure logic behind the local-archive
// commands (recall, catchup, archive coverage, threads stale, health,
// users activity, users whois): Slack timestamp math, retention-wall
// arithmetic, mention extraction, median/rate statistics, and user
// reference resolution.
//
// Everything here is deterministic and free of I/O so the CLI layer can
// stay a thin shell over SQLite queries and this logic can be unit tested
// directly.
package slackanalytics
