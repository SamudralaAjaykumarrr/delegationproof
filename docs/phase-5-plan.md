# DelegationProof — Phase 5 Plan

Status: PLANNING ONLY. Phase 1, Phase 2, Phase 3, and Phase 4 are
implemented, merged, and untouched by this document. This is the
authoritative design contract for the Phase 5 implementation session, in
the same spirit as `docs/phase-1-plan.md`, `docs/phase-2-plan.md`,
`docs/phase-3-plan.md`, and `docs/phase-4-plan.md`.

Module: `github.com/SamudralaAjaykumarrr/delegationproof`

---

## 0. Phase 5 rationale

Phase 1 proved Authority Non-Amplification: does a node exercise or
transmit a scope it was never validly granted? Phase 2 proved
Context-Binding Preservation: is a validly-granted scope being exercised
against the target it was granted for? Phase 3 proved Requester
Authorization Preservation: does the party an operation is actually
performed *for* independently hold the capability being exercised? Phase
4 proved Delegation Depth Preservation: has a capability already been
re-delegated more hops from its origin than that origin's own declared
budget permits? All four invariants share one property: they establish
that authority reaching a node, and the specific act of exercising it, is
*real* — genuinely granted, correctly bound, independently backed, and
within its travel budget. None of them can express, and therefore none
can check, a fifth and different question: *this specific exercise is
completely legitimate by every measure above — but its origin declared
that exercising it also requires a second party's explicit sign-off, and
no valid sign-off exists.*

