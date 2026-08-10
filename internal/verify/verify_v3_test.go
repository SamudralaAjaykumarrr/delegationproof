package verify

import (
	"reflect"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

func mustLoadV3(t *testing.T, path string) *report.Result {
	t.Helper()
	doc, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error for %s: %s", path, loadErr.RenderText())
	}
	if doc.V3 == nil {
		t.Fatalf("expected a version-3 document for %s", path)
	}
	res := RunV3(doc.V3)
	return &res
}

func TestCleanPassV3HasNoFindings(t *testing.T) {
	res := mustLoadV3(t, "../../testdata/valid-v3/clean-pass-v3.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestBillingConfusedDeputyExample(t *testing.T) {
	res := mustLoadV3(t, "../../examples/billing-confused-deputy.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ConfusedDeputyFinding)
	if !ok {
		t.Fatalf("expected a ConfusedDeputyFinding, got %T", res.Findings[0])
	}
	want := report.ConfusedDeputyFinding{
		Violation:             "confused_deputy",
		Point:                 "operation",
		Actor:                 "billing-agent",
		Requester:             "support-agent",
		Action:                "refund-b",
		Requires:              report.Capability{Scope: "billing:refund", Target: "billing-service"},
		ActorHeld:             []report.Capability{{Scope: "billing:refund", Target: "billing-service"}},
		RequesterHeld:         []report.Capability{{Scope: "billing:read", Target: "billing-service"}},
		RequesterBoundTargets: []string{},
		ActorTrace:            []string{"admin", "billing-agent", "refund-b"},
		RequesterTrace:        []string{"admin", "support-agent"},
		Reason:                "refund-b requires billing:refund@billing-service, which billing-agent validly holds, but requester support-agent has never held billing:refund under any target — billing-agent is being induced to exercise authority support-agent was never granted",
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("finding = %+v, want %+v", f, want)
	}
}

func TestBillingConfusedDeputyExampleRefundAPasses(t *testing.T) {
	// refund-a's requester is admin, a principal that axiomatically holds
	// billing:refund@billing-service directly — passes, no finding for it.
	res := mustLoadV3(t, "../../examples/billing-confused-deputy.json")
	for _, f := range res.Findings {
		if cd, ok := f.(report.ConfusedDeputyFinding); ok && cd.Action == "refund-a" {
			t.Fatalf("refund-a must not produce a finding, got %+v", cd)
		}
	}
}

func TestCombinedViolationsV3(t *testing.T) {
	res := mustLoadV3(t, "../../testdata/valid-v3/combined-violations-v3.json")
	if len(res.Findings) != 4 {
		t.Fatalf("expected exactly 4 findings, got %d: %+v", len(res.Findings), res.Findings)
	}

	amp, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok || amp.Violation != report.ViolationAuthorityAmplification {
		t.Fatalf("expected finding[0] to be authority_amplification, got %+v", res.Findings[0])
	}
	if amp.Action != "op-amp" {
		t.Errorf("finding[0] action = %q, want op-amp", amp.Action)
	}

	cdScope, ok := res.Findings[1].(report.ConfusedDeputyFinding)
	if !ok || cdScope.Violation != report.ViolationConfusedDeputy {
		t.Fatalf("expected finding[1] to be confused_deputy, got %+v", res.Findings[1])
	}
	if cdScope.Action != "op-cd-scope" || len(cdScope.RequesterBoundTargets) != 0 {
		t.Errorf("finding[1] = %+v, want action=op-cd-scope, empty RequesterBoundTargets", cdScope)
	}

	cdTarget, ok := res.Findings[2].(report.ConfusedDeputyFinding)
	if !ok || cdTarget.Violation != report.ViolationConfusedDeputy {
		t.Fatalf("expected finding[2] to be confused_deputy, got %+v", res.Findings[2])
	}
	if cdTarget.Action != "op-cd-target" {
		t.Errorf("finding[2] action = %q, want op-cd-target", cdTarget.Action)
	}
	if len(cdTarget.RequesterBoundTargets) != 1 || cdTarget.RequesterBoundTargets[0] != "svc-b" {
		t.Errorf("finding[2] RequesterBoundTargets = %v, want [svc-b]", cdTarget.RequesterBoundTargets)
	}

	ctx, ok := res.Findings[3].(report.CapabilityOperationFinding)
	if !ok || ctx.Violation != report.ViolationContextBinding {
		t.Fatalf("expected finding[3] to be context_binding_violation, got %+v", res.Findings[3])
	}
	if ctx.Action != "op-ctx" {
		t.Errorf("finding[3] action = %q, want op-ctx", ctx.Action)
	}
}

func TestMultiHopRequesterAuthority(t *testing.T) {
	// requester's capability arrives via a 3+ hop chain disjoint from the
	// actor's own chain (§7: requester need not be an ancestor of actor).
	res := mustLoadV3(t, "../../testdata/valid-v3/multi-hop-requester.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestRequesterEqualsActorTrivialPass(t *testing.T) {
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "agent"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "agent", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			{Actor: "agent", Requester: "agent", Action: "op", Requires: "a", Target: "svc"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 0 {
		t.Fatalf("requester == actor must always pass when the actor-side check passes, got %d findings: %+v", len(res.Findings), res.Findings)
	}
}

func TestPrincipalRequesterPasses(t *testing.T) {
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "agent"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "agent", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			{Actor: "agent", Requester: "root", Action: "op", Requires: "a", Target: "svc"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings for a principal requester holding the capability directly, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestAgentRequesterViaIndependentChainPasses(t *testing.T) {
	// requester is an agent whose grant arrives via a chain not overlapping
	// the actor's own chain at all.
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "actor-agent"}, {ID: "requester-agent"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "actor-agent", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "root", Delegatee: "requester-agent", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			{Actor: "actor-agent", Requester: "requester-agent", Action: "op", Requires: "a", Target: "svc"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestUnreachableRequesterIsAlwaysConfusedDeputy(t *testing.T) {
	// An orphan agent (no valid incoming edges) has DA = ∅, which cannot
	// contain any capability — every operation naming it as requester,
	// where the actor legitimately holds the capability, is unconditionally
	// confused_deputy (§10).
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "actor-agent"}, {ID: "orphan"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "actor-agent", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			{Actor: "actor-agent", Requester: "orphan", Action: "op", Requires: "a", Target: "svc"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ConfusedDeputyFinding)
	if !ok {
		t.Fatalf("expected a ConfusedDeputyFinding, got %T", res.Findings[0])
	}
	if !reflect.DeepEqual(f.RequesterTrace, []string{"orphan"}) {
		t.Errorf("requester_trace = %v, want [orphan] (unreachable)", f.RequesterTrace)
	}
	if len(f.RequesterHeld) != 0 {
		t.Errorf("requester_held = %v, want empty", f.RequesterHeld)
	}
}

func TestRequesterAuthorityThroughInvalidEdgeIsDistrusted(t *testing.T) {
	// requester's only apparent grant of the needed capability arrives over
	// an edge that is itself distrusted (over-claims relative to its own
	// delegator) — DA(requester) must exclude it, so confused_deputy fires
	// despite the document superficially naming the right scope somewhere
	// upstream of requester (§9).
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "b", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "actor-agent"}, {ID: "mid"}, {ID: "requester-agent"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "actor-agent", Authority: []model.Capability{{Scope: "b", Target: "svc"}}},
			// root only holds b@svc, but mid attempts to delegate a@svc to
			// requester-agent — an over-claim, so the whole edge is
			// distrusted (strict distrust), not partially honored.
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{{Scope: "b", Target: "svc"}}},
			{Delegator: "mid", Delegatee: "requester-agent", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			{Actor: "actor-agent", Requester: "requester-agent", Action: "op", Requires: "b", Target: "svc"},
		},
	}
	res := RunV3(m)
	// Expect 2 findings: the invalid mid->requester-agent edge, and the
	// resulting confused_deputy operation finding.
	var sawEdge, sawConfusedDeputy bool
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.CapabilityEdgeFinding:
			sawEdge = true
		case report.ConfusedDeputyFinding:
			sawConfusedDeputy = true
			if len(v.RequesterHeld) != 0 {
				t.Errorf("requester_held = %v, want empty — invalid edge must contribute nothing", v.RequesterHeld)
			}
		}
	}
	if !sawEdge || !sawConfusedDeputy {
		t.Fatalf("expected both an edge finding and a confused_deputy finding, got %+v", res.Findings)
	}
}

func TestActorAmplificationMasksRequesterFailure(t *testing.T) {
	// Actor does not hold the scope at all AND requester (independently)
	// also lacks standing — asserts exactly one finding
	// (authority_amplification), not two.
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "z", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "actor-agent"}, {ID: "requester-agent"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "actor-agent", Authority: []model.Capability{{Scope: "z", Target: "svc"}}},
			{Delegator: "root", Delegatee: "requester-agent", Authority: []model.Capability{{Scope: "z", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			// requires "a", which neither actor-agent nor requester-agent
			// ever holds under any target.
			{Actor: "actor-agent", Requester: "requester-agent", Action: "op", Requires: "a", Target: "svc"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok || f.Violation != report.ViolationAuthorityAmplification {
		t.Fatalf("expected authority_amplification, got %+v", res.Findings[0])
	}
}

func TestActorContextBindingMasksRequesterFailure(t *testing.T) {
	// Actor holds the scope only for the wrong target AND requester also
	// lacks standing — asserts exactly one finding
	// (context_binding_violation), not two.
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
		},
		Agents: []model.AgentV3{{ID: "actor-agent"}, {ID: "requester-agent"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "actor-agent", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
		},
		Operations: []model.OperationV3{
			// requires a@svc-b; actor-agent only holds a@svc-a.
			// requester-agent isn't even a valid delegatee target here (no
			// edge to it at all — DA = ∅), which would also fail
			// independently, but the actor-side finding must mask it.
			{Actor: "actor-agent", Requester: "requester-agent", Action: "op", Requires: "a", Target: "svc-b"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok || f.Violation != report.ViolationContextBinding {
		t.Fatalf("expected context_binding_violation, got %+v", res.Findings[0])
	}
}

func TestConfusedDeputyPrecedenceTable(t *testing.T) {
	// Dedicated table test covering every row of §8's precedence table.
	baseModel := func() *model.ModelV3 {
		return &model.ModelV3{
			Version: "3",
			Principals: []model.PrincipalV3{
				{ID: "root", Authority: []model.Capability{
					{Scope: "a", Target: "svc-a"},
					{Scope: "b", Target: "svc-a"},
				}},
			},
			Agents: []model.AgentV3{{ID: "actor"}, {ID: "requester-full"}, {ID: "requester-wrong-scope"}, {ID: "requester-wrong-target"}},
			Delegations: []model.DelegationV3{
				{Delegator: "root", Delegatee: "actor", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
				{Delegator: "root", Delegatee: "requester-full", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
				{Delegator: "root", Delegatee: "requester-wrong-scope", Authority: []model.Capability{{Scope: "b", Target: "svc-a"}}},
			},
		}
	}

	t.Run("actor never holds scope -> authority_amplification, requester not evaluated", func(t *testing.T) {
		m := baseModel()
		m.Operations = []model.OperationV3{
			{Actor: "actor", Requester: "requester-full", Action: "op", Requires: "never-held", Target: "svc-a"},
		}
		res := RunV3(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		if f, ok := res.Findings[0].(report.CapabilityOperationFinding); !ok || f.Violation != report.ViolationAuthorityAmplification {
			t.Errorf("expected authority_amplification, got %+v", res.Findings[0])
		}
	})

	t.Run("actor holds scope wrong target only -> context_binding_violation, requester not evaluated", func(t *testing.T) {
		m := baseModel()
		m.Operations = []model.OperationV3{
			{Actor: "actor", Requester: "requester-full", Action: "op", Requires: "a", Target: "svc-b"},
		}
		res := RunV3(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		if f, ok := res.Findings[0].(report.CapabilityOperationFinding); !ok || f.Violation != report.ViolationContextBinding {
			t.Errorf("expected context_binding_violation, got %+v", res.Findings[0])
		}
	})

	t.Run("actor holds, requester holds -> ALLOW", func(t *testing.T) {
		m := baseModel()
		m.Operations = []model.OperationV3{
			{Actor: "actor", Requester: "requester-full", Action: "op", Requires: "a", Target: "svc-a"},
		}
		res := RunV3(m)
		if len(res.Findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
		}
	})

	t.Run("actor holds, requester never held scope -> confused_deputy, empty bound targets", func(t *testing.T) {
		m := baseModel()
		m.Operations = []model.OperationV3{
			{Actor: "actor", Requester: "requester-wrong-scope", Action: "op", Requires: "a", Target: "svc-a"},
		}
		res := RunV3(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		f, ok := res.Findings[0].(report.ConfusedDeputyFinding)
		if !ok || f.Violation != report.ViolationConfusedDeputy {
			t.Fatalf("expected confused_deputy, got %+v", res.Findings[0])
		}
		if len(f.RequesterBoundTargets) != 0 {
			t.Errorf("RequesterBoundTargets = %v, want empty", f.RequesterBoundTargets)
		}
	})

	t.Run("actor holds, requester holds scope wrong target only -> confused_deputy, non-empty bound targets", func(t *testing.T) {
		m := baseModel()
		m.Delegations = append(m.Delegations, model.DelegationV3{
			Delegator: "root", Delegatee: "requester-wrong-target", Authority: []model.Capability{{Scope: "a", Target: "svc-b"}},
		})
		m.Principals[0].Authority = append(m.Principals[0].Authority, model.Capability{Scope: "a", Target: "svc-b"})
		m.Operations = []model.OperationV3{
			{Actor: "actor", Requester: "requester-wrong-target", Action: "op", Requires: "a", Target: "svc-a"},
		}
		res := RunV3(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		f, ok := res.Findings[0].(report.ConfusedDeputyFinding)
		if !ok || f.Violation != report.ViolationConfusedDeputy {
			t.Fatalf("expected confused_deputy, got %+v", res.Findings[0])
		}
		if len(f.RequesterBoundTargets) != 1 || f.RequesterBoundTargets[0] != "svc-b" {
			t.Errorf("RequesterBoundTargets = %v, want [svc-b]", f.RequesterBoundTargets)
		}
	})
}

func TestSortOrderRequesterBreaksTieOnSharedActorActionRequiresTarget(t *testing.T) {
	// Two operations sharing (actor, action, requires, target) but
	// differing only by requester — requester must break the tie (§13,
	// §21).
	m := &model.ModelV3{
		Version: "3",
		Principals: []model.PrincipalV3{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Agents: []model.AgentV3{{ID: "actor"}, {ID: "z-requester"}, {ID: "a-requester"}},
		Delegations: []model.DelegationV3{
			{Delegator: "root", Delegatee: "actor", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV3{
			{Actor: "actor", Requester: "z-requester", Action: "op", Requires: "a", Target: "svc"},
			{Actor: "actor", Requester: "a-requester", Action: "op", Requires: "a", Target: "svc"},
		},
	}
	res := RunV3(m)
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
	first, ok := res.Findings[0].(report.ConfusedDeputyFinding)
	if !ok {
		t.Fatalf("expected ConfusedDeputyFinding, got %T", res.Findings[0])
	}
	second, ok := res.Findings[1].(report.ConfusedDeputyFinding)
	if !ok {
		t.Fatalf("expected ConfusedDeputyFinding, got %T", res.Findings[1])
	}
	if first.Requester != "a-requester" || second.Requester != "z-requester" {
		t.Errorf("not sorted by trailing requester: got %q then %q", first.Requester, second.Requester)
	}
}

func TestActorTraceEndsWithActionRequesterTraceDoesNot(t *testing.T) {
	res := mustLoadV3(t, "../../examples/billing-confused-deputy.json")
	for _, f := range res.Findings {
		cd, ok := f.(report.ConfusedDeputyFinding)
		if !ok {
			continue
		}
		if len(cd.ActorTrace) == 0 || cd.ActorTrace[len(cd.ActorTrace)-1] != cd.Action {
			t.Errorf("actor_trace = %v, want to end with action %q", cd.ActorTrace, cd.Action)
		}
		if len(cd.RequesterTrace) == 0 || cd.RequesterTrace[len(cd.RequesterTrace)-1] == cd.Action {
			t.Errorf("requester_trace = %v, must not end with action %q", cd.RequesterTrace, cd.Action)
		}
	}
}

func TestRunV3IsDeterministicAcrossRepeatedInvocations(t *testing.T) {
	doc, loadErr := loader.LoadDocument("../../testdata/valid-v3/combined-violations-v3.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	r1, err1 := report.RenderJSON(RunV3(doc.V3))
	r2, err2 := report.RenderJSON(RunV3(doc.V3))
	if err1 != nil || err2 != nil {
		t.Fatalf("render errors: %v, %v", err1, err2)
	}
	if string(r1) != string(r2) {
		t.Errorf("RunV3 produced different output across repeated invocations:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", r1, r2)
	}
}
