# DelegationProof — Phase 6 Plan

Status: PLANNING ONLY. Phase 1, Phase 2, Phase 3, Phase 4, and Phase 5 are
implemented, merged, and untouched by this document. This is the
authoritative design contract for the Phase 6 implementation session, in
the same spirit as `docs/phase-1-plan.md` through `docs/phase-5-plan.md`.

Module: `github.com/SamudralaAjaykumarrr/delegationproof`

Phase 6 capability: **Temporal Approval Preservation**, proved by a new,
narrowly-scoped, bounded state-space exploration engine
(`internal/explore`) attached to Phase 5's existing static approval
model. This is the smallest coherent capability that both (a) is the
literal, explicitly-named "next" item across five prior planning
documents, and (b) is the *first* capability in the project's history
that actually satisfies the trigger condition `docs/phase-1-plan.md`
§8.2/§21 sets for needing state-space exploration at all. §3 proves this
selection from repository evidence rather than assuming it.

---

## 0. Phase 6 rationale

Phase 1 proved Authority Non-Amplification. Phase 2 proved Context-Binding
Preservation. Phase 3 proved Requester Authorization Preservation. Phase 4
proved Delegation Depth Preservation. Phase 5 proved Approval
Preservation — but **deliberately** as a static, declared-fact check: an
approval record either exists, is standing-backed, or it is not, with "no
pending/approved state machine, no timestamps, no revocation"
(`docs/phase-5-plan.md` §0). This was not laziness; it was an explicit,
justified decision (`docs/phase-5-plan.md` §3 Option C, §28) to keep Phase
5 inside the same single deterministic topological pass every prior phase
already fits within, and to **avoid** triggering the one remaining
structural prerequisite the project has named since its very first
planning document: `docs/phase-1-plan.md` §8.2 states, verbatim, "State-
space/bounded-model-checking framing only becomes necessary when a later
phase introduces genuine nondeterminism or temporal/conditional structure
(e.g., 'approval pending' vs 'approved' states, session validity windows,
revocation races)." Phase 5 §31, having just finished designing the
*static* half of approvals, names the *temporal* half explicitly as "the
first genuinely motivated candidate for that framing across all five
phases so far" — and specifies exactly how it must attach: "It would
layer onto, not replace, this phase's static 'approval record + standing
check' foundation: a temporal layer would determine *whether* a given
static approval record is currently active, while the standing check
defined here... would remain the mechanism for verifying *who* may give
it."

**The security problem.** A capability's origin can legitimately declare
that exercising it requires a second party's sign-off (Phase 5). In any
real system, that sign-off is not an eternal fact — it can be revoked,
can still be pending, can expire, can be reinstated. An agentic system
that treats "an approval record exists and the approver has standing" as
permanently sufficient is exposed to exactly the classical
time-of-check/time-of-use (TOCTOU) class of failure named in Phase 1
§8.2's own example: a "revocation race." Phase 5's model cannot even
*express* this risk, let alone check it — every approval it can represent
is, by construction, eternally active once declared. Phase 6's job is to
let a document express an approval's own lifecycle, and to prove — not
assume — that relying on it statically is actually safe across every
state that lifecycle can legally reach.

**Why this is a *failure class* that matters in delegated/agentic
systems**, not an academic nicety: agent frameworks that gate
high-privilege tool calls behind a human-in-the-loop approval are exactly
the deployment shape Phase 5 was designed to model in the first place
(`docs/phase-5-plan.md` §2's `billing:refund`/`billing:void` example).
Every one of those real approval mechanisms has a revoke/expire path —
compliance officers change their minds, sessions expire, employees leave.
A verification tool that structurally cannot represent "this approval
might not still be active" gives a document author a false sense of
completeness precisely where the stakes (irreversible financial/
administrative actions gated by human sign-off) are highest.

**Why it belongs specifically in Phase 6, not earlier and not later.**
§3 works this out from repository evidence in detail; the short version:
it could not have been Phase 1-4 (no approval concept existed yet to have
a lifecycle), it was explicitly and correctly deferred out of Phase 5 on
its own terms (§28 of that plan), and nothing else remains in the
project's own named backlog (quorum, approval-gated delegation,
self-approval prohibition — `docs/phase-5-plan.md` §31) that requires
anything beyond the existing static single-pass engine — they are all
explicitly documented as "additive, not a redesign" extensions of Phase
5's existing algorithm. Temporal approval lifecycle is the *only* named,
deferred item that structurally cannot be built without genuine
reachability search. It is therefore both the correct next roadmap item
by process of elimination and the first one that actually needs what this
phase adds.

Everything else — MCP/A2A ingestion, quorum, approval-gated delegation,
self-approval prohibition, per-edge depth attenuation, multi-hop
request/induced-by chains, SAT/SMT, general model checking — remains
future work (§38).

---

## 1. Phase 1-5 baseline

Verified against the actual merged implementation on `main` (commit
`0936e54`), not just the plan documents:

- **Model types**: `internal/model/types.go` (v1) through `types_v5.go`
  (v5). Five structurally disjoint schema families sharing no envelope
  struct, per every prior phase's "no shared internal model type"
  discipline (`docs/phase-2-plan.md` §9 onward). `Capability{Scope,
  Target}` (from `types_v2.go`) is the one genuinely shared, reused type,
  used verbatim by delegation-edge authority arrays in every version from
  v2 onward. `RootCapabilityV5{Scope, Target, MaxDelegationDepth *int,
  RequiresApproval *bool}` and `ApprovalV5{Approver, Scope, Target}` are
  the two Phase 5 additions; `PrincipalV5`, `AgentV5`, `DelegationV5`,
  `OperationV5` are otherwise byte-for-byte identical in shape to their
  v4 counterparts.
- **Loader dispatch**: `internal/loader/loader_v2.go`'s `LoadDocument`
  peeks `{"version": string}`, dispatches `"1"`-`"5"` to
  `decodeAndValidateV{1..5}`, anything else → one `KindInvalidVersion`
  error, message `` `version must be "1", "2", "3", "4", or "5", got %q` ``.
  `Document{V1, V2, V3, V4, V5 *model.ModelV5}` union, exactly one field
  set.
- **Graph**: `internal/graph/graph.go` — `TopoSort` (Kahn's algorithm,
  min-heap ascending-lexicographic tie-break, returns `ok=false` plus a
  canonical cycle path if the input is cyclic — **DAG-only by design**),
  `LongestPath` (DAG DP over topological order), `CanonicalTrace` (BFS
  from all principals simultaneously, ascending-id expansion order,
  first-path-wins, `[]string{actor}` if unreachable). All three operate
  purely on node ids and `[]graph.Edge{From, To}`. Untouched by any phase
  so far. **Critically for Phase 6**: `TopoSort` assumes and requires
  acyclicity — it is not a general reachability primitive, and reusing it
  for a domain that legitimately contains cycles (an approval lifecycle,
  §8) would be a category error, not a convenience (§20 justifies why
  Phase 6 introduces its own package instead).
- **Verify**: `internal/verify/verify_v5.go`'s `RunV5(*model.ModelV5)` —
  one topological pass builds `da map[string]map[model.Capability]authState`
  (`authState{remaining, configuredMax, requiresApproval}`) for every
  node, reusing `verify_v2.go`/`verify_v3.go`'s unexported
  `isSubsetCap`/`classifyEdge`/`classifyOne`/`heldTargetsForScope`/
  `containsCap` against `flattenApproval`'s presence-only view, unmodified.
  Approval indexing (`declaredApprovers`, `standingApprovers`) is a
  bounded pre-pass over `m.Approvals`, computed once, before operation
  evaluation, keyed by `model.Capability`. Operation evaluation is a
  four-step precedence: presence/binding (Phase 1/2) → requester standing
  (Phase 3) → approval-required check (Phase 5), depth never participating
  at the operation level (`docs/phase-4-plan.md` §12/§13, reaffirmed).