`docs/phase-1-plan.md` §21 named this "approval preservation" (product
idea #4) at the very outset of the project, and every subsequent phase
carried the same deferral forward, untouched:
`docs/phase-2-plan.md` §26, `docs/phase-3-plan.md` §28, and
`docs/phase-4-plan.md` §30 ("**Approval Preservation** (explicitly named
by this task as remaining deferred): a required approval-state concept
attached to operations or edges. Nothing in Phase 4's depth-budget model
changes what an approval layer would need to attach to — it composes with
`DA(n)` and the operation-evaluation precedence chain... exactly as it
would have before Phase 4, orthogonal to depth."). Of the two remaining
named roadmap items — approvals and bounded state-space exploration —
`docs/phase-1-plan.md` §8.2 is explicit that state-space exploration only
becomes *necessary* once a later phase "introduces genuine nondeterminism
or temporal/conditional structure (e.g., 'approval pending' vs 'approved'
states...)." Phase 5's job is to design Approval Preservation as a
**static, declared-fact check** — an approval is either declared, backed
by standing, or it is not, with no pending/approved state machine, no
timestamps, no revocation — so that it attaches to the existing
single-pass, deterministic verification engine exactly as every prior
phase has, and does **not** trigger the need for state-space exploration.
This is the smallest, most directly-motivated remaining invariant in the
roadmap, and the only one repeatedly and explicitly flagged as "next" by
name across all four prior planning documents. Everything else —
multi-hop request chains, explicit per-edge depth attenuation, temporal
approval state, MCP/A2A ingestion — remains future work (§31).

---

## 1. Phase 1-4 baseline

Verified against the actual merged implementation on `main`
(commit `7c18993`), not just the plan documents:

- **Model types**: `internal/model/types.go` (v1), `types_v2.go` (v2,
  `Capability{Scope, Target}`), `types_v3.go` (v3, `OperationV3.Requester`),
  `types_v4.go` (v4, `RootCapability{Scope, Target, MaxDelegationDepth
  *int}` — a pointer specifically so a present, explicit `0` is
  distinguishable from an absent key). All four schemas are structurally
  disjoint Go types sharing no struct, per `docs/phase-2-plan.md` §9's and
  every subsequent phase's explicit "no shared internal model type"
  discipline. `Capability{Scope, Target}` itself, unlike the
  per-version principal/agent/delegation/operation envelope types, *is*
  a genuinely shared, reused type from `types_v2.go` onward — delegation
  edges in v2/v3/v4 all use it verbatim for their `Authority` arrays.
- **Loader dispatch**: `internal/loader/loader_v2.go`'s `LoadDocument`
  peeks `{"version": string}` permissively, then dispatches `"1"`/`"2"`/
  `"3"`/`"4"` to `decodeAndValidateV{1,2,3,4}`, anything else → one
  `KindInvalidVersion` error with message
  `` `version must be "1", "2", "3", or "4", got %q` ``. `Document{V1, V2,
  V3, V4}` union, exactly one field set.
- **Graph**: `internal/graph/graph.go` — `TopoSort` (Kahn's algorithm,
  min-heap lexicographic tie-break), `LongestPath` (DAG DP), `CanonicalTrace`
  (BFS from all principals, sorted expansion, first-path-wins,
  `[]string{actor}` if unreachable). All three operate purely on node ids
  and `[]graph.Edge{From, To}` — no dependency on what a node's derived
  authority contains. Untouched by any phase so far, reusable as-is by
  Phase 5.
- **Verify**: `internal/verify/verify_v4.go`'s `RunV4(*model.ModelV4)` —
  one topological pass builds `da map[string]map[model.Capability]depthState`
  (`depthState{remaining, configuredMax}`, `docs/phase-4-plan.md` §9) for
  every node. An agent's incoming edge is checked in three tiers, in
  ascending-lexicographic-delegator order: (1) presence/binding —
  `isSubsetCap`/`classifyEdge` from `verify_v2.go`, unchanged; if it
  fails, the whole edge is invalid, contributing nothing; (2) depth — if
  any carried capability has `remaining < 1` at the delegator, the whole
  edge is invalid for depth, contributing nothing (whole-edge poisoning,
  identical mechanism to (1)); (3) if both pass, each capability's
  `depthState` is adopted into the delegatee's map via **strict
  improvement only** (`candidate.remaining > existing.remaining`), with no
  new sort needed because incoming edges are already visited in ascending
  lexicographic delegator order. `flatten(map[model.Capability]depthState)
  []model.Capability` (`verify_v4.go`) is the presence-only view every
  Phase 1-3 helper (`isSubsetCap`, `classifyEdge`, `classifyOne`,
  `heldTargetsForScope`, `containsCap`) consumes unmodified. Operation
  evaluation is `docs/phase-3-plan.md` §8's unmodified three-step
  precedence (actor-side check masks everything if it fails; else
  requester-side check; else `ConfusedDeputyFinding`), unaware depth
  exists at all.
- **Report**: `internal/report/finding.go`'s `sortKey{point, subject,
  secondary, scope, target, requester}` — a 6-tuple, extended
  incrementally by Phase 2 (`target`) and Phase 3 (`requester`), each a
  strict extension (empty for older finding shapes). `keyOf`'s type
  switch has one case per finding struct: `EdgeFinding`,
  `OperationFinding` (Phase 1), `CapabilityEdgeFinding`,
  `CapabilityOperationFinding` (Phase 2), `ConfusedDeputyFinding` (Phase
  3), `DelegationDepthFinding` (Phase 4, edge-scoped only — needed no new
  sort-key field because `(point, delegator, delegatee)` is already
  unique per Phase 1 §3.2's at-most-one-edge rule). `RenderText`/
  `RenderJSON` (`text.go`/`json.go`) both switch on finding concrete type;
  `RenderJSON`'s envelope (`{"result", "findings"}`) is generic over
  `[]interface{}` and needs zero change for a new finding type.
- **CLI**: `cmd/delegationproof/main.go`'s `runVerify` dispatch:
  `switch { case doc.V1 != nil: ...; case doc.V2 != nil: ...; case doc.V3
  != nil: ...; case doc.V4 != nil: ... }`.
- **Limits**: `internal/limits/limits.go` — all bounds are exported
  `var`s: `MaxInputFileSize`, `MaxNodes`, `MaxDelegationEdges`,
  `MaxOperations`, `MaxScopeLength`, `MaxIDLength`, `MaxAuthoritySetSize`,
  `MaxChainDepth`, `MaxTargetLength`, `MaxDelegationDepth` (64, a
  resource-safety valve on the *declared* `max_delegation_depth` value,
  deliberately independent of `MaxChainDepth`, the safety valve on actual
  graph shape — CLAUDE.md's explicit invariant that the two must never be
  conflated).
- **Tests**: `internal/loader/loader_v2_test.go` asserts the exact literal
  invalid-version message text — the one sanctioned test-string edit
  every phase since Phase 3 has made, growing the version list by one each
  time.

Phase 5 must not modify any Phase 1, Phase 2, Phase 3, or Phase 4
production code path, and must touch only the sanctioned message-text
lines identified in §5.

---

## 2. Approval threat

Phase 1 answers: *does `billing-agent` hold `billing:refund`?* Phase 2
answers: *is that `billing:refund` valid for `billing-service`?* Phase 3
answers: *does whoever is inducing this operation independently hold
standing?* Phase 4 answers: *has this specific grant already traveled
farther from its origin than its budget permits?* None of them can
answer: *did this exercise receive the second-party sign-off its origin
declared mandatory?*

Concretely: `admin` legitimately owns `billing:refund@billing-service`,
delegates it validly to `billing-agent` within budget, and `billing-agent`
is induced by `admin` itself (a requester with unquestionable standing) to
perform a refund. Every one of Phase 1-4's invariants is satisfied — the
scope is real, the target is right, the requester has standing, the
budget is not exceeded — and every one of them correctly says `ALLOW`.
But if `admin`'s own declared policy for `billing:refund` is "this may
never be exercised without a compliance sign-off," none of the first four
invariants has any vocabulary to express that policy, let alone check it.
This is the classical mandatory-second-party-authorization gap
(four-eyes / dual-control), and it is invisible to Phase 1-4 because none
of them has a concept of "this authority, once real, additionally requires
someone else's explicit backing before it may be used." Phase 5's entire
job is to make that checkable, without reopening or weakening anything
Phase 1-4 already prove.

---

## 3. Minimal new abstraction

Evaluated, in the same spirit as every prior phase's own candidate table:

| Candidate | Verdict | Why |
|---|---|---|
| **A. A boolean policy flag on the root capability declaration (`requires_approval`), plus a new, separate top-level array of declared approval records** | **Chosen** | The approval *requirement* is a property of the origin declaration — exactly parallel to how `max_delegation_depth` (Phase 4) is a property stated once, at the root, and inherited. The approval *itself* (who actually signed off) is a distinct fact, declared once per act of sign-off, not per node — exactly parallel to how a delegation is its own declared fact distinct from the capability it grants. Two atomic additions, no new node kind, no new edge kind. |
| **B. An `approved: bool` field directly on the Operation entity** | Rejected | This would let a document simply assert "this operation was approved" with no independent verification of *who* approved it or whether that party had standing — a document author (or an attacker crafting adversarial input) could trivially set `approved: true` on every operation, making the invariant unfalsifiable. The whole point of an approval invariant is that the approver's own standing is checked, exactly as Phase 3 checks the requester's — a bare boolean on the operation cannot express or check that. |
| **C. An approval-state machine attached to operations or edges (`pending` / `approved` / `rejected`), with transitions** | Rejected | This is precisely the temporal/conditional structure `docs/phase-1-plan.md` §8.2 identifies as the trigger for needing bounded state-space exploration — a state machine implies a *history* (was it ever pending, did it transition), which a single static document with no time dimension cannot represent without inventing a session/event-log concept Phase 5 does not need. A declared fact ("this approval exists, and here is who gave it") is sufficient to state and check the invariant motivating this phase (§2); a workflow with pending/rejected states answers a *different*, harder, currently unmotivated question. |
| **D. An `approved_by` reference field directly on the Operation entity, naming exactly one approver per operation** | Rejected | This conflates *per-operation* granularity with a fact that is more naturally *per-capability*: a compliance sign-off for "billing refunds may be issued" is a property of the capability, not of one specific operation instance, and the project's own operations have no persistent identity across a document (two operations may already legitimately share every other field, `docs/phase-3-plan.md` §15) — there is no stable "this specific operation" for an approval to attach to without inventing an operation-id concept nothing else in the schema needs. Capability-level scoping (Option A) is the correct granularity, exactly as Phase 2's `target` and Phase 4's `max_delegation_depth` are both capability-scoped rather than operation-scoped. |
| **E. A quorum/threshold model (`requires_approval_count: N`)** | Rejected for Phase 5 | Adds a new counting/aggregation dimension (how many *distinct*, independently-standing approvers are required) that nothing in the motivating threat (§2) demands — a single valid, standing-backed sign-off is sufficient to answer the question this phase asks. A later phase may motivate quorum on its own merits (§31) without this phase's design foreclosing it. |

**Decision:** exactly two new, minimal additions, mirroring Phase 4's own
"one field at the root, nothing on the edge or operation" shape as closely
as possible while adding the one genuinely new entity (an approval record)
the threat requires:

1. `requires_approval`, a required boolean, added to the capability tuple
   used **only** in a principal's `authority` array (mirroring
   `max_delegation_depth`'s placement exactly).
2. `approvals`, a new top-level array of declared approval records, each
   naming an approver and the exact capability `(scope, target)` it
   approves. This is the one genuinely new entity Phase 5 introduces — not
   a new graph node, not a new edge kind, not part of the delegation
   graph at all. An approval record is checked, never traversed: it does
   not create an edge, does not participate in `graph.TopoSort` or
   `graph.CanonicalTrace`'s graph, and does not affect any node's
   `DA(n)` presence/binding/depth computation.

Delegation edges and operations gain **no new field at all** in version 5
— byte-for-byte identical in shape to their version-4 counterparts. This
is the smallest possible schema footprint for the threat in §2, and keeps
Phase 5 exactly the same *shape* of addition every prior phase has been:
one or two new atomic facts flowing through the existing algorithm, no new
traversal machinery.

---

## 4. Approval semantics

### 4.1 Is approval part of a capability's identity?

**No**, for the identical reason Phase 4 §4.1 gives for depth:
`(scope, target)` remains the sole identity of a capability. `requires_approval`
is metadata attached to a capability's *origin declaration*, and an
approval record is a *separate* declared fact keyed by the same
`(scope, target)` pair, not a new identity component. The existing
presence/binding subset check (`isSubsetCap`, `classifyEdge`,
`classifyOne` — Phases 2/3/4, entirely unmodified) continues to answer
*is this capability held at all, and for the right target* exactly as
before; approval is a wholly separate, additional dimension of state,
checked only once presence, binding, depth, and requester standing are
already established (§9, §12).

### 4.2 What does `requires_approval` mean, precisely?

- **A capability declared with `requires_approval: false`** behaves
  exactly as every prior-phase capability already does — Phase 5 adds
  nothing to its evaluation.
- **A capability declared with `requires_approval: true`** may be validly
  *exercised* (as an operation's `actor`) only if at least one declared
  approval record names that exact `(scope, target)` pair **and** the
  approval record's named approver independently holds that same
  capability (§8, §12).
- **Approval gates exercise, not transmission.** A delegation edge
  carrying an approval-required capability is evaluated by Phase 1/2/4's
  existing edge-level rules exactly as if `requires_approval` did not
  exist — an approval-required capability may still be freely delegated
  (subject to presence, binding, and depth, unchanged), because
  delegating authority and exercising it are different acts, and Phase 5
  gates only the latter — the identical "usability vs. transmission are
  independently-gated properties of the same held capability" principle
  `docs/phase-4-plan.md` §4.2/§11 already establishes for depth,
  applied to a third dimension. **Consequence:** there is no new
  edge-level structural rule and no new edge-level finding kind in Phase
  5 at all (§8, §12) — a deliberate, explicitly justified minimality,
  mirroring Phase 4 §7's rejection of explicit edge-level depth
  attenuation.
- **An approval does not grant, propagate, or extend authority.**
  Declaring an approval record does not add anything to any node's
  `DA(n)`, does not create a delegation edge, and does not affect
  `graph.TopoSort`/`graph.CanonicalTrace`. It is checked, purely, against
  the approver's own independently-computed `DA(approver)` — the same
  posture `docs/phase-3-plan.md` §4 already establishes for `requester`
  ("orthogonal metadata checked at operation time, not a third input to
  `DA(n)` and not a separate graph").
- **The approver need not be an ancestor of the actor**, and need not be
  the same node as the requester. `DA(approver)` is computed independently,
  via whatever valid chain(s) reach the approver anywhere in the graph —
  the identical "independently held, not ancestor-of-actor" formulation
  Phase 3 §7 establishes for the requester, applied to the approver.

---

## 5. Schema v5

**Decision: a new schema version literal, `"5"`, decoded into a new,
structurally disjoint `model.ModelV5`.** Identical reasoning to every
prior phase's own version-bump decision (`docs/phase-2-plan.md` §9,
`docs/phase-3-plan.md` §5, `docs/phase-4-plan.md` §5): `version` is
checked by hard equality specifically so a new semantic shape never
silently reinterprets old or new documents under the wrong rules.

`ModelV5` is `ModelV4` with exactly two structural changes: (1)
principals' declared authority entries gain a required
`requires_approval` boolean, and (2) the top-level document gains a new,
required `approvals` array. Agents, delegations, and operations are
byte-for-byte identical in shape to their version-4 counterparts.

```json
{
  "version": "5",
  "principals": [
    {
      "id": "admin",
      "authority": [
        {
          "scope": "billing:refund",
          "target": "billing-service",
          "max_delegation_depth": 1,
          "requires_approval": true
        }
      ]
    }
  ],
  "agents": [
    { "id": "billing-agent" }
  ],
  "delegations": [
    {
      "delegator": "admin",
      "delegatee": "billing-agent",
      "authority": [ { "scope": "billing:refund", "target": "billing-service" } ]
    }
  ],
  "approvals": [
    { "approver": "compliance-officer", "scope": "billing:refund", "target": "billing-service" }
  ],
  "operations": [
    {
      "actor": "billing-agent",
      "requester": "admin",
      "action": "refund",
      "requires": "billing:refund",
      "target": "billing-service"
    }
  ]
}
```

**`delegations[].authority` entries and `operations[]` are unchanged from
v4.** No `requires_approval` field, and no approval-related field of any
kind, exists anywhere except (a) inside a principal's `authority` array
and (b) the new top-level `approvals` array. A stray `requires_approval`
key on a delegation's authority entry, or an `approved`/`approval`-shaped
key on an operation, is rejected at decode time by `DisallowUnknownFields`
— the identical "enforced for free by the schema shape" mechanism every
prior phase already relies on (Phase 1 §7.2 for `Agent.authority`, Phase 4
§17 for a stray `max_delegation_depth` on a delegation or operation).

**`requires_approval` is required, with no default.** Considered and
rejected: making it optional, defaulting a missing value to `false` ("no
approval required unless stated"). Rejected on the identical precedent
`docs/phase-4-plan.md` §5 already establishes for `max_delegation_depth`,
extended: `false` is at first glance an innocuous default (unlike an
"unbounded" sentinel for depth, which the brief for Phase 4 barred
outright), but adopting it here would reintroduce exactly the silent
field-omission ambiguity Phase 1's `DisallowUnknownFields` strict-decode
philosophy exists to prevent — a document author who forgets the field
entirely, on a capability that genuinely should require approval, would
have their document silently interpreted as *not* requiring it, with no
signal anywhere that anything is wrong. A required, explicit field forces
every capability's approval policy to be a deliberate, visible
declaration, matching the project's "explicit, not defaulted" discipline
across every prior new field (Phase 2's `target`, Phase 3's `requester`,
Phase 4's `max_delegation_depth`).

**`requires_approval` is typed `*bool`, not `bool`**, for the identical
reason `MaxDelegationDepth` is `*int` and not `int`
(`docs/phase-4-plan.md` §6): `false` is itself a legitimate, meaningful,
commonly-declared value — a plain `bool` field cannot distinguish "author
explicitly wrote `false`" from "author omitted the key entirely," and
Phase 5 needs to reject the latter as a structural error while accepting
the former as valid input (§16).

**Dispatch mechanism**, extending `LoadDocument`'s switch:

```
"1"          -> decodeAndValidateV1 (unchanged)
"2"          -> decodeAndValidateV2 (unchanged)
"3"          -> decodeAndValidateV3 (unchanged)
"4"          -> decodeAndValidateV4 (unchanged)
"5"          -> decodeAndValidateV5 (new)
anything else (including "") -> one KindInvalidVersion error
```

`Document` grows a fifth field: `Document{V1, V2, V3, V4, V5
*model.ModelV5}`, exactly one of which is set on success. The
`invalid_version` message updates from
`` `version must be "1", "2", "3", or "4", got %q` `` to
`` `version must be "1", "2", "3", "4", or "5", got %q` ``, in the five
call sites that must stay textually identical (`validate`, `validateV2`,
`validateV3`, `validateV4`'s existing four copies updated; `validateV5`
introduces the fifth). `internal/loader/loader_v2_test.go`'s asserted
literal must be updated to match — the same sanctioned single-line touch
every prior phase has made.

---

## 6. Root capability semantics

- **`requires_approval` is mandatory on every root capability entry.** No
  optional form, no field omission (§5).
- **`requires_approval` is not part of a capability's identity** (§4.1) —
  duplicate detection (`duplicate_capability`, reused unmodified) is
  still projected onto `(scope, target)` only, extended by Phase 5 to
  cover the same principle Phase 4 §17 already establishes for
  `max_delegation_depth`: two `RootCapability` entries sharing a
  `(scope, target)` pair are a duplicate **regardless of whether their
  `max_delegation_depth` or `requires_approval` values agree** — two
  entries with the same tuple but different declared policy (on either
  dimension) would be genuinely ambiguous about which value governs, so
  both are rejected outright, never implicitly merged, exactly as Phase 4
  already establishes for depth alone.
- **Non-boolean JSON values (`"true"`, `1`, `null`-typed-as-non-pointer
  edge cases aside) are not a `validateV5` concern.** `encoding/json`
  decoding a non-boolean JSON value into `*bool` fails at the decode
  step, surfacing as the existing `LoadError.ParseError` path (`invalid
  JSON: ...`), identical in kind to any other field-type mismatch already
  handled by strict typed decoding today — the exact precedent
  `docs/phase-4-plan.md` §6 establishes for a non-integer
  `max_delegation_depth`, requiring zero new code.

---

## 7. Approval record semantics

A version-5 document gains exactly one new top-level array:

```json
"approvals": [
  { "approver": "compliance-officer", "scope": "billing:refund", "target": "billing-service" }
]
```

- **`approver`** is a reference to an existing node id — the same
  principal/agent id namespace `actor` and `requester` already draw from.
  It carries no new identity concept, no session, no token — the
  identical posture Phase 3 §4 already establishes for `requester`.
- **`scope`/`target`** use the unchanged Phase 2 capability grammar
  (`^[A-Za-z0-9_.:-]{1,256}$` for scope, `^[A-Za-z0-9_.-]{1,128}$` for
  target), reusing `checkScope`/`checkTarget` verbatim.
- **An approval record is capability-scoped, not operation-scoped and not
  actor/requester-scoped.** Evaluated directly against Option D in §3 and
  rejected: an approval names exactly the `(scope, target)` pair it
  covers, and covers *every* operation that exercises that capability,
  regardless of which actor, requester, or action is involved — the
  identical granularity decision Phase 2 makes for `target` and Phase 4
  makes for `max_delegation_depth` (both capability-scoped, neither
  edge- nor operation-scoped).
- **An approval record referencing a `(scope, target)` that no principal
  ever declared is not a structural error.** It is simply inert — never
  matched by any capability check, because nothing will ever hold that
  capability. This is the identical "no registry, nothing to be unknown
  against" reasoning `docs/phase-2-plan.md` §5 already establishes for an
  unrecognized `target`.
- **Multiple approval records may name the same `(scope, target)` with
  different approvers.** This is legal and expected (more than one party
  may be independently empowered to sign off on the same capability); the
  only rejected duplication is an *exact* repeated `(approver, scope,
  target)` triple (§16).
- **An approval record does not need to name a capability the approver
  itself validly holds, structurally.** Whether the named approver
  actually holds standing is a **semantic** question, checked at `verify`
  time (§8, §12), not a `validate`-time structural requirement — the
  identical precedent every prior phase establishes for "a structurally
  well-formed reference that turns out to lack standing is a `verify`-time
  finding, never a `validate`-time error" (`docs/phase-3-plan.md` §15 for
  `requester`, `docs/phase-4-plan.md` §17 for chain length vs. declared
  budget).
- **Self-approval is not structurally prohibited.** An approval record
  may legally name the same node as the operation's `actor` or
  `requester`. Evaluated directly (separation-of-duties is a well-known
  real-world approval-workflow concern) and deliberately **not** enforced
  in Phase 5: the underlying invariant already requires the approver to
  independently hold standing over the capability being approved, which
  is real, checked protection against a blank rubber-stamp — prohibiting
  self-approval on top of that is an *additional*, separable policy
  decision with its own justification burden that nothing in the
  motivating threat (§2) requires. Deferred as a named future option
  (§31), not foreclosed by this decision.

---

## 8. Formal invariant: Approval Preservation

> **Approval Preservation:** for every capability `c = (s, t)` declared by
> any root principal with `c.requires_approval = true`, and for every
> operation `op = (actor, requester, action, s, t)`, let `C =
> Capability{s, t}`. If `C ∈ DA(actor)` and `C ∈ DA(requester)` (i.e.,
> Phases 1, 2, 3, and 4's own invariants are themselves already satisfied
> for `op` — presence, binding, requester standing, all established
> without reference to depth or approval), then `op` is legitimate only if
> there exists at least one declared approval record `a = (approver, s,
> t) ∈ Approvals` such that `C ∈ DA(approver)`. If no such `a` exists at
> all, `op` is an `approval_missing` violation. If at least one such `a`
> exists, naming one or more approvers, but for **every** one of them `C
> ∉ DA(approver)`, `op` is an `approval_unauthorized` violation. If
> `c.requires_approval = false` for the capability actually held by
> `actor` (§10 defines precisely which value governs under multi-path
> delivery), the approval condition is vacuously satisfied and `op` is
> unaffected by anything in this section.

Two things this statement deliberately does **not** say, mirroring
exactly how `docs/phase-3-plan.md` §7 states what Requester Authorization
Preservation does not require:

1. **It does not require the approver to be an ancestor of the actor in
   the actor's own specific delegation chain**, and does not require the
   approver to be the same node as the requester. `DA(approver)` is
   computed independently (§4.2).
2. **It is not evaluated at all when `C ∉ DA(actor)` or `C ∉
   DA(requester)`.** If the actor does not validly hold the capability, or
   the requester lacks standing, Approval Preservation is not the
   applicable diagnosis — a strictly more foundational Phase 1/2/3
   invariant is already violated, and *that* is reported instead (§12).

---

## 9. Derived authority with approval metadata

Extending `docs/phase-4-plan.md` §9's generalization one more dimension:
Phase 5's `DA(n)` is `Capability → authState`, where:

```go
type authState struct {
    remaining        int  // Phase 4, unchanged: best remaining redelegation budget
    configuredMax    int  // Phase 4, unchanged: the root grant's declared max_delegation_depth
    requiresApproval bool // Phase 5, new: does exercising this held instance of c require a valid approval?
}
```

**Why `requiresApproval` is tracked per `(node, capability)`, not
recomputed from the root at finding-assembly time**, for the identical
reason `docs/phase-4-plan.md` §9 gives for `configuredMax`: by the time
verification reaches a deep node, the origin declaration is many hops
upstream, and — critically, unlike `configuredMax`, which is always
carried forward unchanged from whichever single path won the
`remaining`-maximization contest — `requiresApproval` must be aggregated
*independently* across every valid delivering path, not merely carried
along the winning one (§10). Computing it once, in-line, during the same
single topological pass that already computes `remaining`/`configuredMax`,
costs nothing extra algorithmically and requires no additional traversal.

**Presence-only consumers are unaffected.** Every existing Phase 1-4
helper (`isSubsetCap`, `classifyEdge`, `classifyOne`, `heldTargetsForScope`,
`containsCap`) operates on a *flat* `[]model.Capability` and is passed a
view derived from `authState` exactly as it is already passed a view
derived from `depthState` — a new, Phase-5-local function
`flattenApproval(map[model.Capability]authState) []model.Capability`
(structurally identical to `verify_v4.go`'s `flatten`, but over the wider
value type; kept as a distinctly-named function rather than an overload,
since Go does not support overloading by parameter type and the two
`da` maps have different value types — see §24) produces the identical
sorted, deduplicated key-set view. This is the crux of why Phase 5 is
additive rather than a rewrite: presence/binding/requester logic never
needs to know approval exists at all.

---

## 10. Multi-path semantics

Phase 5 has **two independent multi-path questions**, and they are
answered by two independently-justified rules.

### 10.1 Multiple delegation paths delivering the same capability

**Decision: `requiresApproval` is aggregated by logical OR across every
valid delivering path — independently of, and not tied to, whichever
single path wins the `remaining`-maximization contest for depth.**

Worked justification, extending `docs/phase-4-plan.md` §10's own worked
scenario with a second root that disagrees on approval policy:

```
admin-a  --(billing:refund@billing-service, depth 1, requires_approval: true)--> agent-x
admin-b  --(billing:refund@billing-service, depth 3, requires_approval: false)--> agent-y --> agent-x
```

Via `admin-a`: `remaining = 0`, `requiresApproval = true`. Via
`admin-b → agent-y → agent-x`: `remaining = 1` (`3 - 1 - 1`),
`requiresApproval = false`. Applying Phase 4's unmodified `remaining`
rule alone, `agent-x`'s adopted `remaining` is `1` (the `admin-b` path
strictly improves on `admin-a`'s `0`) — but if `requiresApproval` were
naively "carried along with whichever path wins `remaining`" (as
`configuredMax` correctly is, §9), `agent-x` would silently inherit
`requiresApproval = false`, because the winning path for `remaining`
happens to be the non-approval-required one. **This would be an actual
security hole**: a document could declare the same capability at two
roots, one strict (`requires_approval: true`) and one permissive
(`requires_approval: false`), and any node reachable via *both* would
silently launder away the approval requirement by virtue of the
permissive root's path also happening to deliver more remaining budget —
two entirely unrelated policy dimensions would become accidentally
coupled through an implementation artifact of the depth-maximization
rule.

The correct, fail-closed rule — directly required by CLAUDE.md's own
"strict distrust" and "fail closed" invariants — is: `agent-x`'s adopted
`requiresApproval` is `true` **if true for any valid delivering path**,
computed **independently** of which path wins `remaining`. Concretely,
during the same per-node accumulation loop already iterating incoming
valid edges in ascending lexicographic delegator order (§9, unchanged
from Phase 4), two independent aggregations run side by side over the
same loop:

```
remaining/configuredMax:  adopt only on strict improvement (max), Phase 4's rule, unchanged
requiresApproval:         adopt via logical OR across every valid edge, independently
```

**No tie-break is needed for the OR aggregation, and none is defined**,
because logical OR is commutative, associative, and idempotent — the
result is identical regardless of the order incoming edges are visited
in, unlike `remaining`'s max-aggregation, which needs Phase 4 §10's
explicit ascending-lexicographic tie-break specifically because two
*unequal* candidate values need a deterministic choice when they tie at
the same maximum. OR has no comparable ambiguity to resolve: this is a
*stronger* determinism guarantee than Phase 4's own, and is stated here
explicitly rather than left implicit.

**Why this is not "combining untrustworthy partial information," matching
Phase 4 §10's own framing:** each path is independently and fully valid
on its own terms; `agent-x` genuinely, validly holds `billing:refund@
billing-service` with a real obligation to seek approval via the
`admin-a` path, and that obligation is not retroactively erased merely
because a second, independently-true path also delivers the same nominal
capability without that obligation. Adopting the more restrictive of two
true facts is the security-correct choice, symmetric with — but the
inverse polarity of — Phase 4's choice to adopt the more *permissive* of
two true facts for `remaining` (there, more permissive was actually more
informative, not less safe, because higher remaining budget is itself
provable from either path independently; here, the two facts are
outcomes of a strictness policy, and the fail-closed choice is the
stricter one).

### 10.2 Multiple approval records for the same capability

**Decision: existential quantification — one valid, standing-backed
approval record is sufficient; there is no notion of a single "canonical"
approver to select.** Unlike a delegation-graph trace (where a BFS-derived
first-path-wins rule is needed because *some* single, illustrative path
must be chosen for display, §11 of `docs/phase-1-plan.md`), the set of
approval records matching a given `(scope, target)` is a flat, declared
list with no graph structure, no notion of "shorter" or "earlier" — the
diagnostically complete and simplest deterministic answer is the full,
sorted, deduplicated set of matching approvers, not one arbitrarily
chosen representative. Determinism here needs no BFS-style canonicalization
at all: a single ascending-lexicographic sort plus deduplication over
`approvals[]`, applied once (§17), is sufficient and trivially
permutation-invariant regardless of the order `approvals[]` entries
appear in the input document.

---

## 11. Strict distrust semantics

Preserved and extended, per CLAUDE.md's invariant, in the identical spirit
`docs/phase-4-plan.md` §11 already applies to depth:

**Three independently-tracked failure surfaces at the edge level (unchanged
from Phase 4, Phase 5 adds no fourth edge-level surface — §4.2, §12):**
presence/binding (Phase 1/2), depth (Phase 4). Approval is deliberately
**not** an edge-level failure surface at all (§4.2) — it never poisons a
delegation edge, never contributes to `validEdges`/`CanonicalTrace`
exclusion, and requires no change to the edge-evaluation loop beyond
propagating one additional field through the state already carried per
capability (§9).

**A new, operation-level strict-distrust rule, the direct analogue of
Phase 1's "no partial credit":** an approval record whose named approver
lacks independent standing for the exact capability it claims to approve
contributes **nothing** toward satisfying the approval requirement — not
partial credit, not "half approved," not a weaker warning-level finding.
It is treated exactly as if it did not exist. This is the same
`TestStrictDistrustNoPartialCredit`-style discipline CLAUDE.md names
explicitly, applied to a fourth entity kind (an approval record, alongside
delegation edges, capability grants, and now approvals) rather than
loosened for it. Concretely: if `approvals[]` contains ten records naming
`billing:refund@billing-service`, and nine of them are declared by
approvers who do not independently hold that capability, those nine
contribute nothing — the operation is legitimate if and only if at least
one of the ten *does* independently hold standing (§8, §10.2), regardless
of how many non-standing records surround it.

**No new code is required for depth's or presence/binding's strict
distrust to extend correctly to approval**, for the identical reason Phase
4 §9 requires none for depth: an edge that fails presence, binding, or
depth contributes nothing to the delegatee's `da` entry at all — no
`authState` value exists for that capability at that node — so there is
no `requiresApproval` fact to leak, partially or otherwise, from an
already-invalid edge. Strict distrust at the edge level automatically,
mechanically extends to the approval dimension with zero new logic.

---

## 12. Interaction with Phases 1-4

Five invariants now compose over the same graph and the same `DA(n)`.
Precedence is defined **per detection point**, extending
`docs/phase-4-plan.md` §12's own framing.

**Edge-level (`point = "delegation_edge"`), unchanged three-tier
precedence from Phase 4 — Phase 5 adds no new edge-level tier and no new
edge-level finding kind at all (§4.2, §11):**

```
authority_amplification  >  context_binding_violation  >  delegation_depth_violation
```

**Operation-level (`point = "operation"`), extended from Phase 3's
three-step precedence to a new, four-step precedence — approval is
checked strictly last, only once presence, binding, and requester standing
are all already established:**

```
evaluate(op, da, approvals):
    C = Capability{op.Requires, op.Target}

    // Step 1 — unchanged Phase 1/2 check, unchanged classification.
    actorFlat = flattenApproval(da[op.Actor])
    if C not in actorFlat:
        emit CapabilityOperationFinding(classifyOne(...))   // authority_amplification | context_binding_violation
        return                                                // requester and approval NOT evaluated

    // Step 2 — unchanged Phase 3 check.
    requesterFlat = flattenApproval(da[op.Requester])
    if C not in requesterFlat:
        emit ConfusedDeputyFinding(...)
        return                                                // approval NOT evaluated

    // Step 3 — new Phase 5 check, only reached once steps 1-2 have passed.
    actorState = da[op.Actor][C]        // exists: step 1 already confirmed presence
    if not actorState.requiresApproval:
        return                                                // ALLOW, no finding — vacuously satisfied

    declaredApprovers = approvalsByCapability[C]               // §17, precomputed once, sorted+deduped
    if declaredApprovers is empty:
        emit ApprovalFinding("approval_missing", declaredApprovers: [])
        return

    standingApprovers = { a in declaredApprovers : C in flattenApproval(da[a]) }
    if standingApprovers is empty:
        emit ApprovalFinding("approval_unauthorized", declaredApprovers: declaredApprovers)
        return

    // Step 4 — every check passed.
    return   // ALLOW, no finding
