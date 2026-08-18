package verify

import (
	"reflect"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

func mustLoadV6(t *testing.T, path string) *report.Result {
	t.Helper()
	doc, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error for %s: %s", path, loadErr.RenderText())
	}
	if doc.V6 == nil {
		t.Fatalf("expected a version-6 document for %s", path)
	}
	res := RunV6(doc.V6)
	return &res
}

// lifecycleModel builds a minimal single-operation v6 model whose actor
// validly holds a capability (correctly bound, requester-backed) which
// requires approval, with exactly one lifecycle-bearing standing-backed
// approval record — the common scaffold most of these tests vary only the
// lifecycle of.
func lifecycleModel(lc *model.Lifecycle) *model.ModelV6 {
	return &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
			{ID: "officer", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV6{
			{Approver: "officer", Scope: "a", Target: "svc", Lifecycle: lc},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
}

// --- 1-3: clean pass ---

func TestCleanPassV6HasNoFindings(t *testing.T) {
	res := mustLoadV6(t, "../../testdata/valid-v6/clean-pass-v6.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestCleanPassNoLifecycleDeclaredAnywhere(t *testing.T) {
	// A v6 document with no lifecycle field anywhere behaves identically to
	// the equivalent v5 document (docs/phase-6-plan.md §16.5, §34).
	m := lifecycleModel(nil)
	res := RunV6(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestCleanPassSingleStateReachIsSafe(t *testing.T) {
	m := lifecycleModel(&model.Lifecycle{
		Initial: "approved",
		States:  []string{"approved"},
	})
	res := RunV6(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestCleanPassSelfLoopOnApprovedIsSafe(t *testing.T) {
	m := lifecycleModel(&model.Lifecycle{
		Initial: "approved",
		States:  []string{"approved"},
		Transitions: []model.LifecycleTransition{
			{From: "approved", To: "approved", Event: "reapprove"},
		},
	})
	res := RunV6(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

// --- 4-7: unsafe classification ---

func TestUnsafeSingleNonApprovedStateReachable(t *testing.T) {
	res := mustLoadV6(t, "../../testdata/valid-v6/unsafe-lifecycle.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.LifecycleFinding)
	if !ok || f.Violation != report.ViolationApprovalLifecycleUnsafe {
		t.Fatalf("expected approval_lifecycle_unsafe, got %+v", res.Findings[0])
	}
	if f.UnsafeState != "revoked" {
		t.Errorf("UnsafeState = %q, want %q", f.UnsafeState, "revoked")
	}
	wantTrace := []report.LifecycleStep{{From: "approved", Event: "revoke", To: "revoked"}}
	if !reflect.DeepEqual(f.LifecycleTrace, wantTrace) {
		t.Errorf("LifecycleTrace = %+v, want %+v", f.LifecycleTrace, wantTrace)
	}
}

func TestUnsafeMultipleNonApprovedStatesCanonicalSelection(t *testing.T) {
	// Reach = {approved, revoked, expired}; canonical selection must pick
	// the lexicographically smallest non-"approved" state: "expired".
	m := lifecycleModel(&model.Lifecycle{
		Initial: "approved",
		States:  []string{"approved", "revoked", "expired"},
		Transitions: []model.LifecycleTransition{
			{From: "approved", To: "revoked", Event: "revoke"},
			{From: "approved", To: "expired", Event: "expire"},
		},
	})
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0].(report.LifecycleFinding)
	if f.UnsafeState != "expired" {
		t.Errorf("UnsafeState = %q, want %q (lexicographically smallest of revoked/expired)", f.UnsafeState, "expired")
	}
}

func TestUnsafeInitialStateItselfNotApproved(t *testing.T) {
	m := lifecycleModel(&model.Lifecycle{
		Initial: "pending",
		States:  []string{"pending"},
	})
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0].(report.LifecycleFinding)
	if f.UnsafeState != "pending" {
		t.Errorf("UnsafeState = %q, want %q", f.UnsafeState, "pending")
	}
	if len(f.LifecycleTrace) != 0 {
		t.Errorf("LifecycleTrace should be empty (zero hops), got %+v", f.LifecycleTrace)
	}
}

func TestUnreachableStateDoesNotAffectSafety(t *testing.T) {
	// "orphan" is declared but never reachable from "approved" — it must
	// not affect the safety verdict at all.
	m := lifecycleModel(&model.Lifecycle{
		Initial: "approved",
		States:  []string{"approved", "orphan"},
	})
	res := RunV6(m)
	if len(res.Findings) != 0 {
		t.Fatalf("an unreachable declared state must not affect safety, got %d findings: %+v", len(res.Findings), res.Findings)
	}
}

// --- 26-28: fail-closed incomplete search ---

func TestFailClosedTruncationYieldsUnproven(t *testing.T) {
	orig := limits.MaxExplorationStatesPerLifecycle
	limits.MaxExplorationStatesPerLifecycle = 2
	defer func() { limits.MaxExplorationStatesPerLifecycle = orig }()

	res := mustLoadV6(t, "../../testdata/valid-v6/unproven-lifecycle.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.LifecycleFinding)
	if !ok || f.Violation != report.ViolationApprovalLifecycleUnproven {
		t.Fatalf("expected approval_lifecycle_unproven, got %+v", res.Findings[0])
	}
	if f.UnsafeState != "" {
		t.Errorf("UnsafeState should be empty for an unproven finding, got %q", f.UnsafeState)
	}
	if len(f.LifecycleTrace) != 0 {
		t.Errorf("LifecycleTrace should be empty for an unproven finding, got %+v", f.LifecycleTrace)
	}
}

func TestFailClosedNeverResolvesToAllow(t *testing.T) {
	// Under no configuration of limits.MaxExplorationStatesPerLifecycle can
	// truncation produce ALLOW.
	orig := limits.MaxExplorationStatesPerLifecycle
	defer func() { limits.MaxExplorationStatesPerLifecycle = orig }()
	for _, bound := range []int{1, 2, 3} {
		limits.MaxExplorationStatesPerLifecycle = bound
		res := mustLoadV6(t, "../../testdata/valid-v6/unproven-lifecycle.json")
		if len(res.Findings) == 0 {
			t.Fatalf("bound=%d: truncated exploration must never resolve to ALLOW", bound)
		}
	}
}

func TestSafeRecordShortCircuitsBeforeUnproven(t *testing.T) {
	// An operation with two standing approval records, one truncated
	// (unprovable) and one genuinely Safe: Safe wins outright.
	orig := limits.MaxExplorationStatesPerLifecycle
	limits.MaxExplorationStatesPerLifecycle = 2
	defer func() { limits.MaxExplorationStatesPerLifecycle = orig }()

	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
			{ID: "officer-truncated", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
			{ID: "officer-safe", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV6{
			{Approver: "officer-truncated", Scope: "a", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "pending",
				States:  []string{"pending", "step1", "step2", "approved"},
				Transitions: []model.LifecycleTransition{
					{From: "pending", To: "step1"},
					{From: "step1", To: "step2"},
					{From: "step2", To: "approved"},
				},
			}},
			{Approver: "officer-safe", Scope: "a", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "approved",
				States:  []string{"approved"},
			}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (a safe standing record must short-circuit before unproven is ever consulted), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestUnsafeWinsOverUnprovenWhenNeitherIsSafe(t *testing.T) {
	orig := limits.MaxExplorationStatesPerLifecycle
	limits.MaxExplorationStatesPerLifecycle = 2
	defer func() { limits.MaxExplorationStatesPerLifecycle = orig }()

	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
			{ID: "officer-truncated", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
			{ID: "officer-unsafe", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV6{
			{Approver: "officer-truncated", Scope: "a", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "pending",
				States:  []string{"pending", "step1", "step2", "approved"},
				Transitions: []model.LifecycleTransition{
					{From: "pending", To: "step1"},
					{From: "step1", To: "step2"},
					{From: "step2", To: "approved"},
				},
			}},
			{Approver: "officer-unsafe", Scope: "a", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "approved",
				States:  []string{"approved", "revoked"},
				Transitions: []model.LifecycleTransition{
					{From: "approved", To: "revoked", Event: "revoke"},
				},
			}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.LifecycleFinding)
	if !ok || f.Violation != report.ViolationApprovalLifecycleUnsafe {
		t.Fatalf("definitive proof (unsafe) must win over an inconclusive result (unproven), got %+v", res.Findings[0])
	}
}

// --- 29-36: Phase 1-5 interaction ---

func TestPhase1InteractionAmplificationSkipsLifecycle(t *testing.T) {
	m := &model.ModelV6{
		Version:    "6",
		Principals: []model.PrincipalV6{{ID: "root"}},
		Agents:     []model.AgentV6{{ID: "holder"}},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok || f.Violation != report.ViolationAuthorityAmplification {
		t.Errorf("expected authority_amplification, got %+v", res.Findings[0])
	}
}

func TestPhase2InteractionContextBindingSkipsLifecycle(t *testing.T) {
	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc-a", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)}}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc-b"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok || f.Violation != report.ViolationContextBinding {
		t.Errorf("expected context_binding_violation, got %+v", res.Findings[0])
	}
}

func TestPhase3InteractionConfusedDeputySkipsLifecycle(t *testing.T) {
	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)}}},
		},
		Agents: []model.AgentV6{{ID: "holder"}, {ID: "outsider"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "outsider", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	if _, ok := res.Findings[0].(report.ConfusedDeputyFinding); !ok {
		t.Errorf("expected confused_deputy, got %+v", res.Findings[0])
	}
}

func TestPhase4InteractionDepthViolationTwoFindingShape(t *testing.T) {
	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "admin", Authority: []model.RootCapabilityV5{{Scope: "billing:refund", Target: "billing-service", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)}}},
		},
		Agents: []model.AgentV6{{ID: "billing-agent"}, {ID: "support-agent"}},
		Delegations: []model.DelegationV6{
			{Delegator: "admin", Delegatee: "billing-agent", Authority: []model.Capability{{Scope: "billing:refund", Target: "billing-service"}}},
			{Delegator: "billing-agent", Delegatee: "support-agent", Authority: []model.Capability{{Scope: "billing:refund", Target: "billing-service"}}},
		},
		Operations: []model.OperationV6{
			{Actor: "support-agent", Requester: "admin", Action: "refund-deep", Requires: "billing:refund", Target: "billing-service"},
		},
	}
	res := RunV6(m)
	var sawDepth, sawAmp bool
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.DelegationDepthFinding:
			sawDepth = true
		case report.CapabilityOperationFinding:
			if v.Violation == report.ViolationAuthorityAmplification {
				sawAmp = true
			}
		}
	}
	if !sawDepth || !sawAmp {
		t.Fatalf("expected a delegation_depth_violation and a consequent authority_amplification, got %+v", res.Findings)
	}
	for _, f := range res.Findings {
		if _, ok := f.(report.LifecycleFinding); ok {
			t.Errorf("depth exhaustion must never involve lifecycle, got %+v", f)
		}
	}
}

func TestPhase5InteractionRequiresApprovalFalseUnaffectedByUnrelatedLifecycle(t *testing.T) {
	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)}}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV6{
			// An unrelated capability's unsafe lifecycle must have zero
			// bearing on this operation.
			{Approver: "root", Scope: "unrelated", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "approved",
				States:  []string{"approved", "revoked"},
				Transitions: []model.LifecycleTransition{
					{From: "approved", To: "revoked", Event: "revoke"},
				},
			}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestPhase5InteractionApprovalMissingSkipsLifecycle(t *testing.T) {
	m := lifecycleModelNoApprovals()
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok || f.Violation != report.ViolationApprovalMissing {
		t.Errorf("expected approval_missing, got %+v", res.Findings[0])
	}
}

func lifecycleModelNoApprovals() *model.ModelV6 {
	return &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)}}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
}

func TestPhase5InteractionApprovalUnauthorizedSkipsLifecycle(t *testing.T) {
	m := lifecycleModelNoApprovals()
	m.Agents = append(m.Agents, model.AgentV6{ID: "non-standing"})
	m.Approvals = []model.ApprovalV6{
		{Approver: "non-standing", Scope: "a", Target: "svc"}, // non-standing has no independent standing over "a"@svc
	}
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok || f.Violation != report.ViolationApprovalUnauthorized {
		t.Errorf("expected approval_unauthorized, got %+v", res.Findings[0])
	}
}

func TestPhase5InteractionOneUnsafeOneNoLifecycleAllows(t *testing.T) {
	res := mustLoadV6(t, "../../testdata/valid-v6/multi-approver.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (§31.5: a no-lifecycle standing record alone suffices), got %d: %+v", len(res.Findings), res.Findings)
	}
}

