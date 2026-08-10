# DelegationProof — Phase 3 Plan

Status: PLANNING ONLY. Phase 1 and Phase 2 are implemented, merged, and
untouched by this document. This is the authoritative design contract for
the Phase 3 implementation session, in the same spirit as
`docs/phase-1-plan.md` and `docs/phase-2-plan.md`.

Module: `github.com/SamudralaAjaykumarrr/delegationproof`

---

## 0. Phase 3 rationale

Phase 1 proved Authority Non-Amplification: does a node exercise or
transmit a scope it was never validly granted? Phase 2 proved
Context-Binding Preservation: is a validly-granted scope being exercised
against the target it was granted for? Both invariants are entirely about
one relationship — delegator → delegatee, i.e. *how authority flows
through the graph* — and both are checked against a single node's own
Derived Authority, `DA(n)`.

Neither invariant has any vocabulary for a second, orthogonal relationship
that real agentic systems have: an operation is not just "actor A holds
capability C" — it is frequently "actor A holds capability C and is
exercising it **because some other party asked it to**." Phase 1/2 cannot
express, and therefore cannot check, whether that other party actually had
standing to ask. A perfectly legitimate agent (correct scope, correct
target, valid delegation chain) can still be a confused deputy: induced by
a less-privileged caller into exercising authority that caller never held.
`docs/phase-1-plan.md` §21 named this "confused-deputy detection" and
explicitly deferred it, noting it "needs a notion of caller distinct from
delegator... a genuinely new relationship on top of the delegation graph,
not an extension of `DA(n)`." `docs/phase-2-plan.md` §23/§26 confirmed the
same boundary and additionally clarified that target binding ("where") and
confused-deputy ("who") are orthogonal — Phase 2's capability-tuple
generalization neither helps nor hinders this phase.

Phase 3's job is to add the smallest rigorous concept that answers *who
caused this operation, and did they have standing to?* — reusing, not
replacing, everything Phase 1/2 already compute. Everything else —
approvals, revocation, temporal state, multi-hop request chains, runtime
enforcement — remains future work (§28).

---

## 1. Phase 1/2 baseline

Verified against the actual merged implementation on `main`
(commit `2db7571`), not just the plan documents:

- **Model types**: `internal/model/types.go` (`Model`, `Principal`,
  `Agent`, `Delegation`, `Operation{Actor, Action, Requires}` — v1, bare
  scope strings) and `internal/model/types_v2.go` (`ModelV2`,
  `Capability{Scope, Target}`, `PrincipalV2`, `AgentV2`, `DelegationV2`,
  `OperationV2{Actor, Action, Requires, Target}`). The two schemas share no
  struct type.
- **Loader dispatch**: `internal/loader/loader_v2.go`'s `LoadDocument`
  peeks `{"version": string}` permissively via a throwaway
  `json.Unmarshal`, then dispatches: `"1"` → `decodeAndValidateV1` (calls
  the original, untouched `validate`), `"2"` → `decodeAndValidateV2` (calls
  `validateV2`), anything else → one `KindInvalidVersion` error with
  message `` `version must be "1" or "2", got %q` ``. Returns a `*Document{V1, V2}`
  union with exactly one field set. `validateV2` duplicates most of
  `validate`'s structural checks (id/scope/edge/cycle/depth) with
  capability-tuple-shaped variants (`checkCapabilitySet`, `checkTarget`),
  not by calling into `validate`.
- **Graph**: `internal/graph/graph.go` — `TopoSort` (Kahn's algorithm,
  lexicographic tie-break via a min-heap), `LongestPath` (DAG DP),
  `CanonicalTrace` (BFS from all principals, sorted expansion,
  first-path-wins, returns `[]string{actor}` if unreachable). All three
  operate purely on node ids and `[]graph.Edge{From, To}` — **no
  dependency on what authority an edge carries**. Untouched by Phase 2,
  reusable as-is by Phase 3.
- **Verify**: `internal/verify/verify.go`'s `Run(*model.Model)` (v1,
  scope-string DA) and `internal/verify/verify_v2.go`'s
  `RunV2(*model.ModelV2)` (v2, capability-tuple DA). Both: one topological
  pass builds `da map[string][]T` for every node (principals get their
  declared set; agents union in only the capability sets of edges whose
  entire declared set is a subset of the delegator's own `da` entry — all
  other edges are **strictly distrusted**, contributing nothing, not even
  the overlapping part). Then one pass over `m.Operations` checks
  `requires ∈ da[actor]`. `RunV2` additionally classifies a miss via
  `classifyOne`/`classifyEdge` (§8 of `docs/phase-2-plan.md`):
  `heldTargetsForScope(scope, held) == ∅` → `authority_amplification`,
  else → `context_binding_violation`.
- **Report**: `internal/report/finding.go` (`EdgeFinding`,
  `OperationFinding`, the `sortKey{point, subject, secondary, scope,
  target}` total order, `Sort([]interface{})` via `keyOf`/`less`
  type-switches) and `internal/report/capability_finding.go`
  (`CapabilityEdgeFinding`, `CapabilityOperationFinding`,
  `ViolationContextBinding`). `internal/report/text.go`'s `RenderText` and
  `internal/report/json.go`'s `RenderJSON` both switch on finding
  concrete type; `RenderJSON`'s envelope (`{"result", "findings"}`) is
  generic over `[]interface{}` and needs no change for a new finding type.
- **CLI**: `cmd/delegationproof/main.go`'s `runVerify` already contains
  exactly the dispatch point Phase 3 needs: `switch { case doc.V1 != nil:
  ... case doc.V2 != nil: ... }`. No new subcommands, no new flags.
- **Limits**: `internal/limits/limits.go` — all bounds are exported
  `var`s; `MaxTargetLength` was Phase 2's only new bound, added by
  mirroring `MaxIDLength`.
- **Tests**: `internal/loader/loader_v2_test.go` line 74 **asserts the
  exact literal invalid-version message text**
  (`` `version must be "1" or "2", got "9"` ``) — this is a concrete fact
  Phase 3 must account for (§9, §23), not an assumption from the plan
  documents alone.

Phase 3 must not modify any Phase 1 or Phase 2 production code path, and
must touch only the one sanctioned message-text line identified above.

---

## 2. Confused-deputy threat

Phase 1 answers: *does billing-agent hold `billing:refund`?*
Phase 2 answers: *is billing-agent's `billing:refund` valid for
`billing-service`?*

Neither can answer: *who caused billing-agent to exercise it right now?*

Concretely: `billing-agent` legitimately holds `billing:refund@billing-service`
via a valid delegation chain from `admin`. Both Phase 1 and Phase 2 report
`ALLOW` for a `refund` operation performed by `billing-agent` — correctly,
by their own invariants. But if the actual initiator of that specific
refund was `support-agent`, which only ever received
`billing:read@billing-service`, the system has a real problem: a
low-privilege caller induced a high-privilege agent into exercising
authority the caller never had. This is the classical confused-deputy
pattern (Hardy, 1988), and it is invisible to both existing invariants
because neither one has a concept of "the party the operation is being
performed *for*" — only "the party performing it."

---

## 3. Minimal new abstraction

