# CLAUDE.md

Guidance for Claude Code sessions working in this repository.

## What this project is

DelegationProof Phase 1: a small, dependency-free, offline, deterministic
CLI that checks a declared delegation graph for the Authority
Non-Amplification invariant. The authoritative design contract is
[`docs/phase-1-plan.md`](docs/phase-1-plan.md) — read it before making any
change that touches the domain model, the verification algorithm, the
finding contract, the CLI surface, or resource bounds. That document
explains not just *what* Phase 1 does but *why* each excluded feature
(audience binding, approvals, depth-limit policy, confused-deputy
detection, MCP/A2A ingestion, SAT/SMT solving, ...) was deliberately left
out and how it will attach later (§21) — do not casually reintroduce any of
them into Phase 1 code.

## Invariants to preserve

These are load-bearing product properties, not style preferences:

- **Deterministic, always.** Never rely on Go map iteration order,
  filesystem enumeration order, or goroutine scheduling for anything that
  affects output. Every tie is broken by ascending lexicographic id (see
  `internal/graph.TopoSort`, `internal/report.Sort`,
  `internal/graph.CanonicalTrace`). If you add anything that iterates a
  `map`, either sort the keys first or prove the iteration order can never
  affect output.
- **No third-party dependencies.** `go.mod` stays stdlib-only through
  Phase 1. If a task seems to need one, it's a sign the task is out of
  Phase 1 scope.
- **Structural validation is exhaustive, not fail-fast**, except for
  problems that make the document unparseable at all (JSON syntax errors,
  unknown-field decode errors), which are inherently singular. See
  `internal/loader.validate`.
- **Strict distrust.** An invalid delegation edge contributes *nothing* to
  its target's derived authority — not even the overlapping portion. Don't
  "fix" this into partial credit; it's a deliberate, tested design
  decision (`internal/verify.TestStrictDistrustNoPartialCredit`... see
  `internal/verify/verify_test.go`).
- **Bounded, never a panic.** Every resource bound in `internal/limits` is
  an exported var specifically so tests can lower it and exercise the
  bound without generating huge fixtures. Any new parsing/graph code must
  stay within `O(nodes + edges + operations)` and must not be reachable
  with a panic from adversarial input within the size bounds.
- **stdout/stderr split.** Result output (text or JSON) goes to stdout and
  only stdout; diagnostics/errors go to stderr and only stderr. Never mix
  them.

## Layout

```
cmd/delegationproof/   CLI: arg parsing, exit-code mapping (see internal/exitcode)
internal/model/        Pure data types, no logic
internal/limits/       Resource bounds as exported vars
internal/loader/       Parse + structural validation (the sole schema authority — schemas/model.md is docs only)
internal/graph/        Topological sort, cycle detection, canonical BFS trace
internal/verify/       Derived Authority algorithm, finding assembly
internal/report/       Finding types, sort order, text/json renderers
examples/, schemas/, testdata/, docs/
```

## Working in this repo

- Run `gofmt -l .`, `go vet ./...`, and `go test ./... -race -count=1`
  before considering any change done. `go build -o bin/delegationproof
  ./cmd/delegationproof` should also succeed (the `bin/` directory is
  gitignored).
- `testdata/malformed/` has one fixture per structural error kind listed in
  `docs/phase-1-plan.md` §7.4. If you add a new validation rule, add a
  fixture there and a case in `internal/loader/loader_test.go`'s table —
  and note that `cmd/delegationproof/main_test.go` walks that same
  directory automatically, so a new fixture is picked up by the CLI-level
  exit-code test for free.
- `testdata/golden/` holds captured `verify` output (text and json) for
  specific fixtures. If a deliberate output-format change is made,
  regenerate these with the built binary rather than hand-editing them,
  and diff the change to make sure it's the change you intended.
- When adding a finding field or changing a message, update
  `internal/report/finding.go`'s reason-text templates *and* the golden
  files together — they're asserted byte-for-byte.
- Phase 2+ ideas belong in `docs/phase-1-plan.md` §21 discussion, not in
  Phase 1 code, even as a "just in case" flag or unused field.
