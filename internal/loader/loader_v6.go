// Version-6 loading: structural validation for version-6 documents
// (docs/phase-6-plan.md §6, §7, §8, §20). This file is purely additive: the
// version-1/2/3/4/5 decode+validate paths (loader.go, loader_v2.go,
// loader_v3.go, loader_v4.go, loader_v5.go) are not modified by anything in
// this file except the five sanctioned invalid_version message-text
// touches already made in those files.
package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
)

// New version-6-only structural error kinds (docs/phase-6-plan.md §20).
const (
	// KindUnknownLifecycleState covers three related conditions, mirroring
	// the "one kind, one clear underlying reason" discipline
	// unknown_requester/unknown_approver already establish: (a)
	// lifecycle.initial is empty or does not match any declared state
	// name; (b) a transition's from does not match any declared state
	// name; (c) a transition's to does not match any declared state name.
	// A missing (empty-string) reference and a syntactically-malformed one
	// both fall into this kind — neither can ever resolve to a declared
	// name either way.
	KindUnknownLifecycleState ErrorKind = "unknown_lifecycle_state"

	// KindDuplicateLifecycleState fires when two entries within one
	// lifecycle.states array share the exact same name.
	KindDuplicateLifecycleState ErrorKind = "duplicate_lifecycle_state"

	// KindDuplicateLifecycleTransition fires when two entries within one
	// lifecycle.transitions array share the exact same (from, event, to)
	// triple. Two transitions sharing only from/to but different event
	// labels are not a duplicate — branching with distinctly-labeled
	// alternatives is legal and expected.
	KindDuplicateLifecycleTransition ErrorKind = "duplicate_lifecycle_transition"

	// KindEmptyLifecycleStates fires when a lifecycle object is present
	// but its states array has zero entries — a lifecycle with no states
	// at all cannot have a valid initial, and is rejected outright rather
	// than silently treated as "no lifecycle declared."
	KindEmptyLifecycleStates ErrorKind = "empty_lifecycle_states"
)

func decodeAndValidateV6(data []byte) (*model.ModelV6, *LoadError) {
	var m model.ModelV6
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if dec.More() {
		return nil, &LoadError{ParseError: "invalid JSON: unexpected trailing content after top-level document"}
	}

	errs := validateV6(&m)
	if len(errs) > 0 {
		sortErrors(errs)
		return nil, &LoadError{Errors: errs}
	}
	return &m, nil
}

