// Phase 6 verification: Temporal Approval Preservation (docs/phase-6-plan.md
// §9, §16, §21, §26), layered on top of verify_v5.go's Approval Preservation
// algorithm (which is itself layered on Context-Binding Preservation /
// Authority Non-Amplification, Requester Authorization Preservation, and
// Delegation Depth Preservation), with no changes to any of them. Run,
// RunV2, RunV3, RunV4, and RunV5 are unmodified and untouched by anything in
// this file. This file reuses verify_v5.go's unexported authState type and
// flattenApproval function, and verify_v2.go's/verify_v3.go's unexported
// helpers (isSubsetCap, subtractCap, canonicalizeCaps, containsCap,
// heldTargetsForScope, classifyOne, classifyEdge, toReportCaps) directly,
// same package, no duplication. internal/explore's bounded BFS is the one
// new piece of machinery this file introduces.
package verify

import (
	"sort"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/explore"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

// lifecycleOutcome classifies one approval record's own declared lifecycle
// (docs/phase-6-plan.md §26). A record with no declared lifecycle is always
// lifecycleSafeOutcome, unconditionally, with no Explore call at all (§5.2,
// §9).
type lifecycleOutcome int

const (
	lifecycleSafeOutcome lifecycleOutcome = iota
	lifecycleUnsafeOutcome
	lifecycleUnprovenOutcome
)

// lifecycleResult is the cached, precomputed outcome of exploring one
// approval record's lifecycle (or the trivial "no lifecycle declared"
// case). unsafeState/path are set only for lifecycleUnsafeOutcome (§18,
// §22).
type lifecycleResult struct {
	outcome     lifecycleOutcome
	unsafeState string
	path        []explore.Transition
}

// lifecycleKey identifies one approval record for the purposes of the
// lifecycle outcome cache. Validate-time uniqueness of the
// (approver, scope, target) triple (docs/phase-6-plan.md §20,
// duplicate_approval) guarantees this key is distinct per declared
// approval record, so the cache holds exactly one outcome per key.
type lifecycleKey struct {
	approver string
	cap      model.Capability
}

// RunV6 evaluates Temporal Approval Preservation (and, via the same
// generalized subset/flattenApproval machinery, Authority Non-Amplification,
// Context-Binding Preservation, Requester Authorization Preservation,
// Delegation Depth Preservation, and Approval Preservation) over a
// structurally valid version-6 model, returning the sorted finding result
// (§9, §16, §21, §26).
func RunV6(m *model.ModelV6) report.Result {
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

	incoming := map[string][]model.DelegationV6{}
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
		// internal/loader guarantees acyclicity before RunV6 is ever called.
		panic("verify: RunV6 invoked on a cyclic model")
	}

	// Step 1: build the graph and compute da, byte-for-byte the same steps
	// RunV5 already performs, over the unchanged authState{remaining,
	// configuredMax, requiresApproval} (§26).
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

	// Step 2: index approvals once, unchanged from RunV5 (§17).
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

	// Step 3: index lifecycle safety (new, §26). One independent, complete
	// Explore run per distinct lifecycle-bearing approval record, cached by
	// (approver, capability) — never composed across records (§3).
	outcomeCache := map[lifecycleKey]lifecycleResult{}
	for _, a := range m.Approvals {
		key := lifecycleKey{approver: a.Approver, cap: model.Capability{Scope: a.Scope, Target: a.Target}}
		if a.Lifecycle == nil {
			outcomeCache[key] = lifecycleResult{outcome: lifecycleSafeOutcome}
			continue
		}
		res := explore.Explore(a.Lifecycle.Initial, transitionsOf(a.Lifecycle), limits.MaxExplorationStatesPerLifecycle)
		outcome, unsafeState, path := classifyLifecycle(res)
		outcomeCache[key] = lifecycleResult{outcome: outcome, unsafeState: unsafeState, path: path}
	}

	// Step 4: operation evaluation, extended five-step precedence (§16.6).
	// Steps 1-3 are byte-identical to RunV5's own steps 1-3; step 4 is new,
	// reached only once presence, binding, requester standing, and
	// approval standing (a non-empty standingForCap) are all already
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

		// Step 4 (new, Phase 6): lifecycle safety, only reached once
		// presence, binding, requester standing, and approval standing are
		// all already established. standingForCap is already
		// ascending-lexicographic sorted (§17), so filtering it into
		// safe/unsafe/unproven preserves that order — unsafe[0]/
		// unproven[0] are therefore already the canonical
		// (lexicographically smallest) representative (§14.3), with no
		// additional sort needed.
		var safe, unsafe, unproven []string
		for _, approver := range standingForCap {
			res := outcomeCache[lifecycleKey{approver: approver, cap: requires}]
			switch res.outcome {
			case lifecycleSafeOutcome:
				safe = append(safe, approver)
			case lifecycleUnsafeOutcome:
				unsafe = append(unsafe, approver)
			default:
				unproven = append(unproven, approver)
			}
		}
		if len(safe) > 0 {
			continue // ALLOW, no finding
		}

		trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
		if len(unsafe) > 0 {
			winner := unsafe[0]
			res := outcomeCache[lifecycleKey{approver: winner, cap: requires}]
			findings = append(findings, report.NewLifecycleFinding(
				report.ViolationApprovalLifecycleUnsafe, op.Actor, op.Requester, op.Action,
				reportCap, standingForCap, winner, res.unsafeState, toLifecycleSteps(res.path), trace,
			))
			continue
		}

		// unproven is non-empty: standingForCap was non-empty and safe/
		// unsafe are both empty — an incomplete proof is never treated as
		// an implicit pass (§22).
		winner := unproven[0]
		findings = append(findings, report.NewLifecycleFinding(
			report.ViolationApprovalLifecycleUnproven, op.Actor, op.Requester, op.Action,
			reportCap, standingForCap, winner, "", nil, trace,
		))
	}

	report.Sort(findings)
	return report.Result{Findings: findings}
}

