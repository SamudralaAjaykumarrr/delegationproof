package report

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSortOrder(t *testing.T) {
	// Deliberately constructed out of order across all four tie-break
	// keys: point, subject, secondary, scope.
	findings := []interface{}{
		NewOperationFinding("z", "act", "s2", nil, nil),
		NewEdgeFinding("d2", "t1", nil, nil, nil),
		NewOperationFinding("a", "act2", "s1", nil, nil),
		NewOperationFinding("a", "act1", "s1", nil, nil),
		NewEdgeFinding("d1", "t2", nil, nil, nil),
		NewOperationFinding("z", "act", "s1", nil, nil),
	}
	Sort(findings)

	var gotPoints []string
	for _, f := range findings {
		gotPoints = append(gotPoints, keyOf(f).point+":"+keyOf(f).subject+":"+keyOf(f).secondary+":"+keyOf(f).scope)
	}
	want := []string{
		"delegation_edge:d1:t2:",
		"delegation_edge:d2:t1:",
		"operation:a:act1:s1",
		"operation:a:act2:s1",
		"operation:z:act:s1",
		"operation:z:act:s2",
	}
	for i := range want {
		if gotPoints[i] != want[i] {
			t.Errorf("position %d: got %q, want %q (full order: %v)", i, gotPoints[i], want[i], gotPoints)
			break
		}
	}
}

func TestSortIsStableAndRepeatable(t *testing.T) {
	findings := []interface{}{
		NewOperationFinding("b", "x", "s", nil, nil),
		NewEdgeFinding("a", "y", nil, nil, nil),
		NewOperationFinding("a", "x", "s", nil, nil),
	}
	Sort(findings)
	out1, _ := RenderJSON(Result{Findings: findings})

	findings2 := []interface{}{
		NewOperationFinding("a", "x", "s", nil, nil),
		NewOperationFinding("b", "x", "s", nil, nil),
		NewEdgeFinding("a", "y", nil, nil, nil),
	}
	Sort(findings2)
	out2, _ := RenderJSON(Result{Findings: findings2})

	if string(out1) != string(out2) {
		t.Errorf("differently-ordered input produced different sorted output:\n%s\n---\n%s", out1, out2)
	}
}

func TestEdgeFindingReasonText(t *testing.T) {
	f := NewEdgeFinding("agent-a", "agent-c", []string{"billing:write"}, []string{"billing:write"}, []string{"user", "agent-a", "agent-c"})
	want := "agent-a attempted to delegate billing:write, which is not in agent-a's derived authority"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestOperationFindingReasonText(t *testing.T) {
	f := NewOperationFinding("agent-b", "billing.refund", "billing:write", []string{"billing:read"}, nil)
	want := "billing:write was never present in the valid delegation chain reaching agent-b"
	if f.Reason != want {
		t.Errorf("reason = %q, want %q", f.Reason, want)
	}
}

func TestRenderJSONEmptyHeldIsExplicitArray(t *testing.T) {
	f := NewOperationFinding("orphan", "act", "s", nil, []string{"orphan", "act"})
	out, err := RenderJSON(Result{Findings: []interface{}{f}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	if !strings.Contains(string(out), `"held": []`) {
		t.Errorf("expected explicit empty array for held, got:\n%s", out)
	}
	if strings.Contains(string(out), `"held": null`) {
		t.Errorf("held must never render as null:\n%s", out)
	}
}

func TestRenderJSONCleanPass(t *testing.T) {
	out, err := RenderJSON(Result{})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded["result"] != "ALLOW" {
		t.Errorf(`result = %v, want "ALLOW"`, decoded["result"])
	}
	findings, ok := decoded["findings"].([]interface{})
	if !ok || len(findings) != 0 {
		t.Errorf("findings = %v, want empty array", decoded["findings"])
	}
}

func TestRenderJSONFieldOrderMatchesSpec(t *testing.T) {
	edge := NewEdgeFinding("agent-a", "agent-c", []string{"x"}, []string{"x"}, []string{"agent-a", "agent-c"})
	out, err := RenderJSON(Result{Findings: []interface{}{edge}})
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	fields := []string{`"violation"`, `"point"`, `"delegator"`, `"delegatee"`, `"declared"`, `"excess"`, `"trace"`, `"reason"`}
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
}

func TestRenderTextWithFindings(t *testing.T) {
	edge := NewEdgeFinding("agent-a", "agent-c", []string{"billing:write"}, []string{"billing:write"}, []string{"user", "agent-a", "agent-c"})
	op := NewOperationFinding("agent-b", "billing.refund", "billing:write", []string{"billing:read"}, []string{"user", "agent-a", "agent-b", "billing.refund"})

	got := RenderText(Result{Findings: []interface{}{edge, op}})

	for _, want := range []string{
		"DENY\n2 finding(s)\n",
		"[1] authority_amplification (delegation_edge)",
		"delegator: agent-a",
		"delegatee: agent-c",
		"declared:  billing:write",
		"excess:    billing:write",
		"trace:     user -> agent-a -> agent-c",
		"[2] authority_amplification (operation)",
		"actor:    agent-b",
		"action:   billing.refund",
		"requires: billing:write",
		"held:     billing:read",
		"trace:    user -> agent-a -> agent-b -> billing.refund",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestRenderTextCleanPass(t *testing.T) {
	got := RenderText(Result{})
	want := "ALLOW\n0 findings\n"
	if got != want {
		t.Errorf("RenderText = %q, want %q", got, want)
	}
}
