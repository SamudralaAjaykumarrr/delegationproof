# DelegationProof — Phase 2 Plan

Status: PLANNING ONLY. Phase 1 is implemented, merged, and untouched by this
document. This is the authoritative design contract for the Phase 2
implementation session. It should be implementable without further product
redesign, in the same spirit as `docs/phase-1-plan.md`.

Module: `github.com/SamudralaAjaykumarrr/delegationproof`

---

## 0. Phase 2 rationale

Phase 1 proved one formal invariant — Authority Non-Amplification — over a
static delegation graph where authority is an opaque, exact-match scope
string. It deliberately excluded any notion of *where* a scope is valid:
`billing:read` in Phase 1 is a single fact, true or false, for a given node,
with no concept of "valid against billing-service but not payroll-service."

That is a real gap. A scope granted for one purpose can be silently
re-purposed against a different downstream system with zero Phase 1
findings, because Phase 1's model has no vocabulary to even state that this
happened. `docs/phase-1-plan.md` §21 anticipated this exact gap under the
name "audience/resource binding" and explicitly reserved it as the next
invariant to attach to the same foundation, without redesigning it.

Phase 2's job is to add the smallest rigorous concept that lets
DelegationProof distinguish "agent-b has `billing:read`" from "agent-b has
`billing:read`, valid for `billing-service`, and is attempting to use it
against `payroll-service`." Everything else — approvals, depth policy,
confused-deputy detection, MCP/A2A ingestion, state-space exploration —
remains future work (§26), exactly as Phase 1 §21 laid out.

---

## 1. Phase 1 baseline

Verified against the actual merged implementation (branch `main`,
commit `2af312d`), not just the plan document:

- **Model** (`internal/model/types.go`): `Model{Version, Principals,
  Agents, Delegations, Operations}`. `Principal{ID, Authority []string}`.
  `Agent{ID}` — no authority field, enforced absent by
  `DisallowUnknownFields`. `Delegation{Delegator, Delegatee, Authority
  []string}`. `Operation{Actor, Action, Requires string}` — `Requires` is
  a single scope, deliberately not an array.
- **Limits** (`internal/limits/limits.go`): exported `var`s —
  `MaxInputFileSize`, `MaxNodes`, `MaxDelegationEdges`, `MaxOperations`,
  `MaxScopeLength`, `MaxIDLength`, `MaxAuthoritySetSize`, `MaxChainDepth`.
- **Loader** (`internal/loader/loader.go`): reads the file, strict JSON
  decode (`DisallowUnknownFields`), then `validate(*model.Model)
  []ValidationError` — exhaustive, not fail-fast, sorted by
  `(Kind, Primary, Secondary)`. Cycle detection and chain-depth bound both
  ride on `graph.TopoSort`/`graph.LongestPath`.
- **Graph** (`internal/graph/graph.go`): `TopoSort` (Kahn's algorithm,
  lexicographically-tie-broken via a min-heap), `LongestPath` (DAG DP over
  topological order), `CanonicalTrace` (BFS from all principals, sorted
  expansion, first-path-wins).
- **Verify** (`internal/verify/verify.go`): `Run(*model.Model)
  report.Result`. One topological pass computes `da map[string][]string`;
  an incoming edge contributes only if `isSubset(e.Authority,
  da[e.Delegator])` — strict distrust, all-or-nothing, no partial credit.
  Operations are then checked against `da[op.Actor]`. `validEdges` (only
  edges that passed the subset check) is threaded into
  `graph.CanonicalTrace` for finding traces.
- **Report** (`internal/report/`): `EdgeFinding` and `OperationFinding`
  structs, both carrying `Violation` (always
  `"authority_amplification"`), `Point` (`"delegation_edge"` or
  `"operation"`), a deterministic `Reason` sentence, and a total sort
  order `(point, subject_id, secondary_id_or_action, scope)` via
  `report.Sort`. `RenderText`/`RenderJSON` are pure functions of an
  already-sorted `Result`.
- **CLI** (`cmd/delegationproof/main.go`): `validate <model.json>` and
  `verify <model.json> [--format text|json]` only. Exit codes 0/1/2/3 via
  `internal/exitcode`. stdout carries only result output; stderr carries
  only diagnostics.
- **Tests**: `testdata/malformed/` (one fixture per §7.4 error kind, table
  in `loader_test.go`, walked again by `cmd/delegationproof/main_test.go`
  for exit-code coverage), `testdata/golden/` (byte-exact text/json
  output for `clean-pass`, `billing-refund`, `mixed-violations`),
  `testdata/valid/mixed-violations-reordered.json` backing the
  input-array permutation-invariance test.

Phase 2 must not modify any of the above. It is additive.

---

## 2. Security problem

Phase 1 answers: *does agent-b have `billing:read`?*

It cannot answer: *is agent-b's `billing:read` being exercised in the
context it was delegated for?*

Concretely, Phase 1 has no way to express that a principal's
`billing:read` was delegated down a chain **for use against
billing-service**, and therefore has no way to flag an agent that holds a
perfectly legitimate `billing:read` grant but exercises it against
`payroll-service` instead. The scope string is identical in both cases;
Phase 1's exact-match-on-scope-alone model is structurally blind to the
difference. This is a distinct failure mode from amplification (having a
scope nobody ever gave you) — it is having a scope that *was* validly
given, for a context that does not cover the attempted use.

---

## 3. New terminology

| Term | Meaning in Phase 2 |
|---|---|
| **Target** | An opaque, exact-match identifier for the destination context a capability is valid against (e.g. `"billing-service"`). Not a graph node, not validated against any registry (§5). |
| **Capability** | The Phase 2 authority unit: an ordered pair `(scope, target)`. Replaces the bare scope string as the thing a principal declares, a delegation edge grants, and an operation requires, in version-2 documents only. |
| **Capability set** | A flat, deduplicated set of capabilities — the Phase 2 analogue of Phase 1's `set<string>` authority. |
| **Derived Authority, `DA(n)`** | For a version-2 model, generalized from Phase 1: the set of *capabilities* `n` validly holds, computed by the same topological algorithm (§11) over tuples instead of bare strings. For a version-1 model, `DA(n)` is unchanged from Phase 1 (§6.1 of `docs/phase-1-plan.md`) — a set of scope strings. |
| **Context-Binding Preservation** | The new Phase 2 invariant (§7): a capability may only be exercised or transmitted for the target it was delegated with. |
| **`authority_amplification`** | Unchanged violation literal (§8): the scope itself was never validly granted, for any target. |
| **`context_binding_violation`** | New violation literal (§7): the scope was validly granted, but only for a different target than the one exercised. |

