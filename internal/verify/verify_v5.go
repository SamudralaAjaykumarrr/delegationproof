// Phase 5 verification: Approval Preservation (docs/phase-5-plan.md §8, §9,
// §10, §11, §12, §17), layered on top of verify_v2.go's Context-Binding
// Preservation / Authority Non-Amplification algorithm, verify_v3.go's
// Requester Authorization Preservation, and verify_v4.go's Delegation Depth
// Preservation, with no changes to any of them. Run, RunV2, RunV3, and
// RunV4 are unmodified and untouched by anything in this file. This file
// reuses verify_v2.go's/verify_v3.go's unexported helpers (isSubsetCap,
// subtractCap, canonicalizeCaps, containsCap, heldTargetsForScope,
// classifyOne, classifyEdge, toReportCaps) directly against
// flattenApproval's output, same package, no duplication.
package verify

import (
	"sort"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

// authState is Phase 5's per-node, per-capability derived-authority value
// (docs/phase-5-plan.md §9): remaining/configuredMax are unchanged from
// Phase 4's depthState. requiresApproval is Phase 5's new dimension — does
// exercising this held instance of the capability require a valid
// approval? Unlike configuredMax (always carried forward unchanged from
// whichever single path won the remaining-maximization contest),
// requiresApproval is aggregated independently across every valid
// delivering path via logical OR (§10.1), not tied to which path wins
// remaining.
type authState struct {
	remaining        int
	configuredMax    int
	requiresApproval bool
}

// RunV5 evaluates Approval Preservation (and, via the same generalized
// subset/flattenApproval machinery, Authority Non-Amplification,
// Context-Binding Preservation, Requester Authorization Preservation, and
// Delegation Depth Preservation) over a structurally valid version-5 model,
// returning the sorted finding result (§8, §11, §12, §17).
func RunV5(m *model.ModelV5) report.Result {
	nodeIDs := make([]string, 0, len(m.Principals)+len(m.Agents))
	principalIDs := make([]string, 0, len(m.Principals))
	declared := map[string][]model.RootCapabilityV5{}
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

	incoming := map[string][]model.DelegationV5{}
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
		// internal/loader guarantees acyclicity before RunV5 is ever called.
		panic("verify: RunV5 invoked on a cyclic model")
	}

	// da[n] maps a held capability to its full authState (§9).
	// Presence-only consumers use flattenApproval(da[n]) — a sorted
	// []model.Capability key view — passed unmodified into every Phase 1-4
	// helper.
	da := map[string]map[model.Capability]authState{}
	var validEdges []graph.Edge
	var findings []interface{}

	for _, n := range order {
		if isPrincipal[n] {
			states := map[model.Capability]authState{}
			for _, rc := range declared[n] {
				c := model.Capability{Scope: rc.Scope, Target: rc.Target}
				states[c] = authState{
					remaining:        *rc.MaxDelegationDepth,
					configuredMax:    *rc.MaxDelegationDepth,
					requiresApproval: *rc.RequiresApproval,
				}
			}
			da[n] = states
			continue
		}

		states := map[model.Capability]authState{}
		for _, e := range incoming[n] {
			delegatorStates := da[e.Delegator]
			delegatorFlat := flattenApproval(delegatorStates)

			if !isSubsetCap(e.Authority, delegatorFlat) {
				// Presence/binding strict distrust (unchanged, Phase 1-4):
				// the whole edge is invalid, and takes precedence over any
				// depth failure (§11, §12). Approval never participates in
				// edge-level evaluation at all (§4.2).
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

			// Presence/binding passed. Depth strict distrust (unchanged,
			// Phase 4, §11): if any capability in the edge has remaining ==
			// 0 at the delegator, the whole edge is invalid for depth —
			// whole-edge poisoning, matching existing precedent exactly.
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
			// capability's state decrements remaining by one hop, adopted
			// only on strict improvement for remaining/configuredMax
			// (§10.1's max-aggregation, unchanged from Phase 4), while
			// requiresApproval is aggregated independently via logical OR
			// across every valid delivering path (§10.1, new).
			validEdges = append(validEdges, graph.Edge{From: e.Delegator, To: e.Delegatee})
			for _, c := range e.Authority {
				st := delegatorStates[c]
				candidate := authState{
					remaining:        st.remaining - 1,
					configuredMax:    st.configuredMax,
					requiresApproval: st.requiresApproval,
				}
				if existing, has := states[c]; !has {
					states[c] = candidate
				} else {
					remaining, configuredMax := existing.remaining, existing.configuredMax
					if candidate.remaining > existing.remaining {
						remaining, configuredMax = candidate.remaining, candidate.configuredMax
					}
					states[c] = authState{
						remaining:        remaining,
						configuredMax:    configuredMax,
						requiresApproval: existing.requiresApproval || candidate.requiresApproval,
					}
				}
			}
		}
		da[n] = states
	}

	// Index approvals once, before operation evaluation (§10.2, §12, §17):
	// declaredApprovers groups approvals[] by Capability, sorted and
	// deduplicated by ascending approver id. standingApprovers filters each
	// bucket to only those approvers who independently hold the capability,
	// computed once per distinct capability rather than once per operation
	// — an operation-evaluation step is then a plain O(1) map lookup.
	declaredApprovers := map[model.Capability][]string{}
	approverSets := map[model.Capability]map[string]bool{}
	for _, a := range m.Approvals {
		c := model.Capability{Scope: a.Scope, Target: a.Target}
		if approverSets[c] == nil {
			approverSets[c] = map[string]bool{}
		}
		approverSets[c][a.Approver] = true
	}
	for c, set := range approverSets {
		list := make([]string, 0, len(set))
		for approver := range set {
			list = append(list, approver)
		}
		sort.Strings(list)
		declaredApprovers[c] = list
	}
	standingApprovers := map[model.Capability][]string{}
	for c, approvers := range declaredApprovers {
		var standing []string
		for _, approver := range approvers {
			if containsCap(flattenApproval(da[approver]), c) {
				standing = append(standing, approver)
			}
		}
		standingApprovers[c] = standing
	}

	// Operation evaluation: extended four-step precedence (§12). Steps 1-2
	// are unchanged from docs/phase-3-plan.md §8 — depth never
	// participates, and approval is checked strictly last, only once
	// presence, binding, and requester standing are all already
	// established.
	for _, op := range m.Operations {
		actorFlat := flattenApproval(da[op.Actor])
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

		requesterFlat := flattenApproval(da[op.Requester])
		if !containsCap(requesterFlat, requires) {
			boundTargets := heldTargetsForScope(requires.Scope, requesterFlat)
			actorTrace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			requesterTrace := graph.CanonicalTrace(principalIDs, validEdges, op.Requester)
			findings = append(findings, report.NewConfusedDeputyFinding(
				op.Actor, op.Requester, op.Action,
				report.Capability{Scope: requires.Scope, Target: requires.Target},
				toReportCaps(actorFlat), toReportCaps(requesterFlat),
				boundTargets, actorTrace, requesterTrace,
			))
			continue
		}

		// Step 3 (new, Phase 5): approval, reached only once presence,
		// binding, and requester standing are all already established.
		actorState := da[op.Actor][requires] // exists: step 1 already confirmed presence
		if !actorState.requiresApproval {
			continue // ALLOW, no finding — vacuously satisfied
		}

		reportCap := report.Capability{Scope: requires.Scope, Target: requires.Target}
		declaredForCap := declaredApprovers[requires]
		if len(declaredForCap) == 0 {
			trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			findings = append(findings, report.NewApprovalFinding(
				report.ViolationApprovalMissing, op.Actor, op.Requester, op.Action,
				reportCap, []string{}, trace,
			))
			continue
		}

		standingForCap := standingApprovers[requires]
		if len(standingForCap) == 0 {
			trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			findings = append(findings, report.NewApprovalFinding(
				report.ViolationApprovalUnauthorized, op.Actor, op.Requester, op.Action,
				reportCap, declaredForCap, trace,
			))
			continue
		}

		// Step 4: every check passed. ALLOW, no finding.
	}

	report.Sort(findings)
	return report.Result{Findings: findings}
}

// flattenApproval returns the sorted, deduplicated key set of an
// approval-aware da entry as a plain []model.Capability — a presence-only
// view over which every Phase 1-4 helper (isSubsetCap, classifyEdge,
// classifyOne, heldTargetsForScope, containsCap) operates completely
// unmodified (docs/phase-5-plan.md §9). Kept as a distinctly-named function
// rather than an overload of verify_v4.go's flatten, since Go does not
// support overloading by parameter type and the two da maps have different
// value types.
func flattenApproval(states map[model.Capability]authState) []model.Capability {
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
