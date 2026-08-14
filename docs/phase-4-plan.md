# DelegationProof — Phase 4 Plan

Status: PLANNING ONLY. Phase 1, Phase 2, and Phase 3 are implemented,
merged, and untouched by this document. This is the authoritative design
contract for the Phase 4 implementation session, in the same spirit as
`docs/phase-1-plan.md`, `docs/phase-2-plan.md`, and `docs/phase-3-plan.md`.

Module: `github.com/SamudralaAjaykumarrr/delegationproof`

---

## 0. Phase 4 rationale

Phase 1 proved Authority Non-Amplification: does a node exercise or
transmit a scope it was never validly granted? Phase 2 proved
Context-Binding Preservation: is a validly-granted scope being exercised
against the target it was granted for? Phase 3 proved Requester
Authorization Preservation: does the party an operation is actually
performed *for* independently hold the capability being exercised? All
three invariants share one property: they ask whether authority reaching
a node is *real*, not whether it has traveled *too far*. None of them can
express, and therefore none can check, a fourth and different question:
*this authority is real, and the delegator legitimately holds it — but
has it already been re-delegated more times than its origin ever
permitted?*

`docs/phase-1-plan.md` §21 named this "delegation-depth enforcement"
(product idea #5) and explicitly distinguished it from the §12
`MaxChainDepth` resource-safety valve: "a *policy* field (a declared
max-depth) distinct from the resource-bound safety valve... needs its own
finding kind and its own semantics for where the limit is declared."
`docs/phase-2-plan.md` §26 and `docs/phase-3-plan.md` §28 both carried
this forward, untouched, waiting for a phase motivated to design it
properly rather than bolt it on as an afterthought. Phase 4 is that
phase.

The threat is excessive **re-delegation**, not amplification: `admin`
legitimately owns `billing:refund@billing-service` and legitimately
delegates it to `billing-agent` — Phase 1/2/3 all correctly say ALLOW.
The problem appears only when `billing-agent` re-delegates that same
capability one hop further, to `support-agent`, and the origin grant
never intended authority to travel that far. Phase 4's job is to add the
smallest rigorous concept that lets a root grant declare "this may be
delegated, but only N hops from me," and to prove that invariant
statically, deterministically, and compositionally with the three
invariants already proven. Everything else — approvals, revocation,
temporal state, explicit per-edge attenuation, MCP/A2A ingestion — remains
future work (§27).

---

## 1. Phase 1-3 baseline

Verified against the actual merged implementation on `main`
(commit `9222d8d`), not just the plan documents:

- **Model types**: `internal/model/types.go` (v1), `types_v2.go` (v2,
  `Capability{Scope, Target}`), `types_v3.go` (v3, `ModelV3` = `ModelV2`
  shape + `OperationV3.Requester`). All three schemas are structurally
  disjoint Go types sharing no struct, per `docs/phase-2-plan.md` §9's and
  `docs/phase-3-plan.md` §5's explicit "no shared internal model type"
  discipline.
- **Loader dispatch**: `internal/loader/loader_v2.go`'s `LoadDocument`
  peeks `{"version": string}` permissively, then dispatches `"1"`/`"2"`/`"3"`
  to `decodeAndValidateV{1,2,3}`, anything else → one `KindInvalidVersion`
  error with message `` `version must be "1", "2", or "3", got %q` ``.
  `Document{V1, V2, V3}` union, exactly one field set.
- **Graph**: `internal/graph/graph.go` — `TopoSort` (Kahn's algorithm,
  min-heap lexicographic tie-break), `LongestPath` (DAG DP over
  topological order), `CanonicalTrace` (BFS from all principals, sorted
  expansion, first-path-wins, `[]string{actor}` if unreachable). All
  operate purely on node ids and `[]graph.Edge{From, To}` — no dependency
  on what a node's derived authority contains. Untouched by any phase so
  far, reusable as-is by Phase 4.
