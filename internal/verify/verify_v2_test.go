package verify

import (
	"reflect"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

func mustLoadV2(t *testing.T, path string) *report.Result {
	t.Helper()
	doc, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error for %s: %s", path, loadErr.RenderText())
	}
	if doc.V2 == nil {
		t.Fatalf("expected a version-2 document for %s", path)
	}
	res := RunV2(doc.V2)
	return &res
}

func TestCleanPassV2HasNoFindings(t *testing.T) {
	res := mustLoadV2(t, "../../testdata/valid-v2/clean-pass-v2.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestBillingContextBindingExample(t *testing.T) {
	res := mustLoadV2(t, "../../examples/billing-context-binding.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok {
		t.Fatalf("expected a CapabilityOperationFinding, got %T", res.Findings[0])
	}
	want := report.CapabilityOperationFinding{
		Violation:    "context_binding_violation",
		Point:        "operation",
		Actor:        "billing-agent",
		Action:       "read-record",
		Requires:     report.Capability{Scope: "billing:read", Target: "payroll-service"},
		Held:         []report.Capability{{Scope: "billing:read", Target: "billing-service"}},
		BoundTargets: []string{"billing-service"},
		Trace:        []string{"user", "billing-agent", "read-record"},
		Reason:       "billing:read is held by billing-agent only for billing-service, which does not include payroll-service",
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("finding = %+v, want %+v", f, want)
	}
}

func TestCombinedViolations(t *testing.T) {
	res := mustLoadV2(t, "../../testdata/valid-v2/combined-violations.json")
	if len(res.Findings) != 2 {
		t.Fatalf("expected exactly 2 findings, got %d: %+v", len(res.Findings), res.Findings)
	}

	amp, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok || amp.Violation != report.ViolationAuthorityAmplification {
		t.Fatalf("expected first finding to be authority_amplification, got %+v", res.Findings[0])
	}
	if amp.Action != "op-never-held" {
		t.Errorf("amplification finding action = %q, want op-never-held", amp.Action)
	}

	cbv, ok := res.Findings[1].(report.CapabilityOperationFinding)
	if !ok || cbv.Violation != report.ViolationContextBinding {
		t.Fatalf("expected second finding to be context_binding_violation, got %+v", res.Findings[1])
	}
	if cbv.Action != "op-wrong-target" {
		t.Errorf("context-binding finding action = %q, want op-wrong-target", cbv.Action)
	}
	if len(cbv.BoundTargets) != 1 || cbv.BoundTargets[0] != "svc-a" {
		t.Errorf("bound targets = %v, want [svc-a]", cbv.BoundTargets)
	}
}

func TestMultiHopContextPropagation(t *testing.T) {
	res := mustLoadV2(t, "../../testdata/valid-v2/multi-hop-context-binding.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding (only the leaf's wrong-target op), got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.CapabilityOperationFinding)
	if !ok {
		t.Fatalf("expected a CapabilityOperationFinding, got %T", res.Findings[0])
	}
	if f.Violation != report.ViolationContextBinding {
		t.Errorf("violation = %q, want context_binding_violation", f.Violation)
	}
	if f.Action != "read-bad" {
		t.Errorf("action = %q, want read-bad", f.Action)
	}
	wantTrace := []string{"root", "hop1", "hop2", "leaf", "read-bad"}
	if !reflect.DeepEqual(f.Trace, wantTrace) {
		t.Errorf("trace = %v, want %v", f.Trace, wantTrace)
	}
}

func TestStrictDistrustNoPartialCreditV2(t *testing.T) {
	// root holds a@svc only. mid receives a valid grant of {a@svc} from
	// root. target receives ONE incoming edge from mid granting
	// {a@svc, c@svc} — c@svc is not in DA(mid), so the whole edge must be
	// distrusted, including the overlapping a@svc. DA(target) must
	// therefore be empty, not {a@svc}.
	m := &model.ModelV2{
		Version: "2",
		Principals: []model.PrincipalV2{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
		},
		Agents: []model.AgentV2{
			{ID: "mid"},
			{ID: "target"},
		},
		Delegations: []model.DelegationV2{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{{Scope: "a", Target: "svc"}}},
			{Delegator: "mid", Delegatee: "target", Authority: []model.Capability{
				{Scope: "a", Target: "svc"},
				{Scope: "c", Target: "svc"},
			}},
		},
		Operations: []model.OperationV2{
			{Actor: "target", Action: "op", Requires: "a", Target: "svc"},
		},
	}

	res := RunV2(m)
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings (1 edge + 1 operation), got %d: %+v", len(res.Findings), res.Findings)
	}

	edge, ok := res.Findings[0].(report.CapabilityEdgeFinding)
	if !ok || edge.Delegator != "mid" || edge.Delegatee != "target" {
		t.Fatalf("expected edge-level finding mid->target first, got %+v", res.Findings[0])
	}
	if edge.Violation != report.ViolationAuthorityAmplification {
		t.Errorf("edge violation = %q, want authority_amplification (c is never held under any target)", edge.Violation)
	}
	wantExcess := []report.Capability{{Scope: "c", Target: "svc"}}
	if !reflect.DeepEqual(edge.Excess, wantExcess) {
		t.Errorf("excess = %v, want %v", edge.Excess, wantExcess)
	}

	op, ok := res.Findings[1].(report.CapabilityOperationFinding)
	if !ok {
		t.Fatalf("expected operation-level finding second, got %+v", res.Findings[1])
	}
	if len(op.Held) != 0 {
		t.Errorf("held = %v, want empty — invalid edge must contribute nothing to DA(target)", op.Held)
	}
}