```

**Why depth never participates in the operation-level chain**, unchanged
from `docs/phase-4-plan.md` §12: depth gates transmission, not use, and a
depth-exhausted upstream edge already surfaces, if at all, as an ordinary
Phase 1/2 amplification finding at step 1 — approval-checking is never
even reached for such an operation, because the capability is simply
absent from `flattenApproval(da[actor])`. **Why approval never masks, or
is masked by, depth**, extending the same reasoning: the two concerns are
completely orthogonal (edge-scoped vs. operation-scoped, transmission vs.
use), and — as with confused-deputy and depth in Phase 4 — both findings,
when both apply to related but distinct parts of the same document, are
legitimately emitted independently, at their own points, with no masking
between them (§22's worked example demonstrates the analogous
`delegation_depth_violation` + `authority_amplification` two-finding
shape from Phase 4; §21's combined-violation fixture demonstrates all five
violation kinds coexisting without interference).

**Full precedence table** (extending `docs/phase-3-plan.md` §8's table by
one final row):

| Actor holds `C`? | Requester holds `C`? | Requires approval? | Valid approval exists? | Finding |
|---|---|---|---|---|
| No (never held, any target) | — | — | — | `authority_amplification` |
| No (held, wrong target only) | — | — | — | `context_binding_violation` |
| Yes | No (never held, any target) | — | — | `confused_deputy` |
| Yes | No (held, wrong target only) | — | — | `confused_deputy` |
| Yes | Yes | No | — | none — `ALLOW` |
| Yes | Yes | Yes | No approval record at all | `approval_missing` |
| Yes | Yes | Yes | Record(s) exist, none standing-backed | `approval_unauthorized` |
| Yes | Yes | Yes | At least one standing-backed record | none — `ALLOW` |

No ambiguous or duplicate findings are possible: the function returns
exactly one outcome (`PASS` or exactly one violation literal) per
operation, by construction — the identical guarantee
`docs/phase-3-plan.md` §12 establishes for its own three-step chain,
extended by one step.

---

## 13. Requester interaction

**Confirmed directly, mirroring `docs/phase-4-plan.md` §13's own
confirmation for depth: a requester needs capability standing (presence in
`DA(requester)`), never approval-gating of its own, because requesting an
operation is not the same act as exercising it, and is not the same act as
approving it.** `op.Requester` is checked once, at step 2 of §12's chain,
via the same `flattenApproval` presence-only view every other node's
standing is checked through — the requester's own `authState[C]
.requiresApproval` value is never consulted; only the **actor's** copy of
that fact governs the approval gate (§12 step 3), because it is the actor
who is actually performing the exercise the approval requirement
attaches to. A requester whose only apparent standing for `C` arrived via
a now-depth-exhausted or approval-unrelated path is unaffected by
anything Phase 5 adds — its presence in `DA(requester)` is determined
exclusively by Phase 1-4's unmodified presence/binding/depth rules.

**Symmetrically, an approver is checked exactly like a requester is —
via `flattenApproval(da[approver])` — and is never itself subject to a
recursive approval-gate.** An approver's own act of appearing in an
`approvals[]` record is not modeled as "exercising" the capability it
approves (it never appears as an operation's `actor`), so there is no
question of whether *approving* itself needs approval — this would be an
infinite-regress concern the design deliberately avoids by construction:
approval is checked only at the point an operation names an `actor`, and
an approval record is not an operation.

---

## 14. Deterministic findings

One new finding type, alongside the four existing, unmodified finding
types (`EdgeFinding`/`OperationFinding` from Phase 1,
`CapabilityEdgeFinding`/`CapabilityOperationFinding` from Phase 2,
`ConfusedDeputyFinding` from Phase 3, `DelegationDepthFinding` from Phase
4):

```go
// internal/report/approval_finding.go

