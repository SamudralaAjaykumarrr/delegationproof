package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfusedDeputyFindingNeverHeldReasonText(t *testing.T) {
	f := NewConfusedDeputyFinding(
		"billing-agent", "support-agent", "refund-b",
		Capability{Scope: "billing:refund", Target: "billing-service"},
		[]Capability{{Scope: "billing:refund", Target: "billing-service"}},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		nil, []string{"admin", "billing-agent", "refund-b"}, []string{"admin", "support-agent"},
	)
	want := "refund-b requires billing:refund@billing-service, which billing-agent validly holds, but requester support-agent has never held billing:refund under any target — billing-agent is being induced to exercise authority support-agent was never granted"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
	if f.Violation != ViolationConfusedDeputy {
		t.Errorf("violation = %q, want %q", f.Violation, ViolationConfusedDeputy)
	}
	if f.Point != PointOperation {
		t.Errorf("point = %q, want %q", f.Point, PointOperation)
	}
	if len(f.RequesterBoundTargets) != 0 {
		t.Errorf("requester_bound_targets = %v, want empty", f.RequesterBoundTargets)
	}
}

func TestConfusedDeputyFindingWrongTargetReasonText(t *testing.T) {
	f := NewConfusedDeputyFinding(
		"mid", "requester-bad-target", "op-cd-target",
		Capability{Scope: "a", Target: "svc-a"},
		[]Capability{{Scope: "a", Target: "svc-a"}},
		[]Capability{{Scope: "a", Target: "svc-b"}},
		[]string{"svc-b"}, []string{"root", "mid", "op-cd-target"}, []string{"root", "requester-bad-target"},
	)
	want := "op-cd-target requires a@svc-a, which mid validly holds, but requester requester-bad-target holds a only for svc-b, which does not include svc-a — mid is being induced to exercise authority requester-bad-target was never granted for this target"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestConfusedDeputyFindingArraysNeverNull(t *testing.T) {
	f := NewConfusedDeputyFinding("a", "r", "act", Capability{Scope: "s", Target: "t"}, nil, nil, nil, nil, nil)
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	for _, field := range []string{
		`"actor_held": []`, `"requester_held": []`, `"requester_bound_targets": []`,
		`"actor_trace": []`, `"requester_trace": []`,
	} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %s in output, got:\n%s", field, s)
		}
	}
	if strings.Contains(s, "null") {
		t.Errorf("no array field may render as null:\n%s", s)
	}
}

func TestConfusedDeputyFindingJSONFieldOrder(t *testing.T) {
	f := NewConfusedDeputyFinding(
		"billing-agent", "support-agent", "refund-b",
		Capability{Scope: "billing:refund", Target: "billing-service"},
		[]Capability{{Scope: "billing:refund", Target: "billing-service"}},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		nil, []string{"admin", "billing-agent", "refund-b"}, []string{"admin", "support-agent"},
	)
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	fields := []string{
		`"violation"`, `"point"`, `"actor"`, `"requester"`, `"action"`, `"requires"`,
		`"actor_held"`, `"requester_held"`, `"requester_bound_targets"`,
		`"actor_trace"`, `"requester_trace"`, `"reason"`,
	}
	last := -1
	for _, field := range fields {
		idx := strings.Index(s, field)
		if idx == -1 {
			t.Fatalf("missing field %s in output:\n%s", field, s)
		}
		if idx < last {
			t.Errorf("field %s out of order in output:\n%s", field, s)
		}
		last = idx
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestSortOrderRequesterIsTrailingTiebreaker(t *testing.T) {
	findings := []interface{}{
		NewConfusedDeputyFinding("a", "z-req", "act", Capability{Scope: "s", Target: "t"}, nil, nil, nil, nil, nil),
		NewConfusedDeputyFinding("a", "a-req", "act", Capability{Scope: "s", Target: "t"}, nil, nil, nil, nil, nil),
	}
	Sort(findings)
	first := findings[0].(ConfusedDeputyFinding)
	second := findings[1].(ConfusedDeputyFinding)
	if first.Requester != "a-req" || second.Requester != "z-req" {
		t.Errorf("not sorted by trailing requester: got %q then %q", first.Requester, second.Requester)
	}
}

func TestSortOrderConfusedDeputyMixedWithOtherFindingTypes(t *testing.T) {
	findings := []interface{}{
		NewConfusedDeputyFinding("a", "r", "act", Capability{Scope: "s", Target: "t"}, nil, nil, nil, nil, nil),
		NewCapabilityOperationFinding(ViolationAuthorityAmplification, "a", "act0", Capability{Scope: "s", Target: "t"}, nil, nil, nil),
		NewCapabilityEdgeFinding(ViolationAuthorityAmplification, "d", "e", nil, nil, nil, nil),
	}
	Sort(findings)
	if keyOf(findings[0]).point != PointDelegationEdge {
		t.Fatalf("expected edge finding first, got order: %T, %T, %T", findings[0], findings[1], findings[2])
	}
	// Both remaining are point=operation, subject="a"; "act" < "act0"
	// lexicographically (prefix sorts first), so ConfusedDeputyFinding
	// (action="act") sorts before CapabilityOperationFinding
	// (action="act0").
	if _, ok := findings[1].(ConfusedDeputyFinding); !ok {
		t.Errorf("expected ConfusedDeputyFinding second, got %T", findings[1])
	}
	if _, ok := findings[2].(CapabilityOperationFinding); !ok {
		t.Errorf("expected CapabilityOperationFinding third, got %T", findings[2])
	}
}

func TestRenderTextConfusedDeputyFinding(t *testing.T) {
	f := NewConfusedDeputyFinding(
		"billing-agent", "support-agent", "refund-b",
		Capability{Scope: "billing:refund", Target: "billing-service"},
		[]Capability{{Scope: "billing:refund", Target: "billing-service"}},
		[]Capability{{Scope: "billing:read", Target: "billing-service"}},
		nil, []string{"admin", "billing-agent", "refund-b"}, []string{"admin", "support-agent"},
	)
	got := RenderText(Result{Findings: []interface{}{f}})
	for _, want := range []string{
		"[1] confused_deputy (operation)",
		"actor:", "billing-agent",
		"requester:", "support-agent",
		"action:", "refund-b",
		"requires:", "billing:refund@billing-service",
		"actor held:", "requester held:", "billing:read@billing-service",
		"requester bound:", "(none)",
		"actor trace:", "admin -> billing-agent -> refund-b",
		"requester trace:", "admin -> support-agent",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}
