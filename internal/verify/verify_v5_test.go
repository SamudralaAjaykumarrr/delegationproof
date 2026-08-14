package verify

import (
	"reflect"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

func boolPtr(v bool) *bool { return &v }

func mustLoadV5(t *testing.T, path string) *report.Result {
	t.Helper()
	doc, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error for %s: %s", path, loadErr.RenderText())
	}
	if doc.V5 == nil {
		t.Fatalf("expected a version-5 document for %s", path)
	}
	res := RunV5(doc.V5)
	return &res
}

func TestCleanPassV5HasNoFindings(t *testing.T) {
	res := mustLoadV5(t, "../../testdata/valid-v5/clean-pass-v5.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestBillingApprovalExample(t *testing.T) {
	// examples/billing-approval.json (docs/phase-5-plan.md §22): exactly
	// the two-operation shape — refund-approved passes (standing-backed
	// approval exists), void-unapproved fails with approval_missing.
	res := mustLoadV5(t, "../../examples/billing-approval.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok {
		t.Fatalf("expected an ApprovalFinding, got %T", res.Findings[0])
	}
	want := report.ApprovalFinding{
		Violation:         "approval_missing",
		Point:             "operation",
		Actor:             "billing-agent",
		Requester:         "admin",
		Action:            "void-unapproved",
		Requires:          report.Capability{Scope: "billing:void", Target: "billing-service"},
		DeclaredApprovers: []string{},
		Trace:             []string{"admin", "billing-agent", "void-unapproved"},
		Reason:            "void-unapproved requires billing:void@billing-service, which billing-agent validly holds and admin is authorized to request, but billing:void@billing-service requires approval and no approval has been declared for it",
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("finding = %+v, want %+v", f, want)
	}
}

func TestBillingApprovalExampleApprovedOperationPasses(t *testing.T) {
	res := mustLoadV5(t, "../../examples/billing-approval.json")
	for _, f := range res.Findings {
		if af, ok := f.(report.ApprovalFinding); ok && af.Action == "refund-approved" {
			t.Fatalf("refund-approved must not produce a finding (standing-backed approval exists), got %+v", af)
		}
	}
}

func TestApprovalUnauthorizedFocusedViolation(t *testing.T) {
	res := mustLoadV5(t, "../../testdata/valid-v5/approval-unauthorized.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok || f.Violation != report.ViolationApprovalUnauthorized {
		t.Fatalf("expected approval_unauthorized, got %+v", res.Findings[0])
	}
	if !reflect.DeepEqual(f.DeclaredApprovers, []string{"intern"}) {
		t.Errorf("DeclaredApprovers = %v, want [intern]", f.DeclaredApprovers)
	}
}

func TestRequiresApprovalFalseIsUnaffectedByPhase5(t *testing.T) {
	// §25 item 6: a capability validly held, correctly bound,
	// requester-backed, requires_approval: false, zero approval records in
	// the document at all -> passes.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: nil,
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestMultipleStandingApproversBothPass(t *testing.T) {
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
			{ID: "approver-1", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
			{ID: "approver-2", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV5{
			{Approver: "approver-1", Scope: "a", Target: "svc"},
			{Approver: "approver-2", Scope: "a", Target: "svc"},
		},
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (existential quantification, §10.2), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestMixedStandingAndNonStandingOneSufficient(t *testing.T) {
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
			{ID: "standing-approver", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}, {ID: "non-standing-1"}, {ID: "non-standing-2"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV5{
			{Approver: "non-standing-1", Scope: "a", Target: "svc"},
			{Approver: "standing-approver", Scope: "a", Target: "svc"},
			{Approver: "non-standing-2", Scope: "a", Target: "svc"},
		},
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (§11 strict distrust: non-standing records contribute nothing but don't block a valid one), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestAllApproversNonStanding(t *testing.T) {
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}, {ID: "non-standing-1"}, {ID: "non-standing-2"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV5{
			{Approver: "non-standing-2", Scope: "a", Target: "svc"},
			{Approver: "non-standing-1", Scope: "a", Target: "svc"},
		},
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok || f.Violation != report.ViolationApprovalUnauthorized {
		t.Fatalf("expected approval_unauthorized, got %+v", res.Findings[0])
	}
	want := []string{"non-standing-1", "non-standing-2"}
	if !reflect.DeepEqual(f.DeclaredApprovers, want) {
		t.Errorf("DeclaredApprovers = %v, want %v (sorted, deduplicated)", f.DeclaredApprovers, want)
	}
}

func TestMultiPathApprovalORSemantics(t *testing.T) {
	// §10.1's worked scenario as a fixture: a more-restrictive root and a
	// more-permissive root both deliver the same capability to a shared
	// downstream node. requiresApproval must adopt true (the OR of both
	// paths), independent of which path wins the remaining-budget contest
	// (the permissive admin-b path, which delivers more remaining budget).
	res := mustLoadV5(t, "../../testdata/valid-v5/multi-path-approval.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding (approval_missing, proving OR-aggregation adopted), got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok || f.Violation != report.ViolationApprovalMissing {
		t.Fatalf("expected approval_missing, got %+v", res.Findings[0])
	}
}

func TestMultiPathBothAgreeTrue(t *testing.T) {
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "admin-a", Authority: []model.RootCapabilityV5{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)}}},
			{ID: "admin-b", Authority: []model.RootCapabilityV5{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(3), RequiresApproval: boolPtr(true)}}},
		},
		Agents: []model.AgentV5{{ID: "x"}, {ID: "y"}},
		Delegations: []model.DelegationV5{
			{Delegator: "admin-a", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "admin-b", Delegatee: "y", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "y", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
		},
		Operations: []model.OperationV5{
			{Actor: "x", Requester: "admin-a", Action: "use", Requires: "c", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding (both paths agree true -> still requires approval), got %d: %+v", len(res.Findings), res.Findings)
	}
	if _, ok := res.Findings[0].(report.ApprovalFinding); !ok {
		t.Fatalf("expected an ApprovalFinding, got %T", res.Findings[0])
	}
}

func TestMultiPathBothAgreeFalse(t *testing.T) {
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "admin-a", Authority: []model.RootCapabilityV5{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)}}},
			{ID: "admin-b", Authority: []model.RootCapabilityV5{{Scope: "c", Target: "svc", MaxDelegationDepth: intPtr(3), RequiresApproval: boolPtr(false)}}},
		},
		Agents: []model.AgentV5{{ID: "x"}, {ID: "y"}},
		Delegations: []model.DelegationV5{
			{Delegator: "admin-a", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "admin-b", Delegatee: "y", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
			{Delegator: "y", Delegatee: "x", Authority: []model.Capability{{Scope: "c", Target: "svc"}}},
		},
		Operations: []model.OperationV5{
			{Actor: "x", Requester: "admin-a", Action: "use", Requires: "c", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (both paths agree false -> OR degenerates to false), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestApproverReachedViaMultiHopChain(t *testing.T) {
	// §25 item 12: the approver's standing must reflect
	// flattenApproval(da[approver]) for a non-principal approver reached
	// via a multi-hop delegation chain, not just an axiomatic declaration.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(3), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}, {ID: "mid-approver"}, {ID: "deep-approver"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "root", Delegatee: "mid-approver", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "mid-approver", Delegatee: "deep-approver", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV5{
			{Approver: "deep-approver", Scope: "a", Target: "svc"},
		},
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings (deep-approver's derived standing must count), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestApproverOnlyStandingViaInvalidEdgeYieldsUnauthorized(t *testing.T) {
	// §25 item 13: an approver whose only apparent standing arrives via a
	// presence/binding-failed edge never actually held the capability.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}, {ID: "fake-approver"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			// fake-approver attempts to receive a@svc from holder, but
			// holder's remaining budget is already 0 (depth-exhausted) --
			// this edge is entirely invalid, so fake-approver never
			// actually holds a@svc.
			{Delegator: "holder", Delegatee: "fake-approver", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals: []model.ApprovalV5{
			{Approver: "fake-approver", Scope: "a", Target: "svc"},
		},
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	var sawApprovalUnauthorized, sawDepth bool
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.ApprovalFinding:
			if v.Violation == report.ViolationApprovalUnauthorized {
				sawApprovalUnauthorized = true
			}
		case report.DelegationDepthFinding:
			sawDepth = true
		}
	}
	if !sawApprovalUnauthorized {
		t.Errorf("expected approval_unauthorized (fake-approver never held the capability), got %+v", res.Findings)
	}
	if !sawDepth {
		t.Errorf("expected a delegation_depth_violation for the poisoned holder->fake-approver edge, got %+v", res.Findings)
	}
}

func TestApprovalGatesExerciseNotDelegation(t *testing.T) {
	// §4.2: an approval-required capability may still be freely delegated
	// (subject to unchanged presence/binding/depth rules) — there is no
	// edge-level finding produced merely because requires_approval is true
	// and no approval has been declared.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(2), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "mid"}, {ID: "deep"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "mid", Delegatee: "deep", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Approvals:  nil,
		Operations: nil,
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("delegating an approval-required capability with no operations must produce no findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestNoApprovalRelatedEdgeFinding(t *testing.T) {
	// §4.2, §11: approval is never an edge-level finding kind. Scan the
	// combined-violations fixture (which includes approval findings) and
	// confirm every DelegationDepthFinding/CapabilityEdgeFinding is a
	// pre-existing kind, and no ApprovalFinding ever has
	// Point == "delegation_edge".
	res := mustLoadV5(t, "../../testdata/valid-v5/combined-violations-v5.json")
	for _, f := range res.Findings {
		if af, ok := f.(report.ApprovalFinding); ok && af.Point != report.PointOperation {
			t.Errorf("ApprovalFinding must always be point=operation, got %+v", af)
		}
	}
}

func TestApprovalPrecedenceTable(t *testing.T) {
	// §12: dedicated table test covering every precedence-table row
	// mentioning approval — actor-amplification masks a co-declared
	// approval issue, confused-deputy masks a co-declared approval issue,
	// and the genuine approval_missing/approval_unauthorized rows.
	t.Run("actor lacks capability entirely -> authority_amplification, approval not evaluated", func(t *testing.T) {
		m := &model.ModelV5{
			Version:    "5",
			Principals: []model.PrincipalV5{{ID: "root", Authority: nil}},
			Agents:     []model.AgentV5{{ID: "holder"}},
			Operations: []model.OperationV5{
				{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
			},
		}
		res := RunV5(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		f, ok := res.Findings[0].(report.CapabilityOperationFinding)
		if !ok || f.Violation != report.ViolationAuthorityAmplification {
			t.Errorf("expected authority_amplification, got %+v", res.Findings[0])
		}
	})

	t.Run("actor holds wrong target -> context_binding_violation, approval not evaluated", func(t *testing.T) {
		m := &model.ModelV5{
			Version: "5",
			Principals: []model.PrincipalV5{
				{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc-a", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)}}},
			},
			Agents: []model.AgentV5{{ID: "holder"}},
			Delegations: []model.DelegationV5{
				{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
			},
			Operations: []model.OperationV5{
				{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc-b"},
			},
		}
		res := RunV5(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		f, ok := res.Findings[0].(report.CapabilityOperationFinding)
		if !ok || f.Violation != report.ViolationContextBinding {
			t.Errorf("expected context_binding_violation, got %+v", res.Findings[0])
		}
	})

	t.Run("requester lacks standing -> confused_deputy, approval not evaluated", func(t *testing.T) {
		m := &model.ModelV5{
			Version: "5",
			Principals: []model.PrincipalV5{
				{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)}}},
			},
			Agents: []model.AgentV5{{ID: "holder"}, {ID: "outsider"}},
			Delegations: []model.DelegationV5{
				{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			},
			Operations: []model.OperationV5{
				{Actor: "holder", Requester: "outsider", Action: "use", Requires: "a", Target: "svc"},
			},
		}
		res := RunV5(m)
		if len(res.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %+v", len(res.Findings), res.Findings)
		}
		if _, ok := res.Findings[0].(report.ConfusedDeputyFinding); !ok {
			t.Errorf("expected confused_deputy, got %+v", res.Findings[0])
		}
	})

	t.Run("all pass, no approval required -> ALLOW", func(t *testing.T) {
		m := &model.ModelV5{
			Version: "5",
			Principals: []model.PrincipalV5{
				{ID: "root", Authority: []model.RootCapabilityV5{{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)}}},
			},
			Agents: []model.AgentV5{{ID: "holder"}},
			Delegations: []model.DelegationV5{
				{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			},
			Operations: []model.OperationV5{
				{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
			},
		}
		res := RunV5(m)
		if len(res.Findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
		}
	})
}

func TestDepthAndApprovalIndependence(t *testing.T) {
	// §25 item 23: an unrelated capability's edge fails
	// delegation_depth_violation while a different, unrelated capability's
	// operation independently fails approval_missing — both present,
	// correctly ordered, no interference.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "depth-capped", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)},
				{Scope: "approval-capped", Target: "svc", MaxDelegationDepth: intPtr(5), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "mid"}, {ID: "next"}, {ID: "holder"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{{Scope: "depth-capped", Target: "svc"}}},
			{Delegator: "mid", Delegatee: "next", Authority: []model.Capability{{Scope: "depth-capped", Target: "svc"}}},
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "approval-capped", Target: "svc"}}},
		},
		Approvals: nil,
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use-approval-capped", Requires: "approval-capped", Target: "svc"},
		},
	}
	res := RunV5(m)
	var sawDepth, sawApproval bool
	for _, f := range res.Findings {
		switch v := f.(type) {
		case report.DelegationDepthFinding:
			sawDepth = true
		case report.ApprovalFinding:
			sawApproval = true
			if v.Violation != report.ViolationApprovalMissing {
				t.Errorf("expected approval_missing, got %+v", v)
			}
		}
	}
	if !sawDepth || !sawApproval {
		t.Fatalf("expected both a delegation_depth_violation and an approval_missing finding, got %+v", res.Findings)
	}
}

func TestCombinedViolationsV5(t *testing.T) {
	res := mustLoadV5(t, "../../testdata/valid-v5/combined-violations-v5.json")
	if len(res.Findings) != 8 {
		t.Fatalf("expected exactly 8 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
	var sawAmp, sawCD, sawCtx, sawDepth, sawApprovalMissing, sawApprovalUnauthorized int
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
		case report.ApprovalFinding:
			switch v.Violation {
			case report.ViolationApprovalMissing:
				sawApprovalMissing++
			case report.ViolationApprovalUnauthorized:
				sawApprovalUnauthorized++
			}
		}
	}
	if sawAmp != 2 || sawCD != 2 || sawCtx != 1 || sawDepth != 1 || sawApprovalMissing != 1 || sawApprovalUnauthorized != 1 {
		t.Errorf("finding mix = amp:%d cd:%d ctx:%d depth:%d approval_missing:%d approval_unauthorized:%d, want 2,2,1,1,1,1",
			sawAmp, sawCD, sawCtx, sawDepth, sawApprovalMissing, sawApprovalUnauthorized)
	}
}

func TestApprovalFindingSortOrderWithSharedActorAction(t *testing.T) {
	// §25 item 25: multiple ApprovalFindings sharing (actor, action) but
	// differing by requires/target/requester, sorted by the existing
	// 6-tuple key.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
				{Scope: "b", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
			}},
		},
		Agents: []model.AgentV5{{ID: "holder"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{
				{Scope: "a", Target: "svc"}, {Scope: "b", Target: "svc"},
			}},
		},
		Operations: []model.OperationV5{
			{Actor: "holder", Requester: "root", Action: "use", Requires: "b", Target: "svc"},
			{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
	first, ok := res.Findings[0].(report.ApprovalFinding)
	if !ok {
		t.Fatalf("expected ApprovalFinding, got %T", res.Findings[0])
	}
	second, ok := res.Findings[1].(report.ApprovalFinding)
	if !ok {
		t.Fatalf("expected ApprovalFinding, got %T", res.Findings[1])
	}
	if first.Requires.Scope != "a" || second.Requires.Scope != "b" {
		t.Errorf("not sorted ascending by scope: got %q then %q", first.Requires.Scope, second.Requires.Scope)
	}
}

func TestApprovalFindingTraceUsesCanonicalTraceConvention(t *testing.T) {
	res := mustLoadV5(t, "../../examples/billing-approval.json")
	for _, f := range res.Findings {
		af, ok := f.(report.ApprovalFinding)
		if !ok {
			continue
		}
		want := []string{"admin", "billing-agent", "void-unapproved"}
		if !reflect.DeepEqual(af.Trace, want) {
			t.Errorf("Trace = %v, want %v", af.Trace, want)
		}
	}
}

func TestRunV5IsDeterministicAcrossRepeatedInvocations(t *testing.T) {
	doc, loadErr := loader.LoadDocument("../../testdata/valid-v5/combined-violations-v5.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	r1, err1 := report.RenderJSON(RunV5(doc.V5))
	r2, err2 := report.RenderJSON(RunV5(doc.V5))
	if err1 != nil || err2 != nil {
		t.Fatalf("render errors: %v, %v", err1, err2)
	}
	if string(r1) != string(r2) {
		t.Errorf("RunV5 produced different output across repeated invocations:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", r1, r2)
	}
}

func TestRunV5InputArrayPermutationInvariance(t *testing.T) {
	doc1, loadErr1 := loader.LoadDocument("../../testdata/valid-v5/clean-pass-v5.json")
	if loadErr1 != nil {
		t.Fatalf("unexpected load error: %s", loadErr1.RenderText())
	}
	doc2, loadErr2 := loader.LoadDocument("../../testdata/valid-v5/clean-pass-v5-reordered.json")
	if loadErr2 != nil {
		t.Fatalf("unexpected load error: %s", loadErr2.RenderText())
	}
	r1, _ := report.RenderJSON(RunV5(doc1.V5))
	r2, _ := report.RenderJSON(RunV5(doc2.V5))
	if string(r1) != string(r2) {
		t.Errorf("semantically-equivalent reordered v5 input produced different output:\n--- original ---\n%s\n--- reordered ---\n%s", r1, r2)
	}
}

func TestReorderedApprovalsArrayInvariance(t *testing.T) {
	// §10.2, §25 item 26: the sort+dedupe design over approvals[] is
	// genuinely order-independent — reordering multiple approval records
	// for the same capability must not change the result.
	build := func(approvals []model.ApprovalV5) *model.ModelV5 {
		return &model.ModelV5{
			Version: "5",
			Principals: []model.PrincipalV5{
				{ID: "root", Authority: []model.RootCapabilityV5{
					{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(true)},
				}},
				{ID: "approver-x", Authority: []model.RootCapabilityV5{
					{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(0), RequiresApproval: boolPtr(true)},
				}},
			},
			Agents: []model.AgentV5{{ID: "holder"}, {ID: "non-standing-1"}, {ID: "non-standing-2"}},
			Delegations: []model.DelegationV5{
				{Delegator: "root", Delegatee: "holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			},
			Approvals: approvals,
			Operations: []model.OperationV5{
				{Actor: "holder", Requester: "root", Action: "use", Requires: "a", Target: "svc"},
			},
		}
	}
	forward := build([]model.ApprovalV5{
		{Approver: "non-standing-1", Scope: "a", Target: "svc"},
		{Approver: "approver-x", Scope: "a", Target: "svc"},
		{Approver: "non-standing-2", Scope: "a", Target: "svc"},
	})
	reversed := build([]model.ApprovalV5{
		{Approver: "non-standing-2", Scope: "a", Target: "svc"},
		{Approver: "approver-x", Scope: "a", Target: "svc"},
		{Approver: "non-standing-1", Scope: "a", Target: "svc"},
	})
	r1, _ := report.RenderJSON(RunV5(forward))
	r2, _ := report.RenderJSON(RunV5(reversed))
	if string(r1) != string(r2) {
		t.Errorf("reordered approvals[] produced different output:\n--- forward ---\n%s\n--- reversed ---\n%s", r1, r2)
	}
	if len(RunV5(forward).Findings) != 0 {
		t.Fatalf("expected 0 findings (approver-x is standing-backed regardless of array order)")
	}
}

func TestRequesterUsageDoesNotRequireApprovalOfItsOwn(t *testing.T) {
	// §13: the requester's own authState.requiresApproval is never
	// consulted — only the actor's copy governs the gate.
	m := &model.ModelV5{
		Version: "5",
		Principals: []model.PrincipalV5{
			{ID: "root", Authority: []model.RootCapabilityV5{
				{Scope: "a", Target: "svc", MaxDelegationDepth: intPtr(1), RequiresApproval: boolPtr(false)},
			}},
		},
		Agents: []model.AgentV5{{ID: "actor"}, {ID: "requester-holder"}},
		Delegations: []model.DelegationV5{
			{Delegator: "root", Delegatee: "actor", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "root", Delegatee: "requester-holder", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Operations: []model.OperationV5{
			{Actor: "actor", Requester: "requester-holder", Action: "use", Requires: "a", Target: "svc"},
		},
	}
	res := RunV5(m)
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}
