package verify

import (
	"reflect"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

func intPtr(v int) *int { return &v }

func mustLoadV4(t *testing.T, path string) *report.Result {
	t.Helper()
	doc, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error for %s: %s", path, loadErr.RenderText())
	}
	if doc.V4 == nil {
		t.Fatalf("expected a version-4 document for %s", path)
	}
	res := RunV4(doc.V4)
	return &res
}

func TestCleanPassV4HasNoFindings(t *testing.T) {
	res := mustLoadV4(t, "../../testdata/valid-v4/clean-pass-v4.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestBillingRedelegationDepthExample(t *testing.T) {
	// examples/billing-redelegation-depth.json (docs/phase-4-plan.md §22):
	// exactly the two-finding shape — one delegation_depth_violation edge
	// finding, one authority_amplification operation finding (the
	// downstream consequence), never a duplicate or masked pair.
	res := mustLoadV4(t, "../../examples/billing-redelegation-depth.json")
	if len(res.Findings) != 2 {
		t.Fatalf("expected exactly 2 findings, got %d: %+v", len(res.Findings), res.Findings)
	}

	depth, ok := res.Findings[0].(report.DelegationDepthFinding)
	if !ok {
		t.Fatalf("expected finding[0] to be a DelegationDepthFinding, got %T", res.Findings[0])
	}
	want := report.DelegationDepthFinding{
		Violation: "delegation_depth_violation",
		Point:     "delegation_edge",
		Delegator: "billing-agent",
		Delegatee: "support-agent",
		Declared:  []report.Capability{{Scope: "billing:refund", Target: "billing-service"}},
		Excess: []report.DepthExcess{
			{Scope: "billing:refund", Target: "billing-service", ConfiguredMax: 1, RemainingDepth: 0},
		},
		Trace:  []string{"admin", "billing-agent", "support-agent"},
		Reason: "billing-agent attempted to delegate billing:refund@billing-service to support-agent, but billing-agent's remaining delegation budget for this capability is 0 (configured maximum: 1) — it may no longer be redelegated",
	}
	if !reflect.DeepEqual(depth, want) {
		t.Errorf("finding[0] = %+v, want %+v", depth, want)
	}

	amp, ok := res.Findings[1].(report.CapabilityOperationFinding)
	if !ok || amp.Violation != report.ViolationAuthorityAmplification {
		t.Fatalf("expected finding[1] to be authority_amplification, got %+v", res.Findings[1])
	}
	if amp.Actor != "support-agent" || amp.Action != "refund-deep" {
		t.Errorf("finding[1] = %+v, want actor=support-agent action=refund-deep", amp)
	}
	if len(amp.Held) != 0 {
		t.Errorf("finding[1] Held = %v, want empty (the depth-exhausted edge contributed nothing)", amp.Held)
	}
}

func TestBillingRedelegationDepthExampleRefundOkPasses(t *testing.T) {
	res := mustLoadV4(t, "../../examples/billing-redelegation-depth.json")
	for _, f := range res.Findings {
		if amp, ok := f.(report.CapabilityOperationFinding); ok && amp.Action == "refund-ok" {
			t.Fatalf("refund-ok must not produce a finding (usable at remaining depth 0), got %+v", amp)
		}
	}
}

func TestMaxDepthZeroUsableNotDelegable(t *testing.T) {
	// §4.2, §11: max_delegation_depth 0 means non-delegable-but-usable. The
	// principal's own operation using it passes; any outgoing delegation
	// edge attempting to grant it produces delegation_depth_violation with
	// remaining_depth 0, configured_max_depth 0.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0)},
			}},
		},
		Agents: []model.AgentV4{{ID: "agent-a"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "agent-a", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "root", Requester: "root", Action: "use-directly", Requires: "a", Target: "svc"},
		},
	}
	res := RunV4(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.DelegationDepthFinding)
	if !ok {
		t.Fatalf("expected a DelegationDepthFinding, got %T", res.Findings[0])
	}
	if len(f.Excess) != 1 || f.Excess[0].ConfiguredMax != 0 || f.Excess[0].RemainingDepth != 0 {
		t.Errorf("Excess = %+v, want configured_max_depth=0 remaining_depth=0", f.Excess)
	}
}

