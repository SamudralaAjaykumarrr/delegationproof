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

---

# Version 2 — capability (scope, target) model

This section documents the input contract for `"version": "2"` documents
(`docs/phase-2-plan.md` §3–§10). `internal/loader`'s `validateV2` is the sole
runtime source of truth; this is documentation only. Version-1 documents are
entirely unaffected by anything in this section — see the version dispatch
rule below.

## What's new

Phase 2 adds exactly one new atomic concept: **target**, an opaque,
exact-match label for the destination context a scope is valid against
(e.g. `"billing-service"`). Authority is no longer a bare scope string; it
is a **capability**, the ordered pair `(scope, target)`. Everything else
about the Phase 1 model — the graph shape, strict distrust, deterministic
ordering, resource bounds — carries over unchanged, generalized from "scope"
to "capability tuple."

There is no target hierarchy, no wildcard target, no target registry, and no
new graph entity. A target is a label carried on a capability tuple, nothing
more (`docs/phase-2-plan.md` §5).

## Top-level document

```json
{
  "version": "2",
  "principals": [ { "id": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "agents": [ { "id": "..." } ],
  "delegations": [ { "delegator": "...", "delegatee": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "operations": [ { "actor": "...", "action": "...", "requires": "...", "target": "..." } ]
}
```

Unknown fields anywhere in the document (top-level or nested) are rejected,
exactly as in version 1.

## Version dispatch

The loader decodes only `{"version": string}` first, permissively, before
committing to a struct shape. `"1"` routes to the existing, untouched
version-1 decode+validate path. `"2"` routes to the version-2 path described
here. Any other value (including absent, which decodes as `""`) is a single
`invalid_version` error: `version must be "1" or "2", got %q`. The two
schemas share no internal model type — a version-1 document can never be
accidentally interpreted with version-2 semantics or vice versa.

## Field rules

| Field | Rule |
|---|---|
| `version` | Required. Must equal `"1"` or `"2"`. |
| `principals[].id` | Same rules as version 1. |
| `principals[].authority` | Array of capability objects `{"scope": "...", "target": "..."}`. May be empty. No duplicate `(scope, target)` tuples — two entries sharing a scope but differing in target are *not* duplicates. |
| `agents[].id` | Same rules as version 1. |
| `agents[]` | Must **not** contain an `authority` key, same as version 1. |
| `delegations[].delegator` / `.delegatee` | Same rules as version 1. |
| `delegations[].authority` | Required, non-empty array of capability objects. No duplicate `(scope, target)` tuples. At most one delegation edge per `(delegator, delegatee)` pair. |
| `operations[].actor` / `.action` | Same rules as version 1. |
| `operations[].requires` | Required, exactly one scope string (unchanged singular-requires design). |
| `operations[].target` | Required. Together with `requires`, forms the one capability the operation exercises. |

## Capability tuples

A capability is `{"scope": "...", "target": "..."}`:

- `scope` uses the unchanged version-1 grammar: `^[A-Za-z0-9_.:-]{1,256}$`,
  exact byte equality, no hierarchy.
- `target` is a new grammar, matching node-id style rather than scope style:
  `^[A-Za-z0-9_.-]{1,128}$`. A missing `target` decodes as `""`, which fails
  this regex and is reported as `invalid_target` — the same mechanism used
  for missing/empty ids and scopes.
- The pair is compared by **exact tuple equality only**. `billing:read` for
  `billing-service` and `billing:read` for `payroll-service` are different,
  unrelated capabilities with no implied relationship.
- For display (text output), a capability renders as `scope@target`. `@`
  appears in neither grammar, so this round-trips unambiguously.

Targets, node ids, and scopes are three independent namespaces that happen
to share similar grammars. A target string may coincidentally equal an
existing node id or scope string; this has no special meaning. There is no
target registry — a target string is valid input as long as it matches the
grammar; it needs no prior declaration anywhere in the document.

## New structural error kinds

| Kind | When |
|---|---|
| `invalid_target` | A `target` fails `^[A-Za-z0-9_.-]{1,128}$` (including a missing/empty target). |
| `duplicate_capability` | Two entries with the exact same `(scope, target)` pair within one principal's or one edge's `authority` array. |

All version-1 structural error kinds apply unchanged, generalized where the
shape changed (e.g. "empty authority array" now applies to an array of
capability objects).

Explicitly **not** a structural error: an "unknown" target (no registry to
be unknown against), or a delegation/operation whose target doesn't match
what was actually delegated — that is the semantic finding this phase
exists to detect (see "The invariants," below), not a `validate`-time
(exit 2) problem.

## Resource bounds

All version-1 bounds (`internal/limits`) apply unchanged, with "authority
set" read as "capability set" — `MaxAuthoritySetSize` bounds the number of
capability *tuples* per principal/edge, not bare scopes. One new bound:

| Limit | Value | Notes |
|---|---|---|
| `MaxTargetLength` | 128 bytes | Mirrors `MaxIDLength`; applies to the `target` half of each capability tuple. |

## The invariants