- **Verify**: `internal/verify/verify_v3.go`'s `RunV3(*model.ModelV3)` —
  one topological pass builds `da map[string][]model.Capability` for every
  node (principals get their declared set; an agent's incoming edge
  contributes its whole capability array only if that array is a subset
  of the delegator's own `da` entry — strict distrust, all-or-nothing).
  `classifyEdge`/`classifyOne` (`docs/phase-2-plan.md` §8) turn a missing
  capability into `authority_amplification` or `context_binding_violation`.
  Operation evaluation then applies `docs/phase-3-plan.md` §8's
  three-step precedence: actor-side check first (masks everything else if
  it fails), then requester-side check (`confused_deputy` if it fails),
  else ALLOW.
- **Report**: `internal/report/finding.go`'s `sortKey{point, subject,
  secondary, scope, target, requester}` — already a 6-tuple, added to
  incrementally by each phase, each addition a strict extension (empty for
  older finding shapes). `capability_finding.go` (`Capability`,
  `CapabilityEdgeFinding`, `CapabilityOperationFinding`).
  `confused_deputy_finding.go` (`ConfusedDeputyFinding`). `RenderText`/
  `RenderJSON` both switch on finding concrete type; `RenderJSON`'s
  envelope is generic over `[]interface{}`.
- **CLI**: `cmd/delegationproof/main.go`'s `runVerify` dispatch:
  `switch { case doc.V1 != nil: ...; case doc.V2 != nil: ...; case doc.V3
  != nil: ... }`.
- **Limits**: `internal/limits/limits.go` — all bounds are exported
  `var`s. `MaxChainDepth = 64` is explicitly a **resource-safety valve**
  against pathological graph shape, not a policy invariant
  (`docs/phase-1-plan.md` §12, reaffirmed by §21: "not the future
  policy-level depth-limit invariant... the two are conceptually different
  ... and must not be conflated"). Phase 4 must respect this distinction,
  not erase it.
- **Tests**: `internal/loader/loader_v2_test.go`'s message-text assertion
  from Phase 2/3's own sanctioned touch points is the only precedent for
  editing a pre-existing test string; Phase 4 introduces a brand-new
  version literal (`"4"`) through the identical dispatch mechanism and
  requires the identical one-line message update, this time in three call
  sites (`validate`, `validateV2`, `validateV3`) updating to a four-version
  message, with `validateV4` introducing the fourth copy.

Phase 4 must not modify any Phase 1, Phase 2, or Phase 3 production code
path, and must touch only the sanctioned message-text lines identified
above.

---

## 2. Delegation-depth threat

Phase 1 answers: *does `support-agent` hold `billing:refund`?* Phase 2
answers: *is that `billing:refund` valid for `billing-service`?* Phase 3
answers: *does whoever is inducing this operation independently hold
standing?* None of them can answer: *how many times has this specific
grant already been passed along, and was that within what its origin
allowed?*

Concretely: `admin` declares `billing:refund@billing-service` with a
redelegation budget of exactly one hop. `admin → billing-agent` is a
legitimate first hop — `billing-agent` may use the capability and Phase
1/2/3 all correctly ALLOW it. But `billing-agent → support-agent` is a
*second* hop from the same origin grant, and the origin never authorized
that. Phase 1/2/3 have no vocabulary to distinguish "the capability
`billing-agent` holds, re-delegated one hop further" from "a capability
delegated for the first time" — both look like an ordinary, subset-valid
delegation edge. The result is real over-permissioning masquerading as a
perfectly ordinary chain: every individual edge in the chain is
individually valid by Phase 1-3's own invariants, yet the chain as a whole
has silently exceeded a limit its origin declared. This is the classical
delegation-depth / re-delegation-bounding problem
(`docs/phase-1-plan.md` §21, product idea #5), and Phase 4's entire job is
to make it checkable.

---

## 3. Minimal new abstraction

Evaluated per the task's candidate list (§1 of the brief):

| Candidate | Verdict | Why |
|---|---|---|
| **A. Capability grant `{scope, target, max_delegation_depth}`** | **Chosen, in modified form** | The depth budget is a property of the *origin declaration* — it answers "how far may this grant travel," a fact that only makes sense to state once, at the root. Adding it as a third field on the existing capability tuple, but *only where a root grant is declared* (a principal's authority entry), is minimal: one new atomic field, no new entity, no new edge kind. |
| **B. Delegation edge `{scope, target, remaining_depth}`** | Rejected as a *declared* field; adopted as a *derived* quantity | Making `remaining_depth` an explicit, author-supplied field on every delegation edge would let a document author declare a budget independent of the actual chain — an edge could claim `remaining_depth: 5` regardless of what its delegator was actually granted, which is not a constraint at all, it's an unenforceable assertion. The right formulation, developed in §4, is that remaining depth is *derived* by the verifier from the root declaration and hop count — never authored on the edge itself (see §7's rejection of explicit edge attenuation). |
| **C. Root authority declaration carries a redelegation budget** | Folded into A | This is the same decision as A, stated at the principal-declaration level rather than the capability-tuple level. Attaching the budget to the *capability tuple within* the principal's declared authority (rather than one budget governing the whole principal) is strictly more expressive at no extra cost: a principal legitimately holding two different capabilities with two different re-delegation policies (e.g. `billing:refund` non-delegable, `billing:read` delegable three hops) needs per-capability granularity, exactly the reasoning `docs/phase-2-plan.md` §4 used to reject one-target-per-edge in favor of per-tuple targets. |
| **D. Another formal representation (e.g. a depth-budget registry, a policy object referenced by id)** | Rejected | Would require a new top-level document section purely so other fields could reference it — the exact shape of complexity `docs/phase-2-plan.md` §5 already rejected for a target registry, for identical reasons: nothing about this invariant needs indirection through a named policy object. |

**Decision: exactly one new field, `max_delegation_depth`, added to the
capability tuple, but only in the type used for a *root* grant** — i.e.
only on entries in a principal's `authority` array. Delegation edges and
operations continue to use the existing, byte-identical `Capability
{Scope, Target}` shape from `types_v2.go`; nothing is added to
`DelegationV4` or `OperationV4`'s schema at all (§5). This mirrors exactly
how Phase 3 added `requester` to `Operation` alone, touching no other
entity's shape, and how Phase 2 added `target` to the capability tuple
rather than to the edge or the principal.

A new Go type, `model.RootCapability{Scope, Target, MaxDelegationDepth}`,
is introduced solely for `PrincipalV4.Authority`. Everywhere else in a
v4 document — delegation `authority` arrays, operation `requires`/`target`
— the plain, unmodified `model.Capability{Scope, Target}` type continues
to apply. This is deliberate, not an oversight: `max_delegation_depth` is
never re-declared or re-asserted at a delegation edge (§7); it exists in
exactly one place per capability, at its origin.

---

## 4. Delegation-budget semantics

### 4.1 Is depth part of a capability's identity?

**No.** `(scope, target)` remains the sole identity of a capability, exactly
as Phase 2 established. `max_delegation_depth` is *metadata attached to a
capability's origin declaration*, not a fourth identity component. This
matters concretely: if depth were part of identity, the existing
subset-validity check (`A ⊆ DA(d)`) would require an *exact* depth match
between what a delegator holds and what it re-delegates, which is
backwards — a delegator holding a capability at remaining budget 2 must be
able to re-delegate it at remaining budget 1 (one hop consumed), and a
subset check keyed on exact depth would make that impossible without
inventing a separate "depth compatibility" relation on top of set
inclusion. Keeping `(scope, target)` as the sole identity means the
existing presence/binding subset check (`isSubsetCap`, `classifyEdge`,
`classifyOne` — Phase 2/3, entirely unmodified) continues to answer *is
this capability held at all, and for the right target* exactly as before;
depth is a wholly separate, additional dimension of state checked only
once presence is already established (§9, §12).

### 4.2 Answering the task's explicit questions (§2 of the brief)

- **Is the root holder depth 0?** No. The root principal's *remaining*
  redelegation budget for a capability equals the capability's own
  declared `max_delegation_depth` — the full budget, not zero. "Depth 0"
  in this design always means *budget exhausted*, never "root position."
  Framing it as a hop-count-from-root would force every downstream
  consumer of a finding to do subtraction to find out what actually
  matters (how much budget is left); framing it as remaining budget
  directly answers the operationally relevant question at every node.
- **Does max depth 0 mean non-delegable?** Yes. A capability declared with
  `max_delegation_depth: 0` is usable by the principal itself (should the
  principal ever be an operation's actor — legal per
  `docs/phase-1-plan.md` §7.2) but any outgoing delegation edge attempting
  to grant it is invalid from the first hop.
- **Does max depth 1 permit exactly one outgoing delegation?** Yes.
  Remaining budget starts at 1 at the root; the first delegatee receives
  it at remaining budget 0 (usable, non-delegable further); any second
  hop is rejected.
- **How is remaining depth calculated?** `remaining(root, c) =
  c.max_delegation_depth` (the declared value). For an agent `a` reached
  by a valid delegation edge from `d` carrying `c`: `remaining(a, c) =
  remaining(d, c) - 1`, using whichever valid incoming edge yields the
  largest such value when more than one path delivers `c` (§10 multi-path
  semantics).
- **Does an operation consume depth?** No, explicitly. Exercising a
  capability is not delegating it; §13 states and justifies this
  precisely for the requester side, and the same reasoning applies
  symmetrically to the actor side — using authority and transmitting
  authority are different acts, and only the latter is metered.
- **Does requester usage consume depth?** No — see §13. A requester is
  never a delegator in the edge sense; naming a requester on an operation
  creates no delegation edge and therefore cannot decrement anyone's
  budget.

---

## 5. Schema v4

**Decision: a new schema version literal, `"4"`, decoded into a new,
structurally disjoint `model.ModelV4`.** Identical reasoning to
`docs/phase-2-plan.md` §9 and `docs/phase-3-plan.md` §5: `version` is
checked by hard equality specifically so a new semantic shape never
silently reinterprets old or new documents under the wrong rules.

`ModelV4` is `ModelV3` (which is itself `ModelV2` + `Requester`) with
exactly one structural change: **principals' declared authority entries
gain a required `max_delegation_depth` integer.** Agents, delegations, and
operations are byte-for-byte identical in shape to their v3 counterparts.

```json
{
  "version": "4",
  "principals": [
    {
      "id": "admin",
      "authority": [
        { "scope": "billing:refund", "target": "billing-service", "max_delegation_depth": 1 },
        { "scope": "billing:read",   "target": "billing-service", "max_delegation_depth": 5 }
      ]
    }
  ],
  "agents": [
    { "id": "billing-agent" },
    { "id": "support-agent" }
  ],
  "delegations": [
    { "delegator": "admin",         "delegatee": "billing-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] },
    { "delegator": "billing-agent", "delegatee": "support-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin", "action": "refund-x", "requires": "billing:refund", "target": "billing-service" },
    { "actor": "support-agent", "requester": "admin", "action": "refund-y", "requires": "billing:refund", "target": "billing-service" }
  ]
}
```

**`delegations[].authority` entries and `operations[]` are unchanged from
v3.** No `max_delegation_depth` field exists anywhere except inside a
principal's `authority` array (§3, §7). This is the direct schema
consequence of choosing "inherited-only decreasing depth" over "explicit
edge attenuation" (§7).

**`max_delegation_depth` is required, with no default and no "unbounded"
sentinel.** Considered and rejected, per the task's own steer (§11 of the
brief) and identical precedent to Phase 3's rejection of a defaulted
`requester` (`docs/phase-3-plan.md` §5): an optional field defaulting to
"unlimited" would reintroduce exactly the silent-reinterpretation ambiguity
`DisallowUnknownFields` and strict-decode exist to prevent, and a magic
"infinite" sentinel (`-1`, `null`, `"unbounded"`) is explicitly barred by
the brief's own §11 steer ("Should Phase 4 deliberately NOT support
infinity? ... Prefer bounded, explicit values"). A document author who
wants an effectively-unconstrained capability declares a large, finite,
in-bounds value (up to `limits.MaxDelegationDepth`, §15) — the same
posture Phase 1 §12 already takes toward "no field ever means unlimited."

**Dispatch mechanism**, extending `LoadDocument`'s switch:

```
"1"          -> decodeAndValidateV1 (unchanged)
"2"          -> decodeAndValidateV2 (unchanged)
"3"          -> decodeAndValidateV3 (unchanged)
"4"          -> decodeAndValidateV4 (new)
anything else (including "") -> one KindInvalidVersion error
```

`Document` grows a fourth field: `Document{V1, V2, V3, V4 *model.ModelV4}`,
exactly one of which is set on success. The `invalid_version` message
updates from `` `version must be "1", "2", or "3", got %q` `` to
`` `version must be "1", "2", "3", or "4", got %q` ``, in the four call
sites that must stay textually identical (`validate`, `validateV2`,
`validateV3`'s existing three copies updated; `validateV4` introduces the
fourth). `internal/loader/loader_v2_test.go`'s asserted literal must be
updated to match, the same sanctioned single-line touch each prior phase
has made.

---

## 6. Root authority semantics

- **`max_delegation_depth` is mandatory on every `RootCapability` entry.**
  No optional form, no field omission (§5).
- **No unbounded/infinite value is representable.** Rejected outright
  (§5). Every value is a finite non-negative integer.
- **Range: `0 ≤ max_delegation_depth ≤ limits.MaxDelegationDepth`** (§15).
  `0` is legal and means non-delegable-but-usable (§4.2). Negative values
  are a structural error (§17).
- **Representation for "missing" detection.** Because `0` is a legitimate,
  meaningful declared value — unlike Phase 2/3's `target`/`requester`,
  where a missing key decodes to `""` and `""` can never be a valid value
  — an `int` field cannot distinguish "author wrote `0`" from "author
  wrote nothing." `model.RootCapability.MaxDelegationDepth` is therefore
  typed `*int` (a pointer), so `nil` unambiguously means "key absent" and
  is rejected by `validateV4` as a distinct case from a present, explicit
  `0` (§17). This is the one place in the whole v1-v4 schema family where
  a pointer type is required instead of a sentinel value, precisely
  because — uniquely among every field added by any phase so far — the
  field's own zero value is a legal, distinct, meaningful declaration
  rather than an obviously-invalid placeholder.
- **Non-integer JSON values (e.g. `1.5`, `"1"`) are not a `validateV4`
  concern at all.** `encoding/json` decoding a non-integer JSON number (or
  a string) into `*int` fails at the decode step, surfacing as the
  existing `LoadError.ParseError` path (`invalid JSON: ...`) — identical
  in kind to any other field-type mismatch already handled by strict
  typed decoding today, requiring no new code (§17).

---

## 7. Delegation-edge semantics

**Decision: inherited-only decreasing depth. No explicit edge-level
depth/attenuation field.** The task's §12 explicitly raises "delegation
attenuation" (a delegator choosing to grant a shorter remaining budget
than it actually holds) as a candidate and instructs evaluating it
directly. Evaluated and **rejected for Phase 4**:

- It is not required by the stated threat model (§2): bounding total
  distance from origin is exactly what "may be delegated only N hops"
  means, and inherited-only decrement achieves that completely on its own.
- It would require a new field on `DelegationV4.Authority` entries (which
  currently, and would otherwise continue to, use the plain, unmodified
  `Capability{Scope, Target}` type shared with v2/v3), expanding the
  schema at a second location instead of one, and reintroducing exactly
  the "same version, richer meaning in one place but not the other"
  ambiguity Phase 2/3 both avoid.
- It raises real unresolved semantic questions the brief itself flags as
  optional ("could be valuable... may expand Phase 4 unnecessarily") and
  that this document is instructed to resolve toward the smaller model
  absent a demonstrated need: e.g., can attenuation only shrink, never
  grow, remaining budget (presumably yes, but that itself is a new
  invariant requiring its own edge-level check, on top of everything
  already being added).
- The task explicitly steers toward the smaller model "unless explicit
  attenuation is essential," and nothing in the worked threat (§2, §22)
  requires it.

**Consequence:** `DelegationV4` is byte-for-byte identical in shape to
`DelegationV3`/`DelegationV2` — `{delegator, delegatee, authority:
[{scope, target}]}` — no new field, no new validation rule beyond what v2
already has. A delegator's outgoing edge for capability `c` is valid only
if the delegator's own remaining budget for `c` is `≥ 1`; if valid, the
delegatee inherits `remaining - 1`, mechanically, with no way for the
document to override that arithmetic in either direction. If a later
phase demonstrates a real need for deliberate under-delegation of budget
(a delegator choosing to grant fewer hops than it could), that is new,
additive scope for a future phase (§30), not something Phase 4 forecloses
by choosing the smaller model now.

---

## 8. Formal invariant: Delegation Depth Preservation

> **Delegation Depth Preservation:** for every capability `c = (s, t)`
> declared by a root principal with budget `b = c.max_delegation_depth`,
> and for every node `n` reachable from that root via a chain of valid
> delegation edges each carrying `c`, `n`'s *usable* possession of `c` is
> legitimate regardless of chain length — but `n`'s further transmission
> of `c` via an outgoing delegation edge is legitimate **only if** the
> number of edges already traversed from the root to `n` along the
> best available valid path is strictly less than `b`. A delegation edge
> that would carry `c` beyond that budget contributes nothing to its
> delegatee's derived authority for `c` — not partial credit, not the
> capability at a clamped depth, nothing (§11, strict distrust).

Restated operationally, with the remaining-budget framing from §4:
an edge `e = (d, n, A)` carrying capability `c ∈ A` is valid for `c` only
if `remaining(d, c) ≥ 1`; if valid, `remaining(n, c) = remaining(d, c) -
1`. A node may **use** `c` (as an operation's actor or requester) at any
remaining budget `≥ 0`; a node may **delegate** `c` further only at
remaining budget `≥ 1` measured *before* that delegation.

This is a distinct invariant from Non-Amplification and Context-Binding
Preservation, not a special case of either: a delegation edge that fails
Delegation Depth Preservation is emphatically **not** amplification (the
delegator genuinely, validly holds `c` — nothing about its own claim to
`c` is false) and is **not** a context-binding violation (the target is
exactly right). It fails a third, independent property: the *distance*
the capability has already traveled from its origin. §12 defines the
precise composition and precedence with the other three invariants.

---

## 9. Derived authority with depth metadata

Confirms the task's §4 question directly: **yes, Phase 4 requires
`DA(n)` to carry, per capability, more than membership.** Phases 1-3's
`DA(n)` is `set<Capability>` (membership is the only fact that matters).
Phase 4 generalizes this to `DA(n) : Capability → DepthState`, where:

```go
type depthState struct {
    remaining     int // best (max) remaining redelegation budget reachable at this node for this capability
    configuredMax int // the root grant's originally declared max_delegation_depth, carried through unchanged from that grant, alongside remaining
}
```

**Why `configuredMax` must be tracked alongside `remaining`, not
recomputed later.** A `delegation_depth_violation` finding (§14) must
report the capability's *originally declared* maximum, for the reader's
diagnostic benefit ("this was only ever allowed 1 hop"). By the time
verification reaches a deep node, the original declaration is many hops
upstream; recomputing it after the fact would require re-walking the
provenance chain (a second, capability-specific traversal this design
deliberately avoids, §16). Carrying it forward as a second field on the
same state, updated in lockstep with `remaining` at every hop
(`configuredMax` copies unchanged; `remaining` decrements by exactly 1),
costs nothing extra algorithmically and requires no additional traversal.

**Presence-only consumers are unaffected.** Every existing Phase 1-3
helper (`isSubsetCap`, `classifyEdge`, `classifyOne`, `heldTargetsForScope`,
`containsCap`) operates on a *flat* `[]model.Capability` — presence and
binding, never depth. `verify_v4.go` derives this flat view from the
depth-aware `DA(n)` with one small, pure helper (`flatten(da[n])
[]model.Capability`, simply the sorted key set) and passes it into those
functions **completely unmodified**. This is the crux of why Phase 4 is
additive rather than a rewrite: presence/binding logic never needs to know
depth exists at all; it just needs a view over the key set, which it
already had.

---

## 10. Multiple-path semantics

**Decision, formally justified (not assumed): a capability is usable at a
node if *at least one* valid provenance path delivers it, and a node's
further-delegation eligibility is governed by the *best* (maximum
remaining) budget among all valid delivering paths.**

Worked justification of the task's own scenario (§3 of the brief):

```
admin-a  --(billing:refund@billing-service, budget declared 1)--> agent-x
admin-b  --(billing:refund@billing-service, budget declared 3)--> agent-y --> agent-x
```

- Via `admin-a`: `remaining(agent-x, c)` along this path = `1 - 1 = 0`.
- Via `admin-b → agent-y → agent-x`: `remaining(agent-y, c) = 3 - 1 = 2`;
  `remaining(agent-x, c)` along this path = `2 - 1 = 1`.

`agent-x` receives `c` via two independently-valid edges (both pass the
presence/binding subset check and both have delegator-side `remaining ≥
1` at the moment of their own edge). Taking the componentwise **max**
across valid incoming edges per capability — `da[agent-x][c].remaining =
max(0, 1) = 1` — is correct and sound, not merely convenient: each path is
independently and fully valid on its own terms (§9's definition of
`remaining` is path-relative), so adopting the better of two independently
legitimate facts is not "combining" untrustworthy partial information —
it is choosing which of two *true* statements about `agent-x` to report.
`agent-x` is genuinely, validly reachable at remaining budget 1 via the
`admin-b` path; the `admin-a` path's weaker figure does not retroactively
make that false. This directly matches the task's own strong candidate
framing (§3 of the brief) and the required no-path-order-dependence
property (§16): computing the max over a *set* of valid incoming values is
manifestly independent of the order in which incoming edges are visited.

**Tie-break, when two paths yield equal maximum `remaining`.** Per §16 of
the brief ("if several paths grant the same capability with equal
remaining depth, define the canonical tie-break rule... derive it from
existing behavior"): `verify_v2.go`/`verify_v3.go` already iterate a
node's incoming edges in ascending lexicographic order of `e.Delegator`
(`sort.SliceStable` on `incoming[k]`, present in the merged code today).
Phase 4 reuses this exact iteration order and applies a strict-improvement
rule: a candidate `depthState` for capability `c` replaces the
current-best only when its `remaining` is **strictly greater**, never on
equality. Because incoming edges are already visited smallest-delegator-id
first, "first strictly-best wins, ties keep the earlier (lexicographically
smaller delegator) result" falls out of the existing loop structure with
**zero new sorting or comparison logic** — the same "reuse the existing
ascending-id iteration order for determinism" discipline every prior phase
already applies (`docs/phase-1-plan.md` §8, `docs/phase-2-plan.md` §11).

---

## 11. Strict distrust

Preserved and extended, per CLAUDE.md's invariant and the task's explicit
§5 instruction to examine this precisely.

**Two independently-tracked failure surfaces, not one:**

1. **Presence/binding strict distrust (unchanged, Phase 1-3):** if a
   delegation edge's declared capability array is not, as a whole, a
   subset of the delegator's held capability *set* (ignoring depth
   entirely — a pure membership question), the **entire edge** is
   invalid and contributes nothing — not even the tuples that were
   individually present. Identical to today.
2. **Depth strict distrust (new, Phase 4):** given an edge that *did*
   pass the presence/binding check for every capability it carries, if
   **any** of those capabilities has `remaining(delegator, c) == 0`,
   that specific capability may not travel across this edge. Per the
   task's explicit §5 examination ("A node may validly possess capability
   C while having remaining delegation depth 0. It may use C itself but
   may not pass C farther. Determine whether this is the correct
   model.") — **confirmed as the correct model** (§4.2, §8): usability
   and delegability are different, independently-gated properties of the
   same held capability.

**Whole-edge poisoning is preserved for depth failures too, matching
existing precedent exactly.** If a single delegation edge carries multiple
capabilities and *any* of them is depth-exhausted at the delegator, the
**entire edge** is invalid — including capabilities in the same edge that
individually had ample remaining budget. This is not a new design choice
invented for depth; it is the direct, mechanical continuation of the
existing rule ("an edge granting `{billing:read, billing:write}` where
`billing:write` is over-claimed poisons `billing:read` too," CLAUDE.md,
`TestStrictDistrustNoPartialCredit`) applied to a third failure mode. Not
poisoning the whole edge — letting the depth-exhausted capability alone
fail while others in the same edge silently pass — would be exactly the
"partial credit" the strict-distrust invariant exists to forbid, applied
inconsistently depending on which of the three checks fails. Phase 4 does
not introduce a new exception to this rule; it applies the rule a third
way.

**Classification precedence within one poisoned edge, when the excess set
mixes failure kinds (§14):** presence failure (`authority_amplification`)
> binding failure (`context_binding_violation`) > depth failure
(`delegation_depth_violation`) — the same "more foundational problem takes
precedence and is never masked" principle `docs/phase-2-plan.md` §8
already establishes, extended by one more tier. A capability that is
missing outright is a strictly more urgent problem than one that is
present-but-wrong-target, which is itself strictly more urgent than one
that is present-correct-target-but-out-of-budget (the delegator's claim to
`c` is not in question at all in that last case — only how far `c` may
still travel).

---

## 12. Interaction with Phases 1-3

Four invariants now compose over the same graph and the same `DA(n)`. The
precedence rules are defined **per detection point**, because — critically
— depth violations and requester violations occur at *different* points
and never need to be arbitrated against each other directly:

**Edge-level (`point = "delegation_edge"`), three-tier precedence, exactly
one finding per invalid edge:**

```
classifyEdgeV4(edgeCapabilities, delegatorDA):
    missing = { c in edgeCapabilities : scope(c) never held under any target in delegatorDA }
    if missing non-empty:
        return authority_amplification            // §8 docs/phase-2-plan.md, unchanged
    wrongTarget = { c in edgeCapabilities : scope(c) held, only under a different target }
    if wrongTarget non-empty:
        return context_binding_violation           // unchanged
    // every capability in edgeCapabilities is present, correctly bound;
    // reaching here means at least one has remaining(delegator, c) == 0
    return delegation_depth_violation               // new, Phase 4
```

**Operation-level (`point = "operation"`), unchanged three-step precedence
from `docs/phase-3-plan.md` §8 — depth never participates:**

```
evaluate(op, da):
    actorFlat = flatten(da[op.Actor])       // presence-only view; §9
    C = Capability{op.Requires, op.Target}
    if C not in actorFlat:
        emit CapabilityOperationFinding(classifyOne(...))    // unchanged
        return
    requesterFlat = flatten(da[op.Requester])
    if C in requesterFlat:
        return   // ALLOW
    emit ConfusedDeputyFinding(...)          // unchanged
```

**Why depth never manifests as an operation-level finding kind of its
own**, directly answering the task's §7 question: because delegation
depth gates *transmission*, not *use* (§4.2, §8, §13). A node that holds
`c` at `remaining == 0` still has `c` in its flat, presence-only `DA`
view — `flatten` does not filter by remaining budget, by design (§9). An
operation can therefore never fail because of depth; it can only fail
because the actor's or requester's `DA` genuinely lacks the capability —
which, when the *reason* it's missing is that an upstream edge was
depth-exhausted, is correctly and separately visible as an independent
`delegation_depth_violation` **edge** finding earlier in the same
`verify` run. **Both findings are legitimately emitted, at their own
points, with no masking between them** — exactly as an
`authority_amplification` edge finding and an unrelated downstream
`context_binding_violation` operation finding already coexist without
either phase attempting to suppress one in favor of the other. See §22's
worked example for a concrete instance of this two-finding shape.

**No new arbitration is needed between depth and requester precedence**,
because they never compete for the same finding: depth is purely
edge-scoped, requester is purely operation-scoped, and an operation's
actor-side check (which depth can influence only indirectly, through
`DA(actor)` no longer containing a capability that a depth-exhausted edge
failed to deliver) always still runs *before* the requester check, exactly
as `docs/phase-3-plan.md` §8 already established. Phase 4 adds a new
edge-level outcome and leaves the operation-level precedence chain
completely untouched.

---

## 13. Requester interaction

**Confirmed directly, per the task's §13 steer: a requester needs
capability standing (presence in `DA(requester)`), not remaining
delegation depth, because requesting an operation is not delegation.**

`op.Requester` never appears as a delegation edge's `delegator` or
`delegatee` field by virtue of being named as a requester — naming a node
as a requester creates no graph edge, consumes no budget, and is checked
via `flatten(da[op.Requester])` (§9, §12), the identical presence-only
view used for the actor side. A requester whose only apparent standing
for `c` arrived via a now-depth-exhausted path still has `c` in its flat
`DA` — the requester's *use* of standing it independently derived is
legitimate regardless of remaining budget, symmetric with the actor-side
reasoning in §4.2 and §8. If, instead, the requester's only claim to `c`
was itself blocked by a **presence or binding** failure upstream (an
ordinary Phase 1-3 case), that already, correctly, excludes `c` from
`flatten(da[requester])` — unaffected by anything Phase 4 adds.

---

## 14. Deterministic findings

One new finding type, alongside the three existing, unmodified finding
types (`EdgeFinding`/`OperationFinding` from Phase 1, `CapabilityEdgeFinding`/
`CapabilityOperationFinding` from Phase 2, `ConfusedDeputyFinding` from
Phase 3):

```go
// internal/report/delegation_depth_finding.go

const ViolationDelegationDepth = "delegation_depth_violation"

// DepthExcess is one capability, within an invalid edge's declared set,
// that failed specifically because its delegator's remaining redelegation
// budget for it was exhausted (0) — never because of a presence or
// binding failure, which take precedence (§12) and are reported via
// CapabilityEdgeFinding instead.
type DepthExcess struct {
    Scope          string `json:"scope"`
    Target         string `json:"target"`
    ConfiguredMax  int    `json:"configured_max_depth"`
    RemainingDepth int    `json:"remaining_depth"`
}

type DelegationDepthFinding struct {
    Violation string        `json:"violation"` // always "delegation_depth_violation"
    Point     string        `json:"point"`      // always "delegation_edge"
    Delegator string        `json:"delegator"`
    Delegatee string        `json:"delegatee"`
    Declared  []Capability  `json:"declared"`   // the edge's whole declared capability set
    Excess    []DepthExcess `json:"excess"`      // the depth-exhausted subset, each paired with its own configured max / remaining depth
    Trace     []string      `json:"trace"`
    Reason    string        `json:"reason"`
}
```

**Field selection, justified against the task's own candidate list (§8 of
the brief).** The brief's suggested field list includes "attempted
depth" alongside "configured maximum depth" and "remaining depth."
**`attempted_depth` is deliberately omitted as redundant**: it is fully
recoverable as `configured_max_depth - remaining_depth + 1` and carries no
information `configured_max_depth`/`remaining_depth` don't already state
more directly (this is a mechanical restatement of the task's own
instruction — "choose only fields that are formally meaningful... avoid
redundant fields" — applied concretely, the same discipline
`docs/phase-3-plan.md` §12 already used to reject sub-classifying
`confused_deputy`). `Excess` is a struct array, not a parallel
index-aligned array, specifically so each depth-exhausted capability
carries its own `configured_max_depth`/`remaining_depth` self-contained —
two different capabilities in the same poisoned edge can legitimately
have different configured budgets (§5's worked example shows two
capabilities on one principal with different declared depths), so a
single pair of scalar fields on the finding itself would not be
expressive enough; a bare parallel array would work but reintroduce
implicit index-coupling this design avoids on principle.

**Deterministic reason text** (generated, not free-form, matching the
discipline of every prior phase):

- Single depth-exhausted capability:
  `"<delegator> attempted to delegate <scope>@<target> to <delegatee>, but <delegator>'s remaining delegation budget for this capability is 0 (configured maximum: <configured_max>) — it may no longer be redelegated"`
- Multiple depth-exhausted capabilities in one edge:
  `"<delegator> attempted to delegate [<scope>@<target> (configured maximum: <n>), ...] to <delegatee>, but <delegator>'s remaining delegation budget for each is 0 — none may be redelegated further"`

(Exact wording is an implementation-session detail, consistent with the
latitude Phase 2/3's own plans left for text rendering — the field
contract and the presence of a deterministic template are the plan-level
commitment.)

**No new sort-key field is required.** `report.Sort`'s existing 6-tuple
`sortKey{point, subject, secondary, scope, target, requester}` already
keys edge findings on `(point, subject=delegator, secondary=delegatee)`
alone — `scope`/`target`/`requester` stay `""` for `EdgeFinding` and
`CapabilityEdgeFinding` today, because `docs/phase-1-plan.md` §3.2's
at-most-one-edge-per-`(delegator, delegatee)`-pair rule (unchanged,
reused verbatim by v4, §17) already guarantees uniqueness at that
granularity. `DelegationDepthFinding` is keyed identically — `keyOf`
gains one more type-switch case returning `sortKey{point: v.Point,
subject: v.Delegator, secondary: v.Delegatee}`, with no struct-level
extension needed. This is a smaller change than either Phase 2's or
Phase 3's own sort-key extension (both of which *did* need a new trailing
field, because operation findings can share `(actor, action)` and differ
only by scope/target/requester) — worth calling out explicitly as a
direct consequence of depth findings being edge-scoped only (§12).

---

## 15. Canonical provenance

Evaluated directly per the task's §9 instruction: **the verifier needs
neither "one canonical valid provenance" as a materialized path object,
nor "the best remaining delegation budget" as anything beyond the scalar
already carried in `depthState` (§9), nor "all possible paths."** The
scalar DP state (`remaining`, `configuredMax`) computed in §9/§10 is
*sufficient* to state every quantity a `delegation_depth_violation`
finding needs (`configured_max_depth`, `remaining_depth` — §14) via plain
arithmetic on values already computed in the single topological pass;
nothing about producing those two numbers requires reconstructing an
actual path.

**The `trace` field, however, is reused unmodified from existing
precedent** — `graph.CanonicalTrace(principalIDs, validEdges, delegator) +
[delegatee]`, the exact same construction Phase 1's `EdgeFinding` and
Phase 2's `CapabilityEdgeFinding` already use. This trace is illustrative
provenance context (*some* valid path connecting the delegator to a root),
not a formal proof of the specific numeric claim — exactly as it already
is for amplification/binding findings today, where the trace shown is
never guaranteed to be the specific path that carried the missing scope
either (§8.1, `docs/phase-1-plan.md`). The finding's numeric fields
(`configured_max_depth`, `remaining_depth`) are the actual formal evidence;
the trace is orientation. Reusing `CanonicalTrace` exactly as-is, rather
than inventing a capability-specific path-reconstruction pass, is the
direct answer to the task's "avoid enumerating all graph paths unless
mathematically required" and "seek a deterministic polynomial-time
algorithm" steers (§9, §10 of the brief) — it is not required, so it is
not added.

`validEdges`, for Phase 4's trace purposes, means edges that were **fully
valid** — passed presence, binding, *and* depth (§11) — mirroring exactly
how Phase 2 already excludes a binding-invalid edge from every downstream
trace (`docs/phase-2-plan.md` §13).

---

## 16. Verification algorithm

**Decision: Phase 4 still fits entirely within one static, deterministic,
topological pass. No state-space exploration, no backtracking, no
per-capability path search.**

`RunV4(*model.ModelV4) report.Result`, structurally parallel to `RunV3`:

1. **Build the graph.** Nodes = principals ∪ agents, edges = delegations
   — identical to every prior phase; `graph.TopoSort` reused as-is.
2. **Topological evaluation**, node ids in the identical ascending-
   lexicographic tie-broken topological order every phase already uses:
   - Principal `p`: for each declared `RootCapability{Scope, Target,
     MaxDelegationDepth}`, `da[p][Capability{Scope,Target}] =
     depthState{remaining: *MaxDelegationDepth, configuredMax:
     *MaxDelegationDepth}` (§9).
   - Agent `a`: `da[a] = {}` initially. For each incoming edge `e`,
     ascending lexicographic order of `e.Delegator` (existing iteration
     order, reused verbatim):
     1. `flatDelegatorDA = flatten(da[e.Delegator])`. If `e.Authority ⊄
        flatDelegatorDA` (presence/binding, unchanged Phase 2 check):
        classify via unchanged `classifyEdge`, emit
        `CapabilityEdgeFinding`, edge contributes nothing, continue to
        next edge.
     2. Presence/binding passed. For each `c ∈ e.Authority`, read
        `state = da[e.Delegator][c]`. If any `state.remaining < 1`:
        collect those into a `DepthExcess` list, emit
        `DelegationDepthFinding`, edge contributes nothing (§11, whole-
        edge poisoning), continue to next edge.
     3. Edge fully valid: for each `c ∈ e.Authority`, candidate =
        `depthState{remaining: state.remaining - 1, configuredMax:
        state.configuredMax}`; adopt into `da[a][c]` only if no entry
        exists yet or `candidate.remaining > da[a][c].remaining` (strict
        improvement — §10's tie-break).
3. **Operation evaluation**, operations in the existing ascending
   `(actor, action, requires.Scope, requires.Target, requester)` order
   (unchanged from `docs/phase-3-plan.md` §11): run §12's unmodified
   three-step precedence, using `flatten(da[actor])`/`flatten(da[requester])`.
4. **Sort all findings** (all five finding shapes together) by the
   unmodified 6-tuple key (§14).
5. **Result:** `ALLOW` (exit 0) if empty, else `DENY` (exit 1) — unchanged
   result semantics.

**Complexity.** Let `N` = nodes, `E` = delegation edges, `A` = the bound
on per-edge/per-principal capability-set size (`limits.MaxAuthoritySetSize`,
unchanged, §21). Step 2 does O(1) `depthState` work per capability per
edge, so the whole pass is `O(N + E·A + O)` — the identical asymptotic
class Phase 2/3 already run (`isSubsetCap`/`classifyEdge` already iterate
per-capability within an edge; Phase 4 adds one more O(1)-per-capability
check and one O(1) map comparison, not a new order of growth). **No
per-capability path enumeration, no branching over alternative
interpretations, no state-space search is introduced or required** — every
quantity (`da[n][c].remaining`, `da[n][c].configuredMax`, edge validity,
operation pass/fail) has exactly one correct, DP-computed value given the
acyclic input, for the identical reason `docs/phase-1-plan.md` §8.2 gives:
no time dimension, no conditional grant, nothing to search over. This
directly confirms the task's own steer (§10 of the brief): the
`state[node][capability] = maximum remaining delegation budget` DP concept
it suggests **is** sufficient, and is adopted, after being proven so
above rather than assumed.

`model.Model` (v1), `model.ModelV2` (v2), and `model.ModelV3` (v3)
continue to run their existing, entirely untouched `Run`/`RunV2`/`RunV3`
functions, byte-identical to today.

---

## 17. Validation

Every existing structural rule from Phase 1-3 applies to version-4
documents unchanged, generalized only where the shape changed
(`RootCapability` replaces `Capability` in principal authority arrays
only; delegations and operations are unchanged from v3).

**New version-4-only structural rules:**

- **`KindInvalidDelegationDepth = "invalid_delegation_depth"`** — a single
  kind covering both of the following, mirroring how `unknown_requester`
  already covers both "missing" and "malformed" for a single underlying
  reason (`docs/phase-3-plan.md` §15):
  - **Missing `max_delegation_depth`.** Decodes as a `nil` `*int` (§6);
    rejected.
  - **Negative `max_delegation_depth`.** A present, non-nil, negative
    value; rejected (a negative redelegation budget has no meaning).
- **Resource-limit check, reusing the existing `resourceLimitErr`
  mechanism** exactly as `max_chain_depth`/`max_authority_set_size`
  already do: `max_delegation_depth` exceeding `limits.MaxDelegationDepth`
  (§21) is `KindResourceLimitExceeded` with `Primary = "max_delegation_depth"`
  — no new `ErrorKind` needed, this reuses the existing generic resource-
  limit error shape verbatim.
- **Duplicate root-capability declaration, extended.** Two `RootCapability`
  entries within one principal's `authority` array sharing the same
  `(scope, target)` pair — **regardless of whether their
  `max_delegation_depth` values agree** — is `KindDuplicateCapability`
  (existing kind, reused, checked over the `(scope, target)` projection
  only). Depth is deliberately **not** part of the uniqueness key: two
  entries with the same tuple but different declared depths would be
  genuinely ambiguous (which value governs?), so this is correctly
  rejected as a duplicate, not accepted as "the stricter one wins" or any
  other implicit merge rule — identical philosophy to why Phase 1 rejects
  duplicate scopes rather than silently deduplicating them
  (`docs/phase-1-plan.md` §4).

**Explicitly evaluated and rejected** (per the task's own instruction not
to adopt its suggested list wholesale, §14 of the brief):

- **Non-integer depth.** Not a `validateV4` structural-error case at all
  — it is a JSON decode-level type mismatch, already handled by the
  existing `LoadError.ParseError` path with zero new code (§6).
- **"Illegal depth fields on objects where they do not belong"** (e.g. a
  `max_delegation_depth` key on a delegation's authority entry, or on an
  operation). Not a dedicated `validateV4` check — `DelegationV4.Authority`
  uses the plain `Capability{Scope, Target}` Go type (no such field
  exists on it at all) and `OperationV4` has no capability-object field
  either, so `DisallowUnknownFields` rejects any such stray key as a
  decode-level error automatically, with zero new validation code — the
  identical "enforced for free by the schema shape" precedent
  `docs/phase-1-plan.md` §7.2 already establishes for `Agent.authority`.
- **"Delegation-depth policy violated" as a `validate`-time (exit 2)
  error.** Rejected on the identical precedent every prior phase
  establishes (`docs/phase-1-plan.md` §7.4, `docs/phase-2-plan.md` §10,
  `docs/phase-3-plan.md` §15): a structurally well-formed document that
  turns out to violate a semantic invariant is a `verify`-time finding
  (exit 1), never a `validate`-time structural error (exit 2).

`validate` on a version-4 document therefore still never evaluates any
invariant — Non-Amplification, Context-Binding, Requester Authorization,
or Delegation Depth Preservation — exactly as established for v1/v2/v3.

---

## 18. CLI compatibility

**No new subcommands, no new flags.** `validate <model.json>` and
`verify <model.json> [--format text|json]` remain the only two commands.
`main.go`'s existing dispatch switch in `runVerify` gains one more case:

```go
case doc.V4 != nil:
    result = verify.RunV4(doc.V4)
```

`--format text|json` applies identically across all four versions. No
`--depth`/`--max-hops` override flag, no version-selection flag — version
is read from the document, exactly as `"1"`/`"2"`/`"3"` already are.

---

## 19. Text/JSON compatibility

**JSON.** The top-level envelope (`{"result", "findings"}`,
`internal/report/json.go`) is unchanged — already generic over
`[]interface{}`, so `DelegationDepthFinding` requires zero changes to
`RenderJSON`. Version-1/2/3 output is byte-identical to today,
unconditionally: `RunV4` is a new function, called only when `doc.V4 !=
nil`, touching no code path `Run`/`RunV2`/`RunV3` execute.

**Text.** `RenderText`'s type switch gains one new case, matching the
existing label-column style:

```
[1] delegation_depth_violation (delegation_edge)
  delegator:  billing-agent
  delegatee:  support-agent
  declared:   billing:refund@billing-service
  excess:     billing:refund@billing-service (configured max: 1, remaining: 0)
  trace:      admin -> billing-agent -> support-agent
  reason:     billing-agent attempted to delegate billing:refund@billing-service to support-agent, but billing-agent's remaining delegation budget for this capability is 0 (configured maximum: 1) — it may no longer be redelegated
```

(Exact column labels/widths are an implementation-session detail, matching
the latitude every prior phase's plan already left for text rendering.)

---

## 20. Exit codes

Unchanged. `internal/exitcode` gains no new values:

| Code | Meaning (extended) |
|---|---|
| `0` | Structurally valid model (v1-v4); zero findings for `verify`. |
| `1` | One or more findings — `authority_amplification`, `context_binding_violation`, `confused_deputy`, and/or `delegation_depth_violation`, in any combination. A v4 model can `DENY` on any mix; the exit code does not distinguish which. |
| `2` | Structural/model problem for any schema version, including the new `invalid_delegation_depth` kind and the `max_delegation_depth` resource limit. |
| `3` | CLI usage error — unchanged. |

---

## 21. Resource bounds

One new bound, mirroring the existing `MaxChainDepth`/`MaxTargetLength`
pattern exactly, but named and reasoned about **distinctly** from
`MaxChainDepth` per CLAUDE.md's explicit warning against conflating
resource-safety bounds with policy semantics:

| Limit | Value | Notes |
|---|---|---|
| `MaxDelegationDepth` | `64` | New. Bounds the **declared** `max_delegation_depth` value a document may assert on any `RootCapability`. Set equal in value to `MaxChainDepth` (since no chain can ever be longer than `MaxChainDepth` hops regardless of what a document declares, a declared budget beyond that is unreachable and therefore meaningless) but kept as an **independent exported `var`**, not an alias, so a test can lower one without perturbing the other — mirroring exactly why every existing bound is a `var` and not a baked-in constant (`docs/phase-1-plan.md` §12). |

**Why this is a resource-safety bound on a policy field, not itself a new
policy-level concept**, resolving a potential category confusion the task
explicitly flags (§15 of the brief, and CLAUDE.md's own framing of
`MaxChainDepth`): `MaxDelegationDepth` does not decide *whether* re-
delegation should be limited — the document's own `max_delegation_depth`
values do that, per-capability, and that is the actual Phase 4 policy
invariant (§8). `limits.MaxDelegationDepth` only bounds how large a value
a document is permitted to *declare*, exactly the same role
`MaxAuthoritySetSize` plays for array sizes or `MaxScopeLength` plays for
string length — a safety valve against a pathological/adversarial
declared value (e.g. `max_delegation_depth: 2000000000`), not a product
decision about redelegation policy itself.

**No new state-size bound is required.** `da[n]` changes from
`map[Capability]bool`-equivalent (a set) to `map[Capability]depthState` (a
map to a small fixed-size struct) — the **key space is identical** to
what Phase 2/3 already bound via `MaxAuthoritySetSize` (per-principal/per-
edge array size) and the total-distinct-capability ceiling that already
follows from `MaxNodes × MaxAuthoritySetSize`. Attaching two `int` fields
to each existing map value is O(1) extra space per entry already counted,
not a new dimension of growth. This directly confirms the task's own
steer (§15 of the brief: "do not create arbitrary limits without
justification") — none is created beyond the one bound that has a
concrete, named justification above.

All existing Phase 1-3 bounds (`MaxInputFileSize`, `MaxNodes`,
`MaxDelegationEdges`, `MaxOperations`, `MaxScopeLength`, `MaxIDLength`,
`MaxAuthoritySetSize`, `MaxChainDepth`, `MaxTargetLength`) apply to
version-4 documents unchanged.

---

## 22. Worked example

`examples/billing-redelegation-depth.json` (implementation-session file,
matching the task's §19 scenario, extended by one hop to demonstrate both
the passing and failing case in a single small fixture, mirroring exactly
how `examples/billing-confused-deputy.json` demonstrates both a legitimate
and a violating operation in one file):

```json
{
  "version": "4",
  "principals": [
    {
      "id": "admin",
      "authority": [
        { "scope": "billing:refund", "target": "billing-service", "max_delegation_depth": 1 }
      ]
    }
  ],
  "agents": [
    { "id": "billing-agent" },
    { "id": "support-agent" }
  ],
  "delegations": [
    { "delegator": "admin",         "delegatee": "billing-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] },
    { "delegator": "billing-agent", "delegatee": "support-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin", "action": "refund-ok",   "requires": "billing:refund", "target": "billing-service" },
    { "actor": "support-agent", "requester": "admin", "action": "refund-deep", "requires": "billing:refund", "target": "billing-service" }
  ]
}
```

`verify examples/billing-redelegation-depth.json`:

- **Edge `admin → billing-agent`**: `remaining(admin, c) = 1` (the
  declared budget). `1 ≥ 1`, so the edge is valid; `billing-agent`
  receives `c` at `remaining = 0`, `configuredMax = 1`. No finding.
- **Edge `billing-agent → support-agent`**: `remaining(billing-agent, c) =
  0`. `0 < 1`, so this edge is **invalid** for `c` — the entire edge
  contributes nothing. One `delegation_depth_violation` finding:
  `delegator = billing-agent`, `delegatee = support-agent`, `excess = [{
  scope: "billing:refund", target: "billing-service", configured_max_depth:
  1, remaining_depth: 0 }]`.
- **`refund-ok`** (actor `billing-agent`) — `billing:refund@billing-service
  ∈ flatten(da[billing-agent])` (held at `remaining = 0`, but presence is
  all usability requires, §4.2/§8/§13). Requester `admin` axiomatically
  holds it too. **Passes**, no finding.
- **`refund-deep`** (actor `support-agent`) — because the
  `billing-agent → support-agent` edge was entirely invalid, `c ∉
  da[support-agent]` at all. **Fails**: `authority_amplification` at the
  operation level (`docs/phase-2-plan.md` §8's classification — the
  scope was never validly held by `support-agent`, under any target) —
  **not** a second `delegation_depth_violation`, because depth violations
  are edge-scoped only (§12). This is the concrete instance of §12's
  "two findings, two points, no masking" shape: the *cause* is visible at
  the edge (`delegation_depth_violation`), and the *consequence* is
  visible at the operation (`authority_amplification`) — two distinct,
  independently useful findings, not a duplicate.

This single file demonstrates: a valid one-hop delegation, a passing
operation at the budget boundary (remaining exactly 0, still usable), a
depth-violating second hop, and its downstream operation-level
consequence — exactly matching the task's own worked-example instructions
(§19 of the brief) while also exercising §12's precedence/composition
claim in one small, readable fixture.

---

## 23. Architecture/file plan

Purely additive to `docs/phase-1-plan.md` §15 / `docs/phase-2-plan.md`
§19 / `docs/phase-3-plan.md` §21. No existing file is deleted or renamed;
the one sanctioned message-text touch is called out explicitly (§5, §26).

```
internal/model/
  types.go, types_v2.go, types_v3.go  — UNCHANGED
  types_v4.go                          — NEW: RootCapability{Scope, Target,
                                          MaxDelegationDepth *int}, ModelV4,
                                          PrincipalV4{ID, Authority
                                          []RootCapability}, AgentV4
                                          (identical shape to AgentV3),
                                          DelegationV4 (identical shape to
                                          DelegationV3, reuses model.Capability
                                          for its Authority entries),
                                          OperationV4 (byte-identical shape
                                          to OperationV3)

internal/limits/
  limits.go                            — ADD: MaxDelegationDepth var (§21).
                                          One-line, additive.

internal/loader/
  loader.go, loader_v2.go, loader_v3.go — UNCHANGED except the one
                                          sanctioned message-text touch
                                          (§5, §26) inside validate(),
                                          validateV2(), validateV3(); and
                                          LoadDocument's switch gains a "4"
                                          case, Document gains a V4 field.
  loader_v4.go                          — NEW: decodeAndValidateV4,
                                          validateV4 (reuses checkID/
                                          checkScope/checkTarget/
                                          resourceLimitErr/sortErrors
                                          verbatim; adds checkRootCapabilitySet,
                                          a small variant of checkCapabilitySet
                                          that additionally validates each
                                          entry's MaxDelegationDepth pointer
                                          — §6, §17 — while reusing
                                          checkScope/checkTarget for the
                                          (scope, target) portion and the
                                          existing duplicate-detection
                                          pattern projected onto (scope,
                                          target) only, §17).

internal/graph/
  graph.go                              — UNCHANGED. Reused as-is (§16).

internal/verify/
  verify.go, verify_v2.go, verify_v3.go — UNCHANGED.
  verify_v4.go                           — NEW: RunV4(*model.ModelV4)
                                          report.Result, implementing
                                          §11/§12/§16. Introduces the
                                          unexported depthState type and
                                          flatten() helper (§9); reuses
                                          verify_v2.go/verify_v3.go's
                                          unexported helpers (isSubsetCap,
                                          subtractCap, canonicalizeCaps,
                                          containsCap, heldTargetsForScope,
                                          classifyOne, classifyEdge,
                                          toReportCaps) directly against
                                          flatten()'s output, same package,
                                          no duplication.

internal/report/
  finding.go                             — UNCHANGED sortKey struct (§14);
                                          extend keyOf's type switch with
                                          one new case for
                                          DelegationDepthFinding. EXISTING
                                          cases/fields untouched.
  capability_finding.go,
  confused_deputy_finding.go              — UNCHANGED.
  delegation_depth_finding.go              — NEW: DepthExcess,
                                            DelegationDepthFinding,
                                            ViolationDelegationDepth,
                                            NewDelegationDepthFinding
                                            constructor (§14).
  text.go                                  — extend RenderText's type
                                            switch with the one new finding
                                            type (§19); existing cases
                                            untouched.
  json.go                                   — UNCHANGED (already generic).

cmd/delegationproof/
  main.go                                    — runVerify's dispatch switch
                                              gains one case (§18); no new
                                              flags, no new subcommands, no
                                              exit-code changes.

examples/
  billing-refund.json,
  billing-context-binding.json,
  billing-confused-deputy.json                — UNCHANGED.
  billing-redelegation-depth.json               — NEW (§22).

schemas/
  model.md                                      — NOT modified this session
                                                (explicit instruction,
                                                mirroring every prior
                                                phase's own precedent). The
                                                implementation session must
                                                add a "version 4" section
                                                documenting §5/§6/§17,
                                                mirroring how model.md
                                                documents version 3 today.

testdata/
  valid-v4/                                     — NEW directory:
                                                clean-pass-v4.json, a
                                                reordered-arrays variant
                                                (permutation-invariance,
                                                §25), a
                                                combined-violations-v4.json
                                                (amplification +
                                                context-binding +
                                                confused_deputy +
                                                delegation_depth_violation
                                                all present, §25),
                                                multi-path-depth.json (§10's
                                                worked scenario, fixture
                                                form).
  malformed/                                     — ADD v4 fixtures:
                                                missing-delegation-depth.json,
                                                negative-delegation-depth.json,
                                                delegation-depth-exceeds-max.json,
                                                non-integer-delegation-depth.json
                                                (a JSON decode-error case,
                                                §17), duplicate-root-capability-
                                                different-depths.json,
                                                depth-field-on-delegation.json
                                                and depth-field-on-operation.json
                                                (both decode-level "unknown
                                                field" errors, §17).
                                                Existing v1/v2/v3 fixtures
                                                UNCHANGED, still walked
                                                automatically by
                                                cmd/delegationproof/main_test.go.
  golden/                                        — ADD captured v4
                                                text/json output for
                                                billing-redelegation-depth
                                                and a v4 combined-violations
                                                fixture. Existing v1/v2/v3
                                                golden files UNCHANGED,
                                                byte-identical.

docs/
  phase-4-plan.md                                — this document.
```

---

## 24. Testing plan

Mirrors the structure of `docs/phase-1-plan.md` §16 / `docs/phase-2-plan.md`
§20 / `docs/phase-3-plan.md` §22, additive. Test file names follow the
existing `_v3`/`_v2` naming convention (`verify_v4_test.go`,
`loader_v4_test.go`, `main_v4_test.go`).

1. **Full Phase 1 + 2 + 3 regression** — `go test ./... -race -count=1`
   with zero behavioral change to any existing test, **except** the one
   sanctioned message-text lines (§5, §26); every existing golden file
   byte-identical; every existing malformed fixture still produces its
   original `ErrorKind`.
2. **Max depth 0 (non-delegable, still usable)** — a principal declares
   `max_delegation_depth: 0`; the principal's own operation using it
   passes; any outgoing delegation edge attempting to grant it produces
   `delegation_depth_violation` with `remaining_depth: 0`,
   `configured_max_depth: 0`.
3. **Max depth 1 (exactly one hop)** — `admin → agentA` valid, `agentA`'s
   operation passes, `agentA → agentB` invalid (`delegation_depth_violation`).
4. **Deeper allowed chain** — `max_delegation_depth: 4`, a 4-hop chain,
   every edge valid, the leaf's operation passes, a hypothetical 5th hop
   (separate fixture) invalid.
5. **Exactly-at-boundary chain** — a chain whose length equals the
   declared budget exactly: every edge valid, no finding.
6. **Beyond-boundary chain** — one hop past the boundary: the first
   over-budget edge (and only that edge) produces
   `delegation_depth_violation`; downstream edges from the poisoned
   delegatee onward are separately invalid too (via ordinary presence
   failure, since the poisoned edge propagated nothing) — asserts the
   cascade is via presence, not a second depth finding (§12).
7. **Current holder can use capability at remaining depth 0** — dedicated
   test isolating exactly this case from item 2, using an actor whose own
   incoming edge was the budget-exhausting one (holds at remaining 0),
   confirming the operation passes.
8. **Current holder cannot redelegate at remaining depth 0** — the
   symmetric dedicated test: the same node's outgoing edge, same
   capability, fails with `delegation_depth_violation`.
9. **Multiple paths, one exceeds depth, one valid** — §10's worked
   scenario as a fixture: `agent-x` reachable via a depth-exhausted path
   and a valid path simultaneously; asserts `agent-x` is fully usable and
   its own remaining budget equals the valid path's value, not `0`.
10. **Multiple paths with different remaining budgets, no tie** — asserts
    the max-remaining path's value wins outright.
11. **Multiple paths, exact tie in remaining depth, different
    `configuredMax`** — asserts the deterministic tie-break (§10):
    ascending-lexicographic-delegator-id, first-strictly-better-wins,
    reproduced identically across repeated runs.
12. **Deterministic canonical path selection** — asserts `trace`
    construction on a `delegation_depth_violation` finding matches the
    same `CanonicalTrace` convention Phase 1/2 findings already use.
13. **Reordered-input invariance** — v4 analogue of the existing
    permutation-invariance test: byte-identical output for semantically
    equivalent reordered `principals`/`agents`/`delegations`/`operations`/
    capability arrays, including reordered incoming-edge order at a
    multi-path node (item 9-11's fixture, reordered).
14. **Repeated-run byte determinism** — v4 analogue of
    `TestRunIsDeterministicAcrossRepeatedInvocations`.
15. **Strict distrust interaction, presence** — an edge with one
    presence-invalid and one depth-exhausted capability: asserts
    `authority_amplification` wins (§12's three-tier precedence), not
    `delegation_depth_violation`, and the whole edge (including any
    individually-fine capability) is poisoned.
16. **Strict distrust interaction, binding** — same shape, one
    binding-invalid and one depth-exhausted capability: asserts
    `context_binding_violation` wins over `delegation_depth_violation`.
17. **Context-binding interaction, orthogonal case** — a capability that
    is correctly bound and well within budget, alongside an unrelated
    capability that is binding-invalid in the same document (different
    edge): both findings present, correctly classified, no interference.
18. **Confused-deputy interaction** — `examples/billing-redelegation-depth.json`-
    style fixture extended with a requester lacking standing on the
    still-valid (`refund-ok`) operation: asserts `confused_deputy` fires
    there independently of the unrelated `delegation_depth_violation`
    finding on the second edge, both present, correctly ordered.
19. **Combined violation precedence (edge-level)** — dedicated table test
    covering every row of §12's three-tier edge classification.
20. **Combined violation precedence (operation-level, depth-caused)** —
    `examples/billing-redelegation-depth.json` itself (§22): asserts
    exactly the two-finding shape (one `delegation_depth_violation` edge
    finding, one `authority_amplification` operation finding), never a
    duplicate or masked pair.
21. **Malformed depth declarations** — one case per §17 kind: missing
    (`nil` pointer), negative, non-integer (decode-level), exceeds
    `limits.MaxDelegationDepth`, duplicate `(scope, target)` with
    differing depths, stray `max_delegation_depth` key on a delegation
    entry or operation (decode-level, §17).
22. **Resource limits** — `limits.MaxDelegationDepth` white-box test
    (lowered value, same pattern as every existing `internal/limits`-based
    test); confirms `MaxDelegationDepth` and `MaxChainDepth` are
    independently adjustable without affecting each other (§21).
23. **Text output** — golden-file test for the worked example (§22) and a
    multi-finding v4 fixture.
24. **JSON output** — golden-file test for the same fixtures; asserts the
    envelope shape is unchanged and `DelegationDepthFinding` fields
    appear in the documented order, including the nested `DepthExcess`
    struct array shape.
25. **CLI exit codes** — `validate` vs `verify` divergence for a v4 model
    containing only a `delegation_depth_violation` finding (structurally
    valid, `validate` → 0, `verify` → 1), mirroring the existing
    divergence tests.
26. **No-panic malformed-input behavior** — extend the existing
    fuzz/mutation-style CLI test to include v4 fixtures as seeds,
    including truncated/mutated `max_delegation_depth` byte sequences.

---

## 25. Phase 1-3 regression requirements

- Every existing test in `internal/loader`, `internal/graph`,
  `internal/verify`, `internal/report`, and `cmd/delegationproof` must
  pass, with **exactly one documented class of exception**: the literal
  invalid-version message strings in `validate`, `validateV2`, and
  `validateV3` (and their corresponding assertions in
  `internal/loader/loader_v2_test.go`) change to include `"4"` (§5). This
  is the only sanctioned edit to any pre-existing test file.
- Every existing golden file in `testdata/golden/` must remain
  byte-identical output for its existing input.
- Every existing fixture in `testdata/malformed/` must continue to
  produce its documented `ErrorKind`.
- `examples/billing-refund.json`, `examples/billing-context-binding.json`,
  and `examples/billing-confused-deputy.json` must continue to round-trip
  exactly as their respective plan documents specify.
- No line in `internal/verify/verify.go`, `internal/verify/verify_v2.go`,
  `internal/verify/verify_v3.go`, `internal/graph/graph.go`,
  `internal/report/capability_finding.go`,
  `internal/report/confused_deputy_finding.go`, or any existing
  `internal/model` type may change.
- `go vet ./...`, `gofmt -l .`, and `go build -o bin/delegationproof
  ./cmd/delegationproof` must all succeed exactly as CLAUDE.md requires
  today, with the new v4 files included.

---

## 26. Security assumptions

Extends `docs/phase-1-plan.md` §17, `docs/phase-2-plan.md` §22, and
`docs/phase-3-plan.md` §24 without weakening any of them:

- **A declared `max_delegation_depth` is a policy assertion by the
  document's author, not a verified fact about a real system's actual
  redelegation history.** Exactly as Phase 1's principal
  `declared_authority` is the axiomatic root of trust, and exactly as
  Phase 3's `requester` is a declared label rather than an authenticated
  identity, DelegationProof does not verify that a real running system
  actually enforces the declared budget at runtime — that correspondence
  is a separate, later integration concern (§30), identical in kind to
  every prior phase's own security-assumptions boundary.
- **Delegation Depth Preservation proves a property of the declared model
  only:** "this document never claims a capability may be used at a node
  more hops from its origin than its own declared budget permits." It
  does not, and cannot, prove that a real system actually counts or
  enforces hops at runtime — DelegationProof remains a static, offline
  analyzer with no interception or enforcement component (unchanged,
  Phase 1 §17/§18).
- **No new attack surface from parsing.** `max_delegation_depth` is
  decoded via the same `encoding/json` + `DisallowUnknownFields` +
  bounded-read pipeline as every other field, subject to the same
  `MaxInputFileSize` bound applied before any structural field is read.
  The `*int` pointer type (§6) introduces no unbounded allocation risk:
  a JSON document can declare at most one `max_delegation_depth` value per
  `RootCapability` entry, and the number of such entries is already
  bounded by `MaxAuthoritySetSize`.

---

## 27. Explicit non-goals

All of `docs/phase-1-plan.md` §18's, `docs/phase-2-plan.md` §23's, and
`docs/phase-3-plan.md` §25's non-goals continue to apply. Phase 4
additionally, explicitly, does **not** include:

- MCP protocol implementation, A2A protocol implementation, OAuth, JWT
  verification, tokens, networking, hosted services, proxying, runtime
  enforcement, databases, LLMs, Z3/SAT/SMT, SARIF, approvals, revocation,
  temporal state, sessions.
- **Wildcard scopes, wildcard targets, hierarchical IAM, target
  registry** — unchanged, still rejected.
- **State-space exploration / general search** — evaluated directly
  (§16) and confirmed unnecessary; a single deterministic topological DP
  pass suffices.
- **Explicit per-edge depth attenuation** (a delegator declaring a
  shorter remaining budget than it actually holds). Evaluated (§7) and
  rejected for Phase 4 as unmotivated by the stated threat and expansive
  relative to the smaller inherited-only-decrement model; a later phase
  may reconsider this on its own merits (§30) without redesigning
  anything Phase 4 establishes.
- **Approval preservation** — remains exactly as scoped by
  `docs/phase-1-plan.md` §21, untouched by this phase, and explicitly
  kept deferred per this task's own instruction.
- **Web UI, automatic policy generation, CI-vendor integration.**
- **Phase 5 implementation.**
- **Real-world redelegation-count correspondence** (verifying that a
  document's declared budgets match how many times a real running system
  actually re-delegated a credential) — a topology-ingestion concern, not
  a verification-core concern, symmetric with Phase 3's identical
  boundary around `requester` (§26).

---

## 28. Acceptance criteria

- `go build ./...` succeeds; `go.mod` remains stdlib-only.
- `go vet ./...` is clean; `gofmt -l .` reports nothing.
- `go test ./... -race -count=1` passes, including every category in
  §24, with the documented, sanctioned test-string changes (§25) and zero
  other modification to any pre-existing test file.
- Every existing `testdata/golden/` file is unchanged, byte-identical.
- A version-1, version-2, or version-3 document produces byte-identical
  `validate`/`verify` output, on both `text` and `json` formats, to the
  current `main` branch today.
- A version-4 document with no violations → `ALLOW`, exit 0.
- `examples/billing-redelegation-depth.json` → exactly the two-finding
  shape described in §22 (one `delegation_depth_violation`, one
  `authority_amplification`), matching the worked example.
- A version-4 document containing `authority_amplification`,
  `context_binding_violation`, `confused_deputy`, and
  `delegation_depth_violation` findings simultaneously reports all four,
  correctly classified, correctly ordered, with no duplicate finding for
  any single edge or operation (§12).
- `invalid_delegation_depth` has at least one dedicated malformed fixture
  and table-driven test case per its two sub-cases (missing, negative).
- No panic is reachable from `main` for any version-1 through version-4
  input within the (documented) resource bounds.

---

## 29. Definition of DONE

Phase 4 is done when:

1. All items in §28 are met.
2. The file/package layout matches §23, or a documented deviation is
   noted in this document, keeping it authoritative per every prior
   phase's own convention.
3. The new error kind (§17) and every new finding `violation`/`point`
   combination (§14) has at least one dedicated test.
4. The worked example (§22) is reproducible verbatim via
   `delegationproof verify examples/billing-redelegation-depth.json`.
5. `schemas/model.md` has been updated (or a sibling v4 document added)
   by the implementation session to describe the version-4 shape —
   noted as deferred in §23, not done in this planning session, per
   explicit instruction not to modify it now.
6. No open TODOs remain in code for functionality this document describes
   as in-scope; TODOs for §30's deferred items are fine and expected,
   linking back to §30.
7. `docs/phase-1-plan.md`, `docs/phase-2-plan.md`, and
   `docs/phase-3-plan.md` are unmodified — Phase 4 attaches to all three,
   per their own future-phase-boundary sections, without rewriting any of
   them.

---

## 30. Future-phase boundary

Carried forward from `docs/phase-1-plan.md` §21, `docs/phase-2-plan.md`
§26, and `docs/phase-3-plan.md` §28, still deferred, now with Phase 4's
addition noted where it changes the shape of what attaches:

- **Approval Preservation** (explicitly named by this task as remaining
  deferred): a required approval-state concept attached to operations or
  edges. Nothing in Phase 4's depth-budget model changes what an approval
  layer would need to attach to — it composes with `DA(n)` and the
  operation-evaluation precedence chain (§12) exactly as it would have
  before Phase 4, orthogonal to depth.
- **Explicit per-edge depth attenuation** (newly identified as a
  deliberate non-goal in this phase, §27): if a later product need
  genuinely requires a delegator to voluntarily grant a *shorter* budget
  than it holds, that is new scope layered onto — not replacing — the
  inherited-only-decrement model this phase defines. It would need its
  own edge-level field, its own validation (can only shrink, never grow,
  the inherited value), and its own interaction with §11's strict-distrust
  rule; evaluated on its own merits then, not foreclosed by this phase's
  rejection.
- **Multi-hop request/induced-by chains, temporal/session-scoped
  requester validity**: unchanged from `docs/phase-3-plan.md` §28;
  nothing in Phase 4 accelerates or blocks either.
- **Scope/target wildcard or hierarchy semantics**: still deferred,
  unchanged from Phase 1/2/3.
- **MCP/A2A ingestion, JSON Schema enforcement, SARIF, Z3/SMT**:
  unchanged from Phase 1 §21; nothing in Phase 4 accelerates or blocks
  any of them.
- **Real-world redelegation-count correspondence** (identified in this
  phase, §26): verifying that a document's declared budgets match a real
  system's actual redelegation history is a topology-ingestion concern,
  symmetric with Phase 3's identical posture toward real request-provenance
  correspondence (`docs/phase-3-plan.md` §28). This phase defines what to
  check once `max_delegation_depth` is declared; it does not address how a
  real system's declarations get produced truthfully.
</content>
