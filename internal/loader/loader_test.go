package loader

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
)

const testdataRoot = "../../testdata"

func TestValidFixturesLoadCleanly(t *testing.T) {
	dir := filepath.Join(testdataRoot, "valid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one valid fixture")
	}
	for _, e := range entries {
		e := e
		t.Run(e.Name(), func(t *testing.T) {
			m, loadErr := Load(filepath.Join(dir, e.Name()))
			if loadErr != nil {
				t.Fatalf("unexpected load error: %s", loadErr.RenderText())
			}
			if m == nil {
				t.Fatal("expected non-nil model")
			}
		})
	}
}

func TestExampleFixtureLoadsCleanly(t *testing.T) {
	if _, loadErr := Load("../../examples/billing-refund.json"); loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
}

func TestMalformedFixtures(t *testing.T) {
	// Each fixture name maps to the specific error signature it must
	// produce (docs/phase-1-plan.md §7.4). Fixtures whose problem is
	// detected during JSON decoding itself (syntax errors, unknown
	// fields) surface as ParseError, not a collected ValidationError.
	cases := []struct {
		file       string
		parseError bool
		kind       ErrorKind
	}{
		{"invalid-json-syntax.json", true, ""},
		{"invalid-version-missing.json", false, KindInvalidVersion},
		{"invalid-version-wrong.json", false, KindInvalidVersion},
		{"unknown-top-level-field.json", true, ""},
		{"unknown-nested-field.json", true, ""},
		{"agent-declares-authority.json", true, ""},
		{"invalid-id-format.json", false, KindInvalidID},
		{"invalid-scope-format.json", false, KindInvalidScope},
		{"invalid-action-format.json", false, KindInvalidAction},
		{"duplicate-node-id-principal-agent.json", false, KindDuplicateNodeID},
		{"duplicate-node-id-two-agents.json", false, KindDuplicateNodeID},
		{"duplicate-delegation-edge.json", false, KindDuplicateEdge},
		{"delegatee-is-principal.json", false, KindDelegateeIsPrincipal},
		{"self-delegation.json", false, KindSelfDelegation},
		{"empty-authority-array.json", false, KindEmptyAuthority},
		{"duplicate-scope-in-authority.json", false, KindDuplicateScope},
		{"unknown-delegator.json", false, KindUnknownDelegator},
		{"unknown-delegatee.json", false, KindUnknownDelegatee},
		{"unknown-actor.json", false, KindUnknownActor},
		{"cycle-2-node.json", false, KindCycleDetected},
		{"cycle-3-node.json", false, KindCycleDetected},
		// Version-2 fixtures (docs/phase-2-plan.md §10): Load is the
		// version-1-only decode path, so a v2 document's object-shaped
		// capability entries fail to decode into model.Model's []string
		// authority field — a ParseError, not a specific ErrorKind. The
		// version-2-aware assertions for these same fixtures (exact
		// ErrorKind via LoadDocument) live in loader_v2_test.go.
		{"invalid-target-format.json", true, ""},
		{"missing-target.json", true, ""},
		{"duplicate-capability.json", true, ""},
		{"target-exceeds-max-length.json", true, ""},
		// Version-3 fixtures (docs/phase-3-plan.md §15): Load is the
		// version-1-only decode path, so a v3 document's "requester"/
		// "target" operation fields are unknown fields to model.Operation
		// — a ParseError, not a specific ErrorKind. The version-3-aware
		// assertions for these same fixtures (exact ErrorKind via
		// LoadDocument) live in loader_v3_test.go.
		{"unknown-requester.json", true, ""},
		{"missing-requester.json", true, ""},
		// Version-4 fixtures (docs/phase-4-plan.md §17): Load is the
		// version-1-only decode path, so a v4 document's object-shaped
		// authority entries fail to decode into model.Model's []string
		// authority field — a ParseError, not a specific ErrorKind. The
		// version-4-aware assertions for these same fixtures (exact
		// ErrorKind via LoadDocument) live in loader_v4_test.go.
		{"missing-delegation-depth.json", true, ""},
		{"negative-delegation-depth.json", true, ""},
		{"delegation-depth-exceeds-max.json", true, ""},
		{"non-integer-delegation-depth.json", true, ""},
		{"duplicate-root-capability-different-depths.json", true, ""},
		{"depth-field-on-delegation.json", true, ""},
		{"depth-field-on-operation.json", true, ""},
		// Version-5 fixtures (docs/phase-5-plan.md §16): Load is the
		// version-1-only decode path, so a v5 document's object-shaped
		// authority entries and its new top-level "approvals" array are
		// unknown/mismatched fields to model.Model — a ParseError, not a
		// specific ErrorKind. The version-5-aware assertions for these
		// same fixtures (exact ErrorKind via LoadDocument) live in
		// loader_v5_test.go.
		{"missing-approval-requirement.json", true, ""},
		{"non-boolean-approval-requirement.json", true, ""},
		{"unknown-approver.json", true, ""},
		{"duplicate-approval.json", true, ""},
		{"approval-field-on-delegation.json", true, ""},
		{"approval-field-on-operation.json", true, ""},
		{"duplicate-root-capability-different-approval.json", true, ""},
	}

	dir := filepath.Join(testdataRoot, "malformed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	covered := map[string]bool{}
	for _, c := range cases {
		covered[c.file] = true
	}
	for _, e := range entries {
		if !covered[e.Name()] {
			t.Errorf("fixture %s has no test case in this table", e.Name())
		}
	}

	for _, c := range cases {
		c := c
		t.Run(c.file, func(t *testing.T) {
			m, loadErr := Load(filepath.Join(dir, c.file))
			if loadErr == nil {
				t.Fatalf("expected a load error, got valid model %+v", m)
			}
			if c.parseError {
				if loadErr.ParseError == "" {
					t.Fatalf("expected ParseError, got %+v", loadErr)
				}
				return
			}
			if len(loadErr.Errors) == 0 {
				t.Fatalf("expected ValidationErrors, got %+v", loadErr)
			}
			found := false
			for _, ve := range loadErr.Errors {
				if ve.Kind == c.kind {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected an error of kind %q, got %+v", c.kind, loadErr.Errors)
			}
		})
	}
}

func TestValidationErrorString(t *testing.T) {
	ve := ValidationError{Kind: KindInvalidID, Primary: "bad!", Message: "id is malformed"}
	got := ve.String()
	want := `[invalid_id] bad!: id is malformed`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLoadErrorErrorMethod(t *testing.T) {
	t.Run("file error", func(t *testing.T) {
		e := &LoadError{FileError: "cannot read input file"}
		if e.Error() != "cannot read input file" {
			t.Errorf("Error() = %q", e.Error())
		}
	})
	t.Run("parse error", func(t *testing.T) {
		e := &LoadError{ParseError: "invalid JSON: boom"}
		if e.Error() != "invalid JSON: boom" {
			t.Errorf("Error() = %q", e.Error())
		}
	})
	t.Run("validation errors", func(t *testing.T) {
		e := &LoadError{Errors: []ValidationError{
			{Kind: KindInvalidID, Primary: "x", Message: "bad"},
			{Kind: KindInvalidScope, Primary: "y", Message: "also bad"},
		}}
		got := e.Error()
		want := "[invalid_id] x: bad\n[invalid_scope] y: also bad"
		if got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

func TestErrorsSortedBySecondaryKeyToo(t *testing.T) {
	// Two unknown_delegator errors sharing the same Primary (the same
	// unknown delegator id, referenced twice) must fall back to
	// Secondary (the delegatee) for a fully deterministic order.
	path := writeTemp(t, `{
		"version": "1",
		"principals": [{"id": "user", "authority": ["billing:read"]}],
		"agents": [{"id": "agent-a"}, {"id": "agent-b"}],
		"delegations": [
			{"delegator": "ghost", "delegatee": "agent-b", "authority": ["billing:read"]},
			{"delegator": "ghost", "delegatee": "agent-a", "authority": ["billing:read"]}
		],
		"operations": []
	}`)
	_, loadErr := Load(path)
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	var secondaries []string
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindUnknownDelegator {
			secondaries = append(secondaries, ve.Secondary)
		}
	}
	if len(secondaries) != 2 {
		t.Fatalf("expected 2 unknown_delegator errors, got %v", secondaries)
	}
	if secondaries[0] != "agent-a" || secondaries[1] != "agent-b" {
		t.Errorf("secondaries not sorted: %v", secondaries)
	}
}

func TestFileNotFound(t *testing.T) {
	_, loadErr := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if loadErr == nil || loadErr.FileError == "" {
		t.Fatalf("expected a FileError, got %+v", loadErr)
	}
}

func TestDirectoryAsPath(t *testing.T) {
	_, loadErr := Load(t.TempDir())
	if loadErr == nil || loadErr.FileError == "" {
		t.Fatalf("expected a FileError, got %+v", loadErr)
	}
}

func TestTrailingContentAfterDocument(t *testing.T) {
	path := writeTemp(t, `{"version":"1","principals":[],"agents":[],"delegations":[],"operations":[]}{"extra":true}`)
	_, loadErr := Load(path)
	if loadErr == nil || loadErr.ParseError == "" {
		t.Fatalf("expected a ParseError for trailing content, got %+v", loadErr)
	}
}

func TestErrorsAreSortedDeterministically(t *testing.T) {
	// Three independent problems, deliberately declared out of the sort
	// order they must be reported in.
	path := writeTemp(t, `{
		"version": "9",
		"principals": [{"id": "bad id", "authority": []}],
		"agents": [],
		"delegations": [],
		"operations": []
	}`)
	_, loadErr := Load(path)
	if loadErr == nil || len(loadErr.Errors) < 2 {
		t.Fatalf("expected multiple collected errors, got %+v", loadErr)
	}
	for i := 1; i < len(loadErr.Errors); i++ {
		a, b := loadErr.Errors[i-1], loadErr.Errors[i]
		if a.Kind > b.Kind {
			t.Errorf("errors not sorted by kind: %s before %s", a.Kind, b.Kind)
		}
	}
}

func TestResourceLimits(t *testing.T) {
	t.Run("max_input_file_size", func(t *testing.T) {
		orig := limits.MaxInputFileSize
		limits.MaxInputFileSize = 10
		defer func() { limits.MaxInputFileSize = orig }()

		path := writeTemp(t, `{"version":"1","principals":[],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_input_file_size")
	})

	t.Run("max_nodes", func(t *testing.T) {
		orig := limits.MaxNodes
		limits.MaxNodes = 1
		defer func() { limits.MaxNodes = orig }()

		path := writeTemp(t, `{"version":"1","principals":[{"id":"p1","authority":[]}],"agents":[{"id":"a1"}],"delegations":[],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_nodes")
	})

	t.Run("max_delegation_edges", func(t *testing.T) {
		orig := limits.MaxDelegationEdges
		limits.MaxDelegationEdges = 1
		defer func() { limits.MaxDelegationEdges = orig }()

		path := writeTemp(t, `{"version":"1","principals":[{"id":"p1","authority":["a"]}],"agents":[{"id":"a1"},{"id":"a2"}],
			"delegations":[
				{"delegator":"p1","delegatee":"a1","authority":["a"]},
				{"delegator":"p1","delegatee":"a2","authority":["a"]}
			],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_delegation_edges")
	})

	t.Run("max_operations", func(t *testing.T) {
		orig := limits.MaxOperations
		limits.MaxOperations = 1
		defer func() { limits.MaxOperations = orig }()

		path := writeTemp(t, `{"version":"1","principals":[{"id":"p1","authority":["a"]}],"agents":[],"delegations":[],
			"operations":[
				{"actor":"p1","action":"x","requires":"a"},
				{"actor":"p1","action":"y","requires":"a"}
			]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_operations")
	})

	t.Run("max_scope_length", func(t *testing.T) {
		orig := limits.MaxScopeLength
		limits.MaxScopeLength = 3
		defer func() { limits.MaxScopeLength = orig }()

		path := writeTemp(t, `{"version":"1","principals":[{"id":"p1","authority":["abcd"]}],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_scope_length")
	})

	t.Run("max_id_length", func(t *testing.T) {
		orig := limits.MaxIDLength
		limits.MaxIDLength = 3
		defer func() { limits.MaxIDLength = orig }()

		path := writeTemp(t, `{"version":"1","principals":[{"id":"abcd","authority":[]}],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_id_length")
	})

	t.Run("max_authority_set_size", func(t *testing.T) {
		orig := limits.MaxAuthoritySetSize
		limits.MaxAuthoritySetSize = 2
		defer func() { limits.MaxAuthoritySetSize = orig }()

		path := writeTemp(t, `{"version":"1","principals":[{"id":"p1","authority":["a","b","c"]}],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_authority_set_size")
	})

	t.Run("max_chain_depth", func(t *testing.T) {
		orig := limits.MaxChainDepth
		limits.MaxChainDepth = 2
		defer func() { limits.MaxChainDepth = orig }()

		// root -> a1 -> a2 -> a3 : chain depth 3 (edges), exceeds the
		// lowered bound of 2.
		path := writeTemp(t, `{"version":"1",
			"principals":[{"id":"root","authority":["x"]}],
			"agents":[{"id":"a1"},{"id":"a2"},{"id":"a3"}],
			"delegations":[
				{"delegator":"root","delegatee":"a1","authority":["x"]},
				{"delegator":"a1","delegatee":"a2","authority":["x"]},
				{"delegator":"a2","delegatee":"a3","authority":["x"]}
			],"operations":[]}`)
		_, loadErr := Load(path)
		assertResourceLimit(t, loadErr, "max_chain_depth")
	})
}

func assertResourceLimit(t *testing.T, loadErr *LoadError, limitName string) {
	t.Helper()
	if loadErr == nil || len(loadErr.Errors) == 0 {
		t.Fatalf("expected a resource_limit_exceeded error, got %+v", loadErr)
	}
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindResourceLimitExceeded && ve.Primary == limitName {
			return
		}
	}
	t.Errorf("expected resource_limit_exceeded for %q, got %+v", limitName, loadErr.Errors)
}

// TestNoPanicOnMutatedInput feeds truncated and randomly-mutated variants of
// a valid fixture through Load and asserts it never panics, regardless of
// how badly the bytes are mangled.
func TestNoPanicOnMutatedInput(t *testing.T) {
	orig, err := os.ReadFile(filepath.Join(testdataRoot, "valid", "mixed-violations.json"))
	if err != nil {
		t.Fatalf("reading seed fixture: %v", err)
	}

	run := func(name string, data []byte) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Load panicked: %v", r)
				}
			}()
			path := writeTempBytes(t, data)
			Load(path)
		})
	}

	run("empty", nil)
	for n := 1; n < len(orig); n += 7 {
		run("truncated", orig[:n])
	}

	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		mutated := append([]byte(nil), orig...)
		for j := 0; j < 5; j++ {
			mutated[rng.Intn(len(mutated))] = byte(rng.Intn(256))
		}
		run("mutated", mutated)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	return writeTempBytes(t, []byte(content))
}

func writeTempBytes(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing temp fixture: %v", err)
	}
	return path
}
