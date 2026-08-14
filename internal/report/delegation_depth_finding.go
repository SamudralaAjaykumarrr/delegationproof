package report

import (
	"fmt"
	"strings"
)

// ViolationDelegationDepth is the Phase 4 violation literal
// (docs/phase-4-plan.md §8, §14): a delegation edge carries a capability
// that is genuinely, validly held by the delegator — Non-Amplification and
// Context-Binding Preservation both pass — but the delegator's remaining
// re-delegation budget for it, inherited from its root declaration, is
// already exhausted. Emitted only by DelegationDepthFinding, always at
// point "delegation_edge" — delegation depth gates transmission, never use,
// so it never manifests as an operation-level finding of its own (§12,
// §13).
const ViolationDelegationDepth = "delegation_depth_violation"

// DepthExcess is one capability, within an invalid edge's declared set,
// that failed specifically because its delegator's remaining re-delegation
// budget for it was exhausted (0) — never because of a presence or binding
// failure, which take precedence (§12) and are reported via
// CapabilityEdgeFinding instead. It is a struct array rather than parallel
// scalar fields on the finding itself because two capabilities in the same
// poisoned edge can legitimately have different configured budgets.
type DepthExcess struct {
	Scope          string `json:"scope"`
	Target         string `json:"target"`
	ConfiguredMax  int    `json:"configured_max_depth"`
	RemainingDepth int    `json:"remaining_depth"`
}

// DelegationDepthFinding is a Phase 4 edge-level finding: a delegation edge
// attempts to transmit one or more capabilities beyond their declared
// re-delegation budget. point is always "delegation_edge".
type DelegationDepthFinding struct {
	Violation string        `json:"violation"`
	Point     string        `json:"point"`
	Delegator string        `json:"delegator"`
	Delegatee string        `json:"delegatee"`
	Declared  []Capability  `json:"declared"`
	Excess    []DepthExcess `json:"excess"`
	Trace     []string      `json:"trace"`
	Reason    string        `json:"reason"`
}

// NewDelegationDepthFinding constructs an edge-level Phase 4 finding with
// its deterministic reason text (§14). declared/excess/trace must already
// be in canonical order.
func NewDelegationDepthFinding(delegator, delegatee string, declared []Capability, excess []DepthExcess, trace []string) DelegationDepthFinding {
	var reason string
	if len(excess) == 1 {
		e := excess[0]
		reason = fmt.Sprintf(
			"%s attempted to delegate %s@%s to %s, but %s's remaining delegation budget for this capability is 0 (configured maximum: %d) — it may no longer be redelegated",
			delegator, e.Scope, e.Target, delegatee, delegator, e.ConfiguredMax,
		)
	} else {
		parts := make([]string, len(excess))
		for i, e := range excess {
			parts[i] = fmt.Sprintf("%s@%s (configured maximum: %d)", e.Scope, e.Target, e.ConfiguredMax)
		}
		reason = fmt.Sprintf(
			"%s attempted to delegate [%s] to %s, but %s's remaining delegation budget for each is 0 — none may be redelegated further",
			delegator, strings.Join(parts, ", "), delegatee, delegator,
		)
	}
	return DelegationDepthFinding{
		Violation: ViolationDelegationDepth,
		Point:     PointDelegationEdge,
		Delegator: delegator,
		Delegatee: delegatee,
		Declared:  nonNilCaps(declared),
		Excess:    nonNilDepthExcess(excess),
		Trace:     nonNil(trace),
		Reason:    reason,
	}
}

// nonNilDepthExcess guarantees the Excess field marshals as JSON []
// rather than null when empty, mirroring nonNilCaps's rule.
func nonNilDepthExcess(e []DepthExcess) []DepthExcess {
	if e == nil {
		return []DepthExcess{}
	}
	return e
}

// joinDepthExcessOrNone renders an excess list for text output.
func joinDepthExcessOrNone(items []DepthExcess, sep string) string {
	if len(items) == 0 {
		return "(none)"
	}
	parts := make([]string, len(items))
	for i, e := range items {
		parts[i] = fmt.Sprintf("%s@%s (configured max: %d, remaining: %d)", e.Scope, e.Target, e.ConfiguredMax, e.RemainingDepth)
	}
	return strings.Join(parts, sep)
}