// --- 37: full precedence table ---

func TestFullPrecedenceTable(t *testing.T) {
	t.Run("safe lifecycle -> ALLOW", func(t *testing.T) {
		m := lifecycleModel(&model.Lifecycle{Initial: "approved", States: []string{"approved"}})
		if res := RunV6(m); len(res.Findings) != 0 {
			t.Errorf("expected 0 findings, got %+v", res.Findings)
		}
	})
	t.Run("unsafe lifecycle -> approval_lifecycle_unsafe", func(t *testing.T) {
		m := lifecycleModel(&model.Lifecycle{
			Initial: "approved", States: []string{"approved", "revoked"},
			Transitions: []model.LifecycleTransition{{From: "approved", To: "revoked", Event: "revoke"}},
		})
		res := RunV6(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %+v", res.Findings)
		}
		if f, ok := res.Findings[0].(report.LifecycleFinding); !ok || f.Violation != report.ViolationApprovalLifecycleUnsafe {
			t.Errorf("expected approval_lifecycle_unsafe, got %+v", res.Findings[0])
		}
	})
}

// --- 47-52: determinism, ordering, traces, goldens ---

func TestRunV6IsDeterministicAcrossRepeatedInvocations(t *testing.T) {
	doc, loadErr := loader.LoadDocument("../../testdata/valid-v6/combined-violations-v6.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	r1, err1 := report.RenderJSON(RunV6(doc.V6))
	r2, err2 := report.RenderJSON(RunV6(doc.V6))
	if err1 != nil || err2 != nil {
		t.Fatalf("render errors: %v, %v", err1, err2)
	}
	if string(r1) != string(r2) {
		t.Errorf("RunV6 produced different output across repeated invocations:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", r1, r2)
	}
}

func TestRunV6InputArrayPermutationInvariance(t *testing.T) {
	doc1, loadErr1 := loader.LoadDocument("../../testdata/valid-v6/clean-pass-v6.json")
	if loadErr1 != nil {
		t.Fatalf("unexpected load error: %s", loadErr1.RenderText())
	}
	doc2, loadErr2 := loader.LoadDocument("../../testdata/valid-v6/clean-pass-v6-reordered.json")
	if loadErr2 != nil {
		t.Fatalf("unexpected load error: %s", loadErr2.RenderText())
	}
	r1, _ := report.RenderJSON(RunV6(doc1.V6))
	r2, _ := report.RenderJSON(RunV6(doc2.V6))
	if string(r1) != string(r2) {
		t.Errorf("semantically-equivalent reordered v6 input produced different output:\n--- original ---\n%s\n--- reordered ---\n%s", r1, r2)
	}
}

func TestLifecycleFindingTraceEndsWithAction(t *testing.T) {
	res := mustLoadV6(t, "../../testdata/valid-v6/unsafe-lifecycle.json")
	f := res.Findings[0].(report.LifecycleFinding)
	if len(f.Trace) == 0 || f.Trace[len(f.Trace)-1] != f.Action {
		t.Errorf("Trace should end with the action, got %v", f.Trace)
	}
}

// --- 62: reapproval cycle does not retroactively restore safety ---

func TestReapprovalCycleStillUnsafe(t *testing.T) {
	// revoked -> pending -> approved is a real, legal recovery path, but
	// Safe(a) depends on set membership in Reach(L), never on whether a
	// later transition happens to lead back to "approved" — once "revoked"
	// is reachable at all, the record is unsafe, full stop.
	m := lifecycleModel(&model.Lifecycle{
		Initial: "revoked",
		States:  []string{"revoked", "pending", "approved"},
		Transitions: []model.LifecycleTransition{
			{From: "revoked", To: "pending", Event: "resubmit"},
			{From: "pending", To: "approved", Event: "approve"},
		},
	})
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.LifecycleFinding)
	if !ok || f.Violation != report.ViolationApprovalLifecycleUnsafe {
		t.Fatalf("expected approval_lifecycle_unsafe despite a declared recovery path to approved, got %+v", res.Findings[0])
	}
}