// classifyLifecycle implements §26's classify(): a truncated exploration is
// always unproven, regardless of what partial reachable set it produced
// (§22's "partial results: not emitted" rule — Reachable/Path are never
// consulted beyond the Truncated flag itself when truncation occurred). A
// complete exploration is safe iff its reachable set is exactly
// {"approved"}; otherwise it is unsafe, and the canonical unsafe state is
// the lexicographically smallest non-"approved" member of the reachable set
// (§14.3) — the one place this file ranges a map, made safe by the
// unconditional sort immediately following it, before any element is
// consulted (§25).
func classifyLifecycle(res explore.Result) (outcome lifecycleOutcome, unsafeState string, path []explore.Transition) {
	if res.Truncated {
		return lifecycleUnprovenOutcome, "", nil
	}
	var unsafeStates []string
	for q := range res.Reachable {
		if q != "approved" {
			unsafeStates = append(unsafeStates, q)
		}
	}
	if len(unsafeStates) == 0 {
		return lifecycleSafeOutcome, "", nil
	}
	sort.Strings(unsafeStates)
	first := unsafeStates[0]
	return lifecycleUnsafeOutcome, first, res.Path[first]
}

// transitionsOf converts one approval record's declared lifecycle
// transitions into internal/explore's domain-agnostic Transition type.
func transitionsOf(lc *model.Lifecycle) []explore.Transition {
	out := make([]explore.Transition, len(lc.Transitions))
	for i, t := range lc.Transitions {
		out[i] = explore.Transition{From: t.From, Event: t.Event, To: t.To}
	}
	return out
}

// toLifecycleSteps converts internal/explore's Transition path into the
// report package's LifecycleStep finding payload.
func toLifecycleSteps(path []explore.Transition) []report.LifecycleStep {
	if len(path) == 0 {
		return nil
	}
	out := make([]report.LifecycleStep, len(path))
	for i, t := range path {
		out[i] = report.LifecycleStep{From: t.From, Event: t.Event, To: t.To}
	}
	return out
}
