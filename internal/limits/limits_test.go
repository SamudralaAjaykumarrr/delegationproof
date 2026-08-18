package limits

import "testing"

// TestDefaults locks the Phase 1 resource bounds (docs/phase-1-plan.md §12)
// against accidental drift.
func TestDefaults(t *testing.T) {
	cases := []struct {
		name string
		got  int64
		want int64
	}{
		{"MaxInputFileSize", MaxInputFileSize, 5 * 1024 * 1024},
		{"MaxNodes", int64(MaxNodes), 10000},
		{"MaxDelegationEdges", int64(MaxDelegationEdges), 50000},
		{"MaxOperations", int64(MaxOperations), 10000},
		{"MaxScopeLength", int64(MaxScopeLength), 256},
		{"MaxIDLength", int64(MaxIDLength), 128},
		{"MaxAuthoritySetSize", int64(MaxAuthoritySetSize), 256},
		{"MaxChainDepth", int64(MaxChainDepth), 64},
		{"MaxTargetLength", int64(MaxTargetLength), 128},
		{"MaxDelegationDepth", int64(MaxDelegationDepth), 64},
		{"MaxApprovals", int64(MaxApprovals), 10000},
		{"MaxLifecycleStates", int64(MaxLifecycleStates), 32},
		{"MaxLifecycleTransitions", int64(MaxLifecycleTransitions), 128},
		{"MaxExplorationStatesPerLifecycle", int64(MaxExplorationStatesPerLifecycle), 32},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}
