package report

import (
	"fmt"
	"strings"
)

// ViolationConfusedDeputy is the Phase 3 violation literal
// (docs/phase-3-plan.md §7, §12): an actor validly holds a required
// capability, but the requester it is acting on behalf of was never
// independently granted that capability. Emitted only by
// ConfusedDeputyFinding, never by any Phase 1/2 finding type.
const ViolationConfusedDeputy = "confused_deputy"

// ConfusedDeputyFinding is a Phase 3 operation-level finding: an actor
// legitimately holds the required capability (Phase 1/2's own invariants
// pass), but the requester it is acting on behalf of does not independently
// hold it (docs/phase-3-plan.md §13). point is always "operation" —
// confused-deputy is never an edge-level finding (§8).
type ConfusedDeputyFinding struct {
	Violation             string       `json:"violation"`
	Point                 string       `json:"point"`
	Actor                 string       `json:"actor"`
	Requester             string       `json:"requester"`
	Action                string       `json:"action"`
	Requires              Capability   `json:"requires"`
	ActorHeld             []Capability `json:"actor_held"`
	RequesterHeld         []Capability `json:"requester_held"`
	RequesterBoundTargets []string     `json:"requester_bound_targets"`
	ActorTrace            []string     `json:"actor_trace"`
	RequesterTrace        []string     `json:"requester_trace"`
	Reason                string       `json:"reason"`
}

// NewConfusedDeputyFinding constructs a Phase 3 finding with its
// deterministic reason text (§13), distinguishing in prose only (not via
// the violation literal, §12) whether the requester never held the
// required scope under any target (requesterBoundTargets empty) or held it
// only for a different target (requesterBoundTargets non-empty).
// actorHeld/requesterHeld/requesterBoundTargets/actorTrace/requesterTrace
// must already be in canonical order.
func NewConfusedDeputyFinding(actor, requester, action string, requires Capability, actorHeld, requesterHeld []Capability, requesterBoundTargets, actorTrace, requesterTrace []string) ConfusedDeputyFinding {
	var reason string
	if len(requesterBoundTargets) == 0 {
		reason = fmt.Sprintf(
			"%s requires %s, which %s validly holds, but requester %s has never held %s under any target — %s is being induced to exercise authority %s was never granted",
			action, requires.String(), actor, requester, requires.Scope, actor, requester,
		)
	} else {
		reason = fmt.Sprintf(
			"%s requires %s, which %s validly holds, but requester %s holds %s only for %s, which does not include %s — %s is being induced to exercise authority %s was never granted for this target",
			action, requires.String(), actor, requester, requires.Scope, strings.Join(requesterBoundTargets, ", "), requires.Target, actor, requester,
		)
	}
	return ConfusedDeputyFinding{
		Violation:             ViolationConfusedDeputy,
		Point:                 PointOperation,
		Actor:                 actor,
		Requester:             requester,
		Action:                action,
		Requires:              requires,
		ActorHeld:             nonNilCaps(actorHeld),
		RequesterHeld:         nonNilCaps(requesterHeld),
		RequesterBoundTargets: nonNil(requesterBoundTargets),
		ActorTrace:            nonNil(actorTrace),
		RequesterTrace:        nonNil(requesterTrace),
		Reason:                reason,
	}
}
