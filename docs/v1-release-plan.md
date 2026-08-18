# DelegationProof v1.0 — Evidence & Release Capstone Plan

**Status:** planning only. Nothing in this document has been implemented. No
Go code, tests, README, schemas, examples, or testdata are modified by this
plan or its authoring. The only file this plan creates is itself.

**Scope:** this is not a seventh security invariant. Phases 1–6
(`docs/phase-1-plan.md` … `docs/phase-6-plan.md`) already deliver a
coherent, internally consistent, fully-tested security analyzer. This
document plans how to turn that completed engineering work into a
release an external, skeptical reviewer can evaluate without trusting the
author — reproducible verification, CI, evidence, architecture/threat
documentation, release engineering, and legal/distribution hygiene. It
deliberately adds no new domain concept, no new schema version, no new
finding kind, and no new CLI verification behavior beyond one optional,
inert `--version` flag (§17).

---

## 0. Judgment: is v1.0 justified?

**YES.** Phases 1–6 are a coherent, closed release boundary:

- Six invariants (Non-Amplification → Temporal Approval Preservation),
  each additive, each with its own schema version, each documented in a
  full design contract (`docs/phase-*-plan.md`), each with dedicated
  loader/verify/report code and an independent test suite.
- `gofmt -l .`, `go vet ./...`, and `go test ./... -race -count=1` are
  clean today (verified while writing this plan). `go build -o
  bin/delegationproof ./cmd/delegationproof` succeeds.
- The determinism, strict-distrust, bounded-resource, and fail-closed
  properties the project claims are already backed by real tests
  (`TestStrictDistrustNoPartialCredit*`, `TestResourceLimits*`,
  `TestJSONFormatInputArrayPermutationInvariance*`,
  `TestFailClosedTruncationYieldsUnproven`, etc.) — v1.0 does not need to
  invent these properties, only make them *externally legible*.
- Nothing in the six phase-plan documents defers a load-bearing
  correctness property to "later." Everything Phase 7+ would add
  (audience binding, approvals quorum, confused-deputy detection nuance,
  MCP/A2A ingestion, SAT/SMT solving — see each plan's non-goals section)
  is optional *additional* capability, not a missing piece of what's
  already claimed.

**Every genuine blocker below is productization, not functionality:**

1. **No `LICENSE` file.** A public GitHub repository with no license is,
   by default, "all rights reserved" — nobody may legally use, fork, or
   redistribute it. This is a hard blocker for a credible public v1.0.
   (§23 — this plan does not choose one; that is the repository owner's
   call.)
2. **No CI.** `.github/workflows/` does not exist. Nothing currently
   proves to an outside viewer, on every push, that `gofmt`/`vet`/tests/
   build pass. (§4)
3. **No one-command local verification entry point.** `scripts/` is
   present but empty — `docs/phase-1-plan.md` §15 explicitly left it
   empty "as a placeholder for later phases." That later point is now.
   (§3)
4. **No standalone architecture/threat-model/evidence documents.** The
   README already contains excellent architecture, determinism, and
   security-assumptions prose, but it is not organized as
   independently-linkable, reviewer-oriented artifacts, and there is no
   evidence document tying claims to actual command output. (§5, §10,
   §11)
5. **No release engineering.** No tags, no release workflow, no
   published binaries, no checksums. (§14, §16)
6. **No `SECURITY.md`.** No disclosed channel for reporting a soundness
   bug (a case where DelegationProof reports `ALLOW` for a genuinely
   invalid model) or a crash/resource-exhaustion bug. (§21)

None of these require touching `internal/`, `cmd/`, `schemas/`, `examples/`,
or `testdata/` in a way that changes verification semantics. The one
optional code change this plan recommends (`--version`, §17) is additive,
inert with respect to `validate`/`verify` output, and can be deferred
without blocking v1.0 if the owner prefers zero Go-code churn during the
release push.

---

## 1. v1.0 release definition

**DelegationProof v1.0.0 is:** the state of `main` where Phases 1–6's
existing invariant/loader/verify/report code is unchanged, plus the
productization artifacts this plan defines (verification script, CI,
architecture/threat-model/evidence docs, README restructuring, license,
release engineering), tagged `v1.0.0` and published as a GitHub Release
with checksummed binaries for the platforms in §14.

**Why Phases 1–6 are a coherent boundary, not an arbitrary cutoff:**

- Each phase is a strict superset schema version of the last
  (`schemas/model.md`), and each new invariant is evaluated only after
  every earlier one already passes (documented precedence chains in each
  phase plan, culminating in Phase 6's five-step operation-level
  precedence). There is no half-finished invariant — each one has a
  complete formal statement, algorithm, structural-error set, resource
  bound, and test suite.
- The six invariants together answer a complete, self-contained question:
  *given a declared delegation topology, can any node ever legitimately
  exercise authority it wasn't validly, currently, and safely granted?*
  Amplification (is it real), binding (is it for the right target),
  confused-deputy (is it for the right requester), depth (has it traveled
  too far), approval (was sign-off required and present), and temporal
  approval (can that sign-off be relied upon over time) are six
  independent axes of the same question — not six arbitrary stopping
  points along an open-ended list.
- Every phase plan's own non-goals section defers a *different kind* of
  extension (real-time enforcement, quorum approvals, model ingestion
  from live MCP/A2A systems, SAT/SMT solving, cross-approval composition)
  — none of them is "finish what Phase N started," they are all "attach a
  new axis to a foundation that is already complete." That is the
  definition of a stable release boundary.

**v1.0 explicitly does not require:** a seventh invariant, any new schema
version, any new finding kind, any change to `internal/limits` default
values, or any change to the Derived Authority algorithm.

**Objective release gates** (elaborated fully as the numbered checklist in
§28):

- `scripts/verify.sh` exits 0 on a clean checkout at the release commit.
- CI is green on the commit being tagged.
- `docs/architecture.md`, `docs/threat-model.md`, `docs/evidence-report.md`
  exist and are internally consistent with the current code.
- `LICENSE` exists and is referenced from `README.md` and `go.mod`'s
  module path is unaffected.
- The `v1.0.0` tag's release binaries' checksums are published and verify
  against `checksums.txt`.
- No uncommitted or untracked generated files at the tagged commit.

---

## 2. Security capabilities inventory

Precise, non-marketing statement of what v1.0 proves and does not prove,
per invariant. (Source of truth: each `docs/phase-*-plan.md` formal
statement plus the README's existing "Security assumptions" section —
this inventory is a reorganization for reviewer scanning, not a new
claim.)

| # | Invariant | What it proves | What it does NOT prove |
|---|---|---|---|
| 1 | Authority Non-Amplification | No node's derived authority (computed by one deterministic topological pass over declared delegation edges) ever includes a capability that was never validly delegated to it from a root principal's declared authority. An invalid incoming edge contributes nothing — not partial credit. | That the declared model matches a real running system. That a principal's own declared root authority was legitimately obtained (identity/OAuth is out of scope). That authority can't be amplified through a channel this model doesn't represent (e.g. out-of-band credential sharing). |
| 2 | Context/Target Binding Preservation | A capability validly held for one target cannot be exercised or transmitted for a different target — `(scope, target)` is the whole identity of a capability, with no implied hierarchy. | That "target" strings correspond to any real access-control boundary in the reviewer's actual infrastructure — a target is an opaque label the document author chooses. |
| 3 | Requester Authorization Preservation (confused-deputy) | An operation's `requester` (the party it's performed *for*) is independently checked against the same Derived Authority computation as `actor` — a validly-authorized actor cannot be used to exercise a capability on behalf of a requester who never independently held it. | That `requester` is an authenticated identity. It is a declared label in the input document; DelegationProof does not verify that the real party performing an operation is who the document claims. |
| 4 | Delegation Depth Preservation | A capability declared with a root-level `max_delegation_depth` budget cannot be re-delegated (transmitted via an outgoing edge) more hops than that budget permits, regardless of how many valid paths deliver it (best/maximum remaining budget always wins, deterministically tie-broken). | That `max_delegation_depth` corresponds to any real re-delegation counter enforced by a running system. It is a policy assertion by the document's author about intended limits, not observed history. Exercising a capability is never gated by remaining budget — only further delegation is. |
| 5 | Approval Preservation | If a root capability declares `requires_approval: true`, an operation exercising it (once presence/binding/requester checks already pass) is illegitimate unless at least one declared approval record names an approver who independently holds that exact capability. A non-standing approver contributes nothing. | That a named approver is a real, authenticated person, or that a real compliance workflow actually produced that sign-off. An approval record is a declared fact by the document's author, not an observed event. |
| 6 | Temporal Approval Preservation | If a standing-backed approval record declares its own lifecycle automaton, the approval counts only if *every* state reachable from its declared initial state (via bounded, deterministic BFS) is `"approved"` — an approval that can reach any other state, or whose exploration cannot complete within the bounded ceiling, is fail-closed: never treated as `ALLOW`. | That the declared lifecycle transitions correspond to any real revocation/expiry event a real system will actually fire, or *when* (in real time) an operation executes relative to a transition. The safety predicate is a universal, static claim about the declared automaton — not a claim about real-time state at the moment of exercise. |