const (
    ViolationApprovalMissing      = "approval_missing"
    ViolationApprovalUnauthorized = "approval_unauthorized"
)

// ApprovalFinding is always an operation-level finding (point =
// "operation") — approval gates exercise, never delegation (§4.2, §11).
// DeclaredApprovers is [] for approval_missing (no record at all exists
// for this capability) and the full sorted, deduplicated set of approvers
// named by matching records for approval_unauthorized (none of whom
// independently hold the capability — §11's strict distrust means a
// non-standing record contributes nothing, but is still surfaced here for
// diagnostic completeness).
type ApprovalFinding struct {
    Violation         string     `json:"violation"`          // "approval_missing" | "approval_unauthorized"
    Point             string     `json:"point"`               // always "operation"
    Actor             string     `json:"actor"`
    Requester         string     `json:"requester"`
    Action             string    `json:"action"`
    Requires           Capability `json:"requires"`
    DeclaredApprovers  []string   `json:"declared_approvers"`
    Trace              []string   `json:"trace"`
    Reason             string     `json:"reason"`
}
```

`point` reuses Phase 1/2/3's existing `"operation"` literal unchanged —
Phase 5 introduces no new detection point (§4.2, §11: approval is never
edge-level).

**Deterministic reason text** (generated, not free-form, same discipline
as every prior phase):

- `approval_missing`:
  `"<action> requires <scope>@<target>, which <actor> validly holds and
  <requester> is authorized to request, but <scope>@<target> requires
  approval and no approval has been declared for it"`
- `approval_unauthorized`:
  `"<action> requires <scope>@<target>, which requires approval; approval
  was declared by [<declared_approvers joined by ", ">], but none of them
  independently hold <scope>@<target> — an approval must come from a
  party with standing over the capability being approved"`

