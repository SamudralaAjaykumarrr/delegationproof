package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
)

func TestValidV6FixturesLoadCleanly(t *testing.T) {
	files := []string{
		"clean-pass-v6.json",
		"clean-pass-v6-reordered.json",
		"combined-violations-v6.json",
		"unsafe-lifecycle.json",
		"unproven-lifecycle.json",
		"multi-approver.json",
	}
	for _, f := range files {
		f := f
		t.Run(f, func(t *testing.T) {
			doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "valid-v6", f))
			if loadErr != nil {
				t.Fatalf("unexpected load error: %s", loadErr.RenderText())
			}
			if doc.V6 == nil {
				t.Fatal("expected a V6 document")
			}
			if doc.V1 != nil || doc.V2 != nil || doc.V3 != nil || doc.V4 != nil || doc.V5 != nil {
				t.Fatal("expected V1-V5 to be nil for a version-6 document")
			}
		})
	}
}

func TestExampleV6FixtureLoadsCleanly(t *testing.T) {
	doc, loadErr := LoadDocument("../../examples/billing-approval-lifecycle.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V6 == nil {
		t.Fatal("expected a V6 document")
	}
}

func TestVersionDispatchV6(t *testing.T) {
	t.Run("version 6 routes to V6", func(t *testing.T) {
		doc, loadErr := LoadDocument("../../testdata/valid-v6/clean-pass-v6.json")
		if loadErr != nil {
			t.Fatalf("unexpected load error: %s", loadErr.RenderText())
		}
		if doc.V6 == nil || doc.V1 != nil || doc.V2 != nil || doc.V3 != nil || doc.V4 != nil || doc.V5 != nil {
			t.Fatalf("expected V6 set, V1-V5 nil, got %+v", doc)
		}
	})
	t.Run("unrecognized version yields six-literal invalid_version message", func(t *testing.T) {
		path := writeTemp(t, `{"version":"9","principals":[],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		if loadErr == nil || len(loadErr.Errors) != 1 {
			t.Fatalf("expected exactly 1 validation error, got %+v", loadErr)
		}
		ve := loadErr.Errors[0]
		if ve.Kind != KindInvalidVersion {
			t.Errorf("Kind = %q, want %q", ve.Kind, KindInvalidVersion)
		}
		want := `version must be "1", "2", "3", "4", "5", or "6", got "9"`
		if ve.Message != want {
			t.Errorf("Message = %q, want %q", ve.Message, want)
		}
	})
}

func TestMalformedFixturesV6(t *testing.T) {
	cases := []struct {
		file string
		kind ErrorKind
	}{
		{"unknown-lifecycle-state.json", KindUnknownLifecycleState},
		{"duplicate-lifecycle-state.json", KindDuplicateLifecycleState},
		{"duplicate-lifecycle-transition.json", KindDuplicateLifecycleTransition},
		{"empty-lifecycle-states.json", KindEmptyLifecycleStates},
	}
	for _, c := range cases {
		c := c
		t.Run(c.file, func(t *testing.T) {
			doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", c.file))
			if loadErr == nil {
				t.Fatalf("expected a load error, got valid document %+v", doc)
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

func TestUnknownLifecycleStateSubCases(t *testing.T) {
	base := func(lifecycle string) string {
		return `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle": ` + lifecycle + `} ],
			"operations": []
		}`
	}
	cases := []struct {
		name      string
		lifecycle string
	}{
		{"empty initial", `{"initial": "", "states": ["approved"], "transitions": []}`},
		{"initial not declared", `{"initial": "nonexistent", "states": ["approved"], "transitions": []}`},
		{"transition from undeclared", `{"initial": "approved", "states": ["approved"], "transitions": [{"from": "ghost", "to": "approved", "event": "x"}]}`},
		{"transition to undeclared", `{"initial": "approved", "states": ["approved"], "transitions": [{"from": "approved", "to": "ghost", "event": "x"}]}`},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			path := writeTemp(t, base(c.lifecycle))
			_, loadErr := LoadDocument(path)
			if loadErr == nil {
				t.Fatal("expected a load error")
			}
			found := false
			for _, ve := range loadErr.Errors {
				if ve.Kind == KindUnknownLifecycleState {
					found = true
				}
			}
			if !found {
				t.Errorf("expected an unknown_lifecycle_state error, got %+v", loadErr.Errors)
			}
		})
	}
}

func TestLegalLifecycleTransitionsAcceptedWithoutError(t *testing.T) {
	t.Run("self-loop", func(t *testing.T) {
		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
				{"initial": "approved", "states": ["approved"], "transitions": [{"from": "approved", "to": "approved", "event": "reapprove"}]}
			} ],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		if loadErr != nil {
			t.Fatalf("a self-loop must be structurally legal: %s", loadErr.RenderText())
		}
	})
	t.Run("multi-state cycle", func(t *testing.T) {
		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
				{"initial": "a", "states": ["a", "b", "c"], "transitions": [
					{"from": "a", "to": "b"}, {"from": "b", "to": "c"}, {"from": "c", "to": "a"}
				]}
			} ],
			"operations": []
		}`)
		doc, loadErr := LoadDocument(path)
		if loadErr != nil {
			t.Fatalf("a multi-state cycle must be structurally legal (no acyclicity check over a lifecycle): %s", loadErr.RenderText())
		}
		if doc.V6 == nil {
			t.Fatal("expected a V6 document")
		}
	})
}

