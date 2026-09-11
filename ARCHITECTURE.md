# Rootforge Architecture

Rootforge is an unattended, event-driven incident investigation and remediation
system. An incident event starts the workflow; a human does not need to type an
`ask` command before an investigation can begin.

This document defines the repository boundaries. It intentionally contains no
implementation design beyond what is needed to keep packages separated.

## Runtime shape

```text
Production and external systems
        |
        v
Trigger Intake
        |
        v
Incident Controller -----> Case & Evidence Store
        |                           ^
        v                           |
Harness Agent -------------> Tool Gateway
                                  |
                    +-------------+-------------+
                    |             |             |
                 Evidence      Analyzers    Correlation
                    |             |             |
                    +-------------+-------------+
                                  |
                                  v
                              RCA / Finding
                                  |
                                  v
                                Forge
                                  |
                                  v
                         Policy & Action Executor
                           |                 |
                      safe action       notify developer
```

## Process boundaries

The initial repository reserves three executable boundaries:

- `rootforge`: operator-facing CLI for configuration, inspection, replay, and
  explicit administrative actions. It is not the normal incident trigger.
- `rootforged`: long-running control plane that receives events, owns incident
  state, schedules the Harness Agent, and coordinates Forge and notifications.
- `rootforge-retriever`: optional low-privilege component close to a workload. It
  exposes only predefined, bounded, read-only evidence operations.

These may run in one process during early development. Their permissions and
contracts must remain distinct even when deployed together.

## Package boundaries

| Package area | Owns | Must not own |
| --- | --- | --- |
| `trigger` | Event ingestion and normalization | Root-cause reasoning |
| `incident` | Deduplication, lifecycle, scheduling | Evidence interpretation |
| `casefile` | Case state, evidence lineage, artifacts | Production collection |
| `evidence` | Bounded access to external facts | Long-term telemetry storage |
| `analyzer` | Repeatable deterministic findings | Open-ended agent reasoning |
| `correlation` | Runtime-to-code graph and timeline | Final causal claims |
| `harness` | Planning, hypotheses, tool loop, conclusion | Direct credentials or policy bypass |
| `tool` | Schemas, registry, guarded execution | Business decisions |
| `llm` | Model-provider boundary | Incident workflow |
| `forge` | Candidate change and isolated verification | Unapproved production mutation |
| `policy` | Authorization and automation decisions | Tool implementation |
| `action` | Approved action execution and rollback hooks | Deciding its own permission |
| `notification` | Developer escalation and delivery | Selecting a root cause |
| `audit` | Immutable activity records | Mutable Case state |
| `storage` | Persistence adapters | Domain policy |

## Harness Agent boundary

The Harness Agent is a Rootforge-owned runtime, not an integration with
HolmesGPT. It receives a scoped Incident Case and may only act through tools
registered for that Case. Its loop is conceptually:

```text
observe -> hypothesize -> plan -> call tool -> record evidence -> evaluate
        -> conclude, request more evidence, or escalate
```

Every material conclusion must reference evidence. Every tool call must be
bounded, attributable, and auditable. The Harness Agent can propose a write
action, but only the Policy layer can authorize it and only the Action Executor
can perform it.

## Automation levels

Rootforge separates unattended investigation from unattended mutation:

1. `observe`: detect, investigate, and notify.
2. `recommend`: add RCA and remediation guidance.
3. `prepare`: create and verify a candidate patch or pull request.
4. `remediate`: execute explicitly allowlisted, reversible actions.
5. `autonomous`: repair, verify, and roll back within a declared policy.

The default is deny for all write actions. Deployments choose their allowed
automation level per environment and action type.

## Repository map

```text
rootforge/
├── api/v1alpha1/               External API and event schemas
├── cmd/
│   ├── rootforge/              Administrative CLI
│   ├── rootforged/             Control-plane daemon
│   └── rootforge-retriever/    Optional read-only node retriever
├── configs/examples/           Sanitized configuration examples
├── deployments/                Deployment packaging
├── docs/                       Product and architecture decisions
├── heapdump/hprofx/            Existing standalone HPROF analyzer module
├── internal/
│   ├── action/                 Approved action execution
│   ├── analyzer/               Deterministic analyzers
│   ├── audit/                  Audit trail
│   ├── casefile/               Incident Case and evidence ledger
│   ├── config/                 Configuration loading and validation
│   ├── correlation/            Runtime-to-code graph and timeline
│   ├── evidence/               Bounded evidence acquisition
│   ├── forge/                  Patch generation and verification orchestration
│   ├── harness/                Rootforge Harness Agent
│   ├── incident/               Incident lifecycle controller
│   ├── llm/                    Model-provider adapters
│   ├── notification/           Human escalation
│   ├── policy/                 Permission and automation gates
│   ├── storage/                Persistence adapters
│   ├── tool/                   Agent tool contracts and guarded runner
│   └── trigger/                Incident event adapters
└── testdata/cases/             Sanitized, reproducible incident fixtures
```

The detailed product and layer contracts remain in `docs/`.