No new node kind, no new edge kind, no new graph entity. `Target` is a
label carried on capability tuples, nothing more.

---

## 4. Authority representation

Candidates evaluated, in the same spirit as `docs/phase-1-plan.md` §4:

| Representation | Verdict | Why |
|---|---|---|
| Structured capability objects with resource/action/constraints/audience | Rejected | This is the exact policy-language failure mode Phase 1 §4 already rejected. Nothing about the context-binding problem requires constraint narrowing or a containment algebra — it requires exactly one more atomic, exact-match dimension. |
| Hierarchical/namespaced target strings with wildcard matching (`billing-*`) | Rejected | Explicitly out of scope (see Strict Non-Goals). Requires a containment grammar Phase 1 §4 already deferred for scope hierarchy, and this task's instructions bar it outright. |
| One `target` field on the **delegation edge / principal / operation itself**, applying to *all* scopes in that entity | Rejected | Forces one target per node/edge, which cannot express a principal or delegation legitimately holding capabilities for more than one target (a real, unremarkable case — e.g. a principal with both `billing:read@billing-service` and `payroll:read@payroll-service`). Would require either duplicating edges per target or inventing a grouping structure — more complex than the flat alternative below. |
| **Capability tuple `(scope, target)`, flat set, exact-match** | **Chosen** | Directly mirrors Phase 1's own choice (§4: "opaque scope strings, flat set, exact-match") one dimension over. Minimal: one new atomic field, no new containment semantics, no new graph entity. Total-orderable (lexicographic on `(scope, target)`), trivially serializable, and the existing subset-based Non-Amplification algorithm (§11) generalizes to it with no new algorithmic machinery — swap "scope string" for "capability tuple" and the Phase 1 algorithm is already correct. |

**Decision:** in a version-2 document, authority is a `set<(scope,
target)>`. Each element:

- `scope` uses the existing Phase 1 grammar and semantics unchanged:
  `^[A-Za-z0-9_.:-]{1,256}$`, exact byte equality only, no hierarchy.
- `target` is a new grammar, matching node-id style rather than
  scope style (see §5): `^[A-Za-z0-9_.-]{1,128}$`.
- The pair is compared by **exact tuple equality only** — same scope
  *and* same target. `billing:read@billing-service` and
  `billing:read@payroll-service` are different, unrelated capabilities
  with no implied relationship.
- A capability set is canonicalized for output by sorting
  lexicographically on `(scope, target)` and de-duplicating; a duplicate
  tuple within one declared set is a validation error, not silently
  deduped (mirrors Phase 1 §4's duplicate-scope rule).

Why `scope@target` is a safe display separator (text output, §15): `@`
appears in neither the scope grammar nor the target grammar, so
`scope@target` round-trips unambiguously for humans, exactly as Phase 1's
trace separator `" -> "` is safe because `>` is excluded from the id
grammar.

Version-1 documents are entirely unaffected: their authority remains
`set<string>`, their algorithm remains Phase 1's, unchanged (§9).

---

## 5. Context/audience model

Of the candidates named in the task brief — audience, target, service,
resource — **one** minimal abstraction is chosen: **target**, a bare
opaque label.

Explicitly rejected additions, and why:

- **No `Service`/`Resource` graph entity.** Phase 1 §3.1 already rejected
  modeling tools/resources as first-class graph nodes with their own
  identity, on the grounds that it adds no verification power and pulls
  in MCP-shaped concepts prematurely. That reasoning applies unchanged
  here: a target does not need identity, attributes, or relationships to
  other targets — it only needs to be comparable to another target
  string. Introducing a `Service` node type would require its own
  validation (existence, uniqueness) and would not change what the
  invariant checks; it is pure ceremony for Phase 2's purposes.
- **No target registry, no "unknown target" validation error.** A
  registry would require a new top-level document section (e.g.
  `"targets": [...]`) purely so other fields could reference it — the
  same shape of complexity Phase 1 rejected for tools/resources. A target
  string is valid Phase 2 input as long as it matches the grammar; it
  needs no prior declaration. This is a deliberate, permanent design
  decision, not a deferred one: DelegationProof verifies a *declared*
  model's internal consistency (Phase 1 §17's security assumptions), not
  correspondence with a real system's actual set of services. There is
  nothing to check a target string against.
- **No `audience` terminology.** "Audience" carries OAuth/OIDC
  connotations (a token's intended recipient, verified via `aud` claim
  checks against a trusted issuer). DelegationProof does not verify
  tokens or issuers; "target" more accurately describes what is actually
  being modeled — an opaque destination label chosen by the document
  author, compared only against other labels in the same document.

**Grammar decision:** target uses the same character class and length
bound as node ids (`^[A-Za-z0-9_.-]{1,128}$`), not the scope grammar. A
target names a single destination ("billing-service"), structurally closer
to a node id than to a colon-delimited scope. It deliberately excludes `:`
so that a future colon-delimited target-hierarchy grammar (if ever
designed) is not silently pre-empted by allowing that character now.

**No cross-namespace check.** A target string may coincidentally equal an
existing node id or scope string; this has no special meaning and is not
validated against. Targets, node ids, and scopes are three independent
namespaces that happen to share similar grammars.

---

## 6. Delegation semantics

A version-2 delegation edge is exactly:

```json
{
  "delegator": "<node id>",
  "delegatee": "<node id>",
  "authority": [
    { "scope": "<scope>", "target": "<target>" }
  ]
}
```

All Phase 1 structural rules on the edge shape carry over unchanged:
non-empty `authority`, at most one edge per `(delegator, delegatee)` pair,
no self-delegation, `delegatee` cannot resolve to a principal. The only
change is what one entry of `authority` looks like.