- **Report**: `internal/report/finding.go`'s `sortKey{point, subject,
  secondary, scope, target, requester}` — a 6-tuple, unchanged in shape
  since Phase 3, `keyOf`'s type switch with one case per finding struct
  (`EdgeFinding`, `OperationFinding`, `CapabilityEdgeFinding`,
  `CapabilityOperationFinding`, `ConfusedDeputyFinding`,
  `DelegationDepthFinding`, `ApprovalFinding`). `RenderText`/`RenderJSON`
  both switch on finding concrete type; `RenderJSON`'s envelope
  (`{"result", "findings"}`) is generic over `[]interface{}`.
- **CLI**: `cmd/delegationproof/main.go`'s `runVerify` dispatch:
  `switch { case doc.V1 != nil: ...; ...; case doc.V5 != nil: ... }`.
- **Limits**: `internal/limits/limits.go` — every bound is an exported
  `var`. `MaxChainDepth` (64, resource-safety valve on actual graph
  shape) and `MaxDelegationDepth` (64, resource-safety valve on a
  *declared policy value*) are deliberately independent vars sharing a
  default value, never conflated (CLAUDE.md's explicit standing
  invariant) — the direct precedent Phase 6 follows for its own two new,
  independent bound pairs (§23).
- **Tests**: `internal/loader/loader_v2_test.go`'s asserted
  `invalid_version` literal is the one sanctioned pre-existing test-string
  edit every phase since Phase 3 has made, growing the version list by
  one each time.

Phase 6 must not modify any Phase 1-5 production code path, and must
touch only the one sanctioned message-text line identified in §6.

---

## 2. The temporal approval gap

`admin` legitimately owns `billing:refund@billing-service`, requires
approval on it, delegates it validly to `billing-agent` within budget.
`compliance-officer` independently holds standing over the same capability
and is named as its approver. Every Phase 1-5 invariant is checked and
passes: presence, binding, requester standing, depth, and — as of Phase
5 — a standing-backed approval record exists. `verify` says `ALLOW`.

But suppose the document is more honest about what actually happened:
`compliance-officer` approved the refund capability, and *later*, in the
same declared record, revoked it — a compliance mistake was caught, an
employee left, a policy changed. Phase 5 has no vocabulary to represent
this at all: an `ApprovalV5` record is a single, timeless fact ("this
approver, this capability"), so a document author who wants to be honest
about a revocation has exactly two bad options — omit the approval record
entirely (which produces a *false* `approval_missing` finding for an
operation that, at least at some point, genuinely was validly approved),
or leave the stale record in place (which produces a *false* `ALLOW` for
an operation that may now be exercised against a capability whose
approval was actually withdrawn). Neither option is correct, because
Phase 5's data model cannot express "this approval's status has changed
over its own history" at all — it can only express "approved, forever, or
not at all."

This is the concrete instance of `docs/phase-1-plan.md` §8.2's own named
example ("revocation races") and is precisely the gap Phase 5 §31
predicted and scoped in advance: a temporal layer that determines
*whether* a declared, standing-backed approval record is *currently*
active, composing with — never replacing — Phase 5's existing standing
check (*who* may give it).

---

## 3. Why bounded state-space exploration is now necessary

This section directly answers the task's central question: is bounded
state-space exploration the correct Phase 6 capability, and does its
prerequisite semantic structure actually exist?

**The trigger condition, stated three times in the project's own design
history, has never changed:** state-space exploration becomes necessary
only once a phase introduces genuine nondeterminism or temporal/
conditional structure (`docs/phase-1-plan.md` §8.2, restated at §21, and
reaffirmed unmodified through every subsequent phase's own non-goals
section, most recently `docs/phase-5-plan.md` §28: "State-space
exploration / general search — confirmed unnecessary… a single
deterministic topological pass plus one bounded pre-indexing pass over
`approvals[]` suffices"). Phases 1 through 5 all satisfy this without
exception: `DA(n)` is computed by one Kahn's-algorithm topological pass
over a graph the loader has already proven acyclic (`internal/loader`'s
`graph.TopoSort` call, unconditionally rejecting cycles as a structural
error before `verify` ever runs), and every downstream quantity — presence,
binding, remaining depth, requester standing, approval standing — has
*exactly one* correct, DP-computed value given the input, with **no
branching over alternative interpretations and no time dimension** (this
exact sentence, or a close paraphrase of it, appears in the complexity
section of every one of the five prior phase plans). A single-pass DAG
traversal is definitionally incapable of expressing — let alone
searching — "does there exist a legal state this system could reach in
which an unsafe combination holds," because a DAG traversal has no notion
of "legal alternative future" at all: every node's value is fixed the
moment its predecessors are known.

**What changes with a declared approval lifecycle, and why a DAG pass
cannot answer the resulting question.** An approval lifecycle (§8) is a
small automaton: a set of author-named states, a designated initial
state, and a set of declared transitions between them. Unlike the
delegation graph, this automaton is **explicitly not required to be
acyclic** — a real approval can legitimately be re-submitted after
revocation (`revoked → pending → approved` is a completely ordinary real
workflow), and **explicitly permits branching**: an `"approved"` state may
legally transition to *either* `"revoked"` *or* `"expired"`, and the
document does not, and structurally cannot, commit to which one actually
happens or when. This is precisely "genuine nondeterminism… temporal/
conditional structure" as named at project inception. The question Phase
6 must answer — "is there any state reachable from this approval's
declared initial state, via its own declared transitions, in which the
approval is not actually active?" — has no single fixed value the way
`DA(n)`'s quantities do; it is a reachability question over a graph that
may contain cycles and branches, which is by definition not solvable by
a topological pass (a topological order does not even exist for a cyclic
graph) and requires an explicit visited-state search: bounded, explicit-
state model checking's canonical formulation, and the literal meaning of
"state-space exploration."

**What state dimensions must be represented**, and no more (deliberately
narrow, per §37's non-goals): exactly one — the approval record's own
declared lifecycle state, `q ∈ Q`. No global/composed system state is
introduced (§14 justifies this explicitly): operations remain wholly
stateless and independent of each other and of any lifecycle, exactly as
in Phase 1-5 (§17 confirms operations still have no ordering semantics
and remain byte-identical to v5 in shape); delegation edges remain wholly
unaffected by lifecycle (§16); the only new "state" in the entire system
is the position within one approval record's own small automaton.

**What transitions exist**: exactly the ones a document declares inside
one approval record's `lifecycle.transitions` array (§8, §11) — nothing
is inferred, defaulted, or synthesized.

**What security property exploration proves**: reachability of an unsafe
lifecycle state (§13 defines "reachable" precisely; §9 states the formal
invariant). Concretely: **∀-reachable-states safety** — every state
reachable from the initial state must be the single designated safe
state, `"approved"`, or the approval record cannot be relied upon to
statically satisfy Approval Preservation. This is the standard "no
reachable state violates the invariant" formulation of safety-property
model checking, chosen deliberately over its dual ("∃ an unsafe state is
reachable," the literal negation, used only as the internal
implementation of the same check — §13).

**Why naive enumeration would be unsafe/unbounded, and why Phase 6
avoids it on purpose.** The dangerous version of this feature would
compose *every* lifecycle-bearing approval's state into one global system
state (the cross-product of every approval's own automaton) so that
cross-approval interactions could be reasoned about — that state space is
the *product*, not the sum, of each automaton's size, and is genuinely
exponential in the number of lifecycle-bearing approvals declared in a
document (`|Q₁| × |Q₂| × … × |Qₙ|`). Nothing in the motivating threat (§2)
requires this: two different approvals gating two different capabilities
have no declared interaction with each other at all, exactly as Phase 5
§13 already establishes that an approver's own standing is checked wholly
independently, with no cross-approval coupling. **Decision: each
approval record's lifecycle is explored completely independently of every
other approval record's lifecycle** — the total exploration cost is the
*sum* of each record's own bounded automaton size, not the product,
which is what keeps Phase 6 additive-linear rather than combinatorially
explosive (§23, §26 complexity analysis) and is the direct reason bounded
exploration — a small, complete, per-record search — is *sufficient* for
the declared static model, without needing anything resembling general
model checking, SAT/SMT, or symbolic execution (§37).

**If no such structure existed, this document would not recommend state-
space exploration.** The task explicitly warns against fabricating
temporal/conditional structure merely to justify search machinery. Phase
6 does not do this: the lifecycle automaton is a genuinely new, minimal,
additive entity (§4, §8) that a document author must explicitly opt into
per approval record (absence of `lifecycle` is fully Phase-5-compatible,
§8) — Phase 6 does not retroactively impose temporal structure on
anything that does not declare it, and the exploration engine has
literally nothing to run when no `lifecycle` field is present anywhere in
a document (§14 states this precisely: `lifecycleSafe(⊥) = true`
trivially, no BFS invoked at all).

---

## 4. Minimal new abstraction

Evaluated, in the same spirit as every prior phase's own candidate table:

| Candidate | Verdict | Why |
|---|---|---|
| **A. An optional `lifecycle` object attached to each `approvals[]` record: a small named-state automaton (initial state, declared states, declared transitions)** | **Chosen** | Mirrors exactly how Phase 5 attached `requires_approval` to the *origin declaration* and a new, separate `approvals[]` entity to represent the sign-off itself: the lifecycle is a property of *this specific approval record*, additive and optional, so every existing v5-shaped approval record remains fully expressible (§8). No new top-level array, no new graph node, no new edge kind. |
| **B. A global document-level "current time" or "current state" declaration, with every approval consulting it** | Rejected | Reintroduces exactly the "session/event-log concept" `docs/phase-5-plan.md` §3 Option C explicitly rejected — a single global clock is a much bigger, much less minimal addition than a per-record automaton, and answers a different, harder question ("what literally happens at time T") that nothing in the threat model (§2) requires. Phase 6 asks a *safety* question (can an unsafe state ever be reached), not a *simulation* question (what state is it in right now) — the former needs no global clock at all. |
| **C. Coupling operations to lifecycle transitions directly (an operation declares "this executes after event E")** | Rejected | Would require inventing operation ordering/identity concepts the schema has never needed (`docs/phase-3-plan.md` §15 already establishes operations have no persistent identity across a document) and would reintroduce the exact combinatorial cross-entity state composition §3 rejects. It also does not match how the threat actually manifests: DelegationProof cannot observe real execution timing (`docs/phase-1-plan.md`'s own "static, offline analyzer" security-assumptions posture, reaffirmed unmodified through every phase) — the fail-closed, provable question is not "did this operation run before or after revocation" (unknowable, offline) but "can this approval ever *not* be active" (provable, from the declared automaton alone). |
| **D. A global cross-product state machine over every lifecycle-bearing approval simultaneously** | Rejected | Exactly the exponential-blowup design §3 rejects; unmotivated by the threat model, since approvals for unrelated capabilities have no declared relationship to reason about jointly. |
| **E. A quorum/vote-counting temporal model (N-of-M approvers, each independently temporal)** | Rejected for Phase 6 | Composes two independently-deferred features (quorum, `docs/phase-5-plan.md` §31, and temporal lifecycle) that neither prior document motivates jointly. Phase 6 keeps the existing existential ("one standing-backed approval suffices") quantifier from Phase 5 unchanged (§14) and adds only the lifecycle-safety filter on top of it — quorum remains exactly as deferred as `docs/phase-5-plan.md` §31 left it (§38). |

**Decision:** exactly one new, optional, nested field — `lifecycle` — on
`approvals[]` records only. Every other v5 entity (`RootCapabilityV5`,
`DelegationV5`, `OperationV5`) carries forward into v6 with **zero**
structural change (§6).

---

## 5. Lifecycle semantics

### 5.1 Is a lifecycle part of an approval's identity?

**No**, for the identical reason Phase 4 §4.1 and Phase 5 §4.1 give for
depth and approval metadata respectively: `(approver, scope, target)`
remains the sole identity of an approval record (unchanged uniqueness key,
`KindDuplicateApproval`, §20). `lifecycle` is metadata attached to that
identity, checked only once the record has already been determined to be
declared and standing-backed by Phase 5's own unmodified machinery (§16).

### 5.2 What does declaring a `lifecycle` mean, precisely?

- **An approval record with no `lifecycle` field** (the field is entirely
  absent, `nil` in Go) behaves **exactly** as every Phase 5 approval
  record already does — eternally active once declared and standing-
  backed. Phase 6 adds nothing to its evaluation. This is the direct
  backward-compatibility anchor for the whole design (§35).
- **An approval record with a declared `lifecycle`** may be relied upon by
  Phase 6's Temporal Approval Preservation check only if **every state
  reachable from its declared initial state is the single designated safe
  state name, `"approved"`** (§9, §13). If any other state is reachable —
  whether or not `"approved"` is also reachable — the record cannot
  statically prove safety and is excluded from the standing-backed set
  Phase 5's existential check draws from (§14, §16).
- **A lifecycle governs whether an approval record counts, never whether a
  capability is held, bound, or within budget.** It is checked strictly
  after, and only after, presence, binding, requester standing, and Phase
  5's own standing check already pass — the identical "layer onto, never
  replace" posture Phase 5 §31 specifies in advance, and the identical
  "gates exercise, checked last" posture Phase 5 §12 already established
  for approval as a whole relative to Phase 1-4.
- **A lifecycle does not grant, propagate, revoke, or extend authority.**
  Exactly as Phase 5 §4.2 establishes for the base approval fact itself:
  declaring a lifecycle does not add or remove anything from any node's
  `DA(n)`, does not create or remove a delegation edge, and does not
  affect `graph.TopoSort`/`graph.CanonicalTrace`'s DAG at all. Lifecycle
  exploration is a wholly separate, additive, bounded sub-analysis (§21).
- **The reserved safe-state name is exactly `"approved"`, a fixed string,
  not an author-declared role.** Considered and rejected: letting a
  document declare *which* of its own state names counts as "safe" (e.g.
  an `active_states: [...]` array). Rejected because it would let an
  adversarial or careless author simply declare every state "safe,"
  defeating the entire point of the check the same way Phase 5 §3 Option
  B rejects a bare `approved: bool` on an operation as unfalsifiable — a
  single, fixed, reserved keyword keeps the safety condition an objective
  property of the automaton's shape, not a second self-reported
  assertion layered on top of the first.
- **A simpler `revocable: bool` field is not a smaller, competing
  design — it is already the trivial two-state instance of this exact
  design**, and is not a "smaller invariant" this document skipped past.
  A document that wants exactly that binary semantics declares
  `{"initial": "approved", "states": ["approved", "revoked"],
  "transitions": [{"from": "approved", "to": "revoked"}]}` — two states,
  one transition — and gets precisely the outcome a bare boolean would
  have given, with no additional machinery cost (this is §31.2's own
  worked example). The general automaton is strictly necessary, not
  gold-plating, because the task's own review criteria (§4 of the audit
  this document responds to) explicitly expect reapproval/resubmission
  and multi-step workflows to be representable and diagnosable (*which*
  state is unsafe, *via which path* — §14.3, §19) — a bare boolean can
  express neither, and would need to grow into exactly this automaton the
  moment either requirement surfaced. There is therefore no smaller
  invariant sitting between Phase 5 and this design; the smallest useful
  instance of this design *is* the smaller invariant.
- **Declaring a lifecycle with any state reachable from a revoking
  transition will, honestly and correctly, almost always DENY once that
  transition is reachable — this is the intended behavior, not a defect
  that makes the feature self-defeating.** A document author has exactly
  two honest choices: (a) do not declare a lifecycle at all, and accept
  Phase 5's simpler "eternally active once declared" model (unchanged,
  fully available, §5.2's first bullet), or (b) declare a lifecycle and
  have DelegationProof hold it to the standard the declaration itself
  claims — if the document says "this approval can be revoked," Phase 6
  correctly refuses to treat it as permanently reliable, because that is
  a true statement about the declared automaton, not a false negative.
  The value of the feature is not that a "typical" lifecycle-bearing
  approval passes; it is that a document author who *thinks* an approval
  is safe, but whose own declared automaton (possibly large, possibly
  with a non-obvious cycle back through an unsafe state) actually is not,
  gets caught — exactly the same value proposition Phase 1's Non-
  Amplification check has despite "don't grant excess authority" sounding
  equally obvious in isolation.

---

## 6. Schema v6

**Decision: a new schema version literal, `"6"`, decoded into a new,
structurally disjoint `model.ModelV6`.** Identical reasoning to every
prior phase's own version-bump decision. `ModelV6` is `ModelV5` with
exactly one structural change: **approval records gain an optional
`lifecycle` object.** Principals, agents, delegations, and operations are
byte-for-byte identical in shape to their v5 counterparts.

```json
{
  "version": "6",
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
    {
      "approver": "compliance-officer",
      "scope": "billing:refund",
      "target": "billing-service",
      "lifecycle": {
        "initial": "approved",
        "states": ["approved", "revoked"],
        "transitions": [
          { "from": "approved", "to": "revoked", "event": "revoke" }
        ]
      }
    }
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

**`lifecycle` is optional, with no default other than "absent" — a
deliberate departure from Phase 4/5's own "every new field is required,
never defaulted" discipline, explicitly justified.** `max_delegation_depth`
and `requires_approval` are required precisely because *any* value is a
meaningful, distinct policy declaration and a missing key would be
ambiguous with a real value (`docs/phase-4-plan.md` §5, `docs/phase-5-plan.md`
§5). `lifecycle` is different in kind: its absence is not ambiguous with
any value a lifecycle object could hold — it unambiguously means "this
specific approval record declares no additional temporal structure,"
which is a single, precise, and already-meaningful semantic — Phase 5's
own eternal-fact model — never confusable with "the author forgot to
declare a lifecycle policy." A Go `*Lifecycle` (nil pointer) represents
this cleanly with no sentinel-value ambiguity of any kind, unlike
`*bool`/`*int`'s "zero value is itself meaningful" problem that motivated
Phase 4/5's pointer types in the first place.

**`RootCapabilityV6`, `DelegationV6`, `OperationV6` are byte-for-byte
identical in shape to their v5 counterparts.** No `lifecycle` field, or
any lifecycle-related field, exists anywhere except inside an
`approvals[]` entry. A stray `lifecycle` key on a root capability, a
delegation authority entry, or an operation is rejected at decode time by
`DisallowUnknownFields`, the same "enforced for free by the schema shape"
mechanism every prior phase relies on.

**`ApprovalV6`:**

```go
// internal/model/types_v6.go

type LifecycleTransition struct {
	From  string `json:"from"`
	Event string `json:"event"` // optional; "" if omitted, purely diagnostic
	To    string `json:"to"`
}

type Lifecycle struct {
	Initial     string                 `json:"initial"`
	States      []string               `json:"states"`
	Transitions []LifecycleTransition `json:"transitions"`
}

type ApprovalV6 struct {
	Approver  string     `json:"approver"`
	Scope     string     `json:"scope"`
	Target    string     `json:"target"`
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
}
```

**Dispatch mechanism**, extending `LoadDocument`'s switch:

```
"1"          -> decodeAndValidateV1 (unchanged)
"2"          -> decodeAndValidateV2 (unchanged)
"3"          -> decodeAndValidateV3 (unchanged)
"4"          -> decodeAndValidateV4 (unchanged)
"5"          -> decodeAndValidateV5 (unchanged)
"6"          -> decodeAndValidateV6 (new)
anything else (including "") -> one KindInvalidVersion error
```

`Document` grows a sixth field: `Document{V1, V2, V3, V4, V5, V6
*model.ModelV6}`, exactly one of which is set on success. The
`invalid_version` message updates from
`` `version must be "1", "2", "3", "4", or "5", got %q` `` to
`` `version must be "1", "2", "3", "4", "5", or "6", got %q` ``, in the
six call sites that must stay textually identical (the five existing
`validate*` functions' copies updated; `validateV6` introduces the
sixth). `internal/loader/loader_v2_test.go`'s asserted literal must be
updated to match — the same sanctioned single-line touch every prior
phase has made.

---

## 7. Root capability / delegation / operation semantics (unchanged)

**Nothing in this section is new.** Stated explicitly, per every prior
phase's own discipline of confirming what does *not* change: a
`RootCapabilityV6`'s `max_delegation_depth`/`requires_approval` mean
exactly what they meant in v5 (`docs/phase-4-plan.md` §4, `docs/phase-5-plan.md`
§4). `DelegationV6` edges are evaluated by the byte-identical Phase 1/2/4
edge algorithm. `OperationV6`s are evaluated by the byte-identical Phase
1/2/3 actor/requester precedence, extended only at its now-five-step tail
(§16). A capability's `requires_approval` boolean continues to gate
*whether* an approval is needed at all; lifecycle gates only *whether a
specific declared approval record may be relied upon*, strictly
downstream of that (§5.2).

---

## 8. Approval record + lifecycle semantics

A version-6 approval record is `(approver, scope, target, lifecycle?)`.

- **`approver`/`scope`/`target`** are unchanged from `ApprovalV5` (§7 of
  `docs/phase-5-plan.md`): node-id reference, unchanged Phase 2 capability
  grammar, capability-scoped (not operation/actor-scoped), no requirement
  that the approver be an ancestor of the actor, self-approval not
  structurally prohibited (unchanged — still deliberately not enforced,
  §37).
- **`lifecycle.initial`** — the state the automaton is declared to start
  in. Must reference a name present in `lifecycle.states` (§20); no
  default, no omission.
- **`lifecycle.states`** — a non-empty array of distinct state names, each
  matching the unchanged Phase 2 target grammar
  (`^[A-Za-z0-9_.-]{1,128}$`, reused verbatim, no new regex). Bounded by
  `limits.MaxLifecycleStates` (§23). State names are author-chosen and
  opaque — DelegationProof attaches no meaning to any name except the one
  reserved literal, `"approved"` (§5.2). A state named anything else —
  `"pending"`, `"revoked"`, `"expired"`, `"resubmitted"`, any string at
  all — is simply "not the safe state." **The reserved-literal comparison
  is exact, case-sensitive string equality, with no normalization,
  trimming, or case-folding of any kind** — a declared state named
  `"Approved"` or `"APPROVED"` is a distinct, unsafe state, not a variant
  spelling of the safe one. This is not a special rule invented for
  lifecycle; it is the same posture every other opaque string comparison
  in the project already takes (node ids, scopes, targets — nowhere in
  `internal/loader`/`internal/verify` is any string normalized before
  comparison), applied here without exception.
- **`lifecycle.transitions`** — a possibly-empty array of `{from, to,
  event}` triples. `from`/`to` must each reference a declared state name
  (§20) and are therefore never empty (a declared state name is always
  ≥1 byte, §20). `event` is an optional, purely diagnostic label (used
  only in finding trace text, §19) with no effect on reachability, and is
  **exempt from the state-name grammar when empty**: `event == ""` is
  always valid and means "no label declared" (the common case — most
  transitions need no event name at all); a **non-empty** `event` value
  is checked against the same grammar as `to`/`from`
  (`^[A-Za-z0-9_.-]{1,128}$`). This asymmetry — `from`/`to` always
  non-empty and grammar-checked, `event` optionally empty and
  grammar-checked only when present — is the direct consequence of
  `from`/`to` being *references* (they must resolve to a declared state)
  while `event` is a free-standing, optional annotation with nothing to
  resolve against. Bounded by `limits.MaxLifecycleTransitions` (§23).
  **Self-loops
  (`from == to`) are explicitly legal** — e.g. `{"from": "approved", "to":
  "approved", "event": "reapprove"}`, an idempotent re-approval — and
  **cycles across multiple states are explicitly legal** — e.g.
  `revoked → pending → approved → revoked`, an ordinary real resubmission
  workflow. Unlike the delegation graph (§1), a lifecycle automaton is
  never required to be acyclic, and `internal/loader` never runs
  `graph.TopoSort` (or any cycle check) over it (§20 makes this explicit
  by omission — there is no such check to add).
- **Revocation is not required to be monotonic, and reapproval is
  explicitly supported — but a recovery path never retroactively restores
  safety.** A document may legally declare a cycle that returns to
  `"approved"` after leaving it (`revoked → pending → approved`, an
  ordinary resubmission). This is a deliberate design choice, not an
  oversight: forcing revocation to be one-way (a separate, disjoint
  "monotonic-only" schema constraint) would restrict what documents are
  well-formed without changing any safety *outcome*, because `Safe(a)`
  (§9) depends on **set membership in `Reach(L)`**, never on transition
  order or on whether a later transition happens to lead back to
  `"approved"`. Concretely: once `"revoked"` is reachable at all, `a` is
  unsafe, full stop — the presence of a further `revoked → pending →
  approved` path does not undo that, because Phase 6 proves "can this
  ever be observed non-approved," not "does it eventually settle on
  approved." §33 test 62 exercises this directly.
- **A declared state that is never mentioned in any transition, or that
  is unreachable from `initial`, is not a structural error** — it is
  simply inert, identical in spirit to Phase 2 §5's "unrecognized target
  is not a structural error, just never matched" and Phase 5 §7's
  identical treatment of an approval naming an undeclared capability.
- **A lifecycle whose reachable set never includes `"approved"` at all —
  including one that never even declares a state named `"approved"` — is
  not a structural error either.** It is a completely well-formed
  document that will simply always fail Phase 6's semantic safety check
  at `verify` time (exit 1, a finding), never at `validate` time (exit 2,
  a structural error) — the exact "structurally well-formed but
  semantically insufficient is a verify-time finding" precedent every
  prior phase's own plan establishes (`docs/phase-1-plan.md` §7.4,
  `docs/phase-2-plan.md` §10, `docs/phase-3-plan.md` §15,
  `docs/phase-4-plan.md` §17, `docs/phase-5-plan.md` §16).

---

## 9. Formal invariant: Temporal Approval Preservation

**Entities.** An approval record is `a = (approver, s, t, L)` where `L`
is either the distinguished symbol `⊥` (no lifecycle declared) or a
finite automaton `L = (Q, q₀, δ)`: `Q` a finite, non-empty set of state
names; `q₀ ∈ Q` the initial state; `δ ⊆ Q × Event × Q` the declared
transition relation (event labels are cosmetic, carried only for
diagnostic trace text — §19 — and never participate in reachability).

**States.** Every `q ∈ Q` is either the single reserved *safe* state,
`"approved"`, or an *unsafe* state (every other name, entirely
author-chosen).

**Reachability.** `Reach(L) = { q ∈ Q : q = q₀ ∨ ∃ a path from q₀ to q
using zero or more edges in δ }`, computed by bounded breadth-first
search (§21).

**Safety predicate.**

```
Safe(a) :=  L = ⊥
         ∨  Reach(L) = { "approved" }
```

i.e. an approval with no declared lifecycle is vacuously safe (§5.2); one
with a declared lifecycle is safe **iff the only state its own declared
transitions can ever reach, from its own declared initial state, is
`"approved"` itself** — no other reachable state, whether or not
`"approved"` is also reachable, whether the unsafe state is a dead end or
has further outgoing transitions of its own.

> **Temporal Approval Preservation:** for every capability `c = (s, t)`
> declared by a root principal with `requires_approval = true`, and for
> every operation `op = (actor, requester, action, s, t)` for which
> Phases 1 through 5's own invariants already hold — presence, binding,
> requester standing, and Phase 5's own approval-standing existential
> (`∃ a ∈ Approvals : a.scope = s ∧ a.target = t ∧ c ∈ DA(a.approver)`,
> `docs/phase-5-plan.md` §8) — `op` additionally satisfies Temporal
> Approval Preservation only if there exists at least one such
> standing-backed approval record `a` with `Safe(a) = true`. If the
> standing-backed set is non-empty, every member's `Safe` computation
> completes within the bounded exploration budget (§22), and none of
> them is `Safe`, `op` is an `approval_lifecycle_unsafe` violation. If at
> least one member's `Safe` computation cannot complete within budget,
> and no *completed* member is `Safe`, `op` is an
> `approval_lifecycle_unproven` violation — an incomplete proof is never
> treated as an implicit pass (§22).

**Terminal/invalid states.** Every name in `Q` is well-formed by
construction — `internal/loader` rejects any `from`/`to`/`initial`
reference to an undeclared name at validate time (§20), so there is no
"invalid state" concept distinct from "structurally rejected input."
"Terminal" states (no outgoing transitions) receive no special treatment:
an unreachable-from-approved sink is exactly as unsafe as a non-sink
unsafe state under the ∀-reachable-safety rule — reachability, not
sink-ness, is what matters.

