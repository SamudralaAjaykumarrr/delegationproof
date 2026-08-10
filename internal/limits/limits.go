// Package limits defines the fixed resource bounds Phase 1 enforces on any
// input model (see docs/phase-1-plan.md §12). Bounds are exported variables,
// not untyped constants, specifically so tests can lower them to construct
// small fixtures that exceed a bound without generating pathologically large
// input files.
package limits

var (
	// MaxInputFileSize is the maximum size, in bytes, of an input model file.
	MaxInputFileSize int64 = 5 * 1024 * 1024

	// MaxNodes is the maximum number of principals plus agents.
	MaxNodes = 10000

	// MaxDelegationEdges is the maximum number of delegation edges.
	MaxDelegationEdges = 50000

	// MaxOperations is the maximum number of operations.
	MaxOperations = 10000

	// MaxScopeLength is the maximum byte length of a scope string.
	MaxScopeLength = 256

	// MaxIDLength is the maximum byte length of a node id.
	MaxIDLength = 128

	// MaxAuthoritySetSize is the maximum number of scopes in a single
	// authority set (a principal's declared authority, or one delegation
	// edge's granted authority).
	MaxAuthoritySetSize = 256

	// MaxChainDepth is the maximum longest-simple-path length, in edges,
	// through the delegation graph. This is a resource-safety valve
	// against pathological input, not a policy-level depth-limit
	// invariant (that is a distinct, deferred Phase 2+ concept).
	MaxChainDepth = 64
)
