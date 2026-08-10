// Phase 2 domain types (docs/phase-2-plan.md §3, §19): a version-2 document
// replaces the bare scope string with a capability tuple (scope, target).
// These types are pure data — no validation logic lives here; see
// internal/loader for structural validation. ModelV2 shares no type with
// the Phase 1 Model in types.go; the two schemas are structurally disjoint.
package model

// Capability is the Phase 2 authority unit: an ordered pair (scope, target),
// compared by exact tuple equality only (docs/phase-2-plan.md §4).
type Capability struct {
	Scope  string `json:"scope"`
	Target string `json:"target"`
}

// ModelV2 is the whole version-2 input document.
type ModelV2 struct {
	Version     string         `json:"version"`
	Principals  []PrincipalV2  `json:"principals"`
	Agents      []AgentV2      `json:"agents"`
	Delegations []DelegationV2 `json:"delegations"`
	Operations  []OperationV2  `json:"operations"`
}

// PrincipalV2 is a root capability holder. Its authority is axiomatic
// (declared, not derived) and it cannot be a delegation target.
type PrincipalV2 struct {
	ID        string       `json:"id"`
	Authority []Capability `json:"authority"`
}

// AgentV2 is a non-root participant. Identical shape to Agent: it
// deliberately has no Authority field, so a stray "authority" key decodes
// as a structural validation error via DisallowUnknownFields.
type AgentV2 struct {
	ID string `json:"id"`
}

// DelegationV2 is a directed grant of a non-empty capability set.
type DelegationV2 struct {
	Delegator string       `json:"delegator"`
	Delegatee string       `json:"delegatee"`
	Authority []Capability `json:"authority"`
}

// OperationV2 is a declared point where Actor attempts to exercise the
// single capability (Requires, Target). Unlike Phase 1's Operation, it adds
// Target — the singular-requires design (one capability per operation) is
// otherwise unchanged.
type OperationV2 struct {
	Actor    string `json:"actor"`
	Action   string `json:"action"`
	Requires string `json:"requires"`
	Target   string `json:"target"`
}