**Propagation semantics.** `Safe(a)` is a property of one approval
record's own automaton alone; it never propagates to, or is influenced
by, any other approval record, any node's `DA(n)`, or any delegation
edge (§5.2, §16).

**Failure precedence.** §16 gives the complete table; the short version:
`approval_lifecycle_unsafe`/`approval_lifecycle_unproven` are reached
only after every earlier-tier Phase 1-5 check already passes — they are
strictly the final, sixth tier of the operation-level precedence chain.

**Interaction with existing invariants.** Zero change to `DA(n)`,
`graph.TopoSort`, `graph.CanonicalTrace`, or Phase 1-4's edge/operation
algorithms (§16). Phase 5's own `declaredApprovers`/`standingApprovers`
precomputation is reused verbatim, unmodified; Phase 6 adds one further,
independent, precomputed filter on top of `standingApprovers` (§26).

---

## 10. State model

The complete state tuple Phase 6 introduces is deliberately the smallest
possible:

```
LifecycleState = q ∈ Q        // one approval record's own current automaton position
```

That is the entire state Phase 6 adds to the system. Explicitly **not**
included, and why each was considered and rejected:

- **Global/system state (combination across approvals)** — rejected, §3:
  unmotivated by the threat model, and the direct cause of the
  exponential blowup this design avoids.
- **Operation execution state ("has this operation fired yet")** —
  rejected, §4 Candidate C: operations remain wholly stateless, exactly
  as in every prior phase.
- **Delegation-edge state** — unaffected; depth/presence/binding state
  (`authState`, unchanged from Phase 5) is not touched by lifecycle at
  all.
- **Requester/approver identity as state** — these remain ordinary node
  ids, checked via `DA(n)` exactly as before; a node's own identity is
  not part of the automaton.
- **Time itself (a clock, a session, a timestamp)** — rejected, §4
  Candidate B: nothing is provable about *when* an operation executes
  relative to a lifecycle transition (`internal/verify`'s security
  posture, §35), so no clock is modeled; the safety question is phrased
  entirely in terms of what states are *reachable*, never *when* they
  occur.

**Mutability.** `LifecycleState` is not mutated by `verify` at all —
there is no notion of "the" current state at verify time. `verify`
computes the **full reachable set** `Reach(L)` once, per distinct
lifecycle-bearing approval record, and that set (not any single state
within it) is what the safety predicate consults. Nothing about a
lifecycle's `Q`, `q₀`, or `δ` changes during verification; they are pure,
static, declared input, read once, exactly like every other field in the
document.

---

## 11. Transition system

Every declared transition `(from, event, to) ∈ δ` is unconditionally
legal to traverse during exploration — there are no additional
preconditions, no guards, no side effects, and no mutation of anything
outside the automaton itself (§10). This is deliberately the simplest
possible transition system: a plain labeled directed graph.

| Property | Value |
|---|---|
| Precondition to traverse `(from, event, to)` | `from` is currently a member of the reached set under construction (i.e. reachable from `q₀`) |
| State mutation | none — traversal only adds `to` to the reached set; it does not "consume" or remove `from` |
| Resulting authority/security state | none — traversal has zero effect on `DA(n)`, graph edges, or any other node's state (§9, §10) |
| Failure condition | none — every declared transition is unconditionally traversable; the only "failure" in this system is the *safety predicate* finding a reachable unsafe state, not a transition itself failing |
| Deterministic ordering | outgoing transitions from a given state are visited in ascending lexicographic order of `(to, event)` during BFS expansion (§21) — the direct analogue of `graph.CanonicalTrace`'s "ascending destination-id order" rule |
| Observable as a finding? | not individually — only the *aggregate result* of exploring an entire automaton (safe / unsafe / unproven) is observable, surfaced via `approval_lifecycle_unsafe`/`approval_lifecycle_unproven` findings (§18), with the specific canonical path to the first unsafe state carried as diagnostic payload (§19) |

Because every transition is unconditionally legal and has no guard, this
is **not** a general Kripke-structure/model-checking framework with
guarded actions and shared variables — it is the simplest thing that can
still exhibit cycles and branching, which is exactly and only what §3
requires.

---

## 12. Initial states

An approval record's exploration always starts from exactly one state:
`lifecycle.initial`, as declared. There is no derivation, inference, or
computation of an initial state from anything else in the document — it
is read directly off the field, validated to reference a declared state
name (§20), and used as `q₀` unmodified. If `lifecycle` is entirely
absent, there is no automaton and no initial state — `Safe(a) = true`
vacuously, with zero exploration invoked (§9, §14).

---

## 13. Reachability

> **Explicit, quotable statement of what is universally vs. existentially
> quantified, closing any possible ambiguity:** DelegationProof does
> **not** `ALLOW` an operation merely because *some* reachable lifecycle
> history is safe. Safety is quantified **universally over every state
> reachable** from an approval record's declared initial state — `∀q ∈
> Reach(L): q = "approved"` — never **existentially** over *some* path or
> *some* history. The only place an existential quantifier governs
> anywhere in Phase 6 is one layer up: *which approval record* (among
> several independently declared, standing-backed ones) may be relied
> upon (§14.1) — never *which state, within one record's own reachable
> set*, is convenient to believe. If a second, independently reachable
> history from the same declared automaton would produce an unauthorized
> operation (i.e. any reachable state other than `"approved"`), that
> automaton is unsafe, full stop, regardless of how many *other* reachable
> states are `"approved"`.

**"Reachable"** means precisely: a state `q` is reachable from `q₀` if
`q = q₀`, or there exists a finite sequence of declared transitions
`q₀ → q₁ → q₂ → … → q` each drawn from `δ`. This is standard directed-
graph reachability, computed by breadth-first search from a single source
(§21).

**What property Phase 6 proves**, directly answering the task's explicit
question: **operation safety over all reachable states** — specifically,
the ∀-reachable-states-safe formulation (§9's `Safe(a)` predicate: *every*
reachable state must be `"approved"`), not its dual. The two
formulations are logically equivalent (`∀q ∈ Reach(L): q = "approved"`
is the exact negation of `∃q ∈ Reach(L): q ≠ "approved"`), and the
*implementation* computes the existential form (find the first
non-`"approved"` reachable state, if any — §19's canonical trace is
exactly this witness), but the *invariant being proved* is the universal
one: the guarantee a document author and a downstream reader receive from
a `Safe(a) = true` result is "this approval can never be observed in any
state other than approved," not merely "this approval happens to be
approved in at least one reachable state." This distinction matters
concretely: an automaton where `"approved"` is reachable *and* `"revoked"`
is also reachable is **not** safe under Phase 6's invariant, precisely
because DelegationProof has no way to prove an operation only ever
executes during the `"approved"` window (§4 Candidate C's rejection).

---

## 14. Multi-path / multi-history semantics

Phase 6 has **two** independent multi-path questions, mirroring Phase
5's own two-question structure (`docs/phase-5-plan.md` §10) but at a
different layer.

### 14.1 Multiple approval records for the same capability

**Unchanged from Phase 5, extended by one filter.** The existential
quantifier from `docs/phase-5-plan.md` §10.2 ("one standing-backed
approval record is sufficient; no canonical approver to select") still
governs: Phase 6 narrows the set Phase 5's existential draws from — from
"standing-backed" to "standing-backed **and** lifecycle-safe" — but does
not change the *quantifier* itself. If two records both back the same
`(scope, target)` and one is lifecycle-safe while the other is not, the
capability is still validly approved: the safe one alone suffices, the
identical "adopting the more restrictive of two facts is not required
when at least one independently-sufficient fact exists" reasoning already
underlying every existential check in this project.

### 14.2 Multiple states reachable within one lifecycle (why the safety rule is universal, not existential, at the per-record level)

This is the one place Phase 6 deliberately does **not** mirror Phase
5's existential quantifier. §13 explains why: at the level of "which
states can this one approval record reach," the rule must be universal
(all reachable states are `"approved"`), not existential (at least one
reachable state is `"approved"`) — because an existential rule at this
level would accept an approval that is *sometimes* revoked, which is
exactly the race condition §2 exists to catch. The existential quantifier
in this project always operates at the level of "which of several
independently-declared *facts* (paths, approval records) may be relied
upon" — never at the level of "which of several *states within one
temporal history* happens to be the convenient one." Conflating these two
levels would silently reopen the exact TOCTOU gap Phase 6 exists to
close.

### 14.3 Which history is retained for explanation traces

> **History affects explanation only, never reachability semantics.**
> `Safe(a)` (§9) is a pure function of `Reach(L)` **as a set** — it does
> not reference, and cannot be influenced by, which specific path the
> canonicalization procedure below happens to select. The canonical-trace
> machinery in this subsection exists purely to make a `DENY` finding
> human-readable; swapping it for any other deterministic tie-break rule
> (a different but still-deterministic path choice) would change no
> finding's `violation` literal, only the illustrative `lifecycle_trace`
> payload of an already-determined-unsafe finding. This mirrors, and is
> the direct lifecycle-domain instance of, `docs/phase-4-plan.md` §15's
> own established principle for delegation traces ("illustrative
> provenance context… not a formal proof of the specific numeric claim").

**One canonical trace per unsafe finding, chosen deterministically.**
When an approval record's `Reach(L)` contains more than one non-
`"approved"` state, the state used for diagnostic purposes is the
**lexicographically smallest such state name**, and the trace shown is
the **first BFS-discovered path** to it (§21's canonical expansion
order) — the direct analogue of `graph.CanonicalTrace`'s own "first path
BFS finds is canonical" rule, applied to state names instead of node ids.
This is illustrative provenance (*a* path proving the unsafe state is
reachable), not an enumeration of every possible path — identical in
spirit to how a delegation trace is *a* valid path, not a formal proof
of a specific numeric claim (`docs/phase-4-plan.md` §15's own framing,
reused).

When **multiple approval records**, all standing-backed, are all
unsafe/unproven for the same capability, the **canonical representative
approver** is the lexicographically smallest approver id among the
relevant subset (§18), the same ascending-lexicographic tie-break
discipline used everywhere else in this project.

---

## 15. Strict distrust semantics

Preserved and extended, per CLAUDE.md's invariant and the identical
spirit `docs/phase-4-plan.md` §11 and `docs/phase-5-plan.md` §11 already
apply to depth and approval respectively.

**A lifecycle-unsafe or lifecycle-unproven approval record contributes
nothing toward satisfying the approval requirement — not partial credit,
not "safe until the first bad transition," not a weaker warning-level
finding.** It is treated exactly as if it were not standing-backed at
all. Concretely: if three approval records back the same capability, and
two of them are lifecycle-unsafe while the third has no `lifecycle`
declared at all (hence trivially safe, §5.2), the operation is legitimate
— the two unsafe records contribute nothing, exactly as Phase 5 §11
already establishes for non-standing records, extended to a new failure
surface.

**No new code is required for Phase 1-5's own strict distrust to extend
correctly to lifecycle**, for the identical structural reason
`docs/phase-5-plan.md` §11 gives for approval extending depth/presence
distrust for free: an approval record that fails presence/binding/depth/
requester/standing upstream never even enters the set Phase 6's filter
considers (§16's five prior tiers must all pass first) — there is no
lifecycle fact to leak, partially or otherwise, from a record that was
never relied upon in the first place.

**Edge-level distrust is entirely unaffected.** Exactly as Phase 5 §4.2/
§11 establish that approval never gates delegation (only exercise),
lifecycle — being a property of approval alone — never gates delegation
either. `validEdges`, for Phase 6's trace purposes, is computed
identically to Phase 5, with zero new participation from lifecycle.

---

## 16. Interaction with Phases 1-5

Six invariants now compose over the same graph and the same `DA(n)`.
Precedence is defined **per detection point**, extending
`docs/phase-5-plan.md` §12's own framing by exactly one final tier.

### 16.1 Interaction with Phase 1 (Authority Non-Amplification)

**Zero change.** `DA(n)` computation, `isSubsetCap`, `classifyOne`, the
whole presence/absence check — byte-identical to Phase 5, which is
byte-identical to Phase 1's own machinery generalized through
`flattenApproval`. An operation whose actor never validly holds the
capability at all still fails at step 1 of the precedence chain below;
lifecycle is never evaluated.

### 16.2 Interaction with Phase 2 (Context-Binding Preservation)

**Zero change.** `classifyEdge`/`classifyOne`'s binding logic, unchanged.
A capability held only for the wrong target still fails at step 1;
lifecycle is never evaluated.

### 16.3 Interaction with Phase 3 (Requester Authorization Preservation)

**Zero change.** The requester-standing check (step 2) is unchanged,
and — confirmed directly, mirroring `docs/phase-4-plan.md` §13 and
`docs/phase-5-plan.md` §13's own confirmations for depth and approval
respectively — **a requester's own standing is never subject to a
lifecycle check.** Only the specific approval record(s) actually relied
upon to satisfy the approval-required gate are ever explored; a
requester merely needs presence in `DA(requester)`, exactly as before.

### 16.4 Interaction with Phase 4 (Delegation Depth Preservation)

**Zero change.** Depth gates transmission, not exercise, and remains
purely edge-scoped (`docs/phase-4-plan.md` §4.2/§8, reaffirmed unmodified
by Phase 5 §4.2 and again here). Lifecycle, like approval itself, is
purely operation-scoped and never edge-scoped (§8, §15) — the two
concerns cannot mask or interact with each other, exactly as depth and
approval already do not interact (`docs/phase-5-plan.md` §12).

### 16.5 Interaction with Phase 5 (Approval Preservation)

**Additive, one new final tier — Phase 5's own logic is unmodified.**
`requiresApproval`'s computation (per-node, per-capability, OR-aggregated
across valid delivering paths, `docs/phase-5-plan.md` §9/§10.1) is
untouched. `declaredApprovers`/`standingApprovers`'s precomputation
(`docs/phase-5-plan.md` §17) is untouched and reused verbatim. Phase 6
introduces exactly one new precomputed map,
`lifecycleSafeApprovers`/`lifecycleOutcome` (§26), consulted **only**
after `standingApprovers[C]` is already known to be non-empty (i.e. only
in cases that would have been an Approval-Preservation `ALLOW` under
Phase 5 alone). **Consequence, stated explicitly per the task's
requirement not to silently change Phase 5 semantics:** a version-5
document re-expressed as a version-6 document with no `approvals[].lifecycle`
field declared anywhere produces **exactly** the same findings Phase 5
would have produced (modulo the version literal) — `lifecycleSafe(⊥) =
true` unconditionally, so the new tier is a universal no-op whenever no
lifecycle is declared (§5.2, §35).

### 16.6 Full precedence table

Extending `docs/phase-5-plan.md` §12's own table by one final row-set:

| Actor holds `C`? | Requester holds `C`? | Requires approval? | Standing-backed approval exists? | ≥1 standing approval lifecycle-safe? | Finding |
|---|---|---|---|---|---|
| No (never held) | — | — | — | — | `authority_amplification` |
| No (held, wrong target) | — | — | — | — | `context_binding_violation` |
| Yes | No | — | — | — | `confused_deputy` |
| Yes | Yes | No | — | — | none — `ALLOW` |
| Yes | Yes | Yes | No (no record at all) | — | `approval_missing` |
| Yes | Yes | Yes | No (records exist, none standing) | — | `approval_unauthorized` |
| Yes | Yes | Yes | Yes | No — ≥1 proven unsafe, none safe | `approval_lifecycle_unsafe` |
| Yes | Yes | Yes | Yes | No — all remaining unproven (exploration truncated), none safe | `approval_lifecycle_unproven` |
| Yes | Yes | Yes | Yes | Yes — ≥1 safe | none — `ALLOW` |

Exactly one outcome per operation, by construction — the same guarantee
`docs/phase-3-plan.md` §12 and `docs/phase-5-plan.md` §12 establish for
their own chains, now extended by one further step (§26's pseudocode is
the literal implementation of this table).

**Two axes deliberately do not appear as rows, and their absence is
itself part of the precedence contract, not an oversight:**

- **Delegation depth (Phase 4)** never appears in this *operation-level*
  table because it is purely edge-scoped (§16.4) — it has no direct
  operation-level outcome of its own. Its only observable effect at the
  operation level is indirect: a depth-exhausted edge means the
  capability is simply absent from the actor's `DA`, which is already
  row 1 (`authority_amplification`) of this same table. There is no
  scenario in which depth and lifecycle compete for precedence on the
  same finding, because they are never candidates for the same row.
- **Illegal transitions (`unknown_lifecycle_state` and the other §20
  structural kinds)** never appear in this table because they are
  rejected entirely at `validate` time (exit 2), **before** `verify` — and
  therefore this table's whole precedence chain — ever runs at all. A
  document containing an illegal transition never reaches operation
  evaluation; there is no verify-time row for it to occupy, and no
  precedence question to resolve against any of the eight rows above.

---

## 17. Requester interaction

**Unchanged, confirmed directly** (§16.3). A requester is checked purely
via presence in `flattenApproval(da[requester])`; its own lifecycle
exposure, if it happens to also be named as an approver elsewhere in the
document, is irrelevant to its role as requester. **Symmetrically, an
approver's own lifecycle is checked only in its role as *this specific
approval record's* subject — an approver is never itself subject to a
"was the approver's own standing itself approved" recursive check** (the
identical no-infinite-regress structure `docs/phase-5-plan.md` §13
already establishes, reused verbatim: an approval record is not an
operation, so approving is never itself an act requiring approval).

---

## 18. Deterministic findings

Two new finding kinds, alongside the six existing, unmodified finding
types (`EdgeFinding`/`OperationFinding` from Phase 1,
`CapabilityEdgeFinding`/`CapabilityOperationFinding` from Phase 2,
`ConfusedDeputyFinding` from Phase 3, `DelegationDepthFinding` from
Phase 4, `ApprovalFinding` from Phase 5):

```go
// internal/report/lifecycle_finding.go

const (
	ViolationApprovalLifecycleUnsafe   = "approval_lifecycle_unsafe"
	ViolationApprovalLifecycleUnproven = "approval_lifecycle_unproven"
)

