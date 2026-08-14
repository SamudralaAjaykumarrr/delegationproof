// Phase 4 domain types (docs/phase-4-plan.md §5, §23): a version-4 document
// is ModelV3's shape with exactly one addition — a principal's declared
// authority entries gain a required MaxDelegationDepth, tracked via
// RootCapability rather than the plain Capability tuple. Agents,
// delegations, and operations are byte-for-byte identical in shape to their
// v3 counterparts: delegations reuse model.Capability (from types_v2.go) for
// their Authority entries, and max_delegation_depth exists nowhere except
// inside a principal's authority array (§3, §7). These types are pure
// data — no validation logic lives here; see internal/loader for structural
// validation. ModelV4 shares no type with model.Model, model.ModelV2, or
// model.ModelV3; the four schemas are structurally disjoint.
package model

// RootCapability is a Phase 4 root grant: a capability tuple plus the
// maximum number of delegation hops it may travel from this declaration
// (docs/phase-4-plan.md §3, §6). MaxDelegationDepth is a pointer so a
// present, explicit 0 (non-delegable-but-usable) can be distinguished from
// an absent key (nil, structurally invalid) — uniquely among every field
// added by any phase so far, this field's own zero value is a legal,
// meaningful declaration rather than an obviously-invalid placeholder.
type RootCapability struct {
	Scope              string `json:"scope"`
	Target             string `json:"target"`
	MaxDelegationDepth *int   `json:"max_delegation_depth"`
}

// ModelV4 is the whole version-4 input document.
type ModelV4 struct {
	Version     string         `json:"version"`
	Principals  []PrincipalV4  `json:"principals"`
	Agents      []AgentV4      `json:"agents"`
	Delegations []DelegationV4 `json:"delegations"`
	Operations  []OperationV4  `json:"operations"`
}

// PrincipalV4 is a root capability holder. Its authority is axiomatic
// (declared, not derived) and it cannot be a delegation target. Unlike
// PrincipalV3, each declared capability carries its own re-delegation
// budget via RootCapability.
type PrincipalV4 struct {
	ID        string           `json:"id"`
	Authority []RootCapability `json:"authority"`
}

// AgentV4 is a non-root participant. Identical shape to AgentV3: it
// deliberately has no Authority field, so a stray "authority" key decodes
// as a structural validation error via DisallowUnknownFields.
type AgentV4 struct {
	ID string `json:"id"`
}

// DelegationV4 is a directed grant of a non-empty capability set,
// byte-for-byte identical in shape to DelegationV3 — it reuses the plain,
// unmodified Capability{Scope, Target} type, never RootCapability.
// max_delegation_depth is never re-declared or re-asserted at a delegation
// edge (docs/phase-4-plan.md §3, §7): remaining budget is inherited and
// derived by the verifier, not authored here. A stray max_delegation_depth
// key on an authority entry here is rejected at decode time via
// DisallowUnknownFields, with zero new validation code.
type DelegationV4 struct {
	Delegator string       `json:"delegator"`
	Delegatee string       `json:"delegatee"`
	Authority []Capability `json:"authority"`
}

// OperationV4 is byte-for-byte identical in shape to OperationV3:
// exercising a capability neither consumes nor is gated by remaining
// delegation depth (docs/phase-4-plan.md §4.2, §12, §13).
type OperationV4 struct {
	Actor     string `json:"actor"`
	Requester string `json:"requester"`
	Action    string `json:"action"`
	Requires  string `json:"requires"`
	Target    string `json:"target"`
}
