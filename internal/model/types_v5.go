// Phase 5 domain types (docs/phase-5-plan.md §5, §24): a version-5 document
// is ModelV4's shape with exactly two additions — a principal's declared
// authority entries gain a required RequiresApproval boolean, and the
// top-level document gains a new, required Approvals array. Agents,
// delegations, and operations are byte-for-byte identical in shape to their
// v4 counterparts: delegations reuse model.Capability (from types_v2.go) for
// their Authority entries, and requires_approval exists nowhere except
// inside a principal's authority array (§3, §5). These types are pure
// data — no validation logic lives here; see internal/loader for structural
// validation. ModelV5 shares no type with model.Model, model.ModelV2,
// model.ModelV3, or model.ModelV4; the five schemas are structurally
// disjoint.
package model

// RootCapabilityV5 is a Phase 5 root grant: a capability tuple, its Phase 4
// re-delegation budget, and its Phase 5 approval requirement
// (docs/phase-5-plan.md §3, §5, §6). This is a new type, not a reuse of
// RootCapability (v4) — mirroring how types_v4.go introduced its own
// RootCapability rather than reusing types_v3.go's Capability for principal
// declarations. RequiresApproval is a pointer, for the identical reason
// MaxDelegationDepth is a pointer (docs/phase-4-plan.md §6): false is
// itself a legitimate, meaningful, commonly-declared value, so a plain bool
// cannot distinguish "author explicitly wrote false" from "author omitted
// the key entirely."
type RootCapabilityV5 struct {
	Scope              string `json:"scope"`
	Target             string `json:"target"`
	MaxDelegationDepth *int   `json:"max_delegation_depth"`
	RequiresApproval   *bool  `json:"requires_approval"`
}

// ApprovalV5 is a declared approval record (docs/phase-5-plan.md §3, §7):
// Approver independently vouches for the exact capability (Scope, Target).
// An approval record is checked, never traversed — it does not create a
// delegation edge and does not participate in graph.TopoSort or
// graph.CanonicalTrace.
type ApprovalV5 struct {
	Approver string `json:"approver"`
	Scope    string `json:"scope"`
	Target   string `json:"target"`
}

// ModelV5 is the whole version-5 input document.
type ModelV5 struct {
	Version     string         `json:"version"`
	Principals  []PrincipalV5  `json:"principals"`
	Agents      []AgentV5      `json:"agents"`
	Delegations []DelegationV5 `json:"delegations"`
	Approvals   []ApprovalV5   `json:"approvals"`
	Operations  []OperationV5  `json:"operations"`
}

// PrincipalV5 is a root capability holder. Its authority is axiomatic
// (declared, not derived) and it cannot be a delegation target. Unlike
// PrincipalV4, each declared capability additionally carries its own
// approval requirement via RootCapabilityV5.
type PrincipalV5 struct {
	ID        string             `json:"id"`
	Authority []RootCapabilityV5 `json:"authority"`
}

// AgentV5 is a non-root participant. Identical shape to AgentV4: it
// deliberately has no Authority field, so a stray "authority" key decodes
// as a structural validation error via DisallowUnknownFields.
type AgentV5 struct {
	ID string `json:"id"`
}

// DelegationV5 is a directed grant of a non-empty capability set,
// byte-for-byte identical in shape to DelegationV4 — it reuses the plain,
// unmodified Capability{Scope, Target} type, never RootCapabilityV5.
// requires_approval is never re-declared or re-asserted at a delegation
// edge (docs/phase-5-plan.md §3, §4.2): approval gates exercise, not
// transmission. A stray requires_approval key on an authority entry here is
// rejected at decode time via DisallowUnknownFields, with zero new
// validation code.
type DelegationV5 struct {
	Delegator string       `json:"delegator"`
	Delegatee string       `json:"delegatee"`
	Authority []Capability `json:"authority"`
}

// OperationV5 is byte-for-byte identical in shape to OperationV4: approval
// is checked against the actor's derived authState, never re-declared on
// the operation itself (docs/phase-5-plan.md §4.2, §12).
type OperationV5 struct {
	Actor     string `json:"actor"`
	Requester string `json:"requester"`
	Action    string `json:"action"`
	Requires  string `json:"requires"`
	Target    string `json:"target"`
}