// validateV6 is byte-for-byte the same structural checks validateV5
// performs (principals/agents/delegations/operations are identical in
// shape to their v5 counterparts, including principals' RootCapabilityV5
// capability sets, still validated via the unmodified
// checkRootCapabilitySetV5), except the top-level approvals array is
// validated via checkApprovalsV6 instead of checkApprovals, to additionally
// validate each approval record's optional lifecycle (§6, §8, §20).
func validateV6(m *model.ModelV6) []ValidationError {
	var errs []ValidationError

	if m.Version != "6" {
		errs = append(errs, ValidationError{
			Kind:    KindInvalidVersion,
			Primary: m.Version,
			Message: fmt.Sprintf(`version must be "1", "2", "3", "4", "5", or "6", got %q`, m.Version),
		})
	}

	totalNodes := len(m.Principals) + len(m.Agents)
	if totalNodes > limits.MaxNodes {
		errs = append(errs, resourceLimitErr("max_nodes", "",
			fmt.Sprintf("node count %d exceeds max_nodes (%d)", totalNodes, limits.MaxNodes)))
	}
	if len(m.Delegations) > limits.MaxDelegationEdges {
		errs = append(errs, resourceLimitErr("max_delegation_edges", "",
			fmt.Sprintf("delegation edge count %d exceeds max_delegation_edges (%d)", len(m.Delegations), limits.MaxDelegationEdges)))
	}
	if len(m.Operations) > limits.MaxOperations {
		errs = append(errs, resourceLimitErr("max_operations", "",
			fmt.Sprintf("operation count %d exceeds max_operations (%d)", len(m.Operations), limits.MaxOperations)))
	}
	if len(m.Approvals) > limits.MaxApprovals {
		errs = append(errs, resourceLimitErr("max_approvals", "",
			fmt.Sprintf("approval count %d exceeds max_approvals (%d)", len(m.Approvals), limits.MaxApprovals)))
	}

	nodes := map[string]*nodeInfo{}
	registerNode := func(id, kind string) {
		if _, ok := nodes[id]; ok {
			errs = append(errs, ValidationError{
				Kind:      KindDuplicateNodeID,
				Primary:   id,
				Secondary: kind,
				Message:   fmt.Sprintf("id %q is used by more than one node", id),
			})
			return
		}
		nodes[id] = &nodeInfo{kind: kind}
	}

	for _, p := range m.Principals {
		checkID(&errs, "principal", p.ID)
		registerNode(p.ID, "principal")
		checkRootCapabilitySetV5(&errs, "principal", p.ID, p.Authority)
	}
	for _, a := range m.Agents {
		checkID(&errs, "agent", a.ID)
		registerNode(a.ID, "agent")
	}

	seenPairs := map[[2]string]bool{}
	var validEdges []graph.Edge
	for _, d := range m.Delegations {
		_, delegatorKnown := nodes[d.Delegator]
		delegateeNode, delegateeKnown := nodes[d.Delegatee]

		if !delegatorKnown {
			errs = append(errs, ValidationError{
				Kind: KindUnknownDelegator, Primary: d.Delegator, Secondary: d.Delegatee,
				Message: fmt.Sprintf("delegation delegator %q is not a known node id", d.Delegator),
			})
		}
		if !delegateeKnown {
			errs = append(errs, ValidationError{
				Kind: KindUnknownDelegatee, Primary: d.Delegator, Secondary: d.Delegatee,
				Message: fmt.Sprintf("delegation delegatee %q is not a known node id", d.Delegatee),
			})
		}
		if delegateeKnown && delegateeNode.kind == "principal" {
			errs = append(errs, ValidationError{
				Kind: KindDelegateeIsPrincipal, Primary: d.Delegator, Secondary: d.Delegatee,
				Message: fmt.Sprintf("delegatee %q resolves to a principal and cannot be a delegation target", d.Delegatee),
			})
		}
		if d.Delegator != "" && d.Delegator == d.Delegatee {
			errs = append(errs, ValidationError{
				Kind: KindSelfDelegation, Primary: d.Delegator, Secondary: d.Delegatee,
				Message: fmt.Sprintf("delegation from %q to itself is not allowed", d.Delegator),
			})
		}
		if len(d.Authority) == 0 {
			errs = append(errs, ValidationError{
				Kind: KindEmptyAuthority, Primary: d.Delegator, Secondary: d.Delegatee,
				Message: fmt.Sprintf("delegation (%q -> %q) authority must be non-empty", d.Delegator, d.Delegatee),
			})
		}
		checkCapabilitySet(&errs, "delegation", d.Delegator+"->"+d.Delegatee, d.Authority)

		pair := [2]string{d.Delegator, d.Delegatee}
		duplicate := seenPairs[pair]
		if duplicate {
			errs = append(errs, ValidationError{
				Kind: KindDuplicateEdge, Primary: d.Delegator, Secondary: d.Delegatee,
				Message: fmt.Sprintf("duplicate delegation edge (%q -> %q)", d.Delegator, d.Delegatee),
			})
		}
		seenPairs[pair] = true

		structurallySound := delegatorKnown && delegateeKnown && delegateeNode.kind != "principal" &&
			d.Delegator != d.Delegatee && !duplicate
		if structurallySound {
			validEdges = append(validEdges, graph.Edge{From: d.Delegator, To: d.Delegatee})
		}
	}

	checkApprovalsV6(&errs, nodes, m.Approvals)

	for _, op := range m.Operations {
		if _, ok := nodes[op.Actor]; !ok {
			errs = append(errs, ValidationError{
				Kind: KindUnknownActor, Primary: op.Actor, Secondary: op.Action,
				Message: fmt.Sprintf("operation actor %q is not a known node id", op.Actor),
			})
		}
		if _, ok := nodes[op.Requester]; !ok {
			errs = append(errs, ValidationError{
				Kind: KindUnknownRequester, Primary: op.Requester, Secondary: op.Action,
				Message: fmt.Sprintf("operation requester %q is not a known node id", op.Requester),
			})
		}
		checkAction(&errs, op.Actor, op.Action)
		checkScope(&errs, "operation.requires", op.Actor+"/"+op.Action, op.Requires)
		checkTarget(&errs, "operation.target", op.Actor+"/"+op.Action, op.Target)
	}

	nodeIDs := make([]string, 0, len(nodes))
	for id := range nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	order, cyclePath, ok := graph.TopoSort(nodeIDs, validEdges)
	if !ok {
		trace := append(append([]string{}, cyclePath...), cyclePath[0])
		errs = append(errs, ValidationError{
			Kind:      KindCycleDetected,
			Primary:   cyclePath[0],
			Secondary: strings.Join(cyclePath, ","),
			Message:   fmt.Sprintf("delegation graph contains a cycle: %s", strings.Join(trace, " -> ")),
		})
	} else {
		depth := graph.LongestPath(order, validEdges)
		if depth > limits.MaxChainDepth {
			errs = append(errs, resourceLimitErr("max_chain_depth", "",
				fmt.Sprintf("longest delegation chain depth (%d) exceeds max_chain_depth (%d)", depth, limits.MaxChainDepth)))
		}
	}

	return errs
}

