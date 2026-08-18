# DelegationProof

DelegationProof is a small, dependency-free, offline, deterministic CLI
that checks a declared agent-delegation topology against six composable
security invariants:

- **Authority Non-Amplification** (Phase 1): does any node in the graph
  exercise or receive authority it was never validly granted?
- **Context-Binding Preservation** (Phase 2): is a validly-granted
  capability being exercised or transmitted only for the target it was
  granted against?
- **Requester Authorization Preservation** (Phase 3): does the party an
  operation is actually performed *for* independently hold the capability
  being exercised, or is a legitimate actor being used as a confused
  deputy?
- **Delegation Depth Preservation** (Phase 4): has a capability already
  been re-delegated more hops from its origin than that origin's own
  declared budget permits?
- **Approval Preservation** (Phase 5): if a capability's origin declares
  that exercising it also requires a second party's explicit sign-off,
  does at least one declared, standing-backed approval exist for it?
- **Temporal Approval Preservation** (Phase 6): if a standing-backed
  approval declares its own lifecycle (e.g. it can be revoked, expire, or
  be resubmitted), can that approval ever be observed in a state other
  than approved — proved by bounded, deterministic reachability
  exploration over the approval's own small declared state automaton,
  never assumed from its mere existence?

Each invariant is a separate, additive input schema version (`"1"`
through `"6"`) rather than a single ever-growing format — see
[`docs/phase-1-plan.md`](docs/phase-1-plan.md),
[`docs/phase-2-plan.md`](docs/phase-2-plan.md),
[`docs/phase-3-plan.md`](docs/phase-3-plan.md),
[`docs/phase-4-plan.md`](docs/phase-4-plan.md),
[`docs/phase-5-plan.md`](docs/phase-5-plan.md), and
[`docs/phase-6-plan.md`](docs/phase-6-plan.md) for the full design
contract each phase follows, including what is deliberately **out of
scope** and how later phases attach to earlier ones without rewriting
them.

## What it checks

Given a graph of **principals** (root authority holders, e.g. a user) and
**agents** (participants whose authority is only ever *derived* from valid
incoming delegations), plus a set of declared **delegation** edges and
**operations** (points where an actor exercises a capability, optionally
on behalf of a **requester**), DelegationProof computes each node's
Derived Authority in one deterministic topological pass and reports every
place a delegation edge or operation violates one of the invariants above.

An incoming delegation edge that over-claims relative to its delegator's
own derived authority — in scope, in target, or (version 4) in remaining
re-delegation budget — is **fully distrusted**: it contributes nothing to
the delegatee's derived authority, not even the overlapping part. This
strict "no partial credit" semantics is what keeps every invariant precise
and cheap to compute (no backtracking, no search: `O(nodes + edges +
operations)`). Version 5 extends the same discipline to a fourth entity
kind — a declared approval record: one whose named approver lacks
independent standing over the capability it claims to approve contributes
nothing toward satisfying an approval requirement. Version 6 extends it
once more: a standing-backed approval record whose own declared lifecycle
can reach a state other than `"approved"` — proved by a small, bounded,
deterministic breadth-first search over that lifecycle's own declared
states and transitions, run independently per approval record — likewise
contributes nothing toward satisfying the requirement, and an exploration
that cannot complete within its bounded ceiling is treated identically:
never as an implicit pass.

## Build

```sh
go build -o bin/delegationproof ./cmd/delegationproof
```

No third-party dependencies; stdlib only.

## Usage

```
delegationproof validate <model.json>
delegationproof verify   <model.json> [--format text|json]
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

The input document's `"version"` field (`"1"` through `"6"`) selects which
schema and which set of invariants apply — there is no separate CLI flag
for this. Later versions are strict, additive extensions of earlier ones;
a version-1 document is checked only against Authority Non-Amplification,
a version-6 document against all six invariants.

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
declares a principal `user` holding `billing:read` and `billing:write`,
who delegates only `billing:read` down a two-hop chain (`user → agent-a →
agent-b`). `agent-b` then attempts two operations:

```sh
$ delegationproof verify examples/billing-refund.json
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

`billing.view` (requiring `billing:read`) passes silently — it's within
`agent-b`'s derived authority. `billing.refund` (requiring `billing:write`)
fails: that scope exists at the root principal but was never delegated
down the chain, so `agent-b`'s derived authority never included it.

[`examples/billing-context-binding.json`](examples/billing-context-binding.json)
(version 2) shows the same shape of violation one level more precise: a
scope *is* validly held, but only for a different target than the one
exercised — `context_binding_violation`, not amplification.

[`examples/billing-confused-deputy.json`](examples/billing-confused-deputy.json)
(version 3) shows an actor that legitimately holds the required capability
being induced to act on behalf of a requester who never independently held
it — `confused_deputy`.

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

