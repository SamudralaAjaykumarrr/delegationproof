package verify

import (
	"reflect"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
)

func mustLoad(t *testing.T, path string) *report.Result {
	t.Helper()
	m, loadErr := loader.Load(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error for %s: %s", path, loadErr.RenderText())
	}
	res := Run(m)
	return &res
}

func TestCleanPassHasNoFindings(t *testing.T) {
	res := mustLoad(t, "../../testdata/valid/clean-pass.json")
	if len(res.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestBillingRefundExample(t *testing.T) {
	res := mustLoad(t, "../../examples/billing-refund.json")
	if len(res.Findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(res.Findings), res.Findings)
	}
	f, ok := res.Findings[0].(report.OperationFinding)
	if !ok {
		t.Fatalf("expected an OperationFinding, got %T", res.Findings[0])
	}
	want := report.OperationFinding{
		Violation: "authority_amplification",
		Point:     "operation",
		Actor:     "agent-b",
		Action:    "billing.refund",
		Requires:  "billing:write",
		Held:      []string{"billing:read"},
		Trace:     []string{"user", "agent-a", "agent-b", "billing.refund"},
		Reason:    "billing:write was never present in the valid delegation chain reaching agent-b",
	}
	if !reflect.DeepEqual(f, want) {
		t.Errorf("finding = %+v, want %+v", f, want)
	}
}

func TestEdgeLevelAmplification(t *testing.T) {
	res := mustLoad(t, "../../testdata/valid/mixed-violations.json")
	var edge *report.EdgeFinding
	for _, f := range res.Findings {
		if ef, ok := f.(report.EdgeFinding); ok {
			ef := ef
			edge = &ef
		}
	}
	if edge == nil {
		t.Fatal("expected an EdgeFinding among the results")
	}
	want := report.EdgeFinding{
		Violation: "authority_amplification",
		Point:     "delegation_edge",
		Delegator: "mid",
		Delegatee: "over",
		Declared:  []string{"a:2"},
		Excess:    []string{"a:2"},
		Trace:     []string{"root", "mid", "over"},
		Reason:    "mid attempted to delegate a:2, which is not in mid's derived authority",
	}
	if !reflect.DeepEqual(*edge, want) {
		t.Errorf("finding = %+v, want %+v", *edge, want)
	}
}

func TestStrictDistrustNoPartialCredit(t *testing.T) {
	// root has authority [a]. mid receives a valid grant of [a] from root.
	// target receives ONE incoming edge from mid granting [a, c] — c is
	// not in DA(mid), so the whole edge must be distrusted, including the
	// overlapping "a". DA(target) must therefore be empty, not {a}, so an
	// operation on target requiring only "a" must still be a violation.
	m := &model.Model{
		Version: "1",
		Principals: []model.Principal{
			{ID: "root", Authority: []string{"a"}},
		},
		Agents: []model.Agent{
			{ID: "mid"},
			{ID: "target"},
		},
		Delegations: []model.Delegation{
			{Delegator: "root", Delegatee: "mid", Authority: []string{"a"}},
			{Delegator: "mid", Delegatee: "target", Authority: []string{"a", "c"}},
		},
		Operations: []model.Operation{
			{Actor: "target", Action: "op", Requires: "a"},
		},
	}

	res := Run(m)
	if len(res.Findings) != 2 {
		t.Fatalf("expected 2 findings (1 edge + 1 operation), got %d: %+v", len(res.Findings), res.Findings)
	}

	edge, ok := res.Findings[0].(report.EdgeFinding)
	if !ok || edge.Delegator != "mid" || edge.Delegatee != "target" {
		t.Fatalf("expected edge-level finding mid->target first, got %+v", res.Findings[0])
	}
	// excess is defined precisely as authority \ DA(delegator) = {a,c}\{a} = {c}.
	if !reflect.DeepEqual(edge.Declared, []string{"a", "c"}) {
		t.Errorf("declared = %v, want [a c]", edge.Declared)
	}
	if !reflect.DeepEqual(edge.Excess, []string{"c"}) {
		t.Errorf("excess = %v, want [c]", edge.Excess)
	}

	op, ok := res.Findings[1].(report.OperationFinding)
	if !ok {
		t.Fatalf("expected operation-level finding second, got %+v", res.Findings[1])
	}
	if len(op.Held) != 0 {
		t.Errorf("held = %v, want empty — invalid edge must contribute nothing to DA(target)", op.Held)
	}
}

func TestRunIsDeterministicAcrossRepeatedInvocations(t *testing.T) {
	m, loadErr := loader.Load("../../testdata/valid/mixed-violations.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	r1, err1 := report.RenderJSON(Run(m))
	r2, err2 := report.RenderJSON(Run(m))
	if err1 != nil || err2 != nil {
		t.Fatalf("render errors: %v, %v", err1, err2)
	}
	if string(r1) != string(r2) {
		t.Errorf("Run produced different output across repeated invocations:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", r1, r2)
	}
}
