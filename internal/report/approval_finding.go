package report

import (
	"fmt"
	"strings"
)

// ViolationApprovalMissing/ViolationApprovalUnauthorized are the Phase 5
// violation literals (docs/phase-5-plan.md §8, §12, §14): a capability is
// validly held by the actor, correctly bound, and the requester is
// authorized to request it — every Phase 1-4 invariant already passes — but
// the capability's origin declaration requires approval and either no
// approval record exists at all (approval_missing), or every declared
// approval record names an approver who does not independently hold the
// capability (approval_unauthorized). Emitted only by ApprovalFinding,
// always at point "operation" — approval gates exercise, never delegation
// (§4.2, §11), so there is no edge-level counterpart.
const (
	ViolationApprovalMissing      = "approval_missing"
	ViolationApprovalUnauthorized = "approval_unauthorized"
)

// ApprovalFinding is always an operation-level finding (point =
// "operation"). DeclaredApprovers is [] for approval_missing (no record at
// all exists for this capability) and the full sorted, deduplicated set of
// approvers named by matching records for approval_unauthorized (none of
// whom independently hold the capability — §11's strict distrust means a
// non-standing record contributes nothing, but is still surfaced here for
// diagnostic completeness).
type ApprovalFinding struct {
	Violation         string     `json:"violation"`
	Point             string     `json:"point"`
	Actor             string     `json:"actor"`
	Requester         string     `json:"requester"`
	Action            string     `json:"action"`
	Requires          Capability `json:"requires"`
	DeclaredApprovers []string   `json:"declared_approvers"`
	Trace             []string   `json:"trace"`
	Reason            string     `json:"reason"`
}

// NewApprovalFinding constructs a Phase 5 finding with its deterministic
// reason text (§14). violation must be ViolationApprovalMissing or
// ViolationApprovalUnauthorized. declaredApprovers/trace must already be in
// canonical order.
func NewApprovalFinding(violation, actor, requester, action string, requires Capability, declaredApprovers []string, trace []string) ApprovalFinding {
	var reason string
	switch violation {
	case ViolationApprovalUnauthorized:
		reason = fmt.Sprintf(
			"%s requires %s, which requires approval; approval was declared by [%s], but none of them independently hold %s — an approval must come from a party with standing over the capability being approved",
			action, requires.String(), strings.Join(declaredApprovers, ", "), requires.String(),
		)
	default:
		reason = fmt.Sprintf(
			"%s requires %s, which %s validly holds and %s is authorized to request, but %s requires approval and no approval has been declared for it",
			action, requires.String(), actor, requester, requires.String(),
		)
	}
	return ApprovalFinding{
		Violation:         violation,
		Point:             PointOperation,
		Actor:             actor,
		Requester:         requester,
		Action:            action,
		Requires:          requires,
		DeclaredApprovers: nonNil(declaredApprovers),
		Trace:             nonNil(trace),
		Reason:            reason,
	}
}
