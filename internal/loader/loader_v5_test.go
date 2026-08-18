package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
)

func TestValidV5FixturesLoadCleanly(t *testing.T) {
	files := []string{
		"clean-pass-v5.json",
		"clean-pass-v5-reordered.json",
		"combined-violations-v5.json",
		"multi-path-approval.json",
		"approval-unauthorized.json",
	}
	for _, f := range files {
		f := f
		t.Run(f, func(t *testing.T) {
			doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "valid-v5", f))
			if loadErr != nil {
				t.Fatalf("unexpected load error: %s", loadErr.RenderText())
			}
			if doc.V5 == nil {
				t.Fatal("expected a V5 document")
			}
			if doc.V1 != nil || doc.V2 != nil || doc.V3 != nil || doc.V4 != nil {
				t.Fatal("expected V1/V2/V3/V4 to be nil for a version-5 document")
			}
		})
	}
}

func TestExampleV5FixtureLoadsCleanly(t *testing.T) {
	doc, loadErr := LoadDocument("../../examples/billing-approval.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V5 == nil {
		t.Fatal("expected a V5 document")
	}
}

func TestVersionDispatchV5(t *testing.T) {
	t.Run("version 5 routes to V5", func(t *testing.T) {
		doc, loadErr := LoadDocument("../../testdata/valid-v5/clean-pass-v5.json")
		if loadErr != nil {
			t.Fatalf("unexpected load error: %s", loadErr.RenderText())
		}
		if doc.V5 == nil || doc.V1 != nil || doc.V2 != nil || doc.V3 != nil || doc.V4 != nil {
			t.Fatalf("expected V5 set, V1/V2/V3/V4 nil, got %+v", doc)
		}
	})
	t.Run("unrecognized version yields five-literal invalid_version message", func(t *testing.T) {
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

func TestMalformedFixturesV5(t *testing.T) {
	cases := []struct {
		file string
		kind ErrorKind
	}{
		{"missing-approval-requirement.json", KindMissingApprovalRequirement},
		{"unknown-approver.json", KindUnknownApprover},
		{"duplicate-approval.json", KindDuplicateApproval},
		{"duplicate-root-capability-different-approval.json", KindDuplicateCapability},
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

func TestMissingApprovalRequirementDecodesAsNilPointer(t *testing.T) {
	// §16: a missing requires_approval key decodes as a nil *bool, which is
	// rejected as missing_approval_requirement.
	doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "missing-approval-requirement.json"))
	if loadErr == nil {
		t.Fatalf("expected a load error, got valid document %+v", doc)
	}
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindMissingApprovalRequirement {
			return
		}
	}
	t.Errorf("expected a missing_approval_requirement error, got %+v", loadErr.Errors)
}

func TestMissingApproverFallsIntoUnknownApprover(t *testing.T) {
	// §16 item 15: an omitted approver key decodes as "", which can never
	// resolve to a known node id — no separate error kind fires.
	path := writeTemp(t, `{
		"version": "5",
		"principals": [{"id": "admin", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}
		]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"scope": "a", "target": "svc"} ],
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

func TestNonBooleanApprovalRequirementIsDecodeLevelError(t *testing.T) {
	// §6: a non-boolean JSON value for requires_approval is a decode error
	// (ParseError), not a validateV5 structural error — zero new code.
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "non-boolean-approval-requirement.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
	if len(loadErr.Errors) != 0 {
		t.Errorf("expected no ValidationErrors for a decode-level failure, got %+v", loadErr.Errors)
	}
}

func TestApprovalFieldOnDelegationIsDecodeLevelError(t *testing.T) {
	// §5: DelegationV5.Authority uses the plain Capability{Scope, Target}
	// type — no requires_approval field exists there at all, so a stray
	// key is rejected by DisallowUnknownFields at decode time.
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "approval-field-on-delegation.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestApprovalFieldOnOperationIsDecodeLevelError(t *testing.T) {
	// §5: OperationV5 has no capability-object field at all — a stray
	// requires_approval key is rejected by DisallowUnknownFields.
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "approval-field-on-operation.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestDuplicateApprovalSameApproverIsRejectedDifferentApproverIsNot(t *testing.T) {
	// §16: two entries sharing (approver, scope, target) are rejected; two
	// entries sharing only (scope, target) but naming different approvers
	// are legal.
	path := writeTemp(t, `{
		"version": "5",
		"principals": [
			{"id": "admin", "authority": [{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}]},
			{"id": "approver-1", "authority": []},
			{"id": "approver-2", "authority": []}
		],
		"agents": [],
		"delegations": [],
		"approvals": [
			{"approver": "approver-1", "scope": "a", "target": "svc"},
			{"approver": "approver-2", "scope": "a", "target": "svc"}
		],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("two approval records differing only by approver must not be rejected as duplicates: %s", loadErr.RenderText())
	}
}

func TestDuplicateRootCapabilityDifferentApprovalIsRejected(t *testing.T) {
	// §6: two RootCapabilityV5 entries sharing (scope, target) are a
	// duplicate regardless of whether their requires_approval values agree.
	doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "duplicate-root-capability-different-approval.json"))
	if loadErr == nil {
		t.Fatalf("expected a load error, got valid document %+v", doc)
	}
	found := false
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindDuplicateCapability {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a duplicate_capability error, got %+v", loadErr.Errors)
	}
}

func TestApprovalReferencingUndeclaredCapabilityIsNotAStructuralError(t *testing.T) {
	// §7: an approval naming a (scope, target) no principal ever declared
	// is inert, not a structural error.
	path := writeTemp(t, `{
		"version": "5",
		"principals": [{"id": "admin", "authority": [{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"approver": "admin", "scope": "never-declared", "target": "svc"} ],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("an approval referencing an undeclared capability must not be a structural error: %s", loadErr.RenderText())
	}
}

func TestSelfApprovalIsStructurallyLegal(t *testing.T) {
	// §7: self-approval is not structurally prohibited.
	path := writeTemp(t, `{
		"version": "5",
		"principals": [{"id": "admin", "authority": [{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}]}],
		"agents": [],
		"delegations": [],
		"approvals": [ {"approver": "admin", "scope": "a", "target": "svc"} ],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("self-approval must be structurally legal: %s", loadErr.RenderText())
	}
}

func TestRequiresApprovalFalseIsStructurallyValid(t *testing.T) {
	path := writeTemp(t, `{
		"version": "5",
		"principals": [{"id": "root", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 0, "requires_approval": false}
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
	got := doc.V5.Principals[0].Authority[0].RequiresApproval
	if got == nil || *got != false {
		t.Errorf("RequiresApproval = %v, want pointer to false", got)
	}
}

func TestResourceLimitsV5(t *testing.T) {
	t.Run("max_approvals", func(t *testing.T) {
		orig := limits.MaxApprovals
		limits.MaxApprovals = 2
		defer func() { limits.MaxApprovals = orig }()

		path := writeTemp(t, `{
			"version": "5",
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

	t.Run("max_approvals at exact boundary is valid", func(t *testing.T) {
		orig := limits.MaxApprovals
		limits.MaxApprovals = 2
		defer func() { limits.MaxApprovals = orig }()

		path := writeTemp(t, `{
			"version": "5",
			"principals": [{"id": "admin", "authority": [{"scope": "a", "target": "svc", "max_delegation_depth": 1, "requires_approval": true}]}],
			"agents": [{"id":"x1"},{"id":"x2"}],
			"delegations": [],
			"approvals": [
				{"approver": "x1", "scope": "a", "target": "svc"},
				{"approver": "x2", "scope": "a", "target": "svc"}
			],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		if loadErr != nil {
			t.Fatalf("unexpected load error at exact limits.MaxApprovals boundary: %s", loadErr.RenderText())
		}
	})
}

func TestNoPanicOnMutatedInputV5(t *testing.T) {
	seeds := []string{
		"../../testdata/valid-v5/combined-violations-v5.json",
		"../../examples/billing-approval.json",
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