func TestMaxDepthOneExactlyOneHop(t *testing.T) {
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "admin", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "agentA"}, {ID: "agentB"}},
		Delegations: []model.DelegationV4{
			{Delegator: "admin", Delegatee: "agentA", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "agentA", Delegatee: "agentB", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "agentA", Requester: "admin", Action: "use-a", Requires: "a", Target: "svc"},
		},
	}
	res := RunV4(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding (the second hop), got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.DelegationDepthFinding)
	if !ok || f.Delegator != "agentA" || f.Delegatee != "agentB" {
		t.Fatalf("expected a DelegationDepthFinding for agentA->agentB, got %+v", res.Findings[0])
	}
}

func TestDeeperAllowedChainAndFifthHopFails(t *testing.T) {
	// max_delegation_depth 4: a 4-hop chain is entirely valid; a
	// hypothetical 5th hop is invalid.
	buildChain := func(hops int) *model.ModelV4 {
		m := &model.ModelV4{
			Version: "4",
			Principals: []model.PrincipalV4{
				{ID: "root", Authority: []model.RootCapability{
					{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(4)},
				}},
			},
		}
		prev := "root"
		for i := 0; i < hops; i++ {
			next := "n" + string(rune('0'+i))
			m.Agents = append(m.Agents, model.AgentV4{ID: next})
			m.Delegations = append(m.Delegations, model.DelegationV4{
				Delegator: prev, Delegatee: next, Authority: []model.Capability{{Scope: "a", Target: "svc"}},
			})
			prev = next
		}
		m.Operations = []model.OperationV4{
			{Actor: prev, Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		}
		return m
	}

	t.Run("4-hop chain fully valid", func(t *testing.T) {
		res := RunV4(buildChain(4))
		if len(res.Findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
		}
	})

	t.Run("5-hop chain: 5th edge invalid", func(t *testing.T) {
		res := RunV4(buildChain(5))
		var sawDepth bool
		for _, f := range res.Findings {
			if d, ok := f.(report.DelegationDepthFinding); ok {
				sawDepth = true
				if d.Delegator != "n3" || d.Delegatee != "n4" {
					t.Errorf("expected depth violation on n3->n4 (the 5th edge), got %+v", d)
				}
			}
		}
		if !sawDepth {
			t.Fatalf("expected a delegation_depth_violation finding, got %+v", res.Findings)
		}
	})
}

func TestExactlyAtBoundaryChainNoFinding(t *testing.T) {
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(2)},
			}},
		},
		Agents: []model.AgentV4{{ID: "n1"}, {ID: "n2"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "n1", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "n1", Delegatee: "n2", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "n2", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV4(m)
	if len(res.Findings) != 0 {
		t.Fatalf("chain length exactly equal to declared budget must have no findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestBeyondBoundaryChainCascadesViaPresenceNotSecondDepthFinding(t *testing.T) {
	// One hop past the boundary: the first over-budget edge (and only that
	// edge) produces delegation_depth_violation; downstream edges from the
	// poisoned delegatee onward fail separately via ordinary presence
	// failure (authority_amplification), since the poisoned edge propagated
	// nothing — never a second delegation_depth_violation (§12).
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "n1"}, {ID: "n2"}, {ID: "n3"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "n1", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "n1", Delegatee: "n2", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "n2", Delegatee: "n3", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "n3", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV4(m)
	var depthCount, ampCount int
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.DelegationDepthFinding:
			depthCount++
			if v.Delegator != "n1" || v.Delegatee != "n2" {
				t.Errorf("expected the sole depth violation on n1->n2, got %+v", v)
			}
		case report.CapabilityEdgeFinding:
			if v.Violation != report.ViolationAuthorityAmplification {
				t.Errorf("expected the n2->n3 cascade to be authority_amplification, got %+v", v)
			}
			ampCount++
		}
	}
	if depthCount != 1 {
		t.Errorf("expected exactly 1 delegation_depth_violation, got %d: %+v", depthCount, res.Findings)
	}
	if ampCount != 1 {
		t.Errorf("expected exactly 1 cascading edge-level amplification finding, got %d: %+v", ampCount, res.Findings)
	}
}

func TestCurrentHolderCanUseCapabilityAtRemainingDepthZero(t *testing.T) {
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "holder"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV4(m)
	if len(res.Findings) != 0 {
		t.Fatalf("holder at remaining depth 0 must still be able to use the capability, got %d findings: %+v", len(res.Findings), res.Findings)
	}
}

func TestCurrentHolderCannotRedelegateAtRemainingDepthZero(t *testing.T) {
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "holder"}, {ID: "next"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "holder", Delegatee: "next", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{},
	}
	res := RunV4(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	if _, ok := res.Findings[0].(report.DelegationDepthFinding); !ok {
		t.Fatalf("expected a DelegationDepthFinding, got %T", res.Findings[0])
	}
}

func TestMultiplePathsOneExceedsDepthOneValid(t *testing.T) {
	// §10's worked scenario: agent-x reachable via a depth-exhausted path
	// and a valid path simultaneously; agent-x is fully usable and its own
	// remaining budget equals the valid path's value, not 0.
	res := mustLoadV4(t, "../../testdata/valid-v4/multi-path-depth.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (agent-x remains usable via the better path), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestMultiplePathsDifferentRemainingBudgetsNoTie(t *testing.T) {
	// admin-a (budget 1) -> x directly; admin-b (budget 5) -> y -> x.
	// x's remaining via admin-a = 0; via admin-b path = 5-1-1 = 3. The
	// max-remaining path's value (3) must win outright, allowing x to
	// redelegate once more.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "admin-a", Authority: []model.RootCapability{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(1)}}},
			{ID: "admin-b", Authority: []model.RootCapability{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(5)}}},
		},
		Agents: []model.AgentV4{{ID: "x"}, {ID: "y"}, {ID: "z"}},
		Delegations: []model.DelegationV4{
			{Delegator: "admin-a", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "admin-b", Delegatee: "y", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "y", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "x", Delegatee: "z", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
		},
		Operations: []model.OperationV4{},
	}
	res := RunV4(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings — x->z must be valid via the better admin-b path, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestMultiplePathsExactTieDeterministicTieBreak(t *testing.T) {
	// Two paths deliver x the same remaining budget (1) with different
	// configuredMax. Delegator ids are chosen so ascending lexicographic
	// order visits "admin-a" (configuredMax 2) before "admin-b"
	// (configuredMax 9) — first strictly-better wins, ties keep the
	// earlier (lexicographically smaller delegator) result, deterministic
	// and reproducible across repeated runs.
	build := func() *model.ModelV4 {
		return &model.ModelV4{
			Version: "4",
			Principals: []model.PrincipalV4{
				{ID: "admin-a", Authority: []model.RootCapability{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(2)}}},
				{ID: "admin-b", Authority: []model.RootCapability{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(9)}}},
			},
			Agents: []model.AgentV4{{ID: "mid-a"}, {ID: "mid-b"}, {ID: "x"}},
			Delegations: []model.DelegationV4{
				{Delegator: "admin-a", Delegatee: "mid-a", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
				{Delegator: "mid-a", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
				{Delegator: "admin-b", Delegatee: "mid-b", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
				// mid-b -> mid-b2 -> ... consumes extra hops so mid-b's
				// direct delivery to x also lands at remaining 1 (tie).
				{Delegator: "mid-b", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			},
			Operations: []model.OperationV4{},
		}
	}
	// admin-a(2) -> mid-a(1) -> x(0)   [via admin-a]
	// admin-b(9) -> mid-b(8) -> x(7)   [via admin-b] -- not a tie as built.
	// Rebuild with matching depths to force an exact tie at x.
	m := build()
	m.Principals[1].Authority[0].MaxDelegationDepth = intPtr(3) // admin-b(3) -> mid-b(2) -> x(1), ties admin-a's path (1)
	res1 := RunV4(m)
	res2 := RunV4(m)
	j1, _ := report.RenderJSON(res1)
	j2, _ := report.RenderJSON(res2)
	if string(j1) != string(j2) {
		t.Errorf("tie-break result not deterministic across repeated runs:\n%s\n---\n%s", j1, j2)
	}
	if len(res1.Findings) != 0 {
		t.Fatalf("expected 0 findings (x holds c@svc at remaining 1 via the tie), got %d: %+v", len(res1.Findings), res1.Findings)
	}
}

func TestReorderedIncomingEdgeOrderInvarianceAtMultiPathNode(t *testing.T) {
	// Reordering the delegations array (and hence which edge is visited
	// first in insertion order) must not change the result: incoming
	// edges are always re-sorted ascending by delegator id before
	// evaluation (§10).
	forward := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "admin-a", Authority: []model.RootCapability{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(1)}}},
			{ID: "admin-b", Authority: []model.RootCapability{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(3)}}},
		},
		Agents: []model.AgentV4{{ID: "x"}, {ID: "y"}},
		Delegations: []model.DelegationV4{
			{Delegator: "admin-a", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "admin-b", Delegatee: "y", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "y", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "x", Requester: "admin-a", Action: "use", Requires: "c", Target: "svc"},
		},
	}
	reversed := &model.ModelV4{
		Version:    forward.Version,
		Principals: []model.PrincipalV4{forward.Principals[1], forward.Principals[0]},
		Agents:     []model.AgentV4{forward.Agents[1], forward.Agents[0]},
		Delegations: []model.DelegationV4{
			forward.Delegations[2], forward.Delegations[1], forward.Delegations[0],
		},
		Operations: forward.Operations,
	}
	r1, _ := report.RenderJSON(RunV4(forward))
	r2, _ := report.RenderJSON(RunV4(reversed))
	if string(r1) != string(r2) {
		t.Errorf("reordered input produced different output:\n--- forward ---\n%s\n--- reversed ---\n%s", r1, r2)
	}
}

