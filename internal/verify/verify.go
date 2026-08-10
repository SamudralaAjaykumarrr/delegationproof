// Package verify implements the Phase 1 Authority Non-Amplification
// algorithm (docs/phase-1-plan.md §8): topological Derived Authority
// computation, edge/operation evaluation, and deterministic finding
// assembly. Run assumes its input has already passed structural validation
// (internal/loader) and is therefore acyclic.
package verify

import (
	"sort"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

// Run evaluates Authority Non-Amplification over a structurally valid
// model and returns the sorted finding result (§8, §9).
func Run(m *model.Model) report.Result {
	nodeIDs := make([]string, 0, len(m.Principals)+len(m.Agents))
	principalIDs := make([]string, 0, len(m.Principals))
	declared := map[string][]string{}
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

	incoming := map[string][]model.Delegation{}
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
		// internal/loader guarantees acyclicity before Run is ever called.
		panic("verify: Run invoked on a cyclic model")
	}

	da := map[string][]string{}
	var validEdges []graph.Edge
	var findings []interface{}

	for _, n := range order {
		if isPrincipal[n] {
			da[n] = canonicalize(declared[n])
			continue
		}
		held := map[string]bool{}
		for _, e := range incoming[n] {
			delegatorDA := da[e.Delegator]
			if isSubset(e.Authority, delegatorDA) {
				for _, s := range e.Authority {
					held[s] = true
				}
				validEdges = append(validEdges, graph.Edge{From: e.Delegator, To: e.Delegatee})
			} else {
				excess := subtract(e.Authority, delegatorDA)
				trace := append(graph.CanonicalTrace(principalIDs, validEdges, e.Delegator), e.Delegatee)
				findings = append(findings, report.NewEdgeFinding(
					e.Delegator, e.Delegatee, canonicalize(e.Authority), canonicalize(excess), trace,
				))
			}
		}
		da[n] = canonicalizeSet(held)
	}

	for _, op := range m.Operations {
		held := da[op.Actor]
		if !contains(held, op.Requires) {
			trace := append(graph.CanonicalTrace(principalIDs, validEdges, op.Actor), op.Action)
			findings = append(findings, report.NewOperationFinding(op.Actor, op.Action, op.Requires, held, trace))
		}
	}

	report.Sort(findings)
	return report.Result{Findings: findings}
}

func isSubset(a, b []string) bool {
	set := map[string]bool{}
	for _, s := range b {
		set[s] = true
	}
	for _, s := range a {
		if !set[s] {
			return false
		}
	}
	return true
}

func subtract(a, b []string) []string {
	set := map[string]bool{}
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if !set[s] {
			out = append(out, s)
		}
	}
	return out
}

func contains(set []string, target string) bool {
	for _, s := range set {
		if s == target {
			return true
		}
	}
	return false
}

func canonicalize(scopes []string) []string {
	set := map[string]bool{}
	for _, s := range scopes {
		set[s] = true
	}
	return canonicalizeSet(set)
}

func canonicalizeSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
