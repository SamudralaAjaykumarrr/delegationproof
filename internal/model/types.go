// Package model defines the DelegationProof Phase 1 domain types: the
// document shape parsed from an input JSON file (see docs/phase-1-plan.md
// §7). Types in this package are pure data — no validation logic lives
// here; see internal/loader for structural validation.
package model

// Model is the whole input document: principals, agents, delegations, and
// operations.
type Model struct {
	Version     string       `json:"version"`
	Principals  []Principal  `json:"principals"`
	Agents      []Agent      `json:"agents"`
	Delegations []Delegation `json:"delegations"`
	Operations  []Operation  `json:"operations"`
}

// Principal is a root authority holder. Its authority is axiomatic
// (declared, not derived) and it cannot be a delegation target.
type Principal struct {
	ID        string   `json:"id"`
	Authority []string `json:"authority"`
}

// Agent is a non-root participant. It deliberately has no Authority field:
// an agent's authority is never declared, only derived from valid incoming
// delegation edges. Decoding with DisallowUnknownFields turns a stray
// "authority" key in an agent object into a structural validation error.
type Agent struct {
	ID string `json:"id"`
}

// Delegation is a directed grant: Delegator -> Delegatee, carrying a
// specific, non-empty authority set.
type Delegation struct {
	Delegator string   `json:"delegator"`
	Delegatee string   `json:"delegatee"`
	Authority []string `json:"authority"`
}

// Operation is a declared point where Actor attempts to exercise the single
// scope Requires. It is the entire Phase 1 representation of "authority
// exercise" — tools/resources/servers are not modeled as graph entities.
type Operation struct {
	Actor    string `json:"actor"`
	Action   string `json:"action"`
	Requires string `json:"requires"`
}
