package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
)

func TestValidV2FixturesLoadCleanly(t *testing.T) {
	files := []string{
		"clean-pass-v2.json",
		"clean-pass-v2-reordered.json",
		"combined-violations.json",
		"multi-hop-context-binding.json",
	}
	for _, f := range files {
		f := f
		t.Run(f, func(t *testing.T) {
			doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "valid-v2", f))
			if loadErr != nil {
				t.Fatalf("unexpected load error: %s", loadErr.RenderText())
			}
			if doc.V2 == nil {
				t.Fatal("expected a V2 document")
			}
			if doc.V1 != nil {
				t.Fatal("expected V1 to be nil for a version-2 document")
			}
		})
	}
}

func TestExampleV2FixtureLoadsCleanly(t *testing.T) {
	doc, loadErr := LoadDocument("../../examples/billing-context-binding.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V2 == nil {
		t.Fatal("expected a V2 document")
	}
}

func TestVersionDispatch(t *testing.T) {
	t.Run("version 1 routes to V1", func(t *testing.T) {
		doc, loadErr := LoadDocument("../../testdata/valid/clean-pass.json")
		if loadErr != nil {
			t.Fatalf("unexpected load error: %s", loadErr.RenderText())
		}
		if doc.V1 == nil || doc.V2 != nil {
			t.Fatalf("expected V1 set, V2 nil, got %+v", doc)
		}
	})
	t.Run("version 2 routes to V2", func(t *testing.T) {
		doc, loadErr := LoadDocument("../../testdata/valid-v2/clean-pass-v2.json")
		if loadErr != nil {
			t.Fatalf("unexpected load error: %s", loadErr.RenderText())
		}
		if doc.V2 == nil || doc.V1 != nil {
			t.Fatalf("expected V2 set, V1 nil, got %+v", doc)
		}
	})
	t.Run("unrecognized version yields combined-literal invalid_version message", func(t *testing.T) {
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
	t.Run("missing version yields invalid_version", func(t *testing.T) {
		path := writeTemp(t, `{"principals":[],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		if loadErr == nil || len(loadErr.Errors) != 1 || loadErr.Errors[0].Kind != KindInvalidVersion {
			t.Fatalf("expected a single invalid_version error, got %+v", loadErr)
		}
	})
}

func TestMalformedFixturesV2(t *testing.T) {
	// Version-2-aware assertions for the same fixtures loader_test.go's
	// v1-Load-path table records as ParseError (§10). Exercised through
	// LoadDocument, which correctly dispatches these version-2 documents
	// to validateV2 and yields a precise ErrorKind.
	cases := []struct {
		file string
		kind ErrorKind
	}{
		{"invalid-target-format.json", KindInvalidTarget},
		{"missing-target.json", KindInvalidTarget},
		{"duplicate-capability.json", KindDuplicateCapability},
		// target-exceeds-max-length.json: the target regex hardcodes the
		// same {1,128} bound as the default MaxTargetLength, exactly
		// mirroring MaxIDLength/MaxScopeLength's existing Phase 1
		// coupling (docs/phase-1-plan.md never has a static
		// id/scope-length-exceeded fixture for the identical reason —
		// see TestResourceLimitsV2 for the genuine resource_limit_exceeded
		// coverage via a lowered limit). Under default limits this
		// fixture is caught by the regex first.
		{"target-exceeds-max-length.json", KindInvalidTarget},
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

func TestDuplicateCapabilitySameScopeDifferentTargetIsNotADuplicate(t *testing.T) {
	path := writeTemp(t, `{
		"version": "2",
		"principals": [{"id": "user", "authority": [
			{"scope": "billing:read", "target": "billing-service"},
			{"scope": "billing:read", "target": "payroll-service"}
		]}],
		"agents": [],
		"delegations": [],
		"operations": []
	}`)
	doc, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if len(doc.V2.Principals[0].Authority) != 2 {
		t.Fatalf("expected 2 capabilities, got %+v", doc.V2.Principals[0].Authority)
	}
}

func TestResourceLimitsV2(t *testing.T) {
	t.Run("max_target_length", func(t *testing.T) {
		orig := limits.MaxTargetLength
		limits.MaxTargetLength = 3
		defer func() { limits.MaxTargetLength = orig }()

		path := writeTemp(t, `{"version":"2","principals":[{"id":"p1","authority":[{"scope":"a","target":"abcd"}]}],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_target_length")
	})

	t.Run("max_authority_set_size counts capability tuples", func(t *testing.T) {
		orig := limits.MaxAuthoritySetSize
		limits.MaxAuthoritySetSize = 2
		defer func() { limits.MaxAuthoritySetSize = orig }()

		path := writeTemp(t, `{"version":"2","principals":[{"id":"p1","authority":[
			{"scope":"a","target":"t1"},
			{"scope":"a","target":"t2"},
			{"scope":"a","target":"t3"}
		]}],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_authority_set_size")
	})

	t.Run("max_nodes applies to v2", func(t *testing.T) {
		orig := limits.MaxNodes
		limits.MaxNodes = 1
		defer func() { limits.MaxNodes = orig }()

		path := writeTemp(t, `{"version":"2","principals":[{"id":"p1","authority":[]}],"agents":[{"id":"a1"}],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_nodes")
	})
}

func TestNoPanicOnMutatedInputV2(t *testing.T) {
	seeds := []string{
		"../../testdata/valid-v2/combined-violations.json",
		"../../examples/billing-context-binding.json",
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