func TestStrictDistrustPresenceWinsOverDepth(t *testing.T) {
	// An edge carrying one presence-invalid capability and one
	// depth-exhausted capability: authority_amplification wins (§12's
	// three-tier precedence), not delegation_depth_violation, and the
	// whole edge (including the depth-exhausted-but-otherwise-fine
	// capability) is poisoned.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "mid"}, {ID: "next"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			// mid only holds a@svc (at remaining 0); attempts to delegate
			// both a@svc (depth-exhausted) and never-held@svc (presence
			// failure) in one edge.
			{Delegator: "mid", Delegatee: "next", Authority: []model.Capability{
				{Scope: "a", Target: "svc"},
				{Scope: "never-held", Target: "svc"},
			}},
		},
		Operations: []model.OperationV4{},
	}
	res := RunV4(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityEdgeFinding)
	if !ok || f.Violation != report.ViolationAuthorityAmplification {
		t.Fatalf("expected authority_amplification, got %+v", res.Findings[0])
	}
}

func TestStrictDistrustBindingWinsOverDepth(t *testing.T) {
	// One binding-invalid and one depth-exhausted capability in the same
	// edge: context_binding_violation wins over delegation_depth_violation.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc-a", MaxDelegationDepth: intPtr(1)},
				{Scope: "b", Target: "svc-a", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "mid"}, {ID: "next"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{
				{Scope: "a", Target: "svc-a"},
				{Scope: "b", Target: "svc-a"},
			}},
			// mid holds a@svc-a and b@svc-a, both at remaining 0. Attempts
			// to delegate a@svc-b (binding failure: a is held only for
			// svc-a) and b@svc-a (depth-exhausted, but otherwise fine).
			{Delegator: "mid", Delegatee: "next", Authority: []model.Capability{
				{Scope: "a", Target: "svc-b"},
				{Scope: "b", Target: "svc-a"},
			}},
		},
		Operations: []model.OperationV4{},
	}
	res := RunV4(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityEdgeFinding)
	if !ok || f.Violation != report.ViolationContextBinding {
		t.Fatalf("expected context_binding_violation, got %+v", res.Findings[0])
	}
}

