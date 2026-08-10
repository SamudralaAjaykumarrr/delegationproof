package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCapabilityString(t *testing.T) {
	c := Capability{Scope: "billing:read", Target: "billing-service"}
	if got, want := c.String(), "billing:read@billing-service"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCapabilityEdgeFindingAmplificationReasonText(t *testing.T) {
	f := NewCapabilityEdgeFinding(
		ViolationAuthorityAmplification, "agent-a", "agent-c",
		[]Capability{{Scope: "billing:write", Target: "billing-service"}},
		[]Capability{{Scope: "billing:write", Target: "billing-service"}},
		nil, []string{"user", "agent-a", "agent-c"},
	)
	want := "agent-a attempted to delegate billing:write@billing-service, which is not in agent-a's derived authority"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
	if f.Point != PointDelegationEdge {
		t.Errorf("point = %q, want %q", f.Point, PointDelegationEdge)
	}
	if len(f.BoundTargets) != 0 {
		t.Errorf("bound_targets = %v, want empty for authority_amplification", f.BoundTargets)
	}
}

func TestCapabilityEdgeFindingContextBindingReasonText(t *testing.T) {
	f := NewCapabilityEdgeFinding(
		ViolationContextBinding, "mid", "leaf",
		[]Capability{{Scope: "a", Target: "svc-b"}},
		[]Capability{{Scope: "a", Target: "svc-b"}},
		[]string{"svc-a"}, []string{"root", "mid", "leaf"},
	)
	want := "mid attempted to delegate a@svc-b, which mid holds only for svc-a"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestCapabilityOperationFindingAmplificationReasonText(t *testing.T) {
	f := NewCapabilityOperationFinding(
		ViolationAuthorityAmplification, "agent-b", "billing.refund",
		Capability{Scope: "billing:write", Target: "billing-service"},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		nil, nil,
	)
	want := "billing:write@billing-service was never present in the valid delegation chain reaching agent-b"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestCapabilityOperationFindingContextBindingReasonTextMatchesWorkedExample(t *testing.T) {
	// docs/phase-2-plan.md §15's exact worked example.
	f := NewCapabilityOperationFinding(
		ViolationContextBinding, "billing-agent", "read-record",
		Capability{Scope: "billing:read", Target: "payroll-service"},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		[]string{"billing-service"}, []string{"user", "billing-agent", "read-record"},
	)
	want := "billing:read is held by billing-agent only for billing-service, which does not include payroll-service"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestSortOrderExtendedWithTarget(t *testing.T) {
	// Two operation findings sharing (actor, action, requires.Scope) but
	// differing only by target (§12) — target must be the trailing
	// tiebreaker.
	findings := []interface{}{
		NewCapabilityOperationFinding(ViolationContextBinding, "a", "act", Capability{Scope: "s", Target: "z"}, nil, []string{"y"}, nil),
		NewCapabilityOperationFinding(ViolationContextBinding, "a", "act", Capability{Scope: "s", Target: "b"}, nil, []string{"y"}, nil),
	}
	Sort(findings)
	first := findings[0].(CapabilityOperationFinding)
	second := findings[1].(CapabilityOperationFinding)
	if first.Requires.Target != "b" || second.Requires.Target != "z" {
		t.Errorf("not sorted by trailing target: got %q then %q", first.Requires.Target, second.Requires.Target)
	}
}

func TestSortOrderMixesV1AndV2FindingTypes(t *testing.T) {
	// A v1-shaped finding (target always "") must sort before any v2
	// finding sharing the same (point, subject, secondary, scope) prefix,
	// since "" < any non-empty target — but in practice v1 and v2
	// findings never appear in the same Result (disjoint model versions).
	// This asserts Sort doesn't panic or misbehave if ever handed a mixed
	// slice, since keyOf/less must remain total over all four types.
	findings := []interface{}{
		NewCapabilityOperationFinding(ViolationAuthorityAmplification, "a", "act", Capability{Scope: "s", Target: "t"}, nil, nil, nil),
		NewOperationFinding("a", "act", "s", nil, nil),
		NewCapabilityEdgeFinding(ViolationAuthorityAmplification, "d", "e", nil, nil, nil, nil),
		NewEdgeFinding("d", "e", nil, nil, nil),
	}
	Sort(findings)
	// Edge-point findings (delegation_edge) sort before operation-point
	// findings lexicographically ("delegation_edge" < "operation"),
	// regardless of whether the concrete type is Phase 1's EdgeFinding or
	// Phase 2's CapabilityEdgeFinding.
	if keyOf(findings[0]).point != PointDelegationEdge || keyOf(findings[1]).point != PointDelegationEdge {
		t.Fatalf("expected the two edge-point findings first, got order: %T, %T, %T, %T", findings[0], findings[1], findings[2], findings[3])
	}
	if keyOf(findings[2]).point != PointOperation || keyOf(findings[3]).point != PointOperation {
		t.Fatalf("expected the two operation-point findings last, got order: %T, %T, %T, %T", findings[0], findings[1], findings[2], findings[3])
	}
}

func TestRenderJSONCapabilityFindingsArraysNeverNull(t *testing.T) {
	edge := NewCapabilityEdgeFinding(ViolationAuthorityAmplification, "d", "e", nil, nil, nil, nil)
	op := NewCapabilityOperationFinding(ViolationAuthorityAmplification, "a", "act", Capability{Scope: "s", Target: "t"}, nil, nil, nil)
	out, err := RenderJSON(Result{Findings: []interface{}{edge, op}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	for _, field := range []string{`"declared": []`, `"excess": []`, `"bound_targets": []`, `"trace": []`, `"held": []`} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %s in output, got:\n%s", field, s)
		}
	}
	if strings.Contains(s, "null") {
		t.Errorf("no array field may render as null:\n%s", s)
	}
}

func TestRenderJSONCapabilityFieldOrderMatchesSpec(t *testing.T) {
	op := NewCapabilityOperationFinding(
		ViolationContextBinding, "billing-agent", "read-record",
		Capability{Scope: "billing:read", Target: "payroll-service"},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		[]string{"billing-service"}, []string{"user", "billing-agent", "read-record"},
	)
	out, err := RenderJSON(Result{Findings: []interface{}{op}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	fields := []string{`"violation"`, `"point"`, `"actor"`, `"action"`, `"requires"`, `"held"`, `"bound_targets"`, `"trace"`, `"reason"`}
	last := -1
	for _, f := range fields {
		idx := strings.Index(s, f)
		if idx == -1 {
			t.Fatalf("missing field %s in output:\n%s", f, s)
		}
		if idx < last {
			t.Errorf("field %s out of order in output:\n%s", f, s)
		}
		last = idx
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestRenderTextCapabilityOperationFindingMatchesWorkedExample(t *testing.T) {
	op := NewCapabilityOperationFinding(
		ViolationContextBinding, "billing-agent", "read-record",
		Capability{Scope: "billing:read", Target: "payroll-service"},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		[]string{"billing-service"}, []string{"user", "billing-agent", "read-record"},
	)
	got := RenderText(Result{Findings: []interface{}{op}})
	want := "DENY\n1 finding(s)\n\n" +
		"[1] context_binding_violation (operation)\n" +
		"  actor:         billing-agent\n" +
		"  action:        read-record\n" +
		"  requires:      billing:read@payroll-service\n" +
		"  held:          billing:read@billing-service\n" +
		"  bound targets: billing-service\n" +
		"  trace:         user -> billing-agent -> read-record\n" +
		"  reason:        billing:read is held by billing-agent only for billing-service, which does not include payroll-service\n"
	if got != want {
		t.Errorf("RenderText = %q, want %q", got, want)
	}
}

func TestRenderTextCapabilityEdgeFinding(t *testing.T) {
	edge := NewCapabilityEdgeFinding(
		ViolationContextBinding, "mid", "leaf",
		[]Capability{{Scope: "a", Target: "svc-b"}},
		[]Capability{{Scope: "a", Target: "svc-b"}},
		[]string{"svc-a"}, []string{"root", "mid", "leaf"},
	)
	got := RenderText(Result{Findings: []interface{}{edge}})
	for _, want := range []string{
		"[1] context_binding_violation (delegation_edge)",
		"delegator:", "mid",
		"delegatee:", "leaf",
		"declared:", "a@svc-b",
		"excess:", "a@svc-b",
		"bound targets: svc-a",
		"trace:         root -> mid -> leaf",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}
