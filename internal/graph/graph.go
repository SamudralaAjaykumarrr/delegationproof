// Package graph provides the deterministic graph primitives Phase 1 needs:
// Kahn's-algorithm topological sort with cycle detection (docs/phase-1-plan.md
// §8), and the canonical BFS delegation-trace helper (§8.1). Every
// tie-break is by ascending lexicographic node id, so results never depend
// on map iteration order.
package graph

import (
	"container/heap"
	"sort"
)

// stringHeap is a min-heap of strings, used to pop the lexicographically
// smallest ready node in O(log n) instead of re-sorting on every pop.
type stringHeap []string

func (h stringHeap) Len() int            { return len(h) }
func (h stringHeap) Less(i, j int) bool  { return h[i] < h[j] }
func (h stringHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *stringHeap) Push(x interface{}) { *h = append(*h, x.(string)) }
func (h *stringHeap) Pop() interface{} {
	old := *h
	n := len(old)
	v := old[n-1]
	*h = old[:n-1]
	return v
}

// Edge is a directed edge From -> To.
type Edge struct {
	From string
	To   string
}

// TopoSort runs Kahn's algorithm over nodeIDs/edges, breaking ties between
// simultaneously-ready nodes by ascending lexicographic id. If the graph is
// acyclic, it returns the topological order with ok=true. If not, it
// returns ok=false and a canonical cycle path: the lexicographically
// smallest node involved in a cycle, followed by the rest of that cycle in
// edge-forward order, not closed (i.e. the last element has an edge back to
// the first).
func TopoSort(nodeIDs []string, edges []Edge) (order []string, cycle []string, ok bool) {
	indegree := make(map[string]int, len(nodeIDs))
	adj := make(map[string][]string, len(nodeIDs))
	for _, id := range nodeIDs {
		indegree[id] = 0
	}
	for _, e := range edges {
		indegree[e.To]++
		adj[e.From] = append(adj[e.From], e.To)
	}
	for k := range adj {
		sort.Strings(adj[k])
	}

	ready := &stringHeap{}
	for _, id := range nodeIDs {
		if indegree[id] == 0 {
			*ready = append(*ready, id)
		}
	}
	heap.Init(ready)

	remaining := make(map[string]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		remaining[id] = true
	}

	order = make([]string, 0, len(nodeIDs))
	for ready.Len() > 0 {
		n := heap.Pop(ready).(string)
		delete(remaining, n)
		order = append(order, n)

		for _, m := range adj[n] {
			indegree[m]--
			if indegree[m] == 0 {
				heap.Push(ready, m)
			}
		}
	}

	if len(remaining) == 0 {
		return order, nil, true
	}

	cycle = extractCycle(remaining, edges)
	return nil, cycle, false
}

// extractCycle finds one canonical cycle within the set of nodes that Kahn's
// algorithm could not remove. Every node in `remaining` is guaranteed to
// have at least one predecessor also in `remaining` (otherwise Kahn's would
// have removed it), so following a deterministic predecessor pointer
// (always the lexicographically smallest in-remaining predecessor) from any
// start node must eventually repeat a node, yielding a cycle.
func extractCycle(remaining map[string]bool, edges []Edge) []string {
	preds := make(map[string][]string, len(remaining))
	for _, e := range edges {
		if remaining[e.From] && remaining[e.To] {
			preds[e.To] = append(preds[e.To], e.From)
		}
	}
	for k := range preds {
		sort.Strings(preds[k])
	}

	start := lexMin(remaining)

	// Follow the smallest predecessor from `start`, recording the walk,
	// until a node repeats.
	walk := []string{start}
	pos := map[string]int{start: 0}
	cur := start
	for {
		p := preds[cur][0]
		if j, seen := pos[p]; seen {
			// Cycle (forward/edge direction) is:
			// walk[j] -> walk[len(walk)-1] -> walk[len(walk)-2] -> ... -> walk[j+1] -> (back to walk[j])
			forward := make([]string, 0, len(walk)-j)
			forward = append(forward, walk[j])
			for i := len(walk) - 1; i > j; i-- {
				forward = append(forward, walk[i])
			}
			return rotateToMin(forward)
		}
		pos[p] = len(walk)
		walk = append(walk, p)
		cur = p
	}
}

func lexMin(set map[string]bool) string {
	first := true
	var min string
	for k := range set {
		if first || k < min {
			min = k
			first = false
		}
	}
	return min
}

func rotateToMin(cycle []string) []string {
	minIdx := 0
	for i, v := range cycle {
		if v < cycle[minIdx] {
			minIdx = i
		}
	}
	out := make([]string, len(cycle))
	for i := range cycle {
		out[i] = cycle[(minIdx+i)%len(cycle)]
	}
	return out
}

// LongestPath computes, for every node reachable via edges, the length (in
// edges) of the longest simple path ending at that node, using the fact
// that a DAG's longest path is computable in one topological-order dynamic
// programming pass. order must be a valid topological order of the graph
// formed by nodeIDs/edges (e.g. as returned by TopoSort). Returns the
// maximum depth found across all nodes (0 if there are no edges).
func LongestPath(order []string, edges []Edge) int {
	adj := make(map[string][]string, len(order))
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	depth := make(map[string]int, len(order))
	max := 0
	for _, n := range order {
		for _, m := range adj[n] {
			d := depth[n] + 1
			if d > depth[m] {
				depth[m] = d
			}
			if d > max {
				max = d
			}
		}
	}
	return max
}

// CanonicalTrace returns the deterministic delegation path from a root
// principal to actor, per §8.1: BFS from all principals simultaneously,
// visiting principals in ascending id order, expanding edges in ascending
// destination-id order at each step. The first path BFS finds is canonical.
// If actor is unreachable from any principal via edges, the result is
// []string{actor}.
func CanonicalTrace(principalIDs []string, edges []Edge, actor string) []string {
	adj := make(map[string][]string)
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	for k := range adj {
		sort.Strings(adj[k])
	}

	roots := append([]string(nil), principalIDs...)
	sort.Strings(roots)

	visited := make(map[string]bool)
	isRoot := make(map[string]bool)
	parent := make(map[string]string)
	queue := make([]string, 0, len(roots))
	for _, p := range roots {
		if !visited[p] {
			visited[p] = true
			isRoot[p] = true
			queue = append(queue, p)
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !visited[next] {
				visited[next] = true
				parent[next] = cur
				queue = append(queue, next)
			}
		}
	}

	if !visited[actor] {
		return []string{actor}
	}

	path := []string{actor}
	cur := actor
	for !isRoot[cur] {
		cur = parent[cur]
		path = append(path, cur)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