func TestContextBindingInteractionOrthogonalCase(t *testing.T) {
	// A capability correctly bound and well within budget, alongside an
	// unrelated capability that is binding-invalid in a different edge:
	// both findings present, correctly classified, no interference.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{
				{Scope: "a", Target: "svc-a", MaxDelegationDepth: intPtr(3)},
			}},
		},
		Agents: []model.AgentV4{{ID: "fine"}, {ID: "bad"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "fine", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
			{Delegator: "root", Delegatee: "bad", Authority: []model.Capability{{Scope: "a", Target: "svc-b"}}},
		},
		Operations: []model.OperationV4{},
	}
	res := RunV4(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding (the binding failure only), got %d: %+v", len(res.Findings), res.Findings)
	}
	if f, ok := res.Findings[0].(report.CapabilityEdgeFinding); !ok || f.Violation != report.ViolationContextBinding {
		t.Fatalf("expected context_binding_violation, got %+v", res.Findings[0])
	}
}

func TestConfusedDeputyInteractionIndependentOfDepthViolation(t *testing.T) {
	// examples/billing-redelegation-depth.json-style fixture extended with
	// a requester lacking standing on the still-valid (refund-ok)
	// operation: confused_deputy fires there independently of the
	// unrelated delegation_depth_violation finding on the second edge.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "admin", Authority: []model.RootCapability{
				{Scope: "billing:refund", Target: "billing-service", MaxDelegationDepth: intPtr(1)},
			}},
		},
		Agents: []model.AgentV4{{ID: "billing-agent"}, {ID: "support-agent"}, {ID: "outsider"}},
		Delegations: []model.DelegationV4{
			{Delegator: "admin", Delegatee: "billing-agent", Authority: []model.Capability{{Scope: "billing:refund", Target: "billing-service"}}},
			{Delegator: "billing-agent", Delegatee: "support-agent", Authority: []model.Capability{{Scope: "billing:refund", Target: "billing-service"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "billing-agent", Requester: "outsider", Action: "refund-cd", Requires: "billing:refund", Target: "billing-service"},
		},
	}
	res := RunV4(m)
	var sawDepth, sawConfusedDeputy bool
	for _, f := range res.Findings {
		switch f.(type) {
		case report.DelegationDepthFinding:
			sawDepth = true
		case report.ConfusedDeputyFinding:
			sawConfusedDeputy = true
		}
	}
	if !sawDepth || !sawConfusedDeputy {
		t.Fatalf("expected both a delegation_depth_violation and a confused_deputy finding, got %+v", res.Findings)
	}
}

