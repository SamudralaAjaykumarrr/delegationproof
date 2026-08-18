package explore

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestExploreEmptyTransitions(t *testing.T) {
	res := Explore("approved", nil, 32)
	if res.Truncated {
		t.Fatal("expected Truncated = false")
	}
	if got := sortedKeys(res.Reachable); !reflect.DeepEqual(got, []string{"approved"}) {
		t.Errorf("Reachable = %v, want [approved]", got)
	}
	if len(res.Path) != 0 {
		t.Errorf("Path should be empty when only the initial state is reachable, got %v", res.Path)
	}
}

func TestExploreSingleSelfLoop(t *testing.T) {
	res := Explore("approved", []Transition{
		{From: "approved", To: "approved", Event: "reapprove"},
	}, 32)
	if res.Truncated {
		t.Fatal("expected Truncated = false")
	}
	if got := sortedKeys(res.Reachable); !reflect.DeepEqual(got, []string{"approved"}) {
		t.Errorf("Reachable = %v, want [approved]", got)
	}
}

func TestExploreSimpleTwoStateChain(t *testing.T) {
	res := Explore("approved", []Transition{
		{From: "approved", To: "revoked", Event: "revoke"},
	}, 32)
	if res.Truncated {
		t.Fatal("expected Truncated = false")
	}
	want := []string{"approved", "revoked"}
	if got := sortedKeys(res.Reachable); !reflect.DeepEqual(got, want) {
		t.Errorf("Reachable = %v, want %v", got, want)
	}
	wantPath := []Transition{{From: "approved", To: "revoked", Event: "revoke"}}
	if !reflect.DeepEqual(res.Path["revoked"], wantPath) {
		t.Errorf("Path[revoked] = %v, want %v", res.Path["revoked"], wantPath)
	}
}

func TestExploreBranching(t *testing.T) {
	// Two outgoing edges from one state.
	res := Explore("approved", []Transition{
		{From: "approved", To: "revoked", Event: "revoke"},
		{From: "approved", To: "expired", Event: "expire"},
	}, 32)
	want := []string{"approved", "expired", "revoked"}
	if got := sortedKeys(res.Reachable); !reflect.DeepEqual(got, want) {
		t.Errorf("Reachable = %v, want %v", got, want)
	}
}

func TestExploreMultiStateCycle(t *testing.T) {
	res := Explore("pending", []Transition{
		{From: "pending", To: "approved", Event: "submit"},
		{From: "approved", To: "revoked", Event: "revoke"},
		{From: "revoked", To: "pending", Event: "resubmit"},
	}, 32)
	if res.Truncated {
		t.Fatal("expected Truncated = false; a cyclic automaton must still be fully explorable")
	}
	want := []string{"approved", "pending", "revoked"}
	if got := sortedKeys(res.Reachable); !reflect.DeepEqual(got, want) {
		t.Errorf("Reachable = %v, want %v", got, want)
	}
}

func TestExploreTruncation(t *testing.T) {
	// A fixture that would otherwise fully explore to 4 states, bounded to 2.
	res := Explore("a", []Transition{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "c", To: "d"},
	}, 2)
	if !res.Truncated {
		t.Fatal("expected Truncated = true when maxStates is exceeded")
	}
}

func TestExploreDoesNotRevisitState(t *testing.T) {
	// A state reachable via two different paths is visited (and included in
	// Reachable) exactly once; the queue never re-enqueues an already-visited
	// state (docs/phase-6-plan.md §33 test 17).
	res := Explore("a", []Transition{
		{From: "a", To: "b"},
		{From: "a", To: "c"},
		{From: "b", To: "d"},
		{From: "c", To: "d"},
	}, 32)
	want := []string{"a", "b", "c", "d"}
	if got := sortedKeys(res.Reachable); !reflect.DeepEqual(got, want) {
		t.Errorf("Reachable = %v, want %v", got, want)
	}
	// BFS shortest path: "d" is reached via "b" (declared first, and "b" <
	// "c" lexicographically at the point of expansion from "a") in exactly
	// one hop from whichever of b/c is expanded first; the point of this
	// test is only that exactly one Path entry exists for "d" (single
	// discovery, not a merge of both).
	if len(res.Path["d"]) != 2 {
		t.Errorf("Path[d] should be the single first-discovered path, got %v", res.Path["d"])
	}
}

func TestExploreDeterministicOrdering(t *testing.T) {
	// Declared in non-lexicographic order; BFS must still expand in
	// ascending (To, Event) order (docs/phase-6-plan.md §33 test 20).
	transitions := []Transition{
		{From: "approved", To: "suspended", Event: "suspend"},
		{From: "suspended", To: "revoked", Event: "void"},
		{From: "approved", To: "revoked", Event: "revoke"},
	}
	res := Explore("approved", transitions, 32)
	// The one-hop path to "revoked" (approved -[revoke]-> revoked) must win
	// over the two-hop path through "suspended", by BFS's own shortest-path
	// property (docs/phase-6-plan.md §31.4).
	want := []Transition{{From: "approved", To: "revoked", Event: "revoke"}}
	if !reflect.DeepEqual(res.Path["revoked"], want) {
		t.Errorf("Path[revoked] = %v, want %v (shortest, first-discovered)", res.Path["revoked"], want)
	}
}

func TestExploreDeterministicAcrossRepeatedRuns(t *testing.T) {
	transitions := []Transition{
		{From: "pending", To: "approved", Event: "submit"},
		{From: "approved", To: "revoked", Event: "revoke"},
		{From: "approved", To: "expired", Event: "expire"},
		{From: "revoked", To: "pending", Event: "resubmit"},
	}
	res1 := Explore("pending", transitions, 32)
	res2 := Explore("pending", transitions, 32)
	if !reflect.DeepEqual(sortedKeys(res1.Reachable), sortedKeys(res2.Reachable)) {
		t.Error("Reachable differs across repeated runs")
	}
	for k := range res1.Path {
		if !reflect.DeepEqual(res1.Path[k], res2.Path[k]) {
			t.Errorf("Path[%s] differs across repeated runs: %v vs %v", k, res1.Path[k], res2.Path[k])
		}
	}
	if res1.Truncated != res2.Truncated {
		t.Error("Truncated differs across repeated runs")
	}
}

func TestExploreWideAutomaton(t *testing.T) {
	// Many transitions from one state, fully explored within bounds.
	var transitions []Transition
	states := []string{"b", "c", "d", "e", "f", "g", "h", "i"}
	for _, s := range states {
		transitions = append(transitions, Transition{From: "a", To: s})
	}
	res := Explore("a", transitions, 32)
	if res.Truncated {
		t.Fatal("expected Truncated = false")
	}
	if len(res.Reachable) != len(states)+1 {
		t.Errorf("Reachable has %d entries, want %d", len(res.Reachable), len(states)+1)
	}
}

func TestExploreDeepChain(t *testing.T) {
	// A long simple chain, fully explored within bounds.
	var transitions []Transition
	prev := "s0"
	for i := 1; i <= 20; i++ {
		cur := fmt.Sprintf("s%d", i)
		transitions = append(transitions, Transition{From: prev, To: cur})
		prev = cur
	}
	res := Explore("s0", transitions, 32)
	if res.Truncated {
		t.Fatal("expected Truncated = false")
	}
	if len(res.Reachable) != 21 {
		t.Errorf("Reachable has %d entries, want 21", len(res.Reachable))
	}
}
