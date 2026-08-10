package report

import (
	"fmt"
	"strings"
)

// RenderText renders Result as a human-readable report. It is a pure
// function of Result's (already-sorted) content.
func RenderText(r Result) string {
	var b strings.Builder
	if len(r.Findings) == 0 {
		b.WriteString("ALLOW\n0 findings\n")
		return b.String()
	}

	fmt.Fprintf(&b, "DENY\n%d finding(s)\n", len(r.Findings))
	for i, f := range r.Findings {
		b.WriteString("\n")
		switch v := f.(type) {
		case EdgeFinding:
			fmt.Fprintf(&b, "[%d] %s (%s)\n", i+1, v.Violation, v.Point)
			fmt.Fprintf(&b, "  delegator: %s\n", v.Delegator)
			fmt.Fprintf(&b, "  delegatee: %s\n", v.Delegatee)
			fmt.Fprintf(&b, "  declared:  %s\n", joinOrNone(v.Declared, ", "))
			fmt.Fprintf(&b, "  excess:    %s\n", joinOrNone(v.Excess, ", "))
			fmt.Fprintf(&b, "  trace:     %s\n", joinOrNone(v.Trace, " -> "))
			fmt.Fprintf(&b, "  reason:    %s\n", v.Reason)
		case OperationFinding:
			fmt.Fprintf(&b, "[%d] %s (%s)\n", i+1, v.Violation, v.Point)
			fmt.Fprintf(&b, "  actor:    %s\n", v.Actor)
			fmt.Fprintf(&b, "  action:   %s\n", v.Action)
			fmt.Fprintf(&b, "  requires: %s\n", v.Requires)
			fmt.Fprintf(&b, "  held:     %s\n", joinOrNone(v.Held, ", "))
			fmt.Fprintf(&b, "  trace:    %s\n", joinOrNone(v.Trace, " -> "))
			fmt.Fprintf(&b, "  reason:   %s\n", v.Reason)
		case CapabilityEdgeFinding:
			fmt.Fprintf(&b, "[%d] %s (%s)\n", i+1, v.Violation, v.Point)
			fmt.Fprintf(&b, "  %-14s %s\n", "delegator:", v.Delegator)
			fmt.Fprintf(&b, "  %-14s %s\n", "delegatee:", v.Delegatee)
			fmt.Fprintf(&b, "  %-14s %s\n", "declared:", joinCapabilitiesOrNone(v.Declared, ", "))
			fmt.Fprintf(&b, "  %-14s %s\n", "excess:", joinCapabilitiesOrNone(v.Excess, ", "))
			fmt.Fprintf(&b, "  %-14s %s\n", "bound targets:", joinOrNone(v.BoundTargets, ", "))
			fmt.Fprintf(&b, "  %-14s %s\n", "trace:", joinOrNone(v.Trace, " -> "))
			fmt.Fprintf(&b, "  %-14s %s\n", "reason:", v.Reason)
		case CapabilityOperationFinding:
			fmt.Fprintf(&b, "[%d] %s (%s)\n", i+1, v.Violation, v.Point)
			fmt.Fprintf(&b, "  %-14s %s\n", "actor:", v.Actor)
			fmt.Fprintf(&b, "  %-14s %s\n", "action:", v.Action)
			fmt.Fprintf(&b, "  %-14s %s\n", "requires:", v.Requires.String())
			fmt.Fprintf(&b, "  %-14s %s\n", "held:", joinCapabilitiesOrNone(v.Held, ", "))
			fmt.Fprintf(&b, "  %-14s %s\n", "bound targets:", joinOrNone(v.BoundTargets, ", "))
			fmt.Fprintf(&b, "  %-14s %s\n", "trace:", joinOrNone(v.Trace, " -> "))
			fmt.Fprintf(&b, "  %-14s %s\n", "reason:", v.Reason)
		case ConfusedDeputyFinding:
			fmt.Fprintf(&b, "[%d] %s (%s)\n", i+1, v.Violation, v.Point)
			fmt.Fprintf(&b, "  %-17s %s\n", "actor:", v.Actor)
			fmt.Fprintf(&b, "  %-17s %s\n", "requester:", v.Requester)
			fmt.Fprintf(&b, "  %-17s %s\n", "action:", v.Action)
			fmt.Fprintf(&b, "  %-17s %s\n", "requires:", v.Requires.String())
			fmt.Fprintf(&b, "  %-17s %s\n", "actor held:", joinCapabilitiesOrNone(v.ActorHeld, ", "))
			fmt.Fprintf(&b, "  %-17s %s\n", "requester held:", joinCapabilitiesOrNone(v.RequesterHeld, ", "))
			fmt.Fprintf(&b, "  %-17s %s\n", "requester bound:", joinOrNone(v.RequesterBoundTargets, ", "))
			fmt.Fprintf(&b, "  %-17s %s\n", "actor trace:", joinOrNone(v.ActorTrace, " -> "))
			fmt.Fprintf(&b, "  %-17s %s\n", "requester trace:", joinOrNone(v.RequesterTrace, " -> "))
			fmt.Fprintf(&b, "  %-17s %s\n", "reason:", v.Reason)
		}
	}
	return b.String()
}

func joinOrNone(items []string, sep string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, sep)
}
