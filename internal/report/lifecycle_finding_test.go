package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLifecycleFindingUnsafeReasonText(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnsafe, "billing-agent", "admin", "void-unsafe",
		Capability{Scope: "billing:void", Target: "billing-service"},
		[]string{"compliance-officer"},
		"compliance-officer", "revoked",
		[]LifecycleStep{{From: "approved", Event: "revoke", To: "revoked"}},
		[]string{"admin", "billing-agent", "void-unsafe"},
	)
	want := "void-unsafe requires billing:void@billing-service, which billing-agent validly holds and admin is authorized to request, and billing:void@billing-service requires approval; compliance-officer independently hold standing, but none of their declared approval lifecycles can be proven to remain in state 'approved' — compliance-officer's can reach state 'revoked' via approved -[revoke]-> revoked, so it cannot be statically relied upon at time of exercise"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
	if f.Violation != ViolationApprovalLifecycleUnsafe {
		t.Errorf("violation = %q, want %q", f.Violation, ViolationApprovalLifecycleUnsafe)
	}
	if f.Point != PointOperation {
		t.Errorf("point = %q, want %q", f.Point, PointOperation)
	}
}

func TestLifecycleFindingUnprovenReasonText(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnproven, "billing-agent", "admin", "refund",
		Capability{Scope: "billing:refund", Target: "billing-service"},
		[]string{"compliance-officer"},
		"compliance-officer", "",
		nil,
		[]string{"admin", "billing-agent", "refund"},
	)
	want := "refund requires billing:refund@billing-service, which billing-agent validly holds and admin is authorized to request, and billing:refund@billing-service requires approval; compliance-officer independently hold standing, but compliance-officer's declared approval lifecycle is too large to prove safe within the configured exploration bound — an unproven approval is never treated as satisfying the requirement"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
	if f.UnsafeState != "" {
		t.Errorf("UnsafeState should be empty for an unproven finding, got %q", f.UnsafeState)
	}
	if len(f.LifecycleTrace) != 0 {
		t.Errorf("LifecycleTrace should be empty for an unproven finding, got %+v", f.LifecycleTrace)
	}
}

func TestLifecycleFindingZeroHopTraceRendersInitialState(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnsafe, "actor", "requester", "action",
		Capability{Scope: "s", Target: "t"},
		[]string{"approver"},
		"approver", "pending",
		nil, // zero hops: the unsafe state is the automaton's own initial state
		[]string{"root", "actor", "action"},
	)
	if !strings.Contains(f.Reason, "reach state 'pending' via pending") {
		t.Errorf("expected the zero-hop case to describe the state as reached directly, got reason: %q", f.Reason)
	}
	if len(f.LifecycleTrace) != 0 {
		t.Errorf("LifecycleTrace should be empty (zero hops), got %+v", f.LifecycleTrace)
	}
}

func TestLifecycleFindingArraysNeverNull(t *testing.T) {
	f := NewLifecycleFinding(ViolationApprovalLifecycleUnproven, "a", "r", "act", Capability{}, nil, "x", "", nil, nil)
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	for _, field := range []string{`"declared_approvers": []`, `"lifecycle_trace": []`, `"trace": []`} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %s in output, got:\n%s", field, s)
		}
	}
	if strings.Contains(s, "null") {
		t.Errorf("no array field may render as null:\n%s", s)
	}
}

func TestLifecycleFindingJSONFieldOrder(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnsafe, "billing-agent", "admin", "refund",
		Capability{Scope: "billing:refund", Target: "billing-service"},
		[]string{"compliance-officer"},
		"compliance-officer", "revoked",
		[]LifecycleStep{{From: "approved", Event: "revoke", To: "revoked"}},
		[]string{"admin", "billing-agent", "refund"},
	)
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	fields := []string{
		`"violation"`, `"point"`, `"actor"`, `"requester"`, `"action"`, `"requires"`,
		`"declared_approvers"`, `"unsafe_approver"`, `"unsafe_state"`, `"lifecycle_trace"`,
		`"trace"`, `"reason"`,
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
	traceIdx := strings.Index(s, `"lifecycle_trace"`)
	stepFields := []string{`"from"`, `"event"`, `"to"`}
	lastNested := traceIdx
	for _, field := range stepFields {
		idx := strings.Index(s[traceIdx:], field)
		if idx == -1 {
			t.Fatalf("missing nested lifecycle_trace field %s in output:\n%s", field, s)
		}
		if traceIdx+idx < lastNested {
			t.Errorf("nested field %s out of order in output:\n%s", field, s)
		}
		lastNested = traceIdx + idx
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestLifecycleFindingSortKey(t *testing.T) {
	f := NewLifecycleFinding(ViolationApprovalLifecycleUnsafe, "actor", "requester", "action", Capability{Scope: "s", Target: "t"}, nil, "x", "u", nil, nil)
	k := keyOf(f)
	if k.point != PointOperation || k.subject != "actor" || k.secondary != "action" {
		t.Errorf("keyOf = %+v, want point=%q subject=actor secondary=action", k, PointOperation)
	}
	if k.scope != "s" || k.target != "t" || k.requester != "requester" {
		t.Errorf("keyOf trailing fields = (%q,%q,%q), want (s,t,requester)", k.scope, k.target, k.requester)
	}
}

func TestRenderTextLifecycleFinding(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnsafe, "billing-agent", "admin", "void-unsafe",
		Capability{Scope: "billing:void", Target: "billing-service"},
		[]string{"compliance-officer"},
		"compliance-officer", "revoked",
		[]LifecycleStep{{From: "approved", Event: "revoke", To: "revoked"}},
		[]string{"admin", "billing-agent", "void-unsafe"},
	)
	got := RenderText(Result{Findings: []interface{}{f}})
	for _, want := range []string{
		"[1] approval_lifecycle_unsafe (operation)",
		"actor:", "billing-agent",
		"requester:", "admin",
		"declared approvers:", "compliance-officer",
		"unsafe approver:", "compliance-officer",
		"unsafe state:", "revoked",
		"lifecycle trace:", "approved -[revoke]-> revoked",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestRenderTextLifecycleFindingNoEventOmitsBracket(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnsafe, "actor", "requester", "action",
		Capability{Scope: "s", Target: "t"}, []string{"approver"}, "approver", "revoked",
		[]LifecycleStep{{From: "pending", Event: "", To: "approved"}, {From: "approved", Event: "", To: "revoked"}},
		nil,
	)
	got := RenderText(Result{Findings: []interface{}{f}})
	if !strings.Contains(got, "pending -> approved -> revoked") {
		t.Errorf("expected event-less hops rendered with a bare arrow, got:\n%s", got)
	}
}

func TestRenderTextLifecycleFindingUnprovenShowsNone(t *testing.T) {
	f := NewLifecycleFinding(
		ViolationApprovalLifecycleUnproven, "actor", "requester", "action",
		Capability{Scope: "s", Target: "t"}, []string{"approver"}, "approver", "", nil, nil,
	)
	got := RenderText(Result{Findings: []interface{}{f}})
	if !strings.Contains(got, "unsafe state:") {
		t.Errorf("expected an 'unsafe state:' label, got:\n%s", got)
	}
}