A version-2 document is checked against **both** invariants, via one
generalized algorithm (capability tuples in place of bare scopes) plus one
classification step:

- **Authority Non-Amplification** (unchanged from version 1, generalized):
  a capability is amplified if its *scope* was never validly held under any
  target.
- **Context-Binding Preservation** (new): a capability may only be
  exercised or transmitted for the target it was delegated with. If a
  node's scope *is* held, but only under a different target than the one
  attempted, that is a `context_binding_violation`, not amplification.

Classification, for a missing capability `(s, t)`:

```
heldTargetsForScope = { t' : (s, t') is held }
heldTargetsForScope == ∅  =>  authority_amplification
otherwise                 =>  context_binding_violation
```

For an edge-level finding (which can cover multiple missing capabilities at
once), if *any* missing capability's scope was never held under any target,
the whole finding is `authority_amplification` — the more foundational
problem takes precedence and is never masked by a co-occurring binding
issue. Only when every missing capability is a pure context mismatch is the
finding `context_binding_violation`.

Strict distrust is unchanged in spirit: an invalid incoming edge
contributes **nothing** to the delegatee's derived authority — not the
tuples that were individually valid, not even tuples sharing a scope with
something the delegator holds under a different target.

See `docs/phase-2-plan.md` §7, §8, §11 for the full formal statement and
algorithm.

---

# Version 3 — requester (confused-deputy) model

This section documents the input contract for `"version": "3"` documents
(`docs/phase-3-plan.md` §3–§15). `internal/loader`'s `validateV3` is the sole
runtime source of truth; this is documentation only. Version-1 and
version-2 documents are entirely unaffected by anything in this section —
see the version dispatch rule below.

## What's new

Phase 3 adds exactly one new atomic concept: **requester**, a reference to
an existing principal or agent id — the party an operation is actually
performed *for*, as opposed to `actor`, the node that performs it. There is
no new node kind, no new edge kind, no invocation/call graph, and no
multi-hop request chain: each operation names exactly one requester.
Principals, agents, delegations, and capability tuples are byte-for-byte
identical to version 2.

Declaring a requester does not grant it anything and does not change any
node's Derived Authority — it is checked, never propagated. A requester's
own authority is computed by the same unmodified algorithm every node's is.

## Top-level document

```json
{
  "version": "3",
  "principals": [ { "id": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "agents": [ { "id": "..." } ],
  "delegations": [ { "delegator": "...", "delegatee": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "operations": [
    { "actor": "...", "requester": "...", "action": "...", "requires": "...", "target": "..." }
  ]
}
```

Unknown fields anywhere in the document (top-level or nested) are rejected,
exactly as in versions 1 and 2.

## Version dispatch

`"3"` routes to the version-3 path described here. `"1"` and `"2"` are
unaffected. Any other value (including absent, which decodes as `""`) is a
single `invalid_version` error: `` version must be "1", "2", or "3", got %q ``.
The three schemas share no internal model type.

## Field rules

| Field | Rule |
|---|---|
| `version` | Required. Must equal `"1"`, `"2"`, or `"3"`. |
| `principals[]` / `agents[]` / `delegations[]` | Same rules as version 2. |
| `operations[].actor` / `.action` / `.requires` / `.target` | Same rules as version 2. |
| `operations[].requester` | Required. Must reference a known principal or agent id — the same id namespace `actor` draws from. No default, no implicit self-reference: an actor acting on its own behalf must write `"requester"` equal to its own id explicitly. |

`requester == actor` is legal and is the trivial-pass case: both resolve to
the same Derived Authority lookup, so there is no special-cased code path.

## New structural error kind

| Kind | When |
|---|---|
| `unknown_requester` | `requester` does not resolve to a known principal or agent id — mirrors `unknown_actor` exactly. A missing `requester` (decodes as `""`) or a syntactically-malformed one both fall into this same kind; there is no separate "missing field" or "invalid format" kind, identical precedent to how `actor` is handled. |

All version-1/2 structural error kinds apply unchanged.