func TestCyclicLifecycleFullyExplored(t *testing.T) {
	// A genuine multi-state cycle (a -> b -> c -> a) is legal and fully
	// explorable; it is unsafe here only because none of its states is
	// named "approved".
	m := lifecycleModel(&model.Lifecycle{
		Initial: "a",
		States:  []string{"a", "b", "c"},
		Transitions: []model.LifecycleTransition{
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "c", To: "a"},
		},
	})
	res := RunV6(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0].(report.LifecycleFinding)
	if f.Violation != report.ViolationApprovalLifecycleUnsafe {
		t.Errorf("expected approval_lifecycle_unsafe, got %+v", f)
	}
}

// --- combined violations ---

func TestCombinedViolationsV6DefaultLimits(t *testing.T) {
	res := mustLoadV6(t, "../../testdata/valid-v6/combined-violations-v6.json")
	kinds := map[string]bool{}
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.CapabilityOperationFinding:
			kinds[v.Violation] = true
		case report.ConfusedDeputyFinding:
			kinds[v.Violation] = true
		case report.DelegationDepthFinding:
			kinds[v.Violation] = true
		case report.ApprovalFinding:
			kinds[v.Violation] = true
		case report.LifecycleFinding:
			kinds[v.Violation] = true
		}
	}
	wantAtLeast := []string{
		report.ViolationAuthorityAmplification,
		report.ViolationContextBinding,
		report.ViolationConfusedDeputy,
		report.ViolationDelegationDepth,
		report.ViolationApprovalMissing,
		report.ViolationApprovalUnauthorized,
		report.ViolationApprovalLifecycleUnsafe,
	}
	for _, k := range wantAtLeast {
		if !kinds[k] {
			t.Errorf("expected violation kind %q to be present, got kinds=%v", k, kinds)
		}
	}
}

