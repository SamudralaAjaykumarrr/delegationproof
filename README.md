# DelegationProof

[![CI](https://github.com/SamudralaAjaykumarrr/delegationproof/actions/workflows/ci.yml/badge.svg)](https://github.com/SamudralaAjaykumarrr/delegationproof/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/SamudralaAjaykumarrr/delegationproof?include_prereleases&sort=semver)](https://github.com/SamudralaAjaykumarrr/delegationproof/releases)

DelegationProof is a small, dependency-free, offline, deterministic CLI
that statically checks a *declared* agent-delegation topology for six
composable authority-safety invariants — proving properties of the
document you feed it, not of a live system, with no network access and
no trust extended beyond the Go standard library.

## The problem

Multi-agent and delegated-authority systems (an agent calling another
agent, a service acting on a user's behalf, a workflow re-delegating a
capability downstream) fail in specific, recurring ways: authority that
was never actually granted gets exercised anyway; a capability valid for
one target gets used against another; a legitimate actor gets used as a
confused deputy on behalf of someone who never held the capability; a
re-delegation budget or a required approval gets silently bypassed. None
of this requires a runtime exploit — it only requires the *declared
model* of who can do what to be wrong, or to be trusted without being
checked. DelegationProof checks the declared model.

## 60-second quick start

```sh
git clone https://github.com/SamudralaAjaykumarrr/delegationproof.git
cd delegationproof
go build -o bin/delegationproof ./cmd/delegationproof
./bin/delegationproof verify examples/billing-refund.json
```

```
DENY
1 finding(s)

[1] authority_amplification (operation)
  actor:    agent-b
  action:   billing.refund
  requires: billing:write
  held:     billing:read
  trace:    user -> agent-a -> agent-b -> billing.refund
  reason:   billing:write was never present in the valid delegation chain reaching agent-b
```

`agent-b` was only ever delegated `billing:read`, two hops down from the
principal `user` — never `billing:write`. `billing.view` (which requires
`billing:read`) would have passed silently; `billing.refund` (which
requires `billing:write`) fails with exit code `1` because that scope
was never present anywhere in the valid delegation chain reaching
`agent-b`. That's the whole product: a declared graph in, a deterministic
report out, no partial credit for an invalid path.

## What it proves

Given a graph of **principals** (root authority holders), **agents**
(participants whose authority is only ever *derived* from valid incoming
delegations), declared **delegation** edges, and **operations** (points
where an actor exercises a capability, optionally on behalf of a
**requester**), DelegationProof computes each node's Derived Authority in
one deterministic topological pass and reports every place a delegation
edge or operation violates one of six invariants:

| # | Invariant | What it proves | What it does NOT prove |
|---|---|---|---|
| 1 | Authority Non-Amplification | No node's derived authority ever includes a capability never validly delegated to it. An invalid incoming edge contributes nothing — not partial credit. | That the declared model matches a real running system, or that a root principal's own authority was legitimately obtained. |
| 2 | Context/Target Binding Preservation | A capability valid for one target cannot be exercised or transmitted for a different target. | That a `target` string corresponds to any real access-control boundary — it's an opaque label the document author chooses. |
| 3 | Requester Authorization Preservation | An operation's `requester` is independently checked against the same Derived Authority computation as `actor` — a valid actor cannot be used as a confused deputy for a requester who never independently held the capability. | That `requester` is an authenticated identity — it's a declared label. |
| 4 | Delegation Depth Preservation | A capability cannot be re-delegated more hops than its root-declared `max_delegation_depth` budget permits, deterministically, regardless of how many valid paths deliver it. | That the budget corresponds to any real re-delegation counter enforced by a running system — it's a policy assertion. |
| 5 | Approval Preservation | An operation exercising an approval-gated capability is illegitimate unless at least one declared approval names an approver who independently holds that exact capability. | That a named approver is a real, authenticated person, or that a real workflow produced the sign-off. |
| 6 | Temporal Approval Preservation | A standing-backed approval counts only if *every* state reachable from its declared lifecycle's initial state is `"approved"` — proved by bounded, deterministic BFS. Truncated or unsafe exploration is fail-closed, never `ALLOW`. | That the declared lifecycle corresponds to any real revocation/expiry event, or *when* in real time an operation executes relative to a transition. |

Full statement, attacker model, and non-goals: [`docs/threat-model.md`](docs/threat-model.md).
Every claim above is backed by an actual command and actual test names in
[`docs/evidence-report.md`](docs/evidence-report.md) — reproduce it
yourself rather than trusting this table.

Each invariant is a separate, additive input schema version (`"1"`
through `"6"`) rather than a single ever-growing format — see
[`docs/phase-1-plan.md`](docs/phase-1-plan.md) through
[`docs/phase-6-plan.md`](docs/phase-6-plan.md) for the full design
contract each phase follows, including what is deliberately **out of
scope** and how later phases attach to earlier ones without rewriting
them.

## Architecture

```
input JSON → internal/loader → internal/graph → internal/verify → internal/report → cmd/delegationproof
```

`internal/loader` decodes and exhaustively validates the input;
`internal/graph` topologically sorts the delegation DAG and computes
canonical traces; `internal/verify` computes Derived Authority and
evaluates every invariant applicable to the document's schema version
(consulting `internal/explore`'s bounded lifecycle BFS for version 6
only); `internal/report` assembles and renders findings;
`cmd/delegationproof` is the only package touching `os.Args`/stdout/
stderr. Full pipeline diagram, package-responsibility table, and the
version-dispatch design: [`docs/architecture.md`](docs/architecture.md).

## Supported model versions

The input document's `"version"` field (`"1"` through `"6"`) selects
which schema and which set of invariants apply — there is no separate
CLI flag for this. Later versions are strict, additive extensions of
earlier ones; a version-1 document is checked only against Authority
Non-Amplification, a version-6 document against all six invariants. See
[`schemas/model.md`](schemas/model.md) for the full field-by-field
contract (`internal/loader` is the sole runtime source of truth — the
schema doc is a mirror, not a validation dependency).

## Installation

**`go install`** (works once the module is public and tagged; best if
you already have Go):

```sh
go install github.com/SamudralaAjaykumarrr/delegationproof/cmd/delegationproof@latest
# or a specific release: @v1.0.0
```

**Source build** (best for contributors and anyone auditing the exact
code they're running):

```sh
git clone https://github.com/SamudralaAjaykumarrr/delegationproof.git
cd delegationproof
go build -o bin/delegationproof ./cmd/delegationproof
```

**Release binaries** — see "Release information" below; best if you
want to try the tool without a Go toolchain.

No third-party dependencies at any install path; `go.mod` is stdlib-only.

## Usage

```
delegationproof validate <model.json>
delegationproof verify   <model.json> [--format text|json]
delegationproof --version
```

- `validate` parses and structurally validates the input only (schema
  shape, referential integrity, acyclicity, resource bounds). It never
  evaluates any invariant — useful as a fast pre-check or editor hook.
- `verify` runs validation, and if the model is structurally valid,
  evaluates every invariant applicable to that document's schema version
  and reports findings (or a clean pass).
- `--format` (default `text`) selects `text` (human-readable) or `json`
  (the machine-readable finding contract, one JSON object to stdout, no
  trailing prose).
- `--version` prints the build's version string and exits `0`, checked
  before any subcommand dispatch or file I/O — a source build or `go
  install` reports `dev`; a tagged release binary reports its tag.

Errors always go to stderr; result output always goes to stdout — safe to
pipe/parse stdout even when stderr carries diagnostic noise.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Clean pass — structurally valid model, and (for `verify`) zero findings. |
| `1` | Invariant violated — `verify` found one or more findings, of any kind, in any combination. This is a semantic DENY: the tool worked correctly. |
| `2` | Model/input problem — file not found, invalid JSON, any structural validation error, or a resource bound exceeded. Applies to both `validate` and `verify`, for every schema version. |
| `3` | CLI usage error — wrong argument count, unknown flag, unknown subcommand. Never about the model's content. |

## Worked examples

[`examples/billing-refund.json`](examples/billing-refund.json) (version 1)
is the quick-start example above: a two-hop chain (`user → agent-a →
agent-b`) where `billing:write` was never delegated past the root.

[`examples/billing-context-binding.json`](examples/billing-context-binding.json)
(version 2) shows the same shape of violation one level more precise: a
scope *is* validly held, but only for a different target than the one
exercised — `context_binding_violation`, not amplification.

```sh
$ delegationproof verify examples/billing-context-binding.json
DENY
1 finding(s)

[1] context_binding_violation (operation)
  actor:         billing-agent
  action:        read-record
  requires:      billing:read@payroll-service
  held:          billing:read@billing-service
  bound targets: billing-service
  trace:         user -> billing-agent -> read-record
  reason:        billing:read is held by billing-agent only for billing-service, which does not include payroll-service
```

[`examples/billing-confused-deputy.json`](examples/billing-confused-deputy.json)
(version 3) shows an actor that legitimately holds the required capability
being induced to act on behalf of a requester who never independently held
it — `confused_deputy`.

```sh
$ delegationproof verify examples/billing-confused-deputy.json
DENY
1 finding(s)

[1] confused_deputy (operation)
  actor:            billing-agent
  requester:        support-agent
  action:           refund-b
  requires:         billing:refund@billing-service
  actor held:       billing:refund@billing-service
  requester held:   billing:read@billing-service
  requester bound:  (none)
  actor trace:      admin -> billing-agent -> refund-b
  requester trace:  admin -> support-agent
  reason:           refund-b requires billing:refund@billing-service, which billing-agent validly holds, but requester support-agent has never held billing:refund under any target — billing-agent is being induced to exercise authority support-agent was never granted
```

[`examples/billing-redelegation-depth.json`](examples/billing-redelegation-depth.json)
(version 4) declares `billing:refund@billing-service` with
`max_delegation_depth: 1` at the root. The first hop
(`admin → billing-agent`) is legitimate; the second
(`billing-agent → support-agent`) exceeds the declared budget:

```sh
$ delegationproof verify examples/billing-redelegation-depth.json
DENY
2 finding(s)

[1] delegation_depth_violation (delegation_edge)
  delegator:     billing-agent
  delegatee:     support-agent
  declared:      billing:refund@billing-service
  excess:        billing:refund@billing-service (configured max: 1, remaining: 0)
  trace:         admin -> billing-agent -> support-agent
  reason:        billing-agent attempted to delegate billing:refund@billing-service to support-agent, but billing-agent's remaining delegation budget for this capability is 0 (configured maximum: 1) — it may no longer be redelegated

[2] authority_amplification (operation)
  actor:         support-agent
  action:        refund-deep
  requires:      billing:refund@billing-service
  held:          (none)
  bound targets: (none)
  trace:         support-agent -> refund-deep
  reason:        billing:refund@billing-service was never present in the valid delegation chain reaching support-agent
```

Two findings, at two different points, with no masking: the *cause* is the
edge-level depth violation, and the *consequence* is that `support-agent`
never actually received the capability at all, so its own operation fails
Non-Amplification independently. `billing-agent`'s own use of the
capability (`refund-ok`, at exactly its budget boundary — remaining depth
0) still passes: usability and delegability are independently-gated
properties of the same held capability.

[`examples/billing-approval.json`](examples/billing-approval.json)
(version 5) declares `admin` holding both `billing:refund` and
`billing:void` for `billing-service`, each requiring approval, delegated
one hop to `billing-agent`. A declared approval record names
`compliance-officer` — who independently, axiomatically holds
`billing:refund@billing-service` — as an approver for that capability
only:

```sh
$ delegationproof verify examples/billing-approval.json
DENY
1 finding(s)

[1] approval_missing (operation)
  actor:               billing-agent
  requester:           admin
  action:              void-unapproved
  requires:            billing:void@billing-service
  declared approvers:  (none)
  trace:               admin -> billing-agent -> void-unapproved
  reason:              void-unapproved requires billing:void@billing-service, which billing-agent validly holds and admin is authorized to request, but billing:void@billing-service requires approval and no approval has been declared for it
```

`refund-approved` (requiring `billing:refund@billing-service`) passes
silently — `billing-agent` validly holds it, `admin` is authorized to
request it, and a standing-backed approval exists. `void-unapproved`
(requiring `billing:void@billing-service`) fails: that capability also
requires approval, but no approval record was ever declared for it —
`approval_missing`, not amplification, binding, depth, or confused-deputy,
since all four of those invariants pass for this operation.

[`examples/billing-approval-lifecycle.json`](examples/billing-approval-lifecycle.json)
(version 6) declares the same `admin`/`billing-agent` shape, but both
approval records now carry an explicit `lifecycle`. `compliance-officer`'s
approval of `billing:refund` declares a lifecycle that never leaves
`"approved"`; its approval of `billing:void` declares one that can reach
`"revoked"`:

```sh
$ delegationproof verify examples/billing-approval-lifecycle.json
DENY
1 finding(s)

[1] approval_lifecycle_unsafe (operation)
  actor:               billing-agent
  requester:           admin
  action:              void-unsafe
  requires:            billing:void@billing-service
  declared approvers:  compliance-officer
  unsafe approver:     compliance-officer
  unsafe state:        revoked
  lifecycle trace:     approved -[revoke]-> revoked
  trace:               admin -> billing-agent -> void-unsafe
  reason:              void-unsafe requires billing:void@billing-service, which billing-agent validly holds and admin is authorized to request, and billing:void@billing-service requires approval; compliance-officer independently hold standing, but none of their declared approval lifecycles can be proven to remain in state 'approved' — compliance-officer's can reach state 'revoked' via approved -[revoke]-> revoked, so it cannot be statically relied upon at time of exercise
```

`refund-safe` passes silently — `compliance-officer`'s approval of that
capability declares a lifecycle whose only reachable state is `"approved"`.
`void-unsafe` fails even though a standing-backed approval genuinely
exists (version 5's own checks all pass): the approval's *own declared
lifecycle* can reach `"revoked"`, so it can never be statically relied
upon — `approval_lifecycle_unsafe`, the temporal analogue of
`approval_missing`.

## Input model

In brief, the version-1 shape:

```json
{
  "version": "1",
  "principals": [{ "id": "user", "authority": ["billing:read", "billing:write"] }],
  "agents": [{ "id": "agent-a" }],
  "delegations": [{ "delegator": "user", "delegatee": "agent-a", "authority": ["billing:read"] }],
  "operations": [{ "actor": "agent-a", "action": "billing.view", "requires": "billing:read" }]
}
```

and the version-4 shape, showing every field added by every phase:

```json
{
  "version": "4",
  "principals": [
    { "id": "admin", "authority": [
      { "scope": "billing:refund", "target": "billing-service", "max_delegation_depth": 1 }
    ] }
  ],
  "agents": [{ "id": "billing-agent" }],
  "delegations": [
    { "delegator": "admin", "delegatee": "billing-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin", "action": "refund", "requires": "billing:refund", "target": "billing-service" }
  ]
}
```

and the version-6 shape, adding an optional temporal lifecycle to an
individual approval record on top of version 5:

```json
{
  "version": "6",
  "principals": [
    { "id": "admin", "authority": [
      { "scope": "billing:refund", "target": "billing-service", "max_delegation_depth": 1, "requires_approval": true }
    ] }
  ],
  "agents": [{ "id": "billing-agent" }],
  "delegations": [
    { "delegator": "admin", "delegatee": "billing-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] }
  ],
  "approvals": [
    { "approver": "compliance-officer", "scope": "billing:refund", "target": "billing-service",
      "lifecycle": {
        "initial": "approved",
        "states": ["approved", "revoked"],
        "transitions": [ { "from": "approved", "to": "revoked", "event": "revoke" } ]
      } }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin", "action": "refund", "requires": "billing:refund", "target": "billing-service" }
  ]
}
```

Authority is an opaque, exact-match scope string — no wildcards, no
hierarchy. Unknown fields anywhere are rejected. An agent may never
declare its own `authority`; it is always derived. The delegation graph
must be a DAG; principals cannot be delegation targets. A lifecycle
automaton, by contrast, is never required to be acyclic. All structural
problems in one input are collected and reported together, not
fail-fast. See [`schemas/model.md`](schemas/model.md) for the complete
field-by-field contract across all six versions.

## Security guarantees and limitations

DelegationProof is a static, offline analyzer. It proves properties
about a *declared* model; it does not observe, enforce, or intercept
real agent/tool traffic, and it does not verify that a real running
system matches its declared model.

> A principal's declared authority is the axiomatic root of trust — the
> tool does not verify how a principal obtained it. A `requester`,
> `approvals[]` entry, and `lifecycle` declaration are each a claim by
> the document's author, checked for internal consistency against the
> rest of the declared model, never against reality.

Full attacker model, trust boundaries, fail-closed behavior, and
non-goals: [`docs/threat-model.md`](docs/threat-model.md).

## Resource bounds

Fixed, exported variables (`internal/limits`), exceeding any of which is a
`resource_limit_exceeded` validation error — never a panic, never an
unbounded allocation, never a hang:

| Limit | Value |
|---|---|
| Max input file size | 5 MiB |
| Max nodes (principals + agents) | 10,000 |
| Max delegation edges | 50,000 |
| Max operations | 10,000 |
| Max scope-string length | 256 bytes |
| Max id length | 128 bytes |
| Max target length | 128 bytes |
| Max authority-set size (per principal or per edge) | 256 |
| Max delegation chain depth (longest simple path, resource-safety valve) | 64 |
| Max declared `max_delegation_depth` value (policy-value safety valve) | 64 |
| Max approvals (top-level `approvals` array size) | 10,000 |
| Max lifecycle states (per approval record's `lifecycle.states`) | 32 |
| Max lifecycle transitions (per approval record's `lifecycle.transitions`) | 128 |
| Max exploration states per lifecycle (runtime BFS safety valve) | 32 |

## Determinism

Identical input always produces byte-identical output, and reordering
any array in a semantically-equivalent model never changes the output —
Kahn's-algorithm topological sort and every trace-finding BFS break ties
by ascending lexicographic id, findings are sorted by a fixed total
order, and multi-path computations use commutative/associative
operations or an explicit tie-break rather than depending on Go map
iteration order anywhere. Full mechanism list:
[`docs/architecture.md`](docs/architecture.md#determinism-mechanisms).
Locked down by `TestJSONFormatInputArrayPermutationInvariance*` and
`TestJSONFormatDeterministicAcrossRepeatedRuns*` (one pair per schema
version) — see [`docs/evidence-report.md`](docs/evidence-report.md) §5
for byte-for-byte hash evidence against the shipped binary.

## Verification and testing

The one-command release gate, runnable identically by a human or by CI:

```sh
./scripts/verify.sh
```

Runs, in order, fail-fast: `gofmt -l .`, `go vet ./...`, `go test
./... -race -count=1`, a build, deterministic/permutation-invariance
checks against the just-built binary for every `examples/*.json` file,
an exit-code-2 check against every `testdata/malformed/*.json` fixture,
and a repository-hygiene check (`git status --porcelain` empty). Exits
`0` only if every gate passes.

Just the tests:

```sh
go test ./... -race -count=1
```

Test categories include: clean-pass golden output, every structural
error kind in each phase's design contract, strict-distrust ("no
partial credit") semantics, deterministic finding ordering, byte-
identical output across semantically-equivalent reordered input, every
resource bound (exercised via lowered `internal/limits` values), CLI
exit codes and the stdout/stderr split, and no-panic fuzzing over
truncated/mutated input — for every schema version, independently.

## CI

[`.github/workflows/ci.yml`](.github/workflows/ci.yml) runs
`./scripts/verify.sh` on every push to `main` and every pull request
targeting it (the `verify` job), plus a compile-only `cross-build`
matrix over all five release targets. Two first-party GitHub actions
only (`actions/checkout`, `actions/setup-go`), both pinned by commit
SHA. `permissions: contents: read` at the workflow level — CI never
writes to the repository.

## Release information

Version: **v1.0.0** (see [`docs/v1-release-plan.md`](docs/v1-release-plan.md)
for the release definition and acceptance criteria). `delegationproof
--version` reports the tag a release binary was built from, or `dev` for
a source build.

Release binaries — [`.github/workflows/release.yml`](.github/workflows/release.yml),
triggered by pushing a `vMAJOR.MINOR.PATCH` tag — are published for:

| GOOS | GOARCH | Archive |
|---|---|---|
| linux | amd64 | `.tar.gz` |
| linux | arm64 | `.tar.gz` |
| darwin | amd64 | `.tar.gz` |
| darwin | arm64 | `.tar.gz` |
| windows | amd64 | `.zip` |

Each archive is named `delegationproof_<version>_<os>_<arch>.<ext>` and
contains the binary, `LICENSE`, and a short `README.txt` pointer back to
this repository. A `checksums.txt` at the release root lets you verify a
downloaded archive:

```sh
sha256sum -c checksums.txt
```

The release workflow will not publish unless `./scripts/verify.sh`
passes first (`needs: verify` on every later job) — a failed test suite
can never produce a release.

**License:** DelegationProof is licensed under the
[Apache License 2.0](LICENSE).

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — pipeline diagram,
  package responsibilities, determinism mechanisms, version dispatch.
- [`docs/threat-model.md`](docs/threat-model.md) — trust boundaries,
  attacker model, fail-closed behavior, assumptions, non-goals.
- [`docs/evidence-report.md`](docs/evidence-report.md) — every claim in
  this README paired with the exact command and exact output that
  reproduces it.
- [`schemas/model.md`](schemas/model.md) — field-by-field input
  contract, all six schema versions.
- [`docs/phase-1-plan.md`](docs/phase-1-plan.md) …
  [`docs/phase-6-plan.md`](docs/phase-6-plan.md) — the authoritative,
  immutable design contract for each invariant.
- [`docs/v1-release-plan.md`](docs/v1-release-plan.md) — the release
  plan this productization pass implements.
- [`SECURITY.md`](SECURITY.md) — how to report a soundness or
  resource-exhaustion bug.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — PR process and non-negotiable
  invariants.

## Non-goals

Networking, hosted services, OAuth/identity-provider implementation, MCP/A2A
protocol implementation, LLM integration, runtime enforcement/proxying,
SAT/SMT solving, symbolic execution, SARIF output, CI vendor integration,
databases, a web UI, automatic policy generation, scope wildcards/hierarchy,
explicit per-edge delegation-budget attenuation, multi-approver
quorum/threshold requirements, approval-gated delegation,
self-approval/separation-of-duties enforcement, real-world
redelegation-count or approval-workflow correspondence, a general-purpose
model checker or policy language, cross-approval/global lifecycle
composition, and a real event log/session/clock concept are all explicitly
out of scope through Phase 6. Phase 6 adds one narrow, bounded exception to
the earlier "no temporal/session state" boundary: an individual approval
record may declare its own small, optional lifecycle automaton, explored
completely independently of every other approval record's — this is not
general session/temporal state, a workflow engine, or a real-time clock;
DelegationProof still cannot observe or reason about *when* an operation
executes relative to a declared transition. See each `docs/phase-*-plan.md`
for the full non-goals list at that phase, and
[`docs/v1-release-plan.md`](docs/v1-release-plan.md) §30 for this
productization capstone's own non-goals (no seventh invariant, no
networking, no hosted API, no web UI, no database, no runtime
enforcement).