func TestCombinedViolationsV4(t *testing.T) {
	res := mustLoadV4(t, "../../testdata/valid-v4/combined-violations-v4.json")
	if len(res.Findings) != 6 {
		t.Fatalf("expected exactly 6 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
	var sawAmp, sawCD, sawCtx, sawDepth int
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.CapabilityOperationFinding:
			switch v.Violation {
			case report.ViolationAuthorityAmplification:
				sawAmp++
			case report.ViolationContextBinding:
				sawCtx++
			}
		case report.ConfusedDeputyFinding:
			sawCD++
		case report.DelegationDepthFinding:
			sawDepth++
		}
	}
	if sawAmp != 2 || sawCD != 2 || sawCtx != 1 || sawDepth != 1 {
		t.Errorf("finding mix = amp:%d cd:%d ctx:%d depth:%d, want 2,2,1,1", sawAmp, sawCD, sawCtx, sawDepth)
	}
}

func TestDelegationDepthPrecedenceTable(t *testing.T) {
	// Dedicated table test covering every row of §12's three-tier edge
	// classification.
	baseModel := func() *model.ModelV4 {
		return &model.ModelV4{
			Version: "4",
			Principals: []model.PrincipalV4{
				{ID: "root", Authority: []model.RootCapability{
					{Scope: "a", Target: "svc-a", MaxDelegationDepth: intPtr(1)},
					{Scope: "b", Target: "svc-a", MaxDelegationDepth: intPtr(1)},
				}},
			},
			Agents: []model.AgentV4{{ID: "mid"}, {ID: "next"}},
			Delegations: []model.DelegationV4{
				{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{
					{Scope: "a", Target: "svc-a"},
					{Scope: "b", Target: "svc-a"},
				}},
			},
			Operations: []model.OperationV4{},
		}
	}

	t.Run("missing capability -> authority_amplification", func(t *testing.T) {
		m := baseModel()
		m.Delegations = append(m.Delegations, model.DelegationV4{
			Delegator: "mid", Delegatee: "next", Authority: []model.Capability{{Scope: "never-held", Target: "svc-a"}},
		})
		res := RunV4(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		if f, ok := res.Findings[0].(report.CapabilityEdgeFinding); !ok || f.Violation != report.ViolationAuthorityAmplification {
			t.Errorf("expected authority_amplification, got %+v", res.Findings[0])
		}
	})

	t.Run("wrong target -> context_binding_violation", func(t *testing.T) {
		m := baseModel()
		m.Delegations = append(m.Delegations, model.DelegationV4{
			Delegator: "mid", Delegatee: "next", Authority: []model.Capability{{Scope: "a", Target: "svc-b"}},
		})
		res := RunV4(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		if f, ok := res.Findings[0].(report.CapabilityEdgeFinding); !ok || f.Violation != report.ViolationContextBinding {
			t.Errorf("expected context_binding_violation, got %+v", res.Findings[0])
		}
	})

	t.Run("present, correctly bound, budget exhausted -> delegation_depth_violation", func(t *testing.T) {
		m := baseModel()
		m.Delegations = append(m.Delegations, model.DelegationV4{
			Delegator: "mid", Delegatee: "next", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}},
		})
		res := RunV4(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		if _, ok := res.Findings[0].(report.DelegationDepthFinding); !ok {
			t.Errorf("expected delegation_depth_violation, got %+v", res.Findings[0])
		}
	})

	t.Run("present, correctly bound, budget available -> ALLOW", func(t *testing.T) {
		m := baseModel()
		m.Principals[0].Authority[0].MaxDelegationDepth = intPtr(2)
		m.Delegations = append(m.Delegations, model.DelegationV4{
			Delegator: "mid", Delegatee: "next", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}},
		})
		res := RunV4(m)
		if len(res.Findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
		}
	})
}

