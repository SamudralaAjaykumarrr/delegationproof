// Version-4 loading: structural validation for version-4 documents
// (docs/phase-4-plan.md §5, §6, §17). This file is purely additive: the
// version-1/2/3 decode+validate paths (loader.go, loader_v2.go, loader_v3.go)
// are not modified by anything in this file except the three sanctioned
// invalid_version message-text touches already made in those files.
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

// KindInvalidDelegationDepth is the one new version-4 structural error kind
// (docs/phase-4-plan.md §17): a RootCapability's max_delegation_depth is
// either absent (decodes as a nil *int) or present but negative. Both share
// one kind, mirroring how unknown_requester already covers both "missing"
// and "malformed" for a single underlying reason
// (docs/phase-3-plan.md §15).
const KindInvalidDelegationDepth ErrorKind = "invalid_delegation_depth"

func decodeAndValidateV4(data []byte) (*model.ModelV4, *LoadError) {
	var m model.ModelV4
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if dec.More() {
		return nil, &LoadError{ParseError: "invalid JSON: unexpected trailing content after top-level document"}
	}

	errs := validateV4(&m)
	if len(errs) > 0 {
		sortErrors(errs)
		return nil, &LoadError{Errors: errs}
	}
	return &m, nil
}

// validateV4 is byte-for-byte the same structural checks validateV3
// performs (agents/delegations/operations are identical in shape to their
// v3 counterparts), except principals' capability sets are validated via
// checkRootCapabilitySet instead of checkCapabilitySet, to additionally
// check each entry's MaxDelegationDepth (§6, §17).
func validateV4(m *model.ModelV4) []ValidationError {
	var errs []ValidationError

	if m.Version != "4" {
		errs = append(errs, ValidationError{
			Kind:    KindInvalidVersion,
			Primary: m.Version,
			Message: fmt.Sprintf(`version must be "1", "2", "3", or "4", got %q`, m.Version),
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
		checkRootCapabilitySet(&errs, "principal", p.ID, p.Authority)
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

// checkRootCapabilitySet validates one principal's root capability array
// (docs/phase-4-plan.md §6, §17): the size bound and (scope, target)
// grammar/duplicate checks exactly as checkCapabilitySet already performs
// (duplicate detection is projected onto (scope, target) only — two entries
// sharing that pair are a duplicate_capability regardless of whether their
// max_delegation_depth values agree, §17), plus each entry's
// MaxDelegationDepth: nil (missing) or negative is invalid_delegation_depth;
// a present, non-negative value exceeding limits.MaxDelegationDepth is a
// resource_limit_exceeded error, reusing the existing generic mechanism.
func checkRootCapabilitySet(errs *[]ValidationError, contextKind, ownerID string, caps []model.RootCapability) {
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
	}
}