func TestCombinedViolationsV6AllEightKindsUnderLoweredLimit(t *testing.T) {
	// docs/phase-6-plan.md §38 acceptance criterion 8: a version-6
	// document containing all eight violation kinds simultaneously. Under
	// this document's default-limit behavior, both lifecycle-bearing
	// operations classify as approval_lifecycle_unsafe (§22.1: exhaustion
	// is unreachable for a validate-time-legal document at the default
	// bound). Lowering limits.MaxExplorationStatesPerLifecycle to exactly 2
	// keeps the 2-state unsafe lifecycle fully explored (still unsafe)
	// while truncating the 4-state chain (unproven) — producing all eight
	// kinds in the same run, via the defense-in-depth fail-closed path
	// exercised by a lowered bound, never by a validate-time-legal document
	// at its normal bound.
	orig := limits.MaxExplorationStatesPerLifecycle
	limits.MaxExplorationStatesPerLifecycle = 2
	defer func() { limits.MaxExplorationStatesPerLifecycle = orig }()

	res := mustLoadV6(t, "../../testdata/valid-v6/combined-violations-v6.json")
	kinds := map[string]int{}
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.CapabilityOperationFinding:
			kinds[v.Violation]++
		case report.ConfusedDeputyFinding:
			kinds[v.Violation]++
		case report.DelegationDepthFinding:
			kinds[v.Violation]++
		case report.ApprovalFinding:
			kinds[v.Violation]++
		case report.LifecycleFinding:
			kinds[v.Violation]++
		}
	}
	want := []string{
		report.ViolationAuthorityAmplification,
		report.ViolationContextBinding,
		report.ViolationConfusedDeputy,
		report.ViolationDelegationDepth,
		report.ViolationApprovalMissing,
		report.ViolationApprovalUnauthorized,
		report.ViolationApprovalLifecycleUnsafe,
		report.ViolationApprovalLifecycleUnproven,
	}
	for _, k := range want {
		if kinds[k] == 0 {
			t.Errorf("expected violation kind %q to be present, got kinds=%v", k, kinds)
		}
	}
	if len(kinds) != 8 {
		t.Errorf("expected exactly 8 distinct violation kinds, got %d: %v", len(kinds), kinds)
	}
}

