package report

import "encoding/json"

// Result is the top-level machine-readable verify result: ALLOW with no
// findings, or DENY with one or more sorted findings.
type Result struct {
	Findings []interface{}
}

func (r Result) status() string {
	if len(r.Findings) == 0 {
		return "ALLOW"
	}
	return "DENY"
}

// RenderJSON renders Result as the single JSON object described in
// docs/phase-1-plan.md §10: {"result": ..., "findings": [...]}, followed by
// a trailing newline. Output is byte-identical for equivalent inputs run
// repeatedly, and for semantically-equivalent inputs with reordered arrays,
// because Findings must already be in canonical sorted order.
func RenderJSON(r Result) ([]byte, error) {
	findings := r.Findings
	if findings == nil {
		findings = []interface{}{}
	}
	payload := struct {
		Result   string        `json:"result"`
		Findings []interface{} `json:"findings"`
	}{
		Result:   r.status(),
		Findings: findings,
	}
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
