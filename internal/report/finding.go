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

// sortKey is the total order defined in §8 step 4:
// (point, subject_id, secondary_id_or_action, scope).
type sortKey struct {
	point     string
	subject   string
	secondary string
	scope     string
}

func keyOf(f interface{}) sortKey {
	switch v := f.(type) {
	case EdgeFinding:
		return sortKey{v.Point, v.Delegator, v.Delegatee, ""}
	case OperationFinding:
		return sortKey{v.Point, v.Actor, v.Action, v.Requires}
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
	return a.scope < b.scope
}

// Sort orders findings (a mix of EdgeFinding and OperationFinding values) in
// place per the deterministic total order in §8 step 4. The order is a pure
// function of finding content, never of slice/map iteration order.
func Sort(findings []interface{}) {
	sort.SliceStable(findings, func(i, j int) bool {
		return less(keyOf(findings[i]), keyOf(findings[j]))
	})
}
