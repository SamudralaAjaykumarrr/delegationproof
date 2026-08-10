package report

import (
	"fmt"
	"strings"
)

// Capability is the Phase 2 finding-contract rendering of a capability
// tuple (docs/phase-2-plan.md §12). It intentionally does not import
// internal/model — report stays decoupled from the domain layer, exactly
// as Phase 1's EdgeFinding/OperationFinding use bare strings rather than a
// model type.
type Capability struct {
	Scope  string `json:"scope"`
	Target string `json:"target"`
}

// String renders a capability the safe "scope@target" way (§4): "@"
// appears in neither the scope nor target grammar, so this round-trips
// unambiguously for humans.
func (c Capability) String() string {
	return c.Scope + "@" + c.Target
}

// CapabilityEdgeFinding is a Phase 2 edge-level finding: a delegation edge
// grants a capability set beyond the delegator's own derived authority,
// classified per §8 as either authority_amplification or
// context_binding_violation.
type CapabilityEdgeFinding struct {
	Violation    string       `json:"violation"`
	Point        string       `json:"point"`
	Delegator    string       `json:"delegator"`
	Delegatee    string       `json:"delegatee"`
	Declared     []Capability `json:"declared"`
	Excess       []Capability `json:"excess"`
	BoundTargets []string     `json:"bound_targets"`
	Trace        []string     `json:"trace"`
	Reason       string       `json:"reason"`
}

// CapabilityOperationFinding is a Phase 2 operation-level finding: an actor
// attempts an operation requiring a capability it does not hold.
type CapabilityOperationFinding struct {
	Violation    string       `json:"violation"`
	Point        string       `json:"point"`
	Actor        string       `json:"actor"`
	Action       string       `json:"action"`
	Requires     Capability   `json:"requires"`
	Held         []Capability `json:"held"`
	BoundTargets []string     `json:"bound_targets"`
	Trace        []string     `json:"trace"`
	Reason       string       `json:"reason"`
}

// NewCapabilityEdgeFinding constructs an edge-level Phase 2 finding with
// its deterministic reason text. violation must be
// ViolationAuthorityAmplification or ViolationContextBinding (§8's
// precedence rule decides which, at the call site in internal/verify).
// declared/excess/boundTargets/trace must already be in canonical order.
func NewCapabilityEdgeFinding(violation, delegator, delegatee string, declared, excess []Capability, boundTargets, trace []string) CapabilityEdgeFinding {
	var reason string
	switch violation {
	case ViolationContextBinding:
		reason = fmt.Sprintf(
			"%s attempted to delegate %s, which %s holds only for %s",
			delegator, joinCapabilities(excess, ", "), delegator, strings.Join(boundTargets, ", "),
		)
	default:
		reason = fmt.Sprintf(
			"%s attempted to delegate %s, which is not in %s's derived authority",
			delegator, joinCapabilities(excess, ", "), delegator,
		)
	}
	return CapabilityEdgeFinding{
		Violation:    violation,
		Point:        PointDelegationEdge,
		Delegator:    delegator,
		Delegatee:    delegatee,
		Declared:     nonNilCaps(declared),
		Excess:       nonNilCaps(excess),
		BoundTargets: nonNil(boundTargets),
		Trace:        nonNil(trace),
		Reason:       reason,
	}
}

// NewCapabilityOperationFinding constructs an operation-level Phase 2
// finding with its deterministic reason text (§12). held/boundTargets/trace
// must already be in canonical order.
func NewCapabilityOperationFinding(violation, actor, action string, requires Capability, held []Capability, boundTargets, trace []string) CapabilityOperationFinding {
	var reason string
	switch violation {
	case ViolationContextBinding:
		reason = fmt.Sprintf(
			"%s is held by %s only for %s, which does not include %s",
			requires.Scope, actor, strings.Join(boundTargets, ", "), requires.Target,
		)
	default:
		reason = fmt.Sprintf(
			"%s was never present in the valid delegation chain reaching %s",
			requires.String(), actor,
		)
	}
	return CapabilityOperationFinding{
		Violation:    violation,
		Point:        PointOperation,
		Actor:        actor,
		Action:       action,
		Requires:     requires,
		Held:         nonNilCaps(held),
		BoundTargets: nonNil(boundTargets),
		Trace:        nonNil(trace),
		Reason:       reason,
	}
}

// nonNilCaps guarantees array-valued Capability fields marshal as JSON []
// rather than null when empty, mirroring nonNil's rule for []string fields.
func nonNilCaps(c []Capability) []Capability {
	if c == nil {
		return []Capability{}
	}
	return c
}

func joinCapabilities(caps []Capability, sep string) string {
	parts := make([]string, len(caps))
	for i, c := range caps {
		parts[i] = c.String()
	}
	return strings.Join(parts, sep)
}

func joinCapabilitiesOrNone(caps []Capability, sep string) string {
	if len(caps) == 0 {
		return "(none)"
	}
	return joinCapabilities(caps, sep)
}
