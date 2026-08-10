# DelegationProof Phase 1 — model schema

This document is a human-readable mirror of the input contract defined in
`docs/phase-1-plan.md` §7. It is documentation only: `internal/loader` is the
sole runtime source of truth for what makes a model valid. Nothing here is
consulted by the program at runtime, and no schema-validation library is a
dependency of this project.

## Top-level document

```json
{
  "version": "1",
  "principals": [ { "id": "...", "authority": ["..."] } ],
  "agents": [ { "id": "..." } ],
  "delegations": [ { "delegator": "...", "delegatee": "...", "authority": ["..."] } ],
  "operations": [ { "actor": "...", "action": "...", "requires": "..." } ]
}
```

Unknown fields anywhere in the document (top-level or nested) are rejected.

## Field rules

| Field | Rule |
|---|---|
| `version` | Required. Must equal the literal string `"1"`. |
| `principals[].id` | Required, unique across the combined principal+agent id namespace. Matches `^[A-Za-z0-9_.-]{1,128}$`. |
| `principals[].authority` | Array of scope strings. May be empty. No duplicate entries. |
| `agents[].id` | Same rules as `principals[].id`. |
| `agents[]` | Must **not** contain an `authority` key — an agent's authority is always derived, never declared. |
| `delegations[].delegator` | Required. Must reference a known node id. |
| `delegations[].delegatee` | Required. Must reference a known node id that is **not** a principal. Must differ from `delegator`. |
| `delegations[].authority` | Required, non-empty array of scope strings. No duplicate entries. At most one delegation edge per `(delegator, delegatee)` pair. |
| `operations[].actor` | Required. Must reference a known node id (principal or agent). |
| `operations[].action` | Required, non-empty. Matches `^[A-Za-z0-9_.-]{1,128}$`. Opaque label, not interpreted. |
| `operations[].requires` | Required, exactly one scope string. |

## Scope strings

A scope is an opaque string matching `^[A-Za-z0-9_.:-]{1,256}$`, compared by
exact byte equality only. There is no wildcard, hierarchy, or namespace
semantics — `billing:write` does not imply `billing:read`.

## The graph

- Nodes = principals ∪ agents (disjoint, unified id namespace).
- Edges = delegations.
- The graph must be a DAG: any cycle (including a self-loop) is a
  structural validation error.
- Principals have in-degree 0 (cannot be a delegation target).

## Resource bounds

See `internal/limits`. Exceeding any bound is reported as a
`resource_limit_exceeded` validation error (exit code 2), never a panic,
never an unbounded allocation, never a hang.

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

## What makes a model malformed

Every case below is a structural validation error, exit code 2, detected by
both `validate` and `verify`:

- Invalid JSON syntax.
- Missing/invalid `version`.
- Unknown field anywhere in the document.
- Missing required field on any entity.
- An id or scope string failing its regex.
- A duplicate node id (across principals+agents).
- A duplicate `(delegator, delegatee)` pair.
- A delegation referencing an unknown `delegator`/`delegatee`.
- A delegation targeting a principal (`delegatee` resolves to a principal).
- Self-delegation (`delegator == delegatee`).
- An empty `authority` array on a delegation edge.
- A duplicate scope string within one authority array.
- An operation referencing an unknown `actor`.
- A cycle anywhere in the delegation graph.
- Any resource bound exceeded.

All structural errors found in one run are collected and reported together,
not fail-fast — except for problems that prevent building a parseable
document at all (JSON syntax errors, unknown-field decode errors), which are
singular by nature.

## The invariant

See `docs/phase-1-plan.md` §6, §8, §9 for the Authority Non-Amplification
invariant, the Derived Authority algorithm, and the finding contract. Summary:
a delegation edge or operation is a violation if it exercises or transmits a
scope that is not part of the actor's derived authority — the union of
scopes from *valid* incoming delegation edges, computed in one topological
pass over the DAG. An invalid incoming edge contributes nothing to its
target's derived authority, not even the overlapping part (strict distrust).
