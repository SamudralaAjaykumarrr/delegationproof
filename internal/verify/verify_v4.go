// Phase 4 verification: Delegation Depth Preservation (docs/phase-4-plan.md
// §8, §11, §16), layered on top of verify_v2.go's Context-Binding
// Preservation / Authority Non-Amplification algorithm and verify_v3.go's
// Requester Authorization Preservation, with no changes to either. Run,
// RunV2, and RunV3 are unmodified and untouched by anything in this file.
// This file reuses verify_v2.go's unexported helpers (isSubsetCap,
// subtractCap, canonicalizeCaps, containsCap, heldTargetsForScope,
// classifyOne, classifyEdge, toReportCaps) directly against flatten's
// output, same package, no duplication.
package verify

import (
	"sort"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

// depthState is Phase 4's per-node, per-capability derived-authority value
// (docs/phase-4-plan.md §9): remaining is the best (maximum) remaining
// redelegation budget reachable at this node for this capability across all
// valid delivering paths; configuredMax is the root grant's originally
// declared max_delegation_depth, carried through unchanged so a
// delegation_depth_violation finding can report it without re-walking
// provenance.
type depthState struct {
	remaining     int
	configuredMax int
}

// RunV4 evaluates Delegation Depth Preservation (and, via the same
// generalized subset/flatten machinery, Authority Non-Amplification,
// Context-Binding Preservation, and Requester Authorization Preservation)
// over a structurally valid version-4 model, returning the sorted finding
// result (§8, §11, §12, §16).
func RunV4(m *model.ModelV4) report.Result {
	nodeIDs := make([]string, 0, len(m.Principals)+len(m.Agents))
	principalIDs := make([]string, 0, len(m.Principals))
	declared := map[string][]model.RootCapability{}
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

	incoming := map[string][]model.DelegationV4{}
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
		// internal/loader guarantees acyclicity before RunV4 is ever called.
		panic("verify: RunV4 invoked on a cyclic model")
	}

	// da[n] maps a held capability to its depth state (§9). Presence-only
	// consumers use flatten(da[n]) — a sorted []model.Capability key view —
	// passed unmodified into every Phase 1-3 helper.
	da := map[string]map[model.Capability]depthState{}
	var validEdges []graph.Edge
	var findings []interface{}

	for _, n := range order {
		if isPrincipal[n] {
			states := map[model.Capability]depthState{}
			for _, rc := range declared[n] {
				c := model.Capability{Scope: rc.Scope, Target: rc.Target}
				states[c] = depthState{remaining: *rc.MaxDelegationDepth, configuredMax: *rc.MaxDelegationDepth}
			}
			da[n] = states
			continue
		}

		states := map[model.Capability]depthState{}
		for _, e := range incoming[n] {
			delegatorStates := da[e.Delegator]
			delegatorFlat := flatten(delegatorStates)

			if !isSubsetCap(e.Authority, delegatorFlat) {
				// Presence/binding strict distrust (unchanged, Phase 1-3):
				// the whole edge is invalid, and takes precedence over any
				// depth failure (§11, §12).
				excess := subtractCap(e.Authority, delegatorFlat)
				violation, boundTargets := classifyEdge(excess, delegatorFlat)
				trace := append(graph.CanonicalTrace(principalIDs, validEdges, e.Delegator), e.Delegatee)
				findings = append(findings, report.NewCapabilityEdgeFinding(
					violation, e.Delegator, e.Delegatee,
					toReportCaps(canonicalizeCaps(e.Authority)), toReportCaps(canonicalizeCaps(excess)),
					boundTargets, trace,
				))
				continue
			}

			// Presence/binding passed. Depth strict distrust (new, Phase 4,
			// §11): if any capability in the edge has remaining == 0 at the
			// delegator, the whole edge is invalid for depth — whole-edge
			// poisoning, matching existing precedent exactly.
			var depthExcess []report.DepthExcess
			for _, c := range canonicalizeCaps(e.Authority) {
				st := delegatorStates[c]
				if st.remaining < 1 {
					depthExcess = append(depthExcess, report.DepthExcess{
						Scope: c.Scope, Target: c.Target,
						ConfiguredMax:  st.configuredMax,
						RemainingDepth: st.remaining,
					})
				}
			}
			if len(depthExcess) > 0 {
				trace := append(graph.CanonicalTrace(principalIDs, validEdges, e.Delegator), e.Delegatee)
				findings = append(findings, report.NewDelegationDepthFinding(
					e.Delegator, e.Delegatee,
					toReportCaps(canonicalizeCaps(e.Authority)), depthExcess, trace,
				))
				continue
			}

			// Edge fully valid: presence, binding, and depth all pass.
			// Included in validEdges (used by CanonicalTrace, §15) and each
			// capability's depth state decrements by one hop, adopted only
			// on strict improvement (§10's tie-break: incoming edges are
			// already visited in ascending lexicographic delegator-id
			// order, so first-strictly-best wins with zero new sorting).
			validEdges = append(validEdges, graph.Edge{From: e.Delegator, To: e.Delegatee})
			for _, c := range e.Authority {
				st := delegatorStates[c]
				candidate := depthState{remaining: st.remaining - 1, configuredMax: st.configuredMax}
				if existing, has := states[c]; !has || candidate.remaining > existing.remaining {
					states[c] = candidate
				}
			}
		}
		da[n] = states
	}

	// Operation evaluation: unchanged §12/§13 three-step precedence from
	// docs/phase-3-plan.md §8 — depth never participates. A node may use a
	// capability at any remaining budget >= 0; flatten() never filters by
	// remaining budget.
	for _, op := range m.Operations {
		actorFlat := flatten(da[op.Actor])
		requires := model.Capability{Scope: op.Requires, Target: op.Target}

		if !containsCap(actorFlat, requires) {
			violation, boundTargets := classifyOne(requires.Scope, actorFlat)
			trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			findings = append(findings, report.NewCapabilityOperationFinding(
				violation, op.Actor, op.Action,
				report.Capability{Scope: requires.Scope, Target: requires.Target},
				toReportCaps(actorFlat), boundTargets, trace,
			))
			continue
		}

		requesterFlat := flatten(da[op.Requester])
		if containsCap(requesterFlat, requires) {
			continue // ALLOW, no finding
		}

		boundTargets := heldTargetsForScope(requires.Scope, requesterFlat)
		actorTrace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
		requesterTrace := graph.CanonicalTrace(principalIDs, validEdges, op.Requester)
		findings = append(findings, report.NewConfusedDeputyFinding(
			op.Actor, op.Requester, op.Action,
			report.Capability{Scope: requires.Scope, Target: requires.Target},
			toReportCaps(actorFlat), toReportCaps(requesterFlat),
			boundTargets, actorTrace, requesterTrace,
		))
	}

	report.Sort(findings)
	return report.Result{Findings: findings}
}

// flatten returns the sorted, deduplicated key set of a depth-aware da
// entry as a plain []model.Capability — a presence-only view over which
// every Phase 1-3 helper (isSubsetCap, classifyEdge, classifyOne,
// heldTargetsForScope, containsCap) operates completely unmodified
// (docs/phase-4-plan.md §9). It never filters by remaining budget: holding
// a capability at remaining == 0 still makes it present in this view,
// because usability and delegability are independently-gated properties of
// the same held capability (§4.2, §11).
func flatten(states map[model.Capability]depthState) []model.Capability {
	out := make([]model.Capability, 0, len(states))
	for c := range states {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Target < out[j].Target
	})
	return out
}
