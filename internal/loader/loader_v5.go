// Version-5 loading: structural validation for version-5 documents
// (docs/phase-5-plan.md §5, §6, §7, §16). This file is purely additive: the
// version-1/2/3/4 decode+validate paths (loader.go, loader_v2.go,
// loader_v3.go, loader_v4.go) are not modified by anything in this file
// except the four sanctioned invalid_version message-text touches already
// made in those files.
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

// New version-5-only structural error kinds (docs/phase-5-plan.md §16).
const (
	// KindMissingApprovalRequirement covers exactly one condition: a
	// RootCapabilityV5's RequiresApproval is nil (the key was omitted).
	// There is no "negative"/out-of-range sub-case for a boolean field,
	// unlike max_delegation_depth, mirroring the "one kind, one clear
	// condition" discipline unknown_requester/unknown_approver already
	// establish for their own single conditions.
	KindMissingApprovalRequirement ErrorKind = "missing_approval_requirement"

	// KindUnknownApprover mirrors unknown_requester/unknown_actor
	// precisely: approvals[].approver does not resolve to a known
	// principal or agent id. A missing approver (decodes as "") or a
	// syntactically-malformed one both fall into this same kind.
	KindUnknownApprover ErrorKind = "unknown_approver"

	// KindDuplicateApproval fires when two entries within approvals[]
	// share the exact same (approver, scope, target) triple. Two entries
	// sharing only scope/target but naming different approvers are not a
	// duplicate — a real, legitimate case (§7).
	KindDuplicateApproval ErrorKind = "duplicate_approval"
)

func decodeAndValidateV5(data []byte) (*model.ModelV5, *LoadError) {
	var m model.ModelV5
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if dec.More() {
		return nil, &LoadError{ParseError: "invalid JSON: unexpected trailing content after top-level document"}
	}

	errs := validateV5(&m)
	if len(errs) > 0 {
		sortErrors(errs)
		return nil, &LoadError{Errors: errs}
	}
	return &m, nil
}

// validateV5 is byte-for-byte the same structural checks validateV4
// performs (agents/delegations/operations are identical in shape to their
// v4 counterparts), except principals' capability sets are validated via
// checkRootCapabilitySetV5 instead of checkRootCapabilitySet (to
// additionally check each entry's RequiresApproval pointer, §6, §16), plus
// one new check over the new top-level approvals array (§7, §16).
func validateV5(m *model.ModelV5) []ValidationError {
	var errs []ValidationError

	if m.Version != "5" {
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

	checkApprovals(&errs, nodes, m.Approvals)

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

// checkRootCapabilitySetV5 validates one principal's root capability array
// (docs/phase-5-plan.md §6, §16): the size bound and (scope, target)
// grammar/duplicate checks exactly as checkRootCapabilitySet already
// performs (duplicate detection is projected onto (scope, target) only —
// two entries sharing that pair are a duplicate_capability regardless of
// whether their max_delegation_depth or requires_approval values agree,
// §6), plus each entry's MaxDelegationDepth (unchanged from v4) and
// RequiresApproval: nil (missing) is missing_approval_requirement.
func checkRootCapabilitySetV5(errs *[]ValidationError, contextKind, ownerID string, caps []model.RootCapabilityV5) {
	if len(caps) > limits.MaxAuthoritySetSize {
		*errs = append(*errs, resourceLimitErr("max_authority_set_size", ownerID,
			fmt.Sprintf("%s %q authority set (%d capabilities) exceeds max_authority_set_size (%d)", contextKind, ownerID, len(caps), limits.MaxAuthoritySetSize)))
	}
	seen := map[model.Capability]bool{}
	for _, c := range caps {
		checkScope(errs, contextKind, ownerID, c.Scope)
		checkTarget(errs, contextKind, ownerID, c.Target)

		key := model.Capability{Scope: c.Scope, Target: c.Target}
		if seen[key] {
			*errs = append(*errs, ValidationError{
				Kind: KindDuplicateCapability, Primary: ownerID, Secondary: c.Scope + "@" + c.Target,
				Message: fmt.Sprintf("%s %q authority set contains duplicate capability %s@%s", contextKind, ownerID, c.Scope, c.Target),
			})
		}
		seen[key] = true

		switch {
		case c.MaxDelegationDepth == nil:
			*errs = append(*errs, ValidationError{
				Kind: KindInvalidDelegationDepth, Primary: ownerID, Secondary: c.Scope + "@" + c.Target,
				Message: fmt.Sprintf("%s %q capability %s@%s is missing required field max_delegation_depth", contextKind, ownerID, c.Scope, c.Target),
			})
		case *c.MaxDelegationDepth < 0:
			*errs = append(*errs, ValidationError{
				Kind: KindInvalidDelegationDepth, Primary: ownerID, Secondary: c.Scope + "@" + c.Target,
				Message: fmt.Sprintf("%s %q capability %s@%s max_delegation_depth %d must not be negative", contextKind, ownerID, c.Scope, c.Target, *c.MaxDelegationDepth),
			})
		case *c.MaxDelegationDepth > limits.MaxDelegationDepth:
			*errs = append(*errs, resourceLimitErr("max_delegation_depth", ownerID,
				fmt.Sprintf("%s %q capability %s@%s max_delegation_depth %d exceeds max_delegation_depth (%d)", contextKind, ownerID, c.Scope, c.Target, *c.MaxDelegationDepth, limits.MaxDelegationDepth)))
		}

		if c.RequiresApproval == nil {
			*errs = append(*errs, ValidationError{
				Kind: KindMissingApprovalRequirement, Primary: ownerID, Secondary: c.Scope + "@" + c.Target,
				Message: fmt.Sprintf("%s %q capability %s@%s is missing required field requires_approval", contextKind, ownerID, c.Scope, c.Target),
			})
		}
	}
}

// checkApprovals validates the top-level approvals array (docs/phase-5-plan.md
// §7, §16): each entry's approver must resolve to a known node id, scope/target
// must match the unchanged Phase 2 capability grammar, and no two entries may
// share the exact same (approver, scope, target) triple. An approval naming a
// (scope, target) no principal ever declared is not a structural error (§7) —
// it is simply inert, checked at verify time only.
func checkApprovals(errs *[]ValidationError, nodes map[string]*nodeInfo, approvals []model.ApprovalV5) {
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
	}
}
