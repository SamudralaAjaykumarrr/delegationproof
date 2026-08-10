// Phase 2 verification: the Context-Binding Preservation invariant
// (docs/phase-2-plan.md §11), generalizing verify.go's Authority
// Non-Amplification algorithm from bare scope strings to capability tuples
// (scope, target), plus the §8 classification/precedence step. Run (v1)
// is unmodified and untouched by anything in this file.
package verify

import (
	"sort"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

// RunV2 evaluates Context-Binding Preservation (and, via the same
// generalized subset check, Authority Non-Amplification) over a
// structurally valid version-2 model, returning the sorted finding result
// (§8, §11, §12).
func RunV2(m *model.ModelV2) report.Result {
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

	incoming := map[string][]model.DelegationV2{}
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
		// internal/loader guarantees acyclicity before RunV2 is ever called.
		panic("verify: RunV2 invoked on a cyclic model")
	}

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

	for _, op := range m.Operations {
		held := da[op.Actor]
		requires := model.Capability{Scope: op.Requires, Target: op.Target}
		if !containsCap(held, requires) {
			violation, boundTargets := classifyOne(requires.Scope, held)
			trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			findings = append(findings, report.NewCapabilityOperationFinding(
				violation, op.Actor, op.Action,
				report.Capability{Scope: requires.Scope, Target: requires.Target},
				toReportCaps(held), boundTargets, trace,
			))
		}
	}

	report.Sort(findings)
	return report.Result{Findings: findings}
}

// classifyOne implements §8's classify() for a single required capability
// (the operation-level case, which always covers exactly one capability).
func classifyOne(scope string, held []model.Capability) (violation string, boundTargets []string) {
	targets := heldTargetsForScope(scope, held)
	if len(targets) == 0 {
		return report.ViolationAuthorityAmplification, nil
	}
	return report.ViolationContextBinding, targets
}

// classifyEdge implements §8's edge-level precedence rule: one finding
// covers the whole excess set. If any capability in excess has scope never
// held by the delegator under any target, the finding is
// authority_amplification — the more foundational problem takes
// precedence and is never masked by a co-occurring binding issue. Only
// when every capability in excess is a pure context mismatch is the
// finding context_binding_violation, with bound_targets set to the sorted,
// deduplicated union of held targets across all excess scopes.
func classifyEdge(excess, delegatorDA []model.Capability) (violation string, boundTargets []string) {
	boundSet := map[string]bool{}
	anyAmplification := false
	for _, c := range excess {
		targets := heldTargetsForScope(c.Scope, delegatorDA)
		if len(targets) == 0 {
			anyAmplification = true
			continue
		}
		for _, t := range targets {
			boundSet[t] = true
		}
	}
	if anyAmplification {
		return report.ViolationAuthorityAmplification, nil
	}
	out := make([]string, 0, len(boundSet))
	for t := range boundSet {
		out = append(out, t)
	}
	sort.Strings(out)
	return report.ViolationContextBinding, out
}

// heldTargetsForScope returns the sorted, deduplicated set of targets
// scope is held under within caps — §8's heldTargetsForScope(s).
func heldTargetsForScope(scope string, caps []model.Capability) []string {
	set := map[string]bool{}
	for _, c := range caps {
		if c.Scope == scope {
			set[c.Target] = true
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func isSubsetCap(a, b []model.Capability) bool {
	set := map[model.Capability]bool{}
	for _, c := range b {
		set[c] = true
	}
	for _, c := range a {
		if !set[c] {
			return false
		}
	}
	return true
}

func subtractCap(a, b []model.Capability) []model.Capability {
	set := map[model.Capability]bool{}
	for _, c := range b {
		set[c] = true
	}
	var out []model.Capability
	for _, c := range a {
		if !set[c] {
			out = append(out, c)
		}
	}
	return out
}

func containsCap(set []model.Capability, target model.Capability) bool {
	for _, c := range set {
		if c == target {
			return true
		}
	}
	return false
}

func canonicalizeCaps(caps []model.Capability) []model.Capability {
	set := map[model.Capability]bool{}
	for _, c := range caps {
		set[c] = true
	}
	return canonicalizeCapSet(set)
}

func canonicalizeCapSet(set map[model.Capability]bool) []model.Capability {
	out := make([]model.Capability, 0, len(set))
	for c := range set {
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

func toReportCaps(caps []model.Capability) []report.Capability {
	out := make([]report.Capability, len(caps))
	for i, c := range caps {
		out[i] = report.Capability{Scope: c.Scope, Target: c.Target}
	}
	return out
}
