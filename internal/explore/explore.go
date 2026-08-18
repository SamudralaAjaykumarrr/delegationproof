// Package explore provides a narrowly-scoped, bounded, deterministic
// breadth-first reachability primitive over a possibly-cyclic,
// possibly-branching labeled directed graph (docs/phase-6-plan.md §21).
//
// It has zero dependency on model, report, loader, or any
// DelegationProof-specific concept: it operates purely on strings and a
// transition list, exactly like internal/graph does for the (strictly
// acyclic) delegation graph. Unlike internal/graph.TopoSort, which is
// documented and implemented as strictly DAG-only, Explore is built to
// tolerate cycles and self-loops by design, because a version-6 approval
// lifecycle automaton is explicitly, legitimately allowed to contain both
// (docs/phase-6-plan.md §8, §11, §30.3).
package explore

import "sort"

// Transition is one labeled directed edge of a small, possibly-cyclic state
// graph — the exploration-domain analogue of graph.Edge, distinct from it
// because this domain permits cycles and internal/graph's TopoSort
// explicitly does not (docs/phase-6-plan.md §1, §21).
type Transition struct {
	From  string
	Event string
	To    string
}

// Result is the outcome of one bounded BFS run from a single source state.
// Reachable is the full visited-state set (including Initial itself).
// Path[q] is the canonical (first-BFS-discovered) sequence of transitions
// from Initial to q, for every q in Reachable except Initial itself
// (Path[Initial] is unset/empty). Truncated is true only if maxStates was
// reached before the BFS frontier was naturally exhausted — i.e. the search
// is incomplete and Reachable/Path must not be treated as final
// (docs/phase-6-plan.md §22).
type Result struct {
	Reachable map[string]bool
	Path      map[string][]Transition
	Truncated bool
}

// Explore runs a deterministic, bounded BFS from initial over the graph
// implied by transitions, visiting at most maxStates distinct states.
//
// Determinism (docs/phase-6-plan.md §25): the BFS frontier is a plain FIFO
// queue (first-discovered, first-expanded); outgoing transitions from any
// one state are considered in ascending lexicographic order of (To, Event),
// computed by sorting a local slice, never by ranging a map — so the result
// (including every Path entry) is a pure function of (initial, transitions),
// independent of Go's map iteration order and independent of the order
// transitions were declared in the input document.
//
// Completeness (docs/phase-6-plan.md §21): the algorithm always explores the
// complete reachable set (up to the bound) — it never stops early upon
// finding the first non-initial state, because the caller's canonical
// unsafe-state selection rule requires knowing the full reachable set, not
// merely a reachable state.
func Explore(initial string, transitions []Transition, maxStates int) Result {
	adj := make(map[string][]Transition, len(transitions))
	for _, t := range transitions {
		adj[t.From] = append(adj[t.From], t)
	}
	for state := range adj {
		list := adj[state]
		sort.Slice(list, func(i, j int) bool {
			if list[i].To != list[j].To {
				return list[i].To < list[j].To
			}
			return list[i].Event < list[j].Event
		})
		adj[state] = list
	}

	reachable := map[string]bool{initial: true}
	path := map[string][]Transition{}
	queue := []string{initial}
	truncated := false

	for len(queue) > 0 {
		if len(reachable) > maxStates {
			truncated = true
			break
		}
		cur := queue[0]
		queue = queue[1:]
		for _, t := range adj[cur] {
			if reachable[t.To] {
				continue
			}
			reachable[t.To] = true
			p := make([]Transition, len(path[cur])+1)
			copy(p, path[cur])
			p[len(p)-1] = t
			path[t.To] = p
			queue = append(queue, t.To)
		}
	}

	return Result{Reachable: reachable, Path: path, Truncated: truncated}
}