func TestPrecedenceRuleAmplificationWinsOverContextBinding(t *testing.T) {
	// delegator holds a@svc-a only (not b, under any target). An edge
	// grants {a@svc-b, b@svc-a}: a@svc-b is a pure context mismatch
	// (a IS held, just for a different target), but b@svc-a is a true
	// amplification (b is never held at all). Per §8's precedence rule,
	// the single finding covering this whole excess set must be
	// classified authority_amplification, not context_binding_violation.
	m := &model.ModelV2{
		Version: "2",
		Principals: []model.PrincipalV2{
			{ID: "root", Authority: []model.Capability{{Scope: "a", Target: "svc-a"}}},
		},
		Agents: []model.AgentV2{
			{ID: "mid"},
		},
		Delegations: []model.DelegationV2{
			{Delegator: "root", Delegatee: "mid", Authority: []model.Capability{
				{Scope: "a", Target: "svc-b"},
				{Scope: "b", Target: "svc-a"},
			}},
		},
	}

	res := RunV2(m)
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	edge, ok := res.Findings[0].(report.CapabilityEdgeFinding)
	if !ok {
		t.Fatalf("expected a CapabilityEdgeFinding, got %T", res.Findings[0])
	}
	if edge.Violation != report.ViolationAuthorityAmplification {
		t.Errorf("violation = %q, want authority_amplification (precedence rule: any pure-amplification capability in the excess set wins)", edge.Violation)
	}
	if len(edge.BoundTargets) != 0 {
		t.Errorf("bound_targets = %v, want empty for an authority_amplification finding", edge.BoundTargets)
	}
}

func TestRunV2IsDeterministicAcrossRepeatedInvocations(t *testing.T) {
	doc, loadErr := loader.LoadDocument("../../testdata/valid-v2/combined-violations.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	r1, err1 := report.RenderJSON(RunV2(doc.V2))
	r2, err2 := report.RenderJSON(RunV2(doc.V2))
	if err1 != nil || err2 != nil {
		t.Fatalf("render errors: %v, %v", err1, err2)
	}
	if string(r1) != string(r2) {
		t.Errorf("RunV2 produced different output across repeated invocations:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", r1, r2)
	}
}
