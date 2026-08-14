// Package report defines the Phase 1 finding contract (docs/phase-1-plan.md
// §9), its deterministic sort order, and text/json renderers.
package report

import (
	"fmt"
	"sort"
	"strings"
)

// ViolationAuthorityAmplification is the only violation literal Phase 1
// emits. The field exists so later phases can add new literal values
// without a breaking schema change.
const ViolationAuthorityAmplification = "authority_amplification"

// ViolationContextBinding is the Phase 2 violation literal
// (docs/phase-2-plan.md §3, §7): a scope was validly granted, but only for
// a different target than the one exercised or transmitted. Emitted only
// by CapabilityEdgeFinding/CapabilityOperationFinding, never by Phase 1's
// EdgeFinding/OperationFinding.
const ViolationContextBinding = "context_binding_violation"

const (
	PointDelegationEdge = "delegation_edge"
	PointOperation      = "operation"
)

// EdgeFinding is an edge-level authority-amplification finding: a
// delegation edge grants a scope beyond the delegator's own derived
// authority.
type EdgeFinding struct {
	Violation string   `json:"violation"`
	Point     string   `json:"point"`
	Delegator string   `json:"delegator"`
	Delegatee string   `json:"delegatee"`
	Declared  []string `json:"declared"`
	Excess    []string `json:"excess"`
	Trace     []string `json:"trace"`
	Reason    string   `json:"reason"`
}

// OperationFinding is an operation-level authority-amplification finding:
// an actor attempts an operation requiring a scope it does not hold.
type OperationFinding struct {
	Violation string   `json:"violation"`
	Point     string   `json:"point"`
	Actor     string   `json:"actor"`
	Action    string   `json:"action"`
	Requires  string   `json:"requires"`
	Held      []string `json:"held"`
	Trace     []string `json:"trace"`
	Reason    string   `json:"reason"`
}

// NewEdgeFinding constructs an edge-level finding with its deterministic
// reason text. declared/excess/trace must already be in canonical order.
func NewEdgeFinding(delegator, delegatee string, declared, excess, trace []string) EdgeFinding {
	return EdgeFinding{
		Violation: ViolationAuthorityAmplification,
		Point:     PointDelegationEdge,
		Delegator: delegator,
		Delegatee: delegatee,
		Declared:  nonNil(declared),
		Excess:    nonNil(excess),
		Trace:     nonNil(trace),
		Reason: fmt.Sprintf(
			"%s attempted to delegate %s, which is not in %s's derived authority",
			delegator, strings.Join(excess, ", "), delegator,
		),
	}
}

// NewOperationFinding constructs an operation-level finding with its
// deterministic reason text. held/trace must already be in canonical order.
func NewOperationFinding(actor, action, requires string, held, trace []string) OperationFinding {
	return OperationFinding{
		Violation: ViolationAuthorityAmplification,
		Point:     PointOperation,
		Actor:     actor,
		Action:    action,
		Requires:  requires,
		Held:      nonNil(held),
		Trace:     nonNil(trace),
		Reason: fmt.Sprintf(
			"%s was never present in the valid delegation chain reaching %s",
			requires, actor,
		),
	}
}

// nonNil guarantees array-valued finding fields marshal as JSON [] rather
// than null when empty — the finding contract (§9) never omits or nulls an
// array field.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// sortKey is the total order defined in §8 step 4, extended by
// docs/phase-2-plan.md §12 with a trailing target field, and by
// docs/phase-3-plan.md §21 with a further-trailing requester field:
// (point, subject_id, secondary_id_or_action, scope, target, requester).
// For any Phase-1-shaped finding, target and requester are always "", and
// for any Phase-1/2-shaped finding requester is always "" — so this is a
// strict extension of the prior order: it degenerates to exactly the
// original order whenever the trailing field(s) are uniformly empty.
// requester is needed because two version-3 operations can legitimately
// share (actor, action, requires.Scope, requires.Target) and differ only by
// requester (docs/phase-3-plan.md §22).
type sortKey struct {
	point     string
	subject   string
	secondary string
	scope     string
	target    string
	requester string
}

func keyOf(f interface{}) sortKey {
	switch v := f.(type) {
	case EdgeFinding:
		return sortKey{point: v.Point, subject: v.Delegator, secondary: v.Delegatee}
	case OperationFinding:
		return sortKey{point: v.Point, subject: v.Actor, secondary: v.Action, scope: v.Requires}
	case CapabilityEdgeFinding:
		return sortKey{point: v.Point, subject: v.Delegator, secondary: v.Delegatee}
	case CapabilityOperationFinding:
		return sortKey{point: v.Point, subject: v.Actor, secondary: v.Action, scope: v.Requires.Scope, target: v.Requires.Target}
	case ConfusedDeputyFinding:
		return sortKey{point: v.Point, subject: v.Actor, secondary: v.Action, scope: v.Requires.Scope, target: v.Requires.Target, requester: v.Requester}
	case DelegationDepthFinding:
		return sortKey{point: v.Point, subject: v.Delegator, secondary: v.Delegatee}
	case ApprovalFinding:
		return sortKey{point: v.Point, subject: v.Actor, secondary: v.Action, scope: v.Requires.Scope, target: v.Requires.Target, requester: v.Requester}
	default:
		panic(fmt.Sprintf("report: unknown finding type %T", f))
	}
}

func less(a, b sortKey) bool {
	if a.point != b.point {
		return a.point < b.point
	}
	if a.subject != b.subject {
		return a.subject < b.subject
	}
	if a.secondary != b.secondary {
		return a.secondary < b.secondary
	}
	if a.scope != b.scope {
		return a.scope < b.scope
	}
	if a.target != b.target {
		return a.target < b.target
	}
	return a.requester < b.requester
}

// Sort orders findings (a mix of EdgeFinding and OperationFinding values) in
// place per the deterministic total order in §8 step 4. The order is a pure
// function of finding content, never of slice/map iteration order.
func Sort(findings []interface{}) {
	sort.SliceStable(findings, func(i, j int) bool {
		return less(keyOf(findings[i]), keyOf(findings[j]))
	})
}
