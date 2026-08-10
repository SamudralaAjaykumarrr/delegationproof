// Package exitcode defines the four Phase 1 process exit codes
// (docs/phase-1-plan.md §11) and is the single source of truth main.go maps
// every outcome onto.
package exitcode

// Code is a Phase 1 CLI exit code.
type Code int

const (
	// OK: clean pass — model structurally valid, zero findings (verify),
	// or model structurally valid (validate).
	OK Code = 0

	// Deny: invariant violated — one or more findings (verify only).
	Deny Code = 1

	// ModelError: file not found/unreadable, invalid JSON, any structural
	// validation error, or a resource bound exceeded. Applies to both
	// validate and verify.
	ModelError Code = 2

	// UsageError: wrong number of arguments, unknown flag, unknown
	// subcommand. Never about the model's content.
	UsageError Code = 3
)
