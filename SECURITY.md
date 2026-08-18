# Security Policy

DelegationProof is a static, offline analyzer for declared delegation
models — it is not a runtime enforcement point (see
[`docs/threat-model.md`](docs/threat-model.md)). A "security issue" here
means a bug in DelegationProof's own analysis, not a vulnerability in a
system it was used to model.

## Scope

A security issue in DelegationProof is one of the following:

- **A soundness bug.** An input for which the tool reports `ALLOW`
  (exit `0`, zero findings) but a careful reading of the applicable
  `docs/phase-*-plan.md` design contract shows the model actually
  violates one of the six invariants — or, symmetrically, a case where
  the tool reports a finding that the design contract shows should not
  fire. This is the single most serious bug class: it means the tool's
  stated guarantee (§2 of `docs/v1-release-plan.md`, the README's
  "Security assumptions" section) does not hold.
- **A crash, panic, hang, or unbounded resource consumption**, triggered
  by untrusted input within or at the resource bounds documented in
  `internal/limits` and the README's "Resource bounds" table.

**Not in scope** (open an ordinary GitHub issue instead): feature
requests, "I wish it also checked X," design disagreements about what a
phase's invariant should cover, or questions about a modeled system's
real-world behavior — DelegationProof only ever reasons about the
declared document, never about reality (see `docs/threat-model.md`'s
non-goals).

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting: open this
repository's **Security** tab and select **"Report a vulnerability"**.
This keeps the report private until a fix is available and requires no
separate email infrastructure.

Do not open a public issue for a suspected soundness or
resource-exhaustion bug before a fix has landed.

## Response expectations

This is a solo-maintained open-source project. Reports are handled on a
best-effort basis — there is no fixed SLA and no dedicated security team.
Reporters will be credited in the release notes for the fix unless they
prefer to remain anonymous.