Explicitly **not** a structural error: a requester that structurally
resolves to a known node but lacks standing to authorize the operation —
that is the semantic finding this phase exists to detect (see "The
invariant," below), never a `validate`-time (exit 2) problem. Two
operations sharing `actor`/`action`/`requires`/`target` and differing only
by `requester` are not a duplicate — a real, legitimate case.

## Resource bounds

No new bound. `requester` is a reference to an existing node id, validated
by the same mechanism (and the same `MaxIDLength`) `actor` already uses. All
version-1/2 bounds apply unchanged.

## The invariant: Requester Authorization Preservation

A version-3 document is checked against Non-Amplification and
Context-Binding Preservation exactly as version 2 is, plus one new
invariant, evaluated only once the actor-side check has already passed:

> For every operation `(actor, requester, action, requires_scope,
> requires_target)`, let `C = (requires_scope, requires_target)`. If `C ∈
> DA(actor)`, then it must also hold that `C ∈ DA(requester)`. If not, the
> operation is a `confused_deputy` finding: a validly-authorized actor is
> being induced to exercise a capability the requester was never
> independently granted.

`DA(requester)` is computed exactly like `DA(actor)` — no ancestor
relationship is required between them; a requester's standing may come from
anywhere in the graph, independent of the actor's own delegation chain.

**Precedence (deterministic, one finding per operation):**

```
if C not in DA(actor):
    classify per version 2's rule -> authority_amplification | context_binding_violation
    (requester is NOT evaluated)
elif C in DA(requester):
    ALLOW, no finding
else:
    confused_deputy
```

An actor-side amplification or binding failure is strictly more
foundational and is never masked by, or reported alongside, a
requester-side standing failure for the same operation.

`confused_deputy` is a single violation literal regardless of *why* the
requester lacks standing (never held the scope at all, vs. held it only for
a different target) — that finer distinction is carried in `reason` text
and `requester_bound_targets`, not in the violation literal, so existing
literals never get overloaded with a second meaning.

Strict distrust is unchanged and requires no new code: an invalid incoming
edge on the path to a requester contributes nothing to `DA(requester)`,
exactly as it already contributes nothing to `DA(actor)`.

See `docs/phase-3-plan.md` §7, §8, §11, §12 for the full formal statement,
precedence algorithm, and classification rule.

---

# Version 4 — delegation-depth (re-delegation budget) model

This section documents the input contract for `"version": "4"` documents
(`docs/phase-4-plan.md` §3–§17). `internal/loader`'s `validateV4` is the
sole runtime source of truth; this is documentation only. Version-1,
version-2, and version-3 documents are entirely unaffected by anything in
this section — see the version dispatch rule below.

## What's new

Phase 4 adds exactly one new atomic concept: **`max_delegation_depth`**, an
integer re-delegation budget attached to a capability *only at the point it
is declared by a principal* — never on a delegation edge, never on an
operation. It answers a fourth, independent question none of the first
three invariants can express: not "is this authority real," but "has it
already traveled farther from its origin than that origin ever permitted."

Agents, delegations, and operations are byte-for-byte identical in shape to
their version-3 counterparts. Delegation edges continue to use the plain,
unmodified `{"scope": "...", "target": "..."}` capability tuple — a
delegatee's remaining budget is always *derived* by the verifier from the
root declaration and the number of hops actually traversed, never
re-declared or attenuated by the document itself.

## Top-level document

```json
{
  "version": "4",
  "principals": [
    { "id": "...", "authority": [
      { "scope": "...", "target": "...", "max_delegation_depth": 1 }
    ] }
  ],
  "agents": [ { "id": "..." } ],
  "delegations": [ { "delegator": "...", "delegatee": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "operations": [
    { "actor": "...", "requester": "...", "action": "...", "requires": "...", "target": "..." }
  ]
}
```

Unknown fields anywhere in the document (top-level or nested) are
rejected, exactly as in versions 1-3. In particular, a stray
`max_delegation_depth` key on a delegation's authority entry or on an
operation is rejected at decode time (no such field exists on either
type) — the identical "enforced for free by the schema shape" mechanism
Phase 1 already uses for `Agent.authority`.

## Version dispatch

`"4"` routes to the version-4 path described here. `"1"`, `"2"`, and `"3"`
are unaffected. Any other value (including absent, which decodes as `""`)
is a single `invalid_version` error: `` version must be "1", "2", "3", or "4", got %q ``.
The four schemas share no internal model type.

## Field rules

| Field | Rule |
|---|---|
| `version` | Required. Must equal `"1"`, `"2"`, `"3"`, or `"4"`. |
| `principals[].id` | Same rules as version 3. |
| `principals[].authority` | Array of *root* capability objects `{"scope": "...", "target": "...", "max_delegation_depth": N}`. May be empty. No duplicate `(scope, target)` tuples — regardless of whether their `max_delegation_depth` values agree (two entries sharing a tuple with different depths would be genuinely ambiguous, so both are rejected, never implicitly merged). |
| `principals[].authority[].max_delegation_depth` | Required on every entry. An integer, `0 ≤ value ≤ limits.MaxDelegationDepth`. No default, no "unbounded" sentinel — a document wanting an effectively-unconstrained capability declares a large, explicit, in-bounds value. |
| `agents[].id` | Same rules as version 3. |
| `agents[]` | Must **not** contain an `authority` key, same as versions 1-3. |
| `delegations[].delegator` / `.delegatee` / `.authority` | Same rules as version 3 — the plain `{"scope", "target"}` tuple, no `max_delegation_depth` field exists here at all. |
| `operations[].actor` / `.requester` / `.action` / `.requires` / `.target` | Same rules as version 3. |

## `max_delegation_depth` semantics

- **Not part of a capability's identity.** `(scope, target)` remains the
  sole identity of a capability, exactly as version 2 established.
  `max_delegation_depth` is metadata attached to a capability's *origin
  declaration*, checked as an additional, independent dimension once
  presence/binding are already established — never folded into the
  presence/binding subset check itself.
- **The declared value is the root's own remaining budget**, not a
  hop-count-from-root. A capability declared with `max_delegation_depth: 0`
  is usable by the declaring principal itself but may not be delegated
  even one hop further. A capability declared with `max_delegation_depth:
  1` permits exactly one outgoing delegation; the delegatee receives it at
  remaining budget 0 (still usable, no further redelegation).
- **Only re-delegation (an outgoing delegation edge) consumes budget.**
  Exercising a capability — as an operation's actor or as its requester —
  never consumes or is gated by remaining budget; using authority and
  transmitting authority are different acts, and only the latter is
  metered. A node may hold a capability at remaining budget 0 and still
  legitimately use it in an operation, or legitimately be named as a
  requester for it.
- **Multiple valid delivering paths: the best (maximum) remaining budget
  wins.** If a capability reaches a node via more than one independently
  valid delegation path, each path's resulting remaining-budget figure is
  an independently true fact about that node; the node's further-delegation
  eligibility is governed by the largest such figure among them — not an
  average, not the smallest, not the first path found. Ties (equal
  maximum remaining budget delivered by more than one path) are broken
  deterministically by the delegator id's ascending lexicographic order,
  reusing the same iteration order every prior phase already establishes,
  with no new sorting logic.

## New structural error kind

| Kind | When |
|---|---|
| `invalid_delegation_depth` | A `RootCapability`'s `max_delegation_depth` is either absent (decodes as a `nil` pointer — distinct from a present, explicit `0`, which is a legal, meaningful value) or present but negative. Both share this one kind, the same "one kind covers missing and malformed" precedent `unknown_requester` already establishes. |

All version-1/2/3 structural error kinds apply unchanged. `duplicate_capability` is reused, unmodified, projected onto `(scope, target)` only (depth is deliberately excluded from the uniqueness key, per the field-rules table above).

Explicitly **not** a structural error:

- A non-integer JSON value (`1.5`, `"1"`) for `max_delegation_depth` — this
  is a JSON decode-level type mismatch against the `*int` field, surfaced
  as the existing `invalid JSON: ...` `ParseError` path, identical in kind
  to any other field-type mismatch already handled by strict typed
  decoding.
- A document that declares a delegation chain exceeding a capability's
  declared budget — that is the `delegation_depth_violation` semantic
  finding this phase exists to detect (see "The invariant," below), a
  `verify`-time (exit 1) result, never a `validate`-time (exit 2) error.

## Resource bounds

All version-1/2/3 bounds (`internal/limits`) apply unchanged. One new
bound:

| Limit | Value | Notes |
|---|---|---|
| `MaxDelegationDepth` | 64 | Bounds the **declared** `max_delegation_depth` value a document may assert on any root capability. A resource-safety valve on the declared *value*, kept as an independent `var` from `MaxChainDepth` (the resource-safety valve on the graph's actual *shape*) — the two are never conflated, even though they currently share a default. |

## The invariant: Delegation Depth Preservation

A version-4 document is checked against Non-Amplification, Context-Binding
Preservation, and Requester Authorization Preservation exactly as version
3 is, plus one new invariant, evaluated at the delegation-edge level:

> For every capability `c = (s, t)` declared by a root principal with
> budget `b = c.max_delegation_depth`, and for every node `n` reachable
> from that root via a chain of valid delegation edges each carrying `c`,
> `n`'s *usable* possession of `c` is legitimate regardless of chain
> length — but `n`'s further transmission of `c` via an outgoing
> delegation edge is legitimate only if the number of edges already
> traversed from the root to `n` along the best available valid path is
> strictly less than `b`. A delegation edge that would carry `c` beyond
> that budget contributes nothing to its delegatee's derived authority for
> `c` — not partial credit, not the capability at a clamped depth,
> nothing.

**Three-tier edge-level precedence, exactly one finding per invalid edge:**

```
if any capability in the edge's declared set was never held by the
delegator under any target:
    authority_amplification            (unchanged from version 2)
elif any capability was held only under a different target:
    context_binding_violation          (unchanged from version 2)
else:
    # every capability is present, correctly bound; reaching here means
    # at least one has the delegator's remaining budget exhausted (0)
    delegation_depth_violation         (new, version 4)
```

Whole-edge poisoning is preserved for depth failures too: if a single
delegation edge carries multiple capabilities and any of them is
depth-exhausted at the delegator, the entire edge is invalid — including
capabilities in the same edge that individually had ample remaining
budget. This is the same strict-distrust rule Phase 1 already establishes
(`TestStrictDistrustNoPartialCredit`), applied a third way.

`delegation_depth_violation` is **always** an edge-level finding
(`point: "delegation_edge"`), never an operation-level one: delegation
depth gates transmission, not use. A depth-exhausted edge instead shows up
downstream, if at all, as an ordinary `authority_amplification` finding at
the operation level — the *cause* (the edge finding) and the *consequence*
(the operation finding) are both legitimately emitted, at their own
points, with no masking between them.

The `delegation_depth_violation` finding carries `declared` (the edge's
whole declared capability set), `excess` (the depth-exhausted subset, each
paired with its own `configured_max_depth`/`remaining_depth` — a capability
is not part of its own identity's depth, so two capabilities in the same
poisoned edge can legitimately have different configured budgets), `trace`
(the same `CanonicalTrace` convention every prior finding type already
uses), and a deterministic `reason` string.

Strict distrust is unchanged in spirit and requires no new code for the
first two tiers: an invalid incoming edge contributes nothing to
`DA(n)`, exactly as before. Requester interaction is also unaffected:
naming a node as a requester creates no delegation edge, consumes no
budget, and is checked via the same presence-only view every prior phase
already uses — a requester whose only apparent standing for a capability
arrived via a now-depth-exhausted path simply never had that capability in
its derived authority at all (an ordinary Phase 1-3 case), unaffected by
anything Phase 4 adds.

See `docs/phase-4-plan.md` §3, §4, §8, §9, §10, §11, §12, §13, §16 for the
full formal statement, multi-path semantics, and verification algorithm.

---

# Version 5 — approval preservation model

This section documents the input contract for `"version": "5"` documents
(`docs/phase-5-plan.md` §3–§17). `internal/loader`'s `validateV5` is the
sole runtime source of truth; this is documentation only. Version-1,
version-2, version-3, and version-4 documents are entirely unaffected by
anything in this section — see the version dispatch rule below.

## What's new

Phase 5 adds exactly two new atomic concepts: **`requires_approval`**, a
required boolean attached to a capability *only at the point it is declared
by a principal* (mirroring `max_delegation_depth`'s placement exactly), and
**`approvals`**, a new top-level array of declared approval records, each
naming an approver and the exact capability `(scope, target)` it approves.
An approval record is checked, never traversed: it is not a graph node, not
an edge, and does not participate in `graph.TopoSort` or
`graph.CanonicalTrace`.

Agents, delegations, and operations are byte-for-byte identical in shape to
their version-4 counterparts. Delegation edges continue to use the plain,
unmodified `{"scope": "...", "target": "..."}` capability tuple —
`requires_approval` is never re-declared or re-asserted at a delegation
edge, and approval gates *exercise*, not transmission (an approval-required
capability may still be freely delegated, subject to the unchanged
presence/binding/depth rules).

## Top-level document

```json
{
  "version": "5",
  "principals": [
    { "id": "...", "authority": [
      { "scope": "...", "target": "...", "max_delegation_depth": 1, "requires_approval": true }
    ] }
  ],
  "agents": [ { "id": "..." } ],
  "delegations": [ { "delegator": "...", "delegatee": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "approvals": [
    { "approver": "...", "scope": "...", "target": "..." }
  ],
  "operations": [
    { "actor": "...", "requester": "...", "action": "...", "requires": "...", "target": "..." }
  ]
}
```

Unknown fields anywhere in the document (top-level or nested) are
rejected, exactly as in versions 1-4. In particular, a stray
`requires_approval` key on a delegation's authority entry or on an
operation is rejected at decode time (no such field exists on either
type).

## Version dispatch

`"5"` routes to the version-5 path described here. `"1"`, `"2"`, `"3"`,
and `"4"` are unaffected. Any other value (including absent, which decodes
as `""`) is a single `invalid_version` error:
`` version must be "1", "2", "3", "4", or "5", got %q ``. The five schemas
share no internal model type.

## Field rules

| Field | Rule |
|---|---|
| `version` | Required. Must equal `"1"`, `"2"`, `"3"`, `"4"`, or `"5"`. |
| `principals[].id` | Same rules as version 4. |
| `principals[].authority` | Array of *root* capability objects `{"scope": "...", "target": "...", "max_delegation_depth": N, "requires_approval": B}`. May be empty. No duplicate `(scope, target)` tuples — regardless of whether their `max_delegation_depth` or `requires_approval` values agree. |
| `principals[].authority[].max_delegation_depth` | Same rule as version 4. |
| `principals[].authority[].requires_approval` | Required on every entry. A boolean, no default. `false` is itself a legitimate, meaningful, commonly-declared value — a document author who forgets the field entirely is rejected, never silently interpreted as "no approval required." |
| `agents[].id` | Same rules as version 4. |
| `agents[]` | Must **not** contain an `authority` key, same as versions 1-4. |
| `delegations[].delegator` / `.delegatee` / `.authority` | Same rules as version 4 — the plain `{"scope", "target"}` tuple, no `requires_approval` field exists here at all. |
| `operations[].actor` / `.requester` / `.action` / `.requires` / `.target` | Same rules as version 4. |
| `approvals[].approver` | Required. Must reference a known principal or agent id — the same id namespace `actor`/`requester` draw from. |
| `approvals[].scope` / `.target` | Required. Use the unchanged Phase 2 capability grammar. An approval naming a `(scope, target)` no principal ever declared is not a structural error — it is simply inert. |

## `requires_approval` semantics

- **Not part of a capability's identity.** `(scope, target)` remains the
  sole identity of a capability, exactly as version 2 established.
  `requires_approval` is metadata attached to a capability's *origin
  declaration*, checked as an additional, independent dimension once
  presence/binding/requester-standing are already established.
- **Gates exercise, not delegation.** A delegation edge carrying an
  approval-required capability is evaluated by the unchanged Phase 1/2/4
  edge-level rules exactly as if `requires_approval` did not exist — there
  is no new edge-level structural rule and no new edge-level finding kind
  in Phase 5 at all.
- **Multiple valid delivering paths: logical OR, independent of the
  `remaining`-maximization contest.** If a capability reaches a node via
  more than one independently valid delegation path, and the paths
  disagree on `requires_approval`, the node's adopted value is `true` if
  **any** valid delivering path declares `true` — computed independently
  of which path wins the `remaining`/`configuredMax` contest (§10.1's
  fail-closed rule; deliberately the opposite polarity from
  `remaining`'s "adopt the more permissive of two true facts," because
  here the fail-closed choice is the *stricter* one).

## Approval record semantics

- **Capability-scoped, not operation-scoped and not actor/requester-scoped.**
  An approval names exactly the `(scope, target)` pair it covers, and
  covers *every* operation that exercises that capability.
- **Multiple approval records may name the same `(scope, target)` with
  different approvers** — legal and expected. The only rejected
  duplication is an exact repeated `(approver, scope, target)` triple.
- **Whether the named approver actually holds standing is a semantic
  question**, checked at `verify` time, never a `validate`-time structural
  requirement.
- **Self-approval is not structurally prohibited.** An approval record may
  legally name the same node as the operation's `actor` or `requester`.

## New structural error kinds

| Kind | When |
|---|---|
| `missing_approval_requirement` | A `RootCapabilityV5`'s `requires_approval` is `nil` (the key was omitted). |
| `unknown_approver` | `approvals[].approver` does not resolve to a known principal or agent id — mirrors `unknown_requester`/`unknown_actor` precisely. A missing `approver` (decodes as `""`) or a syntactically-malformed one both fall into this same kind. |
| `duplicate_approval` | Two entries within `approvals[]` share the exact same `(approver, scope, target)` triple. Two entries sharing only `scope`/`target` but naming *different* approvers are not a duplicate. |

All version-1/2/3/4 structural error kinds apply unchanged.
`duplicate_capability` is reused, unmodified, projected onto
`(scope, target)` only (neither `max_delegation_depth` nor
`requires_approval` is part of the uniqueness key).

Explicitly **not** a structural error:

- A non-boolean JSON value (`"true"`, `1`) for `requires_approval` — this
  is a JSON decode-level type mismatch against the `*bool` field, surfaced
  as the existing `invalid JSON: ...` `ParseError` path.
- An approval record referencing a `(scope, target)` no principal ever
  declared, or naming an approver who lacks standing over the capability
  it claims to approve — both are `verify`-time (exit 1) findings, never
  `validate`-time (exit 2) errors.
- Self-approval.

## Resource bounds

All version-1/2/3/4 bounds (`internal/limits`) apply unchanged. One new
bound:

| Limit | Value | Notes |
|---|---|---|
| `MaxApprovals` | 10000 | Bounds the number of entries in the top-level `approvals` array — a new, independent top-level array, not nested inside any existing bounded collection. |

## The invariant: Approval Preservation

A version-5 document is checked against Non-Amplification, Context-Binding
Preservation, Requester Authorization Preservation, and Delegation Depth
Preservation exactly as version 4 is, plus one new invariant, evaluated at
the operation level, strictly last, only once presence, binding, and
requester standing are already established:

> For every capability `c = (s, t)` declared by any root principal with
> `c.requires_approval = true`, and for every operation
> `op = (actor, requester, action, s, t)`, if `c ∈ DA(actor)` and
> `c ∈ DA(requester)`, then `op` is legitimate only if there exists at
> least one declared approval record `a = (approver, s, t)` such that
> `c ∈ DA(approver)`. If no such record exists at all, `op` is an
> `approval_missing` violation. If at least one such record exists, naming
> one or more approvers, but for **every** one of them `c ∉ DA(approver)`,
> `op` is an `approval_unauthorized` violation.

**Four-step operation-level precedence, extended from version 3's
three-step chain:**

```
if C not in DA(actor):
    classify per version 2's rule -> authority_amplification | context_binding_violation
    (requester and approval are NOT evaluated)
elif C not in DA(requester):
    confused_deputy
    (approval is NOT evaluated)
elif not actorState[C].requires_approval:
    ALLOW, no finding — vacuously satisfied
elif no approval record declared for C:
    approval_missing
elif no declared approver independently holds C:
    approval_unauthorized
else:
    ALLOW, no finding
```

`approval_missing`/`approval_unauthorized` are **always** operation-level
findings (`point: "operation"`), never edge-level — approval gates
exercise, not transmission, and there is no approval-related edge-level
finding of any kind.

The `ApprovalFinding` carries `declared_approvers` — `[]` for
`approval_missing`, the full sorted, deduplicated set of declared
approvers for `approval_unauthorized` (existential quantification: one
valid, standing-backed approval record is sufficient; there is no
"canonical" approver to select) — `trace` (the same `CanonicalTrace`
convention every prior operation-level finding already uses), and a
deterministic `reason` string.

Strict distrust extends to a fourth entity kind: an approval record whose
named approver lacks independent standing for the exact capability it
claims to approve contributes **nothing** toward satisfying the approval
requirement — not partial credit. An edge that fails presence, binding, or
depth contributes nothing to the delegatee's derived state at all, so
there is no `requires_approval` fact to leak from an already-invalid edge
either.

See `docs/phase-5-plan.md` §3, §4, §8, §9, §10, §11, §12, §13, §17 for the
full formal statement, multi-path semantics, and verification algorithm.

---

# Version 6 — temporal approval lifecycle model

This section documents the input contract for `"version": "6"` documents
(`docs/phase-6-plan.md` §6–§20). `internal/loader`'s `validateV6` is the
sole runtime source of truth; this is documentation only. Version-1 through
version-5 documents are entirely unaffected by anything in this section —
see the version dispatch rule below.

## What's new

Phase 6 adds exactly one new, optional concept: **`lifecycle`**, a small,
possibly-cyclic named-state automaton attached to an individual
`approvals[]` record. It answers a fifth, independent question none of the
first five invariants can express: not "does a standing-backed approval
exist," but "can that approval ever be observed in a state other than
approved." An approval record with no `lifecycle` field behaves exactly as
it already does under version 5 — eternally active once declared and
standing-backed.

Principals, agents, delegations, and operations are byte-for-byte identical
in shape to their version-5 counterparts. `max_delegation_depth` and
`requires_approval` mean exactly what they meant in version 5. A lifecycle
never attaches to a root capability, a delegation edge, or an operation —
only to the specific approval record it describes.

## Top-level document

```json
{
  "version": "6",
  "principals": [
    { "id": "...", "authority": [
      { "scope": "...", "target": "...", "max_delegation_depth": 1, "requires_approval": true }
    ] }
  ],
  "agents": [ { "id": "..." } ],
  "delegations": [ { "delegator": "...", "delegatee": "...", "authority": [ { "scope": "...", "target": "..." } ] } ],
  "approvals": [
    {
      "approver": "...",
      "scope": "...",
      "target": "...",
      "lifecycle": {
        "initial": "approved",
        "states": ["approved", "revoked"],
        "transitions": [ { "from": "approved", "to": "revoked", "event": "revoke" } ]
      }
    }
  ],
  "operations": [
    { "actor": "...", "requester": "...", "action": "...", "requires": "...", "target": "..." }
  ]
}
```

Unknown fields anywhere in the document (top-level or nested, including
inside a `lifecycle` object) are rejected, exactly as in versions 1-5. In
particular, a stray `lifecycle` key on a root capability, a delegation's
authority entry, or an operation is rejected at decode time (no such field
exists on any of those types).

## Version dispatch

`"6"` routes to the version-6 path described here. `"1"` through `"5"` are
unaffected. Any other value (including absent, which decodes as `""`) is a
single `invalid_version` error:
`` version must be "1", "2", "3", "4", "5", or "6", got %q ``. The six
schemas share no internal model type.

## Field rules

| Field | Rule |
|---|---|
| `version` | Required. Must equal `"1"`, `"2"`, `"3"`, `"4"`, `"5"`, or `"6"`. |
| `principals[]` / `agents[]` / `delegations[]` / `operations[]` | Same rules as version 5. |
| `approvals[].approver` / `.scope` / `.target` | Same rules as version 5. |
| `approvals[].lifecycle` | Optional. A `{initial, states, transitions}` object (§ below). Absent means "no additional temporal structure declared" — identical to version 5's eternal-fact model. |
| `lifecycle.initial` | Required when `lifecycle` is present. Must reference a name present in `lifecycle.states`. |
| `lifecycle.states` | Required when `lifecycle` is present. A non-empty array of distinct state names, each matching the unchanged Phase 2 target grammar (`^[A-Za-z0-9_.-]{1,128}$`). |
| `lifecycle.transitions` | Required when `lifecycle` is present (may be empty). Each entry is `{from, to, event}`: `from`/`to` must each reference a declared state name; `event` is optional and purely diagnostic — `""` (or omitted) is always valid, a non-empty value is checked against the same grammar as `to`/`from`. |

## Lifecycle semantics

- **The reserved safe-state name is exactly `"approved"`** — a fixed
  literal, not an author-declared role. The comparison is exact,
  case-sensitive string equality, with no normalization: a state named
  `"Approved"` is a distinct, unsafe state.
- **Self-loops and cycles are explicitly legal.** Unlike the delegation
  graph, a lifecycle automaton is never required to be acyclic —
  `internal/loader` never runs a cycle-detection pass over one.
- **A lifecycle governs whether an approval record counts, never whether a
  capability is held, bound, or within budget.** It is checked strictly
  after presence, binding, requester standing, and the base approval
  standing check already pass.
- **A lifecycle does not grant, propagate, revoke, or extend authority.**
  Declaring one does not add or remove anything from any node's derived
  authority, and does not affect `graph.TopoSort`/`graph.CanonicalTrace` —
  a lifecycle-bearing approval record is never a graph node or edge.
- **A declared state never mentioned in any transition, or unreachable from
  `initial`, is not a structural error** — it is simply inert.
- **A lifecycle whose reachable set never includes `"approved"` at all is
  not a structural error either** — it is a completely well-formed
  document that will always fail Phase 6's semantic safety check at
  `verify` time (exit 1), never at `validate` time (exit 2).

## New structural error kinds

| Kind | When |
|---|---|
| `unknown_lifecycle_state` | `lifecycle.initial` is empty or does not match a declared state name, or a transition's `from`/`to` does not match a declared state name. A missing or malformed reference falls into this same kind — mirrors `unknown_requester`/`unknown_approver` precisely. |
| `duplicate_lifecycle_state` | Two entries within one `lifecycle.states` array share the exact same name. |
| `duplicate_lifecycle_transition` | Two entries within one `lifecycle.transitions` array share the exact same `(from, event, to)` triple. Two transitions sharing only `from`/`to` but different `event` labels are not a duplicate. |
| `empty_lifecycle_states` | A `lifecycle` object is present but its `states` array has zero entries. |

All version-1/2/3/4/5 structural error kinds apply unchanged.

Explicitly **not** a structural error: a lifecycle that never reaches
`"approved"`, a declared state never used in any transition, a cycle within
a lifecycle, or an approver referenced by a lifecycle-bearing approval
record lacking standing — all are `verify`-time semantic outcomes, never
`validate`-time structural errors.

## Resource bounds

All version-1/2/3/4/5 bounds (`internal/limits`) apply unchanged. Three new
bounds:

| Limit | Value | Notes |
|---|---|---|
| `MaxLifecycleStates` | 32 | Bounds `len(lifecycle.states)` per approval record — a validate-time structural bound. |
| `MaxLifecycleTransitions` | 128 | Bounds `len(lifecycle.transitions)` per approval record — a validate-time structural bound (`4 × MaxLifecycleStates`). |
| `MaxExplorationStatesPerLifecycle` | 32 | A runtime BFS visited-state safety valve (defense-in-depth only — provably unreachable for any validate-time-legal document, since `MaxLifecycleStates` already bounds the declarable state count to the same value). |

## The invariant: Temporal Approval Preservation

A version-6 document is checked against Non-Amplification, Context-Binding
Preservation, Requester Authorization Preservation, Delegation Depth
Preservation, and Approval Preservation exactly as version 5 is, plus one
new invariant, evaluated at the operation level, strictly last, only once
every prior tier — including a non-empty standing-backed approval set —
already passes:

> For every capability `c = (s, t)` requiring approval, and every operation
> already satisfying presence, binding, requester standing, and Phase 5's
> approval-standing existential, `op` additionally satisfies Temporal
> Approval Preservation only if at least one standing-backed approval
> record `a` has `Safe(a) = true` — i.e. every state reachable from `a`'s
> declared `lifecycle.initial`, via `a`'s own declared transitions, is the
> single state `"approved"`. An approval record with no declared
> `lifecycle` is vacuously safe.

**Five-step operation-level precedence, extended from version 5's
four-step chain:**

```
if C not in DA(actor):
    classify per version 2's rule -> authority_amplification | context_binding_violation
elif C not in DA(requester):
    confused_deputy
elif not actorState[C].requires_approval:
    ALLOW, no finding
elif no approval record declared for C:
    approval_missing
elif no declared approver independently holds C:
    approval_unauthorized
elif no standing-backed record is lifecycle-safe:
    approval_lifecycle_unsafe   (>=1 record proven to reach a non-"approved" state)
    approval_lifecycle_unproven (every remaining candidate's exploration was truncated by the bounded-search ceiling, none proven safe)
else:
    ALLOW, no finding
```

`approval_lifecycle_unsafe`/`approval_lifecycle_unproven` are **always**
operation-level findings (`point: "operation"`), never edge-level —
lifecycle, like approval itself, gates exercise, not delegation. Safety is
quantified **universally** over every state reachable from an approval
record's own declared initial state — never existentially over some
convenient path. The only existential quantifier is one layer up: *which*
approval record (among several independently declared, standing-backed
ones) may be relied upon — narrowing, never replacing, version 5's own
existential.

Exploration is bounded, deterministic breadth-first search
(`internal/explore`), run once per distinct lifecycle-bearing approval
record, entirely independently of every other record's lifecycle (never
composed into a cross-product global state — this is what keeps total
exploration cost linear in the number of declared approvals rather than
exponential). An exploration that cannot complete within the bounded
ceiling never resolves to `ALLOW` — it fails closed as
`approval_lifecycle_unproven`.

See `docs/phase-6-plan.md` §3, §8, §9, §10, §11, §12, §13, §14, §16, §21,
§22, §26 for the full formal statement, the exploration algorithm, and the
bounded-search fail-closed specification.
