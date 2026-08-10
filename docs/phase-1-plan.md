# DelegationProof — Phase 1 Plan

Status: PLANNING ONLY. No production code exists yet. This document is the
authoritative design contract for the Phase 1 implementation session. It
should be implementable without further product redesign.

Module: `github.com/SamudralaAjaykumarrr/delegationproof`

---

## 0. Phase 1 rationale

DelegationProof's long-term goal is a deterministic model checker for
authorization/delegation topologies in agentic systems (MCP tool access,
A2A delegation, capability scoping, approvals, audience binding, depth
limits, confused-deputy detection, bounded state-space exploration).

That full scope is deliberately **not** Phase 1. Phase 1's job is to prove
the smallest end-to-end slice is *correct, deterministic, and rigorous*:
one formal invariant, evaluated over a static graph, with a precise
input contract, a precise finding contract, and a test suite that locks
down determinism and failure behavior. Every later capability (audience
binding, approvals, depth limits, confused-deputy, state-space search,
MCP/A2A ingestion) is an additive extension of this foundation, not a
rewrite of it.

Concretely, Phase 1 answers: *given a declared, static delegation graph,
does any node exercise or receive authority it was never validly granted?*
Everything else is future work (see §21).

---

## 1. Product objective

Build a small, dependency-free, offline, deterministic CLI that:

1. Parses a JSON delegation model.
2. Structurally validates it (well-formedness, referential integrity,
   acyclicity, resource bounds).
3. Evaluates a single formal safety invariant — **Authority
   Non-Amplification** — over the model.
4. Reports findings (or a clean pass) as stable, sorted, reproducible
   text or JSON, with exit codes suitable for CI gating.

Identical input (up to reordering of array elements that represent the
same semantic model) must always produce byte-identical output.

---

## 2. Terminology

| Term | Meaning in Phase 1 |
|---|---|
| **Node** | A `principal` or an `agent`. The two kinds of participants in the delegation graph. |
| **Principal** | A root authority holder. Its authority is *axiomatic* (declared, trusted, not derived). Cannot be a delegation target. |
| **Agent** | A non-root participant. Its authority is never declared directly — it is *derived* solely from valid incoming delegation edges. |
| **Authority** | An opaque scope string (e.g. `"billing:write"`). Atomic, exact-match only. See §4. |
| **Delegation edge** | A directed grant: delegator → delegatee, carrying a specific authority set. See §5. |
| **Derived Authority**, `DA(n)` | The authority set a node actually, validly holds, computed by the algorithm in §8. |
| **Operation** | A declared point where an actor (node) attempts to exercise one required authority scope. Stands in for "this agent calls this tool/resource." Phase 1 does **not** model tools/resources/servers as graph entities — an Operation is the entire representation of "authority exercise." |
| **Finding** | A structured report that the invariant was violated at a specific edge or operation. |
| **Model** | The whole input document: principals + agents + delegations + operations. |

---

## 3. Formal model

### 3.1 Entities considered, and the decision on each

