# Threat Model

**DelegationProof analyzes a declared delegation model; it is not a
runtime enforcement point, an authentication system, or an observer of
real agent traffic.**

Everything below elaborates that one sentence concretely, per the
structure `docs/v1-release-plan.md` §11 defines.

## Protected assets / properties

The property this tool exists to protect is: **a node's derived
authority, as reported, never overstates what the declared model
actually, validly grants it.** Concretely, this decomposes into the six
invariants (see the README's "What it checks" section and the table
below) — each is one axis of the same question: *given a declared
delegation topology, can any node ever legitimately exercise authority
it wasn't validly, currently, and safely granted?*

| # | Invariant | Protects against |
|---|---|---|
| 1 | Authority Non-Amplification | A node exercising or receiving authority never validly delegated to it. |
| 2 | Context/Target Binding Preservation | A capability valid for one target being treated as valid for a different one. |
| 3 | Requester Authorization Preservation | A legitimate actor being used as a confused deputy on behalf of a requester who never independently held the capability. |
| 4 | Delegation Depth Preservation | A capability traveling farther through re-delegation than its own declared budget permits. |
| 5 | Approval Preservation | An approval-gated capability being exercised with no standing-backed approval declared for it. |
| 6 | Temporal Approval Preservation | An approval being relied upon when its own declared lifecycle can reach a non-`approved` state. |

## Trust boundaries

There is exactly **one** trust boundary: the single input JSON file.
Everything inside `internal/` operates on already-parsed Go values
derived from that one file. There is no second boundary — no network
input, no plugin/extension loading, no second config file, no
environment-variable-driven behavior that affects analysis output.

## Attacker-controlled input

The entire content of the input file is attacker-controlled:

- Arbitrary JSON syntax, including deliberately malformed or truncated
  bytes.
- Arrays sized arbitrarily large, up to and beyond every declared bound
  in `internal/limits`.
- Arbitrarily deep or wide delegation graphs, including cycles (which
  must be rejected as a structural error) and self-delegation.
- Adversarially-crafted, possibly-cyclic lifecycle automata (version 6).
- ids, scopes, and targets crafted to be maximally long, empty, or
  colliding across namespaces.

## Attacker goals

The relevant "attacker" here is a malicious *input author* — not a
runtime attacker (see Non-goals below). What such an author might want
DelegationProof to incorrectly do:

1. **Cause a false `ALLOW` (exit `0`) on a model that actually contains
   a violation.** This is a soundness break and the single most serious
   possible bug class — it is exactly what `SECURITY.md`'s scope
   section names as the top-priority report category.
2. **Cause a crash, panic, hang, or unbounded memory allocation.** A
   resource-exhaustion break.
3. **Cause nondeterministic output**, so that two runs of "the same"
   model disagree — which would undermine the ability to reproduce or
   trust any single result.

## Defenses

- **Strict distrust / no partial credit at every layer.** An invalid
  incoming delegation edge or a non-standing approval record
  contributes *nothing* to its target — never the overlapping portion.
  See `CLAUDE.md` and `TestStrictDistrustNoPartialCredit*` in
  `internal/verify/verify_test.go` / `verify_v2_test.go`.
- **Exhaustive, not fail-fast, structural validation** — except for the
  singular case of an unparseable document (JSON syntax errors,
  unknown-field decode errors). Implemented in `internal/loader`.
- **Every resource bound is an exported, tested `var`**
  (`internal/limits`), so a document that would otherwise cause
  pathological work instead produces a bounded `resource_limit_exceeded`
  structural-validation error.
- **No third-party dependency surface.** `go.mod` is stdlib-only — there
  is no supply-chain trust extended to anything but the Go toolchain
  itself.
- **Pure data deserialization.** No code execution, no dynamic loading,
  no network access, no filesystem access beyond the one input file.

## Fail-closed behavior

- Malformed input → exit `2`.
- A resource bound exceeded → exit `2`.
- An incomplete/truncated lifecycle exploration → the
  `approval_lifecycle_unproven` finding, exit `1` (a DENY, never a
  silent pass — see `TestFailClosedTruncationYieldsUnproven` and
  `TestFailClosedNeverResolvesToAllow` in
  `internal/verify/verify_v6_test.go`).
- Any invalid incoming delegation edge or non-standing approval record
  contributes nothing to the recipient's derived state (no partial
  credit — see Defenses above).

**Exit `1` vs. exit `2`, stated precisely because it is easy to
conflate:** exit `1` means the tool parsed a structurally valid model
and correctly found a real invariant violation — the tool *worked*, and
this includes the `approval_lifecycle_unproven` case, whose
justification is "could not be proven safe" rather than "proven
unsafe," but which is still a semantic DENY, not an invalid-input
failure. Exit `2` means the tool could not even establish that the
input was a legitimate, analyzable model — a JSON syntax error, a
referential-integrity violation, or a resource bound exceeded.

## Resource exhaustion controls

Every bound in the README's "Resource bounds" table
(`internal/limits`) is a hard, enforced ceiling: file size, node count,
edge count, operation count, string lengths, authority-set size, chain
depth, declared `max_delegation_depth` value, approval count, lifecycle
state/transition count, and the runtime lifecycle-exploration visited-
state count. Each turns what would otherwise be unbounded input into a
bounded, structural exit-`2` error — never a panic, never a hang.

## Assumptions

- The input document is the sole source of truth. DelegationProof does
  not verify it against any external system.
- A principal's own declared root authority is the axiomatic root of
  trust — the tool does not verify how a principal obtained it (that is
  identity/OAuth territory, entirely out of scope).
- A `requester` (version 3+) is a declared label, not an authenticated
  identity.
- A declared `approvals[]` entry (version 5+) is a claim by the
  document's author, not an authenticated or observed real-world
  sign-off event.
- A declared `lifecycle` (version 6) is a claim by the document's
  author about a possible state automaton, not an observed event log —
  DelegationProof cannot observe or reason about *when*, in real time,
  an operation executes relative to a declared transition. This is why
  its safety predicate is universal ("every reachable state must be
  `approved`") rather than an attempt to prove an operation happens to
  run during a currently-approved window, which is unknowable from a
  static, offline document.

## Non-goals

DelegationProof is **not** a runtime enforcement point, is **not**
watching real agent/tool traffic, and is **not** an identity or
authentication system. A "threat" to this tool is a bug in its own
analysis — soundness, crash, or nondeterminism (see Attacker goals
above) — not a real-world attacker exploiting the *modeled* system,
which is entirely outside what a static analyzer can see. This mirrors,
and does not duplicate, the product-level non-goals lists already
stated in each `docs/phase-*-plan.md` and the README's own "Non-goals"
section (no networking, no runtime interception, no identity provider,
no MCP/A2A protocol implementation, no SAT/SMT solving) — this
section's framing is specifically the security-relevant angle on the
same boundary.

## Limitations

A dishonest or careless model author can declare a topology that does
not match reality, and DelegationProof has no way to detect that. The
tool proves properties of the *document*, never of the *world* the
document claims to describe. It also does not prove that a `target`
label corresponds to any real access-control boundary, that a named
`approver` is a real authenticated person, or that a declared
`lifecycle` transition corresponds to any real compliance system's
actual behavior — each of these is a declared fact by the document's
author, checked for internal consistency against the rest of the
declared model, never against reality.