func TestLifecycleFindingSortOrderWithSharedActorAction(t *testing.T) {
	// docs/phase-6-plan.md §33 test 47: multiple LifecycleFindings sharing
	// (actor, action) but differing only by requires/target, sorted by the
	// existing 6-tuple key.
	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
				{Scope: "b", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
			{ID: "officer", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
				{Scope: "b", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV6{{ID: "holder"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{
				{Scope: "a", Target: "svc"}, {Scope: "b", Target: "svc"},
			}},
		},
		Approvals: []model.ApprovalV6{
			{Approver: "officer", Scope: "a", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "approved", States: []string{"approved", "revoked"},
				Transitions: []model.LifecycleTransition{{From: "approved", To: "revoked", Event: "revoke"}},
			}},
			{Approver: "officer", Scope: "b", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "approved", States: []string{"approved", "revoked"},
				Transitions: []model.LifecycleTransition{{From: "approved", To: "revoked", Event: "revoke"}},
			}},
		},
		Operations: []model.OperationV6{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "b", Target: "svc"},
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV6(m)
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
	first, ok := res.Findings[0].(report.LifecycleFinding)
	if !ok {
		t.Fatalf("expected LifecycleFinding, got %T", res.Findings[0])
	}
	second, ok := res.Findings[1].(report.LifecycleFinding)
	if !ok {
		t.Fatalf("expected LifecycleFinding, got %T", res.Findings[1])
	}
	if first.Requires.Scope != "a" || second.Requires.Scope != "b" {
		t.Errorf("not sorted ascending by scope: got %q then %q", first.Requires.Scope, second.Requires.Scope)
	}
}

func TestHostileDenseAutomatonAtFullBoundsCompletesWithoutPanic(t *testing.T) {
	// docs/phase-6-plan.md §33 test 57: a lifecycle whose transitions array
	// declares a dense, near-complete graph up to MaxLifecycleTransitions
	// still completes Explore well within the runtime ceiling, at the
	// project's real, non-lowered default bounds — no timeout, no panic.
	states := make([]string, limits.MaxLifecycleStates)
	states[0] = "approved"
	for i := 1; i < len(states); i++ {
		states[i] = "s" + string(rune('a'+i%26)) + string(rune('A'+(i/26)%26))
	}
	var transitions []model.LifecycleTransition
	events := []string{"e1", "e2", "e3", "e4"}
	for i := 0; i < len(states) && len(transitions) < limits.MaxLifecycleTransitions; i++ {
		for j := 0; j < len(states) && len(transitions) < limits.MaxLifecycleTransitions; j++ {
			if i == j {
				continue
			}
			transitions = append(transitions, model.LifecycleTransition{
				From: states[i], To: states[j], Event: events[len(transitions)%len(events)],
			})
		}
	}
	if len(transitions) > limits.MaxLifecycleTransitions {
		transitions = transitions[:limits.MaxLifecycleTransitions]
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RunV6 panicked on a dense, near-complete automaton at full bounds: %v", r)
		}
	}()
	m := lifecycleModel(&model.Lifecycle{Initial: "approved", States: states, Transitions: transitions})
	_ = RunV6(m) // must complete; outcome (safe/unsafe) is not the point of this test
}