func TestNoLifecycleFieldBehavesExactlyLikeV5(t *testing.T) {
	path := writeTemp(t, `{
		"version": "6",
		"principals": [{"id": "root", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": false}
		]}],
		"agents": [],
		"delegations": [],
		"approvals": [],
		"operations": []
	}`)
	doc, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V6 == nil {
		t.Fatal("expected a V6 document")
	}
}

func TestLifecycleFieldOnDelegationIsDecodeLevelError(t *testing.T) {
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "lifecycle-field-on-delegation.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestLifecycleFieldOnOperationIsDecodeLevelError(t *testing.T) {
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "lifecycle-field-on-operation.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestLifecycleFieldOnRootCapabilityIsDecodeLevelError(t *testing.T) {
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "lifecycle-field-on-root-capability.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestStrayFieldInsideLifecycleObjectIsRejected(t *testing.T) {
	path := writeTemp(t, `{
		"version": "6",
		"principals": [{"id": "admin", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
		]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
			{"initial": "approved", "states": ["approved"], "transitions": [], "unexpected_field": true}
		} ],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestUnknownApproverStillFiresWithLifecycleDeclared(t *testing.T) {
	path := writeTemp(t, `{
		"version": "6",
		"principals": [{"id": "admin", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
		]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"approver": "nonexistent", "scope": "a", "target": "svc", "lifecycle":
			{"initial": "approved", "states": ["approved"], "transitions": []}
		} ],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	found := false
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindUnknownApprover {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an unknown_approver error, got %+v", loadErr.Errors)
	}
}

func TestResourceLimitsV6(t *testing.T) {
	t.Run("max_lifecycle_states exact boundary is valid", func(t *testing.T) {
		orig := limits.MaxLifecycleStates
		limits.MaxLifecycleStates = 3
		defer func() { limits.MaxLifecycleStates = orig }()

		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
				{"initial": "s1", "states": ["s1", "s2", "s3"], "transitions": []}
			} ],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		if loadErr != nil {
			t.Fatalf("unexpected load error at exact limits.MaxLifecycleStates boundary: %s", loadErr.RenderText())
		}
	})

	t.Run("max_lifecycle_states exceeded", func(t *testing.T) {
		orig := limits.MaxLifecycleStates
		limits.MaxLifecycleStates = 3
		defer func() { limits.MaxLifecycleStates = orig }()

		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
				{"initial": "s1", "states": ["s1", "s2", "s3", "s4"], "transitions": []}
			} ],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_lifecycle_states")
	})

	t.Run("max_lifecycle_transitions exact boundary is valid", func(t *testing.T) {
		orig := limits.MaxLifecycleTransitions
		limits.MaxLifecycleTransitions = 2
		defer func() { limits.MaxLifecycleTransitions = orig }()

		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
				{"initial": "s1", "states": ["s1", "s2"], "transitions": [
					{"from": "s1", "to": "s2", "event": "e1"},
					{"from": "s1", "to": "s2", "event": "e2"}
				]}
			} ],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		if loadErr != nil {
			t.Fatalf("unexpected load error at exact limits.MaxLifecycleTransitions boundary: %s", loadErr.RenderText())
		}
	})

	t.Run("max_lifecycle_transitions exceeded", func(t *testing.T) {
		orig := limits.MaxLifecycleTransitions
		limits.MaxLifecycleTransitions = 2
		defer func() { limits.MaxLifecycleTransitions = orig }()

		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
				{"initial": "s1", "states": ["s1", "s2"], "transitions": [
					{"from": "s1", "to": "s2", "event": "e1"},
					{"from": "s1", "to": "s2", "event": "e2"},
					{"from": "s1", "to": "s2", "event": "e3"}
				]}
			} ],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_lifecycle_transitions")
	})

	t.Run("max_approvals unchanged from v5", func(t *testing.T) {
		orig := limits.MaxApprovals
		limits.MaxApprovals = 2
		defer func() { limits.MaxApprovals = orig }()

		path := writeTemp(t, `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}]}],
			"agents": [{"id":"x1"},{"id":"x2"},{"id":"x3"}],
			"delegations": [],
			"approvals": [
				{"approver": "x1", "scope": "a", "target": "svc"},
				{"approver": "x2", "scope": "a", "target": "svc"},
				{"approver": "x3", "scope": "a", "target": "svc"}
			],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_approvals")
	})
}

func TestDuplicateLifecycleTransitionAllowsDifferentEvents(t *testing.T) {
	// Two transitions sharing only (from, to) but different event labels
	// are not a duplicate — branching with distinctly-labeled alternatives
	// is legal and expected.
	path := writeTemp(t, `{
		"version": "6",
		"principals": [{"id": "admin", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
		]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
			{"initial": "approved", "states": ["approved", "revoked"], "transitions": [
				{"from": "approved", "to": "revoked", "event": "revoke"},
				{"from": "approved", "to": "revoked", "event": "void"}
			]}
		} ],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("two transitions differing only by event must not be rejected as duplicates: %s", loadErr.RenderText())
	}
}

func TestBoundaryStateNameAtMaxTargetLength(t *testing.T) {
	long := ""
	for i := 0; i < 128; i++ {
		long += "s"
	}
	path := writeTemp(t, `{
		"version": "6",
		"principals": [{"id": "admin", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
		]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"approver": "admin", "scope": "a", "target": "svc", "lifecycle":
			{"initial": "`+long+`", "states": ["`+long+`"], "transitions": []}
		} ],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("a state name at exactly the 128-byte boundary must be valid: %s", loadErr.RenderText())
	}
}

func TestValidationDiagnosticDeterminismV6(t *testing.T) {
	// docs/phase-6-plan.md §33 test 63: a document with two or more
	// simultaneous v6 structural errors (here, two independent
	// unknown_lifecycle_state violations in different approval records)
	// produces identical, sorted ValidationError order across repeated
	// runs and across array-reordered input, via the unmodified existing
	// sortErrors mechanism.
	build := func(reversed bool) string {
		approvals := []string{
			`{"approver": "admin", "scope": "a", "target": "svc", "lifecycle": {"initial": "ghost-1", "states": ["approved"], "transitions": []}}`,
			`{"approver": "admin", "scope": "b", "target": "svc", "lifecycle": {"initial": "ghost-2", "states": ["approved"], "transitions": []}}`,
		}
		if reversed {
			approvals[0], approvals[1] = approvals[1], approvals[0]
		}
		return `{
			"version": "6",
			"principals": [{"id": "admin", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true},
				{"scope": "b", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
			]}],
			"agents": [],
			"delegations": [],
			"approvals": [` + approvals[0] + `, ` + approvals[1] + `],
			"operations": []
		}`
	}

	path1 := writeTemp(t, build(false))
	path2 := writeTemp(t, build(true))

	_, err1a := LoadDocument(path1)
	_, err1b := LoadDocument(path1)
	_, err2 := LoadDocument(path2)

	if err1a == nil || err1b == nil || err2 == nil {
		t.Fatalf("expected load errors, got %+v, %+v, %+v", err1a, err1b, err2)
	}
	if len(err1a.Errors) != len(err1b.Errors) || len(err1a.Errors) != len(err2.Errors) {
		t.Fatalf("error counts differ: %d, %d, %d", len(err1a.Errors), len(err1b.Errors), len(err2.Errors))
	}
	for i := range err1a.Errors {
		if err1a.Errors[i] != err1b.Errors[i] {
			t.Errorf("repeated runs produced different error order at index %d: %+v vs %+v", i, err1a.Errors[i], err1b.Errors[i])
		}
		if err1a.Errors[i] != err2.Errors[i] {
			t.Errorf("reordered approvals[] produced different error order at index %d: %+v vs %+v", i, err1a.Errors[i], err2.Errors[i])
		}
	}
}

func TestNoPanicOnMutatedInputV6(t *testing.T) {
	seeds := []string{
		"../../testdata/valid-v6/combined-violations-v6.json",
		"../../examples/billing-approval-lifecycle.json",
	}
	for _, seed := range seeds {
		orig, err := os.ReadFile(seed)
		if err != nil {
			t.Fatalf("reading seed fixture %s: %v", seed, err)
		}
		for n := 1; n < len(orig); n += 13 {
			data := orig[:n]
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("LoadDocument panicked on truncated %s at %d bytes: %v", seed, n, r)
					}
				}()
				LoadDocument(writeTempBytes(t, data))
			}()
		}
	}
}