| Candidate | Included in Phase 1? | Reasoning |
|---|---|---|
| Principal | Yes | Root of trust; needed to anchor axiomatic authority. |
| Agent | Yes | The delegatee/delegator middle tier; the whole point of the product. |
| Delegation | Yes | The edge the invariant is checked against. |
| Authority/capability | Yes, as opaque scope strings | See §4 for why structured capabilities are rejected for now. |
| Tool | **No**, collapsed into Operation | A "tool call" is fully represented by "some actor requires some scope." Modeling tools as first-class graph nodes with their own identity adds no verification power in Phase 1 and pulls in MCP-shaped concepts prematurely (explicitly out of scope, §18). |
| Resource | **No**, collapsed into Operation | Same reasoning as Tool. Resource/audience *binding* (this authority is only valid *for* resource X) is a distinct, harder invariant (product idea #3) deferred to Phase 2 — see §21. |
| Service/MCP server | **No** | Pure topology-ingestion concern; deferred (§18, §21). |

Phase 1's graph therefore has exactly two node kinds (Principal, Agent),
one edge kind (Delegation), and one non-graph declaration kind
(Operation) used only to pose "does this node have this authority"
questions against the graph.

### 3.2 Graph representation

- A **directed graph** `G = (Nodes, Edges)`.
- `Nodes = Principals ∪ Agents`, disjoint sets, unified id namespace.
- `Edges = Delegations`, each edge `(delegator, delegatee, authority_set)`.
- `G` must be a **DAG**. Cycles (including self-loops) are a structural
  validation error, not a semantic finding — a cyclic delegation graph
  has no well-founded derived-authority solution, so it cannot even be
  evaluated. See §7.4, §9.
- Principals must have **in-degree 0** (cannot be a delegation target).
- At most **one edge per ordered pair** `(delegator, delegatee)`. Two
  grants between the same pair must be expressed as one edge with the
  union of scopes; duplicates are rejected as malformed input (keeps
  derivation a simple union-over-edges with no precedence/merge rules
  to design).

This is a static graph with no time dimension, no conditional grants, no
sessions. That is a deliberate Phase 1 boundary (§6 explains why no
state-space exploration is needed yet).

---

## 4. Authority representation

Candidates evaluated:

| Representation | Verdict | Why |
|---|---|---|
| Structured capability objects (resource, action, constraints, audience) | Rejected for Phase 1 | This is exactly the "policy language with hundreds of features" failure mode called out in the product positioning. It also requires a subset/containment algebra beyond simple set inclusion (e.g. constraint narrowing) — real complexity with no Phase-1 payoff. |
| Action/resource pairs (`{action, resource}`) | Rejected for Phase 1 | Presupposes a resource model, which is explicitly deferred (audience/resource binding is product idea #3, a *separate later invariant*). Baking resource identity into the authority representation now would force a redesign when #3 is tackled properly. |
| Opaque scope strings, flat set, exact-match | **Chosen** | Minimal, total-orderable (lexicographic), trivially serializable, sufficient to state and check Authority Non-Amplification precisely. Matches the worked example in the product brief (`billing:read`, `billing:write`). |

**Decision:** authority is a `set<string>`. A scope string:

- Matches `^[A-Za-z0-9_.:-]{1,256}$`.
- Is compared by **exact byte equality only**. No wildcards
  (`billing:*`), no hierarchy (`billing:write` does *not* imply
  `billing:read`), no namespacing semantics beyond "it's a string
  someone chose to colon-delimit for readability." Wildcard/hierarchy
  semantics are explicitly deferred (§21) because they require a
  defined containment grammar — a real design decision that deserves
  its own phase, not a Phase 1 afterthought.
- A set is canonicalized for output by sorting lexicographically and
  de-duplicating (duplicates within one declared set are a validation
  error, not silently deduped — see §9).

---

## 5. Delegation semantics

A delegation edge is exactly:

```
{
  "delegator": "<node id>",
  "delegatee": "<node id>",
  "authority": ["<scope>", ...]   // non-empty, deduplicated, sorted on output
}
```

Fields deliberately **excluded** from Phase 1, with reasoning:

- **Audience/target restriction** — this is product idea #3
  (audience/resource binding), a distinct invariant from #1
  (non-amplification). Bolting a partial version onto the edge now,
  before the resource/audience model exists, would guess at a shape
  we're not ready to commit to. Deferred to §21.
- **Depth field / depth semantics** — product idea #5 (delegation-depth
  enforcement) is a *policy* invariant (limit chains to N hops). Phase 1
  does include a **resource-bound** on chain length (§12) as a safety
  valve against pathological input, but that is not the same thing as
  a configurable policy invariant, and no such policy field exists on
  the edge in Phase 1.
- **Approval requirement** — product idea #4 needs an approval-state
  concept that doesn't exist yet. Deferred.
- **Validity window / revocation / timestamps** — no temporal dimension
  in Phase 1 at all (see §6).

**Delegator/delegatee/subset relationship (the core rule):** an edge
`e = (d, t, A)` is *valid* iff `A ⊆ DA(d)`, i.e. the authority being
granted must already be part of the delegator's own derived authority
at evaluation time. This is precisely delegation-chain validity
(product idea #2), and it is not a separate invariant from
non-amplification — it is the edge-local mechanism that *enforces*
non-amplification transitively. Phase 1 states one invariant (§6) with
two detection points (edge-level and operation-level), rather than two
separate invariants, to keep the mental model — and the implementation
— singular.

---

## 6. Phase 1 invariant: Authority Non-Amplification

### 6.1 Derived Authority, `DA(n)`

Defined recursively over the DAG:

- If `n` is a Principal: `DA(n) = n.declared_authority` (axiomatic).
- If `n` is an Agent:
  `DA(n) = ⋃ { e.authority | e ∈ incoming_edges(n), e.authority ⊆ DA(e.delegator) }`

  Note precisely: an incoming edge that is **not** a valid subset of its
  delegator's derived authority contributes **nothing** to `DA(n)` — not
  even the overlapping part. An invalid grant is fully distrusted, not
  partially honored. This is a deliberate strictness choice: a
  delegator that over-claims has demonstrated its declaration for that
  edge cannot be trusted, and partial-trust semantics would need their
  own justification and tests that Phase 1 does not need.

  Because `G` is acyclic, `DA` is well-founded and computable in one
  topological pass (§8).

### 6.2 The invariant, stated precisely

> **Authority Non-Amplification:** For every node `n` and every scope
> `s` that `n` is declared to exercise or transmit, `s ∈ DA(n)`.

This is checked at exactly two points:

1. **Edge-level:** for every delegation edge `e = (d, t, A)`,
   `A ⊆ DA(d)` must hold. Violation → finding with
   `point = "delegation_edge"`, excess = `A \ DA(d)`.
2. **Operation-level:** for every operation `op = (actor, action,
   requires)`, `requires ∈ DA(actor)` must hold. Violation → finding
   with `point = "operation"`.

Both are the same invariant (`violation = "authority_amplification"`
in the finding, always — see §9), because both express the same fact:
*authority is being attributed to a node that its valid delegation
chain never actually gave it.*

### 6.3 Why this is the only Phase 1 invariant

Product ideas #2 (chain validity) is subsumed above. Ideas #3
(audience binding), #4 (approval preservation), #5 (depth enforcement),
#6 (confused-deputy detection) all require model concepts Phase 1
deliberately excludes (audience/resource identity, approval state,
depth policy field, caller/deputy role distinction beyond
delegator/delegatee). Building one invariant correctly, with a
verified algorithm, precise finding contract, and full determinism
test suite, is the Phase 1 foundation; §21 lays out how each later
invariant attaches to this same graph without breaking it.

---

## 7. Input/schema contract

### 7.1 Top-level document

```json
{
  "version": "1",
  "principals": [
    { "id": "user", "authority": ["billing:read", "billing:write"] }
  ],
  "agents": [
    { "id": "agent-a" },
    { "id": "agent-b" }
  ],
  "delegations": [
    { "delegator": "user", "delegatee": "agent-a", "authority": ["billing:read"] },
    { "delegator": "agent-a", "delegatee": "agent-b", "authority": ["billing:read"] }
  ],
  "operations": [
    { "actor": "agent-b", "action": "billing.view", "requires": "billing:read" },
    { "actor": "agent-b", "action": "billing.refund", "requires": "billing:write" }
  ]
}
```

### 7.2 Field rules

- `version`: required, must equal the literal string `"1"`. Any other
  value (including absent) is a validation error — future-proofing for
  Phase 2+ schema changes without silently misinterpreting old/new
  files.
- `id` (principals, agents): required, unique across the **combined**
  principal+agent namespace (a principal and an agent may not share an
  id), matches `^[A-Za-z0-9_.-]{1,128}$`.
- `principals[].authority`: array of scope strings (§4 grammar), may be
  empty, no duplicate entries within the array.
- `agents[]`: **must not** contain an `authority` key at all. Decoding
  uses `json.Decoder.DisallowUnknownFields()`, so this is enforced for
  free by the schema shape — an agent's authority is never declared,
  only derived. This is a deliberate hard rule, not an oversight: it
  keeps "where does this node's authority come from" unambiguous.
- `delegations[].delegator` / `.delegatee`: required, must reference a
  known node id. `delegatee` must not resolve to a principal.
  `delegator != delegatee` (no self-delegation). No two delegation
  entries may share the same `(delegator, delegatee)` pair (§3.2).
- `delegations[].authority`: non-empty array of scope strings, no
  duplicates within the array.
- `operations[].actor`: required, must reference a known node id
  (principal or agent — checking a principal's own operations is
  legal and trivially passes since `DA(principal)` is axiomatic).
- `operations[].action`: required, non-empty string, matches
  `^[A-Za-z0-9_.-]{1,128}$` (an opaque label; not interpreted).
- `operations[].requires`: required, exactly one scope string
  (§4 grammar). Phase 1 deliberately keeps this singular — an
  operation needing multiple scopes is expressed as multiple
  Operation entries with the same `actor`/`action` and different
  `requires`, avoiding an AND/OR requirement algebra Phase 1 does not
  need.

### 7.3 Parsing strictness

- Strict JSON decoding: unknown top-level or nested fields are a
  validation error (`json.Decoder.DisallowUnknownFields()`), not
  silently ignored. Silent field drops would make later schema
  evolution and typo-detection worse.
- All structural errors are **collected and reported together**, not
  fail-fast on the first error. A model with five problems should
  report five problems in one run.

### 7.4 What makes a model malformed (validation errors, exit code 2)

- Invalid JSON syntax.
- Missing/invalid `version`.
- Unknown field anywhere in the document.
- Missing required field on any entity.
- `id` or scope string failing its regex.
- Duplicate node id (across principals+agents).
- Duplicate `(delegator, delegatee)` pair.
- Delegation referencing an unknown `delegator`/`delegatee`.
- Delegation targeting a principal (`delegatee` resolves to a
  principal).
- Self-delegation (`delegator == delegatee`).
- Empty `authority` array on a delegation edge.
- Duplicate scope string within one authority array (principal or
  delegation).
- Operation referencing an unknown `actor`.
- A cycle anywhere in the delegation graph (including a 1-node
  self-loop, already excluded above, and longer cycles).
- Any resource bound exceeded (§12).

All of the above are **structural** problems, detected by `validate`
and also by `verify` (which runs validation first). They are distinct
from **semantic findings** (§9), which require a structurally valid
model.

---

## 8. Deterministic verification algorithm

Given a structurally valid model:

1. **Build the graph.** Nodes = principals ∪ agents. Edges =
   delegations. (Already known acyclic — cycle detection happened
   during validation, §7.4, using Kahn's algorithm, which both detects
   cycles and produces a topological order as a side effect.)
2. **Topological evaluation**, processing nodes strictly in topological
   order, and — whenever more than one node is simultaneously ready —
   breaking ties by **ascending lexicographic id**, for determinism
   that does not depend on map iteration:
   - Principal `p`: `DA(p) = canonicalize(p.declared_authority)`.
   - Agent `a`: initialize `DA(a) = ∅`. For each incoming edge `e`,
     considered in ascending lexicographic order of `e.delegator`
     (then irrelevant beyond that, since `a` is fixed and pairs are
     unique per §3.2):
     - If `e.authority ⊆ DA(e.delegator)`: `DA(a) := DA(a) ∪
       e.authority`; mark `e` **valid**.
     - Else: mark `e` **invalid**; emit finding
       `{violation: "authority_amplification", point: "delegation_edge", delegator: e.delegator, delegatee: e.delegatee, declared: canonicalize(e.authority), excess: canonicalize(e.authority \ DA(e.delegator))}`.
   - `DA(a) := canonicalize(DA(a))`.
3. **Operation evaluation**, iterating operations in the order:
   ascending `(actor, action, requires)` lexicographic tuple (not
   input-file order — see §8.1):
   - If `op.requires ∈ DA(op.actor)`: pass, no finding.
   - Else: emit finding
     `{violation: "authority_amplification", point: "operation", actor: op.actor, action: op.action, requires: op.requires, held: canonicalize(DA(op.actor)), trace: canonical_trace(op.actor) + [op.action]}`.
4. **Sort all findings** (edge-level and operation-level together) by
   the key tuple: `(point, subject_id, secondary_id_or_action,
   scope)`, where `subject_id` is `delegator` for edge-level findings
   and `actor` for operation-level findings, and `secondary_id_or_action`
   is `delegatee` or `action` respectively. This total order is a pure
   function of finding content, never of map/slice iteration order.
5. **Result:** `ALLOW` (exit 0) if findings is empty, else `DENY`
   (exit 1) with the sorted findings list.

### 8.1 Canonical trace construction

For an operation-level finding, `canonical_trace(actor)` is the
delegation path from a root principal to `actor`, chosen deterministically:

- Run BFS from **all principals simultaneously**, visiting principals
  in ascending id order, and at each node expanding **valid** outgoing
  edges in ascending `delegatee` id order.
- The first path BFS finds to `actor` is canonical (shortest path;
  ties broken by the lexicographic expansion order above, which makes
  the choice deterministic and reproducible without extra
  tie-breaking logic).
- If `actor` is unreachable from any principal via valid edges (e.g.
  an agent that received no valid grants), `canonical_trace(actor) =
  [actor]` — the trace is just the actor itself, and the finding's
  `held` will be `[]`.
- Edge-level findings don't need this: the trace is simply
  `canonical_trace(delegator) + [delegatee]`, since the finding is
  already anchored to a specific edge.

### 8.2 Complexity and why state-space exploration is not needed (§6 of the prompt / decision point)

The whole algorithm is a single topological pass plus a bounded BFS per
finding: **O(N + E + O)** time and space, where N = nodes, E =
delegation edges, O = operations. There is no branching, no
backtracking, no search over alternative interpretations — the model
is static and every quantity (`DA(n)`, edge validity, operation
pass/fail) has exactly one correct value given the input.

**Decision: Phase 1 does not need state-space exploration.** A smaller
deterministic static verifier is the correct and sufficient foundation,
because Phase 1's model has no time dimension, no conditional grants,
no approval workflow with branching outcomes, and no revocation. State-
space/bounded-model-checking framing only becomes necessary when a
later phase introduces genuine nondeterminism or temporal/conditional
structure (e.g., "approval pending" vs "approved" states, session
validity windows, revocation races) — see §21. Introducing a search
engine now, with nothing for it to search over, would be exactly the
"toy graph traversal demo" the product positioning warns against.

---

## 9. Counterexample/finding contract

A finding is:

```json
{
  "violation": "authority_amplification",
  "point": "operation",
  "actor": "agent-b",
  "action": "billing.refund",
  "requires": "billing:write",
  "held": ["billing:read"],
  "trace": ["user", "agent-a", "agent-b", "billing.refund"],
  "reason": "billing:write was never present in the valid delegation chain reaching agent-b"
}
```

or, for an edge-level finding:

```json
{
  "violation": "authority_amplification",
  "point": "delegation_edge",
  "delegator": "agent-a",
  "delegatee": "agent-c",
  "declared": ["billing:write"],
  "excess": ["billing:write"],
  "trace": ["user", "agent-a", "agent-c"],
  "reason": "agent-a attempted to delegate billing:write, which is not in agent-a's derived authority"
}
```

Rules:

- `violation` is always the literal string `"authority_amplification"`
  in Phase 1 (single invariant, §6.3). The field exists now so later
  phases can add new literal values without a breaking schema change.
- `reason` is generated text, not free-form — it is a deterministic
  function of the other fields (same finding always produces the same
  sentence), so it participates in golden-file testing too.
- All array-valued fields (`held`, `excess`, `declared`, `trace`) are
  emitted in the canonical order defined in §8 (sorted for
  authority sets; the specific BFS-derived order for `trace`).
- A finding never includes an unbounded or non-reproducible field
  (timestamps, random ids, absolute file paths).

---

## 10. CLI contract

Two subcommands only:

```
delegationproof validate <model.json>
delegationproof verify   <model.json> [--format text|json]
```

- `validate`: parses and structurally validates only (§7.4). Never
  evaluates the invariant. Useful as a fast pre-check / editor
  integration hook.
- `verify`: runs validation, and if structurally valid, evaluates
  Authority Non-Amplification (§6) and reports findings or a clean
  pass.
- `--format` (default `text`): `text` is a human-readable rendering;
  `json` is the machine-readable rendering of the same finding
  contract (§9), emitted as a single JSON object to stdout, no
  trailing prose.
- No other flags, no config file, no resource-limit overrides in
  Phase 1 (§12 bounds are fixed constants) — deliberately minimal
  surface per the "do not create unnecessary commands" instruction.
- No subcommand for producing/scaffolding example models, no `--fix`,
  no watch mode. Those are not Phase 1 concerns.

### 10.1 Why JSON output is in Phase 1 (and SARIF is not)

Machine-readable output is not feature creep here — it is required by
Phase 1's own determinism goals: the task explicitly requires testing
"deterministic violation ordering," which is only rigorously testable
by diffing exact machine-readable output, not by eyeballing text.
`--format json` is the direct, minimal way to do that and doubles as
the seed for later CI integration. SARIF is a specific, larger,
externally-governed schema aimed at code-scanning tool interop — a
real feature with real design requirements, explicitly deferred (§18).

---

## 11. Exit codes

| Code | Meaning |
|---|---|
| `0` | Clean pass — model structurally valid, zero findings (`verify`), or model structurally valid (`validate`). |
| `1` | Invariant violated — one or more findings (`verify` only). The tool worked correctly; this is a semantic DENY, the useful CI-gating signal. |
| `2` | Model/input problem — file not found/unreadable, invalid JSON, any structural validation error (§7.4), or a resource bound exceeded (§12). Applies to both `validate` and `verify`. |
| `3` | CLI usage error — wrong number of arguments, unknown flag, unknown subcommand. Never about the model's content. |

Exit codes are stable across `text` and `json` formats.

---

## 12. Resource bounds

Fixed constants for Phase 1 (no CLI override — simplicity over
configurability; revisit only if a real need appears):

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

Exceeding any bound is a **validation error** (`resource_limit_exceeded`,
exit code 2), reported deterministically — never a panic, never an
unbounded allocation, never a hang. The chain-depth bound is a safety
valve against pathological/adversarial input, not the future
policy-level depth-limit invariant (product idea #5, §21) — the two
are conceptually different (resource safety vs. declared policy) and
must not be conflated in the implementation.

These constants live in one internal package (e.g.
`internal/limits`) as exported variables (not untyped consts baked
into logic), specifically so tests can construct small fixtures that
exceed a *lowered* limit without needing to generate a genuinely
10,000-node file (§16).

---

## 13. Error behavior

- Structural validation errors (§7.4) are collected exhaustively, not
  fail-fast, and rendered in the same deterministic sort order as
  findings: `(error_kind, primary_id, secondary_id_or_field)`.
- The CLI never panics on malformed or adversarial input within the
  file-size bound (§12) — every failure path is a typed error mapped
  to exit code 2 or 3. A panic reaching `main` is a Phase 1 bug by
  definition and must be covered by a regression test if found.
- Errors always go to stderr; `text`/`json` result output always goes
  to stdout. This split is required for CI usability (stdout is safe
  to pipe/parse even when stderr has diagnostic noise).
- File-not-found / unreadable file is a distinct, clearly worded error
  (exit 2), not a generic JSON parse error.

---

## 14. Example model

Lives at `examples/billing-refund.json` (matches the worked example in
the product brief). Contains exactly:

- One principal `user` with `["billing:read", "billing:write"]`.
- Two agents, `agent-a` and `agent-b`.
- Two delegation edges, `user → agent-a` and `agent-a → agent-b`, both
  granting only `["billing:read"]` — both **valid** (subset checks
  pass), demonstrating a valid delegation chain end to end.
- Two operations on `agent-b`:
  - `billing.view` requiring `billing:read` — **passes** (demonstrates
    the valid chain actually being usable).
  - `billing.refund` requiring `billing:write` — **fails**, because
    `billing:write` was declared at the root principal but never
    delegated down the chain. This is the amplification violation:
    the authority exists at the root, but `agent-b`'s derived
    authority does not include it.

This single file exercises: a valid multi-hop delegation chain, a
passing operation, and a failing operation — satisfying "at least one
valid delegation chain" and "at least one authority-amplification
violation" in one small, readable fixture, matching the product
brief's own worked example exactly (so the documented sample output in
the product brief is literally this file's `verify --format text`
output).

---

## 15. Architecture/file plan

Mapped onto the existing empty scaffold (no new top-level directories):

```
cmd/delegationproof/
  main.go              — flag/subcommand dispatch, exit code mapping, stdout/stderr split

internal/model/
  types.go             — Principal, Agent, Delegation, Operation, Model structs (json tags, DisallowUnknownFields via decoder, not struct tags)

internal/limits/
  limits.go            — exported resource-bound variables (§12)

internal/loader/
  loader.go            — JSON decode + full structural validation (§7), returns collected errors, not fail-fast

internal/graph/
  graph.go             — DAG construction, Kahn's-algorithm topological sort + cycle detection, canonical BFS trace helper (§8.1)

internal/verify/
  verify.go            — DA(n) computation, edge/operation evaluation, finding assembly (§8)

internal/report/
  finding.go           — Finding struct, canonicalization/sorting (§8, §9), reason-string generation
  text.go              — text renderer
  json.go              — json renderer

internal/exitcode/
  exitcode.go           — the 4-value exit-code type (§11) and mapping helpers

examples/
  billing-refund.json  — §14

schemas/
  model.md             — human-readable schema contract (mirrors §7); a formal JSON-Schema
                          file is optional/documentation-only in Phase 1 and, if added, is
                          NOT consulted at runtime (no schema-validation library dependency;
                          internal/loader is the sole source of truth)

testdata/
  valid/               — structurally valid models, various shapes
  malformed/           — one fixture per §7.4 error kind
  golden/              — expected text/json output pairs for determinism tests

docs/
  phase-1-plan.md      — this document
```

`scripts/` is left empty in Phase 1 (no build/release automation is in
scope yet); it stays as a placeholder for later phases.

No third-party dependencies. `go.mod` should remain stdlib-only through
Phase 1 (`encoding/json`, `flag` or a tiny hand-rolled arg parser,
`sort`, `testing`).

---

## 16. Testing plan

Each item is a concrete test category, mapped to the file plan above.

1. **Valid model, clean pass** — `verify` on a model with no
   violations → exit 0, empty findings, golden text + json output.
2. **Malformed JSON syntax** — exit 2, specific parse-error message.
3. **Unknown references** — delegation with unknown `delegator`,
   unknown `delegatee`, and operation with unknown `actor`; three
   distinct fixtures, all exit 2 with the correct error kind.
4. **Duplicate identities** — a principal and an agent sharing an id;
   two agents sharing an id — exit 2.
5. **Duplicate delegation edge** — same `(delegator, delegatee)` pair
   twice — exit 2.
6. **Principal as delegation target** — exit 2.
7. **Self-delegation** — exit 2.
8. **Cycles** — a 2-node cycle and a 3-node cycle fixture, asserting
   the reported cycle path is canonical (rotated to start at the
   lexicographically smallest node) and stable across repeated runs.
9. **Empty authority array on an edge** — exit 2.
10. **Duplicate scope within one authority array** — exit 2.
11. **Agent declaring an `authority` field** — rejected by
    `DisallowUnknownFields` — exit 2.
12. **Valid delegation subset (pass path)** — an operation whose
    `requires` is within `DA(actor)` → no finding, exit 0.
13. **Authority amplification, edge-level** — a delegator delegates a
    scope beyond its own `DA` → exit 1, one `point: "delegation_edge"`
    finding with correct `excess`.
14. **Authority amplification, operation-level** — the `examples/`
    fixture itself (§14) → exit 1, one `point: "operation"` finding
    with exact expected `trace`, `held`, `reason` text (golden test).
15. **Deterministic violation ordering** — construct a model with
    multiple findings (mixed edge/operation), run `verify --format
    json` on (a) the file as authored and (b) a byte-shuffled variant
    where `principals`, `agents`, `delegations`, `operations` arrays
    are reordered (same semantic content) → assert **byte-identical**
    JSON output for both, and run each twice to rule out any
    incidental nondeterminism (e.g. from accidental map iteration).
16. **Resource-limit behavior** — using `internal/limits` overridden
    to small values in test builds (white-box test, same package):
    fixtures exceeding max nodes, max edges, max operations, max
    scope length, max id length, max authority-set size, and max
    chain depth — each → exit 2 with `resource_limit_exceeded` and
    the specific limit name, and each completes quickly (no
    hang/OOM), verified with a test timeout.
17. **CLI usage errors** — no args, too many args, unknown flag,
    unknown subcommand → exit 3, message to stderr.
18. **stdout/stderr split** — error cases write nothing to stdout;
    success/finding cases write nothing diagnostic to stderr.
19. **`validate` vs `verify` divergence** — a model with an
    amplification violation but otherwise structurally valid:
    `validate` → exit 0; `verify` → exit 1. Confirms `validate` never
    evaluates the invariant.
20. **No panics** — a small fuzz/table test feeding truncated,
    empty, and randomly-mutated-byte versions of a valid fixture
    through the full CLI path, asserting only exit 2/3 and no crash.

---

## 17. Security assumptions

- DelegationProof is a **static, offline analyzer**. It proves
  properties about a *declared* model; it does not observe, enforce,
  or intercept real agent/tool traffic. Phase 1 has no runtime
  enforcement component (explicitly out of scope, §18).
- The input model is assumed to be an **honest declaration** of
  intended topology. Phase 1 does not verify that a real running
  system matches its declared model — that correspondence is a
  separate, later integration concern (topology ingestion from actual
  MCP/A2A systems, §21), not something a static verifier can ever
  fully guarantee on its own.
- A principal's `declared_authority` is the axiomatic root of trust in
  the model. Phase 1 does not verify *how* a principal obtained that
  authority (that is identity/OAuth territory, explicitly out of
  scope, §18).
- Parsing is pure data deserialization via the standard library only —
  no code execution, no dynamic loading, no network access, no
  filesystem access beyond reading the one specified input file.
  Combined with the resource bounds (§12), the tool is safe to run
  against untrusted model files without additional sandboxing.
- Not a server, not multi-tenant, not persistent: one file in, one
  deterministic report out, process exits.

---

## 18. Explicit non-goals (Phase 1)

- Networking of any kind (no HTTP server, no client calls out).
- Hosted/SaaS service.
- OAuth or any identity-provider implementation.
- MCP protocol implementation or live MCP server introspection.
- A2A protocol implementation.
- LLM integration of any kind — verification is deterministic,
  non-LLM, by design (this is permanent product positioning, not just
  a Phase 1 gap).
- Runtime enforcement / interception / proxying of any agent or tool
  traffic.
- Z3 or any SAT/SMT solver integration.
- SARIF output (JSON output is in scope, §10.1; SARIF specifically is
  not).
- CI platform integrations (GitHub Actions, etc.) — the exit codes
  (§11) are designed to make this trivial later, but no workflow
  files ship in Phase 1.
- Multi-tenant features, user accounts, RBAC administration UI.
- Any database — the model is a single input file.
- Web UI.
- Automatic policy/model generation from source code, logs, or
  running systems.
- Scope wildcards/hierarchy semantics (§4).
- Audience/resource binding, approval preservation, depth-limit
  policy, confused-deputy detection — the other four invariants from
  the product brief (§6.3, §21).
- Bounded state-space exploration / model checking beyond a single
  deterministic pass (§8.2).
- Configurable resource limits via flags/config file (§12).

---

## 19. Acceptance criteria

- `go build ./...` succeeds with zero third-party dependencies added
  to `go.mod` beyond the existing `module`/`go` directives.
- `go vet ./...` is clean.
- `go test ./...` passes, including every category in §16, with the
  determinism test (§16.15) asserting byte-identical output across
  reordered-but-equivalent input.
- `validate` and `verify` behave exactly per §10, with exit codes
  exactly per §11, for every fixture in `testdata/`.
- `examples/billing-refund.json` (§14) round-trips exactly as
  documented: `validate` → exit 0; `verify` → exit 1 with exactly one
  finding, matching the worked example in the original product brief.
- No panic is reachable from `main` for any input within the §12
  bounds; inputs exceeding bounds fail cleanly with exit 2.
- Findings and validation errors are sorted deterministically per
  §8/§13, never relying on map iteration order.

---

## 20. Definition of DONE

Phase 1 is done when:

1. All items in §19 are met.
2. The file/package layout matches §15 (or a documented, deliberate
   deviation is noted in this same document, keeping it authoritative).
3. Every error kind in §7.4 and every finding `point` in §9 has at
   least one dedicated test per §16.
4. The worked example from the original product brief is reproducible
   verbatim via `delegationproof verify examples/billing-refund.json`.
5. No open TODOs remain in code for functionality described in this
   document as in-scope (TODOs for explicitly-deferred Phase 2+ items
   are fine and expected, ideally linking back to §21).
6. README/CLAUDE.md updates describing the shipped Phase 1 CLI are
   left for the implementation session to write (this planning session
   did not touch those files per instructions).

---

## 21. Future-phase boundaries

Each item names the product-brief idea it belongs to and, briefly, why
it doesn't fit onto the Phase 1 model without a real design pass:

- **Audience/resource binding** (idea #3): requires a resource/audience
  identity concept that doesn't exist yet (§3.1 explicitly excluded
  Tool/Resource/Service nodes). Needs its own containment semantics
  (authority valid "for" a target) layered onto — not replacing —
  the edge/operation shapes defined here.
- **Approval preservation** (idea #4): requires an approval-state
  concept (required/satisfied/bypassed) attached to operations or
  edges, plus a rule that delegation cannot silently drop that
  requirement. Additive to the Operation shape (§7.2), not a rewrite.
- **Delegation-depth enforcement** (idea #5): a *policy* field (a
  declared max-depth) distinct from the §12 resource-bound safety
  valve; needs its own finding kind and its own semantics for where
  the limit is declared (per-principal? per-edge? global?).
- **Confused-deputy detection** (idea #6): needs a notion of "caller"
  distinct from "delegator" — i.e., who *induced* an agent to act,
  as opposed to who granted it authority. This is a genuinely new
  relationship on top of the delegation graph, not an extension of
  `DA(n)`.
- **Bounded state-space exploration**: becomes relevant once any of
  the above introduces real nondeterminism or temporal state
  (approval pending/approved, revocation, session validity windows).
  Until then (§8.2), a single deterministic pass is strictly correct
  and strictly simpler.
- **MCP topology ingestion / A2A representation**: adapters that
  produce a Phase-1-shaped `Model` from real MCP/A2A system
  descriptions. Explicitly a translation layer *in front of* this
  core, never a change to the core invariant engine.
- **JSON Schema enforcement, SARIF, CI templates**: tooling/ecosystem
  layers on top of the stable `--format json` contract established
  in Phase 1 (§9, §10.1); none require changes to the verification
  core itself.
- **Optional Z3/SMT integration**: only relevant if/when a later
  invariant needs constraint solving beyond set inclusion (e.g.
  numeric/interval constraints in structured capabilities) — Phase 1's
  opaque-scope-string model (§4) has no such constraints to solve.
- **Scope wildcard/hierarchy semantics**: needs an explicit, designed
  containment grammar (§4) before it can be added; adding it later
  changes `⊆` from string-set-equality to a real containment
  function, which is a contained, well-isolated change if `DA(n)`'s
  definition (§6.1) is respected.