// LifecycleStep is one edge of the canonical BFS path from an approval
// record's declared initial state to the first (lexicographically
// smallest) unsafe state its own declared transitions can reach.
type LifecycleStep struct {
	From  string `json:"from"`
	Event string `json:"event"` // "" if the transition declared no event label
	To    string `json:"to"`
}

// LifecycleFinding is always an operation-level finding (point =
// "operation") — lifecycle, like approval itself, gates exercise, never
// delegation (§8, §15). DeclaredApprovers is the full sorted,
// deduplicated set of standing-backed approvers Phase 5's own check
// already narrowed the candidate set to (§16.5) — never the raw
// approvals[] array, and never empty (reaching this finding at all
// requires a non-empty standing set, §16.6). By construction (§26 step
// 4's "safe" short-circuit), every member of DeclaredApprovers is
// guaranteed to be unsafe or unproven — never safe — whenever this
// finding is emitted at all: if any standing-backed approver were safe,
// the operation would already have been ALLOWed before this finding
// type is ever constructed, so no filtering of DeclaredApprovers is ever
// needed or performed. UnsafeApprover is the canonical (lexicographically
// smallest) representative among the unsafe/unproven subset (§14.3).
// UnsafeState/LifecycleTrace are set for approval_lifecycle_unsafe and
// empty for approval_lifecycle_unproven (there is no witness state to
// report when the search itself could not complete, §22). When the
// canonical unsafe state is the automaton's own initial state (i.e. the
// declared lifecycle never even starts in "approved" — §33 test 6),
// LifecycleTrace is the empty array [] (zero hops: the automaton is
// already unsafe before any transition is taken), never null and never
// synthesized as a single degenerate self-referencing step.
type LifecycleFinding struct {
	Violation         string          `json:"violation"`
	Point             string          `json:"point"`
	Actor             string          `json:"actor"`
	Requester         string          `json:"requester"`
	Action            string          `json:"action"`
	Requires          Capability      `json:"requires"`
	DeclaredApprovers []string        `json:"declared_approvers"`
	UnsafeApprover    string          `json:"unsafe_approver"`
	UnsafeState       string          `json:"unsafe_state"`
	LifecycleTrace    []LifecycleStep `json:"lifecycle_trace"`
	Trace             []string        `json:"trace"`
	Reason            string          `json:"reason"`
}
```

`point` reuses the existing `"operation"` literal unchanged — Phase 6
introduces no new detection point (§8, §15: lifecycle is never
edge-level).

**Deterministic reason text** (generated, not free-form, matching every
prior phase's discipline):

- `approval_lifecycle_unsafe`:
  `"<action> requires <scope>@<target>, which <actor> validly holds and
  <requester> is authorized to request, and <scope>@<target> requires
  approval; <declared_approvers joined by ", "> independently hold
  standing, but none of their declared approval lifecycles can be proven
  to remain in state 'approved' — <unsafe_approver>'s can reach state
  '<unsafe_state>' via <lifecycle_trace rendered as 'a -[event]-> b -[event]->
  c'>, so it cannot be statically relied upon at time of exercise"`
- `approval_lifecycle_unproven`:
  `"<action> requires <scope>@<target>, which <actor> validly holds and
  <requester> is authorized to request, and <scope>@<target> requires
  approval; <declared_approvers joined by ", "> independently hold
  standing, but <unsafe_approver>'s declared approval lifecycle is too
  large to prove safe within the configured exploration bound — an
  unproven approval is never treated as satisfying the requirement"`