See [`schemas/model.md`](schemas/model.md) for the full field-by-field
contract across all six schema versions (a documentation mirror of the
`docs/phase-*-plan.md` design contracts — `internal/loader` is the sole
runtime source of truth; no schema-validation library is a dependency). In
brief, the version-1 shape:

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

and the version-5 shape, adding approval preservation on top of version 4:

```json
{
  "version": "5",
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
    { "approver": "compliance-officer", "scope": "billing:refund", "target": "billing-service" }
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
hierarchy. From version 2 onward, authority is a `(scope, target)`
capability tuple rather than a bare scope. From version 3 onward, an
operation names a `requester` in addition to its `actor`. From version 4
onward, a *principal's own declared* capability additionally carries
`max_delegation_depth` — the field exists nowhere else (never on a
delegation edge, never on an operation); remaining budget is derived by
the verifier, never re-declared downstream. From version 5 onward, a
*principal's own declared* capability additionally carries a required
`requires_approval` boolean, and the document gains a new top-level
`approvals` array — each record naming an approver and the exact
`(scope, target)` it approves. An approval record is checked, never
traversed: it is not a graph node or edge, and gates a capability's
*exercise*, never its delegation. From version 6 onward, an individual
`approvals[]` record may additionally carry an optional `lifecycle` — a
small, possibly-cyclic named-state automaton (`initial`, `states`,
`transitions`) describing how that one approval's own standing can change
over time (e.g. revoked, expired, resubmitted). An approval record with no
`lifecycle` behaves exactly as it does under version 5 — eternally active
once declared and standing-backed. Unknown fields anywhere are rejected. An
agent may never declare its own `authority`; it is always derived. The
delegation graph must be a DAG; principals cannot be delegation targets. A
lifecycle automaton, by contrast, is never required to be acyclic — a
declared approval can legitimately be revoked and later resubmitted. All
structural problems in one input are collected and reported together,
not fail-fast.

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

`max_chain_depth` bounds the actual shape of the graph and
`max_delegation_depth` bounds only how large a *declared policy value* a
document may assert — they are deliberately independent, never conflated,
even though they currently share a default value. Likewise, `max lifecycle
states` is a validate-time bound on what a document may *declare*, while
`max exploration states per lifecycle` is an independent runtime bound on
the bounded-BFS engine's own execution — provably unreachable for any
validate-time-legal document (since the former already caps the state
count to the same value the latter allows), kept anyway as defense-in-depth
against an implementation bug. Exceeding the runtime bound never returns
`ALLOW`: it fails closed as an `approval_lifecycle_unproven` finding.

## Determinism

Identical input always produces byte-identical output, and reordering the
`principals`/`agents`/`delegations`/`operations` arrays (or the capability
arrays within them) in a semantically-equivalent model never changes the
output. This holds because:

- Kahn's-algorithm topological sort breaks every tie by ascending
  lexicographic node id — never by map iteration order.
- Findings are sorted by a total order over their own content:
  `(point, subject_id, secondary_id_or_action, scope, target, requester)`.
- Delegation traces are the first path a canonical, tie-broken BFS finds
  from the (sorted) set of principals, using only edges that were fully
  valid (presence, binding, and — for version 4 — depth).
- Authority sets are canonicalized (sorted, deduplicated) wherever they
  appear in output.
- Version 4's multi-path remaining-delegation-budget computation takes the
  componentwise maximum over all valid incoming edges per capability, with
  ties broken by the same ascending-lexicographic-delegator-id iteration
  order already used everywhere else — never a second sort.
- Version 5's multi-path `requires_approval` computation takes the
  logical OR over all valid incoming edges per capability — commutative,
  associative, and idempotent, so it needs no tie-break at all, computed
  independently of which path wins version 4's remaining-budget contest.
  A capability's set of standing-backed approvers is likewise the full,
  sorted, deduplicated set of matching `approvals[]` entries, not one
  arbitrarily chosen representative.
- Version 6's bounded lifecycle exploration (`internal/explore`) is a plain
  FIFO breadth-first search whose frontier order and per-state outgoing-
  transition order (ascending lexicographic `(to, event)`) are both fixed
  functions of the declared `(initial, transitions)` input — never of Go
  map iteration order. Each lifecycle-bearing approval record is explored
  completely independently of every other one (never composed into a
  cross-product global state), so total exploration cost is linear in the
  number of declared approvals. When more than one non-`"approved"` state
  is reachable, the canonical one reported is the lexicographically
  smallest; the one place this selection ranges a map, the result is
  immediately sorted before anything is read from it.

See `internal/report`, `internal/graph`, `internal/explore`, and
`internal/verify` for where
each of these rules is implemented, and each `cmd/delegationproof/main*_test.go`
file (`TestJSONFormatInputArrayPermutationInvariance*`,
`TestJSONFormatDeterministicAcrossRepeatedRuns*`) for the tests that lock
this down.

## Architecture

```
cmd/delegationproof/   CLI entry point: arg parsing, version dispatch, exit-code mapping, stdout/stderr split
internal/model/        Pure data types per schema version (types.go, types_v2.go ... types_v6.go)
internal/limits/       Resource-bound constants (exported, so tests can lower them)
internal/loader/       JSON decode + full structural validation, one file per schema version
internal/graph/        DAG topological sort, cycle detection, canonical BFS trace (shared by every version)
internal/explore/      Generic bounded, deterministic BFS reachability over a possibly-cyclic labeled digraph (version 6 only)
internal/verify/       Derived Authority computation, edge/operation evaluation, findings, one Run* per version
internal/report/       Finding types, deterministic sort order, text/json renderers
internal/exitcode/     The 4-value exit-code type
examples/               One worked example per phase
schemas/                model.md — human-readable input contract, one section per schema version
testdata/               valid*/, malformed/ (one fixture per structural error kind), golden/
docs/                   phase-1-plan.md ... phase-6-plan.md — the authoritative design contracts
```

`internal/explore` is a standalone package with zero dependency on
`model`, `report`, `loader`, or any DelegationProof-specific concept — it
operates purely on strings and a transition list, and is independently
unit-tested with no such scaffolding. It is deliberately not folded into
`internal/graph`: `graph.TopoSort` is strictly DAG-only (it exists
specifically to *reject* cycles as a structural error), while a version-6
lifecycle automaton is explicitly, legitimately allowed to contain them.

Each schema version's model types, loader, and verifier are structurally
disjoint from every other version's — a version-1 document can never be
accidentally interpreted under version-2+ semantics, and adding a new
phase never modifies an earlier phase's production code path (beyond a
single, explicitly sanctioned update to the `invalid_version` error
message text each time a new version literal is added).

## Security assumptions

DelegationProof is a static, offline analyzer. It proves properties about
a *declared* model; it does not observe, enforce, or intercept real
agent/tool traffic, and it does not verify that a real running system
matches its declared model. A principal's declared authority is the
axiomatic root of trust — the tool does not verify how a principal
obtained it (that is identity/OAuth territory, out of scope). A version-3+
`requester` is a declared label, not an authenticated identity. A
version-4 `max_delegation_depth` is a policy assertion by the document's
author, not a verified fact about a real system's actual re-delegation
history — DelegationProof proves that a document's declared model never
claims a capability travels farther than its own declared budget permits,
not that a real system enforces that budget at runtime. A version-5
`approvals[]` entry is likewise a declared fact by the document's author,
not a verified real-world sign-off event — DelegationProof verifies that a
named approver structurally *could* legitimately approve (by
independently holding the capability), not that the named party is who
they claim to be, or that a real compliance workflow actually produced
that sign-off. A version-6 `lifecycle` declaration is likewise a declared
fact by the document's author, not an observed real-world event log —
DelegationProof verifies that a declared automaton *cannot* reach an
unsafe state given its own declared transitions, not that those
transitions correspond to any real compliance system's actual behavior,
or that a real revocation event ever fires when the document claims it
can. DelegationProof also cannot observe or reason about *when*, in real
time, an operation executes relative to a lifecycle transition — this is
precisely why its safety predicate is universal ("every reachable state
must be `"approved"`") rather than an attempt to prove an operation
happens to run during an approved window, which is unknowable from a
static, offline document. Parsing is pure
stdlib data deserialization: no code execution, no dynamic loading, no
network access, no filesystem access beyond the one input file. Combined
with the resource bounds above, it is safe to run against untrusted model
files without additional sandboxing. It is not a server, not multi-tenant,
not persistent: one file in, one deterministic report out, process exits.

## Testing

```sh
go test ./... -race -count=1
```

Test categories include: clean-pass golden output, every structural error
kind in each phase's design contract (one fixture each under
`testdata/malformed/`), the strict-distrust ("no partial credit")
semantics of Derived Authority (extended in version 4 to a third failure
surface — remaining re-delegation budget, and in version 5 to a fourth —
non-standing approval records — without weakening the earlier ones),
deterministic finding ordering, byte-identical output across
semantically-equivalent reordered input, every resource bound (exercised
via lowered `internal/limits` values in white-box tests), CLI exit codes
and the stdout/stderr split, and no-panic fuzzing over truncated/mutated
input — for every schema version, independently. Version 6 additionally
covers: standalone `internal/explore` unit tests (cycles, self-loops,
branching, truncation, determinism) requiring no `model`/`loader` import;
bounded-search fail-closed behavior (`approval_lifecycle_unproven`,
exercised only via a lowered `limits.MaxExplorationStatesPerLifecycle`,
never via a validate-time-legal document at its normal bound, and
confirmed to never resolve to `ALLOW`); canonical unsafe-state/history
selection when multiple non-`"approved"` states or paths are reachable;
and full regression across every prior invariant, confirming a version-6
document with no `lifecycle` field anywhere produces output identical in
finding content to the equivalent version-5 document.

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
for the full non-goals list at that phase and how later phases may attach
to this foundation without rewriting it.