**What v1.0 does not claim at all**, regardless of invariant (see
`docs/threat-model.md`, §11 below, for the full statement): runtime
enforcement, interception, or observation of real agent/tool traffic;
verification that a declared model matches reality; identity
authentication of any kind; protection against a dishonest model author
who simply declares a false topology.

---

## 3. One-command reproducible verification: `scripts/verify.sh`

**Design (not yet implemented):** a single POSIX `sh` script, no
dependencies beyond the Go toolchain already required to build the
project, runnable identically by a human and by CI.

```
scripts/verify.sh
```

**Steps, in order, fail-fast** (each step prints a `==> <step>` header and
either `PASS` or the tool's own failure output; the first failing step
aborts with a non-zero exit and a one-line summary of which gate failed —
fail-fast is correct *here* even though the domain-model validator itself
is exhaustive, §7.4 of `docs/phase-1-plan.md`, because this script is a
release gate over independent, unrelated tool invocations, not a single
structural-validation pass over one input document):

1. **Formatting** — `gofmt -l .` — fail if output is non-empty (print the
   offending files).
2. **Vet** — `go vet ./...`.
3. **Race + unit tests** — `go test ./... -race -count=1`.
4. **Build** — `go build -o bin/delegationproof ./cmd/delegationproof`
   (into the already-gitignored `bin/`).
5. **Deterministic example verification** — for each file in
   `examples/*.json`, run the just-built binary's `verify --format json`
   twice and assert byte-identical output (`diff` or `cmp` between two
   runs), then assert the exit code matches the expected value already
   documented in the README for that example (all six are `1`, DENY).
   This is a black-box, binary-level restatement of what
   `TestJSONFormatDeterministicAcrossRepeatedRuns*` already proves
   in-process — valuable specifically because it exercises the *shipped
   artifact*, not the test binary.
6. **Malformed-input fail-closed check** — for each fixture under
   `testdata/malformed/`, run `verify` and assert exit code `2` (never
   `0`, never `1`) — a shell-level restatement of what
   `cmd/delegationproof/main_test.go`'s directory walk already proves,
   again at the shipped-binary level.
7. **Repository hygiene** — `git status --porcelain` must be empty
   (nothing uncommitted/untracked left behind by the steps above — in
   particular, confirm `bin/` stayed gitignored and nothing under
   `testdata/golden/` was silently modified by the run).

**Pass/fail behavior:** exit `0` only if all seven steps pass; any
failure exits non-zero with the failing step named. This makes
`./scripts/verify.sh` alone sufficient to answer "does this checkout meet
every objective release gate this project defines," per the core
productization goal.

**Explicitly not in `verify.sh`:** anything requiring network access,
anything requiring a specific OS (the script itself should be portable
`sh`, but cross-compilation checks belong in CI's matrix, §4, not local
verification, since not every contributor has every cross-compiler
target installed), and any resource-bound stress test large enough to be
slow (those remain Go white-box tests with lowered `internal/limits`
values, §9 — already fast, already exist).

---

## 4. CI design

**File:** `.github/workflows/ci.yml` (not created by this plan).

**Trigger:** `push` to `main` and `pull_request` targeting `main`. No
schedule, no manual-dispatch complexity needed for v1.0.

**Go version strategy:** `go.mod` currently pins `go 1.26.5`. CI runs
that exact version as the single required job — this project is
stdlib-only and deterministic by design; there is no ecosystem
compatibility matrix to protect (no dependents, no dependency graph). Add
one *informational, non-blocking* job on the previous stable minor
version only if the owner wants early warning of accidental
version-specific stdlib usage; it must not gate merges. Recommendation:
skip the informational job for v1.0 — it adds a second Go toolchain
download to every CI run for a hypothetical benefit this stdlib-only
project doesn't need yet.

**Jobs:**

1. **`verify`** (primary gate, `ubuntu-latest`):
   - `actions/checkout` pinned to a commit SHA (not a floating tag).
   - `actions/setup-go` pinned to a commit SHA, `go-version-file: go.mod`
     (so CI always matches the module's declared version — no drift
     between local and CI).
   - `./scripts/verify.sh`.
2. **`cross-build`** (matrix, build-only, no test execution — catches
   platform-specific compile breakage ahead of a release without needing
   real Windows/macOS runners): matrix over the five `GOOS/GOARCH` pairs
   in §14, `GOOS=$os GOARCH=$arch go build -o /dev/null
   ./cmd/delegationproof` on the same `ubuntu-latest` runner (Go
   cross-compiles without emulation). Depends on `verify` passing first
   (`needs: verify`) so a broken build doesn't waste matrix minutes on
   code that already failed formatting/tests.

**Permissions:** top-level `permissions: contents: read` — CI never
writes to the repository, never needs `pull-requests` or `issues` scope.

**Third-party actions:** only `actions/checkout` and `actions/setup-go`
(both first-party GitHub actions), both pinned by commit SHA per
supply-chain hygiene norms, with the SHA's corresponding version tag in a
trailing comment for human auditability. No other action is needed —
this is precisely the "avoid unnecessary third-party actions" instruction
in practice: two first-party actions is the whole dependency surface.

**What CI proves that `verify.sh` alone doesn't:** that the repository
builds and tests pass in a clean environment with no local state (no
stray `GOFLAGS`, no cached module tricks, no locally-modified
`internal/limits` left from a prior debugging session), on every push —
i.e., CI is `verify.sh` run untrusted, continuously. The two are
deliberately the same commands so a contributor never sees a CI failure
that `verify.sh` didn't already predict locally.

---

## 5. Reproducible security evidence: `docs/evidence-report.md`

**Purpose:** a document a skeptical reviewer reads *instead of* trusting
the README's prose claims — every claim paired with the exact command
that reproduces it and the exact expected output.

**Structure:**

1. **Provenance header** — exact commit SHA and/or tag, `go version`
   output, OS the evidence was captured on. (Re-filled at each tagged
   release — see "generation strategy" below.)
2. **Build evidence** — the four `scripts/verify.sh` steps 1–4 (fmt/vet/
   test/build), with the literal `go test ./... -race -count=1 -v` tail
   showing all packages `ok`.
3. **Known ALLOW case** — a `verify` invocation against a
   `testdata/valid*` fixture (or a hand-described minimal model) that
   exits `0` with zero findings — proves the tool can produce a clean
   pass, not just findings.
