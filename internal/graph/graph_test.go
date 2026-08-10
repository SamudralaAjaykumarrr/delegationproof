package graph

import (
	"reflect"
	"testing"
)

func TestTopoSortAcyclic(t *testing.T) {
	nodes := []string{"c", "a", "b"}
	edges := []Edge{{"a", "b"}, {"a", "c"}, {"b", "c"}}
	order, cycle, ok := TopoSort(nodes, edges)
	if !ok {
		t.Fatalf("expected acyclic, got cycle %v", cycle)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestTopoSortTieBreak(t *testing.T) {
	// No edges: every node is immediately ready. Order must be pure
	// ascending lexicographic, independent of input order.
	nodes := []string{"z", "b", "a", "m"}
	order, _, ok := TopoSort(nodes, nil)
	if !ok {
		t.Fatal("expected acyclic")
	}
	want := []string{"a", "b", "m", "z"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestTopoSortDeterministicAcrossInputOrder(t *testing.T) {
	edges1 := []Edge{{"a", "b"}, {"b", "c"}, {"a", "c"}}
	edges2 := []Edge{{"a", "c"}, {"b", "c"}, {"a", "b"}} // reordered
	nodes1 := []string{"a", "b", "c"}
	nodes2 := []string{"c", "b", "a"} // reordered

	order1, _, ok1 := TopoSort(nodes1, edges1)
	order2, _, ok2 := TopoSort(nodes2, edges2)
	if !ok1 || !ok2 {
		t.Fatal("expected both acyclic")
	}
	if !reflect.DeepEqual(order1, order2) {
		t.Errorf("order1 = %v, order2 = %v; want identical", order1, order2)
	}
}

func TestTopoSortTwoNodeCycle(t *testing.T) {
	nodes := []string{"agent-a", "agent-b"}
	edges := []Edge{{"agent-a", "agent-b"}, {"agent-b", "agent-a"}}
	_, cycle, ok := TopoSort(nodes, edges)
	if ok {
		t.Fatal("expected cycle detected")
	}
	want := []string{"agent-a", "agent-b"}
	if !reflect.DeepEqual(cycle, want) {
		t.Errorf("cycle = %v, want %v", cycle, want)
	}
}

func TestTopoSortThreeNodeCycle(t *testing.T) {
	nodes := []string{"agent-a", "agent-b", "agent-c"}
	edges := []Edge{{"agent-a", "agent-b"}, {"agent-b", "agent-c"}, {"agent-c", "agent-a"}}
	_, cycle, ok := TopoSort(nodes, edges)
	if ok {
		t.Fatal("expected cycle detected")
	}
	want := []string{"agent-a", "agent-b", "agent-c"}
	if !reflect.DeepEqual(cycle, want) {
		t.Errorf("cycle = %v, want %v", cycle, want)
	}
}

func TestTopoSortSelfLoop(t *testing.T) {
	nodes := []string{"a"}
	edges := []Edge{{"a", "a"}}
	_, cycle, ok := TopoSort(nodes, edges)
	if ok {
		t.Fatal("expected cycle detected")
	}
	want := []string{"a"}
	if !reflect.DeepEqual(cycle, want) {
		t.Errorf("cycle = %v, want %v", cycle, want)
	}
}

func TestTopoSortCycleWithDependentTail(t *testing.T) {
	// z depends on the a<->b cycle but is not itself part of any cycle.
	// The reported cycle must contain only {a, b}, never z, and must be
	// rooted at the lexicographically smallest cycle member.
	nodes := []string{"z", "a", "b"}
	edges := []Edge{{"a", "b"}, {"b", "a"}, {"a", "z"}}
	_, cycle, ok := TopoSort(nodes, edges)
	if ok {
		t.Fatal("expected cycle detected")
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(cycle, want) {
		t.Errorf("cycle = %v, want %v", cycle, want)
	}
}

func TestTopoSortCycleCanonicalRotation(t *testing.T) {
	// Cycle b -> c -> a -> b: lexicographically smallest is "a", so the
	// canonical reported cycle must start there, in edge-forward order.
	nodes := []string{"a", "b", "c"}
	edges := []Edge{{"b", "c"}, {"c", "a"}, {"a", "b"}}
	_, cycle, ok := TopoSort(nodes, edges)
	if ok {
		t.Fatal("expected cycle detected")
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cycle, want) {
		t.Errorf("cycle = %v, want %v", cycle, want)
	}
}

func TestLongestPath(t *testing.T) {
	// a -> b -> c -> d : longest path = 3 edges
	nodes := []string{"a", "b", "c", "d"}
	edges := []Edge{{"a", "b"}, {"b", "c"}, {"c", "d"}}
	order, _, ok := TopoSort(nodes, edges)
	if !ok {
		t.Fatal("expected acyclic")
	}
	if got := LongestPath(order, edges); got != 3 {
		t.Errorf("LongestPath = %d, want 3", got)
	}
}

func TestLongestPathNoEdges(t *testing.T) {
	nodes := []string{"a", "b"}
	order, _, ok := TopoSort(nodes, nil)
	if !ok {
		t.Fatal("expected acyclic")
	}
	if got := LongestPath(order, nil); got != 0 {
		t.Errorf("LongestPath = %d, want 0", got)
	}
}

func TestCanonicalTraceShortestPathTieBreak(t *testing.T) {
	// Two principals p1, p2 both reach "leaf". p1 < p2 lexicographically,
	// and both paths have the same length, so p1's path is canonical.
	principals := []string{"p2", "p1"}
	edges := []Edge{
		{"p1", "leaf"},
		{"p2", "leaf"},
	}
	trace := CanonicalTrace(principals, edges, "leaf")
	want := []string{"p1", "leaf"}
	if !reflect.DeepEqual(trace, want) {
		t.Errorf("trace = %v, want %v", trace, want)
	}
}

func TestCanonicalTraceShortestOverLonger(t *testing.T) {
	// p1 -> leaf directly (length 1); p1 -> mid -> leaf (length 2).
	// The shorter path must win regardless of edge expansion order.
	principals := []string{"p1"}
	edges := []Edge{
		{"p1", "mid"},
		{"mid", "leaf"},
		{"p1", "leaf"},
	}
	trace := CanonicalTrace(principals, edges, "leaf")
	want := []string{"p1", "leaf"}
	if !reflect.DeepEqual(trace, want) {
		t.Errorf("trace = %v, want %v", trace, want)
	}
}

func TestCanonicalTraceUnreachable(t *testing.T) {
	principals := []string{"p1"}
	edges := []Edge{{"p1", "a"}}
	trace := CanonicalTrace(principals, edges, "orphan")
	want := []string{"orphan"}
	if !reflect.DeepEqual(trace, want) {
		t.Errorf("trace = %v, want %v", trace, want)
	}
}

func TestCanonicalTracePrincipalIsActor(t *testing.T) {
	principals := []string{"p1", "p2"}
	trace := CanonicalTrace(principals, nil, "p2")
	want := []string{"p2"}
	if !reflect.DeepEqual(trace, want) {
		t.Errorf("trace = %v, want %v", trace, want)
	}
}
