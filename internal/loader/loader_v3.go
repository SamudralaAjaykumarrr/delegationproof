// Version-3 loading: structural validation for version-3 documents
// (docs/phase-3-plan.md §5, §15). This file is purely additive: the
// version-1 and version-2 decode+validate paths (loader.go, loader_v2.go)
// are not modified by anything in this file except the two sanctioned
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

// KindUnknownRequester is the one new version-3 structural error kind
// (docs/phase-3-plan.md §15): requester must resolve to a known node id,
// exactly mirroring unknown_actor. A missing requester (decodes as "") or a
// syntactically-malformed one can never resolve to a known id either, so
// both fall into this same kind — no separate "missing"/"invalid format"
// kind is introduced, identical precedent to how actor is already handled.
const KindUnknownRequester ErrorKind = "unknown_requester"

func decodeAndValidateV3(data []byte) (*model.ModelV3, *LoadError) {
	var m model.ModelV3
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if dec.More() {
		return nil, &LoadError{ParseError: "invalid JSON: unexpected trailing content after top-level document"}
	}

	errs := validateV3(&m)
	if len(errs) > 0 {
		sortErrors(errs)
		return nil, &LoadError{Errors: errs}
	}
	return &m, nil
}

// validateV3 is byte-for-byte the same structural checks validateV2
// performs (principals/agents/delegations are identical in shape to their
// v2 counterparts), plus one new check per operation: requester must
// resolve to a known node id.
func validateV3(m *model.ModelV3) []ValidationError {
	var errs []ValidationError

	if m.Version != "3" {
		errs = append(errs, ValidationError{
			Kind:    KindInvalidVersion,
			Primary: m.Version,
			Message: fmt.Sprintf(`version must be "1", "2", or "3", got %q`, m.Version),
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
		checkCapabilitySet(&errs, "principal", p.ID, p.Authority)
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
