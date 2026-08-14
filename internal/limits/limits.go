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

	// MaxTargetLength is the maximum byte length of a Phase 2 capability
	// target string (docs/phase-2-plan.md §17). Mirrors MaxIDLength, since
	// the target grammar mirrors the id grammar (§5).
	MaxTargetLength = 128

	// MaxDelegationDepth is the maximum value a document may declare for a
	// RootCapability's max_delegation_depth (docs/phase-4-plan.md §21). This
	// is a resource-safety bound on the declared value, distinct from
	// MaxChainDepth (the resource-safety valve on actual graph shape, not a
	// policy invariant — see MaxChainDepth's own comment). Kept as an
	// independent var, not an alias, so a test can lower one without
	// perturbing the other.
	MaxDelegationDepth = 64

	// MaxApprovals is the maximum number of entries in a version-5
	// document's top-level approvals array (docs/phase-5-plan.md §21).
	// approvals is a new, independent top-level array, not nested inside
	// any existing bounded collection, so it needs its own bound rather
	// than reusing MaxAuthoritySetSize or MaxOperations. Mirrors
	// MaxOperations's value and role.
	MaxApprovals = 10000
)
