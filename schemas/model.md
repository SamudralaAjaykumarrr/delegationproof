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
