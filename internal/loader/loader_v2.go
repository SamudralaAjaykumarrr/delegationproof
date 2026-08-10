// Version-2 loading: the version-peek dispatch mechanism and structural
// validation for version-2 documents (docs/phase-2-plan.md §9, §10). This
// file is purely additive to loader.go: Load/validate (the version-1 path)
// are not called from here except where explicitly noted, and are not
// modified by anything in this file.
package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/graph"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/model"
)

var targetRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// Document is the result of a version-dispatched load (§9): exactly one of
// V1/V2 is set on success.
type Document struct {
	V1 *model.Model
	V2 *model.ModelV2
}

// versionPeek decodes only the "version" field, permissively, ignoring
// every other top-level key and imposing no field-shape requirements — it
// exists solely to read the version literal before committing to a struct
// shape (§9 step 2).
type versionPeek struct {
	Version string `json:"version"`
}

// LoadDocument reads, version-dispatches, and structurally validates the
// model at path (§9). A version literal of "1" routes through the
// existing, untouched v1 decode+validate path; "2" routes through the new
// v2 path; anything else (including absent, which decodes as "") is a
// single invalid_version validation error.
func LoadDocument(path string) (*Document, *LoadError) {
	data, loadErr := readInputFile(path)
	if loadErr != nil {
		return nil, loadErr
	}

	var vp versionPeek
	if err := json.Unmarshal(data, &vp); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}

	switch vp.Version {
	case "1":
		m, loadErr := decodeAndValidateV1(data)
		if loadErr != nil {
			return nil, loadErr
		}
		return &Document{V1: m}, nil
	case "2":
		m, loadErr := decodeAndValidateV2(data)
		if loadErr != nil {
			return nil, loadErr
		}
		return &Document{V2: m}, nil
	default:
		return nil, &LoadError{Errors: []ValidationError{{
			Kind:    KindInvalidVersion,
			Primary: vp.Version,
			Message: fmt.Sprintf(`version must be "1" or "2", got %q`, vp.Version),
		}}}
	}
}

// readInputFile applies the same file-access and byte-size bound Load
// applies (docs/phase-1-plan.md §7.1), independently, so LoadDocument can
// peek the version before deciding which struct type to decode into.
func readInputFile(path string) ([]byte, *LoadError) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, &LoadError{FileError: fmt.Sprintf("cannot read input file %q: %v", path, err)}
	}
	if info.IsDir() {
		return nil, &LoadError{FileError: fmt.Sprintf("input path %q is a directory, not a file", path)}
	}
	if info.Size() > limits.MaxInputFileSize {
		return nil, &LoadError{Errors: []ValidationError{resourceLimitErr(
			"max_input_file_size", "",
			fmt.Sprintf("input file size %d bytes exceeds max_input_file_size (%d bytes)", info.Size(), limits.MaxInputFileSize),
		)}}
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, &LoadError{FileError: fmt.Sprintf("cannot read input file %q: %v", path, err)}
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, limits.MaxInputFileSize+1))
	if err != nil {
		return nil, &LoadError{FileError: fmt.Sprintf("cannot read input file %q: %v", path, err)}
	}
	if int64(len(data)) > limits.MaxInputFileSize {
		return nil, &LoadError{Errors: []ValidationError{resourceLimitErr(
			"max_input_file_size", "",
			fmt.Sprintf("input file size exceeds max_input_file_size (%d bytes)", limits.MaxInputFileSize),
		)}}
	}
	return data, nil
}

// decodeAndValidateV1 is the version-1 decode+validate path, byte-for-byte
// the same steps Load performs, reusing the same unmodified validate
// function. It exists so LoadDocument does not need to re-read the file
// (readInputFile already ran) to reach the identical result Load(path)
// would produce.
func decodeAndValidateV1(data []byte) (*model.Model, *LoadError) {
	var m model.Model
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if dec.More() {
		return nil, &LoadError{ParseError: "invalid JSON: unexpected trailing content after top-level document"}
	}

	errs := validate(&m)
	if len(errs) > 0 {
		sortErrors(errs)
		return nil, &LoadError{Errors: errs}
	}
	return &m, nil
}

func decodeAndValidateV2(data []byte) (*model.ModelV2, *LoadError) {
	var m model.ModelV2
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, &LoadError{ParseError: fmt.Sprintf("invalid JSON: %v", err)}
	}
	if dec.More() {
		return nil, &LoadError{ParseError: "invalid JSON: unexpected trailing content after top-level document"}
	}

	errs := validateV2(&m)
	if len(errs) > 0 {
		sortErrors(errs)
		return nil, &LoadError{Errors: errs}
	}
	return &m, nil
}

func validateV2(m *model.ModelV2) []ValidationError {
	var errs []ValidationError

	if m.Version != "2" {
		errs = append(errs, ValidationError{
			Kind:    KindInvalidVersion,
			Primary: m.Version,
			Message: fmt.Sprintf(`version must be "1" or "2", got %q`, m.Version),
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

// checkTarget mirrors checkID's shape: grammar check, then length bound
// (docs/phase-2-plan.md §5, §10, §17). A missing target decodes as "",
// which fails the regex and is reported as invalid_target — the same
// mechanism Phase 1 uses for missing/empty ids and scopes.
func checkTarget(errs *[]ValidationError, context, owner, target string) {
	if !targetRe.MatchString(target) {
		*errs = append(*errs, ValidationError{
			Kind: KindInvalidTarget, Primary: owner, Secondary: target,
			Message: fmt.Sprintf("%s target %q must match ^[A-Za-z0-9_.-]{1,128}$", context, target),
		})
		return
	}
	if len(target) > limits.MaxTargetLength {
		*errs = append(*errs, resourceLimitErr("max_target_length", owner,
			fmt.Sprintf("%s target %q (%d bytes) exceeds max_target_length (%d bytes)", context, target, len(target), limits.MaxTargetLength)))
	}
}

// checkCapabilitySet validates one principal's or one delegation edge's
// capability array: the size bound (reusing MaxAuthoritySetSize, now
// counting tuples instead of bare scopes, §17), each entry's scope/target
// grammar, and duplicate (scope, target) tuples within the array (§10) —
// two entries sharing a scope but differing in target are NOT duplicates.
func checkCapabilitySet(errs *[]ValidationError, contextKind, ownerID string, caps []model.Capability) {
	if len(caps) > limits.MaxAuthoritySetSize {
		*errs = append(*errs, resourceLimitErr("max_authority_set_size", ownerID,
			fmt.Sprintf("%s %q authority set (%d capabilities) exceeds max_authority_set_size (%d)", contextKind, ownerID, len(caps), limits.MaxAuthoritySetSize)))
	}
	seen := map[model.Capability]bool{}
	for _, c := range caps {
		checkScope(errs, contextKind, ownerID, c.Scope)
		checkTarget(errs, contextKind, ownerID, c.Target)
		if seen[c] {
			*errs = append(*errs, ValidationError{
				Kind: KindDuplicateCapability, Primary: ownerID, Secondary: c.Scope + "@" + c.Target,
				Message: fmt.Sprintf("%s %q authority set contains duplicate capability %s@%s", contextKind, ownerID, c.Scope, c.Target),
			})
		}
		seen[c] = true
	}
}
