# ADR 0001: Unattended, event-driven core

- Status: Superseded in part by ADR 0002
- Date: 2026-09-11

## Context

Rootforge is intended to find production problems and either prepare a safe fix
or notify a developer without waiting for a human to ask a question.

## Decision

An incident event, not an interactive prompt, is the primary workflow entry.
Rootforge owns its Harness Agent. The Agent operates on a scoped Incident Case
through registered tools. Write actions are proposed by the investigation and
Forge layers, authorized by policy, and performed by a separate Action Executor.

## Consequences

- Trigger ingestion and Incident lifecycle are first-class modules.
- The CLI is administrative rather than the primary investigation interface.
- Unattended investigation is enabled independently from production mutation.
- Agent decisions, evidence, tool calls, actions, and notifications are auditable.
- HolmesGPT may be used as a benchmark but is not a runtime dependency.
