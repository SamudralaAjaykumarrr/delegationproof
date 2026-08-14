package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDelegationDepthFindingSingleExcessReasonText(t *testing.T) {
	f := NewDelegationDepthFinding(
		"billing-agent", "support-agent",
		[]Capability{{Scope: "billing:refund", Target: "billing-service"}},
		[]DepthExcess{{Scope: "billing:refund", Target: "billing-service", ConfiguredMax: 1, RemainingDepth: 0}},
		[]string{"admin", "billing-agent", "support-agent"},
	)
	want := "billing-agent attempted to delegate billing:refund@billing-service to support-agent, but billing-agent's remaining delegation budget for this capability is 0 (configured maximum: 1) — it may no longer be redelegated"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
	if f.Violation != ViolationDelegationDepth {
		t.Errorf("violation = %q, want %q", f.Violation, ViolationDelegationDepth)
	}
	if f.Point != PointDelegationEdge {
		t.Errorf("point = %q, want %q", f.Point, PointDelegationEdge)
	}
}

func TestDelegationDepthFindingMultipleExcessReasonText(t *testing.T) {
	f := NewDelegationDepthFinding(
		"mid", "next",
		[]Capability{{Scope: "a", Target: "svc"}, {Scope: "b", Target: "svc"}},
		[]DepthExcess{
			{Scope: "a", Target: "svc", ConfiguredMax: 1, RemainingDepth: 0},
			{Scope: "b", Target: "svc", ConfiguredMax: 2, RemainingDepth: 0},
		},
		[]string{"root", "mid", "next"},
	)
	want := "mid attempted to delegate [a@svc (configured maximum: 1), b@svc (configured maximum: 2)] to next, but mid's remaining delegation budget for each is 0 — none may be redelegated further"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestDelegationDepthFindingArraysNeverNull(t *testing.T) {
	f := NewDelegationDepthFinding("d", "e", nil, nil, nil)
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	for _, field := range []string{`"declared": []`, `"excess": []`, `"trace": []`} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %s in output, got:\n%s", field, s)
		}
	}
	if strings.Contains(s, "null") {
		t.Errorf("no array field may render as null:\n%s", s)
	}
}

func TestDelegationDepthFindingJSONFieldOrder(t *testing.T) {
	f := NewDelegationDepthFinding(
		"billing-agent", "support-agent",
		[]Capability{{Scope: "billing:refund", Target: "billing-service"}},
		[]DepthExcess{{Scope: "billing:refund", Target: "billing-service", ConfiguredMax: 1, RemainingDepth: 0}},
		[]string{"admin", "billing-agent", "support-agent"},
	)
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	fields := []string{`"violation"`, `"point"`, `"delegator"`, `"delegatee"`, `"declared"`, `"excess"`, `"trace"`, `"reason"`}
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
	excessIdx := strings.Index(s, `"excess"`)
	excessFields := []string{`"scope"`, `"target"`, `"configured_max_depth"`, `"remaining_depth"`}
	lastNested := excessIdx
	for _, field := range excessFields {
		idx := strings.Index(s[excessIdx:], field)
		if idx == -1 {
			t.Fatalf("missing nested excess field %s in output:\n%s", field, s)
		}
		if excessIdx+idx < lastNested {
			t.Errorf("nested field %s out of order in output:\n%s", field, s)
		}
		lastNested = excessIdx + idx
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestDelegationDepthFindingSortKey(t *testing.T) {
	f := NewDelegationDepthFinding("d", "e", nil, nil, nil)
	k := keyOf(f)
	if k.point != PointDelegationEdge || k.subject != "d" || k.secondary != "e" {
		t.Errorf("keyOf = %+v, want point=%q subject=d secondary=e", k, PointDelegationEdge)
	}
	if k.scope != "" || k.target != "" || k.requester != "" {
		t.Errorf("keyOf trailing fields must be empty for an edge-scoped finding, got %+v", k)
	}
}

func TestRenderTextDelegationDepthFinding(t *testing.T) {
	f := NewDelegationDepthFinding(
		"billing-agent", "support-agent",
		[]Capability{{Scope: "billing:refund", Target: "billing-service"}},
		[]DepthExcess{{Scope: "billing:refund", Target: "billing-service", ConfiguredMax: 1, RemainingDepth: 0}},
		[]string{"admin", "billing-agent", "support-agent"},
	)
	got := RenderText(Result{Findings: []interface{}{f}})
	for _, want := range []string{
		"[1] delegation_depth_violation (delegation_edge)",
		"delegator:", "billing-agent",
		"delegatee:", "support-agent",
		"declared:", "billing:refund@billing-service",
		"excess:", "billing:refund@billing-service (configured max: 1, remaining: 0)",
		"trace:         admin -> billing-agent -> support-agent",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}
