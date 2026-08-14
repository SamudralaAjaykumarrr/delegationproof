// Package loader parses and structurally validates a DelegationProof Phase 1
// input document (docs/phase-1-plan.md §7). All structural errors are
// collected and reported together, never fail-fast, except for problems
// that prevent building a parseable document at all (file access, JSON
// syntax/schema errors), which are singular by nature.
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

// ErrorKind identifies the category of a structural validation error.
type ErrorKind string

const (
	KindInvalidVersion        ErrorKind = "invalid_version"
	KindInvalidID             ErrorKind = "invalid_id"
	KindInvalidScope          ErrorKind = "invalid_scope"
	KindInvalidAction         ErrorKind = "invalid_action"
	KindDuplicateNodeID       ErrorKind = "duplicate_node_id"
	KindDuplicateEdge         ErrorKind = "duplicate_delegation_edge"
	KindUnknownDelegator      ErrorKind = "unknown_delegator"
	KindUnknownDelegatee      ErrorKind = "unknown_delegatee"
	KindDelegateeIsPrincipal  ErrorKind = "delegatee_is_principal"
	KindSelfDelegation        ErrorKind = "self_delegation"
	KindEmptyAuthority        ErrorKind = "empty_authority"
	KindDuplicateScope        ErrorKind = "duplicate_scope"
	KindUnknownActor          ErrorKind = "unknown_actor"
	KindCycleDetected         ErrorKind = "cycle_detected"
	KindResourceLimitExceeded ErrorKind = "resource_limit_exceeded"

	// Version-2-only kinds (docs/phase-2-plan.md §10).
	KindInvalidTarget       ErrorKind = "invalid_target"
	KindDuplicateCapability ErrorKind = "duplicate_capability"
)

// ValidationError is one structural problem found in an otherwise-decodable
// document.
type ValidationError struct {
	Kind      ErrorKind
	Primary   string
	Secondary string
	Message   string
}

func (e ValidationError) String() string {
	return fmt.Sprintf("[%s] %s: %s", e.Kind, e.Primary, e.Message)
}

// LoadError is the union of everything that can prevent Load from returning
// a usable Model. Exactly one of FileError, ParseError, or a non-empty
// Errors is set.
type LoadError struct {
	FileError  string
	ParseError string
	Errors     []ValidationError
}

func (e *LoadError) Error() string {
	switch {
	case e.FileError != "":
		return e.FileError
	case e.ParseError != "":
		return e.ParseError
	default:
		lines := make([]string, len(e.Errors))
		for i, ve := range e.Errors {
			lines[i] = ve.String()
		}
		return strings.Join(lines, "\n")
	}
}

// RenderText renders the error for stderr display.
func (e *LoadError) RenderText() string {
	switch {
	case e.FileError != "":
		return e.FileError + "\n"
	case e.ParseError != "":
		return e.ParseError + "\n"
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "validation failed: %d error(s)\n", len(e.Errors))
		for _, ve := range e.Errors {
			b.WriteString(ve.String())
			b.WriteString("\n")
		}
		return b.String()
	}
}

var (
	idRe     = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
	scopeRe  = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,256}$`)
	actionRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
)

// Load reads, parses, and structurally validates the model at path.
func Load(path string) (*model.Model, *LoadError) {
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

type nodeInfo struct {
	kind string // "principal" or "agent"
}

func validate(m *model.Model) []ValidationError {
	var errs []ValidationError

	if m.Version != "1" {
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
		checkAuthorityScopes(&errs, "principal", p.ID, p.Authority)
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
		checkAuthorityScopes(&errs, "delegation", d.Delegator+"->"+d.Delegatee, d.Authority)

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

func checkID(errs *[]ValidationError, context, id string) {
	if !idRe.MatchString(id) {
		*errs = append(*errs, ValidationError{
			Kind: KindInvalidID, Primary: id, Secondary: context,
			Message: fmt.Sprintf("%s id %q must match ^[A-Za-z0-9_.-]{1,128}$", context, id),
		})
		return
	}
	if len(id) > limits.MaxIDLength {
		*errs = append(*errs, resourceLimitErr("max_id_length", id,
			fmt.Sprintf("%s id %q (%d bytes) exceeds max_id_length (%d bytes)", context, id, len(id), limits.MaxIDLength)))
	}
}

func checkScope(errs *[]ValidationError, context, owner, scope string) {
	if !scopeRe.MatchString(scope) {
		*errs = append(*errs, ValidationError{
			Kind: KindInvalidScope, Primary: owner, Secondary: scope,
			Message: fmt.Sprintf("%s scope %q must match ^[A-Za-z0-9_.:-]{1,256}$", context, scope),
		})
		return
	}
	if len(scope) > limits.MaxScopeLength {
		*errs = append(*errs, resourceLimitErr("max_scope_length", owner,
			fmt.Sprintf("%s scope %q (%d bytes) exceeds max_scope_length (%d bytes)", context, scope, len(scope), limits.MaxScopeLength)))
	}
}

func checkAction(errs *[]ValidationError, owner, action string) {
	if !actionRe.MatchString(action) {
		*errs = append(*errs, ValidationError{
			Kind: KindInvalidAction, Primary: owner, Secondary: action,
			Message: fmt.Sprintf("operation action %q must match ^[A-Za-z0-9_.-]{1,128}$", action),
		})
	}
}

func checkAuthorityScopes(errs *[]ValidationError, contextKind, ownerID string, scopes []string) {
	if len(scopes) > limits.MaxAuthoritySetSize {
		*errs = append(*errs, resourceLimitErr("max_authority_set_size", ownerID,
			fmt.Sprintf("%s %q authority set (%d scopes) exceeds max_authority_set_size (%d)", contextKind, ownerID, len(scopes), limits.MaxAuthoritySetSize)))
	}
	seen := map[string]bool{}
	for _, s := range scopes {
		checkScope(errs, contextKind, ownerID, s)
		if seen[s] {
			*errs = append(*errs, ValidationError{
				Kind: KindDuplicateScope, Primary: ownerID, Secondary: s,
				Message: fmt.Sprintf("%s %q authority set contains duplicate scope %q", contextKind, ownerID, s),
			})
		}
		seen[s] = true
	}
}

func resourceLimitErr(limitName, secondary, message string) ValidationError {
	return ValidationError{Kind: KindResourceLimitExceeded, Primary: limitName, Secondary: secondary, Message: message}
}

func sortErrors(errs []ValidationError) {
	sort.SliceStable(errs, func(i, j int) bool {
		a, b := errs[i], errs[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Primary != b.Primary {
			return a.Primary < b.Primary
		}
		return a.Secondary < b.Secondary
	})
}
