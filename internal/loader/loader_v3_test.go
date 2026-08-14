package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/limits"
)

func TestValidV3FixturesLoadCleanly(t *testing.T) {
	files := []string{
		"clean-pass-v3.json",
		"clean-pass-v3-reordered.json",
		"combined-violations-v3.json",
		"multi-hop-requester.json",
	}
	for _, f := range files {
		f := f
		t.Run(f, func(t *testing.T) {
			doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "valid-v3", f))
			if loadErr != nil {
				t.Fatalf("unexpected load error: %s", loadErr.RenderText())
			}
			if doc.V3 == nil {
				t.Fatal("expected a V3 document")
			}
			if doc.V1 != nil || doc.V2 != nil {
				t.Fatal("expected V1 and V2 to be nil for a version-3 document")
			}
		})
	}
}

func TestExampleV3FixtureLoadsCleanly(t *testing.T) {
	doc, loadErr := LoadDocument("../../examples/billing-confused-deputy.json")
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V3 == nil {
		t.Fatal("expected a V3 document")
	}
}

func TestVersionDispatchV3(t *testing.T) {
	t.Run("version 3 routes to V3", func(t *testing.T) {
		doc, loadErr := LoadDocument("../../testdata/valid-v3/clean-pass-v3.json")
		if loadErr != nil {
			t.Fatalf("unexpected load error: %s", loadErr.RenderText())
		}
		if doc.V3 == nil || doc.V1 != nil || doc.V2 != nil {
			t.Fatalf("expected V3 set, V1/V2 nil, got %+v", doc)
		}
	})
	t.Run("unrecognized version yields three-literal invalid_version message", func(t *testing.T) {
		path := writeTemp(t, `{"version":"9","principals":[],"agents":[],"delegations":[],"operations":[]}`)
		_, loadErr := LoadDocument(path)
		if loadErr == nil || len(loadErr.Errors) != 1 {
			t.Fatalf("expected exactly 1 validation error, got %+v", loadErr)
		}
		ve := loadErr.Errors[0]
		if ve.Kind != KindInvalidVersion {
			t.Errorf("Kind = %q, want %q", ve.Kind, KindInvalidVersion)
		}
		want := `version must be "1", "2", "3", or "4", got "9"`
		if ve.Message != want {
			t.Errorf("Message = %q, want %q", ve.Message, want)
		}
	})
}