func TestLifecycleStatesAndTransitionsReorderingInvariance(t *testing.T) {
	// docs/phase-6-plan.md §33 test 49: reordering lifecycle.states/
	// lifecycle.transitions within one approval record must never change
	// output.
	forward := lifecycleModel(&model.Lifecycle{
		Initial: "approved",
		States:  []string{"approved", "revoked", "expired"},
		Transitions: []model.LifecycleTransition{
			{From: "approved", To: "revoked", Event: "revoke"},
			{From: "approved", To: "expired", Event: "expire"},
		},
	})
	reversed := lifecycleModel(&model.Lifecycle{
		Initial: "approved",
		States:  []string{"expired", "revoked", "approved"},
		Transitions: []model.LifecycleTransition{
			{From: "approved", To: "expired", Event: "expire"},
			{From: "approved", To: "revoked", Event: "revoke"},
		},
	})
	r1, _ := report.RenderJSON(RunV6(forward))
	r2, _ := report.RenderJSON(RunV6(reversed))
	if string(r1) != string(r2) {
		t.Errorf("reordering lifecycle.states/lifecycle.transitions produced different output:\n--- forward ---\n%s\n--- reversed ---\n%s", r1, r2)
	}
}

// --- edge-level distrust: lifecycle never gates delegation ---

func TestLifecycleGatesExerciseNotDelegation(t *testing.T) {
	m := &model.ModelV6{
		Version: "6",
		Principals: []model.PrincipalV6{
			{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(2), RequiresApproval: boolPtr(true)}}},
		},
		Agents: []model.AgentV6{{ID: "mid"}, {ID: "deep"}},
		Delegations: []model.DelegationV6{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "mid", Delegatee: "deep", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV6{
			{Approver: "root", Scope: "a", Target: "svc", Lifecycle: &model.Lifecycle{
				Initial: "approved", States: []string{"approved", "revoked"},
				Transitions: []model.LifecycleTransition{{From: "approved", To: "revoked", Event: "revoke"}},
			}},
		},
	}
	res := RunV6(m)
	for _, f := range res.Findings {
		if lf, ok := f.(report.LifecycleFinding); ok {
			t.Errorf("lifecycle must never produce an edge-level finding, got %+v", lf)
		}
	}
}
