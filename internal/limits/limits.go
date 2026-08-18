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

	// MaxLifecycleStates is the maximum number of entries a version-6
	// approvals[] record's lifecycle.states array may declare
	// (docs/phase-6-plan.md §22). A validate-time structural bound: an
	// approval lifecycle is a small, human-authored policy artifact, not a
	// generated structure, so 32 distinct named states is generous
	// headroom over any realistic workflow.
	MaxLifecycleStates = 32

	// MaxLifecycleTransitions is the maximum number of entries a
	// version-6 approvals[] record's lifecycle.transitions array may
	// declare (docs/phase-6-plan.md §22). Set to 4x MaxLifecycleStates:
	// generous headroom for a densely-connected small automaton, while
	// staying well inside the theoretical maximum a MaxLifecycleStates-
	// state graph could ever express (32^2 = 1024; 128 <= 1024).
	MaxLifecycleTransitions = 128

	// MaxExplorationStatesPerLifecycle is the runtime BFS visited-state
	// safety valve passed as internal/explore.Explore's maxStates
	// parameter (docs/phase-6-plan.md §22). A resource-safety valve on the
	// algorithm's own execution, distinct in kind from MaxLifecycleStates
	// (a bound on what a document may declare) — the same
	// two-independent-bounds-same-default-value pattern already
	// established by MaxChainDepth/MaxDelegationDepth. Set equal to
	// MaxLifecycleStates because a BFS visited-set can never legitimately
	// need to exceed the number of states a validate-time-legal document
	// could possibly declare (docs/phase-6-plan.md §22.1) — this bound
	// exists purely as defense-in-depth, never triggered by legitimate
	// input.
	MaxExplorationStatesPerLifecycle = 32
)
