package main

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func TestValidateExampleClean(t *testing.T) {
	stdout, stderr, code := runCLI(t, "validate", "../../examples/billing-refund.json")
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

func TestVerifyExampleViolation(t *testing.T) {
	stdout, stderr, code := runCLI(t, "verify", "../../examples/billing-refund.json")
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

func TestValidateVsVerifyDivergence(t *testing.T) {
	// mixed-violations.json is structurally valid but violates the
	// invariant: validate must pass (exit 0), verify must deny (exit 1).
	path := "../../testdata/valid/mixed-violations.json"

	_, _, validateCode := runCLI(t, "validate", path)
	if validateCode != 0 {
		t.Errorf("validate exit code = %d, want 0 (validate never evaluates the invariant)", validateCode)
	}

	_, _, verifyCode := runCLI(t, "verify", path)
	if verifyCode != 1 {
		t.Errorf("verify exit code = %d, want 1", verifyCode)
	}
}

func TestGoldenOutputs(t *testing.T) {
	cases := []struct {
		model, formatTextGolden, formatJSONGolden string
	}{
		{"../../testdata/valid/clean-pass.json", "../../testdata/golden/clean-pass.txt", "../../testdata/golden/clean-pass.json"},
		{"../../testdata/valid/mixed-violations.json", "../../testdata/golden/mixed-violations.txt", "../../testdata/golden/mixed-violations.json"},
		{"../../examples/billing-refund.json", "../../testdata/golden/billing-refund.txt", "../../testdata/golden/billing-refund.json"},
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

func TestJSONFormatDeterministicAcrossRepeatedRuns(t *testing.T) {
	out1, _, _ := runCLI(t, "verify", "../../testdata/valid/mixed-violations.json", "--format", "json")
	out2, _, _ := runCLI(t, "verify", "../../testdata/valid/mixed-violations.json", "--format", "json")
	if out1 != out2 {
		t.Errorf("repeated runs produced different output:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", out1, out2)
	}
}

func TestJSONFormatInputArrayPermutationInvariance(t *testing.T) {
	out1, _, code1 := runCLI(t, "verify", "../../testdata/valid/mixed-violations.json", "--format", "json")
	out2, _, code2 := runCLI(t, "verify", "../../testdata/valid/mixed-violations-reordered.json", "--format", "json")
	if code1 != code2 {
		t.Fatalf("exit codes differ: %d vs %d", code1, code2)
	}
	if out1 != out2 {
		t.Errorf("semantically-equivalent reordered input produced different output:\n--- original ---\n%s\n--- reordered ---\n%s", out1, out2)
	}
}

func TestFormatDefaultIsText(t *testing.T) {
	outDefault, _, _ := runCLI(t, "verify", "../../examples/billing-refund.json")
	outExplicit, _, _ := runCLI(t, "verify", "../../examples/billing-refund.json", "--format", "text")
	if outDefault != outExplicit {
		t.Errorf("default format differs from explicit --format text")
	}
}

func TestFormatEqualsSyntax(t *testing.T) {
	out1, _, code1 := runCLI(t, "verify", "../../examples/billing-refund.json", "--format=json")
	out2, _, code2 := runCLI(t, "verify", "../../examples/billing-refund.json", "--format", "json")
	if code1 != code2 || out1 != out2 {
		t.Errorf("--format=json and --format json diverged: (%d,%q) vs (%d,%q)", code1, out1, code2, out2)
	}
}

func TestMalformedModelExitCode2(t *testing.T) {
	dir := "../../testdata/malformed"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		e := e
		path := filepath.Join(dir, e.Name())
		t.Run(e.Name()+"/validate", func(t *testing.T) {
			stdout, stderr, code := runCLI(t, "validate", path)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on error", stdout)
			}
			if stderr == "" {
				t.Error("stderr should contain the error message")
			}
		})
		t.Run(e.Name()+"/verify", func(t *testing.T) {
			stdout, stderr, code := runCLI(t, "verify", path)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on error", stdout)
			}
			if stderr == "" {
				t.Error("stderr should contain the error message")
			}
		})
	}
}

func TestFileNotFoundExitCode2(t *testing.T) {
	_, stderr, code := runCLI(t, "verify", filepath.Join(t.TempDir(), "nope.json"))
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("expected a stderr message")
	}
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{},
		{"validate"},
		{"validate", "a.json", "b.json"},
		{"validate", "--format", "json", "a.json"},
		{"verify"},
		{"verify", "a.json", "b.json"},
		{"verify", "a.json", "--unknown-flag"},
		{"verify", "a.json", "--format"},
		{"verify", "a.json", "--format", "xml"},
		{"bogus-subcommand"},
		{"bogus-subcommand", "a.json"},
	}
	for _, args := range cases {
		args := args
		t.Run(joinArgs(args), func(t *testing.T) {
			stdout, stderr, code := runCLI(t, args...)
			if code != 3 {
				t.Errorf("args=%v exit code = %d, want 3", args, code)
			}
			if stdout != "" {
				t.Errorf("args=%v stdout = %q, want empty", args, stdout)
			}
			if stderr == "" {
				t.Errorf("args=%v expected a stderr message", args)
			}
		})
	}
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return "(none)"
	}
	s := args[0]
	for _, a := range args[1:] {
		s += "_" + a
	}
	return s
}

func TestNoPanicOnMutatedInput(t *testing.T) {
	orig, err := os.ReadFile("../../testdata/valid/mixed-violations.json")
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

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		mutated := append([]byte(nil), orig...)
		for j := 0; j < 5; j++ {
			mutated[rng.Intn(len(mutated))] = byte(rng.Intn(256))
		}
		tryRun("mutated", mutated)
	}
}