func TestOperationNeverFailsWithDelegationDepthViolation(t *testing.T) {
	// §12: depth never manifests as an operation-level finding of its own.
	// Every operation-level finding in this suite must be one of the three
	// pre-existing violation literals, never delegation_depth_violation.
	res := mustLoadV4(t, "../../testdata/valid-v4/combined-violations-v4.json")
	for _, f := range res.Findings {
		if d, ok := f.(report.DelegationDepthFinding); ok && d.Point == report.PointOperation {
			t.Errorf("delegation_depth_violation must never be an operation-level finding, got %+v", d)
		}
	}
}

func TestDelegationDepthFindingSortKeyMatchesEdgeConvention(t *testing.T) {
	// §14: no new sort-key field required — keyed identically to
	// EdgeFinding/CapabilityEdgeFinding by (point, delegator, delegatee).
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)}}},
		},
		Agents: []model.AgentV4{{ID: "z-mid"}, {ID: "a-mid"}, {ID: "next"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "z-mid", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "root", Delegatee: "a-mid", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "z-mid", Delegatee: "next", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "a-mid", Delegatee: "next", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{},
	}
	res := RunV4(m)
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
	first, ok := res.Findings[0].(report.DelegationDepthFinding)
	if !ok {
		t.Fatalf("expected DelegationDepthFinding, got %T", res.Findings[0])
	}
	second, ok := res.Findings[1].(report.DelegationDepthFinding)
	if !ok {
		t.Fatalf("expected DelegationDepthFinding, got %T", res.Findings[1])
	}
	if first.Delegator != "a-mid" || second.Delegator != "z-mid" {
		t.Errorf("not sorted ascending by delegator: got %q then %q", first.Delegator, second.Delegator)
	}
}