**Delegator/delegatee/subset relationship, generalized:** an edge
`e = (d, t, A)`, where `A` is now a set of capability tuples, is *valid*
iff `A ⊆ DA(d)` — exact tuple-set subset, not scope-only subset. This is
the identical rule from `docs/phase-1-plan.md` §5, with "authority set"
read as "capability set." A delegator that holds `billing:read` only for
`billing-service` cannot validly delegate `billing:read` for
`payroll-service`, even though it does hold that scope string — because
it does not hold that *capability*.

**Strict distrust, generalized (unchanged in spirit):** if `A ⊄ DA(d)`,
the edge contributes **nothing** to the delegatee's derived authority —
not the tuples that *were* individually present in `DA(d)`, not even
tuples sharing a scope with something the delegator does hold under a
different target. This is the same all-or-nothing rule from
`docs/phase-1-plan.md` §6.1, generalized from "scope not in `DA(d)`" to
"capability tuple not in `DA(d)`." A future Phase 2 test,
`TestStrictDistrustNoPartialCredit` generalized to capability tuples,
must assert this directly (mirrors the existing Phase 1 test named in
CLAUDE.md).

---

## 7. Formal invariant: Context-Binding Preservation

### 7.1 Statement

> **Context-Binding Preservation:** for every version-2 node `n` and
> every capability `c = (s, t)` that `n` is declared to exercise or
> transmit, if `s` is present in `DA(n)` under some target `t' ≠ t` but
> `(s, t) ∉ DA(n)`, that is a context-binding violation, not silent
> success and not an amplification finding.

This is checked at the same two points Phase 1 already checks (§6.2,
generalized in §8 below):

1. **Edge-level:** for delegation edge `e = (d, t, A)`, every capability
   in `A \ DA(d)` is classified per §8's precedence rule.
2. **Operation-level:** for operation `op = (actor, action, requires_scope,
   requires_target)`, if `(requires_scope, requires_target) ∉ DA(actor)`,
   classify per §8.

### 7.2 Why this is not folded silently into Non-Amplification

Simply redefining `DA(n)` over tuples and re-running Phase 1's exact
algorithm verbatim (tuple equality instead of string equality) *would*
already catch every context-binding violation — a missing tuple is a
missing tuple, algorithmically. That reuse is intentional (§11) and is
exactly why Phase 2 needs no new verification algorithm, only a new
element type flowing through the existing one.

What Phase 1's algorithm alone would **not** do is tell the operator
*why* the tuple was missing: "you never had this scope, anywhere" is a
different, more urgent problem than "you have this scope, just not for
this target." Collapsing both into a single `authority_amplification`
finding would be a real loss of diagnostic value — the former needs a new
delegation; the latter needs either a new binding or a corrected
target on the operation. Phase 2 therefore keeps one algorithm (§11) but
adds one classification step (§8) that produces two distinct finding
kinds from it.

---

## 8. Interaction with Authority Non-Amplification

Phase 1's invariant "must remain independently enforceable" (task
instruction). It does, in the strongest possible sense: **Context-Binding
Preservation does not replace or modify Authority Non-Amplification — it
is the same generalized subset check, with a classification layer on top
that separates two ways the same check can fail.**

For version-1 documents, nothing changes: `DA(n)` is a set of bare
strings, the check is scope membership, and every violation is
`authority_amplification`, exactly as today. Target does not exist in
that document's vocabulary, so context-binding violations are not
representable and cannot occur — Phase 1's invariant runs exactly as it
always has (§9).

