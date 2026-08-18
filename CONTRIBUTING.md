# Contributing to DelegationProof

Most of the substantive engineering guidance for this repository already
lives in [`CLAUDE.md`](CLAUDE.md) (the deterministic-contract rules,
strict-distrust semantics, layout, and the exact commands every change
must pass) — read it before making any change that touches
`internal/loader`, `internal/graph`, `internal/verify`, or
`internal/report`. This file covers the GitHub-facing process specifics
that don't belong in that file.

## Before opening a PR

1. Install the Go version pinned in [`go.mod`](go.mod) (currently
   `go 1.26.5`).
2. Run `./scripts/verify.sh` from the repository root. It runs, in
   order: `gofmt -l .`, `go vet ./...`, `go test ./... -race -count=1`,
   a build, deterministic/permutation-invariance checks against the
   shipped binary, and a malformed-input fail-closed check — the same
   commands CI runs on every push (`.github/workflows/ci.yml`). A PR
   that fails any of these locally will fail CI identically.
3. If your change deliberately alters output format or finding text,
   regenerate the affected files under `testdata/golden/` with the
   built binary and review the diff — never hand-edit a golden file.
   `internal/report/finding.go`'s reason-text templates and the golden
   files must change together.

## Phase-plan immutability

`docs/phase-1-plan.md` through `docs/phase-6-plan.md` are historical
design contracts for already-shipped, already-tested behavior. They are
not living documentation: a PR should never edit their substance (typo
fixes excepted). A new capability proposal belongs in a new document,
never as an edit to a past phase's contract — this repository's own
history (each phase planned before it was implemented, see the git log)
is the model to follow.

## Non-negotiable invariants

This project's deterministic-contract and strict-distrust invariants are
load-bearing product properties, not style preferences — see
[`CLAUDE.md`](CLAUDE.md) and the relevant `docs/phase-*-plan.md` before
changing anything in `internal/verify` or `internal/loader`. In
particular: never rely on Go map iteration order for anything that
affects output; an invalid incoming delegation edge or approval record
must contribute nothing to its target, never partial credit; every
resource bound lives in `internal/limits` as an exported var, and new
parsing/graph code must stay bounded and panic-free on adversarial input
within those bounds.

## Reporting a bug

Ordinary bugs and feature requests: open a GitHub issue. Suspected
soundness bugs (an input that should DENY but gets `ALLOW`, or vice
versa) or crash/resource-exhaustion bugs: see
[`SECURITY.md`](SECURITY.md) instead — please don't file those as public
issues before a fix exists.