func TestDelegationDepthTraceUsesCanonicalTraceConvention(t *testing.T) {
	res := mustLoadV4(t, "../../examples/billing-redelegation-depth.json")
	for _, f := range res.Findings {
		d, ok := f.(report.DelegationDepthFinding)
		if !ok {
			continue
		}
		want := []string{"admin", "billing-agent", "support-agent"}
		if !reflect.DeepEqual(d.Trace, want) {
			t.Errorf("Trace = %v, want %v", d.Trace, want)
		}
	}
}

func TestRunV4IsDeterministicAcrossRepeatedInvocations(t *testing.T) {
	doc, loadErr := loader.LoadDocument("../../testdata/valid-v4/combined-violations-v4.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	r1, err1 := report.RenderJSON(RunV4(doc.V4))
	r2, err2 := report.RenderJSON(RunV4(doc.V4))
	if err1 != nil || err2 != nil {
		t.Fatalf("render errors: %v, %v", err1, err2)
	}
	if string(r1) != string(r2) {
		t.Errorf("RunV4 produced different output across repeated invocations:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", r1, r2)
	}
}

func TestRunV4InputArrayPermutationInvariance(t *testing.T) {
	doc1, loadErr1 := loader.LoadDocument("../../testdata/valid-v4/clean-pass-v4.json")
	if loadErr1 != nil {
		t.Fatalf("unexpected load error: %s", loadErr1.RenderText())
	}
	doc2, loadErr2 := loader.LoadDocument("../../testdata/valid-v4/clean-pass-v4-reordered.json")
	if loadErr2 != nil {
		t.Fatalf("unexpected load error: %s", loadErr2.RenderText())
	}
	r1, _ := report.RenderJSON(RunV4(doc1.V4))
	r2, _ := report.RenderJSON(RunV4(doc2.V4))
	if string(r1) != string(r2) {
		t.Errorf("semantically-equivalent reordered v4 input produced different output:\n--- original ---\n%s\n--- reordered ---\n%s", r1, r2)
	}
}

func TestRequesterUsageDoesNotConsumeDepth(t *testing.T) {
	// §13: naming a node as a requester creates no delegation edge and
	// consumes no budget — a requester holding a capability at remaining
	// depth 0 still satisfies the requester-side check.
	m := &model.ModelV4{
		Version: "4",
		Principals: []model.PrincipalV4{
			{ID: "root", Authority: []model.RootCapability{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1)}}},
		},
		Agents: []model.AgentV4{{ID: "actor"}, {ID: "requester-holder"}},
		Delegations: []model.DelegationV4{
			{Delegator: "root", Delegatee: "actor", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "root", Delegatee: "requester-holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV4{
			{Actor: "actor", Requester: "requester-holder", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV4(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}
