package report

import (
	"fmt"
	"strings"
)

// ViolationApprovalLifecycleUnsafe/ViolationApprovalLifecycleUnproven are
// the Phase 6 violation literals (docs/phase-6-plan.md §9, §16.6, §18): a
// capability is validly held by the actor, correctly bound, the requester
// is authorized to request it, and it requires approval — every Phase 1-5
// invariant already passes, and at least one standing-backed approval
// record exists (Phase 5's own check already passed) — but none of those
// standing-backed records' declared lifecycles can be proven to remain
// permanently in the "approved" state. approval_lifecycle_unsafe means at
// least one such record was completely explored and proven to reach a
// non-"approved" state; approval_lifecycle_unproven means every remaining
// candidate's exploration was truncated by the bounded-search defense-in-
// depth ceiling before a safety verdict could be reached, and none of the
// records that *did* complete was safe — an incomplete proof is never
// treated as an implicit pass (§22). Emitted only by LifecycleFinding,
// always at point "operation" — lifecycle, like approval itself, gates
// exercise, never delegation (§8, §15).
const (
	ViolationApprovalLifecycleUnsafe   = "approval_lifecycle_unsafe"
	ViolationApprovalLifecycleUnproven = "approval_lifecycle_unproven"
)

// LifecycleStep is one edge of the canonical BFS path from an approval
// record's declared initial state to the first (lexicographically
// smallest) unsafe state its own declared transitions can reach
// (docs/phase-6-plan.md §14.3, §19).
type LifecycleStep struct {
	From  string `json:"from"`
	Event string `json:"event"`
	To    string `json:"to"`
}

// LifecycleFinding is always an operation-level finding (point =
// "operation"). DeclaredApprovers is the full sorted, deduplicated set of
// standing-backed approvers Phase 5's own check already narrowed the
// candidate set to (docs/phase-6-plan.md §16.5) — never the raw approvals[]
// array, and never empty (reaching this finding at all requires a
// non-empty standing set, §16.6). By construction, every member of
// DeclaredApprovers is guaranteed to be unsafe or unproven, never safe,
// whenever this finding is emitted at all. UnsafeApprover is the canonical
// (lexicographically smallest) representative among the unsafe/unproven
// subset (§14.3). UnsafeState/LifecycleTrace are set for
// approval_lifecycle_unsafe and empty for approval_lifecycle_unproven
// (there is no witness state to report when the search itself could not
// complete, §22). When the canonical unsafe state is the automaton's own
// initial state, LifecycleTrace is the empty array [] (zero hops), never
// null and never synthesized as a single degenerate self-referencing step.
type LifecycleFinding struct {
	Violation         string          `json:"violation"`
	Point             string          `json:"point"`
	Actor             string          `json:"actor"`
	Requester         string          `json:"requester"`
	Action            string          `json:"action"`
	Requires          Capability      `json:"requires"`
	DeclaredApprovers []string        `json:"declared_approvers"`
	UnsafeApprover    string          `json:"unsafe_approver"`
	UnsafeState       string          `json:"unsafe_state"`
	LifecycleTrace    []LifecycleStep `json:"lifecycle_trace"`
	Trace             []string        `json:"trace"`
	Reason            string          `json:"reason"`
}

// NewLifecycleFinding constructs a Phase 6 finding with its deterministic
// reason text (§18). violation must be ViolationApprovalLifecycleUnsafe or
// ViolationApprovalLifecycleUnproven. declaredApprovers/lifecycleTrace/
// trace must already be in canonical order.
func NewLifecycleFinding(violation, actor, requester, action string, requires Capability, declaredApprovers []string, unsafeApprover, unsafeState string, lifecycleTrace []LifecycleStep, trace []string) LifecycleFinding {
	var reason string
	switch violation {
	case ViolationApprovalLifecycleUnproven:
		reason = fmt.Sprintf(
			"%s requires %s, which %s validly holds and %s is authorized to request, and %s requires approval; %s independently hold standing, but %s's declared approval lifecycle is too large to prove safe within the configured exploration bound — an unproven approval is never treated as satisfying the requirement",
			action, requires.String(), actor, requester, requires.String(), strings.Join(declaredApprovers, ", "), unsafeApprover,
		)
	default:
		reason = fmt.Sprintf(
			"%s requires %s, which %s validly holds and %s is authorized to request, and %s requires approval; %s independently hold standing, but none of their declared approval lifecycles can be proven to remain in state 'approved' — %s's can reach state '%s' via %s, so it cannot be statically relied upon at time of exercise",
			action, requires.String(), actor, requester, requires.String(), strings.Join(declaredApprovers, ", "), unsafeApprover, unsafeState, renderLifecycleTrace(unsafeState, lifecycleTrace),
		)
	}
	return LifecycleFinding{
		Violation:         violation,
		Point:             PointOperation,
		Actor:             actor,
		Requester:         requester,
		Action:            action,
		Requires:          requires,
		DeclaredApprovers: nonNil(declaredApprovers),
		UnsafeApprover:    unsafeApprover,
		UnsafeState:       unsafeState,
		LifecycleTrace:    nonNilLifecycleTrace(lifecycleTrace),
		Trace:             nonNil(trace),
		Reason:            reason,
	}
}

// renderLifecycleTrace renders a lifecycle trace using arrow-and-bracket
// notation (docs/phase-6-plan.md §19): "approved -[revoke]-> revoked",
// with the event bracket omitted when a hop declared no event label:
// "pending -> approved". When the trace is empty (the unsafe state is the
// automaton's own initial state, reached in zero hops), there is no path
// to render, so the unsafe state is described as reached directly.
func renderLifecycleTrace(unsafeState string, steps []LifecycleStep) string {
	if len(steps) == 0 {
		return unsafeState + " (its own declared initial state)"
	}
	var b strings.Builder
	b.WriteString(steps[0].From)
	for _, s := range steps {
		if s.Event == "" {
			fmt.Fprintf(&b, " -> %s", s.To)
		} else {
			fmt.Fprintf(&b, " -[%s]-> %s", s.Event, s.To)
		}
	}
	return b.String()
}

// joinLifecycleTraceOrNone renders a lifecycle trace for text output.
func joinLifecycleTraceOrNone(steps []LifecycleStep) string {
	if len(steps) == 0 {
		return "(none)"
	}
	var b strings.Builder
	b.WriteString(steps[0].From)
	for _, s := range steps {
		if s.Event == "" {
			fmt.Fprintf(&b, " -> %s", s.To)
		} else {
			fmt.Fprintf(&b, " -[%s]-> %s", s.Event, s.To)
		}
	}
	return b.String()
}

// nonNilLifecycleTrace guarantees the LifecycleTrace field marshals as JSON
// [] rather than null when empty, mirroring nonNilCaps's rule.
func nonNilLifecycleTrace(steps []LifecycleStep) []LifecycleStep {
	if steps == nil {
		return []LifecycleStep{}
	}
	return steps
}
