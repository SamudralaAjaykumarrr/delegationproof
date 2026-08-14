package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
)

func TestValidV4FixturesLoadCleanly(t *testing.T) {
	files := []string{
		"clean-pass-v4.json",
		"clean-pass-v4-reordered.json",
		"combined-violations-v4.json",
		"multi-path-depth.json",
	}
	for _, f := range files {
		f := f
		t.Run(f, func(t *testing.T) {
			doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "valid-v4", f))
			if loadErr != nil {
				t.Fatalf("unexpected load error: %s", loadErr.RenderText())
			}
			if doc.V4 == nil {
				t.Fatal("expected a V4 document")
			}
			if doc.V1 != nil || doc.V2 != nil || doc.V3 != nil {
				t.Fatal("expected V1/V2/V3 to be nil for a version-4 document")
			}
		})
	}
}

func TestExampleV4FixtureLoadsCleanly(t *testing.T) {
	doc, loadErr := LoadDocument("../../examples/billing-redelegation-depth.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V4 == nil {
		t.Fatal("expected a V4 document")
	}
}

func TestVersionDispatchV4(t *testing.T) {
	t.Run("version 4 routes to V4", func(t *testing.T) {
		doc, loadErr := LoadDocument("../../testdata/valid-v4/clean-pass-v4.json")
		if loadErr != nil {
			t.Fatalf("unexpected load error: %s", loadErr.RenderText())
		}
		if doc.V4 == nil || doc.V1 != nil || doc.V2 != nil || doc.V3 != nil {
			t.Fatalf("expected V4 set, V1/V2/V3 nil, got %+v", doc)
		}
	})
	t.Run("unrecognized version yields four-literal invalid_version message", func(t *testing.T) {
		path := writeTemp(t, `{"version":"9","principals":[],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		if loadErr == nil || len(loadErr.Errors) != 1 {
			t.Fatalf("expected exactly 1 validation error, got %+v", loadErr)
		}
		ve := loadErr.Errors[0]
		if ve.Kind != KindInvalidVersion {
			t.Errorf("Kind = %q, want %q", ve.Kind, KindInvalidVersion)
		}
		want := `version must be "1", "2", "3", "4", or "5", got "9"`
		if ve.Message != want {
			t.Errorf("Message = %q, want %q", ve.Message, want)
		}
	})
}

func TestMalformedFixturesV4(t *testing.T) {
	cases := []struct {
		file string
		kind ErrorKind
	}{
		{"missing-delegation-depth.json", KindInvalidDelegationDepth},
		{"negative-delegation-depth.json", KindInvalidDelegationDepth},
		{"delegation-depth-exceeds-max.json", KindResourceLimitExceeded},
		{"duplicate-root-capability-different-depths.json", KindDuplicateCapability},
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

func TestMissingDelegationDepthDecodesAsNilPointer(t *testing.T) {
	// §17: a missing max_delegation_depth key decodes as a nil *int, which
	// is rejected as invalid_delegation_depth — distinct from a present,
	// explicit 0 (§6).
	doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "missing-delegation-depth.json"))
	if loadErr == nil {
		t.Fatalf("expected a load error, got valid document %+v", doc)
	}
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindInvalidDelegationDepth {
			return
		}
	}
	t.Errorf("expected an invalid_delegation_depth error, got %+v", loadErr.Errors)
}

func TestNonIntegerDelegationDepthIsDecodeLevelError(t *testing.T) {
	// §17: a non-integer JSON number for max_delegation_depth is a decode
	// error (ParseError), not a validateV4 structural error — zero new code.
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "non-integer-delegation-depth.json"))
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

func TestDepthFieldOnDelegationIsDecodeLevelError(t *testing.T) {
	// §17: DelegationV4.Authority uses the plain Capability{Scope, Target}
	// type — no max_delegation_depth field exists there at all, so a stray
	// key is rejected by DisallowUnknownFields at decode time.
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "depth-field-on-delegation.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestDepthFieldOnOperationIsDecodeLevelError(t *testing.T) {
	// §17: OperationV4 has no capability-object field at all — a stray
	// max_delegation_depth key is rejected by DisallowUnknownFields.
	_, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "depth-field-on-operation.json"))
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	if loadErr.ParseError == "" {
		t.Errorf("expected a ParseError (decode-level failure), got %+v", loadErr)
	}
}

func TestDuplicateRootCapabilitySameScopeDifferentTargetIsNotADuplicate(t *testing.T) {
	path := writeTemp(t, `{
		"version": "4",
		"principals": [{"id": "user", "authority": [
			{"scope": "billing:read", "target": "billing-service", "max_delegation_depth": 1},
			{"scope": "billing:read", "target": "payroll-service", "max_delegation_depth": 2}
		]}],
		"agents": [],
		"delegations": [],
		"operations": []
	}`)
	doc, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if len(doc.V4.Principals[0].Authority) != 2 {
		t.Fatalf("expected 2 capabilities, got %+v", doc.V4.Principals[0].Authority)
	}
}

func TestMaxDelegationDepthZeroIsStructurallyValid(t *testing.T) {
	// §6: 0 is a legitimate, meaningful declared value (non-delegable but
	// usable), never confused with "missing".
	path := writeTemp(t, `{
		"version": "4",
		"principals": [{"id": "root", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 0}
		]}],
		"agents": [],
		"delegations": [],
		"operations": []
	}`)
	doc, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	got := doc.V4.Principals[0].Authority[0].MaxDelegationDepth
	if got == nil || *got != 0 {
		t.Errorf("MaxDelegationDepth = %v, want pointer to 0", got)
	}
}

func TestMaxDelegationDepthAtExactBoundaryIsValid(t *testing.T) {
	path := writeTemp(t, `{
		"version": "4",
		"principals": [{"id": "root", "authority": [
			{"scope": "a", "target": "svc", "max_delegation_depth": 64}
		]}],
		"agents": [],
		"delegations": [],
		"operations": []
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error at exact limits.MaxDelegationDepth boundary: %s", loadErr.RenderText())
	}
}

func TestResourceLimitsV4(t *testing.T) {
	t.Run("max_delegation_depth", func(t *testing.T) {
		orig := limits.MaxDelegationDepth
		limits.MaxDelegationDepth = 3
		defer func() { limits.MaxDelegationDepth = orig }()

		path := writeTemp(t, `{
			"version": "4",
			"principals": [{"id": "root", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 4}
			]}],
			"agents": [],
			"delegations": [],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_delegation_depth")
	})

	t.Run("max_delegation_depth is independent of max_chain_depth", func(t *testing.T) {
		origDepth := limits.MaxDelegationDepth
		origChain := limits.MaxChainDepth
		limits.MaxDelegationDepth = 100
		defer func() {
			limits.MaxDelegationDepth = origDepth
			limits.MaxChainDepth = origChain
		}()

		path := writeTemp(t, `{
			"version": "4",
			"principals": [{"id": "root", "authority": [
				{"scope": "a", "target": "svc", "max_delegation_depth": 50}
			]}],
			"agents": [],
			"delegations": [],
			"operations": []
		}`)
		_, loadErr := LoadDocument(path)
		if loadErr != nil {
			t.Fatalf("raising max_delegation_depth alone must not be affected by max_chain_depth: %s", loadErr.RenderText())
		}
	})

	t.Run("max_operations applies to v4", func(t *testing.T) {
		orig := limits.MaxOperations
		limits.MaxOperations = 1
		defer func() { limits.MaxOperations = orig }()

		path := writeTemp(t, `{
			"version": "4",
			"principals": [{"id": "root", "authority": [{"scope": "read", "target": "svc", "max_delegation_depth": 1}]}],
			"agents": [{"id": "agent-a"}],
			"delegations": [{"delegator": "root", "delegatee": "agent-a", "authority": [{"scope": "read", "target": "svc"}]}],
			"operations": [
				{"actor": "agent-a", "requester": "root", "action": "op1", "requires": "read", "target": "svc"},
				{"actor": "agent-a", "requester": "root", "action": "op2", "requires": "read", "target": "svc"}
			]
		}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_operations")
	})
}

func TestNoPanicOnMutatedInputV4(t *testing.T) {
	seeds := []string{
		"../../testdata/valid-v4/combined-violations-v4.json",
		"../../examples/billing-redelegation-depth.json",
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
