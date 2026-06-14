// Package rtagent exposes the first public SDK surface for embedding the
// RTAgent core runtime in another Go process.
//
// The SDK intentionally presents a narrow facade over the current internal
// runtime. SubmitRun drives a runnable core loop, while lower-level ports keep
// storage, governance, context materialization, model turns, tool execution,
// and read-side projections replaceable by host applications.
//
// Public compatibility for v1 is documented in docs/sdk-handbook.md. Hosts
// should integrate through this package and Config.Host ports instead of
// depending on internal startup, persistence, or product shell packages.
package rtagent