Evaluated, per the task's candidate list: `requester`, `caller`,
`initiator`, `on_behalf_of`, a request edge, an invocation edge.

**Decision: exactly one new field, `requester`, added to the Operation
entity only.** No new node kind, no new edge kind, no invocation/call
graph.

Rejected alternatives and why:

- **A request/invocation edge (`requester → actor`), stored as its own
  array alongside `delegations`.** Rejected. An edge implies a *set* of
  request relationships accumulating meaning over the graph (transitivity,
  multi-hop chains, its own validity rules) — exactly the "full runtime
  call graph" the task instructs against building unless verification
  actually requires it. Verification does not: the invariant (§7) only
  ever needs to ask "does this one named requester independently hold
  standing?", a single lookup against a value (`DA(requester)`) the
  existing algorithm already computes for every node, principal or agent,
  whether or not that node is ever an `actor`. No edge, no adjacency, no
  traversal is added by asking that question once per operation.
- **A generic `on_behalf_of` node attribute (declared once per agent,
  meaning "this agent always acts on behalf of X").** Rejected. This
  conflates a *static* per-node property with what is actually a
  *per-operation* fact: the same `billing-agent` legitimately serves many
  different requesters across different operations (that is the entire
  point of it being a shared, reusable agent). Binding "on behalf of whom"
  to the node rather than the operation would be actively wrong — it is
  exactly analogous to why Phase 2 rejected a target field on the
  delegation edge instead of the capability tuple (`docs/phase-2-plan.md`
  §4): the granularity has to match where the fact actually varies.
- **Separate `caller`/`initiator` terminology instead of `requester`.**
  Considered and rejected as a naming-only question: "requester" most
  directly names the role being modeled (the party a request is made *on
  behalf of*), with no protocol-specific connotation ("caller" leans
  RPC/stack-frame; "initiator" is vaguer). One field, one name.
- **`actor`/`requester`/`action`/`requires` shape suggested in the task
  brief.** Evaluated directly: **sufficient**. `actor` (existing) = the
  node actually performing the operation and whose Derived Authority is
  checked against Phase 1/2's invariants, unchanged. `requester` (new) =
  the principal or agent on whose behalf the operation is performed.
  `action`/`requires`/`target` (existing, v2-shaped) = unchanged. Nothing
  else is needed to state or check the invariant in §7.

This keeps Phase 3 exactly the same *shape* of addition Phase 2 was: one
new atomic field flowing through an existing algorithm, no new graph
entity, no new detection machinery.

---

## 4. Request provenance model

`requester` is a **reference to an existing node id** (principal or
agent) — the same id namespace `actor` already draws from. It carries no
new identity concept, no session, no token. Declaring a requester does not
create a new fact about the graph's topology; it only asks a question
about a value the graph already determines: *what is `DA(requester)`?*

