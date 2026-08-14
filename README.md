# DelegationProof

DelegationProof is a small, dependency-free, offline, deterministic CLI
that checks a declared agent-delegation topology against four composable
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

Each invariant is a separate, additive input schema version (`"1"`
through `"4"`) rather than a single ever-growing format — see
[`docs/phase-1-plan.md`](docs/phase-1-plan.md),
[`docs/phase-2-plan.md`](docs/phase-2-plan.md),
[`docs/phase-3-plan.md`](docs/phase-3-plan.md), and
[`docs/phase-4-plan.md`](docs/phase-4-plan.md) for the full design
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
operations)`).

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

The input document's `"version"` field (`"1"`, `"2"`, `"3"`, or `"4"`)
selects which schema and which set of invariants apply — there is no
separate CLI flag for this. Later versions are strict, additive
extensions of earlier ones; a version-1 document is checked only against
Authority Non-Amplification, a version-4 document against all four
invariants.

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

## Input model

See [`schemas/model.md`](schemas/model.md) for the full field-by-field
contract across all four schema versions (a documentation mirror of the
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

Authority is an opaque, exact-match scope string — no wildcards, no
hierarchy. From version 2 onward, authority is a `(scope, target)`
capability tuple rather than a bare scope. From version 3 onward, an
operation names a `requester` in addition to its `actor`. From version 4
onward, a *principal's own declared* capability additionally carries
`max_delegation_depth` — the field exists nowhere else (never on a
delegation edge, never on an operation); remaining budget is derived by
the verifier, never re-declared downstream. Unknown fields anywhere are
rejected. An agent may never declare its own `authority`; it is always
derived. The delegation graph must be a DAG; principals cannot be
delegation targets. All structural problems in one input are collected
and reported together, not fail-fast.

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

`max_chain_depth` bounds the actual shape of the graph and
`max_delegation_depth` bounds only how large a *declared policy value* a
document may assert — they are deliberately independent, never conflated,
even though they currently share a default value.

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

See `internal/report`, `internal/graph`, and `internal/verify` for where
each of these rules is implemented, and each `cmd/delegationproof/main*_test.go`
file (`TestJSONFormatInputArrayPermutationInvariance*`,
`TestJSONFormatDeterministicAcrossRepeatedRuns*`) for the tests that lock
this down.

## Architecture

```
cmd/delegationproof/   CLI entry point: arg parsing, version dispatch, exit-code mapping, stdout/stderr split
internal/model/        Pure data types per schema version (types.go, types_v2.go, types_v3.go, types_v4.go)
internal/limits/       Resource-bound constants (exported, so tests can lower them)
internal/loader/       JSON decode + full structural validation, one file per schema version
internal/graph/        DAG topological sort, cycle detection, canonical BFS trace (shared by every version)
internal/verify/       Derived Authority computation, edge/operation evaluation, findings, one Run* per version
internal/report/       Finding types, deterministic sort order, text/json renderers
internal/exitcode/     The 4-value exit-code type
examples/               One worked example per phase
schemas/                model.md — human-readable input contract, one section per schema version
testdata/               valid*/, malformed/ (one fixture per structural error kind), golden/
docs/                   phase-1-plan.md ... phase-4-plan.md — the authoritative design contracts
```

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
not that a real system enforces that budget at runtime. Parsing is pure
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
surface — remaining re-delegation budget — without weakening the original
two), deterministic finding ordering, byte-identical output across
semantically-equivalent reordered input, every resource bound (exercised
via lowered `internal/limits` values in white-box tests), CLI exit codes
and the stdout/stderr split, and no-panic fuzzing over truncated/mutated
input — for every schema version, independently.

## Non-goals

Networking, hosted services, OAuth/identity-provider implementation, MCP/A2A
protocol implementation, LLM integration, runtime enforcement/proxying,
SAT/SMT solving, SARIF output, CI vendor integration, databases, a web UI,
automatic policy generation, scope wildcards/hierarchy, approvals,
revocation, temporal/session state, explicit per-edge delegation-budget
attenuation, and real-world redelegation-count correspondence are all
explicitly out of scope through Phase 4. See each `docs/phase-*-plan.md`
for the full non-goals list at that phase and how later phases may attach
to this foundation without rewriting it.