`declared_approvers`/`lifecycle_trace` are always present (`[]`, never
omitted or null — Phase 1 §9's array-field rule, unchanged).

**No new sort-key field is required** — the second finding type in the
project's history (after `ApprovalFinding`) that needs zero changes to
`sortKey`'s struct shape. `keyOf` gains one more type-switch case:
`sortKey{point: v.Point, subject: v.Actor, secondary: v.Action, scope:
v.Requires.Scope, target: v.Requires.Target, requester: v.Requester}` —
identical granularity to `ApprovalFinding`'s own key.

---

## 19. Canonical trace model

**Two independent traces, deliberately kept separate**, matching the
task's explicit instruction to distinguish state-exploration traces from
the existing static delegation trace where necessary:

1. **`Trace []string`** — unchanged in construction from every prior
   operation-level finding: `graph.CanonicalTrace(principalIDs, validEdges,
   actor) + [action]`. Describes *delegation* provenance (how the actor
   came to hold the capability at all) — nothing about approval or
   lifecycle.
2. **`LifecycleTrace []LifecycleStep`** — new, describes *lifecycle*
   provenance (how the unsafe approval record's automaton reaches its
   first unsafe state). Constructed by the bounded BFS in `internal/explore`
   (§21): the parent-pointer path from `q₀` to the canonical unsafe state,
   in traversal order, each hop carrying its `event` label (or `""` if
   none was declared).

**Maximum trace length**: bounded by `limits.MaxLifecycleStates` (§23) —
a BFS shortest path can never revisit a state, so its length is strictly
less than `|Q|`, itself bounded.

**Deterministic tie-breaking**: BFS visits states in FIFO discovery order;
at each state, outgoing transitions are expanded in ascending
lexicographic order of `(to, event)` (§21); the unsafe target state
itself is chosen as the lexicographically smallest non-`"approved"`
member of `Reach(L)` when more than one exists (§14.3) — every step of
this pipeline has a single, total, deterministic answer, with no
dependence on Go map iteration order anywhere (§25).

**JSON representation**: `lifecycle_trace` is an array of `{from, event,
to}` objects, always present (`[]` when the finding is
`approval_lifecycle_unproven`, since no witness path exists to report).

**Text rendering**: `approved -[revoke]-> revoked` (arrow-and-bracket
notation, `event` omitted from the arrow when `""`: `pending -> approved`).

---

## 20. Validation

Every existing structural rule from Phase 1-5 applies to version-6
documents unchanged, generalized only where the shape changed
(`ApprovalV6` gains an optional `Lifecycle`; principals, delegations,
operations are unchanged from v5).

**New version-6-only structural rules** (`internal/loader/loader_v6.go`):

- **`KindUnknownLifecycleState = "unknown_lifecycle_state"`** — a single
  kind covering three related conditions, mirroring the established
  "one kind, one clear underlying reason" discipline
  (`unknown_requester`/`unknown_approver`): (a) `lifecycle.initial` is
  empty or does not match any name in `lifecycle.states`; (b) a
  transition's `from` does not match any declared state name; (c) a
  transition's `to` does not match any declared state name. A missing
  (empty-string) reference and a syntactically-malformed one both fall
  into this kind, identical precedent to `docs/phase-3-plan.md` §15's
  treatment of `requester`.
- **`KindDuplicateLifecycleState = "duplicate_lifecycle_state"`** — two
  entries within one `lifecycle.states` array share the exact same name.
- **`KindDuplicateLifecycleTransition = "duplicate_lifecycle_transition"`**
  — two entries within one `lifecycle.transitions` array share the exact
  same `(from, event, to)` triple. Two transitions sharing only `from`/`to`
  but different `event` labels are not a duplicate (branching with
  distinctly-labeled alternatives is legal and expected).
- **`KindEmptyLifecycleStates = "empty_lifecycle_states"`** — a `lifecycle`
  object is present but its `states` array has zero entries (a lifecycle
  with no states at all cannot have a valid `initial`, and is rejected
  outright rather than silently treated as `⊥`, per the "explicit, not
  implicitly reinterpreted" discipline used throughout).
- **Resource-limit checks, reusing the existing generic mechanism**:
  `len(lifecycle.states) > limits.MaxLifecycleStates` and
  `len(lifecycle.transitions) > limits.MaxLifecycleTransitions` are both
  `KindResourceLimitExceeded`, with `Primary = "max_lifecycle_states"` /
  `"max_lifecycle_transitions"` respectively — no new `ErrorKind` needed,
  reusing the exact mechanism `max_approvals`/`max_delegation_depth`
  already use.
- **State/event name grammar**: `lifecycle.states[]` entries,
  `transitions[].from`, `transitions[].to`, and `transitions[].event`
  (when non-empty) all reuse the unchanged Phase 2 target grammar
  (`checkTarget`'s `^[A-Za-z0-9_.-]{1,128}$`, verbatim, zero new regex).

**Explicitly evaluated and rejected** (per the established discipline of
not adopting a suggested-list wholesale):

- **"Lifecycle never reaches `'approved'`"**, **"declares a state never
  used in any transition"**, **"approver referenced by a lifecycle-bearing
  approval record lacks standing"** — none are structural errors; all
  are `verify`-time semantic outcomes (§8, identical precedent to every
  prior phase).
- **"Cycle within a lifecycle"** — not an error at all; cycles are
  explicitly legal (§8, §11) and no acyclicity check (no
  `graph.TopoSort` call) is ever run over a lifecycle automaton.
- **"Non-object `lifecycle` value" / non-array `states`/`transitions`
  values** — not a `validateV6` concern; a JSON decode-level type
  mismatch, identical precedent to every prior phase's non-typed-value
  rejections (§6's `*Lifecycle` pointer decodes `nil` cleanly for
  omission; a present-but-wrong-shaped value fails at
  `encoding/json`'s decode step via the existing `LoadError.ParseError`
  path).
- **"Stray lifecycle-related field on a root capability, delegation, or
  operation entry"** — not a dedicated check; enforced for free by
  `DisallowUnknownFields` against `RootCapabilityV6`/`DelegationV6`/
  `OperationV6`'s unchanged (from v5) field sets.

`validate` on a version-6 document therefore still never evaluates any
invariant — Non-Amplification, Context-Binding, Requester Authorization,
Delegation Depth, Approval Preservation, or Temporal Approval
Preservation — exactly as established for v1-v5.

**Validation-diagnostic determinism**: every new §20 `ValidationError`
(`unknown_lifecycle_state`, `duplicate_lifecycle_state`,
`duplicate_lifecycle_transition`, `empty_lifecycle_states`, plus the two
resource-limit kinds) is appended to the same `errs []ValidationError`
slice every other v6 structural check already appends to, and is sorted
by the existing, unmodified `sortErrors` call `decodeAndValidateV6`
makes before returning — the identical mechanism `decodeAndValidateV5`
already uses verbatim (`internal/loader/loader_v5.go`'s own
`decodeAndValidateV5`, reused as the literal template). No new sort
logic, comparator, or ordering rule is introduced for validation
diagnostics; a document with multiple simultaneous v6 structural errors
(e.g. two independent `unknown_lifecycle_state` violations in different
approval records) produces identical, deterministically-ordered output
across repeated runs and across array-reordered input purely because
`sortErrors` already sorts by the existing `ValidationError` fields
(`Kind`, `Primary`, `Secondary`), unmodified — §33 test 63.

---

## 21. Exploration algorithm

**Decision: a new, small, generic package, `internal/explore`,
implementing exactly one operation — bounded breadth-first reachability
over a possibly-cyclic, possibly-branching labeled directed graph — used
by `verify_v6.go` once per distinct lifecycle-bearing approval record.**
§30 (architecture) justifies why this is a new package rather than a
function embedded directly in `verify_v6.go` or a reuse of
`internal/graph`.

```go
// internal/explore/explore.go

// Transition is one labeled directed edge of a small, possibly-cyclic
// state graph — the exploration-domain analogue of graph.Edge, distinct
// from it because this domain permits cycles and internal/graph's
// TopoSort explicitly does not (docs/phase-6-plan.md §1, §21).
type Transition struct {
	From  string
	Event string
	To    string
}

// Result is the outcome of one bounded BFS run from a single source
// state. Reachable is the full visited-state set (including Initial
// itself). Path[q] is the canonical (first-BFS-discovered) sequence of
// transitions from Initial to q, for every q in Reachable except
// Initial itself (Path[Initial] is empty). Truncated is true only if
// maxStates was reached before the BFS frontier was naturally exhausted
// — i.e. the search is incomplete and Reachable/Path must not be
// treated as final (docs/phase-6-plan.md §22).
type Result struct {
	Reachable map[string]bool
	Path      map[string][]Transition
	Truncated bool
}

// Explore runs a deterministic, bounded BFS from initial over the graph
// implied by transitions, visiting at most maxStates distinct states.
// Determinism (docs/phase-6-plan.md §25): the BFS frontier is a plain
// FIFO queue (first-discovered, first-expanded); outgoing transitions
// from any one state are considered in ascending lexicographic order of
// (To, Event), computed by sorting a local slice, never by ranging a
// map — so the result (including every Path entry) is a pure function
// of (initial, transitions), independent of Go's map iteration order and
// independent of the order transitions were declared in the input
// document.
func Explore(initial string, transitions []Transition, maxStates int) Result
```

**Pseudocode**, implementation-grade:

```
func Explore(initial, transitions, maxStates):
    adj := map[string][]Transition{}
    for t in transitions:
        adj[t.From] = append(adj[t.From], t)
    for state in keys(adj):                       // deterministic: sort local slice
        sort adj[state] ascending by (t.To, t.Event)

    reachable := { initial: true }
    path := { }                                     // path[initial] left unset (empty)
    queue := [ initial ]                             // FIFO
    truncated := false

    while queue is non-empty:
        if len(reachable) > maxStates:
            truncated = true
            break
        cur := pop-front(queue)
        for t in adj[cur]:                           // already sorted, deterministic
            if not reachable[t.To]:
                reachable[t.To] = true
                path[t.To] = path[cur] + [t]
                push-back(queue, t.To)

    return Result{reachable, path, truncated}
```

**Why BFS, not DFS**: BFS's shortest-path property gives the smallest
possible `LifecycleTrace` for any given unsafe state (§19's bound), and
BFS's natural level-by-level frontier is what makes "visited more than
`maxStates` distinct states" a clean, unambiguous truncation signal
checked once per dequeue — identical reasoning to why `graph.CanonicalTrace`
(§1) already uses BFS from multiple simultaneous roots for the identical
"first-discovered path is canonical, and shortest" property.

**Canonical queue ordering**: pure FIFO — first state discovered is the
first state expanded. Combined with the sorted-adjacency-list expansion
rule, this makes traversal order a total function of `(initial,
transitions)` alone.

**Visited-state representation**: a plain `map[string]bool`, used purely
as a set (no iteration over it ever affects output — every observable
value derived from it, `Path` and `Truncated`, is populated during the
single deterministic BFS pass itself, never by ranging the map
afterward; §14.3's "lexicographically smallest unsafe state" selection,
performed by the caller in `verify_v6.go`, explicitly sorts the
candidate names before choosing — see §25).

**State canonicalization / hashing / key construction**: state names are
used directly as map keys — they are already the canonical
representation (author-declared strings, validated at load time to a
fixed grammar, §20); no additional hashing, interning, or normalization
step exists or is needed.

**Deduplication**: implicit in the `reachable` set — a state already
present is never re-queued (the `if not reachable[t.To]` guard), so each
state is expanded at most once, giving the `O(|Q| + |δ|)` bound §23/§28
rely on.

**Transition ordering**: ascending lexicographic `(To, Event)`, computed
once per source state via a local sort, never via map range order.

**Early termination**: none beyond the `maxStates` truncation check —
the algorithm always explores the *complete* reachable set (up to the
bound), never stops early upon finding the first unsafe state, because
the canonical-unsafe-state selection rule (§14.3, "lexicographically
smallest") requires knowing the *full* reachable set to choose correctly,
not merely *a* reachable unsafe state. This is a deliberate completeness
requirement, not an oversight — a "stop at the first unsafe state found"
version would make the reported `unsafe_state` depend on transition
declaration order, breaking determinism (§25).

**Complete-search behavior**: when `Truncated = false`, `Reachable` is
guaranteed to be the *entire* set of states reachable from `initial`
(standard BFS completeness) — the safety predicate (§9) may be evaluated
with full confidence.

**Result invariants**, maintained by construction and relied upon by
`verify_v6.go` without further defensive checking: (1) `Reachable ⊆ Q`
always (§22.1's proof rests on this); (2) `initial ∈ Reachable` always
(§12); (3) for every `q ∈ Reachable` with `q ≠ initial`, `Path[q]` is
defined and non-empty — a state is only ever added to `Reachable` in the
same step its `Path` entry is written (the pseudocode's `reachable[t.To]
= true` and `path[t.To] = ...` lines execute together, never one without
the other), so there is no reachable non-initial state with a missing or
inconsistent path; (4) when `Truncated = true`, `Reachable`/`Path` reflect
only a prefix of the true reachable set and carry no completeness
guarantee — callers must consult only the `Truncated` flag itself, never
attempt a partial-safety inference from a truncated `Reachable` (§22's
explicit "partial results: not emitted" rule).

**Finding generation / trace reconstruction**: performed by the caller
(`verify_v6.go`'s `lifecycleSafe` helper, §26), consuming `Result`
directly — `internal/explore` itself has no knowledge of `"approved"`,
findings, or the report package at all, keeping it a genuinely generic,
reusable primitive (§30).

---

## 22. Bounded exploration and limit exhaustion semantics

**Bounds, and the rationale for each — not arbitrary large numbers:**

| Limit | Value | Kind | Rationale |
|---|---|---|---|
| `MaxLifecycleStates` | `32` | Validate-time structural bound on `len(lifecycle.states)` per approval record | An approval lifecycle is a small, human-authored policy artifact (pending/approved/revoked/expired/resubmitted and similar), not a generated or derived structure — 32 distinct named states is generous headroom over any realistic workflow while keeping the per-record automaton small enough that a complete BFS is always cheap. Independent var, not a shared alias, mirroring `MaxDelegationDepth`'s own independence from `MaxChainDepth` (CLAUDE.md). |
| `MaxLifecycleTransitions` | `128` | Validate-time structural bound on `len(lifecycle.transitions)` per approval record | Set to `4×MaxLifecycleStates`, generous headroom for a densely-connected small automaton (up to 4 outgoing edges per state on average) while staying well inside the theoretical maximum a 32-state graph with self-loops could ever contain: `|Q|² = 32² = 1024` distinct `(from, to)` pairs (`128 ≤ 1024`, so the bound is mathematically coherent — it is not merely round, it sits at exactly one-eighth of the largest graph `MaxLifecycleStates` could even express). A document declaring anywhere near that density is almost certainly malformed, not a legitimate policy. |
| `MaxExplorationStatesPerLifecycle` | `32` | Runtime BFS visited-state safety valve, consulted by `Explore`'s `maxStates` parameter | A resource-safety valve on the *algorithm's own execution*, distinct in kind from `MaxLifecycleStates` (a bound on what a document may *declare*) — the identical two-independent-bounds-same-default-value pattern CLAUDE.md establishes for `MaxChainDepth`/`MaxDelegationDepth`. Set equal to `MaxLifecycleStates` because a BFS visited-set can never legitimately need to exceed the number of states a validate-time-legal document could possibly declare — this bound exists purely as defense-in-depth (§22.1), not because realistic input can reach it. |
| `MaxApprovals` | `10,000` (unchanged from Phase 5) | Validate-time bound on total `approvals[]` entries | Unchanged — bounds how many lifecycle-bearing records a single document may declare, keeping total exploration work linear (§23). |

**Why these are not "arbitrary huge numbers":** every value above is
derived either from a concrete realistic-authoring ceiling (a compliance
workflow does not plausibly need more than a few dozen named states) or
directly from another already-justified bound (`MaxLifecycleTransitions`
as a small multiple of `MaxLifecycleStates`; `MaxExplorationStatesPerLifecycle`
set exactly equal to `MaxLifecycleStates` because no larger value could
ever be exercised by a validate-time-legal document). This mirrors
`docs/phase-4-plan.md` §21's own explicit reasoning for
`MaxDelegationDepth` ("set equal in value to `MaxChainDepth`… since no
chain can ever be longer… a declared budget beyond that is unreachable
and therefore meaningless") almost exactly.

### 22.1 Why exploration exhaustion cannot occur under a validate-time-legal document, and why the fail-closed path must exist and be tested anyway

**Explicit proof that exhaustion cannot occur under a validate-time-legal
document**, per the task's own demand to prove this relationship rather
than assert it: `internal/explore.Explore` only ever adds a state `q` to
`Reachable` if `q` appears as some transition's `to` (or is `initial`
itself, §12) — every such `q` is, by §20's `unknown_lifecycle_state`
check, guaranteed to be a member of the document's own declared `Q`
(`lifecycle.states`). Therefore `Reachable ⊆ Q` always holds, for any
input, by construction of the algorithm (there is no code path that adds
a state to `Reachable` from any source other than a declared transition's
`to` or the declared `initial`). A document that passed `validate` has
`|Q| ≤ MaxLifecycleStates` (§20's resource-limit check). Combining these:
`|Reachable| ≤ |Q| ≤ MaxLifecycleStates = MaxExplorationStatesPerLifecycle`
at every point during the BFS, so the truncation condition (`len(reachable)
> maxStates`, §21's pseudocode) is `false` at every check for the entire
run — **truncation cannot trigger for any validate-time-legal input,
proved directly from the algorithm's own invariant `Reachable ⊆ Q`, not
merely asserted.** This is a deliberate design property, not an accident:
exactly as `MaxDelegationDepth`
being set equal to `MaxChainDepth` makes chain-length-based depth
exhaustion structurally impossible under legal input
(`docs/phase-4-plan.md` §21), Phase 6's bounds are chosen specifically so
the runtime safety valve is defense-in-depth against an implementation
bug (e.g. a future change that lets `MaxLifecycleStates` be exceeded
without a corresponding validate-time check), never a scenario a
legitimate document author can actually trigger.

**No separate exploration-depth bound is needed, and none is defined.**
A BFS shortest path can never revisit a state (§19), so any path length
(the `LifecycleTrace` bound the task asks about explicitly) is strictly
less than `|Reachable|`, which is itself `≤ MaxLifecycleStates` by the
proof above — depth is therefore already, automatically subsumed by the
state-count bound, with no independent "max exploration depth" variable
required. Introducing one anyway would be a redundant, unjustified
third bound over the same already-bounded quantity — exactly the kind of
"arbitrary limit without justification" this project's own discipline
(`docs/phase-4-plan.md` §21) warns against.

**The fail-closed code path is nonetheless mandatory, specified exactly,
and must be exercised in tests via a lowered `limits.MaxExplorationStatesPerLifecycle`**
— the identical white-box testing discipline CLAUDE.md requires of every
resource bound in this project ("every resource bound… is an exported
var specifically so tests can lower it and exercise the bound without
generating huge fixtures").

**This is security-critical, and the system MUST fail closed, per the
task's explicit requirement.** Specification:

- **Not a `validate`-time error.** A document that is itself structurally
  legal (states/transitions within their own declared bounds) but whose
  exploration is truncated by the defense-in-depth runtime ceiling is
  not malformed — it is a `verify`-time outcome, consistent with the
  "structurally valid, semantically undetermined" category every prior
  phase already has (e.g. an approval referencing an unknown capability,
  §8).
- **Machine-readable kind**: `approval_lifecycle_unproven` (§18) — a
  distinct violation literal from `approval_lifecycle_unsafe`, so a
  reader can tell "proven unsafe" apart from "could not be proven either
  way."
- **Never returns `ALLOW`.** If `Explore` reports `Truncated = true` for
  every standing-backed approval record relied upon by an operation (and
  none of the *completed* records among them was independently `Safe`),
  the operation is `DENY`ed via `approval_lifecycle_unproven` — an
  incomplete search is treated identically in outcome to a failed one
  (§9, §16.6's table). This is the direct implementation of "strict
  distrust" (§15) applied to the exploration process itself: an unproven
  fact contributes nothing, exactly as an unauthorized or absent one
  does.
- **Text output**: rendered with `unsafe_state`/`lifecycle_trace` both
  empty (there is no witness path to show — the search never completed),
  and the reason text explicitly says the lifecycle "is too large to
  prove safe within the configured exploration bound" (§18) — never
  phrased as if a specific unsafe state were found, which would overstate
  what was actually proven.
- **JSON output**: `"violation": "approval_lifecycle_unproven"`,
  `"unsafe_state": ""`, `"lifecycle_trace": []` — present, empty, never
  omitted or null (Phase 1 §9's array-field rule, unchanged).
- **Exit code**: unchanged, `1` (`Deny`) — a `verify`-time finding like
  every other violation kind (§27).
- **Partial results**: not emitted. `Explore`'s `Result.Reachable`/`Path`
  when `Truncated = true` reflect only a prefix of the true reachable
  set and are **never** consulted by `lifecycleSafe` for anything beyond
  the truncation flag itself — no partial-safety inference is drawn from
  a truncated result under any circumstance.

---

## 23. State equivalence

**There is, deliberately, no coarser equivalence relation being applied
at all — state identity *is* the declared name, with nothing else to
collapse.** This is a stronger and more easily audited claim than "the
equivalence relation is sound"; it is the claim that Phase 6 never faces
the general equivalence-soundness problem in the first place, because the
state key has exactly one field.

**Adversarial framing, addressed directly**: an unsound equivalence would
merge two states that a *future* transition or security decision could
still tell apart — i.e. a field is excluded from the key that mattered.
Enumerate every candidate field a state in this design could have had,
and confirm none is excluded improperly:

- **The state name itself** — included in the key (it *is* the key).
- **Which transitions are declared out of a state** — not a separate
  field at all; it is recomputed identically every time a given name is
  visited (`adj[name]`, §21), because the adjacency structure is static,
  declared input, read once (§10's "Mutability"). Two arrivals at the
  same name therefore have byte-identical outgoing options by
  construction, not by an equivalence *decision*.
- **How the state was reached (the specific path/history)** — genuinely
  excluded from the key, but proved harmless in §14.3: `Safe(a)` (§9) is
  defined purely as a predicate over `Reach(L)` **as a set of names**; no
  downstream computation (the safety verdict, the finding's `violation`
  literal) ever inspects *how many* times a state was reached or *via
  which* route, only *whether*. Only the **diagnostic trace** — explicitly
  never a determinant of safety, §14.3's boxed principle — consults path
  information, and it does so via `Path[q]`, populated once per state at
  first discovery (§21's Result invariants), never via the equivalence
  relation.
- **Which approval record the state belongs to** — not excluded from
  anything: it is not part of the state name's namespace at all.
  Equivalence is scoped **per approval record**; two different approval
  records (different `(approver, scope, target)` triples) never share a
  namespace of state names, even if they happen to declare
  identically-spelled states (`"approved"` in one record's automaton has
  no relationship whatsoever to `"approved"` in a different record's
  automaton, consistent with §3's "lifecycles are explored completely
  independently" decision) — `internal/explore.Explore` is called once
  per record, with a fresh, unshared adjacency map each time, so
  cross-record name collision is not merely harmless, it is structurally
  impossible to observe.

**Conclusion**: no field that can affect a future safety decision is
excluded from the state key (there is no such field to exclude), and no
field that is excluded (path history) can affect a future safety
decision (proved by §9's definition and §14.3's separation). The
equivalence relation used — literal string equality, scoped per record —
is therefore sound, not by a delicate argument about which fields happen
not to matter, but because the state space was designed from the outset
(§10) to have no field that *could* raise the question.

---

## 24. Canonicalization

A lifecycle's canonical representation, for the purposes of exploration,
is simply its declared `(initial, states, transitions)` triple, with no
document-order dependence anywhere in how it is consumed: `internal/explore`
builds its adjacency structure by iterating `transitions` once (order
does not matter — the adjacency map is built by appending, and each
source state's outgoing list is explicitly sorted before use, §21), so
two documents differing only in the declared order of `lifecycle.states`
or `lifecycle.transitions` arrays produce byte-identical `Result` values
and byte-identical findings. This is the lifecycle-domain instance of the
project's existing, general "array reordering in semantically-equivalent
input never changes output" guarantee (README's "Determinism" section),
extended to a new array type.

---

## 25. Deterministic exploration

Every determinism requirement the task lists is satisfied, and each is
traceable to a specific, already-established mechanism reused rather than
invented:

- **Deterministic transition ordering**: ascending lexicographic `(To,
  Event)` at each source state (§21), computed via an explicit sort of a
  local slice — never by ranging a map.
- **Deterministic queue/stack behavior**: plain FIFO BFS, no priority
  queue, no randomization (§21).
- **Deterministic state identity**: exact string equality (§23) — no
  hashing with any collision or ordering ambiguity.
- **Deterministic trace choice**: BFS shortest-path, first-discovered
  wins (§19, §21) — the identical rule `graph.CanonicalTrace` already
  uses for delegation traces, applied to a different domain.
- **Deterministic finding order**: `report.Sort`'s existing 6-tuple
  total order, unchanged, with `LifecycleFinding`'s key computed
  identically to `ApprovalFinding`'s (§18) — no new comparator logic.
- **Validation diagnostics**: the existing, unmodified `sortErrors`
  mechanism, reused verbatim (§20) — no new comparator logic for v6's
  structural errors either.
- **The one place `classify` (§26) ranges a map** (`for q := range
  res.Reachable`) **is safe, and is not a violation of the "never rely on
  map iteration order" rule**, for the same reason CLAUDE.md's own rule
  gives two acceptable patterns ("either sort the keys first or prove the
  iteration order can never affect output"): the loop's only effect is to
  build an `unsafe []string` slice, which is **unconditionally sorted**
  (`sort.Strings(unsafe)`) immediately afterward and *only then*
  consumed (`unsafe[0]`) — so the final, observable selection is a pure
  function of the *set* `res.Reachable`, never of the order the map
  happened to be ranged in. This is the identical pattern
  `verify_v5.go`'s own `declaredApprovers` construction already uses
  (`for approver := range set { list = append(list, approver) };
  sort.Strings(list)`) — reused, not invented.
- **Repeated-run byte identity**: follows mechanically from every rule
  above — `Explore`'s only inputs are `(initial, transitions, maxStates)`,
  all pure, immutable, declared data; its only intermediate state is a
  `map[string]bool` used exclusively as a set with no output derived from
  iterating it (§21); and the caller's own selection of the canonical
  unsafe state/representative approver is itself an explicit
  ascending-lexicographic sort (§14.3), never a map range. **No algorithm
  in this design relies on Go map iteration order at any point** — the
  same audit CLAUDE.md requires of every existing package holds for
  `internal/explore` and `verify_v6.go`'s new code by construction.

---

## 26. Verification algorithm

**Decision: Phase 6 still fits entirely within one static, deterministic,
topological pass over the delegation graph (unchanged, reused verbatim
from `RunV5`) plus one bounded pre-indexing pass over `approvals[]`
(unchanged, reused verbatim from `RunV5`) plus exactly one new, additive,
bounded pre-indexing pass: one independent, complete `Explore` run per
distinct lifecycle-bearing approval record.** No state-space exploration
is introduced into the graph/DA(n) computation itself — exploration is
entirely confined to the new, separate lifecycle-safety pre-pass, exactly
matching the "layer onto, not replace" mandate (§0, `docs/phase-5-plan.md`
§31).

`RunV6(*model.ModelV6) report.Result`, structurally parallel to `RunV5`:

1. **Build the graph and compute `da`.** Byte-for-byte the same steps
   `RunV5` already performs, over the unchanged `authState{remaining,
   configuredMax, requiresApproval}` — reusing `verify_v5.go`'s
   unexported `authState` type and `flattenApproval` function directly
   (same package, zero duplication, the identical "same package, no
   duplication" pattern `verify_v5.go`'s own header comment establishes
   for its own reuse of `verify_v2.go`/`verify_v3.go` helpers).
2. **Index approvals** (unchanged from `RunV5` §17): `declaredApprovers`,
   `standingApprovers`, computed identically.
3. **Index lifecycle safety** (new): for every approval record `a` with
   `a.Lifecycle != nil`, run `internal/explore.Explore(a.Lifecycle.Initial,
   transitionsOf(a.Lifecycle), limits.MaxExplorationStatesPerLifecycle)`
   exactly once, and classify the outcome:
   ```go
   type lifecycleOutcome int
   const (
       lifecycleSafeOutcome lifecycleOutcome = iota
       lifecycleUnsafeOutcome
       lifecycleUnprovenOutcome
   )

   func classify(res explore.Result) (outcome lifecycleOutcome, unsafeState string, path []explore.Transition) {
       if res.Truncated {
           return lifecycleUnprovenOutcome, "", nil
       }
       var unsafe []string
       for q := range res.Reachable {
           if q != "approved" {
               unsafe = append(unsafe, q)
           }
       }
       if len(unsafe) == 0 {
           return lifecycleSafeOutcome, "", nil
       }
       sort.Strings(unsafe)               // deterministic selection, §14.3
       first := unsafe[0]
       return lifecycleUnsafeOutcome, first, res.Path[first]
   }
   ```
   Records with `a.Lifecycle == nil` are treated as `lifecycleSafeOutcome`
   unconditionally, with no `Explore` call at all (§5.2, §9). Results are
   cached once per `(approver, scope, target)` key — computed a single
   time regardless of how many operations reference the same capability,
   the identical "precompute once, O(1) lookup per operation" discipline
   `docs/phase-5-plan.md` §17 already establishes for `standingApprovers`.
4. **Operation evaluation**, operations in the existing ascending
   `(actor, action, requires.Scope, requires.Target, requester)` order
   (unchanged from `docs/phase-5-plan.md` §17): run §16.6's five-step
   precedence — steps 1-3 (presence/binding, requester, approval-declared/
   standing) are byte-identical to `RunV5`'s own steps 1-3; step 4 is new:
   ```
   standingForCap := standingApprovers[requires]    // already known non-empty (step 3 passed)
   var safe, unsafe, unproven []string               // all three built from standingForCap, already ascending-lex sorted
   for approver in standingForCap:
       switch outcomeCache[(approver, requires)].outcome:
       case lifecycleSafeOutcome:    safe    = append(safe, approver)
       case lifecycleUnsafeOutcome:  unsafe  = append(unsafe, approver)
       case lifecycleUnprovenOutcome: unproven = append(unproven, approver)

   if len(safe) > 0:
       continue                                       // ALLOW, no finding
   if len(unsafe) > 0:
       emit LifecycleFinding(ViolationApprovalLifecycleUnsafe,
           declaredApprovers: standingForCap,
           unsafeApprover: unsafe[0],                  // lex-smallest, §14.3
           unsafeState/lifecycleTrace: outcomeCache[(unsafe[0], requires)])
       continue
   // unproven is non-empty (standingForCap was non-empty and safe/unsafe are both empty)
   emit LifecycleFinding(ViolationApprovalLifecycleUnproven,
       declaredApprovers: standingForCap,
       unsafeApprover: unproven[0])                    // lex-smallest, §14.3
   ```
5. **Sort all findings** (all eight finding shapes together) by the
   unmodified 6-tuple key.
6. **Result:** `ALLOW` (exit 0) if empty, else `DENY` (exit 1) —
   unchanged result semantics.

**Complexity.** Let `N` = nodes, `E` = delegation edges, `A` = per-edge/
per-principal capability-set-size bound, `O` = operations, `Ap` =
approval records, `Ls`/`Lt` = the per-record bounds on lifecycle states/
transitions (`MaxLifecycleStates`, `MaxLifecycleTransitions` — both small
constants, ≤ 32 / ≤ 128). Steps 1-2 are unchanged from Phase 5's
`O(N + E·A + O + Ap log Ap)`. Step 3 runs one bounded BFS per
lifecycle-bearing approval record, each costing `O(Ls + Lt)` (standard
BFS: linear in the automaton's own state/edge count, §21) — summed across
at most `Ap` records, this is `O(Ap · (Ls + Lt))`. Because `Ls`/`Lt` are
fixed constants independent of `N`/`E`/`O`/`Ap`, this term is `O(Ap)`
with a bounded constant factor, **not** a new asymptotic class — the
identical "one additional independently-bounded linear-ish term, not a
multiplicative blow-up" shape `docs/phase-5-plan.md` §17 already
establishes for its own `Ap log Ap` term. Step 4 is `O(1)` amortized per
operation (a map lookup into the precomputed `outcomeCache`, plus a
bounded-size sort over `standingForCap`, itself bounded by
`MaxAuthoritySetSize`-adjacent considerations already accounted for in
Phase 5's own bound), so `O(O)` total.

**The whole pass is therefore `O(N + E·A + O + Ap log Ap + Ap·(Ls + Lt))`**
— every term already present in Phase 5, plus one new term that is linear
in `Ap` with a small fixed multiplier. **This is emphatically not a
polynomial-in-the-general-model-checking-sense claim being smuggled in
where exponential behavior actually exists** — per §3's explicit
analysis, the *reason* this stays linear is the deliberate decision to
explore each lifecycle **independently**, never composed into a cross-
product global state. If a future phase ever needed to reason about
multiple approvals' lifecycles *jointly* (nothing in the current threat
model requires this, §37), that would reintroduce a genuinely exponential
`O(∏ᵢ|Qᵢ|)` term, and this document explicitly does not claim otherwise
for that hypothetical — it claims linearity only for the narrow,
independently-explored design actually specified here.

`model.Model` (v1) through `model.ModelV5` (v5) continue to run their
existing, entirely untouched `Run`/`RunV2`/`RunV3`/`RunV4`/`RunV5`
functions, byte-identical to today.

---

## 27. CLI compatibility

**No new subcommands, no new flags.** `validate <model.json>` and
`verify <model.json> [--format text|json]` remain the only two commands
— nothing about temporal approval lifecycle motivates a dedicated
command; it is exactly one more schema version dispatched through the
existing two verbs, the same posture every prior phase has taken.
`main.go`'s existing dispatch switch in `runVerify` gains one more case:

```go
case doc.V6 != nil:
    result = verify.RunV6(doc.V6)
```

`--format text|json` applies identically across all six versions. No
`--explore-budget` override flag, no version-selection flag — version is
read from the document, and the exploration bound is a fixed, exported
`internal/limits` var, exactly like every other resource bound in the
project (white-box-testable, never user-configurable at the CLI, §22).

---

## 28. Text/JSON compatibility

**JSON.** The top-level envelope (`{"result", "findings"}`,
`internal/report/json.go`) is unchanged — already generic over
`[]interface{}`, so `LifecycleFinding` requires zero changes to
`RenderJSON`. Version-1 through version-5 output is byte-identical to
today, unconditionally: `RunV6` is a new function, called only when
`doc.V6 != nil`, touching no code path any prior `Run*` function
executes.

**Text.** `RenderText`'s type switch gains one new case:

```
[1] approval_lifecycle_unsafe (operation)
  actor:               billing-agent
  requester:           admin
  action:              refund
  requires:            billing:refund@billing-service
  declared approvers:  compliance-officer
  unsafe approver:     compliance-officer
  unsafe state:        revoked
  lifecycle trace:     approved -[revoke]-> revoked
  trace:               admin -> billing-agent -> refund
  reason:              refund requires billing:refund@billing-service, which billing-agent validly holds and admin is authorized to request, and billing:refund@billing-service requires approval; compliance-officer independently holds standing, but none of their declared approval lifecycles can be proven to remain in state 'approved' — compliance-officer's can reach state 'revoked' via approved -[revoke]-> revoked, so it cannot be statically relied upon at time of exercise
```

(Exact column labels/widths are an implementation-session detail, matching
the latitude every prior phase's plan already left for text rendering.)

---

## 29. Exit codes

Unchanged. `internal/exitcode` gains no new values:

| Code | Meaning (extended) |
|---|---|
| `0` | Structurally valid model (v1-v6); zero findings for `verify`. |
| `1` | One or more findings — any of the eight violation literals, in any combination, including `approval_lifecycle_unsafe`/`approval_lifecycle_unproven`. |
| `2` | Structural/model problem for any schema version, including the new `unknown_lifecycle_state`, `duplicate_lifecycle_state`, `duplicate_lifecycle_transition`, `empty_lifecycle_states` kinds and the two new resource limits. |
| `3` | CLI usage error — unchanged. |

**Why no new exit code**: exploration-limit exhaustion is deliberately
*not* a distinct exit-code category — it is a `verify`-time finding
(exit 1), per §22's fail-closed specification, not a `validate`-time
structural error (exit 2) and not a new class of outcome requiring its
own code. This directly follows the task's own steer to prefer existing
exit codes absent a compelling contract reason, and none exists here: a
finding the reader must act on (narrow the lifecycle, or accept the
`ALLOW` is not provable) is exactly what exit 1 already means.

---

## 30. Resource safety, complexity, and architecture impact

### 30.1 Resource safety

Attacker-controlled complexity is bounded on every dimension the task
asks about:

| Dimension | Bound | Mechanism |
|---|---|---|
| CPU | `O(N + E·A + O + Ap log Ap + Ap·(Ls+Lt))` | §26; every term independently bounded by an existing or new `limits` var |
| State count (per lifecycle) | `Ls ≤ 32` | `MaxLifecycleStates`, validate-time (§20) |
| State count (total, across all lifecycles) | `Ap · Ls ≤ 320,000` | Product of two independently bounded, already-justified constants |
| Memory | `O(Ap·(Ls+Lt))` for cached outcomes/traces, `O(N + E·A + O)` unchanged from Phase 5 for everything else | Every new allocation is sized from a bounded input quantity, never from a derived/combinatorial one |
| Transition count | `Lt ≤ 128` per lifecycle | `MaxLifecycleTransitions`, validate-time (§20) |
| Trace storage | `O(Ls)` per `LifecycleTrace` (BFS shortest path, strictly `< Ls` hops) | §19 |
| Input dimensions | `lifecycle.states`/`lifecycle.transitions` array sizes | Both validate-time bounded (§20), both defended in depth at verify time (§22) |

**Combinatorial explosion is prevented specifically by the
independent-per-record exploration decision (§3, §26)** — the one design
choice this entire safety analysis rests on. No cross-approval
composition exists anywhere in the algorithm, so no exponential term can
arise from any input a validate-time-legal document can construct.

### 30.2 Architecture impact

**Files to add:**

```
internal/model/types_v6.go       — Lifecycle, LifecycleTransition, ApprovalV6, ModelV6,
                                    PrincipalV6, AgentV6, DelegationV6, OperationV6 (§6)
internal/explore/explore.go      — Transition, Result, Explore (§21)
internal/explore/explore_test.go — unit tests for bounded BFS: cycles, branching,
                                    self-loops, determinism, truncation (§34)
internal/loader/loader_v6.go     — decodeAndValidateV6, validateV6, checkApprovalsV6,
                                    checkLifecycle, new ErrorKind constants (§20)
internal/loader/loader_v6_test.go
internal/report/lifecycle_finding.go — LifecycleFinding, LifecycleStep,
                                        ViolationApprovalLifecycleUnsafe/Unproven,
                                        NewLifecycleFinding constructor (§18);
                                        extends finding.go's keyOf switch with one case
internal/verify/verify_v6.go     — RunV6, lifecycle outcome cache/classify (§26)
internal/verify/verify_v6_test.go
cmd/delegationproof/main_v6_test.go
examples/billing-approval-lifecycle.json   — worked example (§32)
testdata/malformed/…             — one fixture per new §20 error kind
testdata/valid-v6/…              — clean-pass, reordered-arrays, unsafe-lifecycle,
                                    unproven-lifecycle, multi-approver, combined-
                                    violations fixtures (§34)
testdata/golden/…                — captured text/json output, generated from the
                                    built binary, diffed for intent (CLAUDE.md)
```

**Files to modify** (sanctioned touches only, mirroring every prior
phase's own minimal-touch discipline):

```
internal/loader/loader_v2.go     — Document struct gains V6 field; LoadDocument switch
                                    gains "6" case; the six-version invalid_version message
internal/loader/loader.go, loader_v3.go, loader_v4.go, loader_v5.go
                                  — the five sanctioned message-text touches (§6)
internal/loader/loader_v2_test.go — the one sanctioned test-string update
internal/report/finding.go       — keyOf gains one LifecycleFinding case (§18); no
                                    struct-shape change
internal/report/text.go          — RenderText gains one type-switch case (§28)
cmd/delegationproof/main.go      — runVerify dispatch gains one case (§27)
internal/limits/limits.go        — four new exported vars (§22)
```

**Files that remain completely untouched**: `internal/model/types.go`
through `types_v5.go`; `internal/loader/loader.go`'s decode/validate
logic for v1-v5 (only the message-text line changes); `internal/graph/graph.go`
in its entirety — no change of any kind, not even an addition, since
`internal/explore` is a new, separate package rather than an extension of
`graph`'s API surface (§30.3); `internal/verify/verify.go` through
`verify_v5.go` in their entirety, including the `authState`/`flattenApproval`
helpers `verify_v6.go` reuses by reference, unmodified; `internal/report/approval_finding.go`,
`capability_finding.go`, `confused_deputy_finding.go`,
`delegation_depth_finding.go` in their entirety; every existing `testdata/golden/`
fixture (byte-identical, per acceptance criteria §39); `schemas/model.md`
(not touched by this planning session, per the task's explicit
instruction, deferred to the implementation session exactly as
`docs/phase-5-plan.md` §30 item 5 deferred its own v5 section).

### 30.3 Why `internal/explore` is a new package, not a function in `verify_v6.go` or an extension of `internal/graph`

Justified directly, per the task's explicit requirement:

- **Not an extension of `internal/graph`**: `graph.TopoSort` is
  documented and implemented as strictly DAG-only — it *detects* cycles
  specifically in order to reject them as a structural error
  (`internal/loader`'s cycle check, §1). A lifecycle automaton is
  explicitly, legitimately cyclic (§8, §11). Adding cycle-tolerant
  reachability to the same package that exists specifically to enforce
  acyclicity elsewhere would blur a contract the whole project currently
  relies on being unambiguous — a future reader of `internal/graph` should
  be able to assume every function in it assumes/enforces a DAG, without
  having to check which functions are the one exception. `graph.CanonicalTrace`
  is architecturally the *closest* existing primitive (also a BFS,
  §11/§21 note the parallel explicitly) but it too implicitly assumes
  the DAG structure the delegation graph already guarantees (its own
  BFS-over-`adj` would loop forever with no bound if handed a cyclic
  input, since it has no visited-set-size ceiling — it doesn't need one,
  because the loader already guarantees acyclicity before it is ever
  called); reusing it as-is for lifecycle exploration would silently
  drop the bounded-truncation safety property §22 requires.
- **Not a function embedded directly in `verify_v6.go`**: the bounded-
  reachability algorithm has zero dependency on `model`, `report`, or any
  DelegationProof-specific concept — it operates purely on strings and a
  transition list. Keeping it in `internal/verify` would couple a
  generically useful, independently testable primitive to a package whose
  own stated purpose (per every prior phase's package-level doc comment)
  is "derived authority computation, finding assembly" — not general graph
  algorithms. A dedicated package (mirroring exactly how `internal/graph`
  itself was factored out in Phase 1 rather than embedded in `internal/verify`)
  keeps `internal/explore` independently unit-testable (§34: cycle
  handling, branching, truncation, determinism can all be tested with
  tiny in-package fixtures, with zero `model`/`loader` scaffolding) and
  reusable, without-modification, by any future phase that needs bounded
  reachability over a different kind of small declared automaton (§38).
- **Distinct resource-bound namespace**: `internal/explore`'s bound
  (`maxStates`, a parameter) is deliberately generic and caller-supplied,
  keeping the package itself free of any DelegationProof-specific
  `limits` import — `verify_v6.go` is the one place that reads
  `limits.MaxExplorationStatesPerLifecycle` and passes it in, the same
  "policy lives in the caller, mechanism lives in the library" separation
  `internal/graph` already exhibits (it has no `limits` dependency
  either; `internal/loader` is the one place that reads `limits.MaxChainDepth`
  and compares `graph.LongestPath`'s return value against it).

---

## 31. Worked examples

### 31.1 Clean Phase 6 model (lifecycle declared, provably safe)

An approval record declaring a trivial single-reachable-state lifecycle —
explicit, but never leaving `"approved"`:

```json
{
  "approver": "compliance-officer",
  "scope": "billing:refund",
  "target": "billing-service",
  "lifecycle": {
    "initial": "approved",
    "states": ["approved"],
    "transitions": [
      { "from": "approved", "to": "approved", "event": "reapprove" }
    ]
  }
}
```

`Reach(L) = {"approved"}` (the self-loop revisits, but never leaves, the
one declared state) → `Safe(a) = true` → identical `ALLOW` outcome to the
equivalent Phase 5 document with no `lifecycle` declared at all.

### 31.2 Focused violation (revocation reachable)

```json
{
  "approver": "compliance-officer",
  "scope": "billing:refund",
  "target": "billing-service",
  "lifecycle": {
    "initial": "approved",
    "states": ["approved", "revoked"],
    "transitions": [
      { "from": "approved", "to": "revoked", "event": "revoke" }
    ]
  }
}
```

`Reach(L) = {"approved", "revoked"}` → `Safe(a) = false`, `unsafe_state =
"revoked"`, `lifecycle_trace = [approved -[revoke]-> revoked]` →
`approval_lifecycle_unsafe`, exactly §2's motivating scenario.

### 31.3 Bounded exploration case (larger, still within bounds, still fully explored)

A five-state resubmission workflow with a genuine cycle:

```
pending -[submit]-> approved -[revoke]-> revoked -[resubmit]-> pending
approved -[expire]-> expired
```

`states = ["pending", "approved", "revoked", "expired"]` (4, well within
`MaxLifecycleStates = 32`), `transitions` = 4 (well within
`MaxLifecycleTransitions = 128`). `initial = "pending"`. BFS visits all
four states (`Truncated = false`); `Reach(L) = {pending, approved,
revoked, expired}`, three of which are non-`"approved"` → unsafe
candidates `{expired, pending, revoked}`, canonical choice (ascending
lex) = `"expired"`, canonical path = `pending -[submit]-> approved
-[expire]-> expired` (BFS shortest path, 2 hops, not the 3-hop path
through `revoked`/`pending` again). Demonstrates a non-trivial but fully,
provably-completely explored automaton — no truncation, full correctness.

### 31.4 Multiple-history / canonical-trace case

Two distinct paths both reach `"revoked"`:

```
approved -[revoke]-> revoked
approved -[suspend]-> suspended -[void]-> revoked
```

BFS (FIFO, sorted-by-`(to,event)` expansion from `"approved"`) discovers
`"revoked"` directly via the single-hop `approved -[revoke]-> revoked`
edge before it would ever explore the two-hop `suspended` path — the
1-hop discovery wins by BFS's own shortest-path property, with no need
for an explicit tie-break rule beyond "BFS itself already guarantees the
first-discovered path is shortest." (§14.3's lexicographic tie-break only
matters when choosing *which unsafe state name* is canonical, when
several are reachable — not for choosing *which path* to a given state,
which BFS already resolves deterministically on its own.)

### 31.5 Interaction with Phase 5

A capability approved by two independent parties, one lifecycle-unsafe,
one with no lifecycle declared at all:

```json
"approvals": [
  { "approver": "compliance-officer", "scope": "billing:refund", "target": "billing-service",
    "lifecycle": { "initial": "approved", "states": ["approved","revoked"],
                    "transitions": [{"from":"approved","to":"revoked","event":"revoke"}] } },
  { "approver": "finance-lead", "scope": "billing:refund", "target": "billing-service" }
]
```

Both independently hold standing over `billing:refund@billing-service`.
`compliance-officer`'s record is lifecycle-unsafe; `finance-lead`'s record
has no `lifecycle` at all, hence trivially safe. Per §14.1's existential
narrowing: `finance-lead`'s record alone suffices → `ALLOW`, no finding —
directly demonstrating that Phase 6 narrows, but does not replace, Phase
5's existential quantifier.

### 31.6 Combined prior-invariant + Phase 6 violation

One document producing an `authority_amplification` (Phase 1),
a `delegation_depth_violation` (Phase 4), and an `approval_lifecycle_unsafe`
(Phase 6) finding simultaneously, on unrelated parts of the graph — no
masking between them, the same "all violation kinds legitimately coexist"
demonstration every prior phase's own combined-violations fixture
provides, extended by one more kind. (Full JSON deferred to the
implementation session's `testdata/valid-v6/combined-violations.json`,
per the same latitude Phase 4 §22/Phase 5 §22 left for exact fixture
construction.)

---

## 32. Complete worked example

`examples/billing-approval-lifecycle.json` (implementation-session file):

```json
{
  "version": "6",
  "principals": [
    {
      "id": "admin",
      "authority": [
        { "scope": "billing:refund", "target": "billing-service",
          "max_delegation_depth": 1, "requires_approval": true },
        { "scope": "billing:void",   "target": "billing-service",
          "max_delegation_depth": 1, "requires_approval": true }
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
    {
      "approver": "compliance-officer",
      "scope": "billing:refund",
      "target": "billing-service",
      "lifecycle": {
        "initial": "approved",
        "states": ["approved"],
        "transitions": []
      }
    },
    {
      "approver": "compliance-officer",
      "scope": "billing:void",
      "target": "billing-service",
      "lifecycle": {
        "initial": "approved",
        "states": ["approved", "revoked"],
        "transitions": [
          { "from": "approved", "to": "revoked", "event": "revoke" }
        ]
      }
    }
  ],
  "operations": [
    { "actor": "billing-agent", "requester": "admin", "action": "refund-safe",
      "requires": "billing:refund", "target": "billing-service" },
    { "actor": "billing-agent", "requester": "admin", "action": "void-unsafe",
      "requires": "billing:void", "target": "billing-service" }
  ]
}
```

**Expected `verify` behavior:**

- **`refund-safe`**: `billing-agent` validly holds `billing:refund@billing-service`
  (delegated within budget), `admin` has standing, the capability requires
  approval, `compliance-officer` independently holds standing and is
  declared as approver with a lifecycle whose only reachable state is
  `"approved"` (empty `transitions`, so `Reach(L) = {"approved"}`
  trivially) → every Phase 1-6 tier passes → **no finding**.
- **`void-unsafe`**: identical presence/binding/requester/standing
  reasoning, but `compliance-officer`'s `billing:void` approval record
  declares a lifecycle that can reach `"revoked"` → Phase 5's own checks
  (`approval_missing`/`approval_unauthorized`) do not fire (a standing-
  backed record does exist), but Phase 6's new tier does → **one
  `approval_lifecycle_unsafe` finding**: `unsafe_approver:
  "compliance-officer"`, `unsafe_state: "revoked"`, `lifecycle_trace:
  [approved -[revoke]-> revoked]`.
- **Result**: `DENY`, exit 1, exactly one finding.

This is the direct temporal analogue of `docs/phase-5-plan.md` §22's own
two-operation worked example (one pass, one `approval_missing`) — here,
one pass, one `approval_lifecycle_unsafe`, demonstrating that a
capability can be fully valid under every static Phase 1-5 check and
still fail Phase 6's new temporal check.

---

## 33. Testing plan / test matrix

Implementation-complete, numbered. Each row is a distinct test case
(unit test, table-driven case, or fixture) some future implementation
session must cover; exact test function names are an implementation-
session detail.

1. Clean pass — v6 document, no `lifecycle` declared anywhere → identical
   output shape to an equivalent v5 document (modulo version literal).
2. Clean pass — v6 document, `lifecycle` declared, single-state,
   `Reach = {"approved"}` → `ALLOW`.
3. Clean pass — v6 document, `lifecycle` with a self-loop on `"approved"`
   → `ALLOW`.
4. Unsafe — `Reach` includes exactly one non-`"approved"` state →
   `approval_lifecycle_unsafe`.
5. Unsafe — `Reach` includes multiple non-`"approved"` states, canonical
   selection picks the lexicographically smallest.
6. Unsafe — `initial` state itself is not `"approved"` (e.g.
   `initial: "pending"`, no path to `"approved"` at all) →
   `approval_lifecycle_unsafe`, `unsafe_state = "pending"` (trivially, the
   initial state itself).
7. Unreachable unsafe state — a declared state exists but is unreachable
   from `initial` → not included in `Reach`, does not affect the safety
   verdict.
8. Legal transitions — self-loop (`from == to`) accepted without error.
9. Legal transitions — a multi-state cycle (`a → b → c → a`) accepted,
   fully explored, `Truncated = false`.
10. Illegal transitions — `from` references an undeclared state →
    `unknown_lifecycle_state`.
11. Illegal transitions — `to` references an undeclared state →
    `unknown_lifecycle_state`.
12. Illegal — `initial` references an undeclared state →
    `unknown_lifecycle_state`.
13. Illegal — `initial` is the empty string →
    `unknown_lifecycle_state`.
14. Initial-state construction — `Explore`'s `Reachable` always contains
    `initial` itself even with zero transitions declared.
15. State canonicalization — two documents differing only in the
    declared order of `lifecycle.states` produce byte-identical output.
16. State canonicalization — two documents differing only in the
    declared order of `lifecycle.transitions` produce byte-identical
    output.
17. State dedupe — a state reachable via two different paths is visited
    (and included in `Reachable`) exactly once; `Explore`'s internal
    queue never re-enqueues an already-visited state.
18. Multiple histories — two distinct transition sequences both reach the
    same unsafe state; the canonical `lifecycle_trace` is the
    BFS-shortest of the two.
19. Canonical history choice — when two paths to the same unsafe state
    have equal length, the one whose next hop is lexicographically
    smaller (per the sorted-adjacency expansion rule) wins, deterministically.
20. Transition ordering — outgoing transitions from one state, declared
    in non-lexicographic input order, are still expanded in ascending
    `(to, event)` order (assert on `Path` content, not just final
    `Reachable`).
21. Exploration breadth/depth behavior — a wide automaton (many
    transitions from one state) and a deep one (a long simple chain) both
    fully explored within bounds, correct `Reachable`/`Path` in both
    shapes.
22. Exact state limit — a lifecycle declaring exactly `MaxLifecycleStates`
    states passes `validate`.
23. State-limit exhaustion — a lifecycle declaring
    `MaxLifecycleStates + 1` states fails `validate` with
    `resource_limit_exceeded`, `primary = "max_lifecycle_states"`.
24. Exact transition limit — a lifecycle declaring exactly
    `MaxLifecycleTransitions` transitions passes `validate`.
25. Transition-limit exhaustion — `MaxLifecycleTransitions + 1` fails
    `validate` with `resource_limit_exceeded`, `primary =
    "max_lifecycle_transitions"`.
26. Fail-closed incomplete search — white-box test lowering
    `limits.MaxExplorationStatesPerLifecycle` below a fixture's true
    reachable-state count: `Explore` reports `Truncated = true`;
    `RunV6` emits `approval_lifecycle_unproven`, never `ALLOW`.
27. Fail-closed incomplete search — an operation with *two* standing
    approval records, one truncated and one genuinely `Safe` → `Safe`
    wins (§26 step 4's `if len(safe) > 0` short-circuit fires before
    `unproven` is ever consulted) → `ALLOW`. (Confirms truncation of one
    record never poisons an otherwise-provable operation.)
28. Fail-closed incomplete search — an operation with two standing
    records, one truncated and one proven unsafe (neither safe) →
    `approval_lifecycle_unsafe` wins over `approval_lifecycle_unproven`
    per §16.6/§26's explicit precedence (definitive proof over
    inconclusive result).
29. Phase 1 interaction — actor never holds the capability at all →
    `authority_amplification`; lifecycle never evaluated (assert no
    `Explore` call / no lifecycle-related finding).
30. Phase 2 interaction — capability held, wrong target →
    `context_binding_violation`; lifecycle never evaluated.
31. Phase 3 interaction — actor holds, requester does not →
    `confused_deputy`; lifecycle never evaluated, and confirm the
    *approver's* lifecycle (as distinct from the requester's own
    standing) plays no role in this check at all.
32. Phase 4 interaction — a depth-exhausted edge produces
    `delegation_depth_violation` at the edge, and a consequent
    `authority_amplification` at the operation level (mirroring
    `docs/phase-4-plan.md` §22's own two-finding shape) with zero
    lifecycle involvement.
33. Phase 5 interaction — `requires_approval = false` → vacuously
    satisfied, no Phase 5 or Phase 6 finding, regardless of any declared
    lifecycle on an unrelated approval record for the same capability.
34. Phase 5 interaction — no declared approval record at all →
    `approval_missing`; lifecycle never evaluated.
35. Phase 5 interaction — declared record(s) exist, none standing →
    `approval_unauthorized`; lifecycle never evaluated for non-standing
    records.
36. Phase 5 interaction — one standing-backed, lifecycle-unsafe record,
    and one standing-backed record with no `lifecycle` declared → `ALLOW`
    (§31.5), confirming Phase 6 narrows rather than replaces Phase 5's
    existential.
37. Precedence — full six-way precedence table (§16.6) exercised end to
    end with one fixture per row.
38. Malformed input — `lifecycle` present with empty `states` array →
    `empty_lifecycle_states`.
39. Malformed input — duplicate name within `states` →
    `duplicate_lifecycle_state`.
40. Malformed input — duplicate exact `(from, event, to)` transition →
    `duplicate_lifecycle_transition`.
41. Malformed input — two transitions sharing `(from, to)` but different
    `event` labels are accepted (not a duplicate).
42. Unknown fields — a stray key inside a `lifecycle` object is rejected
    via `DisallowUnknownFields`.
43. Unknown fields — a stray `lifecycle` key on a root capability,
    delegation authority entry, or operation is rejected via
    `DisallowUnknownFields`.
44. Invalid references — `approvals[].approver` unknown (unchanged Phase
    5 check) still fires even when a `lifecycle` is also declared on the
    same malformed record.
45. Boundary values — a lifecycle with exactly one state (`initial` and
    the sole state coincide) and zero transitions.
46. Boundary values — a lifecycle with a state name at exactly
    `MaxTargetLength` (128) bytes.
47. Deterministic finding order — a document producing multiple
    `LifecycleFinding`s sorts identically regardless of `operations[]`/
    `approvals[]` declaration order.
48. Deterministic state order — `Explore`'s `Result.Reachable` content
    (not iteration order — Go maps have none by contract) is asserted via
    sorted-key comparison in tests, never via raw map equality dependent
    on internal representation.
49. Permutation invariance — reordering `principals`/`agents`/
    `delegations`/`operations`/`approvals` arrays, and reordering
    `lifecycle.states`/`lifecycle.transitions` within one approval record,
    never changes output (extends the existing
    `TestJSONFormatInputArrayPermutationInvariance*` family).
50. Repeated-run byte identity — running `verify` twice over the same v6
    document produces byte-identical stdout both times (extends
    `TestJSONFormatDeterministicAcrossRepeatedRuns*`).
51. Text golden — captured `--format text` output for the worked example
    (§32) and at least one `approval_lifecycle_unproven` fixture.
52. JSON golden — captured `--format json` output for the same fixtures.
53. CLI behavior — `validate` never evaluates any invariant (including
    Phase 6's) for a structurally valid v6 document; only `verify` does.
54. stdout/stderr — findings/results on stdout only; a v6 `LoadError` on
    stderr only, exit 2.
55. Exit codes — `ALLOW` → 0, any Phase 6 finding → 1, any Phase 6
    structural error → 2, CLI usage error → 3, all for v6 input
    specifically.
56. Resource bounds — `MaxApprovals` (unchanged, 10,000) still enforced
    for v6 documents.
57. Hostile input — a `lifecycle` object whose `transitions` array
    declares a dense, near-complete graph up to `MaxLifecycleTransitions`
    still completes `Explore` well within the runtime ceiling, no
    timeout, no panic.
58. No-panic/fuzz — truncated/mutated v6 JSON input (missing `lifecycle`
    sub-fields, wrong JSON types for `states`/`transitions`, deeply
    nested garbage) never panics `main`, for both `validate` and
    `verify`.
59. Versions 1-5 regression — every existing `testdata/golden/` fixture,
    every existing malformed fixture, and the full existing test suite
    pass unmodified, byte-identical, after Phase 6 lands (§35).
60. `internal/explore` unit tests, standalone (no `model`/`loader`
    dependency): empty transitions, single self-loop, simple two-state
    chain, branching (two outgoing edges from one state), multi-state
    cycle, truncation via a tiny `maxStates` on a fixture BFS would
    otherwise fully explore, and determinism (same input run twice
    produces identical `Result`, including `Path` map contents compared
    via sorted iteration).
61. Cyclic lifecycle passes `validate` with zero errors — confirms
    behaviorally, not just by absence of a check in the spec, that no
    acyclicity/cycle-detection error of any kind is ever raised for a
    lifecycle automaton (§8, §20), distinguishing it from the delegation
    graph's own mandatory cycle rejection.
62. Reapproval cycle (`revoked → pending → approved`, reachable from an
    `initial` of `"revoked"` or similar) still correctly yields
    `approval_lifecycle_unsafe` — confirms a declared recovery path back
    to `"approved"` does not retroactively make an already-unsafe
    automaton safe (§8's monotonicity note, §9).
63. Validation-diagnostic determinism — a document with two or more
    simultaneous v6 structural errors (e.g. two independent
    `unknown_lifecycle_state` violations in different approval records)
    produces identical, sorted `ValidationError` order across repeated
    runs and across array-reordered input, via the unmodified existing
    `sortErrors` mechanism (§20).

**Explicitly not modeled, and therefore explicitly not tested**:
"revocation before use" / "use before revocation" style scenarios that
presuppose a real-time ordering between a specific operation and a
specific lifecycle transition. §4 Candidate C and §10 establish directly
that operations are never coupled to lifecycle timing in this design —
an operation's outcome is provably independent of any hypothetical
execution order relative to a lifecycle's transitions, which is already
exercised indirectly by permutation invariance (test 49: reordering
`operations[]` relative to `approvals[]` never changes output). Adding a
dedicated test asserting "operation before/after revocation" would imply
the design tracks such an ordering, which it deliberately does not; no
such test is added, per the audit's own instruction not to inflate the
matrix artificially.

---

## 34. Phase 1-5 regression requirements

- `go build ./...` succeeds; `go.mod` remains stdlib-only — `internal/explore`
  introduces no dependency, only `container/heap`/`sort`-class stdlib
  usage if any (plain BFS needs neither a heap nor anything beyond a
  slice-based FIFO queue and `sort.Strings`/`sort.Slice`).
- `go vet ./...` clean; `gofmt -l .` reports nothing.
- `go test ./... -race -count=1` passes, including every category in
  §33, with the one sanctioned test-string change (§6) and zero other
  modification to any pre-existing test file.
- Every existing `testdata/golden/` file is unchanged, byte-identical.
- A version-1 through version-5 document produces byte-identical
  `validate`/`verify` output, on both `text` and `json` formats, to the
  current `main` branch today.
- A version-6 document with no `lifecycle` field declared anywhere in its
  `approvals[]` array produces output identical in every finding-content
  respect to the equivalent version-5 document (§16.5).

---

## 35. Security assumptions and limitations

Extending `README.md`'s "Security assumptions" section and every prior
phase's own limitations disclosure, unmodified where unchanged:

- DelegationProof remains a **static, offline analyzer**. A version-6
  `lifecycle` declaration is, exactly like a version-5 `approvals[]`
  entry, **a declared fact by the document's author, not an observed
  real-world event log** — DelegationProof verifies that a declared
  automaton *cannot* reach an unsafe state, given its own declared
  transitions; it does not verify that those declared transitions
  correspond to any real compliance system's actual behavior, that a
  real revocation event ever fires when the document claims it can, or
  that the document is honest about which transitions are actually
  possible in the real approval workflow it claims to model. This is the
  same "topology-ingestion is a separate concern" boundary Phase 3
  draws around `requester` and Phase 4 draws around `max_delegation_depth`,
  extended to lifecycle: **Phase 6 proves properties of the declared
  automaton, never of the real system the automaton claims to describe.**
- **DelegationProof still cannot observe or reason about *when*, in real
  time, an operation actually executes relative to a lifecycle
  transition.** This is precisely why the safety predicate is universal
  ("every reachable state must be safe") rather than an attempt to prove
  "the operation happens to run during the approved window" — the latter
  is unknowable offline, and Phase 6 does not pretend otherwise (§4
  Candidate C, §13).
- **A lifecycle-safe result is not a guarantee that a real approval was
  never actually revoked** — it is a guarantee that the *document*, as
  declared, gives DelegationProof no way to conclude a revocation (or any
  other non-approved state) was ever a *legally reachable* outcome under
  its own stated transition policy. If the real world's actual approval
  system permits a transition the document simply forgot to declare,
  Phase 6 cannot catch that — exactly as a Phase 4 `max_delegation_depth`
  declaration is a policy assertion, not a verified fact about a real
  system's actual re-delegation history (`README.md`'s existing framing,
  reused verbatim for lifecycle).
- **Self-approval remains unprohibited** (unchanged from Phase 5, §37) —
  lifecycle safety and self-approval are orthogonal concerns; a
  self-approved, lifecycle-safe record is exactly as valid under Phase 6
  as any other lifecycle-safe record.
- **Quorum/N-of-M approval remains unmodeled** (unchanged from Phase 5,
  §37) — a single lifecycle-safe standing-backed approval remains
  sufficient, exactly as a single standing-backed approval was sufficient
  under Phase 5.
- Combined with the resource bounds in §22/§30, it remains safe to run
  Phase 6's exploration against untrusted model files without additional
  sandboxing — every new bound is validate-time enforced or, as
  defense-in-depth, runtime-enforced with a fail-closed outcome (§22),
  never a source of unbounded work or a panic.

---

## 36. Explicit non-goals

All of `docs/phase-1-plan.md` §18's, `docs/phase-2-plan.md` §23's,
`docs/phase-3-plan.md` §25's, `docs/phase-4-plan.md` §27's, and
`docs/phase-5-plan.md` §28's non-goals continue to apply. Phase 6
additionally, explicitly, does **not** include:

- **Networking, hosted services, runtime enforcement/proxying, real
  identity verification, OAuth implementation, MCP/A2A runtime, LLM
  calls, databases, distributed services, a web UI, an external policy
  engine, an arbitrary policy language.** Unchanged from every prior
  phase; lifecycle exploration is a pure, in-memory, offline computation
  over declared input, exactly like every other invariant in the project.
- **Unlimited/general model checking.** Phase 6 is deliberately the
  narrowest possible instance of state-space exploration that satisfies
  §3's trigger condition: one small, independently-explored automaton per
  approval record, no shared/global state, no guarded transitions, no
  temporal-logic formula language, no CTL/LTL, no user-supplied
  properties to check beyond the one fixed, built-in safety predicate
  (§9). A document author cannot express or ask DelegationProof to check
  any property other than "is this specific approval record's own
  declared lifecycle always approved."
- **Symbolic execution.** Not introduced and not needed: every lifecycle
  is a small, concrete, fully-enumerable finite automaton with no
  symbolic/unbounded-domain variables of any kind — plain explicit-state
  BFS is both necessary and sufficient (§21), so symbolic techniques would
  add complexity with no corresponding capability gain.
- **SAT/SMT solving.** Explicitly deferred, per every prior phase's own
  posture (`docs/phase-1-plan.md` §21: "only relevant if/when a later
  invariant needs constraint solving beyond set inclusion"). Phase 6's
  entire check is graph reachability over a tiny, explicit, boolean-free
  state space — there are no numeric/interval constraints, no boolean
  satisfiability question, and therefore no motivation for a solver of
  any kind.
- **Multi-approver quorum/threshold requirements** (`docs/phase-5-plan.md`
  §31). Unchanged, unaccelerated, unblocked by anything in Phase 6 — the
  existential quantifier from Phase 5 is narrowed (§14.1) but not
  replaced by a counting quantifier.
- **Approval-gated delegation** (`docs/phase-5-plan.md` §31). Unchanged —
  lifecycle, like approval itself, gates exercise only, never
  transmission (§15).
- **Self-approval / separation-of-duties enforcement**
  (`docs/phase-5-plan.md` §31). Unchanged, unaddressed by Phase 6.
- **Cross-approval / global lifecycle composition** (the rejected
  Candidate D of §4). Explicitly and permanently out of scope for the
  reason §3/§30.1 give: it is the one design that would reintroduce
  genuine exponential blow-up, and nothing in the current threat model
  motivates it.
- **A real event log, session concept, or "current time."** Rejected at
  §4 Candidate B/C and not reconsidered — the whole design is phrased in
  terms of provable reachability, not simulated real-time state.
- **Explicit per-edge depth attenuation, multi-hop request/induced-by
  chains, scope/target wildcard or hierarchy semantics, real-world
  approval-workflow correspondence, real-world redelegation-count
  correspondence** — unchanged from Phase 3/4/5's own deferrals; nothing
  in Phase 6 accelerates or blocks any of them.
- **MCP/A2A ingestion, JSON Schema enforcement, SARIF, CI-vendor
  integration, automatic policy generation.** Unchanged from Phase 1 §21;
  nothing in Phase 6 accelerates or blocks any of them.
- Phase 7+ implementation of any kind.

---

## 37. Future-phase boundary

Carried forward from `docs/phase-1-plan.md` §21 through `docs/phase-5-plan.md`
§31, now with Phase 6's own addition noted:

- **Multi-approver quorum/threshold requirements**: if a later phase
  demonstrates a need for "N independent, lifecycle-safe, standing-backed
  approvals required," that generalizes §16.6/§26's existential check to
  a counting check over the same `safe` set already computed — additive,
  not a redesign, exactly as `docs/phase-5-plan.md` §31 already predicted
  for the pre-Phase-6 version of this same extension point.
- **Approval-gated delegation**: unchanged from `docs/phase-5-plan.md`
  §31 — if a later phase requires approval (now potentially including
  lifecycle-safety) before re-delegation, not just exercise, that is a
  new edge-level tier layered onto Phase 4's existing edge precedence,
  symmetric with how depth itself was added.
- **Self-approval / separation-of-duties enforcement**: unchanged from
  `docs/phase-5-plan.md` §31 — could be added as an additional condition
  in §16.6 step without altering how standing or lifecycle safety are
  computed.
- **Cross-record lifecycle correlation**: if a later, concretely
  motivated threat demonstrates that two different approvals' lifecycles
  genuinely need to be reasoned about *jointly* (e.g. "these two sign-offs
  must both still be active simultaneously"), that is new scope requiring
  the composed/global state model this document explicitly rejects (§3,
  §36) — it would need its own resource-bound analysis given the
  combinatorial cost such composition reintroduces, and should not be
  assumed cheap merely because per-record exploration is.
- **A richer event/trigger model for lifecycle transitions** (e.g. a
  transition guarded by a declared precondition referencing another part
  of the document): would be new scope, layered onto — not replacing —
  the plain unconditional transition system defined here (§11).
- **Explicit per-edge depth attenuation, multi-hop request/induced-by
  chains, scope/target wildcard or hierarchy semantics, MCP/A2A
  ingestion, JSON Schema enforcement, SARIF, Z3/SMT**: unchanged from
  Phase 1/3/4's own deferrals; nothing in Phase 6 accelerates or blocks
  any of them.
- **Real-world lifecycle-correspondence verification** (confirming a
  document's declared `lifecycle` actually matches a real approval
  system's real transition policy): a topology-ingestion concern,
  symmetric with every prior phase's identical boundary around
  `requester`, `max_delegation_depth`, and `approvals[]` (§35). This
  phase defines what to check once a lifecycle is declared; it does not
  address how a real system's declarations get produced truthfully.

---

## 38. Acceptance criteria

1. `go build ./...` succeeds; `go.mod` remains stdlib-only.
2. `go vet ./...` is clean; `gofmt -l .` reports nothing.
3. `go test ./... -race -count=1` passes, including every category in
   §33, with the one sanctioned test-string change (§6) and zero other
   modification to any pre-existing test file.
4. Every existing `testdata/golden/` file is unchanged, byte-identical.
5. A version-1 through version-5 document produces byte-identical
   `validate`/`verify` output, on both `text` and `json` formats, to the
   current `main` branch today.
6. A version-6 document with no `lifecycle` field anywhere → `ALLOW` or
   `DENY` identical in finding content to the equivalent version-5
   document.
7. `examples/billing-approval-lifecycle.json` → exactly the two-operation
   shape described in §32 (one pass, one `approval_lifecycle_unsafe`),
   matching the worked example.
8. A version-6 document containing all eight violation kinds
   simultaneously reports all eight, correctly classified, correctly
   ordered, with no duplicate finding for any single edge or operation.
9. `unknown_lifecycle_state`, `duplicate_lifecycle_state`,
   `duplicate_lifecycle_transition`, and `empty_lifecycle_states` each
   have at least one dedicated malformed fixture and table-driven test
   case.
10. `MaxLifecycleStates`/`MaxLifecycleTransitions`/
    `MaxExplorationStatesPerLifecycle` are each exercised at their exact
    boundary and one-past-boundary value in white-box tests.
11. The fail-closed `approval_lifecycle_unproven` path is reachable and
    tested only via a lowered `limits.MaxExplorationStatesPerLifecycle`
    (never via a validate-time-legal, un-modified-limits document) and is
    confirmed to never resolve to `ALLOW`.
12. `internal/explore` has standalone unit tests requiring no
    `model`/`loader` import, covering cycles, branching, self-loops,
    determinism, and truncation.
13. No panic is reachable from `main` for any version-1 through
    version-6 input within the documented resource bounds.
14. Permutation invariance and repeated-run byte identity hold for
    version-6 input, including reordering `lifecycle.states`/
    `lifecycle.transitions`.

---

## 39. Definition of DONE

Phase 6 is done when:

1. All items in §38 are met.
2. The file/package layout matches §30.2, or a documented deviation is
   noted in this document, keeping it authoritative per every prior
   phase's own convention.
3. Every new error kind (§20) and every new finding `violation`/`point`
   combination (§18) has at least one dedicated test.
4. The worked example (§32) is reproducible verbatim via
   `delegationproof verify examples/billing-approval-lifecycle.json`.
5. `schemas/model.md` has been updated (or a sibling v6 document added)
   by the implementation session to describe the version-6 shape — noted
   as deferred in §30.2, not done in this planning session, per explicit
   instruction not to modify it now.
6. No open TODOs remain in code for functionality this document describes
   as in-scope; TODOs for §37's deferred items are fine and expected,
   linking back to §37.
7. `docs/phase-1-plan.md` through `docs/phase-5-plan.md` are unmodified —
   Phase 6 attaches to all five, per their own future-phase-boundary
   sections, without rewriting any of them.

---

## 40. Implementation sequence

Recommended order for the future Phase 6 implementation session,
following the same bottom-up dependency order every prior phase's own
architecture plan implies (types before loader, loader before verify,
verify before report/CLI wiring, and — new for Phase 6 — the generic
exploration primitive before the verifier that consumes it):

1. `internal/explore/explore.go` + `explore_test.go` — `Transition`,
   `Result`, `Explore` (§21), fully unit-tested standalone before
   anything else depends on it.
2. `internal/model/types_v6.go` — `Lifecycle`, `LifecycleTransition`,
   `ApprovalV6`, `ModelV6`, `PrincipalV6`, `AgentV6`, `DelegationV6`,
   `OperationV6` (§6).
3. `internal/limits/limits.go` — add `MaxLifecycleStates`,
   `MaxLifecycleTransitions`, `MaxExplorationStatesPerLifecycle` (§22).
4. `internal/loader/loader_v6.go` — `decodeAndValidateV6`, `validateV6`,
   `checkApprovalsV6`, `checkLifecycle`, new `ErrorKind` constants (§20,
   §30.2); update `LoadDocument`'s switch and `Document` struct in
   `loader_v2.go`; make the five sanctioned message-text touches (§6).
5. `internal/report/lifecycle_finding.go` — `LifecycleFinding`,
   `LifecycleStep`, `ViolationApprovalLifecycleUnsafe/Unproven`,
   `NewLifecycleFinding` constructor (§18); extend `finding.go`'s `keyOf`
   switch with one new case.
6. `internal/verify/verify_v6.go` — `RunV6`, outcome cache/`classify`
   implementing §16.6/§26, reusing `verify_v5.go`'s `authState`/
   `flattenApproval` verbatim.
7. `internal/report/text.go` — extend `RenderText`'s type switch (§28).
8. `cmd/delegationproof/main.go` — extend `runVerify`'s dispatch switch
   (§27).
9. `examples/billing-approval-lifecycle.json` — the worked example
   (§32).
10. `testdata/malformed/` — one fixture per new §20 error kind, plus the
    decode-level and stray-field cases.
11. `testdata/valid-v6/` — clean-pass, reordered-arrays, unsafe-lifecycle,
    unproven-lifecycle (white-box, lowered limits), multi-approver,
    combined-violations fixtures (§31, §33).
12. `testdata/golden/` — captured text/json output for the worked example
    and combined-violations fixture, generated from the built binary and
    diffed for intent, per CLAUDE.md's own instruction for golden-file
    changes.
13. `internal/loader/loader_v2_test.go` — the one sanctioned test-string
    update.
14. Full test suite per §33's matrix, across `explore_test.go`,
    `loader_v6_test.go`, `verify_v6_test.go`, `report`'s existing test
    files, and `main_v6_test.go`.
15. `schemas/model.md` — add the version-6 section (§39 item 5; deferred
    per this planning session's own instruction not to touch it now, but
    sequenced last in the implementation session for completeness).
16. Final verification: `gofmt -l .`, `go vet ./...`, `go test ./...
    -race -count=1`, `go build -o bin/delegationproof
    ./cmd/delegationproof` — all per CLAUDE.md's standing requirement,
    confirming every item in §38's acceptance criteria.

---

## 41. Final design audit

Performed against this document itself before treating it as complete,
per the task's own explicit audit checklist:

1. **Reread against `docs/phase-1-plan.md` through `docs/phase-5-plan.md`**:
   every cited section number/quote in §0, §1, §3 was re-verified against
   the actual file contents read for this planning session (not assumed
   from memory of the prompt) — see §1's "verified against the actual
   merged implementation" framing and the direct quotations throughout
   §0/§3.
2. **No semantic contradictions**: Phase 6 never redefines a Phase 1-5
   term (`DA(n)`, "standing," "presence," "binding," "depth") — every
   Phase 6 concept (`Q`, `q₀`, `δ`, `Safe`) is new vocabulary, introduced
   once (§9-§13), reused consistently thereafter.
3. **Phase 6 is the correct next roadmap capability**: confirmed by
   process of elimination in §0/§3 — it is the only named, deferred item
   (`docs/phase-5-plan.md` §31) that structurally requires anything beyond
   an additive extension of the existing single-pass algorithm; every
   other deferred item is explicitly documented, in the source material
   itself, as "additive, not a redesign."
4. **Previous semantics unchanged**: §16.1-§16.5 individually confirm
   zero change to each of Phases 1-5's own algorithms; §34 makes this a
   testable acceptance criterion (byte-identical output for v1-v5 input,
   and for v6 input with no `lifecycle` declared).
5. **Every state dimension is necessary**: §10 explicitly enumerates and
   rejects every additional dimension considered (global state, operation
   state, edge state, a clock) with a reason tied back to §3/§4's
   threat-model analysis — exactly one dimension (`LifecycleState`)
   survives.
6. **All transitions precisely defined**: §11's table leaves no
   implementation-time ambiguity — precondition, mutation, failure
   condition, ordering, and observability are all specified, and the
   system is a plain unconditional labeled digraph specifically so there
   is nothing further to specify.
7. **Exploration is deterministic**: §25 traces every determinism
   requirement to a specific mechanism (sorted adjacency, FIFO queue,
   explicit lexicographic tie-breaks), with an explicit "no algorithm
   relies on Go map iteration order" claim, checked against §21's
   pseudocode line by line.
8. **State equivalence is sound**: §23 justifies exact-string-equality as
   sound specifically because lifecycle states have no substructure
   beyond their own outgoing-transition set, which is exactly what
   `Reach` already computes over — no coarser equivalence could be sound
   without information the automaton does not carry.
9. **All potentially explosive dimensions have hard bounds**: §22's table
   plus §30.1's per-dimension resource-safety table cover states,
   transitions, total approvals, and runtime visited-state count; §3/§26
   explicitly identify and reject the one design (global composition)
   that would have been genuinely unbounded/exponential.
10. **Exhaustion fails closed**: §22's dedicated subsection is the
    literal, security-critical specification the task demands — no
    silent `ALLOW`, a distinct machine-readable violation kind, explicit
    text/JSON/exit-code contract.
11. **No algorithm relies on Go map iteration**: confirmed in §21 (sorted
    adjacency), §25 (explicit audit statement), and §26 (canonical
    selection via `sort.Strings`, never map range).
12. **Versions 1-5 remain backward compatible**: §16.5, §34, and
    acceptance criteria §38 items 4-6 all state and test this
    independently, at three different levels (algorithm unchanged, golden
    files unchanged, v6-with-no-lifecycle output identical to v5).
13. **Non-goals have not leaked into implementation scope**: §36 is
    exhaustive and cross-references every rejected candidate from §4;
    no section elsewhere in this document introduces quorum, approval-
    gated delegation, cross-record composition, a clock, or general model
    checking — every one of those is named exactly once, as explicitly
    out of scope.
14. **A future implementation agent can implement the entire phase from
    this plan without inventing security semantics**: every new field
    (§6, §8), every new type (§6, §18, §21), every algorithm (§21, §26),
    every bound and its exact value with rationale (§22), every finding
    shape and its exact reason-text template (§18), and a numbered
    implementation sequence with explicit file-by-file dependency order
    (§40) are all specified concretely, mirroring the level of detail
    `docs/phase-4-plan.md`/`docs/phase-5-plan.md` themselves provide.
15. **No fabricated temporal/conditional structure**: §3's closing
    paragraph states directly that the lifecycle automaton is optional,
    additive, and inert when undeclared — Phase 6 does not impose
    temporal structure on any document that does not explicitly opt into
    it, and the entire design is derived from an explicitly pre-existing,
    named roadmap item (`docs/phase-5-plan.md` §31's first bullet) rather
    than invented for this planning session.

This first pass found several real gaps — an internal grammar
contradiction on the `event` field, an asserted-but-not-proved
exhaustion-impossibility claim, an unexamined "is this feature
self-defeating/is there a smaller invariant" objection, and a missing
explicit soundness argument for state equivalence — all corrected by a
subsequent adversarial audit pass (§42). No contradiction, scope leak, or
unresolved ambiguity survives that pass.

---

## 42. Final GO / NO-GO decision

**Decision: GO.**

Phase 6, as corrected by this audit pass, is a coherent, minimal,
bounded, deterministic, fail-closed, and genuinely differentiated
security capability, and should be implemented.

**Strongest justification.** Every other named item in the project's own
five-document roadmap (quorum, approval-gated delegation, self-approval
prohibition) is explicitly documented, in the source material itself, as
an additive extension of the existing single-pass algorithm — none of
them requires anything Phase 6 introduces. Temporal approval lifecycle is
the *only* deferred item that structurally cannot be built without
genuine reachability search, and the project has named exactly this
trigger condition — "genuine nondeterminism or temporal/conditional
structure" — as the sole prerequisite for state-space exploration since
its very first planning document (`docs/phase-1-plan.md` §8.2). Phase 5
explicitly built the attachment point in advance (`docs/phase-5-plan.md`
§31: "the first genuinely motivated candidate for that framing across all
five phases so far") and specified exactly how it must compose (layer
onto, never replace, the static standing check). Phase 6 does exactly
that, and no more: one optional field, one new bounded/generic package
with zero DelegationProof-specific dependencies, one new operation-level
precedence tier reached only after all five prior tiers already pass, and
a total algorithmic cost that is *linear* in the number of declared
approvals (not exponential — that shape was identified and explicitly
rejected, §3/§26) specifically because the one architecturally consequential
decision (explore each lifecycle independently, never compose a global
cross-product state) was made deliberately and is enforced by the design,
not merely hoped for.

**Why this clears the bar the audit set, item by item:**

- **Coherent**: the roadmap chain from Phase 1 §8.2 → Phase 5 §31 → this
  document is unbroken and quotable at every link (§0, §3).
- **Minimal**: the "simpler `revocable: bool`" alternative was checked
  explicitly and shown to already be the trivial two-state instance of
  this exact design (§5.2) — there is no smaller invariant being skipped,
  and no larger one (quorum, cross-record composition, real-time
  coupling) being smuggled in (§36).
- **Implementable**: every ambiguity this audit surfaced (`event`
  grammar, case-sensitivity of `"approved"`, `lifecycle_trace` when the
  unsafe state is the initial state, the map-range-then-sort pattern,
  `DeclaredApprovers`'s invariant content) has been closed with a
  specific, unambiguous rule — no security-relevant judgment call is left
  to the implementation session.
- **Secure**: the safety predicate is universal, not existential
  (§13's boxed statement), state equivalence introduces no soundness gap
  because there is no field to hide an unsafe history behind (§23),
  exhaustion is proved — not merely asserted — impossible for any
  validate-time-legal document while the fail-closed path is still fully
  specified, tested, and never resolves to `ALLOW` (§22.1).
- **Deterministic**: every observable quantity — transition expansion
  order, queue order, trace choice, finding order, and now (closed by
  this audit) validation-diagnostic order — is traced to a specific,
  reused mechanism, with the one map-range in the design explicitly
  shown safe by an immediate sort (§25).
- **Bounded**: every resource dimension the audit asked about (states,
  transitions, total lifecycle-bearing approvals, runtime visited-state
  count, and — closed by this audit — exploration depth, which is proved
  subsumed by the state-count bound rather than needing its own variable,
  §22.1) has an explicit, arithmetically-checked bound (`128 ≤ 1024`,
  §22), not a round number chosen by feel.
- **Worth building**: the cost is genuinely small — one new package with
  no dependencies, one optional field, zero changes to any Phase 1-5
  production file — against a real, previously-inexpressible security gap
  (TOCTOU on approvals, named as a concrete example in the project's own
  design vocabulary since Phase 1) in exactly the class of system
  (human-in-the-loop-gated agentic tool calls) the project already exists
  to analyze.

No basis was found for a NO-GO verdict: nothing in this design turns
DelegationProof into a general-purpose model checker, nothing reopens or
weakens Phases 1-5, and nothing here is disproportionate to the security
value it delivers. `docs/phase-6-plan.md`, as corrected by this audit,
is the final implementation contract.
