package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateV6ExampleClean(t *testing.T) {
	// examples/billing-approval-lifecycle.json is structurally valid —
	// validate never evaluates any invariant, including Temporal Approval
	// Preservation, so it must pass even though verify will deny it.
	stdout, stderr, code := runCLI(t, "validate", "../../examples/billing-approval-lifecycle.json")
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if stdout != "VALID\n" {
		t.Errorf("stdout = %q, want %q", stdout, "VALID\n")
	}
}

func TestVerifyV6ExampleViolation(t *testing.T) {
	stdout, stderr, code := runCLI(t, "verify", "../../examples/billing-approval-lifecycle.json")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if stdout == "" {
		t.Error("stdout should contain the finding report")
	}
}

func TestValidateVsVerifyDivergenceV6(t *testing.T) {
	path := "../../examples/billing-approval-lifecycle.json"

	_, _, validateCode := runCLI(t, "validate", path)
	if validateCode != 0 {
		t.Errorf("validate exit code = %d, want 0 (validate never evaluates any invariant)", validateCode)
	}

	_, _, verifyCode := runCLI(t, "verify", path)
	if verifyCode != 1 {
		t.Errorf("verify exit code = %d, want 1", verifyCode)
	}
}

func TestGoldenOutputsV6(t *testing.T) {
	cases := []struct {
		model, formatTextGolden, formatJSONGolden string
	}{
		{"../../testdata/valid-v6/clean-pass-v6.json", "../../testdata/golden/clean-pass-v6.txt", "../../testdata/golden/clean-pass-v6.json"},
		{"../../testdata/valid-v6/combined-violations-v6.json", "../../testdata/golden/combined-violations-v6.txt", "../../testdata/golden/combined-violations-v6.json"},
		{"../../examples/billing-approval-lifecycle.json", "../../testdata/golden/billing-approval-lifecycle.txt", "../../testdata/golden/billing-approval-lifecycle.json"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.model, func(t *testing.T) {
			stdout, _, _ := runCLI(t, "verify", c.model)
			wantText, err := os.ReadFile(c.formatTextGolden)
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}
			if stdout != string(wantText) {
				t.Errorf("text output mismatch.\ngot:\n%s\nwant:\n%s", stdout, wantText)
			}

			stdoutJSON, _, _ := runCLI(t, "verify", c.model, "--format", "json")
			wantJSON, err := os.ReadFile(c.formatJSONGolden)
			if err != nil {
				t.Fatalf("reading golden file: %v", err)
			}
			if stdoutJSON != string(wantJSON) {
				t.Errorf("json output mismatch.\ngot:\n%s\nwant:\n%s", stdoutJSON, wantJSON)
			}
		})
	}
}

func TestJSONFormatDeterministicAcrossRepeatedRunsV6(t *testing.T) {
	out1, _, _ := runCLI(t, "verify", "../../testdata/valid-v6/combined-violations-v6.json", "--format", "json")
	out2, _, _ := runCLI(t, "verify", "../../testdata/valid-v6/combined-violations-v6.json", "--format", "json")
	if out1 != out2 {
		t.Errorf("repeated runs produced different output:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", out1, out2)
	}
}

func TestJSONFormatInputArrayPermutationInvarianceV6(t *testing.T) {
	out1, _, code1 := runCLI(t, "verify", "../../testdata/valid-v6/clean-pass-v6.json", "--format", "json")
	out2, _, code2 := runCLI(t, "verify", "../../testdata/valid-v6/clean-pass-v6-reordered.json", "--format", "json")
	if code1 != code2 {
		t.Fatalf("exit codes differ: %d vs %d", code1, code2)
	}
	if out1 != out2 {
		t.Errorf("semantically-equivalent reordered v6 input produced different output:\n--- original ---\n%s\n--- reordered ---\n%s", out1, out2)
	}
}

func TestFormatDefaultIsTextV6(t *testing.T) {
	outDefault, _, _ := runCLI(t, "verify", "../../examples/billing-approval-lifecycle.json")
	outExplicit, _, _ := runCLI(t, "verify", "../../examples/billing-approval-lifecycle.json", "--format", "text")
	if outDefault != outExplicit {
		t.Errorf("default format differs from explicit --format text")
	}
}

func TestMalformedModelV6ExitCode2(t *testing.T) {
	cases := []string{
		"../../testdata/malformed/unknown-lifecycle-state.json",
		"../../testdata/malformed/duplicate-lifecycle-state.json",
		"../../testdata/malformed/duplicate-lifecycle-transition.json",
		"../../testdata/malformed/empty-lifecycle-states.json",
		"../../testdata/malformed/lifecycle-field-on-delegation.json",
		"../../testdata/malformed/lifecycle-field-on-operation.json",
		"../../testdata/malformed/lifecycle-field-on-root-capability.json",
	}
	for _, path := range cases {
		path := path
		t.Run(path, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, "validate", path)
			if code != 2 {
				t.Errorf("validate exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("validate stdout = %q, want empty on error", stdout)
			}
			if stderr == "" {
				t.Error("validate stderr should contain the error message")
			}

			stdout, stderr, code = runCLI(t, "verify", path)
			if code != 2 {
				t.Errorf("verify exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("verify stdout = %q, want empty on error", stdout)
			}
			if stderr == "" {
				t.Error("verify stderr should contain the error message")
			}
		})
	}
}

func TestStdoutStderrSplitV6(t *testing.T) {
	t.Run("finding case: result on stdout only", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, "verify", "../../examples/billing-approval-lifecycle.json")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
		if stdout == "" {
			t.Error("stdout should contain the finding report")
		}
	})
	t.Run("error case: diagnostics on stderr only", func(t *testing.T) {
		stdout, stderr, code := runCLI(t, "verify", "../../testdata/malformed/unknown-lifecycle-state.json")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
		if stderr == "" {
			t.Error("stderr should contain the error message")
		}
	})
}

func TestNoPanicOnMutatedInputV6(t *testing.T) {
	orig, err := os.ReadFile("../../testdata/valid-v6/combined-violations-v6.json")
	if err != nil {
		t.Fatalf("reading seed fixture: %v", err)
	}

	tryRun := func(name string, data []byte) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("run panicked: %v", r)
				}
			}()
			path := filepath.Join(t.TempDir(), "fixture.json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("writing fixture: %v", err)
			}
			_, _, code := runCLI(t, "verify", path)
			if code != 0 && code != 1 && code != 2 {
				t.Errorf("unexpected exit code %d for mutated input", code)
			}
		})
	}

	tryRun("empty", nil)
	for n := 1; n < len(orig); n += 11 {
		tryRun("truncated", orig[:n])
	}

	rng := rand.New(rand.NewSource(6))
	for i := 0; i < 50; i++ {
		mutated := append([]byte(nil), orig...)
		for j := 0; j < 5; j++ {
			mutated[rng.Intn(len(mutated))] = byte(rng.Intn(256))
		}
		tryRun("mutated", mutated)
	}
}
