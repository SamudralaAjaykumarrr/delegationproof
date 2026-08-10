// Phase 3 domain types (docs/phase-3-plan.md §5, §21): a version-3 document
// is ModelV2 with exactly one addition — OperationV3 gains a required
// Requester field, the party an operation is actually performed for. These
// types are pure data — no validation logic lives here; see internal/loader
// for structural validation. ModelV3 shares no type with model.Model or
// model.ModelV2; the three schemas are structurally disjoint. Principals,
// agents, and delegations are byte-for-byte identical in shape to their v2
// counterparts (they reuse the same Capability type from types_v2.go).
package model

// ModelV3 is the whole version-3 input document.
type ModelV3 struct {
	Version     string         `json:"version"`
	Principals  []PrincipalV3  `json:"principals"`
	Agents      []AgentV3      `json:"agents"`
	Delegations []DelegationV3 `json:"delegations"`
	Operations  []OperationV3  `json:"operations"`
}

// PrincipalV3 is a root capability holder. Its authority is axiomatic
// (declared, not derived) and it cannot be a delegation target. Identical
// shape to PrincipalV2.
type PrincipalV3 struct {
	ID        string       `json:"id"`
	Authority []Capability `json:"authority"`
}

// AgentV3 is a non-root participant. Identical shape to AgentV2: it
// deliberately has no Authority field, so a stray "authority" key decodes
// as a structural validation error via DisallowUnknownFields.
type AgentV3 struct {
	ID string `json:"id"`
}

// DelegationV3 is a directed grant of a non-empty capability set. Identical
// shape to DelegationV2. Delegation edges have no requester concept
// (docs/phase-3-plan.md §8): Requester Authorization Preservation is
// defined only over operations.
type DelegationV3 struct {
	Delegator string       `json:"delegator"`
	Delegatee string       `json:"delegatee"`
	Authority []Capability `json:"authority"`
}

// OperationV3 is a declared point where Actor attempts to exercise the
// capability (Requires, Target) on behalf of Requester — the principal or
// agent the operation is actually performed for (docs/phase-3-plan.md
// §3-§5). Requester is a reference into the same node-id namespace Actor
// already draws from; it is required, with no default and no implicit
// self-reference (an actor acting for itself must write its own id
// explicitly).
type OperationV3 struct {
	Actor     string `json:"actor"`
	Requester string `json:"requester"`
	Action    string `json:"action"`
	Requires  string `json:"requires"`
	Target    string `json:"target"`
}
