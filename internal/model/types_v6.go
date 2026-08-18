// Phase 6 domain types (docs/phase-6-plan.md §6, §30.2): a version-6
// document is ModelV5's shape with exactly one addition — an approvals[]
// record gains an optional Lifecycle object. Principals, agents,
// delegations, and operations are byte-for-byte identical in shape to their
// v5 counterparts: delegations reuse model.Capability (from types_v2.go)
// for their Authority entries, and a lifecycle exists nowhere except inside
// an approvals[] entry (§6, §7). These types are pure data — no validation
// logic lives here; see internal/loader for structural validation. ModelV6
// shares no type with model.Model, model.ModelV2, model.ModelV3,
// model.ModelV4, or model.ModelV5; the six schemas are structurally
// disjoint.
package model

// LifecycleTransition is one declared, unconditionally-legal transition of
// an approval record's own lifecycle automaton (docs/phase-6-plan.md §6,
// §8, §11). Event is optional and purely diagnostic — it never affects
// reachability. Self-loops (From == To) and cycles across multiple states
// are both explicitly legal; a lifecycle automaton is never required to be
// acyclic, unlike the delegation graph.
type LifecycleTransition struct {
	From  string `json:"from"`
	Event string `json:"event"`
	To    string `json:"to"`
}

// Lifecycle is a small, author-declared, possibly-cyclic state automaton
// attached to one approval record (docs/phase-6-plan.md §6, §8, §9): a
// finite set of state names, a designated initial state, and a set of
// declared transitions between them. The reserved safe-state name is
// exactly "approved" (case-sensitive, exact string equality, no
// normalization) — every other declared state name is, by construction,
// unsafe (§5.2, §8).
type Lifecycle struct {
	Initial     string                `json:"initial"`
	States      []string              `json:"states"`
	Transitions []LifecycleTransition `json:"transitions"`
}

// ApprovalV6 is a declared approval record (docs/phase-6-plan.md §6, §8):
// Approver independently vouches for the exact capability (Scope, Target),
// exactly as ApprovalV5 already establishes. Lifecycle is new and optional
// (a nil *Lifecycle means "no additional temporal structure declared,"
// unambiguously identical to Phase 5's eternal-fact model, §5.2, §6) — a
// pointer, not a value, because "absent" is a distinct, meaningful third
// state that no zero-valued Lifecycle struct could represent without
// sentinel ambiguity.
type ApprovalV6 struct {
	Approver  string     `json:"approver"`
	Scope     string     `json:"scope"`
	Target    string     `json:"target"`
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
}

// ModelV6 is the whole version-6 input document.
type ModelV6 struct {
	Version     string         `json:"version"`
	Principals  []PrincipalV6  `json:"principals"`
	Agents      []AgentV6      `json:"agents"`
	Delegations []DelegationV6 `json:"delegations"`
	Approvals   []ApprovalV6   `json:"approvals"`
	Operations  []OperationV6  `json:"operations"`
}

// PrincipalV6 is a root capability holder, byte-for-byte identical in shape
// to PrincipalV5: its authority is axiomatic (declared, not derived) and it
// cannot be a delegation target. max_delegation_depth/requires_approval
// mean exactly what they meant in v5 (docs/phase-6-plan.md §7) — lifecycle
// never attaches here, only to an approvals[] entry.
type PrincipalV6 struct {
	ID        string             `json:"id"`
	Authority []RootCapabilityV5 `json:"authority"`
}

// AgentV6 is a non-root participant. Identical shape to AgentV5: it
// deliberately has no Authority field, so a stray "authority" key decodes
// as a structural validation error via DisallowUnknownFields.
type AgentV6 struct {
	ID string `json:"id"`
}

// DelegationV6 is a directed grant of a non-empty capability set,
// byte-for-byte identical in shape to DelegationV5 — it reuses the plain,
// unmodified Capability{Scope, Target} type, never RootCapabilityV5 and
// never carries a lifecycle. A stray lifecycle key on an authority entry
// here is rejected at decode time via DisallowUnknownFields, with zero new
// validation code (docs/phase-6-plan.md §6).
type DelegationV6 struct {
	Delegator string       `json:"delegator"`
	Delegatee string       `json:"delegatee"`
	Authority []Capability `json:"authority"`
}

// OperationV6 is byte-for-byte identical in shape to OperationV5: lifecycle
// is checked only against the specific approval record(s) relied upon to
// satisfy the approval-required gate, never re-declared on the operation
// itself (docs/phase-6-plan.md §7, §16).
type OperationV6 struct {
	Actor     string `json:"actor"`
	Requester string `json:"requester"`
	Action    string `json:"action"`
	Requires  string `json:"requires"`
	Target    string `json:"target"`
}