// checkApprovalsV6 validates the top-level approvals array exactly as
// checkApprovals (v5) does — approver must resolve to a known node id,
// scope/target must match the unchanged Phase 2 capability grammar, no two
// entries may share the exact same (approver, scope, target) triple — plus
// one new step: each entry's optional lifecycle is validated via
// checkLifecycle (§8, §20).
func checkApprovalsV6(errs *[]ValidationError, nodes map[string]*nodeInfo, approvals []model.ApprovalV6) {
	seen := map[[3]string]bool{}
	for _, a := range approvals {
		if _, ok := nodes[a.Approver]; !ok {
			*errs = append(*errs, ValidationError{
				Kind: KindUnknownApprover, Primary: a.Approver, Secondary: a.Scope + "@" + a.Target,
				Message: fmt.Sprintf("approval approver %q is not a known node id", a.Approver),
			})
		}
		checkScope(errs, "approval", a.Approver, a.Scope)
		checkTarget(errs, "approval", a.Approver, a.Target)

		key := [3]string{a.Approver, a.Scope, a.Target}
		if seen[key] {
			*errs = append(*errs, ValidationError{
				Kind: KindDuplicateApproval, Primary: a.Approver, Secondary: a.Scope + "@" + a.Target,
				Message: fmt.Sprintf("duplicate approval record (%q, %s, %s)", a.Approver, a.Scope, a.Target),
			})
		}
		seen[key] = true

		checkLifecycle(errs, a.Approver, a.Scope+"@"+a.Target, a.Lifecycle)
	}
}

