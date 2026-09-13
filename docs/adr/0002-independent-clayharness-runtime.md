# ADR 0002: Independent ClayHarness runtime

- Status: Accepted
- Date: 2026-09-12
- Supersedes: the Harness ownership statement in ADR 0001

## Context

Rootforge needs an evidence-driven Agent loop, but the loop itself is not
incident-specific. Coupling it to `IncidentCase`, OOM, or RCA types would prevent
independent testing and reuse. ClayHarness is also intended to be a standalone
CLI and multi-client Agent platform. Embedding its runtime into `rootforged`
would couple release cadence, failure domains, language choice, and lifecycle to
one consumer, and would make a later service extraction an architectural rewrite.

The most important security boundary is not the process boundary. It is the
host-owned Tool Gateway that validates every Agent-requested operation before
any external system is accessed.

## Decision

The team develops and releases ClayHarness as an independent Rust repository,
product, and service. Its headless App Server owns the generic Runtime and is
the shared backend for ClayHarness CLI/TUI, Web, SDK automation, and domain
applications. Rootforge connects through the versioned App Server Protocol,
normally using the generated Go SDK; it does not embed the Runtime.

ClayHarness public contracts remain application-neutral. Rootforge translates
its domain into `RunRequest`, `ArtifactRef`, tool descriptors, and a requested
output schema, then translates `RunResult` into an RCA or escalation.

ClayHarness owns generic Agent execution, reasoning orchestration, budgets,
events, and resumable runtime checkpoints. Rootforge owns the incident domain,
Case state, evidence and findings, ContextGraph, tool registry and execution,
authorization, credentials, audit, RCA versions, actions, and notifications.

For a Rootforge Session, ClayHarness uses a delegated Execution Host profile. It
receives neither infrastructure credentials nor arbitrary shell access. It
requests declared tools through the protocol Tool Bridge. Rootforge checks the
Run and Case scope and policy, performs the bounded read-only operation, persists
Evidence and audit records, and returns a structured result.

The Rootforge Case remains authoritative. A ClayHarness checkpoint is only
runtime continuation state and cannot independently establish incident facts,
RCA state, approval state, or action history.

## Consequences

- ClayHarness and Rootforge have independent versions and test suites.
- Rootforge pins a compatible protocol/SDK version and owns adapter contract tests.
- Incident-specific terms do not enter ClayHarness public APIs.
- The independent service adds deployment, authentication, availability, and
  protocol-compatibility responsibilities; these are accepted product costs.
- Local development may let the SDK supervise a child App Server process, but
  this is process supervision rather than Runtime embedding.
- The service boundary does not transfer domain authority: Rootforge's Case and
  audit ledger remain authoritative, while ClayHarness checkpoints only resume
  generic Runtime execution.
- HolmesGPT and coding-agent Harnesses remain benchmarks and design references,
  not Rootforge runtime dependencies.
