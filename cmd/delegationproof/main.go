// Command delegationproof is a small, dependency-free, offline,
// deterministic CLI that parses and validates a delegation model and
// evaluates the Authority Non-Amplification invariant over it (see
// docs/phase-1-plan.md).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SamudralaAjaykumarrr/delegationproof/internal/exitcode"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/loader"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/report"
	"github.com/SamudralaAjaykumarrr/delegationproof/internal/verify"
)

const usage = `usage:
  delegationproof validate <model.json>
  delegationproof verify   <model.json> [--format text|json]`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return int(exitcode.UsageError)
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n%s\n", args[0], usage)
		return int(exitcode.UsageError)
	}
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	path, uerr := parseValidateArgs(args)
	if uerr != "" {
		fmt.Fprintf(stderr, "%s\nusage: delegationproof validate <model.json>\n", uerr)
		return int(exitcode.UsageError)
	}

	_, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		fmt.Fprint(stderr, loadErr.RenderText())
		return int(exitcode.ModelError)
	}

	fmt.Fprintln(stdout, "VALID")
	return int(exitcode.OK)
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	path, format, uerr := parseVerifyArgs(args)
	if uerr != "" {
		fmt.Fprintf(stderr, "%s\nusage: delegationproof verify <model.json> [--format text|json]\n", uerr)
		return int(exitcode.UsageError)
	}

	doc, loadErr := loader.LoadDocument(path)
	if loadErr != nil {
		fmt.Fprint(stderr, loadErr.RenderText())
		return int(exitcode.ModelError)
	}

	var result report.Result
	switch {
	case doc.V1 != nil:
		result = verify.Run(doc.V1)
	case doc.V2 != nil:
		result = verify.RunV2(doc.V2)
	case doc.V3 != nil:
		result = verify.RunV3(doc.V3)
	case doc.V4 != nil:
		result = verify.RunV4(doc.V4)
	case doc.V5 != nil:
		result = verify.RunV5(doc.V5)
	case doc.V6 != nil:
		result = verify.RunV6(doc.V6)
	}

	switch format {
	case "json":
		out, err := report.RenderJSON(result)
		if err != nil {
			fmt.Fprintf(stderr, "internal error rendering JSON output: %v\n", err)
			return int(exitcode.ModelError)
		}
		stdout.Write(out)
	default:
		fmt.Fprint(stdout, report.RenderText(result))
	}

	if len(result.Findings) > 0 {
		return int(exitcode.Deny)
	}
	return int(exitcode.OK)
}

func parseValidateArgs(args []string) (path string, usageErr string) {
	var positionals []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return "", fmt.Sprintf("unknown flag: %s", a)
		}
		positionals = append(positionals, a)
	}
	if len(positionals) != 1 {
		return "", fmt.Sprintf("expected exactly 1 argument (model path), got %d", len(positionals))
	}
	return positionals[0], ""
}

func parseVerifyArgs(args []string) (path, format, usageErr string) {
	format = "text"
	var positionals []string

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--format":
			if i+1 >= len(args) {
				return "", "", "missing value for --format"
			}
			format = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
			i++
		case strings.HasPrefix(a, "-"):
			return "", "", fmt.Sprintf("unknown flag: %s", a)
		default:
			positionals = append(positionals, a)
			i++
		}
	}

	if len(positionals) != 1 {
		return "", "", fmt.Sprintf("expected exactly 1 argument (model path), got %d", len(positionals))
	}
	if format != "text" && format != "json" {
		return "", "", fmt.Sprintf("invalid --format value %q (expected text or json)", format)
	}
	return positionals[0], format, ""
}