4. **Known DENY cases, one per invariant** — reuse the six documented
   `examples/*.json` verbatim: exact command, exact `text` output block,
   exact exit code. (Already written once in the README — the evidence
   report's job is to be the reviewer-facing, evidence-framed copy with
   an explicit "reproduce this yourself" framing, not to re-derive new
   content.)
5. **Determinism evidence** — run each example's `verify --format json`
   twice, `sha256sum` both outputs, show the hashes match; run it again
   against the semantically-reordered permutation fixtures already used
   by `TestJSONFormatInputArrayPermutationInvariance*`, show the hash is
   identical to the unreordered version. This is `scripts/verify.sh`
   step 5 (§3), captured as evidence rather than just a pass/fail gate.
6. **Fail-closed evidence** — one `testdata/malformed/` fixture run
   through `verify`, showing exit `2` with its structural-error message;
   explicit prose distinguishing this from the DENY (exit `1`) cases
   above — semantic violation vs. invalid input are different exit codes
   for a reason (§18).
7. **Resource-bound evidence** — this is **not** a shell demonstration
   (triggering a real 10,000-node or 5 MiB bound in a demo fixture is
   either impractical to read or impractical to ship as an example). It
   is a citation: name the exact white-box tests
   (`TestResourceLimits`, `TestResourceLimitsV2` … `V6`) and the fact
   that they exercise every bound in `internal/limits` via deliberately
   lowered values, in-process, with no observed panic/hang — plus a
   one-line pointer at "run `go test ./internal/loader -race -v -run
   ResourceLimits`" for a reviewer who wants to watch it happen.
8. **Temporal approval evidence** — the version-6 example's DENY output
   (already shows `approval_lifecycle_unsafe`) plus a citation of
   `TestFailClosedTruncationYieldsUnproven` (proves a bounded-search
   truncation resolves to `approval_lifecycle_unproven`, never `ALLOW`)
   and `TestSafeRecordShortCircuitsBeforeUnproven`.

**Generated vs. manual:** the *commands* and *structure* of this document
are fixed and version-controlled prose. The *captured output blocks*
(hashes, exact stdout, exact exit codes) must be regenerated by actually
running the commands at each tagged release — never hand-typed or
extrapolated. This plan recommends the discipline (documented in
`CONTRIBUTING.md`/release checklist, §22, §28) rather than building an
auto-generation script for v1.0: the commands are simple enough
(`scripts/verify.sh` plus a handful of `sha256sum`/`diff` invocations)
that a template with clearly-marked "paste output here" blocks is
sufficient, and a generator script is exactly the kind of "complexity
without value" §20 warns against for a project this size. If a future
maintainer wants to automate it, `evidence/manifest.json` (§20) is the
natural place to make the example list machine-readable first.

---

## 6. Security demonstration suite

**No new examples are needed.** The six existing files under `examples/`
already cover every category the instructions ask for, because each one
is a version's canonical worked example showing *both* a passing
operation and the failing one that names the version's new invariant —
this is a compact set (6 files), not a sprawl:

| File | Version | Clean/ALLOW action shown | DENY finding demonstrated |
|---|---|---|---|
| `billing-refund.json` | 1 | `billing.view` (passes silently) | `authority_amplification` |
| `billing-context-binding.json` | 2 | (implicit — same-target use) | `context_binding_violation` |
| `billing-confused-deputy.json` | 3 | (implicit — requester-holds-it case) | `confused_deputy` |
| `billing-redelegation-depth.json` | 4 | `refund-ok` at exact budget boundary (passes) | `delegation_depth_violation` + consequent `authority_amplification` |
| `billing-approval.json` | 5 | `refund-approved` (passes) | `approval_missing` |
| `billing-approval-lifecycle.json` | 6 | `refund-safe` (passes) | `approval_lifecycle_unsafe` |

The version-6 example is already, structurally, the **combined-violation**
case the instructions ask about "if useful": a v6 document is checked
against all six invariants at once, so its DENY output is evidence that
Phases 1–5's checks all silently passed while only the new Phase 6 check
fired — i.e. it already demonstrates the full precedence chain, not just
temporal approval in isolation. No seventh "kitchen sink" example is
needed or recommended (would violate the "do NOT create dozens" and "do
not add features/examples beyond what's needed" instructions).

**Exact commands** (identical for all six, differing only in path —
already documented in the README, restated here for the evidence
report's "reproduce it yourself" framing):

```sh
delegationproof verify examples/billing-refund.json
delegationproof verify examples/billing-context-binding.json
delegationproof verify examples/billing-confused-deputy.json
delegationproof verify examples/billing-redelegation-depth.json
delegationproof verify examples/billing-approval.json
delegationproof verify examples/billing-approval-lifecycle.json
```

Each exits `1` (DENY) with the exact finding count/content already shown
in the README. `scripts/verify.sh` step 5 and `docs/evidence-report.md`
§5 both consume this same table as their source list.

---

## 7. Determinism proof/evidence

Already proven in-process by
`TestJSONFormatDeterministicAcrossRepeatedRuns*` and
`TestJSONFormatInputArrayPermutationInvariance*` (one pair per schema
version, `cmd/delegationproof/main*_test.go`). v1.0 adds an
**external, black-box restatement** using only `sha256sum`/`diff`
against the *shipped binary* (not the test binary), specified in
`scripts/verify.sh` step 5 and captured in `docs/evidence-report.md` §5:

- **Repeated identical input** — run `verify --format json` on the same
  file twice, hash both outputs, assert equal.
- **Reordered semantically-equivalent arrays** — run against the
  permutation fixtures the existing tests already construct in Go
  (`testdata/` already has the raw fixtures the tests build from;
  no *new* fixture files are needed — reuse what the Go tests already
  read). Note: constructing the reordered variant is currently done
  inline in Go test code rather than as a checked-in JSON file; if the
  implementation phase (§29, Batch A) finds it cleaner to have `verify.sh`
  invoke a small `go run` snippet or a `go test -run
  Permutation -v` pass-through instead of hand-maintaining a duplicate
  shell-level fixture, that is an acceptable implementation detail, not a
  planning decision — the property to be demonstrated is fixed
  regardless of which reordering source is used.
- **Deterministic findings/traces/JSON output** — already the subject of
  the cited tests; the evidence report cites them by name rather than
  re-deriving the proof.

This keeps the "hash comparison" ask concrete and cheap (no new Go code,
no new fixtures required beyond what already exists) while adding a
form of proof (byte-for-byte shipped-artifact hashing) the existing test
suite doesn't itself produce as a human-readable artifact.

---

## 8. Fail-closed evidence

Three genuinely distinct fail-closed surfaces, kept separate because
they are different mechanisms proving different things — conflating them
would blur exactly the distinction the exit-code contract exists to
preserve:

1. **Malformed input never becomes ALLOW.** Every fixture in
   `testdata/malformed/` (one per structural-error kind across all six
   schema versions) exits `2`, never `0` or `1`. Already proven by
   `cmd/delegationproof/main_test.go`'s directory walk (per CLAUDE.md);
   restated at the shipped-binary level in `scripts/verify.sh` step 6.
2. **Resource-limit exhaustion never becomes ALLOW.** Proven by the
   `TestResourceLimits*` family (white-box, lowered `internal/limits`
   values) — cited, not re-demonstrated at shell level (§5, item 7,
   explains why a shell demo is impractical here).
3. **Incomplete lifecycle exploration never becomes ALLOW (or an
   implicit pass).** Proven by `TestFailClosedTruncationYieldsUnproven`
   (lowered `limits.MaxExplorationStatesPerLifecycle`, asserts the
   result is `approval_lifecycle_unproven`, never a clean pass) — cited
   in the evidence report, §5 item 8.

**Semantic DENY vs. invalid-input failure, stated explicitly** (this is
the exact distinction §18/§28 both need to be unambiguous about):
exit `1` means the tool parsed a structurally valid model and correctly
found a real invariant violation — the tool *worked*. Exit `2` means the
tool could not even establish that the input was a legitimate model to
reason about — a `resource_limit_exceeded`, a malformed reference, a
truncated-lifecycle-exploration `approval_lifecycle_unproven` finding is
still an exit-`1` DENY (it *is* a semantic finding, just one whose
justification is "could not be proven safe" rather than "proven
unsafe") — this one is worth calling out explicitly in the evidence
report because it's the one place "fail-closed" produces a `1`, not a
`2`, and a careless reader could otherwise conflate the two exit codes.

---

## 9. Resource-safety evidence

**No new benchmarks.** The existing white-box tests already are the
correct evidence and already run in milliseconds inside `go test
./... -race -count=1` — a stress benchmark on top of them would be
redundant at best and, per the instructions, a source of CI flakiness at
worst (wall-clock-based benchmarks are exactly the kind of "unstable
benchmark" to avoid). What v1.0 adds is *visibility*, not new proof:

- `docs/evidence-report.md` §7 names the exact tests and the exact bound
  each one exercises (a small table: limit name → test name → asserted
  outcome), so a reviewer doesn't have to read six `loader_v*_test.go`
  files to find them.
- `docs/architecture.md` and `docs/threat-model.md` both state the
  `O(nodes + edges + operations)` complexity bound and where it comes
  from (Kahn's-algorithm topological sort, one bounded BFS per
  lifecycle-bearing approval record, no backtracking anywhere) as the
  actual performance/safety claim — see §19 for why this replaces formal
  benchmarking rather than sitting alongside it.

---

## 10. Architecture documentation: `docs/architecture.md`

**Purpose:** a standalone, linkable document for the "inspect
threat model / architecture" step of the reviewer journey (§25) — the
README's existing architecture section (already good) is the summary;
this is the expanded, diagrammed version for someone who wants the full
picture without reading `docs/phase-*-plan.md` end to end.

**Contents:**

1. **Pipeline overview** — the exact stage list from the task
   description, one paragraph each, cross-referencing real packages:

   ```
   input JSON
       ↓
   internal/loader   (decode + full structural validation, one file per version)
       ↓
   internal/graph    (Kahn's-algorithm topological sort, cycle rejection, canonical BFS trace)
       ↓
   internal/verify   (version-specific Derived Authority computation + invariant evaluation)
       ↓
   internal/explore  (bounded, deterministic BFS reachability — version 6 only, lifecycle safety)
       ↓
   internal/report   (finding assembly, deterministic sort, text/json rendering)
       ↓
   cmd/delegationproof (exit-code mapping, stdout/stderr split)
   ```

2. **One Mermaid diagram** of exactly this pipeline (flowchart, top to
   bottom, one node per package, one branch showing `internal/explore`
   as version-6-only) — this earns its place because it's the single
   picture that answers "where does my JSON go and what touches it,"
   which is exactly the kind of engineering-value diagram the
   instructions ask for, not a decorative one. No second diagram is
   planned — a class diagram of `internal/model` types or a sequence
   diagram of a CLI invocation would be decorative relative to what a
   reviewer actually needs.
3. **Package responsibility table** — reuse/expand the README's existing
   layout table (`internal/model`, `internal/limits`, `internal/loader`,
   `internal/graph`, `internal/explore`, `internal/verify`,
   `internal/report`, `internal/exitcode`, `cmd/delegationproof`), each
   with a one-line "why this boundary exists" note (e.g. why
   `internal/explore` is deliberately not folded into `internal/graph` —
   already well-articulated in the README, moved here as the canonical
   location).
4. **Determinism mechanisms** — the bulleted list already in the README
   ("Determinism" section), moved/linked here as the architectural
   explanation, with the README's copy trimmed to a shorter summary plus
   a link (§12 — avoiding duplicated prose drifting out of sync is part
   of the README restructuring goal).
5. **Version-dispatch note** — one paragraph on how `internal/loader`
   peeks at `{"version": string}` before committing to a struct shape,
   and how six schema versions share no internal model type — this is
   the answer to "how does adding an invariant not risk the earlier
   ones," which is exactly the kind of design-reasoning question an
   architecture doc should pre-empt.

---

## 11. Threat model: `docs/threat-model.md`

**Contents**, per the task's own outline, populated concretely rather
than left as a template:

- **Protected assets/properties** — the six invariants themselves (§2);
  more precisely, the property that a node's derived authority, as
  reported, never overstates what the declared model actually,
  validly grants it.
- **Trust boundaries** — the one input JSON file is the sole boundary;
  everything inside `internal/` operates on already-parsed Go values.
  There is no second boundary (no network, no plugin, no config file
  beyond the one input).
- **Attacker-controlled inputs** — the entire content of the input file:
  arbitrary JSON syntax (including deliberately malformed/truncated
  bytes), arbitrarily large arrays up to and beyond declared bounds,
  arbitrarily deep/wide delegation graphs, adversarially-crafted cyclic
  lifecycle automata, ids/scopes/targets crafted to be maximally long or
  to collide across namespaces.
- **Attacker goals** (what a malicious *input author* — not a runtime
  attacker, see non-goals below — might want DelegationProof to
  incorrectly do): cause a false `ALLOW`/exit-`0` result on a model that
  actually contains a violation (a soundness break — the single most
  serious possible bug class); cause a crash, panic, hang, or unbounded
  memory allocation (a resource-exhaustion break); cause nondeterministic
  output that could be exploited to make two runs of "the same" model
  disagree.
- **Defenses** — strict distrust / no-partial-credit at every layer
  (§ CLAUDE.md, `TestStrictDistrustNoPartialCredit*`); exhaustive,
  fail-fast-only-when-truly-singular structural validation
  (`internal/loader`); every resource bound as an exported, tested `var`
  (`internal/limits`); no third-party dependency surface (`go.mod` is
  stdlib-only, so there is no supply-chain trust extended to anything
  but the Go toolchain itself); pure data deserialization with no code
  execution, no dynamic loading, no network access, no filesystem access
  beyond the one input file.
- **Fail-closed behavior** — restated precisely from §8: malformed input
  → exit 2; resource bound exceeded → exit 2; incomplete/truncated
  lifecycle exploration → `approval_lifecycle_unproven`, exit 1 (a DENY,
  never a silent pass); any invalid incoming edge/approval → contributes
  nothing to the recipient's derived state.
- **Resource exhaustion controls** — the `internal/limits` table
  (already in the README, cross-referenced here), each bound named as a
  hard, enforced ceiling that turns unbounded input into a bounded exit-2
  error rather than a panic/hang.
- **Assumptions** — the input document is the sole source of truth;
  DelegationProof does not verify it against any external system; a
  principal's own declared root authority is the axiomatic root of
  trust; a `requester`/`approver`/`lifecycle` declaration is a claim by
  the document's author, not an authenticated or observed fact (each of
  these is already precisely worded in the README's "Security
  assumptions" section — reused verbatim here as the threat model's
  formal assumptions list, not rewritten).
- **Non-goals** (security-relevant framing, distinct from the product
  non-goals list in README/each phase plan): DelegationProof is **not**
  a runtime enforcement point, is **not** watching real agent/tool
  traffic, is **not** an identity/authentication system, and a "threat"
  to this tool is a bug in its own analysis (soundness, crash,
  nondeterminism) — not a real-world attacker exploiting the *modeled*
  system, which is entirely outside what a static analyzer can see.
- **Limitations** — a dishonest or careless model author can declare a
  topology that doesn't match reality, and DelegationProof has no way to
  detect that; the tool proves properties of the *document*, never of
  the *world* the document claims to describe.

This document should open with one sentence making the static-vs-runtime
distinction unmissable, exactly as instructed: **"DelegationProof
analyzes a declared delegation model; it is not a runtime enforcement
point, an authentication system, or an observer of real agent traffic."**

---

## 12. README restructuring

The current README is already technically excellent (worked examples,
exact exit-code table, full determinism explanation, honest security
assumptions) but reads as one long linear document optimized for someone
who already intends to read all of it. v1.0 restructures it around the
5-minute reviewer journey (§25) without cutting rigor — the rigor moves
to the linked documents (`docs/architecture.md`,
`docs/threat-model.md`, `docs/evidence-report.md`) rather than being
deleted.

**Recommended information hierarchy** (evaluated against the current
content, not blindly copied from the task prompt):

1. One-sentence value proposition (new — currently the README opens
   directly into the six-invariant list, which is correct detail but not
   a hook).
2. The security problem in 2–3 sentences (new, short framing before the
   existing invariant list).
3. 60-second quick start: build command, then *one* `verify` invocation
   against a DENY example with its real output pasted inline (the
   `billing-refund.json` example already serves this purpose almost
   verbatim — move it up, don't rewrite it).
4. "What it proves" — replace the current inline six-invariant prose
   with the §2 inventory table (proves / does not prove), linking to
   `docs/threat-model.md` for the full statement.
5. Architecture — trim the current full section to a short summary plus
   a link to `docs/architecture.md` (avoids the two documents drifting
   out of sync, per §10).
6. Supported model versions — the existing version-dispatch explanation,
   largely unchanged.
7. Installation/build — expanded per §13 (go install / source / release
   binaries), not just the current source-build line.
8. Remaining worked examples (2 through 6) — unchanged content, moved
   later since example 1 already appears in the quick start.
9. Security guarantees / limitations — link to `docs/threat-model.md`
   rather than duplicating its full content; keep the existing
   "Security assumptions" paragraph as a short pull-quote.
10. Resource bounds table — unchanged, stays in README (it's short and
    load-bearing enough to want inline, not just linked).
11. Determinism — trim to a short summary plus link to
    `docs/architecture.md` §4 (per §10 above).
12. Testing / verification — replace the current bare `go test` line
    with `./scripts/verify.sh` as the primary instruction, `go test
    ./... -race -count=1` kept as the "just the tests" alternative.
13. CI — one line + badge (§24), linking to the workflow file.
14. Release information — version, install-from-release instructions,
    checksum verification instructions (§14/§16).
15. Documentation links — a short list: architecture, threat model,
    evidence report, schema, each phase plan, `SECURITY.md`,
    `CONTRIBUTING.md`.
16. Non-goals — unchanged, stays near the end.

This is a reorganization plan, not new prose to draft now — README.md is
explicitly out of scope for this task's file changes.

---

## 13. Installation / distribution

Three install paths, all evaluated, all recommended (they serve
different audiences, none is redundant):

1. **`go install github.com/SamudralaAjaykumarrr/delegationproof/cmd/delegationproof@latest`**
   (or `@v1.0.0` once tagged) — zero release-engineering dependency,
   works the moment the module is public and tagged, best for anyone
   who already has Go installed. Recommended as the primary
   "quick start" install path in the restructured README.
2. **Source build** — already documented (`go build -o
   bin/delegationproof ./cmd/delegationproof`), kept as-is; best for
   contributors and anyone auditing the exact code they're running.
3. **GitHub Release binaries** (§14) — best for a reviewer without a Go
   toolchain who wants the fastest possible "try it" path, and for the
   installability leg of the core productization goal
   ("clone → build" is not the only on-ramp a credible v1.0 needs).

**No packaging ecosystem** (Homebrew tap, `apt`/`deb`, Scoop, `npm`
wrapper, Docker image) is recommended for v1.0 — each adds an ongoing
maintenance surface (a Homebrew formula needs updating per release, a
Docker image needs its own base-image supply-chain story) disproportionate
to a single-binary, stdlib-only CLI's actual distribution need. `go
install` plus raw release binaries already cover "has Go" and "doesn't
have Go" — the two audiences that matter.

---

## 14. Release artifacts

**Targets** (5, matching the task's own evaluation list minus
`windows/arm64`, which is not yet a mainstream target and would be the
first genuinely speculative addition):

| GOOS | GOARCH | Archive |
|---|---|---|
| linux | amd64 | `.tar.gz` |
| linux | arm64 | `.tar.gz` |
| darwin | amd64 | `.tar.gz` |
| darwin | arm64 | `.tar.gz` |
| windows | amd64 | `.zip` |

**Naming convention:** `delegationproof_<version>_<os>_<arch>.<ext>`,
e.g. `delegationproof_v1.0.0_linux_amd64.tar.gz`,
`delegationproof_v1.0.0_windows_amd64.zip` — version included in the
filename (not just the release page) so a downloaded artifact is
self-identifying even after being moved/renamed locally.

**Archive contents:** the single binary (`delegationproof` or
`delegationproof.exe`) plus a copy of `LICENSE` and a short `README.txt`
pointer back to the GitHub repository — not the full README/docs tree
(keeps archives small; full docs are one clone away).

**Checksums:** a single `checksums.txt` at the release root, generated
by `sha256sum` over every archive, in a format `sha256sum -c
checksums.txt` can directly verify — this is the one supply-chain
mechanism every release *must* have (§15).

**Compressed archives: yes, both formats needed** — `.tar.gz` preserves
Unix executable permissions on extraction (a bare uploaded binary loses
its `+x` bit through some download paths and archive-less GitHub asset
handling is generally worse UX); `.zip` is the Windows-native
expectation. This is standard practice, not gold-plating, for a
cross-platform CLI release.

---

## 15. Software supply-chain evidence

Evaluated proportionately, per the explicit instruction not to add
heavyweight tooling for a stdlib-only project:

- **Checksums (`checksums.txt`): yes, required.** Cheapest possible
  integrity mechanism, zero new tooling (`sha256sum` is already on every
  CI runner), directly answers "did I download what was actually built
  from this commit."
- **GitHub artifact attestation / build provenance
  (`actions/attest-build-provenance`): recommended as a nice-to-have,
  not a blocker.** It's a first-party GitHub action (no new third-party
  trust), costs one extra CI step, and gives a reviewer a
  cryptographically-verifiable link from a downloaded binary back to the
  exact workflow run and commit that built it — genuinely more trust
  than checksums alone (checksums prove the file wasn't corrupted in
  transit; attestation proves it was actually built by this repository's
  CI from this source, not hand-uploaded). Include it in the release
  workflow design (§16) as an additional step after artifact build,
  gated so its absence never blocks a release if it's unavailable in a
  given runner context.
- **SBOM (CycloneDX/SPDX): explicitly rejected for v1.0.** `go.mod`
  itself already *is* the complete, exhaustive dependency inventory —
  it declares zero third-party dependencies. Generating a formal SBOM
  document for a project with nothing to enumerate is pure ceremony that
  would exist only to satisfy a checklist, not to convey any information
  a reader can't get from reading `go.mod` in five seconds. Document
  this reasoning explicitly in `docs/evidence-report.md` rather than
  silently omitting it, so a reviewer doesn't wonder whether it was
  simply forgotten.
- **Dependency inventory: yes, but as a one-line statement, not a
  generated artifact** — "zero third-party dependencies; `go.mod`
  requires only the Go standard library," stated in
  `docs/evidence-report.md` and the README, verifiable by any reviewer
  running `go list -m all` and seeing only the module itself.

---

## 16. Release workflow

**File:** `.github/workflows/release.yml` (not created by this plan).

**Trigger:** `push` of a tag matching `v[0-9]+.[0-9]+.[0-9]+` (strict
semver, no `-rc`/`-beta` suffix handling needed for a first v1.0 —
pre-release tag support can be added later if the project ever needs
release candidates, which is not a v1.0 requirement).

**Jobs, in dependency order:**

1. **`verify`** — identical to the CI `verify` job (§4): checkout,
   setup-go, `./scripts/verify.sh`. This is the hard gate: **a failed
   test suite must never be able to publish a release.** Implemented by
   making every subsequent job declare `needs: verify` — GitHub Actions'
   default behavior already refuses to run a dependent job if its
   `needs` target failed, so this requires no extra logic, only correct
   job graph structure.
2. **`build`** (matrix over the 5 targets in §14, `needs: verify`) —
   `CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags
   "-s -w -X main.version=${{ github.ref_name }}" -o
   delegationproof<ext> ./cmd/delegationproof`, then package into the
   named archive (§14) alongside `LICENSE`, upload as a workflow
   artifact per target.
3. **`checksum`** (`needs: build`) — download all build-job artifacts,
   run `sha256sum` over every archive into one `checksums.txt`, upload
   as a workflow artifact.
4. **`publish`** (`needs: [verify, build, checksum]`) — download all
   artifacts, create the GitHub Release from the pushed tag using the
   GitHub CLI already present on every runner (`gh release create
   "$GITHUB_REF_NAME" ./artifacts/* --title "$GITHUB_REF_NAME" --notes
   "..."`) — deliberately **not** a third-party release action
   (`softprops/action-gh-release` or similar): `gh` is first-party,
   preinstalled, and needs no additional trust or pinning beyond what
   the runner image already provides. Optionally follow with the
   `actions/attest-build-provenance` step from §15.

**Permissions:** default `contents: read` at workflow level; the
`publish` job alone escalates to `contents: write` (the minimum scope
`gh release create` needs), scoped to that single job only — every other
job keeps read-only permissions.

**Failure behavior:** any job failure halts the graph before `publish`
runs at all (native `needs:` semantics — no custom rollback logic
needed); a partially-built release is never published because `publish`
is the last job and depends on everything else. If `publish` itself
fails partway through asset upload, `gh release create` on a
still-existing draft is idempotent enough to re-run manually after
diagnosing the cause — no automatic retry logic is planned or needed for
a low-frequency, human-triggered release process.

**Tag format:** `vMAJOR.MINOR.PATCH` (e.g. `v1.0.0`), matching the
`${{ github.ref_name }}` value injected as the CLI's `--version` string
(§17) — this is the mechanism that keeps "the tag" and "what the binary
reports" from ever silently disagreeing (§28 acceptance criterion "release
tag matches binary version").

---

## 17. Version information

**Recommended: yes, add `delegationproof --version`** (a flag, not a
third subcommand — keeps the existing two-subcommand surface
(`validate`/`verify`) conceptually unchanged; `--version` is checked
before subcommand dispatch, mirroring how most single-binary CLIs treat
it). This materially improves release usability: a reviewer who
downloaded a release binary needs a way to confirm which build they're
actually running, and a bug report without a version string is
substantially harder to triage.

**Implementation shape** (for a future implementation batch, not this
task): a package-level `var version = "dev"` in `cmd/delegationproof`,
overridden at build time via `-ldflags "-X main.version=$TAG"` (already
specified in the release workflow, §16). `go build`/`go install` without
that flag (the source-build and `go install` paths, §13) simply reports
`dev` — a legitimate, honest value for an unreleased/local build, not an
error.

**Zero interaction with deterministic analysis behavior:** `--version`
is handled as the very first argument check in `run()`, before any
subcommand dispatch, before any file I/O, before any invocation of
`internal/loader`/`internal/verify`. It cannot appear in, be confused
with, or perturb the output of `validate`/`verify` in any format,
because it is a distinct, mutually-exclusive code path that returns
before either of those functions is ever called. This is the one
functional Go-code change this entire plan recommends — flagged
explicitly here so the owner can approve or defer it independently of
every documentation/CI/release-engineering item, all of which involve
zero Go-code changes.

**Existing test surface affected:** `cmd/delegationproof/main_test.go`'s
usage-string assertions and argument-count tests would need one new case
(`--version` → exit 0, prints `dev` or the injected value) — a small,
additive test, not a change to any existing assertion.

---

## 18. Exit-code demonstration

The contract (already correct and already documented in the README) is
restated here paired with concrete, reproducible commands for the
evidence report and `scripts/verify.sh`:

| Code | Meaning | Demonstration command |
|---|---|---|
| `0` | Clean pass | `delegationproof validate examples/billing-refund.json` (structurally valid — `validate` never evaluates invariants, so this always exits 0 for any of the six examples) |
| `1` | Semantic invariant violation (DENY) | `delegationproof verify examples/billing-refund.json` (any of the six examples — all documented as DENY, §6) |
| `2` | Invalid model/input | `delegationproof verify testdata/malformed/<any-fixture>.json` (any fixture; §8) |
| `3` | CLI usage error | `delegationproof` (no arguments) or `delegationproof verify` (missing path argument) |

**Semantic DENY vs. invalid-input, stated once more for this section's
purpose:** exit `1` is a successful run that found a real problem; exit
`2` is a run that couldn't establish the input was analyzable at all.
Both are "the tool did the right thing," which is why neither is
treated as a `scripts/verify.sh` failure when deliberately exercised
against a malformed/DENY fixture — only an *unexpected* exit code (e.g.
a malformed fixture returning `0` or `1`) is a failure.

---

## 19. Performance characterization

**Formal benchmarking is explicitly rejected for v1.0.** Reasoning:

- The project's real, externally-verifiable performance claim is
  algorithmic: `O(nodes + edges + operations)`, no backtracking, no
  search over the delegation graph, and independently-linear (not
  cross-product/exponential) lifecycle exploration per approval record
  (`docs/architecture.md` §9, README's own existing framing). This is a
  *structural* guarantee, provable by reading the algorithm and by the
  resource-bound tests (§9) — not something a wall-clock benchmark
  number would add confidence to.
- A wall-clock `go test -bench` number is noisy across CI runners
  (shared, variable-performance GitHub-hosted runners), provides no
  security-relevant signal (an attacker cannot exploit "10% slower than
  last commit," but *can* exploit an unbounded allocation — which is
  already covered by the resource-bound tests, not by a benchmark), and
  historically becomes a source of CI flakiness or ignored red herrings
  once it starts failing on infrastructure noise rather than real
  regressions.
- Correctness and resource-bound enforcement (§8, §9) are the properties
  that actually matter for a security tool analyzing untrusted input;
  raw throughput on already-bounded input sizes (max 10,000 nodes,
  50,000 edges) is fast enough on any modern machine that a benchmark
  would only ever produce a reassuring number nobody needed reassurance
  about.

`docs/evidence-report.md` states this rejection explicitly (one short
paragraph, referencing this reasoning) rather than silently having no
performance section — an explicit non-goal is more credible than a
missing one.

---

## 20. Release evidence manifest

**Recommended: yes, but small — `evidence/manifest.json`.** Its value is
narrow and specific: it is the one machine-readable artifact a future
external tool (a badge generator, a second-party auditor's own script, a
package-manager trust check) could parse without scraping Markdown.
`scripts/verify.sh` itself should **not** be rewritten to parse this
JSON (a bash script adding a JSON-parsing dependency, even
`jq`-if-available-else-degrade logic, is more complexity than the value
returned) — the script keeps its own short, hardcoded example list
(§3/§6), and `manifest.json` is instead treated as a release-time,
hand-updated (or later, optionally scripted) declarative record kept in
sync with that same list by the release checklist (§28), not by shared
code. This is a deliberate, named tradeoff, not an oversight.

**Shape:**

```json
{
  "release": "v1.0.0",
  "commit": "<full 40-char sha>",
  "go_version": "go1.26.5",
  "verify_script": "scripts/verify.sh",
  "examples": [
    { "file": "examples/billing-refund.json", "command": "verify", "expected_exit_code": 1, "expected_violation": "authority_amplification" },
    { "file": "examples/billing-context-binding.json", "command": "verify", "expected_exit_code": 1, "expected_violation": "context_binding_violation" },
    { "file": "examples/billing-confused-deputy.json", "command": "verify", "expected_exit_code": 1, "expected_violation": "confused_deputy" },
    { "file": "examples/billing-redelegation-depth.json", "command": "verify", "expected_exit_code": 1, "expected_violation": "delegation_depth_violation" },
    { "file": "examples/billing-approval.json", "command": "verify", "expected_exit_code": 1, "expected_violation": "approval_missing" },
    { "file": "examples/billing-approval-lifecycle.json", "command": "verify", "expected_exit_code": 1, "expected_violation": "approval_lifecycle_unsafe" }
  ],
  "checksums_file": "checksums.txt"
}
```

No embedded checksum values inside `manifest.json` itself (they'd
duplicate `checksums.txt` and risk drifting from it) — it references the
checksums file by name instead. This keeps the manifest small and
single-purpose rather than becoming a second source of truth.

---

## 21. Security disclosure policy: `SECURITY.md`

**Recommended: yes.** Minimal, honest, solo-maintainer-appropriate — no
invented SLA, no invented security team, no CVE-issuing process:

- **Scope:** a "security issue" in DelegationProof means (a) a
  *soundness* bug — an input the tool reports `ALLOW`/exit-`0` for, that
  a careful reading of `docs/phase-*-plan.md` shows should have been a
  DENY (or vice versa in a way that misrepresents the model); or (b) a
  crash, panic, hang, or unbounded resource consumption triggered by
  untrusted input within or at the documented resource bounds. Design
  disagreements, feature requests, and "I wish it also checked X" are
  explicitly **not** security issues — they're ordinary GitHub issues.
- **Reporting channel:** GitHub's private vulnerability reporting
  (Security tab → "Report a vulnerability"), which requires no separate
  email infrastructure and keeps the report private until resolved —
  the standard, zero-infrastructure-cost mechanism for a solo
  open-source maintainer.
- **Response expectation:** stated honestly as best-effort, no fixed SLA
  — a small sentence acknowledging this is a solo project, not a
  corporate promise.
- **Disclosure norms:** ask reporters not to open a public issue before
  a fix is available; credit reporters in the release notes unless they
  prefer anonymity.

---

## 22. Contributing documentation: `CONTRIBUTING.md`

**Recommended: yes, short.** Most of the substantive guidance already
lives in `CLAUDE.md` (deterministic-contract rules, layout, "run these
three commands before considering any change done"); `CONTRIBUTING.md`
should be a short, GitHub-facing pointer plus PR-process specifics that
don't belong in an AI-agent guidance file:

- Go version required (`go.mod`'s pinned version) and how to run
  `./scripts/verify.sh` before opening a PR.
- One line: "This project's deterministic-contract and strict-distrust
  invariants are non-negotiable design decisions, not style
  preferences — see `CLAUDE.md` and the relevant `docs/phase-*-plan.md`
  before changing anything in `internal/verify` or `internal/loader`."
- **Phase-plan immutability, stated explicitly:** `docs/phase-1-plan.md`
  through `docs/phase-6-plan.md` are historical design contracts for
  already-shipped, already-tested behavior — they are not living
  documentation and pull requests should never edit their substance
  (typo fixes excepted). A new capability proposal belongs in a new
  document, never as an edit to a past phase's contract.
- PR expectations: `gofmt`/`vet`/`test -race` green
  (`./scripts/verify.sh` covers this), golden files
  (`testdata/golden/`) regenerated deliberately with a diff reviewed,
  never hand-edited, exactly as CLAUDE.md already instructs.

---

## 23. License

**Confirmed absent** — no `LICENSE`, `LICENSE.md`, or `LICENSE.txt`
anywhere in the repository (checked directly). This is a genuine v1.0
blocker: a public repository with no license grants no rights to use,
modify, or redistribute the code, which undermines the entire "credible,
independently verifiable" goal — a reviewer who can't legally use the
tool can't meaningfully evaluate it as open source.

**This plan does not choose a license.** It surfaces the decision with
the tradeoffs an owner needs:

| Option | Characteristics |
|---|---|
| **MIT** | Shortest, most widely recognized, maximally permissive (allows proprietary derivative use with attribution only). Best if the goal is maximum adoption/portfolio visibility with minimal friction. |
| **Apache-2.0** | Similarly permissive, but adds an explicit patent grant and patent-retaliation clause — common choice for security/infrastructure tooling specifically because of the patent grant. Longer text than MIT. |
| **BSD-3-Clause** | Comparable permissiveness to MIT, plus a non-endorsement clause (can't use the author's name to promote derivatives without permission). Less common than the other two for new Go projects. |

Given this is a solo security-tooling project aimed at demonstrating
engineering credibility and inviting adoption/review, **MIT or
Apache-2.0** are the two worth the owner's real consideration; this plan
takes no further position and implementation must not silently select
one — it requires explicit owner approval before a `LICENSE` file is
created (§29, Batch C).

---

## 24. Repository metadata

- **Description:** a one-line GitHub repo description matching the
  README's new one-sentence value proposition (§12, item 1) once
  drafted — not duplicated here since that sentence doesn't exist yet.
- **Topics:** a short, accurate set — `go`, `security`,
  `static-analysis`, `authorization`, `delegation`, `cli`,
  `deterministic`. Deliberately **excluding** `mcp`/`a2a`/`agent` as
  standalone topics — the project analyzes delegation topologies that
  *could* describe MCP/A2A-style systems, but implements no such
  protocol (confirmed non-goal in every phase plan); a topic tag
  implying protocol integration would overclaim.
- **Badges (README header): exactly three, no more** — CI status (links
  to `.github/workflows/ci.yml`'s badge endpoint), License (once §23 is
  resolved), and latest GitHub Release version. **Explicitly skip** a Go
  version badge — `go.mod`'s pinned version is already one click away
  and a badge for it adds visual clutter without adding information a
  reviewer couldn't get from the "Installation" section in three
  seconds. This directly answers the task's "avoid badge clutter"
  instruction with a concrete count rather than a vague caution.

---

## 25. External reviewer journey (5 minutes)

The README restructuring (§12) and evidence package (§5) are designed
around this exact walk:

- **Minute 1** — README's one-sentence value proposition + security
  problem framing: understand what DelegationProof checks and why it
  matters.
- **Minute 2** — README's 60-second quick start: build, run one `verify`
  command, see real DENY output immediately (no need to write a fixture
  first — the shipped example does it).
- **Minute 3** — read the DENY explanation inline (the `reason` string
  is already designed to be self-explanatory, per every phase plan's
  finding-contract design) and the §2 "what it proves / doesn't prove"
  table.
- **Minute 4** — run `./scripts/verify.sh` themselves: reproduce every
  objective release gate on their own machine in one command, no trust
  required.
- **Minute 5** — skim `docs/architecture.md`'s one Mermaid diagram and
  `docs/threat-model.md`'s opening static-vs-runtime sentence, enough to
  know where to go deeper if they choose to.

---

## 26. Recruiter / hiring-manager journey

Without turning the repository into a résumé, these existing (and
planned) artifacts already communicate the relevant signal, evidence-based
rather than asserted:

- **Independent system design** — six additive schema versions, each a
  strict superset, none rewriting an earlier one (`docs/phase-*-plan.md`,
  `docs/architecture.md`'s version-dispatch note, §10).
- **Security reasoning** — `docs/threat-model.md` (§11): explicit
  attacker model, explicit fail-closed behavior, explicit non-goals
  stated as design decisions rather than gaps.
- **Deterministic algorithms** — the determinism mechanism list (README
  today, `docs/architecture.md` going forward) and the tests that lock
  it down by name.
- **Go engineering** — stdlib-only, zero dependencies, clean `go vet`,
  race-tested concurrency-free-by-design code.
- **Testing rigor** — one malformed fixture per structural-error kind
  across six schema versions, golden-file byte-for-byte assertions,
  permutation-invariance tests, white-box resource-bound tests.
- **Adversarial thinking** — strict distrust / no-partial-credit as a
  named, tested design decision (`TestStrictDistrustNoPartialCredit*`),
  fail-closed lifecycle-exploration truncation.
- **Release engineering** — CI (§4), tag-triggered multi-platform
  release workflow with checksums (§16), least-privilege permissions
  throughout.
- **Documentation quality** — six phase-plan design contracts kept
  immutable as historical record (§22), plus the new
  architecture/threat-model/evidence trio aimed at a reader who isn't
  the author.

No new artifact is proposed solely for this audience — everything above
is already planned for the external-reviewer journey (§25); this section
exists only to confirm the overlap is real, per the task's own framing.

---

## 27. Final LinkedIn/demo readiness

Before any public promotion (not designed here — explicitly out of
scope, "do not design social-media copy yet"), the underlying project
should have, in order of dependency:

1. `./scripts/verify.sh` passing locally and in CI (§3, §4).
2. `LICENSE` present (§23 — owner decision required first).
3. Restructured README live (§12).
4. `docs/architecture.md`, `docs/threat-model.md`,
   `docs/evidence-report.md` published (§5, §10, §11).
5. `v1.0.0` tagged, released, checksums published and independently
   verified by the owner at least once by hand (§14, §16, §28).
6. A terminal screenshot of one real DENY output (captured from the
   actual released binary, not a mockup) — for later use in any
   promotional material, not produced by this plan.
7. A short (under two-minute) recorded demo of the 60-second quick start
   (§25, minutes 1–2) — likewise for later use, not produced here.

Items 6–7 are listed for completeness against the task's own checklist
but are explicitly deferred artifacts, not documents this plan authors.

---

## 28. Release acceptance criteria (numbered, implementation-ready)

1. `main` is clean: `git status --porcelain` empty at the release
   commit.
2. `gofmt -l .` produces no output.
3. `go vet ./...` succeeds.
4. `go test ./... -count=1` passes.
5. `go test ./... -race -count=1` passes.
6. `go build -o bin/delegationproof ./cmd/delegationproof` succeeds.
7. `./scripts/verify.sh` exits 0 end to end (supersedes/covers 2–6, kept
   itemized for auditability).
8. CI (`.github/workflows/ci.yml`) is green on the exact commit being
   tagged.
9. Every documented DENY example (§6) reproduces its documented exit
   code and finding kind when run against the release binary.
10. The deterministic-repeated-run and permutation-invariance evidence
    (§7) reproduces (identical hashes) against the release binary.
11. Every `testdata/malformed/` fixture reproduces exit `2` against the
    release binary.
12. `docs/architecture.md` exists, its pipeline description matches the
    actual package list in `internal/` (no drift from a rename/removal).
13. `docs/threat-model.md` exists and its non-goals section matches the
    non-goals already stated across `docs/phase-*-plan.md`/README (no
    contradiction).
14. `docs/evidence-report.md` exists, and every command in it has been
    re-run against the release commit (not stale output from an earlier
    commit).
15. `LICENSE` exists, its SPDX identifier matches what the README states,
    and it was explicitly approved by the repository owner (§23 — not
    silently chosen).
16. `SECURITY.md` exists and its reporting channel is live/functional
    (the GitHub private-vulnerability-reporting feature is enabled for
    the repository).
17. `CONTRIBUTING.md` exists and its stated Go version matches
    `go.mod`.
18. The `v1.0.0` git tag exists, points at the exact commit that passed
    criteria 1–14, and matches the version string the release binaries
    report via `--version` (if §17 is implemented) or, at minimum,
    matches the version embedded in each release archive's filename.
19. Release binaries exist for all five targets in §14, each
    successfully extracted and executed (`--version` or `validate` on a
    trivial fixture) on at least one real machine per OS family the
    owner has access to, not solely inferred from a successful
    cross-compile.
20. `checksums.txt` is published alongside the release archives, and
    `sha256sum -c checksums.txt` succeeds against freshly re-downloaded
    archives (verified once, by hand, by the releaser — not merely
    generated and assumed correct).
21. No secrets, tokens, or credentials appear anywhere in the tagged
    commit's tree (a manual `git log -p` skim plus a check that no
    `.env`/credential-shaped file was ever added — this repository's
    history is short enough for a direct check, no scanner tooling
    required for v1.0).
22. No untracked or generated junk in the tagged tree (`bin/`,
    `*.test`, `*.out` remain gitignored and absent from the commit;
    `evidence/manifest.json` if adopted (§20) is deliberately checked
    in, not generated-and-gitignored).
23. All links in `README.md` and every `docs/*.md` file resolve (every
    relative path exists in the tree; no dangling reference to a
    document this plan didn't actually create).
24. `go.mod` remains stdlib-only (`go list -m all` shows only the
    module itself) — confirms nothing in the productization work
    accidentally introduced a dependency.
25. No file under `internal/`, `cmd/`, `schemas/`, `examples/`, or
    `testdata/` differs from the pre-release-push commit except the one
    sanctioned `--version` change (§17), if the owner chooses to
    implement it — i.e., v1.0 shipped as *productization*, not as
    disguised feature work.

---

## 29. Implementation sequence

Five batches, ordered so each is independently mergeable and each
later batch depends only on earlier ones already having landed — no
batch requires guessing at a later batch's output.

**Batch A — Local verification + CI**
`scripts/verify.sh` (§3), `.github/workflows/ci.yml` (§4). Landing this
first means every subsequent batch's own documentation/release work is
itself continuously checked from the moment it exists.

**Batch B — Architecture, threat-model, and evidence documentation**
`docs/architecture.md` (§10), `docs/threat-model.md` (§11),
`docs/evidence-report.md` (§5) — the evidence report is written last
within this batch since it cites commands that Batch A's
`scripts/verify.sh` must already exist to reference.

**Batch C — Release engineering + legal**
License decision and `LICENSE` file (§23 — requires an explicit owner
go/no-go before this batch can close), `SECURITY.md` (§21),
`CONTRIBUTING.md` (§22), `.github/workflows/release.yml` (§16),
`evidence/manifest.json` (§20), optional `--version` flag (§17) if the
owner approves the one Go-code change in this plan.

**Batch D — Final README and reviewer experience**
README restructuring (§12) — deliberately last among the documentation
work because it links to every artifact from Batches A–C
(`scripts/verify.sh`, the three new `docs/*.md` files, `LICENSE`,
`SECURITY.md`, `CONTRIBUTING.md`, the CI badge) and would otherwise need
a second pass to fix dangling links. Repository metadata (§24: topics,
description, badges) lands alongside this batch since badges depend on
CI/license/release existing first.

**Batch E — v1.0.0 release verification**
Tag `v1.0.0`, let the Batch C release workflow run, then manually walk
the full §28 acceptance-criteria checklist against the published
release (including the hands-on binary-execution and checksum-verification
steps that can't be fully automated) before calling v1.0.0 done.

---

## 30. Explicit non-goals of this capstone

This plan adds no Phase 7 security invariant, no networking, no hosted
API, no web UI, no database, no runtime enforcement, no OAuth, no
MCP/A2A runtime, no LLM integration, no arbitrary policy DSL, no
SAT/SMT solving, no symbolic execution, no distributed model checking, no
telemetry backend, and no unrelated cloud infrastructure. Every artifact
planned above is productization and proof over the existing, complete
Phases 1–6 — evidence, reproducibility, release engineering, and
documentation, not new domain capability. The one Go-code change this
plan recommends (`--version`, §17) is explicitly scoped as inert with
respect to `validate`/`verify` semantics and is independently
approvable/deferrable without affecting anything else in this plan.
