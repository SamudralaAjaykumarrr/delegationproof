# DelegationProof

DelegationProof is a small, dependency-free, offline, deterministic CLI
that checks a declared agent-delegation topology for **Authority
Non-Amplification**: does any node in the graph exercise or receive
authority it was never validly granted?

This is Phase 1 of a larger project. Phase 1 proves one formal invariant,
evaluated over a static graph, with a precise input contract, a precise
finding contract, and a test suite that locks down determinism and failure
behavior. See [`docs/phase-1-plan.md`](docs/phase-1-plan.md) for the full
design contract this implementation follows, including what is
deliberately **out of scope** for Phase 1 (§18) and how later invariants —
audience binding, approvals, depth limits, confused-deputy detection,
bounded state-space exploration — attach to this same foundation without
rewriting it (§21).

## What it checks

Given a graph of **principals** (root authority holders, e.g. a user) and
**agents** (participants whose authority is only ever *derived* from valid
incoming delegations), plus a set of declared **delegation** edges and
**operations** (points where an actor exercises one required scope),
DelegationProof computes each node's Derived Authority in one deterministic
topological pass and reports every place a delegation edge or operation
exceeds it.

An incoming delegation edge that over-claims relative to its delegator's
own derived authority is **fully distrusted** — it contributes nothing to
the delegatee's derived authority, not even the overlapping part. This
strict "no partial credit" semantics is what makes the invariant precise
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
  evaluates the invariant — useful as a fast pre-check or editor hook.
- `verify` runs validation, and if the model is structurally valid,
  evaluates Authority Non-Amplification and reports findings (or a clean
  pass).
- `--format` (default `text`) selects `text` (human-readable) or `json`
  (the machine-readable finding contract, one JSON object to stdout, no
  trailing prose).

Errors always go to stderr; result output always goes to stdout — safe to
pipe/parse stdout even when stderr carries diagnostic noise.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Clean pass — structurally valid model, and (for `verify`) zero findings. |
| `1` | Invariant violated — `verify` found one or more findings. This is a semantic DENY: the tool worked correctly. |
| `2` | Model/input problem — file not found, invalid JSON, any structural validation error, or a resource bound exceeded. Applies to both `validate` and `verify`. |
| `3` | CLI usage error — wrong argument count, unknown flag, unknown subcommand. Never about the model's content. |

## Worked example

[`examples/billing-refund.json`](examples/billing-refund.json) declares a
principal `user` holding `billing:read` and `billing:write`, who delegates
only `billing:read` down a two-hop chain (`user → agent-a → agent-b`).
`agent-b` then attempts two operations:

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
fails: that scope exists at the root principal but was never delegated down
the chain, so `agent-b`'s derived authority never included it. This is
Authority Non-Amplification caught in exactly the place it matters.

## Input model

See [`schemas/model.md`](schemas/model.md) for the full field-by-field
contract (a documentation mirror of `docs/phase-1-plan.md` §7 —
`internal/loader` is the sole runtime source of truth; no schema-validation
library is a dependency). In brief:

```json
{
  "version": "1",
  "principals": [{ "id": "user", "authority": ["billing:read", "billing:write"] }],
  "agents": [{ "id": "agent-a" }],
  "delegations": [{ "delegator": "user", "delegatee": "agent-a", "authority": ["billing:read"] }],
  "operations": [{ "actor": "agent-a", "action": "billing.view", "requires": "billing:read" }]
}
```

Authority is an opaque, exact-match scope string — no wildcards, no
hierarchy. Unknown fields anywhere are rejected. An agent may never declare
its own `authority`; it is always derived. The delegation graph must be a
DAG; principals cannot be delegation targets. All structural problems in
one input are collected and reported together, not fail-fast.

## Resource bounds

Fixed constants (`internal/limits`), exceeding any of which is a
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
| Max authority-set size (per principal or per edge) | 256 |
| Max delegation chain depth (longest simple path) | 64 |

## Determinism

Identical input always produces byte-identical output, and reordering the
`principals`/`agents`/`delegations`/`operations` arrays in a
semantically-equivalent model never changes the output. This holds because:

- Kahn's-algorithm topological sort breaks every tie by ascending
  lexicographic node id — never by map iteration order.
- Findings are sorted by a total order over their own content:
  `(point, subject_id, secondary_id_or_action, scope)`.
- Delegation traces are the first path a canonical, tie-broken BFS finds
  from the (sorted) set of principals.
- Authority sets are canonicalized (sorted, deduplicated) wherever they
  appear in output.

See `internal/report`, `internal/graph`, and `internal/verify` for where
each of these rules is implemented, and `cmd/delegationproof/main_test.go`
(`TestJSONFormatInputArrayPermutationInvariance`,
`TestJSONFormatDeterministicAcrossRepeatedRuns`) for the tests that lock
this down.

## Architecture

```
cmd/delegationproof/   CLI entry point: arg parsing, exit-code mapping, stdout/stderr split
internal/model/        Principal, Agent, Delegation, Operation, Model types
internal/limits/       Resource-bound constants (exported, so tests can lower them)
internal/loader/       JSON decode + full structural validation
internal/graph/        DAG topological sort, cycle detection, canonical BFS trace
internal/verify/       Derived Authority computation, edge/operation evaluation, findings
internal/report/       Finding types, deterministic sort order, text/json renderers
internal/exitcode/     The 4-value exit-code type
examples/               billing-refund.json (the worked example above)
schemas/                model.md — human-readable input contract
testdata/               valid/, malformed/ (one fixture per structural error kind), golden/
docs/                   phase-1-plan.md — the authoritative Phase 1 design contract
```

## Security assumptions

DelegationProof is a static, offline analyzer. It proves properties about a
*declared* model; it does not observe, enforce, or intercept real
agent/tool traffic, and it does not verify that a real running system
matches its declared model. A principal's declared authority is the
axiomatic root of trust — Phase 1 does not verify how a principal obtained
it (that is identity/OAuth territory, out of scope). Parsing is pure
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
kind in `docs/phase-1-plan.md` §7.4 (one fixture each under
`testdata/malformed/`), the strict-distrust ("no partial credit") semantics
of Derived Authority, deterministic finding ordering, byte-identical output
across semantically-equivalent reordered input, every resource bound
(exercised via lowered `internal/limits` values in white-box tests), CLI
exit codes and the stdout/stderr split, and no-panic fuzzing over
truncated/mutated input.

## Non-goals (Phase 1)

Networking, hosted services, OAuth/identity-provider implementation, MCP/A2A
protocol implementation, LLM integration, runtime enforcement/proxying,
SAT/SMT solving, SARIF output, CI vendor integration, databases, a web UI,
automatic policy generation, scope wildcards/hierarchy, audience/resource
binding, approvals, revocation, depth-limit policy, confused-deputy
detection, and bounded state-space exploration are all explicitly out of
scope for Phase 1. See `docs/phase-1-plan.md` §18 for the full list and
§21 for how each attaches to this foundation in a later phase.