**Single canonical requester, not a chain.** Each `OperationV3` entry
declares exactly one `requester` — not a list, not a chain of
`induced_by → induced_by → ...` relationships. This is deliberately the
smaller of two designs (see §17's explicit non-goal and the rationale
there): a single requester field answers "does the party this action is
actually for have standing," which is the confused-deputy question as
posed in `docs/phase-1-plan.md` §21 and this task's brief. A multi-hop
induction chain would answer a different, harder, and currently
unmotivated question ("was every intermediate inducer itself properly
authorized to induce the next"), requiring a new graph entity Phase 3 does
not need.

**No effect on `DA(n)` propagation.** `requester` is checked, never
propagated. A requester's own Derived Authority is computed by the
unmodified Phase 1/2 algorithm, exactly as it already is for every
principal and agent, whether or not that node ever appears as an
`operations[].actor` or `operations[].requester`. Declaring an operation
with a given requester does not grant that requester anything, does not
grant the actor anything on the requester's behalf, and does not change
any other node's `DA`. This directly answers the task's §3 question:
request provenance is **orthogonal metadata checked at operation time**,
not a third input to `DA(n)` and not a separate graph.

---

## 5. Schema v3

**Decision: a new schema version literal, `"3"`, decoded into a new,
structurally disjoint `model.ModelV3`, alongside untouched `"1"` and
`"2"`.** Same reasoning `docs/phase-2-plan.md` §9 already established for
adding `"2"`: `version` is checked by hard equality specifically so a new
semantic shape never silently reinterprets old or new documents under the
wrong rules; a v3-shaped `requester` field must not be interpretable by
v2 code, and a v2 document must never be expected to carry one.

`ModelV3` is `ModelV2` with exactly one change: `OperationV3` gains a
required `Requester` field. Principals, agents, and delegations are
byte-for-byte identical in shape to their v2 counterparts (capability
tuples, target grammar, all unchanged) — Phase 3 adds nothing to the
authority-representation or delegation-semantics layer at all.

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

**`requester` is required, with no default and no implicit
self-reference.** Considered and rejected: making `requester` optional,
defaulting a missing value to `actor` ("no requester declared means the
actor is acting for itself"). This was rejected for the identical reason
`docs/phase-2-plan.md` §9 rejected an optional/defaulted `target`: it
reintroduces exactly the "two representations, same version, different
meaning" ambiguity `DisallowUnknownFields` and Phase 1's strict-decode
philosophy exist to prevent. If an actor genuinely acts for itself, the
document says so explicitly (`"requester": "<same-id-as-actor>"`) — a
trivial, always-passing case (§10), not a silent default.

**Why not stretch `"2"` instead of minting `"3"`.** An optional
`requester` field bolted onto `OperationV2` under the same version literal
would be the same silent-reinterpretation problem: an existing v2 consumer
parsing a file that now sometimes carries a `requester` key would have no
version-level signal that new semantics exist. A new literal is the
mechanism Phase 1/2 already built for exactly this.

**Dispatch mechanism**, extending `LoadDocument`'s existing switch
(`internal/loader/loader_v2.go`):

```
"1"          -> decodeAndValidateV1 (unchanged)
"2"          -> decodeAndValidateV2 (unchanged)
"3"          -> decodeAndValidateV3 (new)
anything else (including "") -> one KindInvalidVersion error
```

`Document` grows a third field: `Document{V1, V2, V3 *model.ModelV3}`,
exactly one of which is set on success.

**The one sanctioned message-text touch, precisely identified.** The
`invalid_version` message must change from
`` `version must be "1" or "2", got %q` `` to
`` `version must be "1", "2", or "3", got %q` ``, in exactly two call
sites that must stay textually identical to each other (both `validate`'s
message and `validateV2`'s message already duplicate this string
verbatim; Phase 3 adds a third copy in `validateV3` and updates the first
two). This requires editing `internal/loader/loader_v2_test.go` line 74
(`want := `version must be "1" or "2", got "9"``) to the new three-version
text — **an existing Phase 2 test file changes**, which is new relative
to Phase 2's own regression claim ("zero modification to any pre-existing
Phase 1 test file") and must be called out explicitly rather than silently
assumed away (§23). No other line in `loader_v2_test.go`, and no line in
any `_test.go` file for `internal/graph`, `internal/verify`,
`internal/report`, or `cmd/delegationproof`, may change.

---

## 6. Authority semantics

Unchanged from Phase 2, verbatim, for the `actor` side of every check.
`DA(n)` for a version-3 model is computed by the **identical algorithm**
Phase 2 already runs (topological pass, strict-distrust subset check over
capability tuples) — Phase 3 introduces no new element type into the
authority representation, no new grammar, no new resource-limit-relevant
dimension. `requester` is not an authority-bearing field; it never appears
inside a `Capability`, a delegation's `authority` array, or a principal's
declared set. It is purely a second node-id reference alongside `actor` on
the `Operation` entity, and its authority is looked up via the exact same
`da` map `actor`'s is.

---

## 7. New formal invariant

> **Requester Authorization Preservation:** for every version-3 operation
> `op = (actor, requester, action, requires_scope, requires_target)`, let
> `C = (requires_scope, requires_target)`. If `C ∈ DA(actor)` (i.e. Phase
> 1/2's invariants are themselves satisfied for this operation), then it
> must also hold that `C ∈ DA(requester)`. If `C ∈ DA(actor)` but
> `C ∉ DA(requester)`, the operation is a confused-deputy violation: a
> validly-authorized actor is being induced to exercise a capability the
> requester was never independently granted.

Two things this statement deliberately does **not** say, addressed
directly per the task's §2 worked scenario:

1. **It does not require `requester` to be an ancestor of `actor` in
   `actor`'s own specific delegation chain.** `DA(requester)` is computed
   independently, via whatever valid chain(s) reach `requester` anywhere
   in the graph — which may share no edges at all with the chain that
   grants `actor` its authority. This is the correct and intentionally
   looser formulation: the real-world question is "does the requester
   have standing to authorize this," not "did the requester specifically
   hand this exact grant to this exact actor." A root principal
   requesting an operation performed by an agent it never directly
   delegated to (but which is itself validly authorized) is legitimate
   precisely because `DA(root)` already contains the capability.
2. **It is not evaluated at all when `C ∉ DA(actor)`.** If the actor
   itself does not validly hold the capability, Requester Authorization
   Preservation is not the applicable diagnosis — Phase 1/2's own
   invariant is already violated, and *that* is reported (§8, §12).
   Asking "was the actor induced by an under-authorized requester to do
   something it doesn't even legitimately hold" is a moot second question
   once the first, more foundational one already has a negative answer.

Re-examining the task's own worked scenario against this statement: a
principal delegates `billing:refund@billing-service` to a trusted
`billing-agent`; a requester holding only `billing:read@billing-service`
attempts to induce a refund via `billing-agent`. `C = billing:refund@billing-service`.
`C ∈ DA(billing-agent)` (the delegation is valid) — Phase 1/2 pass, so
the requester check applies. `C ∉ DA(requester)` (requester only ever
held `billing:read`) — confused-deputy violation, exactly matching the
task's stated expectation ("likely no"). Conversely, if that same
requester had *also* independently received `billing:refund@billing-service`
through a valid chain elsewhere in the graph, `C ∈ DA(requester)` would
hold and the operation is legitimate — matching the task's second stated
expectation ("if authority was legitimately delegated to that requester
... it may be valid").

---

## 8. Composition with Phase 1/2 invariants

Precedence is **strict and total**, evaluated in this order for every
version-3 operation, producing **at most one finding per operation** (no
new precedence ambiguity is introduced — this extends, rather than
parallels, Phase 2's own single-finding-per-edge precedence rule in
`docs/phase-2-plan.md` §8):

```
evaluate(op, da):
    C = Capability{op.Requires, op.Target}

    // Step 1 — unchanged Phase 1/2 check, unchanged classification.
    if C not in da[op.Actor]:
        violation, boundTargets = classifyOne(op.Requires, da[op.Actor])   // §8, docs/phase-2-plan.md
        emit CapabilityOperationFinding(violation, ...)                    // authority_amplification | context_binding_violation
        return                                                             // requester is NOT evaluated

    // Step 2 — new Phase 3 check, only reached if step 1 passed.
    if C in da[op.Requester]:
        return   // ALLOW, no finding

    // Step 3 — actor legitimate, requester lacks standing.
    boundTargets = heldTargetsForScope(op.Requires, da[op.Requester])
    emit ConfusedDeputyFinding("confused_deputy", ...)
```

**Rationale for "actor problem masks requester problem," restated.** This
is the identical masking principle Phase 2 already applies at the edge
level ("the more foundational problem takes precedence and is never
masked by a co-occurring... issue," `docs/phase-2-plan.md` §8), lifted one
level: an actor-side amplification or binding failure is strictly more
foundational than a requester-side standing failure, because fixing the
requester problem alone would still leave a broken operation. Reporting
both would not add diagnostic value — it would report the same operation
twice under two different theories of what's wrong, which the task
explicitly warns against ("do not produce ambiguous duplicate findings
unless deliberately justified").

Concrete precedence examples (also §16 test cases):

| Actor holds `C`? | Requester holds `C`? | Finding |
|---|---|---|
| No (scope never held, any target) | — (not evaluated) | `authority_amplification` |
| No (scope held, wrong target only) | — (not evaluated) | `context_binding_violation` |
| Yes | Yes | none — `ALLOW` |
| Yes | No (scope never held, any target) | `confused_deputy` |
| Yes | No (scope held, wrong target only) | `confused_deputy` (§7's reason text distinguishes the two sub-cases in prose; the violation literal does not — see §12) |

Delegation edges are **entirely unaffected**. Edges have no `requester`
field and no requester concept — Requester Authorization Preservation is
defined only over operations, because "who is this authority being
*exercised* for" is only a meaningful question at the point of exercise,
not at the point of grant. `CapabilityEdgeFinding` and its `classifyEdge`
precedence rule (`docs/phase-2-plan.md` §8) are untouched.

---

## 9. Strict-distrust semantics

Unchanged, and — critically — **requires no new code to enforce**, because
`DA(requester)` is computed by literally the same topological pass and the
same `isSubsetCap`/strict-distrust rule that already computes
`DA(actor)`. An invalid incoming edge on the path to `requester`
contributes nothing to `DA(requester)`, not even a scope requester holds
under some other target — identical to how it already contributes
nothing to `DA(actor)`. A confused-deputy finding computed against a
requester whose only apparent grant arrived over an invalid edge is
therefore automatically, correctly reported — the requester's apparent
authority was never real to begin with. This is directly testable (§16):
a requester reachable only via a distrusted edge must produce the same
`confused_deputy` outcome as a requester with no relevant grant at all.

---

## 10. Operation/requester semantics

- **Entity kinds that may be a requester:** principals and agents — the
  same universe `actor` already draws from, and the same universe
  `operations[].actor` has always accepted (`docs/phase-1-plan.md` §7.2
  already permits a principal as `actor`, "checking a principal's own
  operations is legal and trivially passes since `DA(principal)` is
  axiomatic" — the identical reasoning now applies to `requester`).
- **`requester == actor` is legal and is the trivial-pass case.** No
  special-cased code path: `da[op.Requester]` and `da[op.Actor]` are the
  same map lookup with the same key, so `C ∈ DA(actor)` and
  `C ∈ DA(requester)` are literally the same boolean when the two ids are
  equal. An actor genuinely acting on its own behalf must still write
  `"requester": "<its own id>"` explicitly (§5) — no implicit
  self-reference.
- **Every v3 operation must declare a requester.** No optional/omitted
  form (§5).
- **Requester authority is computed exactly as any node's `DA(n)` is** —
  no separate function, no separate pass; §6, §9.
- **Unreachable/untrusted requesters:** a requester unreachable from any
  principal via valid edges has `DA(requester) = ∅` (the existing,
  unmodified behavior for any orphan agent) — since `∅` cannot contain any
  capability, every such operation where the actor legitimately holds `C`
  is unconditionally a `confused_deputy` finding. No special-casing
  needed; this falls directly out of the existing `da` map's default
  behavior for a node with no valid incoming edges.
- **Requester identity must already exist in the model.** `requester`
  must resolve to a declared principal or agent id, exactly like `actor`
  (§12).

---

## 11. Verification algorithm

**Decision: Phase 3 still fits entirely within static, deterministic,
single-pass verification. No state-space exploration, no backtracking, no
second graph.**

`RunV3(*model.ModelV3) report.Result`, structurally identical to `RunV2`
through DA computation, then extended at the operation-evaluation step:

1. **Build the graph and compute `da`.** Byte-for-byte the same steps
   `RunV2` already performs: nodes = principals ∪ agents, edges =
   delegations, one topological pass (ascending-lexicographic tie-break,
   unchanged `graph.TopoSort`) computing `da[n]` for **every** node,
   independent of whether that node is ever referenced by an operation as
   `actor` or `requester`. This single pass already produces
   `DA(requester)` for every possible requester before any operation is
   evaluated — no additional graph work is triggered by Phase 3.
2. **Operation evaluation**, operations in the order needed for
   deterministic finding assembly (§12's extended sort key, applied after
   collection — the same "collect during a single forward pass, then sort"
   structure `RunV2` already uses, not a pre-sort of operations
   themselves): for each operation, run the precedence algorithm in §8,
   producing zero or one finding.
3. **Sort all findings** (`CapabilityEdgeFinding`, `CapabilityOperationFinding`,
   `ConfusedDeputyFinding` together) by the extended key (§13).
4. **Result:** `ALLOW` (exit 0) if empty, else `DENY` (exit 1) — unchanged
   result semantics.

**Complexity: unchanged, `O(N + E + O)`.** Requester evaluation adds one
extra map lookup (`da[op.Requester]`) and, only on the confused-deputy
path, one extra `graph.CanonicalTrace` call (§14) — a bounded, `O(N + E)`
operation already used twice per finding in Phase 1/2. No branching over
alternative interpretations is introduced; every quantity (`DA(n)`, the
precedence outcome for a given operation) still has exactly one correct
value given the input, for the same reason `docs/phase-1-plan.md` §8.2
gives: no time dimension, no conditional grant, no approval workflow, no
revocation. `requester` adds a second *reference* to check per operation,
not a *state* to search over.

`model.Model` (v1) and `model.ModelV2` continue to run their existing,
untouched `Run`/`RunV2` functions, byte-identical to today.

---

## 12. Violation classification/precedence

Extends Phase 2's `classify`/`classifyOne`/`classifyEdge` (all unchanged,
called verbatim for the actor-side check) with one new, separate
classifier that only runs once step 1 of §8 has already passed:

```
classifyRequester(scope, requesterHeld) -> (violation, requesterBoundTargets):
    // Deliberately NOT reusing classifyOne's two-way split (amplification
    // vs context-binding) for the requester side: that split answers "why
    // does the ACTOR's own DA not contain this," a question about the
    // actor's delegation history. The requester-side question is always
    // the same question regardless of *why* the requester lacks
    // standing — "does this requester have standing to cause this
    // specific actor to do this specific thing" — so it always yields the
    // single literal "confused_deputy". heldTargetsForScope is still
    // computed and carried in the finding (§13) purely for diagnostic
    // reason text (§14), not to select a different violation literal.
    targets = heldTargetsForScope(scope, requesterHeld)
    return "confused_deputy", targets
```

**Why the requester side is not sub-classified into an
amplification/binding split of its own** (a design point the task
explicitly asks to be examined, §5/§12): doing so would produce, for the
identical underlying fact pattern, a violation literal that depends on
*which side of the actor/requester pair* failed — `authority_amplification`
already means "actor's own DA lacks this scope entirely" and
`context_binding_violation` already means "actor's own DA has it, wrong
target." Reusing those same two literals to describe a *requester's* DA
gap would silently overload their meaning: a consumer parsing
`"violation": "authority_amplification"` could no longer tell, without
also checking `point` and which id fields are populated, whether the
*actor* or the *requester* was the one lacking the scope. A single,
distinct `confused_deputy` literal keeps every existing literal's meaning
exactly what it has always meant, and keeps the new one unambiguous by
construction, at the small cost of putting the finer "never held / held,
wrong target" distinction into `reason` text and the `requester_bound_targets`
field instead of the `violation` field.

**Full deterministic precedence, actor-side then requester-side, restated
as the total classification function:**

```
classifyOperationV3(op, da):
    heldByActor = da[op.Actor]
    C = Capability{op.Requires, op.Target}
    if C not in heldByActor:
        return classifyOne(op.Requires, heldByActor)          // §8 docs/phase-2-plan.md, unchanged
    heldByRequester = da[op.Requester]
    if C in heldByRequester:
        return PASS
    return classifyRequester(op.Requires, heldByRequester)     // new, §12 above
```

No ambiguous or duplicate findings are possible: the function returns
exactly one outcome (`PASS` or exactly one violation literal) per
operation, by construction.

---

## 13. Deterministic findings

One new finding type, alongside Phase 1/2's unmodified `EdgeFinding` /
`OperationFinding` / `CapabilityEdgeFinding` / `CapabilityOperationFinding`:

```go
// internal/report/confused_deputy_finding.go

const ViolationConfusedDeputy = "confused_deputy"

type ConfusedDeputyFinding struct {
    Violation             string       `json:"violation"`               // always "confused_deputy"
    Point                 string       `json:"point"`                    // always "operation"
    Actor                 string       `json:"actor"`
    Requester              string      `json:"requester"`
    Action                 string      `json:"action"`
    Requires                Capability `json:"requires"`
    ActorHeld              []Capability `json:"actor_held"`
    RequesterHeld           []Capability `json:"requester_held"`
    RequesterBoundTargets  []string     `json:"requester_bound_targets"` // targets requester holds requires.Scope for; [] if none
    ActorTrace             []string     `json:"actor_trace"`
    RequesterTrace          []string    `json:"requester_trace"`
    Reason                  string      `json:"reason"`
}
```

`point` reuses Phase 1/2's existing `"operation"` literal unchanged — no
new detection point is introduced (§8: confused-deputy is only ever an
operation-level finding, never edge-level).

**Deterministic reason text** (generated, not free-form, same discipline
as `docs/phase-1-plan.md` §9 / `docs/phase-2-plan.md` §12), distinguishing
the two requester-side sub-cases in prose only (§12):

- `requester_bound_targets` empty (requester never held the scope, any
  target):
  `"<action> requires <scope>@<target>, which <actor> validly holds, but
  requester <requester> has never held <scope> under any target — <actor>
  is being induced to exercise authority <requester> was never granted"`
- `requester_bound_targets` non-empty (requester holds the scope, wrong
  target only):
  `"<action> requires <scope>@<target>, which <actor> validly holds, but
  requester <requester> holds <scope> only for [<requester_bound_targets
  joined by ", ">], which does not include <target> — <actor> is being
  induced to exercise authority <requester> was never granted for this
  target"`

`actor_held`/`requester_held`/`requester_bound_targets` are always present
(`[]`, never omitted or null — Phase 1 §9's array-field rule, unchanged).

---

## 14. Counterexample traces

**Two traces, not one — deliberately, per the task's §8 prompt to examine
this.** `actor_trace = graph.CanonicalTrace(principalIDs, validEdges,
actor) + [action]` and `requester_trace =
graph.CanonicalTrace(principalIDs, validEdges, requester)` (no `+ action]`
suffix — the requester did not perform the action, it only stands or
fails to stand behind it; appending `action` to the requester's trace
would misleadingly imply the requester was the one exercising the
operation). Both reuse the exact same unmodified `graph.CanonicalTrace`
helper Phase 1/2 already call twice per finding elsewhere in the codebase
— no new trace-construction logic (mirrors `docs/phase-2-plan.md` §13's
own conclusion that `CanonicalTrace` needed no changes for Phase 2
either).

**Why two traces are justified, not noise.** They answer two different,
independently useful questions, not the same fact restated: `actor_trace`
demonstrates *the actor did nothing wrong* — it shows the valid delegation
chain that legitimately granted it the capability (useful to rule out an
actor-side fix). `requester_trace` demonstrates *why the requester lacks
standing* — either `[requester]` (never reachable via any valid edge:
§9) or a real path whose accumulated capability set demonstrably excludes
`C`. Collapsing to one trace would force a reader to guess which of the
two questions it answers. This is different from Phase 1/2's own findings,
which only ever needed one trace because they only ever had one node in
question.

`validEdges` reuse is identical to Phase 2's rule (`docs/phase-2-plan.md`
§13): only edges that passed the **entire** capability-subset check
contribute to either trace; a context-binding-invalid or
amplification-invalid edge is excluded from both, exactly as it already
is from every existing trace.

---

## 15. Validation

Every existing Phase 1/2 structural rule applies to version-3 documents
unchanged, generalized only where the shape changed (principals, agents,
delegations are identical to v2; the size/grammar bounds on capability
tuples, targets, ids, actions are all reused verbatim).

**Exactly one new structural error kind:** `unknown_requester`, mirroring
`unknown_actor` precisely — `requester` must resolve to a known node id
(principal or agent).

```go
KindUnknownRequester ErrorKind = "unknown_requester"
```

**Explicitly evaluated and rejected as separate error kinds** (the task's
suggested list in §12 of the brief is not adopted wholesale — only what
this model actually justifies, mirroring `docs/phase-2-plan.md` §10's own
discipline):

- **"Missing requester."** Not a separate kind. A missing `requester` key
  decodes as `""`, which — exactly like a missing `target` in Phase 2 —
  can never resolve to a known node id, so it falls straight into
  `unknown_requester` with no dedicated "missing field" mechanism needed
  (identical precedent: Phase 2 explicitly declined a separate
  "missing target" kind for the same reason).
- **"Invalid requester format."** Not a separate kind. `requester`'s
  grammar is the node-id grammar, already enforced at the point a
  principal/agent id is *declared*; a syntactically-malformed requester
  string can, by construction, never match a registered node id either,
  so it is caught by `unknown_requester`, exactly as a malformed `actor`
  reference already is today (Phase 1 has never had a separate
  `invalid_actor_format` kind, for the identical reason).
- **"Duplicate operation identity."** Not introduced. Phase 1/2 never
  prohibited duplicate `(actor, action, requires)` operation entries (an
  actor may legitimately perform the same action twice, or be checked
  from two different callers' perspectives) — Phase 3 does not change
  this. Two v3 operations may legitimately share `actor`/`action`/`requires`/`target`
  and differ only by `requester` — a new, real, and valid case (§16
  determinism, §13's sort key), not a validation error.
- **"Requester lacks standing" as a validate-time (exit 2) error.**
  Rejected outright, on the identical precedent Phase 1 §7.4 and Phase 2
  §10 both already establish: a structurally well-formed document that
  turns out to violate a semantic invariant is a `verify`-time finding
  (exit 1), never a `validate`-time structural error (exit 2). Treating a
  confused-deputy pattern as a structural problem would be the same
  category error `docs/phase-2-plan.md` §10 already rejected for
  context-mismatched targets.

`validate` on a version-3 document therefore still never evaluates any
invariant — Non-Amplification, Context-Binding, or Requester Authorization
Preservation — exactly as established for v1/v2.

---

## 16. CLI compatibility

**No new subcommands, no new flags.** `validate <model.json>` and
`verify <model.json> [--format text|json]` remain the only two commands,
unchanged in name and shape. `main.go`'s existing `switch { case doc.V1
!= nil ...; case doc.V2 != nil ... }` in `runVerify` gains one more case:

```go
case doc.V3 != nil:
    result = verify.RunV3(doc.V3)
```

`--format text|json` applies identically across all three versions. No
`confused-deputy` subcommand, no `--requester` override flag, no
version-selection flag — version is read from the document, exactly as
`"1"`/`"2"` already are.

---

## 17. Text/JSON compatibility

**JSON.** The top-level envelope (`{"result": "ALLOW"|"DENY", "findings":
[...]}`, `internal/report/json.go`) is unchanged — it is already generic
over `[]interface{}`, so a new finding type requires zero changes to
`RenderJSON`. Version-1 and version-2 output is byte-identical to today,
unconditionally: `RunV3` is a new function, called only when `doc.V3 !=
nil`, and touches no code path `Run`/`RunV2` execute.

**Text.** `RenderText`'s type switch (`internal/report/text.go`) gains one
new case, matching the label-column style already used for
`CapabilityEdgeFinding`/`CapabilityOperationFinding` (`%-14s` field
labels):

```
[1] confused_deputy (operation)
  actor:            billing-agent
  requester:        support-agent
  action:            refund-b
  requires:          billing:refund@billing-service
  actor held:        billing:refund@billing-service
  requester held:    billing:read@billing-service
  requester bound:   billing-service
  actor trace:       admin -> billing-agent -> refund-b
  requester trace:   admin -> support-agent
  reason:            refund-b requires billing:refund@billing-service, which billing-agent validly holds, but requester support-agent holds billing:read only for billing-service, which does not include billing:refund — billing-agent is being induced to exercise authority support-agent was never granted for this target
```

(Exact column labels/widths are an implementation-session detail, not a
plan-level commitment beyond "consistent with the existing v2 label
style" — same latitude Phase 2's own plan left for its text rendering.)

---

## 18. Exit codes

Unchanged. `internal/exitcode` gains no new values:

| Code | Meaning (extended) |
|---|---|
| `0` | Structurally valid model (v1/v2/v3); zero findings for `verify`. |
| `1` | One or more findings — `authority_amplification`, `context_binding_violation`, and/or `confused_deputy`, in any combination. A v3 model can `DENY` on any mix; the exit code does not distinguish which. |
| `2` | Structural/model problem for any schema version, including the new `unknown_requester` kind. |
| `3` | CLI usage error — unchanged. |

---

## 19. Resource bounds

**No new bound.** `requester` is a reference to an existing node id,
validated by the same mechanism (`unknown_requester` resolution against
the already-bounded node set) `actor` already uses — it adds no new
countable quantity to the model (not a new array, not a new per-node
collection). `MaxOperations` already bounds the number of operation
entries, and one v3 operation entry is still exactly one entry regardless
of whether it now carries a `requester` field. This directly follows the
task's own steer (§11): "if requester is merely another existing ID
reference, Phase 3 may require no new major graph-size limit" — confirmed
correct after inspecting the actual `da` map's construction (§1, §11),
which already computes `DA` for every node up front regardless of
operation count.

All existing Phase 1/2 bounds (`MaxInputFileSize`, `MaxNodes`,
`MaxDelegationEdges`, `MaxOperations`, `MaxScopeLength`, `MaxIDLength`,
`MaxAuthoritySetSize`, `MaxChainDepth`, `MaxTargetLength`) apply to
version-3 documents unchanged.

---

## 20. Worked example

`examples/billing-confused-deputy.json` (implementation-session file,
matching the task's §15 scenario exactly, deliberately small):

```json
{
  "version": "3",
  "principals": [
    { "id": "admin", "authority": [
      { "scope": "billing:refund", "target": "billing-service" },
      { "scope": "billing:read",   "target": "billing-service" }
    ] }
  ],
  "agents": [
    { "id": "billing-agent" },
    { "id": "support-agent" }
  ],
  "delegations": [
    { "delegator": "admin", "delegatee": "billing-agent", "authority": [
      { "scope": "billing:refund", "target": "billing-service" }
    ] },
    { "delegator": "admin", "delegatee": "support-agent", "authority": [
      { "scope": "billing:read", "target": "billing-service" }
    ] }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin",         "action": "refund-a", "requires": "billing:refund", "target": "billing-service" },
    { "actor": "billing-agent", "requester": "support-agent", "action": "refund-b", "requires": "billing:refund", "target": "billing-service" }
  ]
}
```

`verify examples/billing-confused-deputy.json`:

- **`refund-a`** (requester `admin`) — `billing-agent` validly holds
  `billing:refund@billing-service` (Phase 1/2 pass). `admin`, as a
  principal, axiomatically holds the same capability directly — `C ∈
  DA(admin)`. **Passes**, no finding.
- **`refund-b`** (requester `support-agent`) — `billing-agent` still
  validly holds `billing:refund@billing-service` (identical actor-side
  check, same result). `support-agent` only ever received
  `billing:read@billing-service` — `C ∉ DA(support-agent)`. **Fails**:
  `confused_deputy`, with `requester_bound_targets = ["billing-service"]`
  (support-agent does hold *some* capability for that target, just not
  this scope) and `requester_trace = ["admin", "support-agent"]`.

This single file demonstrates: a legitimate root-requester operation, and
a confused-deputy violation, using the same actor and the same nominal
capability in both operations so the *only* difference is `requester` —
mirroring exactly how Phase 1's and Phase 2's own worked examples isolate
the one variable each phase's invariant is meant to catch.

---

## 21. Architecture/file plan

Purely additive to `docs/phase-1-plan.md` §15 / `docs/phase-2-plan.md`
§19. No existing file is deleted or renamed; the one sanctioned message
touch is called out explicitly (§5, §23).

```
internal/model/
  types.go                — UNCHANGED (v1)
  types_v2.go              — UNCHANGED (v2)
  types_v3.go               — NEW: ModelV3, PrincipalV3/AgentV3/DelegationV3
                              (identical shape to their V2 counterparts —
                              could alias, but kept disjoint like V2 kept
                              disjoint from V1, per docs/phase-2-plan.md
                              §9's "no shared internal model type" rule),
                              OperationV3{Actor, Requester, Action,
                              Requires, Target}

internal/limits/
  limits.go                 — UNCHANGED. No new bound (§19).

internal/loader/
  loader.go                  — UNCHANGED except the one sanctioned message
                                text touch (§5, §23) inside validate().
  loader_v2.go                — UNCHANGED except the same message touch
                                inside validateV2(), and LoadDocument's
                                switch gains a "3" case and Document gains
                                a V3 field.
  loader_v3.go                 — NEW: decodeAndValidateV3, validateV3
                                (reuses checkID/checkScope/checkTarget/
                                checkCapabilitySet/resourceLimitErr/
                                sortErrors verbatim, adds one new check:
                                requester resolves to a known node id ->
                                KindUnknownRequester).

internal/graph/
  graph.go                     — UNCHANGED. Reused as-is (§11, §14).

internal/verify/
  verify.go                     — UNCHANGED (v1).
  verify_v2.go                   — UNCHANGED (v2).
  verify_v3.go                    — NEW: RunV3(*model.ModelV3)
                                    report.Result, implementing §8/§11/§12.
                                    Reuses verify_v2.go's unexported
                                    helpers (isSubsetCap, canonicalizeCaps,
                                    heldTargetsForScope, classifyOne,
                                    toReportCaps, etc.) directly — same
                                    package, no duplication needed.

internal/report/
  finding.go                      — ADD: extend sortKey with a trailing
                                    `requester string` field (§13; empty
                                    for every existing finding type, so
                                    this is a strict extension exactly
                                    like Phase 2 adding `target`); extend
                                    keyOf's type switch with one new case.
                                    EXISTING cases/fields untouched.
  capability_finding.go            — UNCHANGED.
  confused_deputy_finding.go        — NEW: ConfusedDeputyFinding,
                                      ViolationConfusedDeputy,
                                      NewConfusedDeputyFinding constructor
                                      (§13).
  text.go                           — extend RenderText's type switch with
                                      the one new finding type (§17);
                                      existing cases untouched.
  json.go                            — UNCHANGED (already generic).

cmd/delegationproof/
  main.go                            — runVerify's dispatch switch gains
                                      one case (§16); no new flags, no new
                                      subcommands, no exit-code changes.

examples/
  billing-refund.json                — UNCHANGED.
  billing-context-binding.json        — UNCHANGED.
  billing-confused-deputy.json         — NEW (§20).

schemas/
  model.md                             — NOT modified this session
                                        (explicit instruction, mirroring
                                        Phase 2's own precedent). The
                                        implementation session must add a
                                        "version 3" section documenting
                                        §5/§10/§15, mirroring how model.md
                                        documents version 2 today.

testdata/
  valid-v3/                             — NEW directory: clean-pass-v3.json,
                                        a reordered-arrays variant (for
                                        permutation-invariance testing,
                                        §22), a combined-violations-v3.json
                                        (amplification + context-binding +
                                        confused_deputy all present, §22),
                                        multi-hop-requester.json.
  malformed/                             — ADD one v3 fixture:
                                        unknown-requester.json. Existing
                                        v1/v2 fixtures UNCHANGED, still
                                        walked automatically by
                                        cmd/delegationproof/main_test.go
                                        (CLAUDE.md's convention extends for
                                        free, exactly as it did for v2).
  golden/                                — ADD captured v3 text/json output
                                        for billing-confused-deputy and a
                                        v3 combined-violations fixture.
                                        Existing v1/v2 golden files
                                        UNCHANGED, byte-identical.

docs/
  phase-3-plan.md                        — this document.
```

---

## 22. Testing plan

Mirrors the structure of `docs/phase-1-plan.md` §16 / `docs/phase-2-plan.md`
§20, additive. Test file names follow the existing `_v2` naming convention
(`verify_v3_test.go`, `loader_v3_test.go`, `main_v3_test.go`).

1. **Full Phase 1 + Phase 2 regression** — `go test ./... -race -count=1`
   with zero behavioral change to any existing test, **except** the one
   sanctioned message-text line in `loader_v2_test.go` (§5, §23); every
   existing golden file byte-identical; every existing malformed fixture
   still produces its original `ErrorKind`.
2. **Valid requester-authorized operation** — `refund-a` from
   `examples/billing-confused-deputy.json` (§20) → no finding.
3. **Confused-deputy violation** — `refund-b` from the same example → one
   `ConfusedDeputyFinding`, golden text+json, exact `reason`/`requester_bound_targets`/
   both traces asserted.
4. **`requester == actor`** — an operation with `requester` set to the
   same id as `actor` → always passes when the actor-side check passes
   (trivial by construction, §10) — dedicated test asserting no finding
   regardless of what capability is required.
5. **Principal requester** — a v3 model where `requester` names a
   principal with the capability declared directly (axiomatic) → passes.
6. **Agent requester** — a v3 model where `requester` names an agent that
   received the capability via its own independent, multi-hop delegation
   chain not overlapping the actor's chain → passes, confirming §7's
   "independently held, not ancestor-of-actor" formulation.
7. **Unknown requester** — `requester` referencing a nonexistent node id
   → `unknown_requester`, exit 2 (`validate` and `verify` both).
8. **Missing requester** — `requester` key omitted entirely → decodes as
   `""`, falls into `unknown_requester` (§15) — dedicated fixture
   confirming no separate error kind fires.
9. **Multi-hop requester authority** — requester's capability arrives via
   a 3+ hop chain; confirms `DA(requester)` tuple propagation is correct
   end-to-end via the unmodified Phase 2 algorithm.
10. **Requester authority arriving through an invalid edge** — requester's
    only apparent grant of the needed capability arrives over an edge that
    is itself distrusted (over-claims relative to its own delegator) →
    `DA(requester)` excludes it → `confused_deputy` fires despite the
    document superficially naming the right scope somewhere upstream of
    requester (§9).
11. **Actor amplification + requester failure combination** — actor does
    not hold the scope at all AND requester (independently) also lacks
    standing → asserts **exactly one** finding (`authority_amplification`),
    not two, confirming §8's precedence/masking rule.
12. **Actor context-binding + requester failure combination** — actor
    holds the scope only for the wrong target AND requester also lacks
    standing → asserts **exactly one** finding
    (`context_binding_violation`), not two.
13. **Deterministic classification precedence** — a dedicated table test
    covering every row of §8's precedence table.
14. **Deterministic findings / sort order** — a v3 model with multiple
    `authority_amplification`, `context_binding_violation`, and
    `confused_deputy` findings mixed together, asserting the extended
    6-tuple sort key (§13) produces a stable, documented order; includes
    two operations sharing `(actor, action, requires, target)` but
    differing only by `requester`, asserting `requester` correctly
    breaks the tie.
15. **Deterministic traces** — asserts `actor_trace` ends with `action`
    and `requester_trace` does not (§14); asserts both are `[node]` when
    unreachable.
16. **Reordered-input invariance** — v3 analogue of the existing
    permutation-invariance test: byte-identical output for
    semantically-equivalent reordered `principals`/`agents`/`delegations`/
    `operations` arrays.
17. **Repeated-run byte determinism** — v3 analogue of
    `TestRunIsDeterministicAcrossRepeatedInvocations`.
18. **Text output** — golden-file test for the worked example (§20) and a
    multi-finding v3 fixture.
19. **JSON output** — golden-file test for the same fixtures; asserts the
    envelope shape is unchanged and `ConfusedDeputyFinding` fields appear
    in the documented order.
20. **CLI exit codes** — `validate` vs `verify` divergence for a v3 model
    containing only a `confused_deputy` finding (structurally valid,
    `validate` → 0, `verify` → 1), mirroring the existing v1/v2 divergence
    tests.
21. **Malformed-input behavior** — `unknown_requester` fixture exercised
    through the full CLI path (exit 2), alongside all existing v1/v2
    malformed fixtures (still walked automatically).
22. **Resource-limit regression** — confirms no new limit is needed
    (§19) by asserting existing bounds (`MaxOperations`, `MaxIDLength`
    applied to `requester`, etc.) still behave correctly on v3 fixtures at
    their existing values.
23. **No-panic malformed-data behavior** — extend the existing
    fuzz/mutation-style CLI test to include v3 fixtures as seeds.

---

## 23. Phase 1/2 regression requirements

- Every existing test in `internal/loader`, `internal/graph`,
  `internal/verify`, `internal/report`, and `cmd/delegationproof` must
  pass, with **exactly one documented exception**: the literal string
  asserted in `internal/loader/loader_v2_test.go` line 74 changes from
  `` `version must be "1" or "2", got "9"` `` to
  `` `version must be "1", "2", or "3", got "9"` `` (§5). This is the only
  sanctioned edit to any pre-existing test file in the repository. No
  other assertion, in that file or any other, may change.
- Every existing golden file in `testdata/golden/` must remain
  byte-identical output for its existing input.
- Every existing fixture in `testdata/malformed/` must continue to
  produce its documented `ErrorKind`.
- `examples/billing-refund.json` and `examples/billing-context-binding.json`
  must continue to round-trip exactly as their respective plan documents
  specify.
- No line in `internal/verify/verify.go`, `internal/verify/verify_v2.go`,
  `internal/graph/graph.go`, `internal/report/capability_finding.go`, or
  any existing `internal/model` type may change.
- `go vet ./...`, `gofmt -l .`, and `go build -o bin/delegationproof
  ./cmd/delegationproof` must all succeed exactly as CLAUDE.md requires
  today, with the new v3 files included.

---

## 24. Security assumptions

Extends `docs/phase-1-plan.md` §17 and `docs/phase-2-plan.md` §22 without
weakening either:

- **A `requester` value is a declared label, not a verified identity or
  authenticated caller.** Exactly as a `target` string is not verified
  against any real service (`docs/phase-2-plan.md` §22), DelegationProof
  does not verify that the real system's actual caller matches the
  document's declared `requester` — that correspondence (real request
  provenance, e.g. an authenticated session or an MCP/A2A call frame,
  mapped into this document's `requester` field) is a separate, later
  integration concern, identical in kind to the existing gap between a
  principal's declared authority and how it was really obtained
  (Phase 1 §17).
- **Requester Authorization Preservation proves a property of the
  declared model only:** "this document never claims a validly-authorized
  actor may be induced by a requester lacking independent standing." It
  does not, and cannot, prove that a real running system actually checks
  or enforces who caused each real operation — DelegationProof remains a
  static, offline analyzer with no interception or enforcement component
  (unchanged, Phase 1 §17/§18).
- **No new attack surface from parsing.** `requester` is decoded via the
  same `encoding/json` + `DisallowUnknownFields` + bounded-read pipeline
  as every other field, subject to the same `MaxInputFileSize` bound
  applied before any structural field is read, and validated by reusing
  the existing node-id resolution mechanism (§15) rather than any new
  parsing logic.

---

## 25. Explicit non-goals

All of `docs/phase-1-plan.md` §18's and `docs/phase-2-plan.md` §23's
non-goals continue to apply. Phase 3 additionally, explicitly, does
**not** include:

- MCP protocol implementation, A2A protocol implementation, OAuth, JWT
  verification, tokens, networking, hosted services, proxying, runtime
  enforcement, databases, LLMs, Z3/SAT/SMT, SARIF, approvals, revocation,
  temporal state, sessions.
- **State-space exploration** — not required (§11) and not added.
- **Multi-hop request/induced-by chains.** Evaluated directly per the
  task's explicit prompt to do so (§17's brief) and rejected for Phase 3:
  a single canonical `requester` reference per operation is sufficient to
  state and check Requester Authorization Preservation (§7). A chain
  (`requester` induced by another `requester`, transitively) would answer
  a different, harder question this task did not motivate — "was every
  intermediate inducer itself properly authorized to induce the next" —
  and would require a genuinely new graph entity (a request/invocation
  edge with its own validity semantics) that nothing in §7's invariant
  needs. If a later phase motivates that question specifically, it is a
  new relationship layered on top of this one, not a redesign of it — the
  same posture Phase 1 §21 already took toward confused-deputy detection
  itself relative to the delegation graph.
- **Wildcard scopes, wildcard targets, hierarchical IAM, target registry**
  — unchanged, still rejected (Phase 1 §4, Phase 2 §5/§9).
- **A `Service`/`Resource`/registry entity, or any new node/edge kind** —
  evaluated (§3) and rejected as unnecessary for this invariant, same
  posture as Phase 2 §23 toward a target registry.
- **Sub-classifying the confused-deputy finding into
  amplification-flavored vs. context-binding-flavored variants** —
  evaluated (§12) and rejected: a single `confused_deputy` literal, with
  the finer distinction carried in `reason` text and
  `requester_bound_targets` instead.
- A web UI, automatic policy generation, CI-vendor integration.
- Phase 4 implementation.
- **Approval preservation, delegation-depth policy** — both remain
  exactly as scoped by `docs/phase-1-plan.md` §21, untouched by this
  phase.

---

## 26. Acceptance criteria

- `go build ./...` succeeds; `go.mod` remains stdlib-only.
- `go vet ./...` is clean; `gofmt -l .` reports nothing.
- `go test ./... -race -count=1` passes, including every category in
  §22, with the one documented, sanctioned test-string change (§23) and
  zero other modification to any pre-existing test file.
- Every existing `testdata/golden/` file is unchanged, byte-identical.
- A version-1 or version-2 document produces byte-identical
  `validate`/`verify` output, on both `text` and `json` formats, to the
  current `main` branch today.
- A version-3 document with no violations → `ALLOW`, exit 0.
- `examples/billing-confused-deputy.json` → exactly one `confused_deputy`
  finding, matching §20's worked example.
- A version-3 document containing `authority_amplification`,
  `context_binding_violation`, and `confused_deputy` findings
  simultaneously reports all three, correctly classified, correctly
  ordered, with no duplicate finding for any single operation (§8, §12).
- `unknown_requester` has at least one dedicated malformed fixture and
  table-driven test case, mirroring the existing convention CLAUDE.md
  documents for Phase 1/2.
- No panic is reachable from `main` for any version-1, version-2, or
  version-3 input within the (unchanged) resource bounds.

---

## 27. Definition of DONE

Phase 3 is done when:

1. All items in §26 are met.
2. The file/package layout matches §21, or a documented deviation is
   noted in this document, keeping it authoritative per Phase 1 §20 /
   Phase 2 §25's own convention.
3. The new error kind (§15) and every new finding
   `violation`/`point` combination (§13) has at least one dedicated test.
4. The worked example (§20) is reproducible verbatim via
   `delegationproof verify examples/billing-confused-deputy.json`.
5. `schemas/model.md` has been updated (or a sibling v3 document added)
   by the implementation session to describe the version-3 shape — noted
   as deferred in §21, not done in this planning session, per explicit
   instruction not to modify it now.
6. No open TODOs remain in code for functionality this document describes
   as in-scope; TODOs for §28's deferred items are fine and expected,
   linking back to §28.
7. `docs/phase-1-plan.md` and `docs/phase-2-plan.md` are unmodified —
   Phase 3 attaches to both, per their own §21/§26, without rewriting
   either.

---

## 28. Future-phase boundary

Carried forward from `docs/phase-1-plan.md` §21 and `docs/phase-2-plan.md`
§26, still deferred, now with Phase 3's addition noted where it changes
the shape of what attaches:

- **Multi-hop request/induced-by chains** (newly identified as a
  deliberate non-goal in this phase, §25): if a later product need
  genuinely requires verifying that every intermediate inducer in a
  request chain was itself properly authorized to induce the next (not
  just that the ultimate named requester has standing), that is a new
  relationship — a request/invocation edge with its own validity rule,
  analogous to how a delegation edge has `A ⊆ DA(delegator)` — layered on
  top of, not replacing, the single-`requester` model this phase defines.
  Evaluated on its own merits then, not foreclosed by this phase's
  rejection (§25).
- **Approval preservation, delegation-depth policy, MCP/A2A ingestion,
  JSON Schema enforcement, SARIF, Z3/SMT**: unchanged from Phase 1 §21 /
  Phase 2 §26; nothing in Phase 3 accelerates or blocks any of them.
- **Scope/target wildcard or hierarchy semantics**: still deferred, still
  requires its own containment grammar (Phase 1 §21, Phase 2 §26). Note
  for whoever eventually designs it: `requester` resolution (§15) is
  currently exact-id-match only, exactly like `actor`; if node-id
  aliasing or hierarchy is ever introduced, both reference points would
  need the same treatment simultaneously, not just one.
- **Real request-provenance correspondence** (identified in this phase,
  §24): verifying that a document's declared `requester` values actually
  match a real system's real caller identity (e.g. from an authenticated
  MCP/A2A call frame or session) is a topology-ingestion concern, not a
  verification-core concern — the same posture Phase 1 §21 already takes
  toward real MCP/A2A topology ingestion generally. This phase defines
  what to check once `requester` is declared; it does not address how a
  real system's declarations get produced truthfully.
- **Temporal/session-scoped requester validity** (e.g. "this requester's
  standing was valid at request time but has since been revoked"):
  requires the same temporal/state dimension Phase 1 §21 and Phase 2 §26
  already identified as the trigger for eventually needing bounded
  state-space exploration — Phase 3's static, declaration-time check is a
  strict, deliberate subset of that harder future problem, not an
  attempt to solve it.
