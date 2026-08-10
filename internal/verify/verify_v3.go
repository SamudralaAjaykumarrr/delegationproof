// Phase 3 verification: Requester Authorization Preservation
// (docs/phase-3-plan.md §7, §11, §12), layered on top of verify_v2.go's
// Context-Binding Preservation / Authority Non-Amplification algorithm
// with no changes to it. RunV2 (and Run) are unmodified and untouched by
// anything in this file. This file reuses verify_v2.go's unexported
// helpers (isSubsetCap, subtractCap, canonicalizeCaps, canonicalizeCapSet,
// containsCap, heldTargetsForScope, classifyOne, classifyEdge,
// toReportCaps) directly, same package, no duplication.
package verify

import (
	"sort"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

// RunV3 evaluates Requester Authorization Preservation (and, via the same
// generalized subset check, Authority Non-Amplification and Context-Binding
// Preservation) over a structurally valid version-3 model, returning the
// sorted finding result (§8, §11, §12).
func RunV3(m *model.ModelV3) report.Result {
	nodeIDs := make([]string, 0, len(m.Principals)+len(m.Agents))
	principalIDs := make([]string, 0, len(m.Principals))
	declared := map[string][]model.Capability{}
	isPrincipal := map[string]bool{}

	for _, p := range m.Principals {
		nodeIDs = append(nodeIDs, p.ID)
		principalIDs = append(principalIDs, p.ID)
		declared[p.ID] = p.Authority
		isPrincipal[p.ID] = true
	}
	for _, a := range m.Agents {
		nodeIDs = append(nodeIDs, a.ID)
	}

	incoming := map[string][]model.DelegationV3{}
	allEdges := make([]graph.Edge, 0, len(m.Delegations))
	for _, d := range m.Delegations {
		incoming[d.Delegatee] = append(incoming[d.Delegatee], d)
		allEdges = append(allEdges, graph.Edge{From: d.Delegator, To: d.Delegatee})
	}
	for k := range incoming {
		sort.SliceStable(incoming[k], func(i, j int) bool {
			return incoming[k][i].Delegator < incoming[k][j].Delegator
		})
	}

	order, _, ok := graph.TopoSort(nodeIDs, allEdges)
	if !ok {
		// internal/loader guarantees acyclicity before RunV3 is ever called.
		panic("verify: RunV3 invoked on a cyclic model")
	}

	// Step 1: build the graph and compute da for EVERY node — identical to
	// RunV2 (§11 step 1). This already produces DA(requester) for every
	// possible requester before any operation is evaluated, whether or not
	// that node is ever referenced as an operation's actor.
	da := map[string][]model.Capability{}
	var validEdges []graph.Edge
	var findings []interface{}

	for _, n := range order {
		if isPrincipal[n] {
			da[n] = canonicalizeCaps(declared[n])
			continue
		}
		held := map[model.Capability]bool{}
		for _, e := range incoming[n] {
			delegatorDA := da[e.Delegator]
			if isSubsetCap(e.Authority, delegatorDA) {
				for _, c := range e.Authority {
					held[c] = true
				}
				validEdges = append(validEdges, graph.Edge{From: e.Delegator, To: e.Delegatee})
			} else {
				excess := subtractCap(e.Authority, delegatorDA)
				violation, boundTargets := classifyEdge(excess, delegatorDA)
				trace := append(graph.CanonicalTrace(principalIDs, validEdges, e.Delegator), e.Delegatee)
				findings = append(findings, report.NewCapabilityEdgeFinding(
					violation, e.Delegator, e.Delegatee,
					toReportCaps(canonicalizeCaps(e.Authority)), toReportCaps(canonicalizeCaps(excess)),
					boundTargets, trace,
				))
			}
		}
		da[n] = canonicalizeCapSet(held)
	}

	// Step 2/3: operation evaluation, per §8's precedence algorithm.
	for _, op := range m.Operations {
		actorHeld := da[op.Actor]
		requires := model.Capability{Scope: op.Requires, Target: op.Target}

		// Step 1 of §8 — unchanged Phase 1/2 check, unchanged
		// classification. If the actor itself does not validly hold the
		// capability, that is the applicable diagnosis; requester is not
		// evaluated at all.
		if !containsCap(actorHeld, requires) {
			violation, boundTargets := classifyOne(requires.Scope, actorHeld)
			trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			findings = append(findings, report.NewCapabilityOperationFinding(
				violation, op.Actor, op.Action,
				report.Capability{Scope: requires.Scope, Target: requires.Target},
				toReportCaps(actorHeld), boundTargets, trace,
			))
			continue
		}

		// Step 2 of §8 — new Phase 3 check, only reached once the actor
		// side has already passed. DA(requester) was already computed in
		// the pass above, independent of DA(actor) — no new graph work.
		requesterHeld := da[op.Requester]
		if containsCap(requesterHeld, requires) {
			continue // ALLOW, no finding
		}

		// Step 3 of §8 — actor legitimate, requester lacks standing:
		// confused_deputy, always (§12: not sub-classified into
		// amplification/binding flavors).
		boundTargets := heldTargetsForScope(requires.Scope, requesterHeld)
		actorTrace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
		requesterTrace := graph.CanonicalTrace(principalIDs, validEdges, op.Requester)
		findings = append(findings, report.NewConfusedDeputyFinding(
			op.Actor, op.Requester, op.Action,
			report.Capability{Scope: requires.Scope, Target: requires.Target},
			toReportCaps(actorHeld), toReportCaps(requesterHeld),
			boundTargets, actorTrace, requesterTrace,
		))
	}

	report.Sort(findings)
	return report.Result{Findings: findings}
}