// checkLifecycle validates one approval record's optional lifecycle object
// (docs/phase-6-plan.md §6, §8, §20). A nil lifecycle is valid and requires
// no checks at all (§5.2 — absence is a complete, meaningful declaration on
// its own).
//
// State names (lifecycle.states[]) are *declarations*, validated for
// grammar/length via the unchanged Phase 2 target grammar (checkTarget,
// verbatim, zero new regex) — the identical treatment principal/agent ids
// receive at their own point of declaration. initial/from/to are pure
// *references* into that declared name set — mirroring exactly how
// delegator/delegatee/requester/approver are validated only via "does this
// resolve to a known declaration," never via a second, redundant grammar
// check at the reference site — so a malformed or empty initial/from/to
// falls straight into unknown_lifecycle_state with no separate
// grammar-error kind, identical precedent to unknown_requester/
// unknown_approver. event is the one free-standing, optional annotation
// with nothing to resolve against, so it alone gets an explicit grammar
// check (via checkTarget) when non-empty.
//
// A cyclic lifecycle automaton is never a structural error: unlike the
// delegation graph, no acyclicity/cycle-detection check of any kind is
// ever run over lifecycle.transitions (§8, §11, §20 — there is no such
// check to add).
func checkLifecycle(errs *[]ValidationError, ownerID, capKey string, lc *model.Lifecycle) {
	if lc == nil {
		return
	}

	if len(lc.States) > limits.MaxLifecycleStates {
		*errs = append(*errs, resourceLimitErr("max_lifecycle_states", ownerID,
			fmt.Sprintf("approval %q lifecycle for %s declares %d states, exceeds max_lifecycle_states (%d)", ownerID, capKey, len(lc.States), limits.MaxLifecycleStates)))
	}
	if len(lc.Transitions) > limits.MaxLifecycleTransitions {
		*errs = append(*errs, resourceLimitErr("max_lifecycle_transitions", ownerID,
			fmt.Sprintf("approval %q lifecycle for %s declares %d transitions, exceeds max_lifecycle_transitions (%d)", ownerID, capKey, len(lc.Transitions), limits.MaxLifecycleTransitions)))
	}
	if len(lc.States) == 0 {
		*errs = append(*errs, ValidationError{
			Kind: KindEmptyLifecycleStates, Primary: ownerID, Secondary: capKey,
			Message: fmt.Sprintf("approval %q lifecycle for %s declares a lifecycle with zero states", ownerID, capKey),
		})
	}

	declared := map[string]bool{}
	seenState := map[string]bool{}
	for _, s := range lc.States {
		checkTarget(errs, "lifecycle.state", ownerID, s)
		if seenState[s] {
			*errs = append(*errs, ValidationError{
				Kind: KindDuplicateLifecycleState, Primary: ownerID, Secondary: s,
				Message: fmt.Sprintf("approval %q lifecycle for %s declares duplicate state %q", ownerID, capKey, s),
			})
		}
		seenState[s] = true
		declared[s] = true
	}

	if !declared[lc.Initial] {
		*errs = append(*errs, ValidationError{
			Kind: KindUnknownLifecycleState, Primary: ownerID, Secondary: lc.Initial,
			Message: fmt.Sprintf("approval %q lifecycle for %s initial state %q is not a declared state", ownerID, capKey, lc.Initial),
		})
	}

	seenTransition := map[[3]string]bool{}
	for _, t := range lc.Transitions {
		if !declared[t.From] {
			*errs = append(*errs, ValidationError{
				Kind: KindUnknownLifecycleState, Primary: ownerID, Secondary: t.From,
				Message: fmt.Sprintf("approval %q lifecycle for %s transition references undeclared state %q (from)", ownerID, capKey, t.From),
			})
		}
		if !declared[t.To] {
			*errs = append(*errs, ValidationError{
				Kind: KindUnknownLifecycleState, Primary: ownerID, Secondary: t.To,
				Message: fmt.Sprintf("approval %q lifecycle for %s transition references undeclared state %q (to)", ownerID, capKey, t.To),
			})
		}
		if t.Event != "" {
			checkTarget(errs, "lifecycle.event", ownerID, t.Event)
		}

		key := [3]string{t.From, t.Event, t.To}
		if seenTransition[key] {
			*errs = append(*errs, ValidationError{
				Kind: KindDuplicateLifecycleTransition, Primary: ownerID, Secondary: t.From + "->" + t.To + "@" + t.Event,
				Message: fmt.Sprintf("approval %q lifecycle for %s declares duplicate transition (%q, %q, %q)", ownerID, capKey, t.From, t.Event, t.To),
			})
		}
		seenTransition[key] = true
	}
}
