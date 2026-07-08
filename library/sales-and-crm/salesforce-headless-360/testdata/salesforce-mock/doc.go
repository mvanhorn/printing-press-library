// Package sfmock provides an in-repository Salesforce HTTP mock for tests and
// for `doctor --mock`.
//
// The mock serves hand-authored REST API v63.0 fixture envelopes for the
// Salesforce surfaces this CLI calls: Composite Graph, UI API records, Tooling,
// Data Cloud, SOQL query, limits, and basic sObject reads. It intentionally uses
// only net/http and a small path-pattern matcher so downstream tests can depend
// on deterministic Salesforce-shaped responses without pulling in a router or a
// live org.
package sfmock