func TestMalformedFixturesV3(t *testing.T) {
	// Version-3-aware assertions for the v3 fixtures loader_test.go's
	// v1-Load-path table records as ParseError (§15). Exercised through
	// LoadDocument, which correctly dispatches these version-3 documents to
	// validateV3 and yields a precise ErrorKind.
	cases := []struct {
		file string
		kind ErrorKind
	}{
		{"unknown-requester.json", KindUnknownRequester},
		{"missing-requester.json", KindUnknownRequester},
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

func TestMissingRequesterDecodesAsEmptyStringNotASeparateKind(t *testing.T) {
	// §15: no dedicated "missing field" mechanism — a missing requester
	// decodes as "", which can never resolve to a known node id, so it
	// falls straight into unknown_requester like any other unresolved
	// reference.
	doc, loadErr := LoadDocument(filepath.Join(testdataRoot, "malformed", "missing-requester.json"))
	if loadErr == nil {
		t.Fatalf("expected a load error, got valid document %+v", doc)
	}
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindUnknownRequester && ve.Primary == "" {
			return
		}
	}
	t.Errorf("expected an unknown_requester error with Primary == \"\", got %+v", loadErr.Errors)
}

func TestUnknownActorAndUnknownRequesterBothReportedForSameOperation(t *testing.T) {
	// Structural validation is exhaustive, not fail-fast (CLAUDE.md):
	// an operation with both an unknown actor and an unknown requester
	// must report both problems in one run.
	path := writeTemp(t, `{
		"version": "3",
		"principals": [{"id": "root", "authority": [{"scope": "read", "target": "svc"}]}],
		"agents": [],
		"delegations": [],
		"operations": [
			{"actor": "nobody", "requester": "nobody-else", "action": "do-read", "requires": "read", "target": "svc"}
		]
	}`)
	_, loadErr := LoadDocument(path)
	if loadErr == nil {
		t.Fatal("expected a load error")
	}
	var sawActor, sawRequester bool
	for _, ve := range loadErr.Errors {
		if ve.Kind == KindUnknownActor {
			sawActor = true
		}
		if ve.Kind == KindUnknownRequester {
			sawRequester = true
		}
	}
	if !sawActor || !sawRequester {
		t.Errorf("expected both unknown_actor and unknown_requester, got %+v", loadErr.Errors)
	}
}

func TestRequesterEqualsActorIsStructurallyValid(t *testing.T) {
	path := writeTemp(t, `{
		"version": "3",
		"principals": [{"id": "root", "authority": [{"scope": "read", "target": "svc"}]}],
		"agents": [{"id": "agent-a"}],
		"delegations": [{"delegator": "root", "delegatee": "agent-a", "authority": [{"scope": "read", "target": "svc"}]}],
		"operations": [
			{"actor": "agent-a", "requester": "agent-a", "action": "do-read", "requires": "read", "target": "svc"}
		]
	}`)
	doc, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if doc.V3 == nil {
		t.Fatal("expected a V3 document")
	}
}

func TestDuplicateActorActionRequiresDifferByRequesterIsNotAnError(t *testing.T) {
	// §15: two v3 operations may legitimately share
	// actor/action/requires/target and differ only by requester — not a
	// validation error.
	path := writeTemp(t, `{
		"version": "3",
		"principals": [
			{"id": "root", "authority": [{"scope": "read", "target": "svc"}]},
			{"id": "other", "authority": [{"scope": "read", "target": "svc"}]}
		],
		"agents": [{"id": "agent-a"}],
		"delegations": [{"delegator": "root", "delegatee": "agent-a", "authority": [{"scope": "read", "target": "svc"}]}],
		"operations": [
			{"actor": "agent-a", "requester": "root", "action": "do-read", "requires": "read", "target": "svc"},
			{"actor": "agent-a", "requester": "other", "action": "do-read", "requires": "read", "target": "svc"}
		]
	}`)
	doc, loadErr := LoadDocument(path)
	if loadErr != nil {
		t.Fatalf("unexpected load error: %s", loadErr.RenderText())
	}
	if len(doc.V3.Operations) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(doc.V3.Operations))
	}
}

func TestResourceLimitsV3(t *testing.T) {
	t.Run("max_operations applies to v3", func(t *testing.T) {
		orig := limits.MaxOperations
		limits.MaxOperations = 1
		defer func() { limits.MaxOperations = orig }()

		path := writeTemp(t, `{
			"version": "3",
			"principals": [{"id": "root", "authority": [{"scope": "read", "target": "svc"}]}],
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

	t.Run("max_id_length applies to requester", func(t *testing.T) {
		orig := limits.MaxIDLength
		limits.MaxIDLength = 4
		defer func() { limits.MaxIDLength = orig }()

		longID := "this-id-is-too-long-for-the-lowered-limit"
		path := writeTemp(t, `{
			"version": "3",
			"principals": [{"id": "root", "authority": [{"scope": "read", "target": "svc"}]}],
			"agents": [{"id": "`+longID+`"}],
			"delegations": [],
			"operations": [
				{"actor": "`+longID+`", "requester": "root", "action": "op1", "requires": "read", "target": "svc"}
			]
		}`)
		_, loadErr := LoadDocument(path)
		assertResourceLimit(t, loadErr, "max_id_length")
	})
}

func TestNoPanicOnMutatedInputV3(t *testing.T) {
	seeds := []string{
		"../../testdata/valid-v3/combined-violations-v3.json",
		"../../examples/billing-confused-deputy.json",
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