For version-2 documents, given a missing capability `(s, t)` that should
have been reachable at node `n` (either as part of an edge's declared set,
or as an operation's requirement), classification is:

```
classify(s, t, heldCapabilities):
    heldTargetsForScope = { t' : (s, t') ∈ heldCapabilities }
    if heldTargetsForScope == ∅:
        return authority_amplification   # scope never validly held, any target
    else:
        return context_binding_violation # scope held, wrong target(s) only
```

**Precedence rule for edge-level findings with multiple missing
capabilities:** one edge finding covers the whole excess set `A \
DA(d)` (Phase 1 §9 keeps one finding per invalid edge, not one per
missing scope). If that excess set contains *any* capability whose scope
has `heldTargetsForScope == ∅`, the finding's `violation` is
`authority_amplification` — the more foundational problem (a scope
missing outright) takes precedence over a subtler binding mismatch, so it
is never masked by a co-occurring binding issue. Only when *every*
capability in the excess set has a non-empty `heldTargetsForScope` (i.e.
every missing capability is a pure context mismatch) is the finding's
`violation` set to `context_binding_violation`. This is a pure function of
the finding's own content, hence deterministic, and requires no new
input.

Operation-level findings always cover exactly one required capability
(Phase 1 §7.2's singular-`requires` design is preserved, §7's schema),
so `classify` is applied directly with no precedence question.

---

## 9. Schema/version compatibility

**Decision: a new schema version (`"2"`), interpreted alongside version
`"1"` by the same binary, with no shared internal model type between
them.** This is the "new schema version" option from the task's decision
list, not a silent compatibility interpretation of `"1"` and not an
unchanged `"1"` schema stretched to carry an optional field.

Why not stretch version `"1"`: Phase 1 §7.2 requires `version` to equal
the literal string `"1"`, checked as a **hard equality**, specifically
"future-proofing for Phase 2+ schema changes without silently
misinterpreting old/new files" (`docs/phase-1-plan.md` §7.2). Adding an
optional `target` field to `authority` entries under the same version
literal would be exactly the silent misinterpretation that check exists
to prevent — a v1 consumer parsing a file with per-entry targets would
have no signal that semantics changed. A new version literal is the
mechanism Phase 1 built for this.

Why not a wildcard/implicit-any target for old-style bare-string
authority under version `"2"`: this was considered and rejected. Making
`target` optional per capability entry, defaulting a missing target to
"valid against anything," is exactly the wildcard-matching semantics the
task instructions bar outright. It would also reintroduce ambiguity
Phase 1's strict-decode philosophy (`DisallowUnknownFields`, §7.3)
exists to avoid: two representations (with/without target) meaning
different things under the same version. Version `"2"` therefore
requires every capability entry and every operation to carry an explicit
`target` — no optional target, no default, no wildcard.

**Dispatch mechanism (loader):**

1. Read the file, apply the byte-size bound (`limits.MaxInputFileSize`),
   exactly as today.
2. Decode *only* `{"version": string}` from the raw bytes, permissively
   (no `DisallowUnknownFields`, ignore all other top-level keys) — this
   step exists solely to read the version literal before committing to a
   struct shape.
3. Dispatch on the literal:
   - `"1"`: decode the full document into the existing `model.Model`
     type with `DisallowUnknownFields`, and run the **existing, untouched**
     `validate` function. Byte-for-byte identical to Phase 1 today.
   - `"2"`: decode into a new `model.ModelV2` type (§19) with
     `DisallowUnknownFields`, and run a new `validateV2` function (§10).
   - anything else (including absent, which step 2 yields as `""`): a
     single `invalid_version` validation error. The message text updates
     from `version must be "1", got %q` to `version must be "1" or "2",
     got %q` — the one deliberate, sanctioned wording change to existing
     Phase 1 code (§21 regression requirements explain why this is safe).

This keeps the two schemas — and their entire downstream code paths —
structurally disjoint. A version-1 document can never accidentally be
interpreted with version-2 semantics or vice versa; there is no shared
"maybe-has-a-target" struct anywhere.

---

## 10. Validation rules

Every existing §7.4 structural rule from `docs/phase-1-plan.md` applies
to version-2 documents unchanged, generalized where the shape changed
(e.g. "empty authority array" still applies — now to an array of
capability objects instead of scope strings; "duplicate scope within one
authority array" becomes "duplicate capability tuple within one authority
array").

New version-2-only structural rules:

- **Invalid target format.** `target` must match
  `^[A-Za-z0-9_.-]{1,128}$` (§5). New error kind `invalid_target`,
  mirroring `invalid_id`/`invalid_scope`.
- **Missing target.** A capability object or operation lacking `target`
  decodes as an empty string, which fails the regex above and is
  reported as `invalid_target` — the same mechanism Phase 1 already uses
  for missing/empty ids and scopes (no separate "missing field" error
  kind needed).
- **Duplicate capability binding.** Two entries with the exact same
  `(scope, target)` pair within one principal's or one edge's `authority`
  array. New error kind `duplicate_capability`, generalizing
  `duplicate_scope`. (Two entries with the same scope but *different*
  targets are not a duplicate — they are two distinct, legitimate
  capabilities.)
- **Resource-limit check on target length.** New bound
  `limits.MaxTargetLength` (§17), checked the same way
  `MaxIDLength`/`MaxScopeLength` are today.

Explicitly evaluated and **rejected** as structural rules (the task's
suggested list is not adopted wholesale — only cases the chosen model
actually justifies):

- **"Unknown target/audience."** Rejected by design (§5): there is no
  target registry to be unknown against. Not a validation error.
- **"Invalid delegation context" / "operation context mismatch."**
  These are not structural problems — a document where every capability,
  scope, and target is individually well-formed but a delegation edge
  or operation's target doesn't match what was actually delegated is
  exactly the **semantic finding** this phase exists to detect (§7, §8).
  Treating it as a structural (exit-2) error would be a category error:
  Phase 1 §7.4 draws this same line for over-claimed scopes (a
  structurally valid delegation that grants more than the delegator
  holds is not rejected at validate time — it is a §9 finding at verify
  time). Context-binding mismatches follow the identical precedent.

`validate` on a version-2 document therefore still never evaluates
either invariant (Non-Amplification or Context-Binding) — exactly as
Phase 1 §10 established for `validate` vs `verify`.

---

## 11. Verification algorithm

Given a structurally valid version-2 model, the algorithm is Phase 1's
`docs/phase-1-plan.md` §8 algorithm with "scope" read as "capability
tuple `(scope, target)`" throughout, plus one classification step. It
remains a single topological pass — no new algorithmic structure, no
backtracking, no branching:

1. **Build the graph.** Identical to Phase 1: nodes = principals ∪
   agents, edges = delegations. Already known acyclic (validated in
   §10). `graph.TopoSort` and `graph.LongestPath` are reused **as-is** —
   they operate on node ids and edges only, never on authority content,
   so nothing in `internal/graph` needs to change for Phase 2.
2. **Topological evaluation**, node ids in ascending-lexicographic
   tie-broken topological order (identical tie-break rule to Phase 1):
   - Principal `p`: `DA(p) = canonicalize(p.capabilities)` — sorted,
     deduplicated by `(scope, target)`.
   - Agent `a`: `DA(a) = ∅` initially. For each incoming edge `e`,
     ascending lexicographic order of `e.delegator`:
     - If `e.capabilities ⊆ DA(e.delegator)` (tuple-set subset):
       `DA(a) := DA(a) ∪ e.capabilities`; mark `e` valid.
     - Else: mark `e` invalid; compute `excess = e.capabilities \
       DA(e.delegator)`; classify the finding's `violation` per §8's
       precedence rule; emit a `CapabilityEdgeFinding` (§12).
   - `DA(a) := canonicalize(DA(a))`.
3. **Operation evaluation**, operations ordered by ascending
   `(actor, action, requires_scope, requires_target)` tuple (extends
   Phase 1's `(actor, action, requires)` ordering with `target` as a
   trailing tiebreaker — needed because two operations can now share
   `(actor, action, requires_scope)` and differ only by target):
   - If `(requires_scope, requires_target) ∈ DA(actor)`: pass.
   - Else: classify per §8; emit a `CapabilityOperationFinding` (§12).
4. **Sort all findings** by the extended key tuple (§12).
5. **Result:** `ALLOW` (exit 0) if findings is empty, else `DENY`
   (exit 1) — identical result semantics to Phase 1.

**Complexity:** unchanged asymptotically from Phase 1 §8.2 —
`O(N + E + O)` — because a capability tuple is still a constant-size
comparable value; the subset/membership checks are the same set
operations Phase 1 already performs, just over a different element type.
**No state-space exploration is introduced or required.** Target adds a
second dimension to the *data*, not a time dimension, a branching
decision, or a source of nondeterminism to the *model*. Phase 1 §8.2's
reasoning for why a single deterministic pass suffices applies to Phase 2
without modification: there is still no conditional grant, no approval
workflow, no revocation, nothing to search over.

Version-1 models continue to run `internal/verify.Run`, completely
untouched, producing byte-identical output to Phase 1 today.

---

## 12. Deterministic findings

Two new finding types, alongside Phase 1's unmodified `EdgeFinding` /
`OperationFinding`:

```go
type Capability struct {
    Scope  string `json:"scope"`
    Target string `json:"target"`
}

type CapabilityEdgeFinding struct {
    Violation    string       `json:"violation"`     // "authority_amplification" | "context_binding_violation"
    Point        string       `json:"point"`          // "delegation_edge"
    Delegator    string       `json:"delegator"`
    Delegatee    string       `json:"delegatee"`
    Declared     []Capability `json:"declared"`
    Excess       []Capability `json:"excess"`
    BoundTargets []string     `json:"bound_targets"`   // targets s IS bound to, for context_binding_violation; [] otherwise
    Trace        []string     `json:"trace"`
    Reason       string       `json:"reason"`
}

type CapabilityOperationFinding struct {
    Violation    string     `json:"violation"`
    Point        string     `json:"point"`             // "operation"
    Actor        string     `json:"actor"`
    Action       string     `json:"action"`
    Requires     Capability `json:"requires"`
    Held         []Capability `json:"held"`
    BoundTargets []string     `json:"bound_targets"`
    Trace        []string     `json:"trace"`
    Reason       string       `json:"reason"`
}
```

`violation` is one of the two literals defined in §3/§8 —
`authority_amplification` (reused byte-for-byte from Phase 1) or the new
`context_binding_violation`. `point` reuses Phase 1's two existing
values unchanged; Phase 2 introduces no new detection point.

**Deterministic reason text** (new templates, generated, not free-form —
same discipline as `docs/phase-1-plan.md` §9):

- `authority_amplification` (edge or operation, capability-flavored):
  `"<subject> attempted to delegate <scope>@<target>, which is not in
  <subject>'s derived authority"` / `"<scope>@<target> was never present
  in the valid delegation chain reaching <actor>"` — the exact Phase 1
  sentence, with the capability rendered `scope@target`.
- `context_binding_violation`:
  `"<scope> is held by <subject> only for [<bound_targets joined by
  ", ">], which does not include <target>"`.

`bound_targets` is always present (never omitted or null, per Phase 1
§9's array-field rule) — `[]` for `authority_amplification` findings,
non-empty for `context_binding_violation` findings.

**Extended sort order.** `report.Sort`'s key tuple grows one field:
`(point, subject_id, secondary_id_or_action, scope, target)`. For edge
findings, `scope`/`target` stay `""` (a `(delegator, delegatee)` pair is
already unique — Phase 1 §3.2's at-most-one-edge-per-pair rule holds
unchanged in Phase 2, §6). For operation findings, `scope` and `target`
are `requires.Scope`/`requires.Target` — needed because two operations
can now legitimately share `(actor, action, requires.Scope)` and differ
only by target. This is a strict extension: for any Phase-1-shaped
finding (target always `""`), the 5-tuple order degenerates to exactly
Phase 1's existing 4-tuple order, so `TestJSONFormatDeterministicAcross...`-
style tests over version-1 fixtures are unaffected.

---

## 13. Counterexample traces

Reused without modification: `graph.CanonicalTrace` (§1) already operates
purely on node ids and a `[]graph.Edge` list — it has no dependency on
what authority an edge carries. Phase 2 supplies it the same `validEdges`
accumulator pattern Phase 1 uses, with one change to what "valid" means:
an edge is added to `validEdges` only if the **entire capability subset
check** (§11 step 2) passed — i.e. the edge is fully valid, not merely
scope-valid-but-context-invalid. A context-binding-invalid edge is
excluded from all downstream traces, exactly as an amplification-invalid
edge is excluded today (Phase 1 §8.1's strict-distrust framing extends
unchanged: an edge that fails on *any* classification is not a path
Context-Binding Preservation, or the trace builder, should ever route
through).

No new trace construction logic is needed.

---

## 14. CLI compatibility

**No new subcommands.** `validate <model.json>` and `verify <model.json>
[--format text|json]` are unchanged in name, argument shape, and flag
surface. The version dispatch (§9) happens entirely inside `loader.Load`
and `verify.Run`'s call site; `main.go`'s `runValidate`/`runVerify`
functions need only route to the version-appropriate `Run` function based
on which model type `loader.Load` returned — a small, mechanical addition
(a type switch or a tagged union return), not a new user-facing surface.

No new flags. `--format text|json` applies identically to version-1 and
version-2 results. No `--schema-version` override flag, no "target"-
specific CLI knob — the version is read from the document itself,
exactly as today's `version: "1"` already is.

---

## 15. Text/JSON output

**JSON.** The top-level envelope is unchanged: `{"result": "ALLOW" |
"DENY", "findings": [...]}` (`internal/report/json.go`, untouched). For a
version-1 model, every finding is shaped exactly as today — zero bytes
differ for any existing consumer parsing a version-1 result. For a
version-2 model, `findings` contains `CapabilityEdgeFinding` /
`CapabilityOperationFinding` objects (§12) — a strict superset of
information (capability objects instead of bare strings, plus
`bound_targets`), never a breaking change to a schema no version-1
consumer has ever seen, because version-2-shaped findings only ever
appear in the output of a version-2 input.

**Text.** Mirrors Phase 1's `text.go` structure and tone
(`[n] <violation> (<point>)` header, then labeled fields), rendering
each `Capability` as `scope@target`:

```
[1] context_binding_violation (operation)
  actor:         billing-agent
  action:        read-record
  requires:      billing:read@payroll-service
  held:          billing:read@billing-service
  bound targets: billing-service
  trace:         user -> billing-agent -> read-record
  reason:        billing:read is held by billing-agent only for billing-service, which does not include payroll-service
```

`joinOrNone` (existing helper) is reused for `bound targets` and `held`
exactly as it is for Phase 1's `declared`/`excess`/`held` today.

---

## 16. Exit codes

Unchanged. `internal/exitcode` gains no new values:

| Code | Meaning (extended) |
|---|---|
| `0` | Structurally valid model (both versions); zero findings for `verify`. |
| `1` | One or more findings — `authority_amplification` and/or `context_binding_violation`, in any combination, at either point. A model can `DENY` on either violation kind, or both simultaneously; the exit code does not distinguish which. |
| `2` | Structural/model problem for either schema version, including the new `invalid_target`/`duplicate_capability` kinds and the `max_target_length` resource limit. |
| `3` | CLI usage error — unchanged, never about model content or version. |

---

## 17. Resource bounds

One new bound, mirroring the existing `MaxIDLength` pattern exactly:

| Limit | Value | Notes |
|---|---|---|
| `MaxTargetLength` | 128 bytes | New. Mirrors `MaxIDLength`, since target grammar mirrors id grammar (§5). |

All existing Phase 1 bounds apply unchanged to version-2 documents, with
"authority set" read as "capability set" where relevant:

| Limit | Applies to v2 as |
|---|---|
| `MaxInputFileSize` | Unchanged — byte-size check happens before version dispatch. |
| `MaxNodes` | Unchanged — node/edge count is orthogonal to capability shape. |
| `MaxDelegationEdges` | Unchanged. |
| `MaxOperations` | Unchanged — one operation entry still equals one required capability. |
| `MaxScopeLength` | Unchanged — applies to the `scope` half of each capability tuple. |
| `MaxIDLength` | Unchanged. |
| `MaxAuthoritySetSize` | Unchanged, reused: now bounds the number of capability *tuples* per principal/edge (256), not the number of bare scopes. No new bound needed — a tuple is still one entry. |
| `MaxChainDepth` | Unchanged — depth is a property of the graph, not of authority content. |

No new bound for "number of distinct targets per scope" or similar — a
principal/edge's capability set is already bounded in total size by
`MaxAuthoritySetSize`, which is sufficient; inventing a second,
narrower bound would be exactly the kind of unjustified extra knob
Phase 1 §12 avoided by using a small, fixed set of constants.

---

## 18. Example

New fixture, `examples/billing-context-binding.json`, matching the task's
suggested scenario exactly, and structured the same deliberate way
`examples/billing-refund.json` is (Phase 1 §14): small, readable, and
exercising exactly the invariant it is meant to demonstrate — one valid
capability, one context-binding violation, same nominal scope in both
operations so the *only* difference is target:

```json
{
  "version": "2",
  "principals": [
    { "id": "user", "authority": [
      { "scope": "billing:read", "target": "billing-service" }
    ] }
  ],
  "agents": [
    { "id": "billing-agent" }
  ],
  "delegations": [
    { "delegator": "user", "delegatee": "billing-agent", "authority": [
      { "scope": "billing:read", "target": "billing-service" }
    ] }
  ],
  "operations": [
    { "actor": "billing-agent", "action": "read-invoice", "requires": "billing:read", "target": "billing-service" },
    { "actor": "billing-agent", "action": "read-record",  "requires": "billing:read", "target": "payroll-service" }
  ]
}
```

`verify examples/billing-context-binding.json`:

- `read-invoice` (target `billing-service`) — **passes**: the capability
  `billing:read@billing-service` is exactly what was delegated down the
  chain.
- `read-record` (target `payroll-service`) — **fails**:
  `billing:read@payroll-service` is not held; `billing:read` *is* held,
  but only for `billing-service`. Classified `context_binding_violation`
  (§8: `heldTargetsForScope = {"billing-service"}`, non-empty), not
  `authority_amplification`.

This single file demonstrates a valid context-bound delegation chain, a
passing operation, and a context-binding violation, mirroring exactly
how `billing-refund.json` demonstrates the equivalent trio for
Non-Amplification in Phase 1.

A second fixture, `testdata/valid/combined-violations.json` (or similar,
under `testdata/`, not `examples/`), additionally covers a model with
*both* an `authority_amplification` finding and a `context_binding_violation`
finding in one `verify` run, per the regression requirement in §20.

---

## 19. File/architecture plan

Purely additive to `docs/phase-1-plan.md` §15's layout. No existing file
is deleted or renamed; existing Phase 1 functions are not modified except
the one sanctioned touch point named in §9/§21.

```
internal/model/
  types.go              — UNCHANGED (Phase 1 types)
  types_v2.go            — NEW: Capability{Scope, Target}, PrincipalV2,
                            AgentV2 (identical shape to Agent), DelegationV2,
                            OperationV2 (adds Target), ModelV2

internal/limits/
  limits.go               — ADD: MaxTargetLength var (§17). One-line, additive.

internal/loader/
  loader.go                — existing validate()/Load() UNCHANGED in
                              behavior; ADD a thin version-peek step (§9)
                              in front of the existing decode, and a new
                              validateV2() function (§10) that reuses
                              existing helpers (checkID-style regex check
                              for target, checkScope, resourceLimitErr,
                              sortErrors) rather than duplicating them.

internal/graph/
  graph.go                  — UNCHANGED. Reused as-is (§11, §13).

internal/verify/
  verify.go                 — Run(*model.Model) UNCHANGED.
  verify_v2.go               — NEW: RunV2(*model.ModelV2) report.Result,
                                implementing §11 and the §8 classification
                                step.

internal/report/
  finding.go                 — EdgeFinding/OperationFinding UNCHANGED;
                                ADD Capability, CapabilityEdgeFinding,
                                CapabilityOperationFinding,
                                ViolationContextBinding const,
                                NewCapabilityEdgeFinding/
                                NewCapabilityOperationFinding constructors,
                                and extend keyOf/less (§12) with two more
                                type-switch cases.
  text.go                     — extend RenderText's type switch with the
                                two new finding types (§15); existing
                                cases untouched.
  json.go                     — UNCHANGED (already generic over
                                []interface{}).

cmd/delegationproof/
  main.go                     — runValidate/runVerify gain a small,
                                mechanical dispatch on which model type
                                loader.Load returned (§14); no new flags,
                                no new subcommands, no change to exit-code
                                mapping (§16).

examples/
  billing-refund.json          — UNCHANGED.
  billing-context-binding.json — NEW (§18).

schemas/
  model.md                     — NOT modified this session (explicit
                                  instruction); the implementation session
                                  must add a "version 2" section (or a
                                  sibling model-v2.md) documenting §6/§10,
                                  mirroring how model.md documents §7
                                  today. Flagged here so it is not
                                  forgotten, per Phase 1's own precedent
                                  of schemas/ being docs-only, secondary
                                  to internal/loader.

testdata/
  valid/                        — ADD v2 fixtures: a clean-pass v2 model,
                                  a reordered-arrays v2 variant (for the
                                  permutation-invariance test, §20).
  malformed/                    — ADD one v2 fixture per new §10 error
                                  kind: invalid-target-format.json,
                                  missing-target.json,
                                  duplicate-capability.json,
                                  target-exceeds-max-length.json. Existing
                                  v1 fixtures UNCHANGED, still walked by
                                  cmd/delegationproof/main_test.go
                                  automatically (CLAUDE.md's existing
                                  convention extends for free).
  golden/                       — ADD captured v2 text/json output for
                                  billing-context-binding and a v2
                                  combined-violations fixture. Existing
                                  v1 golden files UNCHANGED, asserted
                                  byte-identical (§21).

docs/
  phase-2-plan.md               — this document.
```

---

## 20. Testing plan

Mirrors the structure of `docs/phase-1-plan.md` §16, additive:

1. **Phase 1 full regression** — `go test ./... -race -count=1` with zero
   modifications to any existing `_test.go` file; every existing golden
   file byte-identical; every existing malformed fixture still produces
   its original `ErrorKind`.
2. **Valid context-bound delegation, clean pass** — a v2 model with
   matching scope+target end to end → `ALLOW`, golden text+json.
3. **Invalid cross-target use** — `examples/billing-context-binding.json`
   (§18) → exactly one `context_binding_violation` finding, golden
   text+json, exact `bound_targets`/`reason` text asserted.
4. **Propagation across multiple delegation hops** — a 3+ hop chain where
   the capability is correctly bound at every hop, then one operation at
   the leaf uses the wrong target → confirms `DA(n)` tuple propagation is
   correct end-to-end, not just single-hop.
5. **Deterministic findings** — a v2 model with multiple mixed
   `authority_amplification` and `context_binding_violation` findings,
   asserting exact sort order per the extended 5-tuple key (§12).
6. **Reordered-input invariance** — v2 analogue of
   `TestJSONFormatInputArrayPermutationInvariance`: byte-identical output
   for semantically-equivalent reordered `principals`/`agents`/
   `delegations`/`operations`/capability-array orderings.
7. **Malformed context declarations** — one case per new §10 error kind:
   invalid target format, missing target (empty string), target
   exceeding `MaxTargetLength`, duplicate `(scope, target)` tuple within
   one authority array.
8. **Resource limits** — `MaxTargetLength` exceeded (white-box test,
   lowered limit, same pattern as existing `internal/limits`-based
   tests); confirm existing bounds (`MaxAuthoritySetSize` etc.) still
   apply correctly when counting tuples instead of bare scopes.
9. **Text/JSON output** — golden-file tests for both formats on the new
   example and at least one multi-finding v2 fixture.
10. **Exit codes** — v2 `validate` vs `verify` divergence test (mirrors
    Phase 1 §16 item 19): a structurally valid v2 model with a
    context-binding violation → `validate` exit 0, `verify` exit 1.
11. **Strict distrust interaction** — a v2 edge that grants a capability
    set where some tuples are individually valid and one is not (either
    kind of invalid) → confirm the *entire* edge is distrusted, zero
    tuples propagate, matching §6's generalized strict-distrust rule.
12. **Combined Phase 1 + Phase 2 violations** — a single v2 model
    producing both an `authority_amplification` and a
    `context_binding_violation` finding in one `verify` run (§18's second
    fixture), asserting both appear, correctly classified, correctly
    ordered relative to each other.
13. **Precedence rule** — a dedicated test for §8's edge-level
    precedence: an edge whose excess set contains one capability with
    `heldTargetsForScope = ∅` and one with a non-empty
    `heldTargetsForScope` → asserts the finding is classified
    `authority_amplification`, not `context_binding_violation`.
14. **No panics** — extend the existing fuzz/mutation-style CLI test to
    include v2 fixtures as seeds.

---

## 21. Phase 1 regression requirements

- Every existing test in `internal/loader`, `internal/graph`,
  `internal/verify`, `internal/report`, and `cmd/delegationproof` must
  pass unmodified.
- Every existing golden file in `testdata/golden/` must remain
  byte-identical output for its existing input.
- Every existing fixture in `testdata/malformed/` must continue to
  produce its documented `ErrorKind`.
- `examples/billing-refund.json` must continue to round-trip exactly as
  `docs/phase-1-plan.md` §19 specifies.
- **The one sanctioned touch point:** the `invalid_version` error
  message text changes from `version must be "1", got %q` to `version
  must be "1" or "2", got %q`, because the loader must now accept two
  literals instead of one. This is confirmed safe because (a)
  `loader_test.go`'s malformed-fixture table asserts `ErrorKind`, not
  message text (verified directly in the existing test file — see §1),
  and (b) no `testdata/golden/` fixture is a malformed/invalid-version
  document (golden files are `verify` output on structurally valid
  models only). No other line in `loader.go`'s existing `validate`
  function, nor any line in `internal/verify/verify.go`,
  `internal/graph/graph.go`, or `internal/report/`'s existing
  types/functions, may change.
- `go vet ./...`, `gofmt -l .`, and `go build -o bin/delegationproof
  ./cmd/delegationproof` must all succeed exactly as CLAUDE.md requires
  today, with the new v2 files included.

---

## 22. Security assumptions

Extends `docs/phase-1-plan.md` §17 without weakening it:

- A `target` string is a **declared label**, not a verified identity. As
  with Phase 1's principal `declared_authority` being the axiomatic root
  of trust, Phase 2 does not verify that a target string corresponds to
  any real, running service, nor that the document author's choice of
  target names is meaningful or consistent with a real deployment. That
  correspondence remains, as in Phase 1 §17, a separate later integration
  concern (real topology ingestion).
- Context-Binding Preservation proves a property of the **declared**
  model only: "this document never claims a capability is valid for a
  target it wasn't delegated for." It does not, and cannot, prove that
  the real system enforces target boundaries at runtime — DelegationProof
  remains a static, offline analyzer with no interception or enforcement
  component (Phase 1 §17, §18, unchanged).
- No new attack surface is introduced by parsing: `target` is decoded via
  the same `encoding/json` + `DisallowUnknownFields` + bounded-read
  pipeline as every other Phase 1 field, subject to the same
  `MaxInputFileSize` bound applied before any structural field is even
  read.

---

## 23. Explicit non-goals

All of `docs/phase-1-plan.md` §18's non-goals continue to apply. Phase 2
additionally, explicitly, does **not** include:

- MCP protocol implementation, A2A protocol implementation, OAuth
  implementation, networking, hosted service, proxying, runtime
  enforcement, databases, LLMs, Z3/SAT/SMT, SARIF, approvals, revocation,
  temporal state.
- State-space exploration — not required (§11) and not added.
- Wildcard scopes, regex permissions, hierarchical IAM language, or any
  wildcard/hierarchy semantics for `target` (§4, §5, §9) — target is
  exact-match only, exactly like scope.
- A web UI, automatic policy generation, CI-vendor integration.
- Phase 3 implementation.
- **A `Service`/`Resource`/target-registry graph entity** (§5) —
  evaluated and rejected as unnecessary for this invariant.
- **Confused-deputy detection.** Per explicit product direction: Phase 2
  provides the context-binding foundation only. Confused-deputy detection
  needs a "caller" concept distinct from "delegator" (who *induced* an
  action, vs. who granted authority for it) — an orthogonal relationship
  to target binding, not a refinement of it. Target tells you *where*
  authority may be exercised; confused-deputy is about *who* induced the
  exercise. Nothing in this design's capability-tuple generalization
  helps or hinders that later addition, matching
  `docs/phase-1-plan.md` §21's own framing of confused-deputy detection
  as a distinct, later relationship layered on top of the same graph.
- **Approval preservation, delegation-depth policy** — both remain
  exactly as scoped by `docs/phase-1-plan.md` §21, untouched by this
  phase.

---

## 24. Acceptance criteria

- `go build ./...` succeeds; `go.mod` remains stdlib-only.
- `go vet ./...` is clean; `gofmt -l .` reports nothing.
- `go test ./... -race -count=1` passes, including every category in
  §20, with zero modification to any pre-existing Phase 1 test file.
- Every existing `testdata/golden/` file is unchanged, byte-identical.
- A version-1 document produces byte-identical `validate`/`verify`
  output, on both `text` and `json` formats, to Phase 1 today — verified
  directly by re-running the existing golden fixtures through the
  post-Phase-2 binary.
- A version-2 document with no violations → `ALLOW`, exit 0.
- `examples/billing-context-binding.json` → exactly one
  `context_binding_violation` finding, matching the worked example in
  §18.
- A version-2 document containing both violation kinds simultaneously
  reports both, correctly classified, correctly ordered.
- Every new error kind in §10 has at least one dedicated malformed
  fixture and table-driven test case, mirroring the existing convention
  CLAUDE.md documents for Phase 1's `testdata/malformed/`.
- No panic is reachable from `main` for any version-1 or version-2 input
  within the §17 bounds.

---

## 25. Definition of DONE

Phase 2 is done when:

1. All items in §24 are met.
2. The file/package layout matches §19, or a documented deviation is
   noted in this document (keeping it authoritative, per Phase 1's own
   §20 convention).
3. Every new error kind (§10) and every new finding `violation`/`point`
   combination (§12) has at least one dedicated test.
4. The worked example (§18) is reproducible verbatim via
   `delegationproof verify examples/billing-context-binding.json`.
5. `schemas/model.md` has been updated (or a sibling v2 document added)
   by the implementation session to describe the version-2 shape — noted
   as deferred in §19, not done in this planning session per explicit
   instruction not to modify it now.
6. No open TODOs remain in code for functionality this document
   describes as in-scope; TODOs for §26's deferred items are fine and
   expected, linking back to §26.
7. `docs/phase-1-plan.md` is unmodified — Phase 2 attaches to it, per
   its own §21, without rewriting it.

---

## 26. Future-phase boundary

Carried forward from `docs/phase-1-plan.md` §21, still deferred, now with
Phase 2's addition noted where it changes the shape of what attaches:

- **Confused-deputy detection** (still deferred, §23): now has slightly
  more to attach to — a "caller" relationship would need to be checked
  against target binding too (did the *caller* have standing to induce
  use of this specific capability against this specific target?), but
  that composition is future design work, not started here.
- **Approval preservation, delegation-depth policy, MCP/A2A ingestion,
  JSON Schema enforcement, SARIF, Z3/SMT**: unchanged from
  `docs/phase-1-plan.md` §21; nothing in Phase 2 accelerates or blocks
  any of them.
- **Scope wildcard/hierarchy semantics**: still deferred, still requires
  its own containment grammar before it can be added (Phase 1 §21). Note
  for whoever eventually designs it: if/when scope hierarchy is added, a
  parallel decision will be needed for whether *target* ever gains
  hierarchy too (e.g. `billing-service/*`) — Phase 2 deliberately leaves
  target flat and exact-match (§4, §5), so that is a real future design
  question, not a foregone conclusion, and should not be assumed
  symmetric with whatever scope hierarchy design is eventually chosen.
- **Target/service registry as a first-class entity** (§5, newly
  identified in this phase): if a later product need genuinely requires
  validating target strings against a real, declared set of known
  services (as opposed to Phase 2's registry-free opaque labels), that
  is new scope, evaluated on its own merits then — not something this
  phase's rejection (§5) should be read as permanently foreclosing, only
  as not currently justified.
- **Multi-capability operations**: Phase 1's singular-`requires` design
  (§7.2) is preserved unchanged by Phase 2 (§7's operation schema keeps
  exactly one required capability per operation entry). An AND/OR
  requirement algebra across multiple capabilities per operation remains
  out of scope for the same reason Phase 1 gave: express it as multiple
  operation entries instead.