`declared_approvers` is always present (`[]`, never omitted or null —
Phase 1 §9's array-field rule, unchanged).

**No new sort-key field is required.** `report.Sort`'s existing 6-tuple
`sortKey{point, subject, secondary, scope, target, requester}` already
has exactly the granularity `ApprovalFinding` needs — `(point: "operation",
subject: Actor, secondary: Action, scope: Requires.Scope, target:
Requires.Target, requester: Requester)` — identical in shape to
`ConfusedDeputyFinding`'s key. `keyOf` gains one more type-switch case
returning that tuple directly, with **no struct-level extension needed**
— a smaller change than either Phase 2's or Phase 3's own sort-key
extension, and, unlike `DelegationDepthFinding` (which needed no new
field because it is edge-scoped and already unique at a coarser
granularity), `ApprovalFinding` needs no new field because it fits the
*existing full* six-field granularity exactly, the same as
`ConfusedDeputyFinding` already does. Worth stating explicitly: this is
the first new finding type in the project's history that requires
*literally zero* changes to the `sortKey` struct itself.

---

## 15. Canonical traces

**One trace, not two — deliberately, contrasting with Phase 3's two-trace
design for confused-deputy.** `ApprovalFinding.Trace =
graph.CanonicalTrace(principalIDs, validEdges, actor) + [action]`, the
identical construction every operation-level finding from Phase 1 onward
already uses. No `approver_trace` is added, for a reason specific to §10.2:
an approval record's matching approvers are a flat, unordered, potentially
multi-element set with no single canonical representative to trace (§10.2
already establishes this — there is no "the" approver to walk a path to,
the way there is exactly one requester per operation). Adding one trace
per declared approver would produce an unbounded-in-principle array of
traces (bounded only by `MaxApprovals`, §21), which is exactly the kind of
unbounded finding-size growth the project's resource-bounds discipline
avoids; `declared_approvers` (a flat id list, §14) is deliberately kept as
the diagnostic payload instead, and a reader who needs to understand *why*
a specific named approver lacks standing can re-run `verify` against a
document reduced to that one approval record, or independently inspect
`DA(approver)` — the identical latitude Phase 4 §15 already grants for its
own trace ("illustrative provenance context... not a formal proof of the
specific numeric claim").

`validEdges`, for Phase 5's trace purposes, means edges that were **fully
valid** — passed presence, binding, and depth (§4.2's decision that
approval never gates edges means `validEdges` is computed identically to
Phase 4, with zero new participation from approval).

---

## 16. Validation

Every existing structural rule from Phase 1-4 applies to version-5
documents unchanged, generalized only where the shape changed
(`RootCapability` in principal authority arrays gains `RequiresApproval`;
delegations and operations are unchanged from v4; a new top-level
`approvals` array is validated).

**New version-5-only structural rules:**

- **`KindMissingApprovalRequirement = "missing_approval_requirement"`** —
  a `RootCapability`'s `RequiresApproval` is `nil` (the key was omitted).
  There is no "negative" or out-of-range sub-case for a boolean field
  (unlike `max_delegation_depth`), so this kind covers exactly one
  condition, mirroring the "one kind, one clear condition" discipline
  `unknown_requester`/`unknown_approver` already establish for their own
  single conditions.
- **`KindUnknownApprover = "unknown_approver"`** — `approvals[].approver`
  does not resolve to a known principal or agent id, mirroring
  `unknown_requester`/`unknown_actor` precisely. A missing `approver`
  (decodes as `""`) or a syntactically-malformed one both fall into this
  same kind — identical precedent to `docs/phase-3-plan.md` §15's
  treatment of `requester` ("a syntactically-malformed... reference can,
  by construction, never match a registered node id either").
- **`KindDuplicateApproval = "duplicate_approval"`** — two entries within
  `approvals[]` share the exact same `(approver, scope, target)` triple.
  Two entries sharing only `scope`/`target` but naming *different*
  approvers are not a duplicate — a real, legitimate case (§7).
- **Resource-limit check, reusing the existing generic mechanism**:
  `len(m.Approvals) > limits.MaxApprovals` (§21) is
  `KindResourceLimitExceeded` with `Primary = "max_approvals"` — no new
  `ErrorKind` needed.
- **`duplicate_capability`, extended** (§6): two `RootCapability` entries
  sharing `(scope, target)` are a duplicate regardless of whether their
  `max_delegation_depth` **or** `requires_approval` values agree.

**Explicitly evaluated and rejected** (per the established discipline of
not adopting a suggested-list wholesale):

- **"Unknown capability referenced by an approval"** — rejected (§7): no
  registry to be unknown against, identical to Phase 2's rejection of
  "unknown target."
- **"Approval standing violated" as a `validate`-time (exit 2) error** —
  rejected outright, on the identical precedent every prior phase
  establishes (`docs/phase-1-plan.md` §7.4, `docs/phase-2-plan.md` §10,
  `docs/phase-3-plan.md` §15, `docs/phase-4-plan.md` §17): a structurally
  well-formed document that turns out to violate a semantic invariant is
  a `verify`-time finding (exit 1), never a `validate`-time structural
  error (exit 2).
- **"Non-boolean `requires_approval` value"** — not a `validateV5`
  concern at all; a JSON decode-level type mismatch (§6).
- **"Stray approval-related field on a delegation or operation entry"** —
  not a dedicated check; enforced for free by `DisallowUnknownFields`
  against `DelegationV5`/`OperationV5`'s unchanged (from v4) field sets
  (§5).
- **"Self-approval"** — not a structural error (§7); deliberately legal.

`validate` on a version-5 document therefore still never evaluates any
invariant — Non-Amplification, Context-Binding, Requester Authorization,
Delegation Depth, or Approval Preservation — exactly as established for
v1-v4.

---

## 17. Verification algorithm

**Decision: Phase 5 still fits entirely within one static, deterministic,
topological pass plus one bounded pre-indexing pass over `approvals[]`.
No state-space exploration, no backtracking, no per-capability path
search, no temporal/session state (§0).**

`RunV5(*model.ModelV5) report.Result`, structurally parallel to `RunV4`:

1. **Build the graph and compute `da`.** Byte-for-byte the same steps
   `RunV4` already performs, over the extended `authState{remaining,
   configuredMax, requiresApproval}` (§9): nodes = principals ∪ agents,
   edges = delegations, one topological pass, ascending-lexicographic
   tie-break, unchanged `graph.TopoSort`. Per node, per incoming valid
   edge (ascending lexicographic delegator order, unchanged):
   - Presence/binding check (unchanged Phase 2 `isSubsetCap`/
     `classifyEdge`, operating over `flattenApproval(delegatorStates)`):
     invalid → `CapabilityEdgeFinding`, edge contributes nothing.
   - Depth check (unchanged Phase 4): any carried capability with
     `remaining < 1` at the delegator → `DelegationDepthFinding`,
     whole edge contributes nothing (whole-edge poisoning, §11).
   - Edge fully valid: for each `c ∈ e.Authority`, read
     `parentState := delegatorStates[c]`. Compute
     `candidate := authState{remaining: parentState.remaining - 1,
     configuredMax: parentState.configuredMax, requiresApproval:
     parentState.requiresApproval}`. Adopt into `states[c]` via **two
     independent rules, applied together** (§10.1):
     ```go
     existing, has := states[c]
     if !has {
         states[c] = candidate
     } else {
         remaining, configuredMax := existing.remaining, existing.configuredMax
         if candidate.remaining > existing.remaining {
             remaining, configuredMax = candidate.remaining, candidate.configuredMax
         }
         states[c] = authState{
             remaining:        remaining,
             configuredMax:    configuredMax,
             requiresApproval: existing.requiresApproval || candidate.requiresApproval,
         }
     }
     ```
2. **Index approvals once, before operation evaluation** (new, §10.2,
   §12): build `declaredApprovers map[model.Capability][]string` by
   iterating `m.Approvals` once, grouping by `Capability{Scope, Target}`,
   then sort-and-deduplicate each bucket by ascending approver id.
   Simultaneously compute `standingApprovers map[model.Capability][]string`
   by, for each distinct capability key with at least one declared
   approver, filtering to only those approvers `a` where `C ∈
   flattenApproval(da[a])` — computed **once per distinct capability**,
   not once per operation, so that an operation-evaluation step is a plain
   O(1) map lookup rather than a re-scan of the approver list. This keeps
   the overall complexity linear in `Ap` (the number of approval records)
   regardless of how many operations reference the same capability.
3. **Operation evaluation**, operations in the existing ascending
   `(actor, action, requires.Scope, requires.Target, requester)` order
   (unchanged from `docs/phase-4-plan.md` §16): run §12's four-step
   precedence, using `flattenApproval(da[actor])`/
   `flattenApproval(da[requester])` for steps 1-2 (unchanged machinery)
   and the precomputed `declaredApprovers`/`standingApprovers` maps for
   step 3 (§12).
4. **Sort all findings** (all six finding shapes together — Phase 1's two,
   Phase 2's two, Phase 3's one, Phase 4's one, Phase 5's one) by the
   unmodified 6-tuple key (§14).
5. **Result:** `ALLOW` (exit 0) if empty, else `DENY` (exit 1) — unchanged
   result semantics.

**Complexity.** Let `N` = nodes, `E` = delegation edges, `A` = the bound
on per-edge/per-principal capability-set size (`limits.MaxAuthoritySetSize`),
`O` = operations, `Ap` = approval records. Step 1 is unchanged from Phase
4's `O(N + E·A)`. Step 2 does `O(Ap log Ap)` work (sorting each capability
bucket) plus, for each distinct capability with declared approvers, an
`O(k)` standing check where `k` is that capability's own declared-approver
count — summed across all distinct capabilities, this is bounded by `O(Ap)`
total (each approval record is visited a constant number of times). Step 3
is `O(1)` per operation (a plain map lookup into the precomputed
`standingApprovers`), so `O(O)` total. The whole pass is therefore
`O(N + E·A + O + Ap log Ap)` — the identical asymptotic class every prior
phase already runs, with `Ap` entering as one additional, independently
bounded linear-ish term, **not** as a multiplicative `O(Ap · O)` term (the
naive per-operation-rescan design would have been `O(Ap · O)` in the
worst case — up to `10,000 × 10,000` map lookups under the resource bounds
in §21 — still technically bounded and panic-free, but this design
deliberately avoids that quadratic-in-the-worst-case shape by precomputing
per-capability standing once, up front). **No per-capability path
enumeration, no branching over alternative interpretations, no
state-space search is introduced or required** — every quantity
(`da[n][c].requiresApproval`, `standingApprovers[c]`, operation pass/fail)
has exactly one correct, deterministically-computed value given the
acyclic input, for the identical reason `docs/phase-1-plan.md` §8.2 gives:
no time dimension, no conditional grant, nothing to search over.

`model.Model` (v1), `model.ModelV2` (v2), `model.ModelV3` (v3), and
`model.ModelV4` (v4) continue to run their existing, entirely untouched
`Run`/`RunV2`/`RunV3`/`RunV4` functions, byte-identical to today.

---

## 18. CLI compatibility

**No new subcommands, no new flags.** `validate <model.json>` and
`verify <model.json> [--format text|json]` remain the only two commands.
`main.go`'s existing dispatch switch in `runVerify` gains one more case:

```go
case doc.V5 != nil:
    result = verify.RunV5(doc.V5)
```

`--format text|json` applies identically across all five versions. No
`--approver` override flag, no version-selection flag — version is read
from the document, exactly as `"1"`-`"4"` already are.

---

## 19. Text/JSON compatibility

**JSON.** The top-level envelope (`{"result", "findings"}`,
`internal/report/json.go`) is unchanged — already generic over
`[]interface{}`, so `ApprovalFinding` requires zero changes to
`RenderJSON`. Version-1/2/3/4 output is byte-identical to today,
unconditionally: `RunV5` is a new function, called only when `doc.V5 !=
nil`, touching no code path `Run`/`RunV2`/`RunV3`/`RunV4` execute.

**Text.** `RenderText`'s type switch gains one new case, matching the
existing label-column style:

```
[1] approval_missing (operation)
  actor:              billing-agent
  requester:          admin
  action:             void-unapproved
  requires:           billing:void@billing-service
  declared approvers: (none)
  trace:              admin -> billing-agent -> void-unapproved
  reason:             void-unapproved requires billing:void@billing-service, which billing-agent validly holds and admin is authorized to request, but billing:void@billing-service requires approval and no approval has been declared for it
```

```
[1] approval_unauthorized (operation)
  actor:              billing-agent
  requester:          admin
  action:             refund
  requires:           billing:refund@billing-service
  declared approvers: intern-agent
  trace:              admin -> billing-agent -> refund
  reason:             refund requires billing:refund@billing-service, which requires approval; approval was declared by intern-agent, but none of them independently hold billing:refund@billing-service — an approval must come from a party with standing over the capability being approved
```

`joinOrNone` (existing helper) is reused for `declared approvers` exactly
as it is for `held`/`declared`/`excess`/`requester bound` today.

(Exact column labels/widths are an implementation-session detail, matching
the latitude every prior phase's plan already left for text rendering.)

---

## 20. Exit codes

Unchanged. `internal/exitcode` gains no new values:

| Code | Meaning (extended) |
|---|---|
| `0` | Structurally valid model (v1-v5); zero findings for `verify`. |
| `1` | One or more findings — `authority_amplification`, `context_binding_violation`, `confused_deputy`, `delegation_depth_violation`, and/or `approval_missing`/`approval_unauthorized`, in any combination. A v5 model can `DENY` on any mix; the exit code does not distinguish which. |
| `2` | Structural/model problem for any schema version, including the new `missing_approval_requirement`/`unknown_approver`/`duplicate_approval` kinds and the `max_approvals` resource limit. |
| `3` | CLI usage error — unchanged. |

---

## 21. Resource bounds

One new bound, mirroring the existing `MaxOperations`/`MaxDelegationEdges`
pattern exactly:

| Limit | Value | Notes |
|---|---|---|
| `MaxApprovals` | `10000` | New. Bounds the number of entries in the top-level `approvals` array. Mirrors `MaxOperations`'s value and role: `approvals` is a new, independent top-level array, not nested inside any existing bounded collection, so it needs its own bound rather than reusing `MaxAuthoritySetSize` or `MaxOperations`. |

**No new bound for "approvers per capability."** Already sufficiently
bounded transitively by `MaxApprovals` globally — inventing a second,
narrower per-capability bound would be exactly the kind of unjustified
extra knob `docs/phase-2-plan.md` §17 already declined for "distinct
targets per scope," and `docs/phase-4-plan.md` §21 declined for a
depth-specific state-size bound.

**No new bound for approver-id length** — `approver` reuses the existing
node-id namespace and its existing `MaxIDLength` bound, validated by the
same "resolves to a known node id" mechanism `requester` already uses
(§16), not a separate grammar check.

**Complexity confirmation, §17.** The precomputed
`standingApprovers`/`declaredApprovers` design keeps total approval-related
work at `O(Ap log Ap)` (dominated by per-capability-bucket sorting) —
under the bounds above (`Ap ≤ 10000`), this is a small, fast, one-time
cost, independent of `MaxOperations`.

All existing Phase 1-4 bounds (`MaxInputFileSize`, `MaxNodes`,
`MaxDelegationEdges`, `MaxOperations`, `MaxScopeLength`, `MaxIDLength`,
`MaxAuthoritySetSize`, `MaxChainDepth`, `MaxTargetLength`,
`MaxDelegationDepth`) apply to version-5 documents unchanged.

---

## 22. Worked example

`examples/billing-approval.json` (implementation-session file, structured
the same deliberate way every prior phase's own example file is:
small, readable, and exercising exactly one isolated variable — here, the
presence versus absence of a valid, standing-backed approval for an
otherwise-identical class of capability):

```json
{
  "version": "5",
  "principals": [
    {
      "id": "admin",
      "authority": [
        { "scope": "billing:refund", "target": "billing-service", "max_delegation_depth": 1, "requires_approval": true },
        { "scope": "billing:void",   "target": "billing-service", "max_delegation_depth": 1, "requires_approval": true }
      ]
    },
    {
      "id": "compliance-officer",
      "authority": [
        { "scope": "billing:refund", "target": "billing-service", "max_delegation_depth": 0, "requires_approval": true }
      ]
    }
  ],
  "agents": [
    { "id": "billing-agent" }
  ],
  "delegations": [
    { "delegator": "admin", "delegatee": "billing-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" },
      { "scope": "billing:void",   "target": "billing-service" }
    ] }
  ],
  "approvals": [
    { "approver": "compliance-officer", "scope": "billing:refund", "target": "billing-service" }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin", "action": "refund-approved",  "requires": "billing:refund", "target": "billing-service" },
    { "actor": "billing-agent", "requester": "admin", "action": "void-unapproved",  "requires": "billing:void",   "target": "billing-service" }
  ]
}
```

`verify examples/billing-approval.json`:

- **`refund-approved`** (requires `billing:refund@billing-service`) —
  `billing-agent` validly holds it (one hop, within depth-1 budget);
  `admin` holds it directly, axiomatically (Phase 1-4 all pass). The
  capability's `requires_approval` is `true`. `approvals[]` contains a
  record naming `compliance-officer` for exactly this `(scope, target)`;
  `compliance-officer` independently, axiomatically holds
  `billing:refund@billing-service` — a standing-backed approval exists.
  **Passes**, no finding.
- **`void-unapproved`** (requires `billing:void@billing-service`) —
  `billing-agent` validly holds it identically (same delegation edge,
  same budget shape); `admin` holds it directly. The capability's
  `requires_approval` is also `true`. `approvals[]` contains **no**
  record naming `billing:void@billing-service` at all. **Fails**:
  `approval_missing`, with `declared_approvers = []`.

This single file demonstrates: two structurally identical
approval-required capabilities, differing only in whether a valid,
standing-backed approval was declared — isolating exactly the one
variable Phase 5 introduces, mirroring exactly how every prior phase's
own worked example isolates its one new variable
(`docs/phase-3-plan.md` §20's `requester`-only difference,
`docs/phase-4-plan.md` §22's depth-boundary difference).

---

## 23. Examples

Beyond the single-violation worked example (§22, `examples/`), Phase 5
requires the following additional fixtures, following the established
`testdata/valid-vN/` convention (`docs/phase-3-plan.md` §21,
`docs/phase-4-plan.md` §23) for fixtures whose purpose is comprehensive
coverage rather than minimal illustration:

1. **Clean example** — `testdata/valid-v5/clean-pass-v5.json`: every
   approval-required capability in the document has at least one
   standing-backed approval; `verify` → `ALLOW`, exit 0, zero findings.
2. **Focused violation example** — `examples/billing-approval.json` (§22)
   — already satisfies this requirement; the `void-unapproved` operation
   is the single isolated `approval_missing` violation.
3. **Combined-violation example** — `testdata/valid-v5/combined-violations-v5.json`:
   a single document producing `authority_amplification`,
   `context_binding_violation`, `confused_deputy`,
   `delegation_depth_violation`, `approval_missing`, and
   `approval_unauthorized` findings simultaneously, on independent
   edges/operations, correctly classified and correctly ordered, with no
   masking between unrelated findings and correct masking within any
   single edge/operation per §12's precedence table.
4. **Multi-path example** — `testdata/valid-v5/multi-path-approval.json`:
   the §10.1 worked scenario as a fixture (two roots declaring the same
   capability with disagreeing `requires_approval` values, delivered to a
   shared downstream node via two independently-valid paths), confirming
   the OR-aggregation is adopted (the node's exercise of the capability
   is correctly gated by approval despite the more-permissive root also
   delivering it).
5. **`approval_unauthorized`-focused fixture** — `testdata/valid-v5/approval-unauthorized.json`:
   a structurally valid document where an approval record exists for the
   exercised capability, but the named approver does not independently
   hold it — isolates `approval_unauthorized` distinctly from
   `approval_missing`, since §22's worked example only exercises the
   latter.

---

## 24. Architecture/file plan

Purely additive to `docs/phase-1-plan.md` §15 / `docs/phase-2-plan.md`
§19 / `docs/phase-3-plan.md` §21 / `docs/phase-4-plan.md` §23. No
existing file is deleted or renamed; the one sanctioned message-text
touch is called out explicitly (§5, §27).

```
internal/model/
  types.go, types_v2.go, types_v3.go, types_v4.go  — UNCHANGED
  types_v5.go                            — NEW: RootCapabilityV5{Scope,
                                          Target, MaxDelegationDepth *int,
                                          RequiresApproval *bool} (a new
                                          type, not a reuse of v4's
                                          RootCapability — mirrors how
                                          types_v4.go introduced its own
                                          RootCapability rather than
                                          reusing types_v3.go's Capability
                                          for principal declarations),
                                          ApprovalV5{Approver, Scope,
                                          Target string} (new entity),
                                          ModelV5{Version, Principals,
                                          Agents, Delegations, Approvals,
                                          Operations}, PrincipalV5{ID,
                                          Authority []RootCapabilityV5},
                                          AgentV5 (identical shape to
                                          AgentV4), DelegationV5
                                          (identical shape to
                                          DelegationV4, reuses
                                          model.Capability), OperationV5
                                          (byte-identical shape to
                                          OperationV4)

internal/limits/
  limits.go                              — ADD: MaxApprovals var (§21).
                                          One-line, additive.

internal/loader/
  loader.go, loader_v2.go,
  loader_v3.go, loader_v4.go              — UNCHANGED except the one
                                          sanctioned message-text touch
                                          (§5, §27) inside validate(),
                                          validateV2(), validateV3(),
                                          validateV4(); and
                                          LoadDocument's switch gains a
                                          "5" case, Document gains a V5
                                          field.
  loader_v5.go                            — NEW: decodeAndValidateV5,
                                          validateV5 (reuses checkID/
                                          checkScope/checkTarget/
                                          checkAction/resourceLimitErr/
                                          sortErrors verbatim; adds
                                          checkRootCapabilitySetV5, a
                                          variant of checkRootCapabilitySet
                                          that additionally validates each
                                          entry's RequiresApproval pointer
                                          — §6, §16 — while reusing
                                          checkScope/checkTarget and the
                                          existing duplicate-detection
                                          pattern projected onto (scope,
                                          target) only, §6; and
                                          checkApprovals, a new function
                                          validating the approvals[] array
                                          — unknown_approver,
                                          duplicate_approval,
                                          max_approvals — §16).

internal/graph/
  graph.go                                — UNCHANGED. Reused as-is
                                          (§17). Not touched by approvals
                                          at all — approval records never
                                          participate in the delegation
                                          graph (§4.2, §7).

internal/verify/
  verify.go, verify_v2.go,
  verify_v3.go, verify_v4.go              — UNCHANGED.
  verify_v5.go                             — NEW: authState{remaining,
                                          configuredMax, requiresApproval}
                                          (§9), flattenApproval (§9, a
                                          distinctly-named function, not
                                          an overload of verify_v4.go's
                                          flatten — Go does not support
                                          overloading by parameter type,
                                          and the two da maps have
                                          different value types),
                                          RunV5(*model.ModelV5)
                                          report.Result implementing
                                          §10/§12/§17. Reuses
                                          verify_v2.go's/verify_v3.go's
                                          unexported helpers (isSubsetCap,
                                          subtractCap, canonicalizeCaps,
                                          containsCap, heldTargetsForScope,
                                          classifyOne, classifyEdge,
                                          toReportCaps) directly against
                                          flattenApproval's output, same
                                          package, no duplication.

internal/report/
  finding.go                               — UNCHANGED sortKey struct
                                          (§14, the first new finding type
                                          in the project's history that
                                          needs no sortKey field
                                          addition); extend keyOf's type
                                          switch with one new case for
                                          ApprovalFinding. EXISTING
                                          cases/fields untouched.
  capability_finding.go,
  confused_deputy_finding.go,
  delegation_depth_finding.go               — UNCHANGED.
  approval_finding.go                        — NEW: ApprovalFinding,
                                            ViolationApprovalMissing,
                                            ViolationApprovalUnauthorized,
                                            NewApprovalFinding constructor
                                            (§14).
  text.go                                    — extend RenderText's type
                                            switch with the one new
                                            finding type (§19); existing
                                            cases untouched.
  json.go                                     — UNCHANGED (already
                                            generic).

cmd/delegationproof/
  main.go                                      — runVerify's dispatch
                                              switch gains one case
                                              (§18); no new flags, no new
                                              subcommands, no exit-code
                                              changes.

examples/
  billing-refund.json,
  billing-context-binding.json,
  billing-confused-deputy.json,
  billing-redelegation-depth.json               — UNCHANGED.
  billing-approval.json                          — NEW (§22).

schemas/
  model.md                                        — NOT modified this
                                                  session (explicit
                                                  instruction, mirroring
                                                  every prior phase's own
                                                  precedent). The
                                                  implementation session
                                                  must add a "version 5"
                                                  section documenting
                                                  §5/§6/§7/§16, mirroring
                                                  how model.md documents
                                                  version 4 today.

testdata/
  valid-v5/                                        — NEW directory:
                                                  clean-pass-v5.json,
                                                  a reordered-arrays
                                                  variant (permutation
                                                  invariance, including
                                                  reordered approvals[],
                                                  §25),
                                                  combined-violations-v5.json,
                                                  multi-path-approval.json,
                                                  approval-unauthorized.json
                                                  (§23).
  malformed/                                        — ADD v5 fixtures:
                                                  missing-approval-requirement.json,
                                                  non-boolean-approval-requirement.json
                                                  (a JSON decode-error
                                                  case, §6),
                                                  unknown-approver.json,
                                                  duplicate-approval.json,
                                                  approval-field-on-delegation.json
                                                  and
                                                  approval-field-on-operation.json
                                                  (both decode-level
                                                  "unknown field" errors,
                                                  §5),
                                                  duplicate-root-capability-different-approval.json
                                                  (extends the existing
                                                  duplicate_capability
                                                  coverage, §6). Existing
                                                  v1-v4 fixtures
                                                  UNCHANGED, still walked
                                                  automatically by
                                                  cmd/delegationproof/main_test.go.
  golden/                                           — ADD captured v5
                                                  text/json output for
                                                  billing-approval and a
                                                  v5 combined-violations
                                                  fixture. Existing v1-v4
                                                  golden files UNCHANGED,
                                                  byte-identical.

docs/
  phase-5-plan.md                                   — this document.
```

---

## 25. Testing plan

Mirrors the structure of `docs/phase-1-plan.md` §16 /
`docs/phase-2-plan.md` §20 / `docs/phase-3-plan.md` §22 /
`docs/phase-4-plan.md` §24, additive. Test file names follow the existing
`_v4`/`_v3` naming convention (`verify_v5_test.go`, `loader_v5_test.go`,
`main_v5_test.go`).

1. **Full Phase 1-4 regression** — `go test ./... -race -count=1` with
   zero behavioral change to any existing test, **except** the sanctioned
   message-text lines (§5, §27); every existing golden file
   byte-identical; every existing malformed fixture still produces its
   original `ErrorKind`.
2. **Clean pass** — `testdata/valid-v5/clean-pass-v5.json` (§23 item 1)
   → `ALLOW`, exit 0, golden text+json.
3. **`approval_missing` focused violation** — `examples/billing-approval.json`'s
   `void-unapproved` operation (§22) → one `ApprovalFinding` with
   `violation: "approval_missing"`, `declared_approvers: []`, golden
   text+json, exact `reason`/`trace` asserted.
4. **`approval_unauthorized` focused violation** —
   `testdata/valid-v5/approval-unauthorized.json` (§23 item 5) → one
   `ApprovalFinding` with `violation: "approval_unauthorized"`,
   `declared_approvers` containing exactly the non-standing approver(s).
5. **`refund-approved` pass path** — `examples/billing-approval.json`'s
   passing operation (§22) → no finding, confirming a standing-backed
   approval satisfies the gate.
6. **`requires_approval: false` is unaffected by Phase 5** — a v5 model
   where a capability is validly held, correctly bound, requester-backed,
   and `requires_approval: false`, with **zero** approval records in the
   document at all → passes, confirming the approval gate is vacuously
   satisfied and Phase 5 introduces no new failure mode for
   non-approval-required capabilities.
7. **Multiple standing approvers** — a capability with two independent
   approval records, both naming standing-backed approvers → passes
   (existential quantification, §10.2, §11).
8. **Mixed standing and non-standing approvers, one sufficient** — a
   capability with three approval records, only one of which names a
   standing-backed approver → passes (§11's strict distrust: the two
   non-standing records contribute nothing but do not block the one
   valid record either).
9. **All approvers non-standing** — every approval record for a
   capability names a non-standing approver → `approval_unauthorized`,
   with `declared_approvers` listing all of them, sorted, deduplicated.
10. **Multi-path approval requirement, OR-semantics** —
    `testdata/valid-v5/multi-path-approval.json` (§23 item 4, §10.1's
    worked scenario): confirms the node's adopted `requiresApproval` is
    `true` despite a more-permissive root also delivering the same
    capability with greater remaining depth, and confirms `remaining`
    independently still reflects the higher-budget path (the two
    aggregations do not interfere with each other).
11. **Multi-path, both paths agree** — both delivering paths declare the
    same `requires_approval` value (both `true`, and separately both
    `false`) → confirms OR-aggregation degenerates correctly to the
    agreed value in both cases.
12. **Approver reached via a multi-hop delegation chain, not just an
    axiomatic principal declaration** — confirms `flattenApproval(da[approver])`
    correctly reflects a non-principal approver's derived standing.
13. **Approver's only apparent standing arrives via an invalid (presence/
    binding/depth-failed) edge** — confirms `approval_unauthorized` fires
    correctly, since the approver's `DA` never actually contained the
    capability (§11, mirroring `docs/phase-3-plan.md` §16's item 10 for
    confused-deputy).
14. **Unknown approver** — `approver` referencing a nonexistent node id →
    `unknown_approver`, exit 2 (`validate` and `verify` both).
15. **Missing approver** — `approver` key omitted entirely → decodes as
    `""`, falls into `unknown_approver` (§16) — dedicated fixture
    confirming no separate error kind fires.
16. **Missing `requires_approval`** — `nil` pointer (key omitted) →
    `missing_approval_requirement`, exit 2.
17. **Non-boolean `requires_approval`** — a JSON string/number value in
    that position → decode-level `ParseError` (§6), not a `validateV5`
    error kind.
18. **Duplicate approval record** — two `approvals[]` entries sharing the
    exact `(approver, scope, target)` triple → `duplicate_approval`, exit
    2. A dedicated adjacent fixture confirms two entries sharing only
    `scope`/`target` but differing `approver` is **not** rejected.
19. **Duplicate root capability with differing `requires_approval`
    values** — extends the existing `duplicate_capability` coverage
    (§6): two entries sharing `(scope, target)` but disagreeing on
    `requires_approval` (with `max_delegation_depth` agreeing or not) →
    `duplicate_capability`, exit 2.
20. **Stray approval field on a delegation or operation entry** — decode-
    level "unknown field" error via `DisallowUnknownFields`, exercised
    through the full CLI path (exit 2).
21. **Resource limits** — `limits.MaxApprovals` white-box test (lowered
    value, same pattern as every existing `internal/limits`-based test):
    exact-at-boundary acceptance and over-limit rejection
    (`resource_limit_exceeded`, `Primary: "max_approvals"`).
22. **Combined violation precedence (operation-level, all four
    precedence-table rows involving approval)** — dedicated table test
    covering every row of §12's precedence table that mentions approval:
    actor-amplification masks a co-declared approval issue,
    confused-deputy masks a co-declared approval issue, and the
    genuine `approval_missing`/`approval_unauthorized` rows.
23. **Depth and approval independence** — a document where an unrelated
    capability's edge fails `delegation_depth_violation` while a
    different, unrelated capability's operation independently fails
    `approval_missing` → both findings present, correctly ordered, no
    interference between the two orthogonal detection points (§12).
24. **Combined-violation fixture** — `testdata/valid-v5/combined-violations-v5.json`
    (§23 item 3): asserts all six possible finding kinds
    (`authority_amplification`, `context_binding_violation`,
    `confused_deputy`, `delegation_depth_violation`, `approval_missing`,
    `approval_unauthorized`) present simultaneously, correctly classified,
    correctly ordered, with no duplicate finding for any single edge or
    operation.
25. **Deterministic findings / sort order** — a v5 model with multiple
    `ApprovalFinding`s sharing `(actor, action)` but differing only by
    `requires`/`target`/`requester`, asserting the existing 6-tuple sort
    key (§14, unmodified) produces a stable, documented order.
26. **Reordered-input invariance** — v5 analogue of the existing
    permutation-invariance test: byte-identical output for
    semantically-equivalent reordered `principals`/`agents`/
    `delegations`/`operations`/`approvals` arrays (including reordered
    `approvals[]` entries themselves, confirming §10.2's sort+dedupe
    design is genuinely order-independent).
27. **Repeated-run byte determinism** — v5 analogue of
    `TestRunIsDeterministicAcrossRepeatedInvocations`.
28. **Deterministic traces** — asserts `ApprovalFinding.Trace` ends with
    `action` and matches the same `CanonicalTrace` convention every prior
    operation-level finding uses (§15).
29. **Text output** — golden-file test for the worked example (§22) and
    the combined-violations fixture.
30. **JSON output** — golden-file test for the same fixtures; asserts the
    envelope shape is unchanged and `ApprovalFinding` fields appear in
    the documented order.
31. **CLI exit codes** — `validate` vs `verify` divergence for a v5 model
    containing only an `approval_missing` finding (structurally valid,
    `validate` → 0, `verify` → 1), mirroring the existing divergence
    tests.
32. **stdout/stderr split** — v5 error and finding cases confirm the
    unchanged split (result output only on stdout, diagnostics only on
    stderr).
33. **No-panic malformed-input behavior** — extend the existing
    fuzz/mutation-style CLI test to include v5 fixtures as seeds,
    including truncated/mutated `requires_approval` and `approvals[]`
    byte sequences.

---

## 26. Phase 1-4 regression requirements

- Every existing test in `internal/loader`, `internal/graph`,
  `internal/verify`, `internal/report`, and `cmd/delegationproof` must
  pass, with **exactly one documented class of exception**: the literal
  invalid-version message strings in `validate`, `validateV2`,
  `validateV3`, and `validateV4` (and their corresponding assertions in
  `internal/loader/loader_v2_test.go`) change to include `"5"` (§5). This
  is the only sanctioned edit to any pre-existing test file.
- Every existing golden file in `testdata/golden/` must remain
  byte-identical output for its existing input.
- Every existing fixture in `testdata/malformed/` must continue to
  produce its documented `ErrorKind`.
- `examples/billing-refund.json`, `examples/billing-context-binding.json`,
  `examples/billing-confused-deputy.json`, and
  `examples/billing-redelegation-depth.json` must continue to round-trip
  exactly as their respective plan documents specify.
- No line in `internal/verify/verify.go`, `internal/verify/verify_v2.go`,
  `internal/verify/verify_v3.go`, `internal/verify/verify_v4.go`,
  `internal/graph/graph.go`, `internal/report/capability_finding.go`,
  `internal/report/confused_deputy_finding.go`,
  `internal/report/delegation_depth_finding.go`, or any existing
  `internal/model` type may change.
- `go vet ./...`, `gofmt -l .`, and `go build -o bin/delegationproof
  ./cmd/delegationproof` must all succeed exactly as CLAUDE.md requires
  today, with the new v5 files included.

---

## 27. Security assumptions

Extends `docs/phase-1-plan.md` §17, `docs/phase-2-plan.md` §22,
`docs/phase-3-plan.md` §24, and `docs/phase-4-plan.md` §26 without
weakening any of them:

- **A declared approval record is a declared fact by the document's
  author, not a verified real-world sign-off event.** Exactly as a
  `requester` value is a declared label rather than an authenticated
  caller (Phase 3), and a `max_delegation_depth` is a policy assertion
  rather than a verified redelegation history (Phase 4), DelegationProof
  does not verify that a real compliance officer, manager, or other
  real-world party actually gave the sign-off a document's `approvals[]`
  entry claims — that correspondence (real approval-workflow provenance,
  e.g. from an actual ticketing/change-management system, mapped into
  this document's `approvals` array) is a separate, later integration
  concern (§31), identical in kind to every prior phase's own
  security-assumptions boundary.
- **Approval Preservation proves a property of the declared model only:**
  "this document never claims a validly-authorized, correctly-bound,
  requester-backed operation on an approval-required capability may
  proceed without at least one declared, standing-backed approval." It
  does not, and cannot, prove that a real running system actually
  requires or enforces that sign-off at runtime — DelegationProof remains
  a static, offline analyzer with no interception or enforcement
  component (unchanged, Phase 1 §17/§18).
- **An approver's independent standing is verified structurally (against
  the same `DA(n)` every other node's standing is verified against), but
  the approver's real-world identity is not.** This is the same boundary
  Phase 3 already draws around `requester`, applied to `approver`: the
  document asserts *who* approved, and DelegationProof verifies that
  named party *could* legitimately approve (by holding the capability
  independently), but not that the named party is who they claim to be
  in a real system.
- **No new attack surface from parsing.** `requires_approval` and
  `approvals[]` entries are decoded via the same `encoding/json` +
  `DisallowUnknownFields` + bounded-read pipeline as every other field,
  subject to the same `MaxInputFileSize` bound applied before any
  structural field is read. The `*bool` pointer type (§5) introduces no
  unbounded allocation risk, and `approvals[]`'s size is bounded by
  `MaxApprovals` (§21) exactly as `operations[]` is bounded by
  `MaxOperations`.

---

## 28. Explicit non-goals

All of `docs/phase-1-plan.md` §18's, `docs/phase-2-plan.md` §23's,
`docs/phase-3-plan.md` §25's, and `docs/phase-4-plan.md` §27's non-goals
continue to apply. Phase 5 additionally, explicitly, does **not**
include:

- MCP protocol implementation, A2A protocol implementation, OAuth, JWT
  verification, tokens, networking, hosted services, proxying, runtime
  enforcement, databases, LLMs, Z3/SAT/SMT, SARIF, revocation, sessions.
- **Temporal/state-based approval workflow** (`pending`/`approved`/
  `rejected` transitions, approval expiry, approval timestamps). Evaluated
  directly (§3, Option C) and rejected for Phase 5: a static declared
  fact ("this approval record exists, and its named approver has
  standing") is sufficient for the invariant motivating this phase
  (§2, §8), and is the deliberate choice that keeps Phase 5 inside the
  single deterministic topological pass every prior phase already fits
  within, avoiding the state-space-exploration trigger
  `docs/phase-1-plan.md` §8.2 identifies. A later phase may reconsider
  this on its own merits (§31) without redesigning anything Phase 5
  establishes.
- **State-space exploration / general search** — confirmed unnecessary
  (§17, §0); a single deterministic topological pass plus one bounded
  pre-indexing pass over `approvals[]` suffices.
- **Multi-approver quorum/threshold requirements** (`requires_approval_count:
  N`). Evaluated (§3, Option E) and rejected: existential (one
  standing-backed approval suffices) is the minimal model the motivating
  threat requires.
- **Per-operation-scoped or per-actor-scoped approvals** (Option D, §3).
  Evaluated and rejected: capability-level scoping matches the
  granularity of every prior phase's own additions (`target` in Phase 2,
  `max_delegation_depth` in Phase 4).
- **Self-approval / separation-of-duties prohibition.** Evaluated (§7)
  and deliberately not enforced: the standing requirement already
  provides real protection; disallowing self-approval on top of it is a
  separable policy decision with no motivation in the current threat
  model.
- **Approval-gated delegation** (requiring approval before re-delegating
  a capability, not just before exercising it). Evaluated (§4.2) and
  rejected for Phase 5's minimal scope, mirroring Phase 4 §7's identical
  rejection of explicit edge-level depth attenuation: nothing in the
  motivating threat (§2) requires gating transmission, only exercise.
- **Wildcard scopes, wildcard targets, hierarchical IAM, target
  registry** — unchanged, still rejected.
- **Explicit per-edge depth attenuation** — remains exactly as scoped by
  `docs/phase-4-plan.md` §27, untouched by this phase.
- **Multi-hop request/induced-by chains** — remains exactly as scoped by
  `docs/phase-3-plan.md` §25, untouched by this phase.
- **Real-world approval-workflow correspondence** (verifying that a
  document's declared `approvals[]` entries match a real system's actual
  sign-off history) — a topology-ingestion concern, not a
  verification-core concern, symmetric with Phase 3's identical boundary
  around `requester` and Phase 4's around `max_delegation_depth` (§27).
- Web UI, automatic policy generation, CI-vendor integration.
- Phase 6 implementation.

---

## 29. Acceptance criteria

- `go build ./...` succeeds; `go.mod` remains stdlib-only.
- `go vet ./...` is clean; `gofmt -l .` reports nothing.
- `go test ./... -race -count=1` passes, including every category in
  §25, with the documented, sanctioned test-string changes (§26) and
  zero other modification to any pre-existing test file.
- Every existing `testdata/golden/` file is unchanged, byte-identical.
- A version-1, version-2, version-3, or version-4 document produces
  byte-identical `validate`/`verify` output, on both `text` and `json`
  formats, to the current `main` branch today.
- A version-5 document with no violations → `ALLOW`, exit 0.
- `examples/billing-approval.json` → exactly the two-operation shape
  described in §22 (one pass, one `approval_missing`), matching the
  worked example.
- A version-5 document containing `authority_amplification`,
  `context_binding_violation`, `confused_deputy`,
  `delegation_depth_violation`, `approval_missing`, and
  `approval_unauthorized` findings simultaneously reports all six,
  correctly classified, correctly ordered, with no duplicate finding for
  any single edge or operation (§12).
- `missing_approval_requirement`, `unknown_approver`, and
  `duplicate_approval` each have at least one dedicated malformed
  fixture and table-driven test case.
- The multi-path OR-semantics for `requires_approval` (§10.1) is
  confirmed by a dedicated test distinguishing it from the max-based
  aggregation used for `remaining`/`configuredMax`.
- No panic is reachable from `main` for any version-1 through version-5
  input within the (documented) resource bounds.

---

## 30. Definition of DONE

Phase 5 is done when:

1. All items in §29 are met.
2. The file/package layout matches §24, or a documented deviation is
   noted in this document, keeping it authoritative per every prior
   phase's own convention.
3. The new error kinds (§16) and every new finding `violation`/`point`
   combination (§14) has at least one dedicated test.
4. The worked example (§22) is reproducible verbatim via
   `delegationproof verify examples/billing-approval.json`.
5. `schemas/model.md` has been updated (or a sibling v5 document added)
   by the implementation session to describe the version-5 shape —
   noted as deferred in §24, not done in this planning session, per
   explicit instruction not to modify it now.
6. No open TODOs remain in code for functionality this document describes
   as in-scope; TODOs for §31's deferred items are fine and expected,
   linking back to §31.
7. `docs/phase-1-plan.md`, `docs/phase-2-plan.md`, `docs/phase-3-plan.md`,
   and `docs/phase-4-plan.md` are unmodified — Phase 5 attaches to all
   four, per their own future-phase-boundary sections, without rewriting
   any of them.

---

## 31. Future-phase boundary

Carried forward from `docs/phase-1-plan.md` §21, `docs/phase-2-plan.md`
§26, `docs/phase-3-plan.md` §28, and `docs/phase-4-plan.md` §30, still
deferred, now with Phase 5's addition noted where it changes the shape of
what attaches:

- **Temporal/state-based approval workflow** (newly sharpened by this
  phase, §28): if a later product need genuinely requires modeling
  approval as a state machine (pending/approved/rejected, with expiry or
  revocation), that is new scope requiring the temporal/conditional
  structure `docs/phase-1-plan.md` §8.2 identifies as the actual trigger
  for bounded state-space exploration — the first genuinely motivated
  candidate for that framing across all five phases so far. It would
  layer onto, not replace, this phase's static "approval record + standing
  check" foundation: a temporal layer would determine *whether* a given
  static approval record is currently active, while the standing check
  defined here (§8, §12) would remain the mechanism for verifying *who*
  may give it.
- **Multi-approver quorum/threshold requirements** (§28): if a later
  phase demonstrates a need for "N independent standing-backed approvals
  required," that generalizes §12's existential check to a counting
  check over the same `standingApprovers` set already computed (§17) —
  additive, not a redesign.
- **Approval-gated delegation** (§28): if a later phase demonstrates a
  need to require approval before re-delegating (not just exercising) an
  approval-required capability, that is a new edge-level check layered
  onto Phase 4's existing three-tier edge precedence (§12), analogous to
  how depth itself was added as edge-level tier three in Phase 4.
- **Self-approval / separation-of-duties enforcement** (§28): a
  structural or semantic prohibition on `approver == actor` or `approver
  == requester` could be added as an additional condition in §12 step 3
  without altering anything about how standing is computed.
- **Explicit per-edge depth attenuation, multi-hop request/induced-by
  chains, scope/target wildcard or hierarchy semantics**: unchanged from
  Phase 3/4's own deferrals; nothing in Phase 5 accelerates or blocks any
  of them.
- **MCP/A2A ingestion, JSON Schema enforcement, SARIF, Z3/SMT**:
  unchanged from Phase 1 §21; nothing in Phase 5 accelerates or blocks
  any of them.
- **Real-world approval-workflow correspondence** (identified in this
  phase, §27): verifying that a document's declared `approvals[]` match
  a real system's actual sign-off history is a topology-ingestion
  concern, symmetric with Phase 3's and Phase 4's identical postures
  toward real request-provenance and real redelegation-count
  correspondence. This phase defines what to check once an approval is
  declared; it does not address how a real system's declarations get
  produced truthfully.

---

## 32. Implementation sequence

Recommended order for the future Phase 5 implementation session,
following the same bottom-up dependency order every prior phase's own
architecture plan implies (types before loader, loader before verify,
verify before report/CLI wiring):

1. `internal/model/types_v5.go` — `RootCapabilityV5`, `ApprovalV5`,
   `ModelV5`, `PrincipalV5`, `AgentV5`, `DelegationV5`, `OperationV5`
   (§5, §24).
2. `internal/limits/limits.go` — add `MaxApprovals` (§21, §24).
3. `internal/loader/loader_v5.go` — `decodeAndValidateV5`, `validateV5`,
   `checkRootCapabilitySetV5`, `checkApprovals`, new `ErrorKind`
   constants (§16, §24); update `LoadDocument`'s switch and `Document`
   struct in `loader_v2.go`; make the four sanctioned message-text
   touches in `validate`/`validateV2`/`validateV3`/`validateV4` (§5,
   §26).
4. `internal/report/approval_finding.go` — `ApprovalFinding`,
   `ViolationApprovalMissing`, `ViolationApprovalUnauthorized`,
   `NewApprovalFinding` constructor (§14); extend `finding.go`'s `keyOf`
   switch with one new case (§14, §24).
5. `internal/verify/verify_v5.go` — `authState`, `flattenApproval`,
   `RunV5` implementing §10/§12/§17 (§24).
6. `internal/report/text.go` — extend `RenderText`'s type switch (§19).
7. `cmd/delegationproof/main.go` — extend `runVerify`'s dispatch switch
   (§18).
8. `examples/billing-approval.json` — the worked example (§22).
9. `testdata/malformed/` — one fixture per new §16 error kind, plus the
   decode-level and stray-field cases (§24).
10. `testdata/valid-v5/` — clean-pass, reordered-arrays, combined-
    violations, multi-path, approval-unauthorized fixtures (§23, §24).
11. `testdata/golden/` — captured text/json output for the worked example
    and combined-violations fixture, generated from the built binary and
    diffed for intent, per CLAUDE.md's own instruction for golden-file
    changes (§24).
12. `internal/loader/loader_v2_test.go` — the one sanctioned test-string
    update (§5, §26).
13. Full test suite per §25's matrix, across `loader_v5_test.go`,
    `verify_v5_test.go`, `report`'s existing test files, and
    `main_v5_test.go`.
14. `schemas/model.md` — add the version-5 section (§30 item 5; deferred
    per this planning session's own instruction not to touch it now, but
    sequenced last in the implementation session for completeness).
15. Final verification: `gofmt -l .`, `go vet ./...`, `go test ./...
    -race -count=1`, `go build -o bin/delegationproof
    ./cmd/delegationproof` — all per CLAUDE.md's standing requirement,
    confirming every item in §29's acceptance criteria.
</content>
